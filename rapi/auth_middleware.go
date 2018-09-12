package rapi

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jrivets/gorivets"
	"github.com/jrivets/log4g"
	"github.com/pixty/console/common"
	"github.com/pixty/console/model"
	"github.com/pixty/console/service/auth"
)

type (
	auth_middleware struct {
		authServ    auth.AuthService
		sessService auth.SessionService
		persister   model.Persister
		lock        sync.Mutex
		obj2orgId   gorivets.LRU
		logger      log4g.Logger
	}

	auth_ctx struct {
		am       *auth_middleware
		login    string
		sessDesc auth.SessionDesc
	}
)

const (
	cPixtyCtxParam  = "PxtyCtx"
	cSessCookieName = "session"
	cSessHeaderName = "X-Pixty-Session"
)

func newAuthMW(authServ auth.AuthService, sessService auth.SessionService, persister model.Persister) *auth_middleware {
	am := new(auth_middleware)
	am.authServ = authServ
	am.sessService = sessService
	am.persister = persister
	am.obj2orgId = gorivets.NewLRU(1000, nil)
	am.logger = log4g.GetLogger("pixty.rest.AuthMW")
	return am
}

func isAuthContext(c *gin.Context) bool {
	_, ok := c.Get(cPixtyCtxParam)
	return ok
}

func (am *auth_middleware) getAuthContext(c *gin.Context) auth.Context {
	ac, ok := c.Get(cPixtyCtxParam)
	if ok {
		return ac.(*auth_ctx)
	}
	return am.newAuthContext(c, "")
}

func (am *auth_middleware) newAuthContext(c *gin.Context, login string) *auth_ctx {
	ac := new(auth_ctx)
	ac.am = am
	ac.login = login
	c.Set(cPixtyCtxParam, ac)
	return ac
}

func (am *auth_middleware) newAuthContextBySession(c *gin.Context, sd auth.SessionDesc) *auth_ctx {
	ac := new(auth_ctx)
	ac.am = am
	ac.login = sd.User()
	ac.sessDesc = sd
	c.Set(cPixtyCtxParam, ac)
	return ac
}

// Uses basic authentication and creates new context, if needed
func (am *auth_middleware) basicAuth(c *gin.Context) {
	if isAuthContext(c) {
		am.logger.Debug("Skip basic authentication, there is already auth context")
		return
	}

	basic := c.Request.Header.Get("Authorization")
	if !strings.HasPrefix(strings.ToLower(basic), "basic ") {
		am.logger.Debug("Skip basic authentication, no header")
		return
	}

	bts, err := base64.StdEncoding.DecodeString(basic[6:])
	if err != nil {
		am.logger.Warn("Could not decode base64 for value=", basic[6:], " header=", basic, " skip authentication")
		return
	}

	usrPasswd := string(bts)
	idx := strings.Index(usrPasswd, ":")
	if idx > 0 {
		user := usrPasswd[:idx]
		passwd := usrPasswd[idx+1:]
		ok, err := am.authServ.AuthN(user, passwd)
		if err != nil {
			am.logger.Error("Could not authenticate user=", user, ", got the err=", err)
			return
		}

		if !ok {
			am.logger.Warn("Wrong credentials for user=", user)
			return
		}

		am.newAuthContext(c, user)
		return
	}
	am.logger.Warn("Got basic auth header=", basic, ", but it has ", usrPasswd, " after encoding(no colon in between)")
	return
}

// Does session/cookie authentication and creates the context if needed
func (am *auth_middleware) sessionAuth(c *gin.Context) {
	if isAuthContext(c) {
		am.logger.Debug("Skip session authentication, there is already auth context")
		return
	}

	sessId := am.getRequestSession(c)
	if sessId == "" {
		am.logger.Debug("Did not find header or session cookie, still not authenticated...")
		return
	}

	sd := am.sessService.GetBySession(sessId)
	if sd == nil {
		am.logger.Debug("Could not authenticate by sessionId=", sessId, " remove cookie just in case")
		cookie := &http.Cookie{Name: cSessCookieName, Value: "", Expires: time.Now()}
		http.SetCookie(c.Writer, cookie)
		return
	}

	am.logger.Debug("Authenticated by sessionId=", sessId)
	am.newAuthContextBySession(c, sd)
}

