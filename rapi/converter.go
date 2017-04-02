package rapi

import (
	"errors"

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

	if person.ProfileId != common.ID_NULL {
		res.Profile = rc.getProfile(person.ProfileId)
	} else {
		res.Matches = rc.getMatches(res.Id)
	}
	res.Pictures = rc.getFaceInfos(person.Faces)
	res.CapturedAt = person.SeenAt.ToISO8601Time()
	res.LostAt = person.LostAt.ToISO8601Time()
	return res
}

func (rc *RequestCtx) getProfile(pid common.Id) *Profile {
	prfDAO := rc.getTxPersister().GetCrudExecutor(common.STGE_PROFILE)
	prf := prfDAO.Read(pid).(*common.Profile)
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

func (rc *RequestCtx) getMatches(personId common.Id) []*ProfileMatch {
	rc.logger.Debug("getMatches(): personId=", personId)
	txPersister := rc.getTxPersister()
	matches, err := txPersister.GetMatches(personId)
	if err != nil {
		rc.logger.Error("getMatches(): err=", err)
		return []*ProfileMatch{}
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

func (rc *RequestCtx) toProfileMatches(matches []*common.PersonMatch) []*ProfileMatch {
	if matches == nil || len(matches) == 0 {
		return []*ProfileMatch{}
	}
	res := make([]*ProfileMatch, len(matches))
	for i, mch := range matches {
		res[i] = rc.toProfileMatch(mch)
	}
	return res
}

func (rc *RequestCtx) toProfileMatch(match *common.PersonMatch) *ProfileMatch {
	res := new(ProfileMatch)
	res.Occuracy = match.Occuracy
	res.Profile = rc.getProfile(match.ProfileId)
	return res
}

func (rc *RequestCtx) associatePersonToProfile(person *Person, profileId common.Id) error {
	rc.logger.Info("Associating person=", person, " with profileId=", profileId)
	if person.Id == common.ID_NULL {
		rc.logger.Warn("person Id is not specified")
		return errors.New("Expecting non-null person Id")
	}

	prsnDAO := rc.getTxPersister().GetCrudExecutor(common.STGE_PERSON)
	p := prsnDAO.Read(person.Id).(*common.Person)
	if p == nil {
		rc.logger.Warn("personId=", person.Id, " is not found")
		return errors.New("Person with Id=" + string(person.Id) + " is not found.")
	}

	prfDAO := rc.getTxPersister().GetCrudExecutor(common.STGE_PROFILE)
	prf := prfDAO.Read(profileId).(*common.Profile)
	if prf == nil {
		rc.logger.Warn("profileId=", profileId, " is not found")
		return errors.New("Profile with Id=" + string(profileId) + " is not found.")
	}

	p.ProfileId = profileId
	err := prsnDAO.Update(p)
	if err != nil {
		rc.logger.Warn("Could not update profile. err=", err)
		return err
	}
	return nil
}
