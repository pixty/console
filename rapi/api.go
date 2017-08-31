package rapi

import (
	"errors"
	"io"
	"math"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/jrivets/gorivets"
	"github.com/jrivets/log4g"
	"github.com/pixty/console/common"
	"github.com/pixty/console/model"
	"github.com/pixty/console/service"
	"github.com/pixty/console/service/scene"
	"golang.org/x/net/context"
	"gopkg.in/gin-gonic/gin.v1"
	"gopkg.in/tylerb/graceful.v1"
)

type api struct {
	ge           *gin.Engine
	Config       *common.ConsoleConfig  `inject:""`
	ImgService   common.ImageService    `inject:"imgService"`
	ScnProcessor *scene.SceneProcessor  `inject:"scnProcessor"`
	MainCtx      context.Context        `inject:"mainCtx"`
	Dc           service.DataController `inject:""`
	logger       log4g.Logger
}

const (
	cScnPersonsMinLimit = 3
	cScnPersonsDefLimit = 20
	cScnPersonsMaxLimit = 50
)

func NewAPI() *api {
	return new(api)
}

// =========================== PostConstructor ===============================
func (a *api) DiPostConstruct() {
	a.logger = log4g.GetLogger("pixty.rest")
	a.logger.Info("HTTP Debug mode=", a.Config.HttpDebugMode)
	if !a.Config.HttpDebugMode {
		gin.SetMode(gin.ReleaseMode)
	}

	a.ge = gin.New()
	if a.Config.HttpDebugMode {
		a.logger.Info("Gin logger and gin.debug is enabled. You can set up DEBUG mode for the pixty.rest group to obtain requests dumps and more logs for the API group.")
		a.ge.Use(gin.Logger())
	}
	a.ge.Use(a.PrintRequest)

	a.logger.Info("Constructing ReST API")

	a.ge.GET("/ping", a.h_GET_ping)
	a.ge.GET("/cameras/:camId/timeline", a.h_GET_cameras_timeline)
	a.ge.GET("/images/:imgName", a.h_GET_images_png_download)
	a.ge.POST("/orgs", a.h_POST_orgs)
	a.ge.GET("/orgs/:orgId", a.h_GET_orgs_orgId)
	a.ge.POST("/orgs/:orgId/fields", a.h_POST_orgs_orgId_fields)
	a.ge.GET("/orgs/:orgId/fields", a.h_GET_orgs_orgId_fields)
	a.ge.PUT("/orgs/:orgId/fields/:fldId", a.h_PUT_orgs_orgId_fields_fldId)
	a.ge.DELETE("/orgs/:orgId/fields/:fldId", a.h_DELETE_orgs_orgId_fields_fldId)
	a.ge.POST("/profiles", a.h_POST_profiles)
	a.ge.GET("/profiles/:prfId", a.h_GET_profiles_prfId)
	a.ge.PUT("/profiles/:prfId", a.h_PUT_profiles_prfId)
	a.ge.DELETE("/profiles/:prfId", a.h_DELETE_profiles_prfId)
	a.ge.GET("/persons/:persId", a.h_GET_persons_persId)
	a.ge.PUT("/persons/:persId", a.h_PUT_persons_persId)
}

// =============================== Handlers ==================================
// GET /ping
func (a *api) h_GET_ping(c *gin.Context) {
	a.logger.Debug("GET /ping")
	c.String(http.StatusOK, "pong URL conversion is "+composeURI(c.Request, ""))
}

// Returns timeline object for the camera. The timeline object contains list
// of persons sorted in descending order. The timeline object also has reference
// to the last frame for the requested camera
// GET /cameras/:camId/timeline?limit=20&maxTime=12341234
func (a *api) h_GET_cameras_timeline(c *gin.Context) {
	camId := c.Param("camId")
	a.logger.Debug("GET /cameras/", camId, "/timeline")

	// Parse query
	q := c.Request.URL.Query()
	limit, err := parseInt64QueryParam("limit", q)
	if err != nil {
		limit = cScnPersonsDefLimit
		a.logger.Debug("h_GET_cameras_timeline: Limit is not provided or wrong, err=", err, " set it to ", cScnPersonsDefLimit)
	}

	if limit < cScnPersonsMinLimit {
		a.logger.Info("h_GET_cameras_timeline: limit=", limit, ", less than ", cScnPersonsMinLimit, ", set it to ", cScnPersonsMinLimit)
		limit = cScnPersonsMinLimit
	}

	if limit > cScnPersonsMaxLimit {
		a.logger.Info("h_GET_cameras_timeline: limit=", limit, ", greater than ", cScnPersonsMaxLimit, ", set it to ", cScnPersonsMaxLimit)
		limit = cScnPersonsMaxLimit
	}

	maxTime, err := parseInt64QueryParam("maxTime", q)
	if err != nil {
		a.logger.Debug("maxTime is not provided or wrong, err=", err)
		maxTime = math.MaxInt64
	}

	stl, err := a.ScnProcessor.GetTimelineView(camId, common.Timestamp(maxTime), int(limit))
	if a.errorResponse(c, err) {
		return
	}

	c.JSON(http.StatusOK, a.toSceneTimeline(stl))
}

