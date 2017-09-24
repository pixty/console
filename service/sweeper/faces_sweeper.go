package sweeper

import (
	"fmt"
	"time"

	"github.com/jrivets/gorivets"
	"github.com/jrivets/log4g"
	"github.com/pixty/console/common"
	"github.com/pixty/console/model"
	"golang.org/x/net/context"
)

type (
	FacesSweeper interface {
	}

	faces_sweeper struct {
		CConfig   *common.ConsoleConfig `inject:""`
		Persister model.Persister       `inject:"persister"`
		MainCtx   context.Context       `inject:"mainCtx"`
		stats     *faces_swpr_stats
		logger    log4g.Logger
	}

	faces_swpr_stats struct {
		startedAt    time.Time
		from         common.Timestamp
		dbTrans      int
		persns       int
		facesDeleted int
		err          error
	}
)

// Used to be constructed and passed to dependency injection for further initialization
func NewFacesSweeper() FacesSweeper {
	return new(faces_sweeper)
}

// ========================== PostConstructor ================================
func (fs *faces_sweeper) DiPostConstruct() {
	fs.logger = log4g.GetLogger("pixty.FacesSweeper")
	fs.logger.Info("Post construct.")
	fs.stats = &faces_swpr_stats{}

	go func() {
		fs.logger.Info("Entering job routine.")
		for {
			select {
			case <-fs.MainCtx.Done():
				fs.logger.Info("Leaving job routine.")
				return
			// kick the sweeping routine
			case <-time.After(time.Second * time.Duration(fs.CConfig.SweepFacesToSec)):
				err := gorivets.CheckPanic(fs.sweepFaces)
				if err != nil {
					fs.logger.Error("Got the panic in sweeping faces: ", err)
				}
			}
		}
	}()
}

func (fs *faces_sweeper) sweepFaces() {
	ptx, err := fs.Persister.GetPartitionTx("FAKE")
	if err != nil {
		fs.logger.Warn("Could not obtain persister. err=", err)
		return
	}

	fs.stats.start()

	for fs.sweepFacesTx(ptx) {
	}
	fs.logger.Info("Done with sweep faces, stats=\"", fs.stats, "\"")
}

func (fs *faces_sweeper) sweepFacesTx(ptx model.PartTx) bool {
	err := ptx.Begin()
	if err != nil {
		fs.logger.Warn("Could not start transaction err=", err)
		fs.stats.onError(err)
		return false
	}
	defer ptx.Commit()

	fs.stats.onDbTrans()
	persns, err := ptx.FindPersons(&model.PersonsQuery{MinCreatedAt: &fs.stats.from, MinFacesCount: common.MAX_FACES_PER_PERSON + 1, Order: model.PQO_CREATED_AT_ASC, Limit: 100})
	if err != nil {
		fs.logger.Warn("Could not read persons err=", err)
		fs.stats.onError(err)
		return false
	}

	if len(persns) == 0 {
		// Over
		return false
	}

	fs.stats.onFound(len(persns))
	fs.stats.from = common.Timestamp(persns[len(persns)-1].CreatedAt)
	pids := make([]string, len(persns))
	for i, p := range persns {
		pids[i] = p.Id
	}

	faces, err := ptx.FindFaces(&model.FacesQuery{Short: true, PersonIds: pids, Limit: 10000})
	if err != nil {
		fs.logger.Warn("Could not read faces err=", err)
		fs.stats.onError(err)
		return false
	}

	// sort the faces by person Id
	p2fs := make(map[string][]*model.Face)
	for _, f := range faces {
		s, ok := p2fs[f.PersonId]
		if !ok {
			s = make([]*model.Face, 0, common.MAX_FACES_PER_PERSON+1)
		}
		s = append(s, f)
		p2fs[f.PersonId] = s
	}

	// Build list of faces to be deleted
	delIds := make([]int64, 0, 100)
	for _, fcs := range p2fs {
		fcsCnt := len(fcs)
		toDel := fcsCnt - common.MAX_FACES_PER_PERSON
		step := (float32(fcsCnt) - 2.0) / float32(toDel)
		startIdx := fcsCnt - 2
		for i := 0; i < toDel; i++ {
			idx := int(float32(startIdx) - float32(i)*step)
			delIds = append(delIds, fcs[idx].Id)
		}
		if len(delIds) > 100 {
			err = ptx.DeleteFaces(delIds)
			if err != nil {
				fs.logger.Warn("Could not delete faces err=", err)
				ptx.Rollback()
				fs.stats.onError(err)
				return false
			}
			fs.stats.onDelete(len(delIds))
			delIds = delIds[:0]
		}
	}

	err = ptx.DeleteFaces(delIds)
	if err != nil {
		fs.logger.Warn("Could not delete faces err=", err)
		ptx.Rollback()
		fs.stats.onError(err)
		return false
	}
	fs.stats.onDelete(len(delIds))
	return true
}

func (s *faces_swpr_stats) start() {
	s.startedAt = time.Now()
	s.persns = 0
	s.facesDeleted = 0
	s.from = common.Timestamp(0)
	s.dbTrans = 0
	s.err = nil
}

func (s *faces_swpr_stats) onFound(persns int) {
	s.persns += persns
}

func (s *faces_swpr_stats) onDelete(faces int) {
	s.facesDeleted += faces
}

func (s *faces_swpr_stats) onDbTrans() {
	s.dbTrans++
}

func (s *faces_swpr_stats) onError(err error) {
	s.err = err
}

func (s *faces_swpr_stats) String() string {
	return fmt.Sprint(s.persns, " persons found, ", s.facesDeleted, " faces deleted, and it took ",
		time.Now().Sub(s.startedAt), " for ", s.dbTrans, " db transactions")
}
