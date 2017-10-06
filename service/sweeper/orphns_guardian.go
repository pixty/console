package sweeper

import (
	"context"
	"fmt"
	"time"

	"github.com/jrivets/log4g"
	"github.com/pixty/console/common"
	"github.com/pixty/console/model"
	"github.com/pixty/console/service/matcher"
)

type (
	// The orphants persons are the persons who don't have match group assigned.
	// the guardian (sweeper) takes care about them and sends them to the matcher
	// for obtaining profile or match group out of there
	OrphPersonsGuardian interface {
	}

	orph_persons_sweeper struct {
		CConfig   *common.ConsoleConfig `inject:""`
		Persister model.Persister       `inject:"persister"`
		MainCtx   context.Context       `inject:"mainCtx"`
		Matcher   matcher.Matcher       `inject:"matcher"`

		logger     log4g.Logger
		checkSince time.Time
	}

	hldr struct {
		persons []*model.Person
		faces   []*model.Face
	}
)

func (h *hldr) String() string {
	return fmt.Sprint("{persons=", len(h.persons), ", faces=", len(h.faces), "}")
}

func NewOrphPersonsGuardian() OrphPersonsGuardian {
	ops := new(orph_persons_sweeper)
	ops.logger = log4g.GetLogger("pixty.OrphPersonsGuardian")
	ops.checkSince = time.Now()

	return ops
}

// ========================== PostConstructor ================================

// LifeCycler because of persistor should be initialized before the component
// ----------------------------- LifeCycler ----------------------------------
func (ops *orph_persons_sweeper) DiPhase() int {
	return common.CMP_PHASE_DB + 1
}

func (ops *orph_persons_sweeper) DiInit() error {
	go func() {
		ops.process()
	}()
	return nil
}

func (ops *orph_persons_sweeper) DiShutdown() {
	ops.logger.Info("Shutting down.")
}

func (ops *orph_persons_sweeper) process() {
	ops.logger.Info("Start processing")
	for ops.MainCtx.Err() == nil {
		ops.foundAndHandleOrphants()
		select {
		case <-ops.MainCtx.Done():
			ops.logger.Info("Main context is closed. Exit processing.")
			return
		case <-time.After(time.Duration(ops.CConfig.SweepOrphPersonsMins) * time.Minute):
			ops.logger.Info("Time to check for orphants")
		}
	}
}

func (ops *orph_persons_sweeper) foundAndHandleOrphants() {
	total := 0
	var mg int64
	var pq model.PersonsQuery
	pq.Limit = 200
	pq.Order = model.PQO_ID_ASC
	pq.MaxLastSeenAt = common.ToTimestamp(ops.checkSince)
	pq.MatchGroup = &mg
	for ops.MainCtx.Err() == nil {
		recs, err := ops.selectRecords(&pq)
		if err != nil {
			ops.logger.Warn("Got error while trying to read records for ", pq, ", err=", err)
			return
		}

		persCnt := len(recs.persons)
		if persCnt == 0 {
			ops.logger.Info("No persons with match_group == 0 before ", ops.checkSince, ". ", total, " persons were found this round. Will check again in ", ops.CConfig.SweepOrphPersonsMins, "mins.")
			ops.checkSince = time.Now()
			return
		}

		pq.MinId = &recs.persons[persCnt-1].Id
		total += persCnt

		hm := ops.splitOnCams(recs)
		for camId, h := range hm {
			ops.logger.Info("Found ", h, " persons with match_group=0, for camId=", camId)
			ops.Matcher.OnNewFaces(camId, h.persons, h.faces)
		}
	}
}

func (ops *orph_persons_sweeper) splitOnCams(recs *hldr) map[int64]*hldr {
	res := make(map[int64]*hldr)
	pm := make(map[string]*hldr)
	for _, p := range recs.persons {
		h, ok := res[p.CamId]
		if !ok {
			h = new(hldr)
			h.persons = make([]*model.Person, 0, 20)
			h.faces = make([]*model.Face, 0, 5)
			res[p.CamId] = h
		}
		h.persons = append(h.persons, p)
		pm[p.Id] = h
	}

	for _, f := range recs.faces {
		h, ok := pm[f.PersonId]
		if !ok {
			ops.logger.Error("Something goes wrong - we have face with person_id=", f.PersonId, ", but don't have the person in the map!")
			continue
		}
		h.faces = append(h.faces, f)
	}

	return res
}

func (ops *orph_persons_sweeper) selectRecords(pq *model.PersonsQuery) (*hldr, error) {
	var res hldr
	ctx, err := ops.Persister.GetPartitionTx("FAKE")
	if err != nil {
		return nil, err
	}
	ctx.Begin()
	defer ctx.Commit()

	persons, err := ctx.FindPersons(pq)
	if err != nil {
		return nil, err
	}

	res.persons = persons
	if len(persons) == 0 {
		return &res, nil
	}

	persIds := make([]string, len(persons))
	for i, p := range persons {
		persIds[i] = p.Id
	}

	faces, err := ctx.FindFaces(&model.FacesQuery{PersonIds: persIds})
	res.faces = faces
	return &res, err
}
