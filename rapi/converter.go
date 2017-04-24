package rapi

import (
	"errors"

	"gopkg.in/mgo.v2/bson"

	"github.com/jrivets/gorivets"
	"github.com/pixty/console/common"
	"github.com/pixty/fpcp"
)

type (
	RequestCtx struct {
		a      *api
		ch     *common.CtxHolder
		logger gorivets.Logger
		orgId  common.Id
	}
)

func newRequestCtx(a *api, ch *common.CtxHolder) *RequestCtx {
	res := new(RequestCtx)
	res.a = a
	res.ch = ch
	res.logger = a.logger
	return res
}

func (rc *RequestCtx) getTxPersister() common.TxPersister {
	return rc.ch.TxPersister()
}

func (rc *RequestCtx) getScenes(query *common.SceneQuery) ([]*Scene, error) {
	rc.logger.Debug("Get scenes for query=", query)
	txPersister := rc.getTxPersister()

	scenes, err := txPersister.GetScenes(query)
	if err != nil {
		rc.logger.Error("Could not get scenes by query=", query, ", err=", err)
		return []*Scene{}, err
	}

	return rc.toScenes(scenes), nil
}

func (rc *RequestCtx) toScenes(scenes []*common.Scene) []*Scene {
	if scenes == nil || len(scenes) == 0 {
		return []*Scene{}
	}

	res := make([]*Scene, len(scenes))
	for i, sc := range scenes {
		res[i] = rc.toScene(sc)
	}
	return res
}

func (rc *RequestCtx) toScene(scene *common.Scene) *Scene {
	res := new(Scene)
	res.CamId = scene.CamId
	res.Timestamp = scene.Timestamp.ToISO8601Time()
	res.Persons, _ = rc.getPersonsByQuery(&common.PersonsQuery{PersonIds: scene.PersonsIds})
	return res
}

func (rc *RequestCtx) getPersonsByQuery(query *common.PersonsQuery) ([]*Person, error) {
	rc.logger.Debug("getPersonsByIds(): looking for persons by query=", query)
	txPersister := rc.getTxPersister()
	persns, err := txPersister.FindPersons(query)
	if err != nil {
		rc.logger.Error("getPersonsByIds(): err=", err)
		return []*Person{}, err
	}
	return rc.getPersons(persns), nil
}

func (rc *RequestCtx) getPersons(persons []*common.Person) []*Person {
	if persons == nil || len(persons) == 0 {
		return []*Person{}
	}

	res := make([]*Person, len(persons))
	for i, pn := range persons {
		res[i] = rc.toPerson(pn)
	}
	return res
}

func (rc *RequestCtx) toPerson(person *common.Person) *Person {
	res := &Person{}
	res.Id = person.Id
	res.CamId = person.CamId

	if person.Profile != nil {
		p := rc.getProfile(person.Profile.ProfileId)
		p.Occuracy = person.Profile.Occuracy
	}
	res.Matches = rc.getMatches(res.Id)
	res.Pictures = rc.getFaceInfos(person.Faces)
	res.CapturedAt = person.SeenAt.ToISO8601Time()
	res.LostAt = person.LostAt.ToISO8601Time()
	return res
}

func (rc *RequestCtx) getProfile(pid common.Id) *Profile {
	prfDAO := rc.getTxPersister().GetCrudExecutor(common.STGE_PROFILE)
	prf := &common.Profile{}
	prfDAO.Read(pid, prf)
	return rc.toProfile(prf)
}

func (rc *RequestCtx) toProfile(profile *common.Profile) *Profile {
	if profile == nil {
		return nil
	}

	res := new(Profile)
	res.Id = profile.Id
	res.OrgId = rc.orgId
	res.Attributes = profile.OrgInfo[rc.orgId].Metadata

	return res
}

func (rc *RequestCtx) getMatches(personId common.Id) []*Profile {
	rc.logger.Debug("getMatches(): personId=", personId)
	txPersister := rc.getTxPersister()
	matches, err := txPersister.GetMatches(personId)
	if err != nil {
		rc.logger.Error("getMatches(): err=", err)
		return []*Profile{}
	}

	return rc.toProfileMatches(matches)
}

