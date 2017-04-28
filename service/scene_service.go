package service

import (
	"bytes"

	"github.com/jrivets/log4g"
	"github.com/pixty/console/common"
	"github.com/pixty/fpcp"
	"golang.org/x/net/context"
)

type (
	SceneService struct {
		fpcpSp      *fpcp.HttpSceneProcessor
		Persister   common.Persister    `inject:"persister"`
		ImgService  common.ImageService `inject:"imgService"`
		txPersister common.TxPersister
		scenesCache map[string]*common.Scene
		ctx         context.Context
		logger      log4g.Logger
	}
)

func NewSceneService() *SceneService {
	ss := new(SceneService)
	ss.fpcpSp = fpcp.NewHttpSceneProcessor(log4g.GetLogger("pixty.fpcp.sp"), 10)
	ss.logger = log4g.GetLogger("pixty.service.scene")
	ss.fpcpSp.RespListener(ss.onFPCPResponse)
	// So far so good for this...
	ss.ctx = context.Background()
	ss.scenesCache = make(map[string]*common.Scene)
	ss.logger.Info("New instance created.")
	return ss
}

// ============================= LifeCycler ==================================
func (ss *SceneService) DiPhase() int {
	return common.CMP_PHASE_SCENE_SERVICE
}

func (ss *SceneService) DiInit() error {
	ss.txPersister = ss.Persister.NewTxPersister(ss.ctx)
	return nil
}

func (ss *SceneService) DiShutdown() {
}

// FPCP SP response listener
func (ss *SceneService) onFPCPResponse(fpId string, resp *fpcp.Resp) {
	if resp.Scene == nil {
		ss.logger.Warn("Oops got a response, but there is no scene there. Ignore it.")
		return
	}

	ss.saveImage(fpId, resp)
	ss.updateScenePersons(fpId, resp.Scene)
	ss.newScene(fpId, resp)
}

func (ss *SceneService) GetHttpSceneProcessor() *fpcp.HttpSceneProcessor {
	return ss.fpcpSp
}

func (ss *SceneService) GetScenes(ctx common.CtxHolder, q *common.SceneQuery) ([]*common.Scene, error) {
	return ss.txPersister.GetScenes(q)
}

func (ss *SceneService) newScene(fpId string, resp *fpcp.Resp) *common.Scene {
	fpcpSc := resp.Scene
	sc, ok := ss.scenesCache[fpId]
	if ok && sc.Timestamp == common.Timestamp(fpcpSc.Timestamp) {
		// ok, we have cached one
		return sc
	}

	sc = nil
	scs, err := ss.txPersister.GetScenes(&common.SceneQuery{CamId: common.Id(fpId), Limit: 1})
	if err == nil && len(scs) > 0 {
		sc = scs[0]
		if sc.Timestamp == common.Timestamp(fpcpSc.Timestamp) {
			ss.logger.Debug("Found scene in DB, but not in cache. caching it... ", sc)
			ss.scenesCache[fpId] = sc
			return sc
		}
	}

	sc = new(common.Scene)
	sc.Id = common.NewId()
	sc.Timestamp = common.Timestamp(fpcpSc.Timestamp)
	sc.CamId = common.Id(fpId)
	sc.PersonsIds = make([]common.Id, len(fpcpSc.Persons), len(fpcpSc.Persons))
	for i, p := range fpcpSc.Persons {
		sc.PersonsIds[i] = common.Id(p.Id)
	}
	sc.ImgId = common.Id(fpcpSc.ImageId)

	sc_col := ss.txPersister.GetCrudExecutor(common.STGE_SCENE)
	sc_col.Create(sc)
	ss.scenesCache[fpId] = sc
	return sc
}

// Creates persons if they are not there yet
func (ss *SceneService) updateScenePersons(fpId string, fpcpSc *fpcp.Scene) {
	pers_count := len(fpcpSc.Persons)
	if pers_count == 0 {
		return
	}

	pids := make([]common.Id, pers_count, pers_count)
	pm := make(map[common.Id]*fpcp.Person)
	for i, p := range fpcpSc.Persons {
		pid := common.Id(p.Id)
		pids[i] = pid
		pm[pid] = p
	}

	prsns, err := ss.txPersister.FindPersons(&common.PersonsQuery{PersonIds: pids})
	if err != nil {
		ss.logger.Error("Ooops, could not find persons by ids=", pids, ", err=", err)
		return
	}

	pp := ss.txPersister.GetCrudExecutor(common.STGE_PERSON)
	// update existing persons
	for _, p := range prsns {
		if updatePersonImgs(pm[p.Id], p) {
			ss.logger.Debug("Updating person due to images update: ", p)
			err := pp.Update(p.Id, p)
			if err != nil {
				ss.logger.Error("Could not update person, err=", err)
			}
		}
		delete(pm, p.Id)
	}

	if len(pm) == 0 {
		// no new persons
		return
	}

	persons := make([]interface{}, len(pm), len(pm))
	idx := 0
	// Create new persons
	for _, p := range pm {
		person := new(common.Person)
		person.Id = common.Id(p.Id)
		person.CamId = common.Id(fpId)
		person.Faces = []common.FacePic{}
		person.LostAt = common.Timestamp(p.LostAt)
		person.SeenAt = common.Timestamp(p.FirstSeenAt)
		updatePersonImgs(p, person)
		persons[idx] = person
		idx++
	}

	ss.logger.Debug("Creating ", len(persons), " new scene persons")
	pp.CreateMany(persons...)
}

func (ss *SceneService) saveImage(fpId string, resp *fpcp.Resp) {
	img := resp.Image
	if img == nil {
		return
	}

	id := &common.ImageDescriptor{
		Id:        common.Id(img.Id),
		Reader:    bytes.NewReader(img.Data),
		CamId:     common.Id(fpId),
		Width:     img.Region.R + 1,
		Height:    img.Region.B + 1,
		Timestamp: common.Timestamp(img.Timestamp),
	}

	ss.logger.Debug()
	ss.ImgService.New(id)
}

func updatePersonImgs(fpP *fpcp.Person, p *common.Person) bool {
	imgs := make(map[string]*fpcp.Face)
	for _, f := range fpP.Faces {
		imgs[f.ImgId] = f
	}

	for _, img := range p.Faces {
		delete(imgs, string(img.ImageId))
	}

	if len(imgs) == 0 {
		return false
	}

	newImgs := make([]common.FacePic, len(imgs), len(imgs))
	i := 0
	for _, pic := range imgs {
		newImgs[i] = common.FacePic{
			ImageId: common.Id(pic.ImgId),
			Rect:    toRectangle(&pic.Region),
		}
		i++
	}
	p.Faces = append(p.Faces, newImgs...)
	if len(p.Faces) > 15 {
		p.Faces = p.Faces[len(p.Faces)-15:]
	}
	return true
}

func toRectangle(r *fpcp.Rect) common.Rectangle {
	return common.Rectangle{
		LeftTop:     common.Point{r.L, r.T},
		RightBottom: common.Point{r.R, r.B},
	}
}
