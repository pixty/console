package service

import "github.com/pixty/console/common"
import "github.com/jrivets/log4g"

type DefaultOrgService struct {
	logger log4g.Logger
}

func NewDefaultOrgService() *DefaultOrgService {
	return &DefaultOrgService{log4g.GetLogger("console.service.orgService")}
}

func (orgS *DefaultOrgService) GetById(OrgId common.Id) *common.Organization {
	return nil
}

func (orgS *DefaultOrgService) GetPersons(OrgId common.Id, PIds ...common.Id) []*common.PersonOrgInfo {
	return nil
}