// Creates a new organization. List of fields will be ignored
// POST /orgs - superadmin(sa)
func (a *api) h_POST_orgs(c *gin.Context) {
	a.logger.Debug("POST /orgs")
	var org Organization
	if a.errorResponse(c, c.Bind(&org)) {
		return
	}

	a.logger.Info("New organization ", org)
	id, err := a.Dc.InsertOrg(a.org2morg(&org))
	if a.errorResponse(c, err) {
		return
	}

	w := c.Writer
	uri := composeURI(c.Request, strconv.FormatInt(id, 10))
	a.logger.Info("New organization with location ", uri, " has been just created")
	w.Header().Set("Location", uri)
	c.Status(http.StatusCreated)
}

// Retruns full organiation object, including fields list configured
// GET /orgs/:orgId - owner (o), sa
func (a *api) h_GET_orgs_orgId(c *gin.Context) {
	orgId, err := parseInt64Param(c, "orgId")
	a.logger.Debug("GET /orgs/", orgId)
	if a.errorResponse(c, err) {
		return
	}

	org, fis, err := a.Dc.GetOrgAndFields(orgId)
	if a.errorResponse(c, err) {
		return
	}

	c.JSON(http.StatusOK, a.morg2org(org, fis))
}

// Create new field. the number of fields is limited
// POST /orgs/:orgId/fields - owner (o), sa
func (a *api) h_POST_orgs_orgId_fields(c *gin.Context) {
	orgId, err := parseInt64Param(c, "orgId")
	a.logger.Debug("POST /orgs/", orgId, "/fields")
	if a.errorResponse(c, err) {
		return
	}

	var mis OrgMetaInfoArr
	if a.errorResponse(c, c.Bind(&mis)) {
		return
	}

	fis := a.metaInfos2FieldInfos(mis, orgId)
	if a.errorResponse(c, a.Dc.InsertNewFields(orgId, fis)) {
		return
	}

	a.logger.Info("New fields were added for orgId=", orgId, " ", fis)
	c.Status(http.StatusCreated)
}

// Retrieves list of fields only
// GET /orgs/:orgId/fields - owner (o), sa
func (a *api) h_GET_orgs_orgId_fields(c *gin.Context) {
	orgId, err := parseInt64Param(c, "orgId")
	a.logger.Debug("GET /orgs/", orgId, "/fields")
	if a.errorResponse(c, err) {
		return
	}

	fis, err := a.Dc.GetFieldInfos(orgId)
	if a.errorResponse(c, err) {
		return
	}

	c.JSON(http.StatusOK, a.fieldInfos2MetaInfos(fis))
}

// Updates the field value (only display name update is allowed)
// PUT /orgs/:orgId/fields/:fldId - owner (o), sa
func (a *api) h_PUT_orgs_orgId_fields_fldId(c *gin.Context) {
	orgId, err := parseInt64Param(c, "orgId")
	if a.errorResponse(c, err) {
		return
	}
	fldId, err := parseInt64Param(c, "fldId")
	a.logger.Info("PUT /orgs/", orgId, "/fields/", fldId)
	if a.errorResponse(c, err) {
		return
	}

	mi := &OrgMetaInfo{}
	if a.errorResponse(c, c.Bind(mi)) {
		return
	}
	fi := a.metaInfo2FieldInfo(mi)
	fi.OrgId = orgId

	if a.errorResponse(c, a.Dc.UpdateFieldInfo(fi)) {
		return
	}

	c.Status(http.StatusNoContent)
}

