package rapi

import (
	"bytes"
	"errors"
	"image"
	"image/png"
	"io"
	"math"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"time"

	"github.com/jrivets/gorivets"
	"github.com/jrivets/log4g"
	"github.com/pixty/console/common"
	"github.com/pixty/console/cors"
	"github.com/pixty/console/model"
	"github.com/pixty/console/service/scene"
	"golang.org/x/net/context"
	"gopkg.in/gin-gonic/gin.v1"
	"gopkg.in/tylerb/graceful.v1"
)

type api struct {
	ge           *gin.Engine
	Config       *common.ConsoleConfig `inject:""`
	ImgService   common.ImageService   `inject:"imgService"`
	ScnProcessor *scene.SceneProcessor `inject:"scnProcessor"`
	MainCtx      context.Context       `inject:"mainCtx"`
	Persister    model.Persister       `inject:"persister"`
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
	if !a.Config.HttpDebugMode {
		gin.SetMode(gin.ReleaseMode)
	}

	a.ge = gin.New()

	a.ge.Use(cors.Default())
	a.logger = log4g.GetLogger("pixty.rest")
	a.logger.Info("Constructing ReST API")

	a.endpoint("GET", "/ping", func(c *gin.Context) { a.h_GET_ping(c) })
	a.endpoint("GET", "/cameras/:camId/timeline", func(c *gin.Context) { a.h_GET_cameras_timeline(c, c.Param("camId")) })
	//	a.endpoint("GET", "/profiles/:profileId", func(c *gin.Context) { a.h_GET_profile(c, common.Id(c.Param("profileId"))) })
	//	a.endpoint("GET", "/profiles/:profileId/persons", func(c *gin.Context) { a.h_GET_profile_persons(c, common.Id(c.Param("profileId"))) })
	//	a.endpoint("POST", "/profiles/:profileId/persons", func(c *gin.Context) { a.h_POST_profile_persons(c, common.Id(c.Param("profileId"))) })
	//	a.endpoint("GET", "/profiles/:profileId/persons/:personId", func(c *gin.Context) {
	//		a.h_GET_profile_persons_person(c, common.Id(c.Param("profileId")), common.Id(c.Param("personId")))
	//	})
	//	a.endpoint("POST", "/profiles/", func(c *gin.Context) { a.h_POST_profile(c) })
	//	a.endpoint("GET", "/pictures/:picId", func(c *gin.Context) { a.h_GET_pictures_pic(c, common.Id(c.Param("picId"))) })
	//	a.endpoint("GET", "/pictures/:picId/download", func(c *gin.Context) { a.h_GET_pictures_pic_download(c, common.Id(c.Param("picId"))) })
	//	a.endpoint("GET", "/images/:imgName", func(c *gin.Context) { a.h_GET_images_png_download(c, c.Param("imgName")) })

}

func (a *api) String() string {
	return "api: {}"
}

// Will block invoker until an error happens
// If the application is interrupted by SIGINT, it will complete gracefully and return
func (a *api) Run() {
	port := strconv.FormatInt(int64(a.Config.HttpPort), 10)
	a.logger.Info("Running API on ", port)
	graceful.Run(":"+port, 100*time.Millisecond, a.ge)
}

// =============================== Handlers ==================================
// GET /ping
func (a *api) h_GET_ping(c *gin.Context) {
	a.logger.Debug("GET /ping")
	c.String(http.StatusOK, "pong")
}

func parseInt64Param(prmName string, vals url.Values) (int64, error) {
	v := vals[prmName]
	if v == nil || len(v) == 0 {
		return 0, errors.New("Param " + prmName + " not found.")
	}
	return gorivets.ParseInt64(v[0], 0, math.MaxInt64, 0)
}

