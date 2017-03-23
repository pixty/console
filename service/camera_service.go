package service

import "github.com/pixty/console/common"
import "github.com/jrivets/log4g"

type DefaultCameraService struct {
	logger log4g.Logger
}

func NewDefaultCameraService() *DefaultCameraService {
	return &DefaultCameraService{log4g.GetLogger("console.service.cameraService")}
}

func (camS *DefaultCameraService) GetById(CamId common.Id) *common.Camera {
	//TODO: fix me
	return nil
}

func (camS *DefaultCameraService) GetByOrgId(OrgId common.Id) []*common.Camera {
	//TODO: fake so far
	return []*common.Camera{&common.Camera{IO: common.IO{"12345"}, OrgId: OrgId, Name: "Lisa"}}
}

func (camS *DefaultCameraService) GetScene(ctx *common.Context, camId common.Id) []*common.PersonLog {
	txP := ctx.TxPersister()
	var res []*common.PersonLog
	pl, err := txP.GetLatestScene(camId)
	if err == nil {
		// Scene can contain several snapshots for the same person. We don't want
		// to return all of them, but will provide only latest one. Latest snapshot
		// comes first and we can remove other ones.
		m := make(map[common.Id]*common.PersonLog)
		for _, p := range pl {
			_, ok := m[p.DetectionId]
			if !ok {
				m[p.DetectionId] = p
			}
		}

		camS.logger.Debug("Found ", len(pl), " records for last scene, and ", len(m), " unique persons")
		res = make([]*common.PersonLog, 0, len(m))
		for _, p := range m {
			res = append(res, p)
		}
	} else {
		camS.logger.Error("Oops, something goes wrong when we try to get the scene: camId=", camId, ", err=", err)
	}
	return res
}

func (camS *DefaultCameraService) UpdateScene(ctx *common.Context, plogs []*common.PersonLog) {
	camS.logger.Debug("Updating scene ", plogs)
	txP := ctx.TxPersister()
	dao := txP.GetCrudExecutor(common.STGE_PERSON_LOG)
	objs := make([]interface{}, 0, len(plogs))
	for _, p := range plogs {
		objs = append(objs, p)
	}
	dao.CreateMany(objs...)
}