// Delete field.
// DELETE /orgs/:orgId/fields/:fldId - owner (o), sa
func (a *api) h_DELETE_orgs_orgId_fields_fldId(c *gin.Context) {
	orgId, err := parseInt64Param(c, "orgId")
	if a.errorResponse(c, err) {
		return
	}
	fldId, err := parseInt64Param(c, "fldId")
	if a.errorResponse(c, err) {
		return
	}
	a.logger.Info("DELETE /orgs/", orgId, "/fields/", fldId)

	if a.errorResponse(c, a.Dc.DeleteFieldInfo(orgId, fldId)) {
		return
	}

	c.Status(http.StatusNoContent)
}

// Returns authorized organization id, or error if not authenticated for an org
func getOrgId(c *gin.Context) (int64, error) {
	//TODO fix me
	return 1, nil
}

func wrapError(msg string, e error) error {
	if e == nil {
		return nil
	}
	return errors.New(msg + e.Error())
}

// POST /profiles - o, u, sa
func (a *api) h_POST_profiles(c *gin.Context) {
	a.logger.Info("POST /profiles")
	orgId, err := getOrgId(c)
	if a.errorResponse(c, err) {
		return
	}

	var p Profile
	if a.errorResponse(c, wrapError("Cannot unmarshal body, err=", c.Bind(&p))) {
		return
	}

	prf, err := a.profile2mProfile(&p)
	if a.errorResponse(c, err) {
		return
	}
	prf.OrgId = orgId

	pid, err := a.Dc.InsertProfile(prf)
	if a.errorResponse(c, err) {
		return
	}

	w := c.Writer
	uri := composeURI(c.Request, strconv.FormatInt(pid, 10))
	a.logger.Debug("New profile with location ", uri, " has been just created")
	w.Header().Set("Location", uri)
	c.Status(http.StatusCreated)
}

// GET /profiles/:prfId - o, u, sa
func (a *api) h_GET_profiles_prfId(c *gin.Context) {
	prfId, err := parseInt64Param(c, "prfId")
	if a.errorResponse(c, err) {
		return
	}

	a.logger.Info("GET /profiles/", prfId)
	orgId, err := getOrgId(c)
	if a.errorResponse(c, err) {
		return
	}

	p, err := a.Dc.GetProfile(prfId)
	if a.errorResponse(c, err) {
		return
	}

	if p.OrgId != orgId {
		a.errorResponse(c, common.NewError(common.ERR_NOT_FOUND, "No profile with id="+strconv.FormatInt(prfId, 10)))
		return
	}
	c.JSON(http.StatusOK, a.profile2mprofile(p))
}

// Updates profile. All provided values will be replaced, all other ones will be lost
// PUT /profiles/:profileId - o, u, sa
func (a *api) h_PUT_profiles_prfId(c *gin.Context) {
	prfId, err := parseInt64Param(c, "prfId")
	if a.errorResponse(c, err) {
		return
	}
	a.logger.Debug("PUT /profiles/", prfId)
	orgId, err := getOrgId(c)
	if a.errorResponse(c, err) {
		return
	}

	var p Profile
	if a.errorResponse(c, wrapError("Cannot unmarshal body, err=", c.Bind(&p))) {
		return
	}

	prf, err := a.profile2mProfile(&p)
	if a.errorResponse(c, err) {
		return
	}
	prf.OrgId = orgId
	prf.Id = prfId

	err = a.Dc.UpdateProfile(prf)
	if a.errorResponse(c, err) {
		return
	}
	c.Status(http.StatusNoContent)
}

// DELETE /profiles/:profileId - o, u, sa
func (a *api) h_DELETE_profiles_prfId(c *gin.Context) {
	prfId, err := parseInt64Param(c, "prfId")
	if a.errorResponse(c, err) {
		return
	}
	a.logger.Debug("DELETE /profiles/", prfId)
	orgId, err := getOrgId(c)
	if a.errorResponse(c, err) {
		return
	}

	err = a.Dc.DeleteProfile(prfId, orgId)
	if a.errorResponse(c, err) {
		return
	}
	c.Status(http.StatusNoContent)
}

// GET /persons/:persId
func (a *api) h_GET_persons_persId(c *gin.Context) {
	persId := c.Param("persId")
	orgId, err := getOrgId(c)
	if a.errorResponse(c, err) {
		return
	}

	// Parse query
	q := c.Request.URL.Query()
	includeDetails := parseBoolQueryParam("details", q, false)
	includeMeta := parseBoolQueryParam("meta", q, false)

	a.logger.Debug("GET /persons/", persId, "?details=", includeDetails, "&meta=", includeMeta, " [orgId=", orgId, "]")

	desc, err := a.Dc.DescribePerson(persId, orgId, includeDetails, includeMeta)
	if a.errorResponse(c, err) {
		return
	}
	c.JSON(http.StatusOK, a.prsnDesc2Person(desc))
}