func (rc *RequestCtx) getFaceInfos(faces []common.FacePic) []*PictureInfo {
	if faces == nil || len(faces) == 0 {
		return []*PictureInfo{}
	}

	res := make([]*PictureInfo, len(faces))
	for i, face := range faces {
		res[i] = rc.getFaceInfo(&face)
	}
	return res
}

func (rc *RequestCtx) getFaceInfo(face *common.FacePic) *PictureInfo {
	pi := new(PictureInfo)
	pi.Id = face.ImageId
	pi.Rect = &fpcp.Rect{
		L: face.Rect.LeftTop.X,
		T: face.Rect.LeftTop.Y,
		R: face.Rect.RightBottom.X,
		B: face.Rect.RightBottom.Y,
	}
	return pi
}

func (rc *RequestCtx) toProfileMatches(matches []*common.PersonMatch) []*Profile {
	if matches == nil || len(matches) == 0 {
		return []*Profile{}
	}
	res := make([]*Profile, len(matches))
	for i, mch := range matches {
		res[i] = rc.toProfileMatch(mch)
	}
	return res
}

func (rc *RequestCtx) toProfileMatch(match *common.PersonMatch) *Profile {
	res := rc.getProfile(match.ProfileId)
	res.Occuracy = match.Occuracy
	return res
}

func (rc *RequestCtx) associatePersonToProfile(person *Person, profileId common.Id) error {
	rc.logger.Info("Associating person=", person, " with profileId=", profileId)
	if person.Id == common.ID_NULL {
		rc.logger.Warn("person Id is not specified")
		return errors.New("Expecting non-null person Id")
	}

	prsnDAO := rc.getTxPersister().GetCrudExecutor(common.STGE_PERSON)
	p := &common.Person{}
	err := prsnDAO.Read(person.Id, p)
	if err != nil {
		rc.logger.Warn("personId=", person.Id, " is not found? err=", err)
		return errors.New("Person with Id=" + string(person.Id) + " is not found.")
	}

	prfDAO := rc.getTxPersister().GetCrudExecutor(common.STGE_PROFILE)
	prf := &common.Profile{}
	err = prfDAO.Read(profileId, prf)
	if err != nil {
		rc.logger.Warn("profileId=", profileId, " is not found? err=", err)
		return errors.New("Profile with Id=" + string(profileId) + " is not found.")
	}

	pm := &common.PersonMatch{ProfileId: profileId, PersonId: p.Id, Occuracy: 0}
	rc.logger.Info("Person Match ", pm, ", uses 0% uccuracy due to manual association.")

	p.Profile = pm
	err = prsnDAO.Update(p.Id, p)
	if err != nil {
		rc.logger.Warn("Could not update profile. err=", err)
		return err
	}
	return nil
}

// Creates new profile by profile request.
func (rc *RequestCtx) newProfile(profile *Profile) (common.Id, error) {
	rc.logger.Info("Creating new profile=", profile)

	id := common.Id(bson.NewObjectId().Hex())
	poi := common.ProfileOrgInfo{
		Metadata: profile.Attributes,
	}
	prf := &common.Profile{
		Id:      id,
		OrgInfo: map[common.Id]common.ProfileOrgInfo{rc.orgId: poi},
	}

	prfDAO := rc.getTxPersister().GetCrudExecutor(common.STGE_PROFILE)
	err := prfDAO.Create(prf)
	return id, err
}

func (rc *RequestCtx) getPictureInfo(picId common.Id) (*PictureInfo, error) {
	imgD := rc.a.ImgService.Read(picId, false)
	if imgD == nil {
		return nil, common.NewError(common.ERR_NOT_FOUND, "Picture with id="+string(picId))
	}

	pi := new(PictureInfo)
	pi.Id = imgD.Id
	pi.CamId = imgD.CamId
	pi.Size.H = imgD.Height
	pi.Size.W = imgD.Width
	pi.Timestamp = imgD.Timestamp.ToISO8601Time()
	return pi, nil
}
