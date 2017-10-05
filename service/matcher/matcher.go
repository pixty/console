package matcher

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"sync"
	"time"

	"github.com/jrivets/gorivets"
	"github.com/jrivets/log4g"
	"github.com/pixty/console/common"
	"github.com/pixty/console/model"
)

type (
	Matcher interface {
	}

	matcher struct {
		C2oCache common.CamId2OrgIdCache `inject:"cam2orgCache"`
		MainCtx  context.Context         `inject:"mainCtx"`
		Cache    MatcherCache            `inject:"matcherCache"`
		CConfig  common.ConsoleConfig    `inject:""`

		logger      log4g.Logger
		lock        sync.Mutex
		orgMatchers map[int64]*org_matcher
		cmp_params  face_cmp_params
	}

	mchr_packet struct {
		persons map[string]*person_desc
	}

	org_matcher struct {
		logger log4g.Logger
		orgId  int64
		// counts references to the object, used for closing channels
		refCntr       int
		matcher       *matcher
		inpChnl       chan *mchr_packet
		fresh_packets []*mchr_packet
		mchngPers     map[string]*person_desc
	}

	person_desc struct {
		person *model.Person
		faces  []*face_desc
	}

	face_desc struct {
		face     *model.Face
		state    int
		startIdx int64
		endIdx   int64
	}

	face_cmp_params struct {
		positiveTshld float32
		maxDistance   float64
	}
)

const (
	FD_STATE_INIT     = 0
	FD_STATE_MIDL     = 1
	FD_STATE_FRMSTART = 2
	FD_STATE_END      = 3
)

func NewMatcher() Matcher {
	return new(matcher)
}

// ========================== PostConstructor ================================
func (m *matcher) DiPostConstruct() {
	m.logger = log4g.GetLogger("Matcher")
	m.orgMatchers = make(map[int64]*org_matcher)
	m.cmp_params.maxDistance = m.CConfig.MchrDistance
	m.cmp_params.positiveTshld = float32(m.CConfig.MchrPositiveTrshld) / 100.0
}

// ============================== Matcher ====================================
// When new faces are received and this person are not matched with somebody else yet
func (m *matcher) OnNewFaces(camId int64, persons []*model.Person, faces []*model.Face) {
	if len(persons) == 0 || len(faces) == 0 {
		return
	}

	var mp mchr_packet
	mp.persons = make(map[string]*person_desc)

	for _, p := range persons {
		if p.MatchGroup <= 0 {
			pd := &person_desc{p, make([]*face_desc, 0, 1)}
			mp.persons[p.Id] = pd
		}
	}

	for _, f := range faces {
		pd, ok := mp.persons[f.PersonId]
		if !ok {
			continue
		}
		pd.faces = append(pd.faces, &face_desc{face: f, state: FD_STATE_INIT})
	}

	if len(mp.persons) == 0 {
		return
	}

	om, err := m.getOrgMatcher(camId)
	if err != nil {
		m.logger.Warn("Could not obtain org matcher, err=", err)
		return
	}

	defer m.releaseOrgMatcher(om)
	om.inpChnl <- &mp
}

// release org_matcher
func (m *matcher) releaseOrgMatcher(om *org_matcher) {
	m.lock.Lock()
	defer m.lock.Unlock()

	om.refCntr--
	if om.refCntr == 0 {
		close(om.inpChnl)
		delete(m.orgMatchers, om.orgId)
	}
}

// release org_matcher only if there is no other users
func (m *matcher) releaseOrgMatcherIfNoUsers(om *org_matcher) bool {
	m.lock.Lock()
	defer m.lock.Unlock()

	if om.refCntr > 1 {
		return false
	}
	close(om.inpChnl)
	delete(m.orgMatchers, om.orgId)
	return true
}

// returns an org matcher or error, after using releaseOrgMatcher() must be called for the matcher
func (m *matcher) getOrgMatcher(camId int64) (*org_matcher, error) {
	orgId := m.C2oCache.GetOrgId(camId)
	if orgId <= 0 {
		return nil, common.NewError(common.ERR_NOT_FOUND, "Could not find org by camId="+strconv.FormatInt(camId, 10))
	}

	m.lock.Lock()
	om, ok := m.orgMatchers[orgId]
	if ok {
		om.refCntr++
		return om, nil
	}
	m.lock.Unlock()

	om = new(org_matcher)
	om.logger = m.logger.WithId("{orgId=" + strconv.FormatInt(orgId, 10) + "}").(log4g.Logger)
	om.orgId = orgId
	om.matcher = m
	om.inpChnl = make(chan *mchr_packet, 1000)
	om.refCntr = 1
	go func(om *org_matcher) {
		om.process()
	}(om)

	m.lock.Lock()
	m.orgMatchers[orgId] = om
	m.lock.Unlock()

	return om, nil
}

// Processing organization requests
func (om *org_matcher) process() {
	om.logger.Info("Starting process")
	defer om.matcher.releaseOrgMatcher(om)
	for {
		select {
		case <-om.matcher.MainCtx.Done():
			om.logger.Info("Main context is closed, shutting down.")
			return
		case mp, ok := <-om.inpChnl:
			if !ok {
				om.logger.Error("input channel is closed, shutting down, but it never should happen here :(")
				return
			}
			om.fresh_packets = append(om.fresh_packets, mp)
			om.processFaces()
		case <-time.After(time.Minute):
			if om.matcher.releaseOrgMatcherIfNoUsers(om) {
				om.logger.Info("Closing by timeout")
				return
			}
			om.logger.Info("Tried to release the org matcher, but it seems there are still other users.")
		}
	}
}

