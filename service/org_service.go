package service

import (
	"github.com/pixty/console/model"
)

type (
	Org struct {
		Org    *model.Organization
		Fields []*model.FieldInfo
	}

	OrgServcie interface {
		Create(org *Org) error
		Get(orgId int64) (*Org, error)
		Update(org *Org, okDeleteFields bool) error
	}

	org_service struct {
		Persister model.Persister `inject:"persister"`
	}
)

func NewOrgService() OrgServcie {
	os := new(org_service)
	return os
}

func (os *org_service) Create(org *Org) error {
	return nil
}

func (os *org_service) Get(orgId int64) (*Org, error) {
	return nil, nil
}

func (os *org_service) Update(org *Org, okDeleteFields bool) error {
	return nil
}