// Only the following fields must be both provided and will be updated:
// - AvatarUrl
// - ProfileId
// PUT /persons/:persId
func (a *api) h_PUT_persons_persId(c *gin.Context) {
	persId := c.Param("persId")
	orgId, err := getOrgId(c)
	if a.errorResponse(c, err) {
		return
	}
	var p Person
	if a.errorResponse(c, wrapError("Cannot unmarshal body, err=", c.Bind(&p))) {
		return
	}

	p.Id = persId
	mp, err := a.person2mperson(&p)
	if a.errorResponse(c, err) {
		return
	}
	if a.errorResponse(c, a.Dc.UpdatePerson(mp, orgId)) {
		return
	}
	c.Status(http.StatusNoContent)
}

// GET /cameras
func (a *api) h_GET_cameras(c *gin.Context) {

}

// POST /cameras
func (a *api) h_POST_cameras(c *gin.Context) {

}

// GET /cameras/:camId
func (a *api) h_GET_cameras_camId(c *gin.Context) {

}

// DELETE /cameras/:camId
func (a *api) h_DELETE_cameras_camId(c *gin.Context) {

}

// GET /profiles/:profileId/persons
func (a *api) h_GET_profile_persons(c *gin.Context, profileId common.Id) {
	a.logger.Debug("GET /profiles/", profileId)
	//	rctx := a.newRequestCtx(c)
	//	now := common.CurrentTimestamp()

	//	pers, err := rctx.getPersonsByQuery(&common.PersonsQuery{ProfileId: profileId, Limit: 100, FromTime: now})
	//	if a.errorResponse(c, err) {
	//		return
	//	}

	//	c.JSON(http.StatusOK, pers)
}

// POST /profiles/:profileId/persons
func (a *api) h_POST_profile_persons(c *gin.Context, profileId common.Id) {
	a.logger.Info("POST /profiles/", profileId, "/persons")
	//	rctx := a.newRequestCtx(c)
	//	var person Person
	//	err := c.Bind(&person)
	//	if a.errorResponse(c, err) {
	//		return
	//	}

	//	err = rctx.associatePersonToProfile(&person, profileId)
	//	if a.errorResponse(c, err) {
	//		return
	//	}

	//	r := c.Request
	//	w := c.Writer
	//	w.Header().Set("Location", composeURI(r, string(person.Id)))
}

// GET /profiles/:profileId/persons/:personId
func (a *api) h_GET_profile_persons_person(c *gin.Context, profileId common.Id, personId common.Id) {
	a.logger.Debug("GET /profiles/", profileId, "/persons/", personId)
	//	rctx := a.newRequestCtx(c)
	//	pers, err := rctx.getPersonsByQuery(&common.PersonsQuery{PersonIds: []common.Id{personId}})
	//	if pers == nil || len(pers) != 1 {
	//		a.logger.Warn("Could not find personId=", personId, ", err=", err)
	//		c.Status(http.StatusNotFound)
	//		return
	//	}

	//	prsn := pers[0]
	//	if prsn.Profile == nil || prsn.Profile.Id != profileId {
	//		a.logger.Warn("The person person=", prsn, " is not associated with profileId=", profileId)
	//		c.Status(http.StatusNotFound)
	//		return
	//	}

	//	c.JSON(http.StatusOK, prsn)
}

// POST /profiles/
func (a *api) h_POST_profile(c *gin.Context) {
	a.logger.Info("POST /profiles/")
	//	rctx := a.newRequestCtx(c)

	//	var profile Profile
	//	err := c.Bind(&profile)
	//	if a.errorResponse(c, err) {
	//		return
	//	}

	//	prfId, err := rctx.newProfile(&profile)
	//	if a.errorResponse(c, err) {
	//		return
	//	}

	//	r := c.Request
	//	w := c.Writer
	//	w.Header().Set("Location", composeURI(r, string(prfId)))
}