func (om *org_matcher) readAllPackUnblocked() {
	for {
		select {
		case mp := <-om.inpChnl:
			om.fresh_packets = append(om.fresh_packets, mp)
		default:
			return
		}
	}
}

func (om *org_matcher) addRequestsToWork() bool {
	om.readAllPackUnblocked()
	for _, mp := range om.fresh_packets {
		for pid, pd := range mp.persons {
			mpd, ok := om.mchngPers[pid]
			if !ok {
				om.mchngPers[pid] = pd
				continue
			}
			mpd.addFaces(pd)
		}
	}
	om.fresh_packets = om.fresh_packets[:0]
	return len(om.mchngPers) > 0
}

func (om *org_matcher) processFaces() {
	for om.addRequestsToWork() {
		cBlk := om.matcher.Cache.NextCacheBlock(om.orgId)
		if cBlk == nil {
			return
		}
	pdLoop:
		for _, pd := range om.mchngPers {
			for _, fd := range pd.faces {
				mr := fd.compareWithCacheBlock(cBlk, &om.matcher.cmp_params)
				if mr != nil {
					cBlk.onMatch(mr, pd.toMatcherRecord())
					delete(om.mchngPers, pd.person.Id)
					continue pdLoop
				}
			}
			if pd.pruneFaces() {
				cBlk.onNewMG(pd.toMatcherRecord())
				delete(om.mchngPers, pd.person.Id)
			}
		}
	}
}

func maxInt64(a, b int64) int64 {
	if a < b {
		return b
	}
	return a
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

// ---------------------------- person_desc ----------------------------------
func (pd *person_desc) addFaces(othrPD *person_desc) {
	if pd.person.Id != othrPD.person.Id {
		panic("addFaces(othrPD *person_desc): wrong person Ids: expected " + pd.person.Id + ", but actually is " + othrPD.person.Id)
	}
	pd.faces = append(pd.faces, othrPD.faces...)
}

func (pd *person_desc) pruneFaces() bool {
	nf := make([]*face_desc, 0, len(pd.faces))
	for _, fd := range pd.faces {
		if fd.state != FD_STATE_END {
			nf = append(nf, fd)
		}
	}
	pd.faces = nf
	return len(nf) == 0
}

func (pd *person_desc) toMatcherRecord() *model.MatcherRecord {
	faces := make([]*model.Face, len(pd.faces))
	for i, fd := range pd.faces {
		faces[i] = fd.face
	}
	return &model.MatcherRecord{Person: pd.person, Faces: faces}
}

// ----------------------------- face_desc ------------------------------------
func (fd *face_desc) String() string {
	return fmt.Sprint("{faceId=", fd.face.Id, ", state=", fd.state, ", startIdx=", fd.startIdx, ", endIdx=", fd.endIdx, "}")
}

// gets a fd and compares it with a block of faces. returns
func (fd *face_desc) compareWithCacheBlock(cb *cache_block, fcp *face_cmp_params) *model.MatcherRecord {
	if fd.state == FD_STATE_INIT {
		fd.startIdx = cb.startIdx
		fd.endIdx = math.MaxInt64
		if cb.startIdx == 0 {
			fd.state = FD_STATE_FRMSTART
		} else {
			fd.state = FD_STATE_MIDL
		}
	} else if cb.startIdx == 0 && fd.state == FD_STATE_MIDL {
		fd.endIdx = fd.startIdx
		fd.startIdx = 0
		fd.state = FD_STATE_FRMSTART
	}

	// startIndex inclusive
	cmpStart := maxInt64(fd.startIdx, cb.startIdx)
	// end index exclusive
	cmpEnd := minInt64(fd.endIdx, cb.endIdx+1)

	if cmpStart > cb.endIdx || cmpEnd < cb.startIdx {
		fd.state = FD_STATE_END
		log4g.GetLogger("Matcher").Warn("Strange things happen, we have fd=", fd,
			", which doesn't fit by indexes with cb=", cb, ", marking it as we done with this!")
		return nil
	}

	endIdx := cb.getInsertIdx(cmpEnd)
	for i := cb.getInsertIdx(cmpStart); i < endIdx; i++ {
		mr := cb.records.Records[i]
		if fd.matchWithCacheRecord(mr, fcp) {
			fd.state = FD_STATE_END
			return mr
		}
	}
	// next time will start from the index which we did not check yet
	fd.startIdx = cmpEnd
	if fd.startIdx >= fd.endIdx || (fd.state == FD_STATE_FRMSTART && cb.lastBlock) {
		fd.state = FD_STATE_END
	}
	return nil
}

func (fd *face_desc) matchWithCacheRecord(mr *model.MatcherRecord, fcp *face_cmp_params) bool {
	total := len(mr.Faces)
	needed := gorivets.Max(1, int(float32(total)*fcp.positiveTshld+0.5))
	for i := 0; needed > 0 && needed+i <= total; i++ {
		if common.MatchAdvanced2V128D(fd.face.V128D, mr.Faces[i].V128D, fcp.maxDistance) {
			needed--
		}
	}
	return needed == 0
}