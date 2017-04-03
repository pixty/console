package rapi

import (
	"net/http"
	"path"
	"strconv"
	"time"

	"github.com/jrivets/log4g"
	"github.com/pixty/console/common"
	"golang.org/x/net/context"
	"gopkg.in/gin-gonic/gin.v1"
	"gopkg.in/tylerb/graceful.v1"
)

type api struct {
	ge         *gin.Engine
	Config     *common.ConsoleConfig `inject:""`
	OrgService common.OrgService     `inject:"orgService"`
	ImgService common.ImageService   `inject:"imgService"`
	MainCtx    context.Context       `inject:"mainCtx"`
	Persister  common.Persister      `inject:"persister"`
	logger     log4g.Logger
}

func NewAPI() *api {
	return new(api)
}

// =========================== PostConstructor ===============================
func (a *api) DiPostConstruct() {
	if !a.Config.DebugMode {
		gin.SetMode(gin.ReleaseMode)
	}

	a.ge = gin.New()
	a.logger = log4g.GetLogger("console.rest")
	a.logger.Info("Constructing ReST API")

	a.endpoint("GET", "/ping", func(c *gin.Context) { a.h_GET_ping(c) })
	a.endpoint("GET", "/cameras/:camId/scenes", func(c *gin.Context) { a.h_GET_cameras_scenes(c, common.Id(c.Param("camId"))) })
	a.endpoint("GET", "/profiles/:profileId", func(c *gin.Context) { a.h_GET_profile(c, common.Id(c.Param("profileId"))) })
	a.endpoint("GET", "/profiles/:profileId/persons", func(c *gin.Context) { a.h_GET_profile_persons(c, common.Id(c.Param("profileId"))) })
	a.endpoint("POST", "/profiles/:profileId/persons", func(c *gin.Context) { a.h_POST_profile_persons(c, common.Id(c.Param("profileId"))) })
	a.endpoint("GET", "/profiles/:profileId/persons/:personId", func(c *gin.Context) {
		a.h_GET_profile_persons_person(c, common.Id(c.Param("profileId")), common.Id(c.Param("personId")))
	})
	a.endpoint("POST", "/profiles/", func(c *gin.Context) { a.h_POST_profile(c) })
	a.endpoint("GET", "/pictures/:picId", func(c *gin.Context) { a.h_GET_pictures_pic(c, common.Id(c.Param("picId"))) })
	a.endpoint("GET", "/pictures/:picId/download", func(c *gin.Context) { a.h_GET_pictures_pic_download(c, common.Id(c.Param("picId"))) })
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

// GET /cameras/:camId/scenes
func (a *api) h_GET_cameras_scenes(c *gin.Context, camId common.Id) {
	a.logger.Debug("GET /cameras/", camId, "/scenes")
	rctx := a.newRequestCtx(c)
	scene, err := rctx.getScenes(&common.SceneQuery{
		CamId: camId,
	})

	if err != nil {
		a.logger.Error("Could not get scene by for camId=", camId, ", err=", err)
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, scene)
}

// GET /profiles/:profileId
func (a *api) h_GET_profile(c *gin.Context, profileId common.Id) {
	a.logger.Debug("GET /profiles/", profileId)
	rctx := a.newRequestCtx(c)

	prf := rctx.getProfile(profileId)
	if prf == nil {
		c.Status(http.StatusNotFound)
		return
	}

	c.JSON(http.StatusOK, prf)
}

// GET /profiles/:profileId/persons
func (a *api) h_GET_profile_persons(c *gin.Context, profileId common.Id) {
	a.logger.Debug("GET /profiles/", profileId)
	rctx := a.newRequestCtx(c)
	now := common.CurrentTimestamp()

	pers, err := rctx.getPersonsByQuery(&common.PersonsQuery{ProfileId: profileId, Limit: 100, FromTime: now})
	if err != nil {
		a.logger.Error("Bad request err=", err)
		c.JSON(http.StatusBadRequest, err.Error())
		return
	}

	c.JSON(http.StatusOK, pers)
}

// POST /profiles/:profileId/persons
func (a *api) h_POST_profile_persons(c *gin.Context, profileId common.Id) {
	a.logger.Info("POST /profiles/", profileId, "/persons")
	rctx := a.newRequestCtx(c)
	var person Person
	err := c.Bind(&person)
	if err != nil {
		c.JSON(http.StatusBadRequest, err.Error())
		return
	}

	err = rctx.associatePersonToProfile(&person, profileId)
	if err != nil {
		a.logger.Warn("Could not complete person to profile association. err=", err)
		c.JSON(http.StatusBadRequest, err.Error())
		return
	}

	r := c.Request
	w := c.Writer
	w.Header().Set("Location", composeURI(r, string(person.Id)))
}

// GET /profiles/:profileId/persons/:personId
func (a *api) h_GET_profile_persons_person(c *gin.Context, profileId common.Id, personId common.Id) {
	a.logger.Debug("GET /profiles/", profileId, "/persons/", personId)
	rctx := a.newRequestCtx(c)
	pers, err := rctx.getPersonsByQuery(&common.PersonsQuery{PersonIds: []common.Id{personId}})
	if pers == nil || len(pers) != 1 {
		a.logger.Warn("Could not find personId=", personId, ", err=", err)
		c.Status(http.StatusNotFound)
		return
	}

	prsn := pers[0]
	if prsn.Profile == nil || prsn.Profile.Id != profileId {
		a.logger.Warn("The person person=", prsn, " is not associated with profileId=", profileId)
		c.Status(http.StatusNotFound)
		return
	}

	c.JSON(http.StatusOK, prsn)
}

// POST /profiles/
func (a *api) h_POST_profile(c *gin.Context) {
	a.logger.Info("POST /profiles/")
	rctx := a.newRequestCtx(c)

	var profile Profile
	err := c.Bind(&profile)
	if err != nil {
		a.logger.Warn("Could not turn body to profile object, err=", err)
		c.JSON(http.StatusBadRequest, err.Error())
		return
	}

	prfId, err := rctx.newProfile(&profile)
	if err != nil {
		a.logger.Error("Could not create new Profile err=", err)
		c.JSON(http.StatusBadRequest, err.Error())
		return
	}

	r := c.Request
	w := c.Writer
	w.Header().Set("Location", composeURI(r, string(prfId)))
}

// GET /pictures/:picId
func (a *api) h_GET_pictures_pic(c *gin.Context, picId common.Id) {
	a.logger.Debug("GET /pictures/", picId)
	rctx := a.newRequestCtx(c)

}

// GET /pictures/:picId/download
func (a *api) h_GET_pictures_pic_download(c *gin.Context, picId common.Id) {
}

//// GET /organizations/:orgId/cameras
//func (a *api) getOrgCameras(c *gin.Context, orgId common.Id) {
//	a.logger.Debug("GET /organizations/", orgId, "/cameras")
//	result := a.CamService.GetByOrgId(orgId)
//	c.JSON(http.StatusOK, result)
//}

// POST /images
//func (a *api) newImage(c *gin.Context) {
//	a.logger.Debug("POST /images")

//	iDesc := &common.ImageDescriptor{CamId: "c1"}
//	file, header, err := c.Request.FormFile("file")
//	if err != nil {
//		a.logger.Error("could not obtain file for upload err=", err)
//		c.JSON(http.StatusBadRequest, err.Error())
//		return
//	}

//	iDesc.FileName = header.Filename
//	iDesc.Timestamp = common.CurrentTimestamp()
//	iDesc.Reader = file

//	id, err := a.ImgService.New(iDesc)
//	c.Status(http.StatusCreated)
//	a.logger.Info("New image with id=", id, " is created")

//	r := c.Request
//	w := c.Writer
//	w.Header().Set("Location", composeURI(r, string(id)))
//}

// GET /images/:imgId
//func (a *api) getImageById(c *gin.Context, imgId common.Id) {
//	a.logger.Debug("GET /images/", imgId)
//	r := c.Request
//	w := c.Writer

//	imgD := a.ImgService.Read(imgId)
//	if imgD == nil {
//		w.WriteHeader(http.StatusNotFound)
//		return
//	}
//	fn := imgD.FileName
//	rd := imgD.Reader.(io.ReadSeeker)
//	ts := imgD.Timestamp

//	w.Header().Set("Content-Disposition", "attachment; filename=\""+fn+"\"")

//	http.ServeContent(w, r, fn, ts.ToTime(), rd)
//}

// GET /cameras/:camId
//func (a *api) getSceneByCamId(c *gin.Context, camId common.Id) {
//	a.logger.Debug("GET /cameras/", camId)

//	ctx := a.CtxFactory.NewContext()
//	defer ctx.Close()

//	pls := a.CamService.GetScene(ctx, camId)
//	c.JSON(http.StatusOK, a.toScene(camId, pls))
//}

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

//func (a *api) toScene(camId common.Id, pl []*common.PersonLog) *Scene {
//	var persons []*ScenePerson
//	scTime := common.CurrentISO8601Time()
//	if pl != nil && len(pl) != 0 {
//		scTime = pl[0].SceneTs.ToISO8601Time()
//		persons = make([]*ScenePerson, 0, len(pl))
//		for _, p := range pl {
//			persons = append(persons, a.toScenePerson(p))
//		}
//	}
//	return &Scene{CamId: camId, Timestamp: scTime, Persons: persons}
//}

//func (a *api) toScenePerson(p *common.PersonLog) *ScenePerson {
//	res := &ScenePerson{}
//	res.Person = &Person{Id: p.PersonId}
//	res.CapturedAt = p.CaptureTs.ToISO8601Time()
//	res.PicId = p.Snapshot.ImageId
//	res.PicPos = &p.Snapshot.Position
//	res.PicTime = p.Snapshot.Timestamp.ToISO8601Time()
//	return res
//}

func (a *api) newRequestCtx(c *gin.Context) *RequestCtx {
	rctx := newRequestCtx(a, a.newContext())

	// TODO Fix me later. we don't support org so far, so use the fake
	rctx.orgId = "org-1234"
	return rctx
}

func (a *api) newContext() *common.CtxHolder {
	ch, _ := common.NewCtxHolder(a.MainCtx)
	ch.WithPersister(a.Persister)
	return ch
}

func composeURI(r *http.Request, id string) string {
	var resURL string
	if r.URL.IsAbs() {
		resURL = path.Join(r.URL.String(), id)
	} else {
		resURL = path.Join(r.Host, r.URL.String(), id)
	}
	return resURL
}
