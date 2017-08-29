package service

import (
	"errors"
	"strconv"

	"github.com/pixty/console/common"
	"github.com/pixty/console/model"
)

type (
	DataController interface {
		InsertOrg(org *model.Organization) (int64, error)
		GetOrgAndFields(orgId int64) (*model.Organization, []*model.FieldInfo, error)
		InsertNewFields(orgId int64, fis []*model.FieldInfo) error
		GetFieldInfos(orgId int64) ([]*model.FieldInfo, error)
		UpdateFieldInfo(fi *model.FieldInfo) error
		DeleteFieldInfo(orgId, fldId int64) error
	}

	dta_controller struct {
		Persister model.Persister `inject:"persister"`
	}
)

const (
	cOrgMaxFieldsCount = 20
)

func NewDataController() DataController {
	dc := new(dta_controller)
	return dc
}

func (dc *dta_controller) InsertOrg(org *model.Organization) (int64, error) {
	mmp := dc.Persister.GetMainPersister()
	return mmp.InsertOrg(org)
}

func (dc *dta_controller) GetOrgAndFields(orgId int64) (*model.Organization, []*model.FieldInfo, error) {
	mmp := dc.Persister.GetMainPersister()
	org, err := mmp.GetOrg(orgId)
	if err != nil {
		return nil, nil, err
	}
	fis, err := mmp.GetFieldInfos(orgId)
	return org, fis, err
}

func (dc *dta_controller) InsertNewFields(orgId int64, fis []*model.FieldInfo) error {
	err := checkFieldInfos(fis, orgId)
	if err != nil {
		return err
	}

	mmp := dc.Persister.GetMainPersister()
	efis, err := mmp.GetFieldInfos(orgId)
	if err != nil {
		return err
	}

	// Not atomic check, but it is ok so far...
	newCount := len(efis) + len(fis)
	if newCount > cOrgMaxFieldsCount {
		return errors.New("Your organization can have up to " + strconv.Itoa(cOrgMaxFieldsCount) +
			" fields, but it is going to be " + strconv.Itoa(newCount))
	}

	return mmp.InsertFieldInfos(fis)
}

func (dc *dta_controller) GetFieldInfos(orgId int64) ([]*model.FieldInfo, error) {
	mmp := dc.Persister.GetMainPersister()
	return mmp.GetFieldInfos(orgId)
}

func (dc *dta_controller) UpdateFieldInfo(fi *model.FieldInfo) error {
	mmp := dc.Persister.GetMainPersister()
	efi, err := mmp.GetFieldInfo(fi.Id)
	if err != nil {
		return err
	}

	if efi.OrgId != fi.OrgId {
		return errors.New("No field Id=" +
			strconv.FormatInt(fi.Id, 10) + " in the organization")
	}

	efi.DisplayName = fi.DisplayName
	return mmp.UpdateFiledInfo(efi)
}

func (dc *dta_controller) DeleteFieldInfo(orgId, fldId int64) error {
	mmp := dc.Persister.GetMainPersister()
	efi, err := mmp.GetFieldInfo(fldId)
	if err != nil {
		return err
	}

	if efi.OrgId != orgId {
		return errors.New("No field Id=" +
			strconv.FormatInt(efi.Id, 10) + " in the organization")
	}
	return mmp.DeleteFieldInfo(efi)
}

func checkFieldInfo(fi *model.FieldInfo, orgId int64) error {
	if fi.FieldType != "text" {
		return errors.New("Unknown fieldType=" + fi.FieldType + " expected: text")
	}
	if fi.OrgId != orgId {
		return common.NewError(common.ERR_NOT_FOUND, "Unproperly formed DO object: orgId="+strconv.FormatInt(fi.OrgId, 10)+
			", but expected one is "+strconv.FormatInt(orgId, 10))
	}
	return nil
}

func checkFieldInfos(fis []*model.FieldInfo, orgId int64) error {
	for _, fi := range fis {
		err := checkFieldInfo(fi, orgId)
		if err != nil {
			return err
		}
	}
	return nil
}
