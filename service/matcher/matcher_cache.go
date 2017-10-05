package matcher

import (
	"fmt"
	"strconv"
	"sync"

	"github.com/jrivets/gorivets"
	"github.com/jrivets/log4g"
	"github.com/pixty/console/common"
	"github.com/pixty/console/model"
)

type (
	// Will implement it for dependency injecting purposes
	MatcherCache interface {
		NextCacheBlock(orgId int64) *cache_block
	}

	cache struct {
		Persister model.Persister      `inject:""`
		CConfig   common.ConsoleConfig `inject:""`

		lock      sync.Mutex
		logger    log4g.Logger
		orgBlocks map[int64]*org_cache
		// keeps and controls caches in memory if needed 1 block per 1 org
		mainCache gorivets.LRU
	}

	// the object keep org cache state
	org_cache struct {
		orgId  int64
		ch     *cache
		logger log4g.Logger
		// contains an index the next block should be searched from
		nextIdx int64
	}

	cache_block struct {
		orgCache *org_cache
		// records sorted in ascending MG order
		records *model.MatcherRecords
		// for first block it is 0!
		startIdx  int64 // contains a value which is same or less to min one from records set
		endIdx    int64 // contains maximum match group value in records set, or 0 if len(records) == 0
		lastBlock bool
	}
)

func NewMatcherCache() MatcherCache {
	ch := new(cache)
	return ch
}

// ========================== PostConstructor ================================
func (ch *cache) DiPostConstruct() {
	ch.logger = log4g.GetLogger("MatcherCache")
	ch.mainCache = gorivets.NewLRU(int64(ch.CConfig.MchrCacheSize), nil)
}

// =========================== MatcherCache ==================================
// read the next cache block and return it when it is ready. returns nil, when over
func (ch *cache) NextCacheBlock(orgId int64) *cache_block {
	ch.lock.Lock()
	orgCh, ok := ch.orgBlocks[orgId]
	if !ok {
		ch.logger.Debug("NextCacheBlock(): New cache block created for orgId=", orgId)
		orgCh = new(org_cache)
		orgCh.orgId = orgId
		orgCh.ch = ch
		orgCh.logger = ch.logger.WithId("{orgId=" + strconv.FormatInt(orgId, 10) + "}").(log4g.Logger)
		orgCh.nextIdx = 0
	}

	inf, ok := ch.mainCache.Get(orgId)
	if ok {
		cb := inf.(*cache_block)
		ch.logger.Debug("NextCacheBlock(): found block in cache ", cb)
		if cb.isCompleted() {
			ch.logger.Debug("NextCacheBlock(): use this one, because it is completed")
			ch.lock.Unlock()
			return cb
		}
	}
	ch.lock.Unlock()

	cb := orgCh.readNextBlock()
	return cb
}

// ============================= org_cache ===================================
func (oc *org_cache) readNextBlock() *cache_block {
	ptx, err := oc.ch.Persister.GetPartitionTx("FAKE")
	if err != nil {
		oc.logger.Error("readNextBlock(): Oops, could not get ptx, err=", err)
		return nil
	}

	limit := oc.ch.CConfig.MchrCachePerOrgSize
	oc.logger.Debug("readNextBlock(): startIdx=", oc.nextIdx, ", limit=", limit)
	res, err := ptx.FindPersonsForMatchCache(oc.orgId, oc.nextIdx, limit)
	if err != nil {
		oc.logger.Error("readNextBlock(): could not read cache block err=", err)
		return nil
	}

	if res.FacesCnt == 0 {
		oc.nextIdx = 0
		oc.logger.Debug("readNextBlock(): read 0 records, starting from beginning.")
		res, err = ptx.FindPersonsForMatchCache(oc.orgId, oc.nextIdx, limit)
		if err != nil {
			oc.logger.Error("readNextBlock(): weird. Second read attempt failed, err=", err)
			return nil
		}
	}

	resCb := new(cache_block)
	resCb.orgCache = oc
	resCb.records = res
	resCb.startIdx = oc.nextIdx
	resCb.lastBlock = res.FacesCnt < limit

	if res.FacesCnt == limit {
		oc.logger.Debug("readNextBlock(): ", limit, " records from DB were read, what hits limit, trim last person, it could be uncompleted.")
		lastIdx := len(res.Records) - 1
		res.FacesCnt -= len(res.Records[lastIdx].Faces)
		res.Records = res.Records[:lastIdx]
		res.MaxMG = res.Records[lastIdx-1].Person.MatchGroup
	}

	if res.FacesCnt > 0 {
		resCb.endIdx = res.MaxMG
		oc.nextIdx = resCb.endIdx + 1
	}

	// adding to block to the cache
	oc.putToCache(resCb)
	oc.logger.Debug("readNextBlock(): read from DB ", resCb)

	return resCb
}