// Returns timeline object for the camera. The timeline object contains list
// of persons sorted in descending order. The timeline object also has reference
// to the last frame for the requested camera
// GET /cameras/:camId/timeline?limit=20&maxTime=12341234
func (a *api) h_GET_cameras_timeline(c *gin.Context, camId string) {
	a.logger.Debug("GET /cameras/", camId, "/timeline")

	// Parse query
	q := c.Request.URL.Query()
	limit, err := parseInt64Param("limit", q)
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

	maxTime, err := parseInt64Param("maxTime", q)
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

func (a *api) imgURL(imgId string) string {
	if imgId == "" {
		return ""
	}
	return a.Config.ImgsPrefix + imgId
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
		prsn := a.personToPerson(p, prfMap)
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

func (a *api) personToPerson(p *model.Person, profs map[int64]*Profile) *Person {
	ps := new(Person)
	ps.Id = p.Id
	ps.AvatarUrl = a.imgURL(p.PictureId)
	ps.LastSeenAt = common.Timestamp(p.LastSeenAt).ToISO8601Time()
	if pr, ok := profs[p.ProfileId]; ok {
		ps.Profile = pr
	}
	return ps
}

func (a *api) profilesToProfiles(profiles map[int64]*model.Profile) map[int64]*Profile {
	res := make(map[int64]*Profile)
	for pid, p := range profiles {
		res[pid] = a.profileToProfile(p)
	}
	return res
}

func (a *api) profileToProfile(prf *model.Profile) *Profile {
	p := new(Profile)
	p.Id = prf.Id
	p.AvatarUrl = a.imgURL(prf.PictureId)
	p.Attributes = prf.Meta
	return p
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

// GET /profiles/:profileId
func (a *api) h_GET_profile(c *gin.Context, profileId common.Id) {
	a.logger.Debug("GET /profiles/", profileId)
	//	rctx := a.newRequestCtx(c)

	//	prf := rctx.getProfile(profileId)
	//	if prf == nil {
	//		c.Status(http.StatusNotFound)
	//		return
	//	}

	//	c.JSON(http.StatusOK, prf)
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

// GET /pictures/:picId/download
func (a *api) h_GET_pictures_pic_download(c *gin.Context, picId common.Id) {
	a.logger.Debug("GET /pictures/", picId, "/download")

	r := c.Request
	w := c.Writer
	imgD := a.ImgService.Read(picId, false)
	if imgD == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	fn := imgD.FileName
	rd := imgD.Reader.(io.ReadSeeker)
	ts := imgD.Timestamp

	w.Header().Set("Content-Disposition", "attachment; filename=\""+fn+"\"")

	http.ServeContent(w, r, fn, ts.ToTime(), rd)
}

// GET /images/:imgName
// the image name is encoded like <id>[_l_t_r_b].png
//
// so the size part can be missed. Valid names are:
// asbasdfasdf-234.png	 - no region
// 1234987239487.png	 - no region
// 12341234_0_3_200_300.png - get the region(l:0, t:3, r:200, b:300) for 12341234.png
func (a *api) h_GET_images_png_download(c *gin.Context, imgName string) {
	a.logger.Debug("GET /images/", imgName)

	imgId, rect, err := parseImgName(imgName)
	if err != nil {
		a.logger.Warn("Cannot parse image name err=", err)
		c.JSON(http.StatusBadRequest, err.Error())
		return
	}

	w := c.Writer
	imgD := a.ImgService.Read(imgId, false)
	if imgD == nil {
		a.logger.Warn("Could not find image with id=", imgId, ", err=", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	r := c.Request
	rd := imgD.Reader.(io.ReadSeeker)

	if rect != nil {
		a.logger.Debug("Converting image ", imgId, " to ", *rect)
		img, err := png.Decode(imgD.Reader)
		if err != nil {
			a.logger.Warn("Cannot decode png image err=", err)
			c.JSON(http.StatusBadRequest, err.Error())
			return
		}

		si := img.(interface {
			SubImage(r image.Rectangle) image.Image
		}).SubImage(*rect)

		bb := bytes.NewBuffer([]byte{})
		err = png.Encode(bb, si)
		if err != nil {
			a.logger.Warn("Cannot encode png image err=", err)
			c.JSON(http.StatusBadRequest, err.Error())
			return
		}
		rd = bytes.NewReader(bb.Bytes())
		a.logger.Debug("Done with converting image ", imgId)
	}

	fn := imgName
	w.Header().Set("Content-Disposition", "attachment; filename=\""+fn+"\"")
	http.ServeContent(w, r, fn, imgD.Timestamp.ToTime(), rd)
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

func (a *api) endpoint(method string, relativePath string, handler gin.HandlerFunc) {
	a.logger.Info("Registering endpoint: ", method, " ", relativePath, " funcs: ", handler)
	switch method {
	case "GET":
		a.ge.GET(relativePath, handler)
	case "POST":
		a.ge.POST(relativePath, handler)
	default:
		a.logger.Error("Unknonwn method ", method)
		panic("cannot register endpoint: " + method + " " + relativePath)
	}
}

//func (a *api) newRequestCtx(c *gin.Context) *RequestCtx {
//	rctx := newRequestCtx(a, a.newContext())

//	// TODO Fix me later. we don't support org so far, so use the fake
//	rctx.orgId = "org-1234"
//	return rctx
//}

func composeURI(r *http.Request, id string) string {
	var resURL string
	if r.URL.IsAbs() {
		resURL = path.Join(r.URL.String(), id)
	} else {
		resURL = path.Join(r.Host, r.URL.String(), id)
	}
	return resURL
}
