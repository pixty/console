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
	return []*common.Camera{&common.Camera{Id: "12345", OrgId: OrgId, Name: "Lisa"}}
}
