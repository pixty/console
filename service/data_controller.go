package service

import (
	"errors"
	"regexp"
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

		// Users

		//Cameras
		GetCameraById(camId string) (*model.Camera, error)
		GetAllCameras(orgId int64) ([]*model.Camera, error)
		NewCamera(mcam *model.Camera) error
		NewCameraKey(camId string, orgId int64) (*model.Camera, string, error)

		// Profiles
		InsertProfile(prf *model.Profile) (int64, error)
		UpdateProfile(prf *model.Profile) error
		DeleteProfile(prfId, orgId int64) error
		GetProfile(prfId int64) (*model.Profile, error)

		// Persons
		DescribePerson(pId string, orgId int64, includeDetails, includeMeta bool) (*PersonDesc, error)
		UpdatePerson(mp *model.Person, orgId int64) error
	}

	PersonDesc struct {
		Person *model.Person
		// Faces of the person
		Faces []*model.Face
		// Profiles that meet in the Profile and match groups all together
		Profiles map[int64]*model.Profile
	}

	dta_controller struct {
		Persister   model.Persister    `inject:"persister"`
		BlobStorage common.BlobStorage `inject:"blobStorage"`
		logger      log4g.Logger
	}
)

const (
	cOrgMaxFieldsCount = 20
)

var camIdRegexp = regexp.MustCompile(`^[a-zA-Z]{1}([0-9a-zA-Z-_]+){2,39}$`)

func NewDataController() DataController {
	dc := new(dta_controller)
	dc.logger = log4g.GetLogger("pixty.DataController")
	return dc
}