// GET /pictures/:picId
func (a *api) h_GET_pictures_pic(c *gin.Context, picId common.Id) {
	a.logger.Debug("GET /pictures/", picId)
	//	rctx := a.newRequestCtx(c)

	//	pi, err := rctx.getPictureInfo(picId)
	//	if a.errorResponse(c, err) {
	//		return
	//	}

	//	c.JSON(http.StatusOK, pi)
}

// GET /images/:imgName
// the image name is encoded like <id>[_l_t_r_b].png
//
// so the size part can be missed. Valid names are:
// asbasdfasdf-234.png	 - no region
// 1234987239487.png	 - no region
// 12341234_0_3_200_300.png - get the region(l:0, t:3, r:200, b:300) for 12341234.png
func (a *api) h_GET_images_png_download(c *gin.Context) {
	imgName := c.Param("imgName")
	a.logger.Debug("GET /images/", imgName)

	imgId, err := common.ImgParseFileNameNotDeep(imgName)
	if a.errorResponse(c, err) {
		return
	}

	w := c.Writer
	imgD := a.ImgService.Read(common.Id(imgId), false)
	if imgD == nil {
		a.logger.Debug("Could not find image with id=", imgId, ", err=", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	r := c.Request
	rd := imgD.Reader.(io.ReadSeeker)
	fn := imgD.FileName
	w.Header().Set("Content-Disposition", "attachment; filename=\""+fn+"\"")
	http.ServeContent(w, r, fn, imgD.Timestamp.ToTime(), rd)
}

// ================================ Helpers ==================================
func (a *api) String() string {
	return "api: {}"
}

func ptr2int64(i *int64, defVal int64) int64 {
	if i == nil {
		return defVal
	}
	return *i
}

func ptr2string(s *string, defVal string) string {
	if s == nil {
		return defVal
	}
	return *s
}

func toPtrInt64(v int64) *int64 {
	return &v
}

func toPtrString(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

// Will block invoker until an error happens
// If the application is interrupted by SIGINT, it will complete gracefully and return
func (a *api) Run() {
	port := strconv.FormatInt(int64(a.Config.HttpPort), 10)
	a.logger.Info("Running API on ", port)
	graceful.Run(":"+port, 100*time.Millisecond, a.ge)
}

func (a *api) PrintRequest(c *gin.Context) {
	if a.logger.GetLevel() >= log4g.DEBUG {
		r, _ := httputil.DumpRequest(c.Request, true)
		a.logger.Debug("\n>>> REQUEST\n", string(r), "\n<<< REQUEST")
	}
}

func parseInt64QueryParam(prmName string, vals url.Values) (int64, error) {
	v := vals[prmName]
	if v == nil || len(v) == 0 {
		return 0, errors.New("Param " + prmName + " not found.")
	}
	return gorivets.ParseInt64(v[0], 0, math.MaxInt64, 0)
}

func parseBoolQueryParam(prmName string, vals url.Values, defVal bool) bool {
	v := vals[prmName]
	if v == nil || len(v) == 0 {
		return defVal
	}
	b, err := strconv.ParseBool(v[0])
	if err != nil {
		return defVal
	}
	return b
}

func parseInt64Param(c *gin.Context, prmName string) (int64, error) {
	prm := c.Param(prmName)
	if prm == "" {
		return -1, errors.New("Expecting some value for int parameter(" + prmName + ")")
	}
	val, err := strconv.ParseInt(prm, 10, 64)
	if err != nil {
		return -1, errors.New("Expecting an integer value, but got \"" + prm + "\"")
	}
	return val, nil
}

//============================== Transformers ================================
func (a *api) imgURL(imgId string) string {
	if imgId == "" {
		return ""
	}
	return a.Config.ImgsPrefix + common.ImgMakeFileName(imgId, nil)
}

func (a *api) prsnDesc2Person(prsnDesc *service.PersonDesc) *Person {
	p := a.mperson2person(prsnDesc.Person, nil)
	if prsnDesc.Profiles != nil {
		mchs := make([]*Profile, 0, len(prsnDesc.Profiles))
		for _, mp := range prsnDesc.Profiles {
			pr := a.profile2mprofile(mp)
			if mp.Id == prsnDesc.Person.ProfileId {
				p.Profile = pr
			}
			mchs = append(mchs, pr)
		}
		if prsnDesc.Person.MatchGroup > 0 {
			p.Matches = mchs
		}
	}
	p.Pictures = a.facesToPictureInfos(prsnDesc.Faces)
	return p
}

func (a *api) toSceneTimeline(scnTl *scene.SceneTimeline) *SceneTimeline {
	prfMap := a.profilesToProfiles(scnTl.Profiles)
	mg2Profs := make(map[int64][]*Profile)
	for pid, mgId := range scnTl.Prof2MGs {
		arr, ok := mg2Profs[mgId]
		if !ok {
			arr = make([]*Profile, 0, 1)
		}
		pr, ok := prfMap[pid]
		if ok {
			arr = append(arr, pr)
		}
		mg2Profs[mgId] = arr
	}

	stl := new(SceneTimeline)
	stl.Persons = make([]*Person, len(scnTl.Persons))
	for i, p := range scnTl.Persons {
		prsn := a.mperson2person(p, prfMap)
		prsn.CamId = &scnTl.CamId
		fcs, ok := scnTl.Faces[p.Id]
		if ok {
			prsn.Pictures = a.facesToPictureInfos(fcs)
		}
		m2p, ok := mg2Profs[p.MatchGroup]
		if ok {
			prsn.Matches = m2p
		}
		stl.Persons[i] = prsn
	}

	stl.CamId = common.Id(scnTl.CamId)
	stl.Frame.Id = scnTl.LatestPicId
	stl.Frame.PicURL = a.imgURL(scnTl.LatestPicId)

	return stl
}

func (a *api) mperson2person(p *model.Person, profs map[int64]*Profile) *Person {
	ps := new(Person)
	ps.Id = p.Id
	ps.AvatarUrl = a.imgURL(p.PictureId)
	ps.LastSeenAt = common.Timestamp(p.LastSeenAt).ToISO8601Time()
	ps.ProfileId = toPtrInt64(p.ProfileId)
	if profs != nil {
		if pr, ok := profs[p.ProfileId]; ok {
			ps.Profile = pr
		}
	}
	return ps
}

func (a *api) person2mperson(p *Person) (*model.Person, error) {
	ps := new(model.Person)
	ps.Id = p.Id
	if p.AvatarUrl != "" {
		id, err := common.ImgParseFileNameNotDeep(p.AvatarUrl)
		if err != nil {
			return nil, err
		}
		ps.PictureId = id
	}
	ps.ProfileId = ptr2int64(p.ProfileId, -1)
	return ps, nil
}

func (a *api) profilesToProfiles(profiles map[int64]*model.Profile) map[int64]*Profile {
	res := make(map[int64]*Profile)
	if profiles != nil && len(profiles) > 0 {
		for pid, p := range profiles {
			res[pid] = a.profile2mprofile(p)
		}
	}
	return res
}

func (a *api) profile2mprofile(prf *model.Profile) *Profile {
	p := new(Profile)
	p.Id = prf.Id
	p.OrgId = prf.OrgId
	p.AvatarUrl = toPtrString(a.imgURL(prf.PictureId))
	p.Attributes = a.metasToAttributes(prf.Meta)
	return p
}

func (a *api) metasToAttributes(pms []*model.ProfileMeta) []*ProfileAttribute {
	if pms == nil {
		return nil
	}
	res := make([]*ProfileAttribute, len(pms))
	for i, pm := range pms {
		res[i] = a.metaToAttribute(pm)
	}
	return res
}

func (a *api) metaToAttribute(prf *model.ProfileMeta) *ProfileAttribute {
	pa := new(ProfileAttribute)
	pa.FieldId = &prf.FieldId
	pa.Name = toPtrString(prf.DisplayName)
	pa.Value = toPtrString(prf.Value)
	return pa
}

func (a *api) facesToPictureInfos(faces []*model.Face) []*PictureInfo {
	if faces == nil {
		return []*PictureInfo{}
	}
	res := make([]*PictureInfo, len(faces))
	for i, f := range faces {
		res[i] = a.faceToPictureInfo(f)
	}
	return res
}

func (a *api) faceToPictureInfo(face *model.Face) *PictureInfo {
	pi := new(PictureInfo)
	pi.Id = face.FaceImageId
	fUrl := a.imgURL(face.FaceImageId)
	pi.FaceURL = &fUrl
	pi.PicURL = a.imgURL(face.ImageId)
	pi.Rect = &face.Rect
	ts := common.Timestamp(face.CapturedAt)
	tss := ts.ToISO8601Time()
	pi.Timestamp = &tss
	return pi
}

func (a *api) org2morg(org *Organization) *model.Organization {
	mo := new(model.Organization)
	mo.Id = org.Id
	mo.Name = org.Name
	return mo
}

func (a *api) morg2org(mo *model.Organization, fis []*model.FieldInfo) *Organization {
	org := new(Organization)
	org.Id = mo.Id
	org.Name = mo.Name
	org.Meta = a.fieldInfos2MetaInfos(fis)
	return org
}

func (a *api) fieldInfos2MetaInfos(fieldInfos []*model.FieldInfo) []*OrgMetaInfo {
	if fieldInfos == nil {
		return nil
	}
	res := make([]*OrgMetaInfo, len(fieldInfos))
	for i, fi := range fieldInfos {
		res[i] = a.fieldInfo2MetaInfo(fi)
	}
	return res
}

func (a *api) fieldInfo2MetaInfo(fieldInfo *model.FieldInfo) *OrgMetaInfo {
	mi := new(OrgMetaInfo)
	mi.FieldName = fieldInfo.DisplayName
	mi.FieldType = fieldInfo.FieldType
	mi.Id = fieldInfo.Id
	return mi
}

func (a *api) metaInfos2FieldInfos(mis OrgMetaInfoArr, orgId int64) []*model.FieldInfo {
	if mis == nil {
		return nil
	}
	res := make([]*model.FieldInfo, len(mis))
	for i, mi := range mis {
		res[i] = a.metaInfo2FieldInfo(mi)
		res[i].OrgId = orgId
	}
	return res
}

func (a *api) metaInfo2FieldInfo(mi *OrgMetaInfo) *model.FieldInfo {
	fi := new(model.FieldInfo)
	fi.DisplayName = strings.Trim(mi.FieldName, " \t")
	fi.FieldType = mi.FieldType
	fi.Id = mi.Id
	return fi
}

func (a *api) profile2mProfile(p *Profile) (*model.Profile, error) {
	prf := new(model.Profile)
	prf.Id = p.Id
	prf.OrgId = p.OrgId
	avtUrl := ptr2string(p.AvatarUrl, "")
	if avtUrl != "" {
		id, err := common.ImgParseFileNameNotDeep(avtUrl)
		if err != nil {
			return nil, err
		}
		prf.PictureId = id
	}
	prf.Meta = a.prfAttrbs2prfMetas(p.Attributes)
	return prf, nil
}

func (a *api) prfAttrbs2prfMetas(pas []*ProfileAttribute) []*model.ProfileMeta {
	if pas == nil {
		return nil
	}

	pms := make([]*model.ProfileMeta, len(pas))
	for i, pa := range pas {
		pms[i] = a.prfAttrb2prfMeta(pa)
	}
	return pms
}

// It never fills ProfileId(!!!)
func (a *api) prfAttrb2prfMeta(pa *ProfileAttribute) *model.ProfileMeta {
	pm := new(model.ProfileMeta)
	pm.FieldId = ptr2int64(pa.FieldId, 0)
	pm.Value = ptr2string(pa.Value, "")
	pm.DisplayName = ptr2string(pa.Name, "")
	return pm
}

func (a *api) errorResponse(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}

	if common.CheckError(err, common.ERR_NOT_FOUND) {
		a.logger.Warn("Not Found err=", err)
		c.JSON(http.StatusNotFound, err.Error())
		return true
	}

	a.logger.Warn("Bad request err=", err)
	c.JSON(http.StatusBadRequest, err.Error())
	return true
}

func composeURI(r *http.Request, id string) string {
	var resURL string
	if r.URL.IsAbs() {
		resURL = path.Join(r.URL.String(), id)
	} else {
		resURL = resolveScheme(r) + "://" + path.Join(resolveHost(r), r.URL.String(), id)
	}
	return resURL
}

func resolveScheme(r *http.Request) string {
	switch {
	case r.Header.Get("X-Forwarded-Proto") == "https":
		return "https"
	case r.URL.Scheme == "https":
		return "https"
	case r.TLS != nil:
		return "https"
	case strings.HasPrefix(r.Proto, "HTTPS"):
		return "https"
	default:
		return "http"
	}
}

func resolveHost(r *http.Request) (host string) {
	switch {
	case r.Header.Get("X-Forwarded-For") != "":
		return r.Header.Get("X-Forwarded-For")
	case r.Header.Get("X-Host") != "":
		return r.Header.Get("X-Host")
	case r.Host != "":
		return r.Host
	case r.URL.Host != "":
		return r.URL.Host
	default:
		return ""
	}
}
