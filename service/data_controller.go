package service

import (
	"errors"
	"strconv"

	"github.com/jrivets/log4g"
	"github.com/pixty/console/common"
	"github.com/pixty/console/model"
)

type (
	DataController interface {
		// Orgs and fields
		InsertOrg(org *model.Organization) (int64, error)
		GetOrgAndFields(orgId int64) (*model.Organization, []*model.FieldInfo, error)
		InsertNewFields(orgId int64, fis []*model.FieldInfo) error
		GetFieldInfos(orgId int64) ([]*model.FieldInfo, error)
		UpdateFieldInfo(fi *model.FieldInfo) error
		DeleteFieldInfo(orgId, fldId int64) error

		// Profiles
		InsertProfile(prf *model.Profile) (int64, error)
		GetProfile(prfId int64) (*model.Profile, error)
	}

	dta_controller struct {
		Persister model.Persister `inject:"persister"`
		logger    log4g.Logger
	}
)

const (
	cOrgMaxFieldsCount = 20
)

func NewDataController() DataController {
	dc := new(dta_controller)
	dc.logger = log4g.GetLogger("pixty.DataController")
	return dc
}

func (dc *dta_controller) InsertOrg(org *model.Organization) (int64, error) {
	mmp, err := dc.Persister.GetMainTx()
	if err != nil {
		return -1, err
	}
	return mmp.InsertOrg(org)
}

func (dc *dta_controller) GetOrgAndFields(orgId int64) (*model.Organization, []*model.FieldInfo, error) {
	mmp, err := dc.Persister.GetMainTx()
	if err != nil {
		return nil, nil, err
	}

	org, err := mmp.GetOrg(orgId)
	if err != nil {
		return nil, nil, err
	}
	mpp, err := dc.Persister.GetPartitionTx("FAKE")
	if err != nil {
		return nil, nil, err
	}
	fis, err := mpp.GetFieldInfos(orgId)
	return org, fis, err
}

func (dc *dta_controller) InsertNewFields(orgId int64, fis []*model.FieldInfo) error {
	err := checkFieldInfos(fis, orgId)
	if err != nil {
		return err
	}

	mpp, err := dc.Persister.GetPartitionTx("FAKE")
	if err != nil {
		return err
	}
	efis, err := mpp.GetFieldInfos(orgId)
	if err != nil {
		return err
	}

	// Not atomic check, but it is ok so far...
	newCount := len(efis) + len(fis)
	if newCount > cOrgMaxFieldsCount {
		return errors.New("Your organization can have up to " + strconv.Itoa(cOrgMaxFieldsCount) +
			" fields, but it is going to be " + strconv.Itoa(newCount))
	}

	return mpp.InsertFieldInfos(fis)
}

func (dc *dta_controller) GetFieldInfos(orgId int64) ([]*model.FieldInfo, error) {
	mpp, err := dc.Persister.GetPartitionTx("FAKE")
	if err != nil {
		return nil, err
	}
	return mpp.GetFieldInfos(orgId)
}

func (dc *dta_controller) UpdateFieldInfo(fi *model.FieldInfo) error {
	mpp, err := dc.Persister.GetPartitionTx("FAKE")
	if err != nil {
		return err
	}
	efi, err := mpp.GetFieldInfo(fi.Id)
	if err != nil {
		return err
	}

	if efi.OrgId != fi.OrgId {
		return errors.New("No field Id=" +
			strconv.FormatInt(fi.Id, 10) + " in the organization")
	}

	efi.DisplayName = fi.DisplayName
	return mpp.UpdateFiledInfo(efi)
}

func (dc *dta_controller) DeleteFieldInfo(orgId, fldId int64) error {
	mpp, err := dc.Persister.GetPartitionTx("FAKE")
	if err != nil {
		return err
	}
	efi, err := mpp.GetFieldInfo(fldId)
	if err != nil {
		return err
	}

	if efi.OrgId != orgId {
		return errors.New("No field Id=" +
			strconv.FormatInt(efi.Id, 10) + " in the organization")
	}
	return mpp.DeleteFieldInfo(efi)
}

func (dc *dta_controller) InsertProfile(prf *model.Profile) (int64, error) {
	mpp, err := dc.Persister.GetPartitionTx("FAKE")
	if err != nil {
		return -1, err
	}

	err = mpp.Begin()
	if err != nil {
		return -1, err
	}
	defer mpp.Commit()

	pid, err := mpp.InsertProfile(prf)
	if err != nil {
		mpp.Rollback()
		return -1, err
	}

	if prf.Meta != nil {
		for _, pm := range prf.Meta {
			pm.ProfileId = pid
		}
		err = mpp.InsertProfleMetas(prf.Meta)
		if err != nil {
			mpp.Rollback()
			return -1, err
		}
	}
	return pid, nil
}

func (dc *dta_controller) GetProfile(prfId int64) (*model.Profile, error) {
	mpp, err := dc.Persister.GetPartitionTx("FAKE")
	if err != nil {
		return nil, err
	}

	prfs, err := mpp.GetProfiles(&model.ProfileQuery{ProfileIds: []int64{prfId}})
	if err != nil {
		return nil, err
	}

	if prfs == nil || len(prfs) == 0 {
		dc.logger.Debug("No profiles found by id=", prfId)
		return nil, common.NewError(common.ERR_NOT_FOUND, "Could not find profile by id="+strconv.FormatInt(prfId, 10))
	}

	return prfs[0], nil
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
