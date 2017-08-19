package rapi

import (
	"bytes"
	"image"
	"image/png"
	"io"
	"net/http"
	"path"
	"strconv"
	"time"

	"github.com/jrivets/log4g"
	"github.com/pixty/console/common"
	"github.com/pixty/console/cors"
	"golang.org/x/net/context"
	"gopkg.in/gin-gonic/gin.v1"
	"gopkg.in/tylerb/graceful.v1"
)

type api struct {
	ge     *gin.Engine
	Config *common.ConsoleConfig `inject:""`
	//	SceneService common.SceneService   `inject:"sceneService"`
	ImgService common.ImageService `inject:"imgService"`
	MainCtx    context.Context     `inject:"mainCtx"`
	//Persister  common.Persister    `inject:"persister"`
	logger log4g.Logger
}

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
	a.endpoint("GET", "/images/:imgName", func(c *gin.Context) { a.h_GET_images_png_download(c, c.Param("imgName")) })

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
	//	rctx := a.newRequestCtx(c)
	//	scene, err := rctx.getScenes(&common.SceneQuery{
	//		CamId: camId,
	//		Limit: 1,
	//	})
	//	if a.errorResponse(c, err) {
	//		return
	//	}

	//	c.JSON(http.StatusOK, scene)
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
