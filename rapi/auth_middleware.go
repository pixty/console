package rapi

import (
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jrivets/log4g"
	"github.com/pixty/console/service/auth"
)

type (
	auth_middleware struct {
		authServ    auth.AuthService
		sessService auth.SessionService
		logger      log4g.Logger
	}
)

const (
	cPixtyLoginCtxParam = "PixtyLogin"
	cSessCookieName     = "session"
	cSessHeaderName     = "X-Pixty-Session"
)

func newAuthMW(authServ auth.AuthService, sessService auth.SessionService) *auth_middleware {
	am := new(auth_middleware)
	am.authServ = authServ
	am.sessService = sessService
	am.logger = log4g.GetLogger("pixty.rest.AuthMW")
	return am
}

func (am *auth_middleware) basicAuthMiddleware(realm string) gin.HandlerFunc {
	if realm == "" {
		realm = "Authorization Required"
	}
	realm = "Basic realm=" + strconv.Quote(realm)
	return func(c *gin.Context) {
		if !am.basicAuth(c) && !am.sessionAuth(c) {
			c.Header("WWW-Authenticate", realm)
			c.AbortWithStatus(http.StatusUnauthorized)
			cookie := &http.Cookie{Name: cSessCookieName, Value: "", Expires: time.Now()}
			http.SetCookie(c.Writer, cookie)
			return
		}
		c.Next()
	}
}

func (am *auth_middleware) basicAuth(c *gin.Context) bool {
	basic := c.Request.Header.Get("Authorization")
	if !strings.HasPrefix(strings.ToLower(basic), "basic ") {
		return false
	}
	bts, err := base64.StdEncoding.DecodeString(basic[6:])
	if err != nil {
		am.logger.Debug("Could not decode base64 for value=", basic[6:], " header=", basic)
		return false
	}
	usrPasswd := string(bts)
	idx := strings.Index(usrPasswd, ":")
	if idx > 0 {
		user := usrPasswd[:idx]
		passwd := usrPasswd[idx+1:]
		ok, err := am.authServ.AuthN(user, passwd)
		if err != nil {
			am.logger.Error("Could not authenticate user=", user, ", got the err=", err)
			return false
		}

		if ok {
			oldSessId := am.getRequestSession(c)
			if oldSessId != "" {
				am.logger.Info("Re-issue session due to new basic authentication, the old session=", oldSessId, " will be deleted.")
				am.sessService.DeleteSesion(oldSessId)
			}

			sd, err := am.sessService.NewSession(user)
			if err != nil {
				am.logger.Warn("Could not create new session for user=", user, ", err=", err)
				return false
			}

			// remember login in the conxtext
			c.Set(cPixtyLoginCtxParam, sd.User())
			am.setCookieAndSessId(c, sd.Session())
		}

		return ok
	}
	am.logger.Warn("Got basic auth header=", basic, ", but it has ", usrPasswd, " after encoding(no : separator!)")
	return false
}

func (am *auth_middleware) sessionAuth(c *gin.Context) bool {
	sessId := am.getRequestSession(c)
	if sessId == "" {
		return false
	}
	return am.isValidSession(c, sessId)
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

func (am *auth_middleware) isValidSession(c *gin.Context, sessId string) bool {
	sd := am.sessService.GetBySession(sessId)
	if sd == nil {
		return false
	}
	am.logger.Debug("Found session descriptor by sessionId: ", sd)

	// remember login in the conxtext
	c.Set(cPixtyLoginCtxParam, sd.User())
	return true
}

func (am *auth_middleware) setCookieAndSessId(c *gin.Context, sessId string) {
	c.Header(cSessHeaderName, sessId)
	cookie := &http.Cookie{Name: cSessCookieName, Value: sessId, Expires: time.Now().Add(365 * 24 * time.Hour)}
	http.SetCookie(c.Writer, cookie)
}

func (am *auth_middleware) authenticatedUser(c *gin.Context) string {
	un, ok := c.Get(cPixtyLoginCtxParam)
	if !ok {
		return ""
	}
	return un.(string)
}

// Checks authenticated user from context(context) with provided login.
// returns true if the context user and login are same, or context user is superadmin
func (am *auth_middleware) isUserOrSuperadmin(c *gin.Context, login string) bool {
	user := am.authenticatedUser(c)
	return user != "" && (login == user || am.isSuperadmin(user))
}

func (am *auth_middleware) isSuperadmin(login string) bool {
	al, err := am.authServ.AuthZ(login, 0)
	return err == nil && al == auth.AUTHZ_LEVEL_SA
}