func (am *auth_middleware) getRequestSession(c *gin.Context) string {
	sessId := c.Request.Header.Get(cSessHeaderName)
	if sessId == "" {
		// no header, checking cookie
		cookie, err := c.Request.Cookie(cSessCookieName)
		if err != nil {
			am.logger.Debug("Could not obtain session from cookie (not set?), err=", err)
			return ""
		}
		sessId = cookie.Value
	}
	return sessId
}

func (am *auth_middleware) getCamOrgIdFromCache(camId int64) int64 {
	am.lock.Lock()
	defer am.lock.Unlock()
	inf, ok := am.obj2orgId.Get(camId)
	if ok {
		return inf.(int64)
	}
	return -1
}

func (am *auth_middleware) getCamOrgId(camId int64) (int64, error) {
	res := am.getCamOrgIdFromCache(camId)
	if res > 0 {
		return res, nil
	}
	mpp, err := am.persister.GetPartitionTx("FAKE")
	if err != nil {
		return -1, err
	}
	cam, err := mpp.GetCameraById(camId)
	if err != nil {
		return -1, err
	}

	am.lock.Lock()
	defer am.lock.Unlock()
	am.obj2orgId.Add(camId, cam.OrgId, 1)
	return cam.OrgId, nil
}

//=============================== auth_ctx ===================================
var cAuthnErr = common.NewError(common.ERR_AUTH_REQUIRED, "The call requires authentication")

func (ac *auth_ctx) String() string {
	return fmt.Sprintf("auth_ctx{login=", ac.login, "}")
}

func (ac *auth_ctx) AuthN() error {
	if ac.login == "" {
		return cAuthnErr
	}
	return nil
}

func (ac *auth_ctx) IsSuperadmin() bool {
	if ac.login == "" {
		return false
	}
	al, err := ac.am.authServ.AuthZ(ac.login, 0)
	return err == nil && al == auth.AUTHZ_LEVEL_SA
}

func (ac *auth_ctx) AuthZSuperadmin() error {
	if !ac.IsSuperadmin() {
		return common.NewError(common.ERR_UNAUTHORIZED, "Only superadmins are authorized to make the call")
	}
	return nil
}

func (ac *auth_ctx) AuthZOrgAdmin(orgId int64) error {
	err := ac.AuthN()
	if err != nil {
		return err
	}

	al, err := ac.am.authServ.AuthZ(ac.login, orgId)
	if err != nil {
		return err
	}
	if al < auth.AUTHZ_LEVEL_OA {
		ac.am.logger.Debug("access level=", al, " for ac=", ac)
		return common.NewError(common.ERR_UNAUTHORIZED, "Only organization admins are authorized to make the call")
	}
	return nil
}

func (ac *auth_ctx) AuthZUser(userLogin string) error {
	err := ac.AuthN()
	if err != nil {
		return err
	}

	if userLogin == ac.login || ac.IsSuperadmin() {
		return nil
	}
	return common.NewError(common.ERR_UNAUTHORIZED, "Only "+userLogin+" user is authorized to make the call")
}

func (ac *auth_ctx) AuthZHasOrgLevel(orgId int64, level auth.AZLevel) error {
	err := ac.AuthN()
	if err != nil {
		return err
	}

	azl, err := ac.am.authServ.AuthZ(ac.login, orgId)
	if err != nil {
		return err
	}
	if azl < level {
		return common.NewError(common.ERR_UNAUTHORIZED, "The user "+ac.login+" requered to be "+level.String()+", but the user has "+azl.String()+" role for orgId="+strconv.FormatInt(orgId, 10))
	}
	return nil
}

func (ac *auth_ctx) AuthZCamAccess(camId int64, lvl auth.AZLevel) error {
	err := ac.AuthN()
	if err != nil {
		return err
	}

	orgId, err := ac.am.getCamOrgId(camId)
	if err != nil {
		return err
	}

	azl, err := ac.am.authServ.AuthZ(ac.login, orgId)
	if err != nil {
		return err
	}

	if azl < lvl {
		return common.NewError(common.ERR_UNAUTHORIZED, "The user "+ac.login+" azl:"+azl.String()+" lvl:"+lvl.String()+" is not authorized to get information from  "+strconv.FormatInt(camId, 10))
	}
	return nil
}

func (ac *auth_ctx) UserLogin() string {
	return ac.login
}
