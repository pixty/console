package rapi

import "gopkg.in/gin-gonic/gin.v1"
import "strconv"
import "github.com/jrivets/log4g"
import "github.com/pixty/console/common"
import "gopkg.in/tylerb/graceful.v1"
import "time"
import "net/http"
import "io"
import "path"

type api struct {
	ge         *gin.Engine
	Config     *common.ConsoleConfig `inject:""`
	OrgService common.OrgService     `inject:"orgService"`
	CamService common.CameraService  `inject:"camService"`
	ImgService common.ImageService   `inject:"imgService"`
	CtxFactory common.ContextFactory `inject:"ctxFactory"`
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

	a.endpoint("GET", "/ping", func(c *gin.Context) { a.ping(c) })
	a.endpoint("GET", "/organizations/:orgId/cameras", func(c *gin.Context) { a.getOrgCameras(c, common.Id(c.Param("orgId"))) })

	a.endpoint("POST", "/images", func(c *gin.Context) { a.newImage(c) })
	a.endpoint("GET", "/images/:imgId", func(c *gin.Context) { a.getImageById(c, common.Id(c.Param("imgId"))) })
	a.endpoint("GET", "/cameras/:camId", func(c *gin.Context) { a.getSceneByCamId(c, common.Id(c.Param("camId"))) })
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

// GET /ping
func (a *api) ping(c *gin.Context) {
	a.logger.Debug("GET /ping")
	c.String(http.StatusOK, "pong")
}

// GET /organizations/:orgId/cameras
func (a *api) getOrgCameras(c *gin.Context, orgId common.Id) {
	a.logger.Debug("GET /organizations/", orgId, "/cameras")
	result := a.CamService.GetByOrgId(orgId)
	c.JSON(http.StatusOK, result)
}

// POST /images
func (a *api) newImage(c *gin.Context) {
	a.logger.Debug("POST /images")

	iDesc := &common.ImageDescriptor{CamId: "c1"}
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		a.logger.Error("could not obtain file for upload err=", err)
		c.JSON(http.StatusBadRequest, err.Error())
		return
	}

	iDesc.FileName = header.Filename
	iDesc.Timestamp = common.CurrentTimestamp()
	iDesc.Reader = file

	id, err := a.ImgService.New(iDesc)
	c.Status(http.StatusCreated)
	a.logger.Info("New image with id=", id, " is created")

	r := c.Request
	w := c.Writer
	w.Header().Set("Location", composeURI(r, string(id)))
}

// GET /images/:imgId
func (a *api) getImageById(c *gin.Context, imgId common.Id) {
	a.logger.Debug("GET /images/", imgId)
	r := c.Request
	w := c.Writer

	imgD := a.ImgService.Read(imgId)
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

// GET /cameras/:camId
func (a *api) getSceneByCamId(c *gin.Context, camId common.Id) {
	a.logger.Debug("GET /cameras/", camId)

	ctx := a.CtxFactory.NewContext()
	defer ctx.Close()

	//pls := a.CamService.GetScene(ctx, camId)

}

func (a *api) endpoint(method string, relativePath string, handlers ...gin.HandlerFunc) {
	a.logger.Info("Registering endpoint: ", method, " ", relativePath, " funcs: ", handlers)
	switch method {
	case "GET":
		a.ge.GET(relativePath, handlers...)
	case "POST":
		a.ge.POST(relativePath, handlers...)
	default:
		a.logger.Error("Unknonwn method ", method)
		panic("cannot register endpoint: " + method + " " + relativePath)
	}
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