func (oc *org_cache) putToCache(cb *cache_block) {
	oc.ch.lock.Lock()
	defer oc.ch.lock.Unlock()

	if cb.oversized() {
		oc.ch.mainCache.Delete(oc.orgId)
	} else {
		oc.ch.mainCache.Add(oc.orgId, cb, int64(cb.records.FacesCnt))
	}
}

func (oc *org_cache) applyMatchGroup(personId string, mg int64) error {
	ptx, err := oc.ch.Persister.GetPartitionTx("FAKE")
	if err != nil {
		oc.logger.Warn("applyMatchGroup(): could not get persister err=", err)
		return err
	}

	return ptx.UpdatePersonMatchGroup(personId, mg)
}

func (oc *org_cache) applyNewMatchGroup(personId string) (int64, error) {
	ptx, err := oc.ch.Persister.GetPartitionTx("FAKE")
	if err != nil {
		oc.logger.Warn("applyNewMatchGroup(): could not get persister, err=", err)
		return 0, err
	}
	ptx.Begin()
	defer ptx.Commit()

	person, err := ptx.GetPersonById(personId)
	if err != nil {
		oc.logger.Warn("applyNewMatchGroup(): could not find person by personId=", personId, ", err=", err)
		ptx.Rollback()
		return 0, err
	}

	prfId, err := ptx.InsertProfile(&model.Profile{OrgId: oc.orgId, PictureId: person.PictureId})
	if err != nil {
		oc.logger.Warn("applyNewMatchGroup(): could not insert new profile, err=", err)
		ptx.Rollback()
		return 0, err
	}

	err = ptx.UpdatePersonMatchGroup(personId, prfId)
	if err != nil {
		oc.logger.Warn("applyNewMatchGroup(): could not apply match group persId=", personId, ", mg=", prfId, ", err=", err)
		ptx.Rollback()
	}
	return prfId, err
}

// ============================ cache_block ===================================
func (cb *cache_block) String() string {
	return fmt.Sprint("{orgId=", cb.orgCache.orgId, ", persons=", len(cb.records.Records), ", faces=", cb.records.FacesCnt,
		", startIdx=", cb.startIdx, ", endIdx=", cb.endIdx, ", lastBlock=", cb.lastBlock, ", completed=", cb.isCompleted(), "}")
}

// when a match happens the candidate (cand) MatcherRecord is receiving
// the match group from an existing(exst) one.
func (cb *cache_block) onMatch(cand, exst *model.MatcherRecord) error {
	cb.orgCache.logger.Debug("Applying existing match group ", exst.Person.MatchGroup, " to a candidate persoinId=", cand.Person.Id)
	err := cb.orgCache.applyMatchGroup(cand.Person.Id, exst.Person.MatchGroup)
	if err != nil {
		return err
	}

	cand.Person.MatchGroup = exst.Person.MatchGroup
	idx := cb.getInsertIdx(exst.Person.MatchGroup)
	cb.records.Records = append(cb.records.Records, cand) // makes the len of records one element bigger
	if idx < len(cb.records.Records)-1 {
		copy(cb.records.Records[idx+1:], cb.records.Records[idx:])
		cb.records.Records[idx] = cand
	}
	cb.records.FacesCnt += len(cand.Faces)
	cb.orgCache.putToCache(cb)
	return nil
}

// all faces were checked and nothing was found, now assign new MG
func (cb *cache_block) onNewMG(cand *model.MatcherRecord) error {
	mg, err := cb.orgCache.applyNewMatchGroup(cand.Person.Id)
	if err != nil {
		return err
	}

	cand.Person.MatchGroup = mg
	if cb.lastBlock {
		if !cb.oversized() {
			cb.records.Records = append(cb.records.Records, cand)
			cb.records.MaxMG = mg
			cb.records.FacesCnt += len(cand.Faces)
			cb.endIdx = mg
			cb.orgCache.putToCache(cb)
		} else {
			cb.lastBlock = false
		}
	}
	return nil
}

// returns whether the block is oversized
func (cb *cache_block) oversized() bool {
	return cb.records.FacesCnt > cb.orgCache.ch.CConfig.MchrCachePerOrgSize
}

// returns whether the block is completed - the block contains all records for the org
func (cb *cache_block) isCompleted() bool {
	return cb.startIdx == 0 && cb.lastBlock
}

// returns index for inserting new element to the records array with the provided MG
func (cb *cache_block) getInsertIdx(mg int64) int {
	ln := len(cb.records.Records)
	if ln == 0 {
		return 0
	}

	if cb.records.Records[ln-1].Person.MatchGroup < mg {
		return ln
	}

	h := ln - 1
	l := 0
	for l <= h {
		m := (l + h) >> 1
		v := cb.records.Records[m].Person.MatchGroup
		switch {
		case v > mg:
			h = m - 1
		case v < mg:
			l = m + 1
		default:
			for m > 0 && cb.records.Records[m-1].Person.MatchGroup == mg {
				m--
			}
			return m
		}
	}
	return l
}
