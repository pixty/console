package rapi

import "gopkg.in/gin-gonic/gin.v1"
import "strconv"
import "github.com/jrivets/log4g"
import "github.com/pixty/console/common"
import "gopkg.in/tylerb/graceful.v1"
import "time"
import "net/http"

type api struct {
	ge     *gin.Engine
	Config *common.ConsoleConfig `inject:""`
	logger log4g.Logger
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
}

// Will block invoker until an error happens
// If the application is interrupted by SIGINT, it will complete gracefully and return
func (a *api) Run() {
	port := strconv.FormatInt(int64(a.Config.HttpPort), 10)
	a.logger.Info("Running API on ", port)
	graceful.Run(":"+port, 100*time.Millisecond, a.ge)
}

func (a *api) ping(c *gin.Context) {
	c.String(http.StatusOK, "pong")
}

func (a *api) endpoint(method string, relativePath string, handlers ...gin.HandlerFunc) {
	a.logger.Info("Registering endpoint: ", method, " ", relativePath, " funcs: ", handlers)
	switch method {
	case "GET":
		a.ge.GET(relativePath, handlers...)
	default:
		a.logger.Error("Unknonwn method ", method)
		panic("cannot register endpoint: " + method + " " + relativePath)
	}
}
