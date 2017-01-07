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
	pl, err := txP.GetLatestScene(camId)
	if err != nil {
		camS.logger.Error("Oops, something goes wrong when we try to get the scene: camId=", camId, ", err=", err)
	}
	return pl
}

func (camS *DefaultCameraService) UpdateScene(ctx *common.Context, plogs []*common.PersonLog) {
	txP := ctx.TxPersister()
	dao := txP.GetCrudExecutor(common.STGE_PERSON_LOG)
	for pl := range plogs {
		dao.Create(pl)
	}
}