func isCamIdValid(camId string) error {
	if !camIdRegexp.MatchString(camId) {
		return common.NewError(common.ERR_INVALID_VAL, "Invalid cameraId="+camId+", expected string up to 40 chars length.")
	}
	return nil
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
	err = mpp.Begin()
	if err != nil {
		return err
	}
	defer mpp.Commit()

	efis, err := mpp.GetFieldInfos(orgId)
	if err != nil {
		return err
	}

	newCount := len(efis) + len(fis)
	if newCount > cOrgMaxFieldsCount {
		return errors.New("Your organization can have up to " + strconv.Itoa(cOrgMaxFieldsCount) +
			" fields, but it is going to be " + strconv.Itoa(newCount))
	}

	// Basic checks
	for _, fi := range fis {
		if fi.DisplayName == "" {
			return errors.New("Field display name cannot be empty!")
		}
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

//Cameras
func (dc *dta_controller) GetCameraById(camId string) (*model.Camera, error) {
	err := isCamIdValid(camId)
	if err != nil {
		return nil, err
	}

	mpp, err := dc.Persister.GetPartitionTx("FAKE")
	if err != nil {
		return nil, err
	}

	return mpp.GetCameraById(camId)
}

func (dc *dta_controller) GetAllCameras(orgId int64) ([]*model.Camera, error) {
	mpp, err := dc.Persister.GetPartitionTx("FAKE")
	if err != nil {
		return nil, err
	}

	return mpp.FindCameras(&model.CameraQuery{OrgId: orgId})
}

func (dc *dta_controller) NewCamera(cam *model.Camera) error {
	err := isCamIdValid(cam.Id)
	if err != nil {
		return err
	}

	mpp, err := dc.Persister.GetPartitionTx("FAKE")
	if err != nil {
		return err
	}

	return mpp.InsertCamera(cam)

}

func (dc *dta_controller) NewCameraKey(camId string, orgId int64) (*model.Camera, string, error) {
	mpp, err := dc.Persister.GetPartitionTx("FAKE")
	if err != nil {
		return nil, "", err
	}

	cam, err := mpp.GetCameraById(camId)
	if err != nil {
		return nil, "", err
	}

	if cam.OrgId != orgId {
		dc.logger.Warn("The camera camId=", camId, " not in the target orgId=", orgId, " reports not found...")
		return nil, "", common.NewError(common.ERR_NOT_FOUND, "No camera with id="+camId)
	}

	sk := common.NewSecretKey(8)
	hash := common.Hash(sk)
	cam.SecretKey = hash
	err = mpp.UpdateCamera(cam)
	if err != nil {
		return nil, "", err
	}
	return cam, sk, nil
}

func (dc *dta_controller) InsertProfile(prf *model.Profile) (int64, error) {
	mpp, err := dc.Persister.GetPartitionTx("FAKE")
	if err != nil {
		return -1, err
	}

	if prf.PictureId != "" && dc.BlobStorage.ReadMeta(common.Id(prf.PictureId)) == nil {
		dc.logger.Warn("Inserting profile with unknown pictureId=", prf.PictureId)
		return -1, errors.New("There is no picture with id=" + prf.PictureId)
	}

	err = mpp.Begin()
	if err != nil {
		return -1, err
	}
	defer mpp.Commit()

	pid, err := mpp.InsertProfile(prf)
	if err != nil {
		return -1, err
	}
	prf.Id = pid

	err = dc.insertProfileMeta(prf, mpp)
	return pid, err
}

func (dc *dta_controller) insertProfileMeta(prf *model.Profile, mpp model.PartTx) error {
	if prf.Meta != nil {
		valid := prf.Meta[:0]
		for _, pm := range prf.Meta {
			if pm.Value == "" {
				continue
			}
			valid = append(valid, pm)
			pm.ProfileId = prf.Id
		}
		err := mpp.InsertProfleMetas(valid)
		if err != nil {
			mpp.Rollback()
			return err
		}
	}
	return nil
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

func (dc *dta_controller) UpdateProfile(prf *model.Profile) error {
	mpp, err := dc.Persister.GetPartitionTx("FAKE")
	if err != nil {
		return err
	}
	err = mpp.Begin()
	if err != nil {
		return err
	}
	defer mpp.Commit()

	prfs, err := mpp.GetProfiles(&model.ProfileQuery{ProfileIds: []int64{prf.Id}})
	if err != nil {
		return err
	}

	if prfs == nil || len(prfs) == 0 || prfs[0].OrgId != prf.OrgId {
		dc.logger.Debug("No profiles found by id=", prf.Id, " for the orgId=", prf.OrgId, ", query result was ", prfs)
		return common.NewError(common.ERR_NOT_FOUND, "Could not find profile by id="+strconv.FormatInt(prf.Id, 10))
	}

	// Delete all profile metas and keep eye on rollbacking the transaction in case of error
	err = mpp.DeleteAllProfileMetas(prf.Id)
	if err != nil {
		mpp.Rollback()
		return err
	}

	err = mpp.UpdateProfile(prf)
	if err != nil {
		mpp.Rollback()
		return err
	}

	return dc.insertProfileMeta(prf, mpp)
}

func (dc *dta_controller) DeleteProfile(prfId, orgId int64) error {
	mpp, err := dc.Persister.GetPartitionTx("FAKE")
	if err != nil {
		return err
	}
	err = mpp.Begin()
	if err != nil {
		return err
	}
	defer mpp.Commit()

	prfs, err := mpp.GetProfiles(&model.ProfileQuery{ProfileIds: []int64{prfId}})
	if err != nil {
		return err
	}

	if prfs == nil || len(prfs) == 0 || (prfs[0].OrgId != orgId && orgId > 0) {
		dc.logger.Debug("No profiles found by id=", prfId, " for the orgId=", orgId, ", query result was ", prfs)
		return common.NewError(common.ERR_NOT_FOUND, "Could not find profile by id="+strconv.FormatInt(prfId, 10))
	}

	// in case of error we will commit the transaction either. It's ok
	return mpp.DeleteProfile(prfId)
}

// Builds a person description, can return just person object or completed one with
// full list of profiles and faces.
// Flags:
// includeDetails: - whether description will include faces and profiles (true), or not (false)
// includeFields: - whether to include profiles meta data (true), or not (false).
func (dc *dta_controller) DescribePerson(pId string, orgId int64, includeDetails, includeMeta bool) (*PersonDesc, error) {
	pp, err := dc.Persister.GetPartitionTx("FAKE")
	if err != nil {
		return nil, err
	}
	//transaction
	err = pp.Begin()
	if err != nil {
		return nil, err
	}
	defer pp.Commit()

	inOrg, err := pp.CheckPersonInOrg(pId, orgId)
	if err != nil {
		return nil, err
	}
	if !inOrg {
		return nil, common.NewError(common.ERR_NOT_FOUND, "Could not find person by id="+pId)
	}

	person, err := pp.GetPersonById(pId)
	if err != nil {
		return nil, err
	}

	if !includeDetails {
		return &PersonDesc{Person: person}, nil
	}

	// collecting faces
	faces, err := pp.FindFaces(&model.FacesQuery{PersonIds: []string{pId}, Short: true})
	if err != nil {
		return nil, err
	}

	prfArr := []int64{}
	if person.MatchGroup > 0 {
		prof2MGs, err := pp.GetProfilesByMGs([]int64{person.MatchGroup})
		if err != nil {
			return nil, err
		}

		for pid, _ := range prof2MGs {
			prfArr = append(prfArr, pid)
		}
	}
	if person.ProfileId > 0 {
		prfArr = append(prfArr, person.ProfileId)
	}

	profs, err := pp.GetProfiles(&model.ProfileQuery{ProfileIds: prfArr, NoMeta: !includeMeta})
	if err != nil {
		return nil, err
	}
	profiles := make(map[int64]*model.Profile)
	for _, p := range profs {
		profiles[p.Id] = p
	}
	res := new(PersonDesc)
	res.Faces = faces
	res.Person = person
	res.Profiles = profiles
	return res, nil
}

func (dc *dta_controller) UpdatePerson(mp *model.Person, orgId int64) error {
	pp, err := dc.Persister.GetPartitionTx("FAKE")
	if err != nil {
		return err
	}
	//transaction
	err = pp.Begin()
	if err != nil {
		return err
	}
	defer pp.Commit()

	inOrg, err := pp.CheckPersonInOrg(mp.Id, orgId)
	if err != nil {
		return err
	}
	if !inOrg {
		return common.NewError(common.ERR_NOT_FOUND, "Could not find person by id="+mp.Id)
	}

	if mp.ProfileId > 0 {
		inOrg, err := pp.CheckProfileInOrg(mp.ProfileId, orgId)
		if err != nil {
			return err
		}
		if !inOrg {
			dc.logger.Warn("UpdatePerson(): the profile=", mp.ProfileId, ", seems to be not in the orgId=", orgId)
			return errors.New("There is not profile with id=" + strconv.FormatInt(mp.ProfileId, 10))
		}
	}

	// Check imageId
	if mp.PictureId != "" && dc.BlobStorage.ReadMeta(common.Id(mp.PictureId)) == nil {
		dc.logger.Warn("UpdatePerson(): Unknown pictureId=", mp.PictureId)
		return errors.New("There is no picture with id=" + mp.PictureId)
	}
	p, err := pp.GetPersonById(mp.Id)
	if err != nil {
		return err
	}

	p.PictureId = mp.PictureId
	p.ProfileId = mp.ProfileId
	return pp.UpdatePerson(p)
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
