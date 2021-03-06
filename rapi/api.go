package rapi

import (
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jrivets/gorivets"
	"github.com/jrivets/log4g"
	"github.com/pixty/console/common"
	"github.com/pixty/console/model"
	"github.com/pixty/console/service"
	"github.com/pixty/console/service/auth"
	"github.com/pixty/console/service/email"
	"github.com/pixty/console/service/image"
	"github.com/pixty/console/service/scene"
	"golang.org/x/net/context"
	"gopkg.in/tylerb/graceful.v1"
)

type (
	api struct {
		ge           *gin.Engine
		Config       *common.ConsoleConfig  `inject:""`
		ImageService *image.ImageService    `inject:""`
		ScnProcessor *scene.SceneProcessor  `inject:"scnProcessor"`
		Persister    model.Persister        `inject:"persister"`
		MainCtx      context.Context        `inject:"mainCtx"`
		Dc           service.DataController `inject:""`
		SessService  auth.SessionService    `inject:""`
		AuthService  auth.AuthService       `inject:""`
		EmSender     email.Sender           `inject:""`
		authMW       *auth_middleware
		logger       log4g.Logger
	}

	error_resp struct {
		Status       int    `json:"status"`
		ErrorMessage string `json:"error"`
	}
)

const (
	cScnPersonsMinLimit = 3
	cScnPersonsDefLimit = 20
	cScnPersonsMaxLimit = 50
)

func NewAPI() *api {
	a := new(api)
	a.logger = log4g.GetLogger("pixty.rest")
	return a
}

// =========================== PostConstructor ===============================
func (a *api) DiPostConstruct() {
	a.logger.Info("HTTP Debug mode=", a.Config.HttpDebugMode)
	if !a.Config.HttpDebugMode {
		gin.SetMode(gin.ReleaseMode)
	}

	a.authMW = newAuthMW(a.AuthService, a.SessService, a.Persister)

	a.ge = gin.New()
	if a.Config.HttpDebugMode {
		a.logger.Info("Gin logger and gin.debug is enabled. You can set up DEBUG mode for the pixty.rest group to obtain requests dumps and more logs for the API group.")
		a.ge.Use(gin.Logger())
	}
	// Request logger middleware
	a.ge.Use(a.PrintRequest)
	// Basic authentication scheme
	a.ge.Use(a.authMW.basicAuth)
	// Session and cokie auth scheme
	a.ge.Use(a.authMW.sessionAuth)

	a.logger.Info("Constructing ReST API")

	// The ping returns pong and URI of the ping, how we see it.
	a.ge.GET("/ping", a.h_GET_ping)

	// Create new secured session JSON {"login": "user", "password": "abc"} is exepected
	a.ge.POST("/sessions", a.h_POST_sessions)

	// Delete session by its sessionId
	a.ge.DELETE("/sessions/:sessId", a.h_DELETE_sessions_sessId)

	// Returns a composite object which contains list of persons(different faces) seen
	// from a time (or last seen) sorted in descending order. Every object in the list
	// is a Person JSON, which has references to list of faces of the person,
	// assigned profile (if it was) and matched profiles (the person seems to be
	// already associated). Profiles can be selected with meta data (fields) or
	// not.
	// Allowed paramas are:
	// - limit: number of records to be selected
	// - maxTime: the maximum time where the first person in the list last seen(all
	// 	other persons will have same or less time) - used for paging
	//
	// Example: curl https://api.pixty.io/cameras/12/timeline?limit=20&maxTime=12341234
	a.ge.GET("/cameras/:camId/timeline", a.h_GET_cameras_timeline)

	// Get an known image by its file name
	// Example: curl https://api.pixty.io/images/cm-1-504241500992.png
	a.ge.GET("/images/:imgName", a.h_GET_images_png_download)

	// Create new org - will be used by superadmin only
	a.ge.POST("/orgs", a.h_POST_orgs)

	// Gets all authenticated user's organizations JSON object
	a.ge.GET("/orgs", a.h_GET_orgs)

	// Gets organization JSON object
	a.ge.GET("/orgs/:orgId", a.h_GET_orgs_orgId)

	// Creates list of fields for specified organization. Every fields should
	// have display name and type (only 'text' is allowed) for now. Not for often use
	// when the field is deleted all profiles lost its values
	//
	// Example: curl -v -H "Content-Type: application/json" -X POST -d '[{"fieldName": "First Name", "fieldType": "text"}, { "fieldName": "Last Name", "fieldType": "text"}]' http://api.pixty.io/orgs/1/fields
	a.ge.POST("/orgs/:orgId/fields", a.h_POST_orgs_orgId_fields)

	// Get list of fields for the organization
	a.ge.GET("/orgs/:orgId/fields", a.h_GET_orgs_orgId_fields)

	// Gets field by its id
	a.ge.PUT("/orgs/:orgId/fields/:fldId", a.h_PUT_orgs_orgId_fields_fldId)

	// Delete an organization field - all data will be lost
	a.ge.DELETE("/orgs/:orgId/fields/:fldId", a.h_DELETE_orgs_orgId_fields_fldId)

	// returns list of user roles assignments
	a.ge.GET("/orgs/:orgId/userRoles", a.h_GET_orgs_orgId_userRoles)

	// Allows to assign user role
	a.ge.POST("/orgs/:orgId/userRoles", a.h_POST_orgs_orgId_userRoles)

	// Removes user role
	a.ge.DELETE("/orgs/:orgId/userRoles/:userId", a.h_DELETE_orgs_orgId_userRoles_userId)

	// Creates new user. The request accepts password optional field which allows
	// to set a new password due to creation. If it is not providede the password is empty.
	a.ge.POST("/users", a.h_POST_users)

	// Changes the user password. Only owner or superadmin can make the change.
	// Authenticated session is not affected
	a.ge.POST("/users/:userId/password", a.h_POST_users_userId_password)

	// Returns user info by the userId. Only owner and superadmin are authorized
	a.ge.GET("/users/:userId", a.h_GET_users_userId)

	// Returns user roles assigned through all orgs. Only owner and superadmin are authorized
	a.ge.GET("/users/:userId/userRoles", a.h_GET_users_userId_userRoles)

	// Creates a new profile. The call allows to provide some list of field values
	//
	// Example: curl -v -H "Content-Type: application/json" -X POST -d '{"AvatarUrl": "https://api/pixty.io/images/cm-1-1504241567000_731_353_950_572.png", "Attributes": [{"FieldId": 1, "Value": "Dmitry"}, {"FieldId": 2, "Value": "Spasibenko"}]}' http://api.pixty.io/profiles
	a.ge.POST("/profiles", a.h_POST_profiles)

	// Gets profile by its id. Only not empty fields will be returned(!)
	a.ge.GET("/profiles/:prfId", a.h_GET_profiles_prfId)

	// Gets profile persons by its id. Persons will not contain profile or matches references
	a.ge.GET("/profiles/:prfId/persons", a.h_GET_profiles_prfId_persons)

	// Updates profile AvatarUrl and list values. All fieds will be updated like
	// provided. It is not a PATCH, if a field is not set, it is considered as
	// removed. It is SNAPSHOT UPDATE
	a.ge.PUT("/profiles/:prfId", a.h_PUT_profiles_prfId)

	// Merges 2 profiles. It actually just re-assigns all persons with profileId=prf2Id
	// to prf1Id
	a.ge.POST("/profiles/:prf1Id/merge/:prf2Id", a.h_POST_profiles_merge)

	// Delete the profile
	a.ge.DELETE("/profiles/:prfId", a.h_DELETE_profiles_prfId)

	// Retrieves person by its id. The call can be light or include profiles and
	// pictures information. THe following query params are allowed:
	// - datails=true: includes information about the person pictures and profiles matched
	// - meta=true: includes fields in profiles
	a.ge.GET("/persons/:persId", a.h_GET_persons_persId)

	// Updates either avatar or profile assigned. Only this 2 fields will be updated.
	// Both values must be relevant in the request, it is not a PATCH! Ommitting
	// considered like an empty value, but not ignored!
	a.ge.PUT("/persons/:persId", a.h_PUT_persons_persId)

	// Creates new profile and assign the profile to the person. The old one will be rewritten
	// POST /persons/:persId/profiles - o, u, sa
	a.ge.POST("/persons/:persId/profiles", a.h_POST_persons_persId_profiles)

	// Deletes the person. All faces will be removed too.
	a.ge.DELETE("/persons/:persId", a.h_DELETE_persons_persId)

	// Deletes a person faces
	a.ge.DELETE("/persons/:persId/faces", a.h_DELETE_persons_persId_faces)

	// Gets list of cameras for the orgId (right now orgId=1), which comes from
	// the authorization of the call
	a.ge.GET("/orgs/:orgId/cameras", a.h_GET_orgs_orgId_cameras)

	// Creates new camera
	a.ge.POST("/orgs/:orgId/cameras", a.h_POST_orgs_orgId_cameras)

	// Gets information about a camera
	a.ge.GET("/cameras/:camId", a.h_GET_cameras_camId)

	// Generates new secret key for the camera. We don't keep the secret key, but its
	// hash, so it is user responsibility to get the key from the response and keeps
	// it safely. If they lost, they have to regenerate.
	a.ge.POST("/cameras/:camId/newkey", a.h_POST_cameras_camId_newkey)
}

// =========================== CamId2OrgIdCache ==============================
// The api surprisingly provides CamId2OrgIdCache interface
func (a *api) GetOrgId(camId int64) int64 {
	orgId, err := a.authMW.getCamOrgId(camId)
	if err != nil {
		a.logger.Error("Could not find orgId by camId err=", err)
		orgId = -1
	}
	return orgId
}

// =============================== Handlers ==================================
// GET /ping
func (a *api) h_GET_ping(c *gin.Context) {
	a.logger.Debug("GET /ping")
	//	err := a.EmSender.Send("dspasibenko@gmail.com", "Hello Dima", "How are U?/nDima.")
	//	if err != nil {
	//		a.logger.Error("Could not send err=", err)
	//	}
	c.String(http.StatusOK, "pong URL conversion is "+composeURI(c.Request, ""))
}

// POST /sessions
func (a *api) h_POST_sessions(c *gin.Context) {
	var user User
	if a.errorResponse(c, bindAppJson(c, &user)) {
		return
	}

	passwd := ptr2string(user.Password, "")
	ok, err := a.AuthService.AuthN(user.Login, passwd)
	if a.errorResponse(c, err) {
		return
	}

	if !ok {
		a.logger.Warn("Wrong credentials when session is create for user=", user.Login)
		c.Status(http.StatusUnauthorized)
		return
	}

	sd, err := a.SessService.NewSession(user.Login)
	if a.errorResponse(c, err) {
		return
	}

	// Hack create context in the middle
	aCtx := a.authMW.newAuthContext(c, user.Login)

	// compose sessions response
	urs, err := a.Dc.GetUserRoles(user.Login, 0)
	var orgDesc *service.OrgDesc
	if err == nil && len(urs) > 0 && urs[0].OrgId > 0 {
		orgDesc, _ = a.Dc.GetOrgDesc(aCtx, urs[0].OrgId)
	}
	mu, _ := a.Dc.GetUser(user.Login)
	res := new(Session)
	res.User = a.muser2user(mu)
	res.UserRoles = a.muserRoles2userRoles(urs)
	res.Organization = a.morg2org(orgDesc)
	res.SessionId = sd.Session()

	// set cookie, header and the response
	sessId := sd.Session()
	w := c.Writer
	c.Header(cSessHeaderName, sessId)
	cookie := &http.Cookie{Name: cSessCookieName, Value: sessId, Expires: time.Now().Add(365 * 24 * time.Hour)}
	http.SetCookie(w, cookie)
	uri := composeURI(c.Request, sessId)
	a.logger.Info("New session for ", user.Login, " has been just created")
	w.Header().Set("Location", uri)
	c.JSON(http.StatusCreated, res)
}

// DELETE /sessions/:sessId
func (a *api) h_DELETE_sessions_sessId(c *gin.Context) {
	sessId := c.Param("sessId")
	sd := a.SessService.GetBySession(sessId)
	if sd == nil {
		a.logger.Warn("No session with sessId=", sessId)
		c.Status(http.StatusNotFound)
		return
	}

	aCtx := a.getAuthContext(c)
	if a.errorResponse(c, aCtx.AuthZUser(sd.User())) {
		return
	}

	a.logger.Info("Deleting sessId=", sessId)
	a.SessService.DeleteSesion(sessId)
	c.Status(http.StatusNoContent)
}

// Returns timeline object for the camera. The timeline object contains list
// of persons sorted in descending order. The timeline object also has reference
// to the last frame for the requested camera
// GET /cameras/:camId/timeline?limit=20&maxTime=12341234
func (a *api) h_GET_cameras_timeline(c *gin.Context) {
	camId, err := parseInt64Param(c, "camId")
	if a.errorResponse(c, err) {
		return
	}
	a.logger.Debug("GET /cameras/", camId, "/timeline")

	aCtx := a.getAuthContext(c)
	if a.errorResponse(c, aCtx.AuthZCamAccess(camId, auth.AUTHZ_LEVEL_OU)) {
		return
	}

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

// Returns orgs for the logged user
// GET /orgs
func (a *api) h_GET_orgs(c *gin.Context) {
	a.logger.Debug("GET /orgs")

	aCtx := a.getAuthContext(c)
	if aCtx.UserLogin() == "" {
		c.Status(http.StatusUnauthorized)
		return
	}

	ods, err := a.Dc.GetOrgDescs(aCtx)
	if a.errorResponse(c, err) {
		return
	}
	c.JSON(http.StatusOK, a.morgs2orgs(ods))
}

// Creates a new organization. List of fields will be ignored
// POST /orgs
func (a *api) h_POST_orgs(c *gin.Context) {
	a.logger.Debug("POST /orgs")

	aCtx := a.getAuthContext(c)
	if a.errorResponse(c, aCtx.AuthZSuperadmin()) {
		return
	}

	var org Organization
	if a.errorResponse(c, bindAppJson(c, &org)) {
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

	aCtx := a.getAuthContext(c)
	if a.errorResponse(c, aCtx.AuthZHasOrgLevel(orgId, auth.AUTHZ_LEVEL_OU)) {
		return
	}

	orgDesc, err := a.Dc.GetOrgDesc(aCtx, orgId)
	if a.errorResponse(c, err) {
		return
	}

	c.JSON(http.StatusOK, a.morg2org(orgDesc))
}

// Create new field. the number of fields is limited
// POST /orgs/:orgId/fields - owner (o), sa
func (a *api) h_POST_orgs_orgId_fields(c *gin.Context) {
	orgId, err := parseInt64Param(c, "orgId")
	a.logger.Debug("POST /orgs/", orgId, "/fields")
	if a.errorResponse(c, err) {
		return
	}

	aCtx := a.getAuthContext(c)
	if a.errorResponse(c, aCtx.AuthZOrgAdmin(orgId)) {
		return
	}

	var mis OrgMetaInfoArr
	if a.errorResponse(c, bindAppJson(c, &mis)) {
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
// GET /orgs/:orgId/fields
func (a *api) h_GET_orgs_orgId_fields(c *gin.Context) {
	orgId, err := parseInt64Param(c, "orgId")
	a.logger.Debug("GET /orgs/", orgId, "/fields")
	if a.errorResponse(c, err) {
		return
	}

	aCtx := a.getAuthContext(c)
	if a.errorResponse(c, aCtx.AuthZHasOrgLevel(orgId, auth.AUTHZ_LEVEL_OU)) {
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

	aCtx := a.getAuthContext(c)
	if a.errorResponse(c, aCtx.AuthZOrgAdmin(orgId)) {
		return
	}

	mi := &OrgMetaInfo{}
	if a.errorResponse(c, bindAppJson(c, mi)) {
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

	aCtx := a.getAuthContext(c)
	if a.errorResponse(c, aCtx.AuthZOrgAdmin(orgId)) {
		return
	}

	if a.errorResponse(c, a.Dc.DeleteFieldInfo(orgId, fldId)) {
		return
	}

	c.Status(http.StatusNoContent)
}

// Retruns all user roles for the org
// GET /orgs/:orgId/userRoles
func (a *api) h_GET_orgs_orgId_userRoles(c *gin.Context) {
	orgId, err := parseInt64Param(c, "orgId")
	a.logger.Debug("POST /orgs/", orgId, "/userRoles")
	if a.errorResponse(c, err) {
		return
	}

	aCtx := a.getAuthContext(c)
	if a.errorResponse(c, aCtx.AuthZOrgAdmin(orgId)) {
		return
	}

	// returns everyone for the org
	urs, err := a.Dc.GetUserRoles("", orgId)
	if a.errorResponse(c, err) {
		return
	}
	c.JSON(http.StatusOK, a.muserRoles2userRoles(urs))
}

// Create new User role
// POST /orgs/:orgId/userRoles
func (a *api) h_POST_orgs_orgId_userRoles(c *gin.Context) {
	orgId, err := parseInt64Param(c, "orgId")
	a.logger.Debug("POST /orgs/", orgId, "/userRoles")
	if a.errorResponse(c, err) {
		return
	}

	aCtx := a.getAuthContext(c)
	if a.errorResponse(c, aCtx.AuthZOrgAdmin(orgId)) {
		return
	}

	var ur UserRole
	if a.errorResponse(c, bindAppJson(c, &ur)) {
		return
	}
	mur := a.userRole2muserRole(&ur)
	if a.errorResponse(c, a.Dc.InsertUserRole(aCtx, orgId, mur)) {
		return
	}
	c.Status(http.StatusNoContent)
}

// Delete a user role
// DELETE /orgs/:orgId/userRoles/:userId
func (a *api) h_DELETE_orgs_orgId_userRoles_userId(c *gin.Context) {
	orgId, err := parseInt64Param(c, "orgId")
	a.logger.Debug("POST /orgs/", orgId, "/userRoles")
	if a.errorResponse(c, err) {
		return
	}

	aCtx := a.getAuthContext(c)
	if a.errorResponse(c, aCtx.AuthZOrgAdmin(orgId)) {
		return
	}

	login := c.Param("userId")
	if a.errorResponse(c, a.Dc.RevokeUserRole(orgId, login)) {
		return
	}
	c.Status(http.StatusNoContent)
}

// POST /users - create new user
func (a *api) h_POST_users(c *gin.Context) {
	usr := &User{}
	if a.errorResponse(c, bindAppJson(c, usr)) {
		return
	}
	a.logger.Info("Creating new user ", usr)
	mu := a.user2muser(usr)
	if a.errorResponse(c, a.Dc.CreateUser(mu)) {
		return
	}

	if usr.Password != nil {
		if err := a.Dc.SetUserPasswd(usr.Login, ptr2string(usr.Password, "")); err != nil {
			a.logger.Warn("Could not set user password for new user ", usr.Login, ", err=", err, ". Will leave it intact")
		}
	}

	w := c.Writer
	uri := composeURI(c.Request, usr.Login)
	w.Header().Set("Location", uri)
	c.Status(http.StatusCreated)
}

// Get a user info
// GET /users/:userId
func (a *api) h_GET_users_userId(c *gin.Context) {
	login := c.Param("userId")
	aCtx := a.getAuthContext(c)
	if a.errorResponse(c, aCtx.AuthZUser(login)) {
		return
	}

	u, err := a.Dc.GetUser(login)
	if a.errorResponse(c, err) {
		return
	}
	c.JSON(http.StatusOK, a.muser2user(u))
}

// Get the user roles
// GET /users/:userId/userRoles
func (a *api) h_GET_users_userId_userRoles(c *gin.Context) {
	login := c.Param("userId")
	aCtx := a.getAuthContext(c)
	if a.errorResponse(c, aCtx.AuthZUser(login)) {
		return
	}

	urs, err := a.Dc.GetUserRoles(login, 0)
	if a.errorResponse(c, err) {
		return
	}
	c.JSON(http.StatusOK, a.muserRoles2userRoles(urs))
}

// POST /users/:userId/password - set new password
func (a *api) h_POST_users_userId_password(c *gin.Context) {
	login := c.Param("userId")
	aCtx := a.getAuthContext(c)
	if a.errorResponse(c, aCtx.AuthZUser(login)) {
		return
	}

	usr := &User{}
	if a.errorResponse(c, bindAppJson(c, usr)) {
		return
	}

	err := a.Dc.SetUserPasswd(login, ptr2string(usr.Password, ""))
	if a.errorResponse(c, err) {
		a.logger.Warn("Could not set user password for user ", login, ", err=", err, ". Will leave it intact")
		return
	}

	a.logger.Info("Password for user ", login, ", was changed successfully!")

	c.Status(http.StatusNoContent)
}

// POST /profiles - o, u, sa
func (a *api) h_POST_profiles(c *gin.Context) {
	a.logger.Info("POST /profiles")

	var p Profile
	if a.errorResponse(c, bindAppJson(c, &p)) {
		return
	}

	prf, err := a.profile2mprofile(&p)
	if a.errorResponse(c, err) {
		return
	}

	aCtx := a.getAuthContext(c)
	if a.errorResponse(c, aCtx.AuthZHasOrgLevel(prf.OrgId, auth.AUTHZ_LEVEL_OU)) {
		return
	}

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

// POST /profiles/:prf1Id/merge/:prf2Id
func (a *api) h_POST_profiles_merge(c *gin.Context) {
	prf1Id, err := parseInt64Param(c, "prf1Id")
	if a.errorResponse(c, err) {
		return
	}

	prf2Id, err := parseInt64Param(c, "prf2Id")
	if a.errorResponse(c, err) {
		return
	}

	a.logger.Info("POST /profiles/", prf1Id, "/merge/", prf2Id)
	aCtx := a.getAuthContext(c)

	if a.errorResponse(c, a.Dc.MergeProfiles(aCtx, prf1Id, prf2Id)) {
		return
	}

	c.Status(http.StatusNoContent)
}

// GET /profiles/:prfId - o, u, sa
func (a *api) h_GET_profiles_prfId(c *gin.Context) {
	prfId, err := parseInt64Param(c, "prfId")
	if a.errorResponse(c, err) {
		return
	}

	a.logger.Debug("GET /profiles/", prfId)

	p, err := a.Dc.GetProfile(prfId)
	if a.errorResponse(c, err) {
		return
	}

	aCtx := a.getAuthContext(c)
	if a.errorResponse(c, aCtx.AuthZHasOrgLevel(p.OrgId, auth.AUTHZ_LEVEL_OU)) {
		return
	}

	c.JSON(http.StatusOK, a.mprofile2profile(p))
}

// GET /profiles/:prfId/persons
func (a *api) h_GET_profiles_prfId_persons(c *gin.Context) {
	prfId, err := parseInt64Param(c, "prfId")
	if a.errorResponse(c, err) {
		return
	}

	a.logger.Debug("GET /profiles/", prfId, "persons")
	aCtx := a.getAuthContext(c)
	pds, err := a.Dc.DescribePersonsByProfile(aCtx, prfId)
	if a.errorResponse(c, err) {
		return
	}

	c.JSON(http.StatusOK, a.prsnDescs2Persons(pds))
}

// Updates profile. All provided values will be replaced, all other ones will be lost
// PUT /profiles/:profileId - o, u, sa
func (a *api) h_PUT_profiles_prfId(c *gin.Context) {
	prfId, err := parseInt64Param(c, "prfId")
	if a.errorResponse(c, err) {
		return
	}
	a.logger.Debug("PUT /profiles/", prfId)

	var p Profile
	if a.errorResponse(c, bindAppJson(c, &p)) {
		return
	}

	prf, err := a.profile2mprofile(&p)
	if a.errorResponse(c, err) {
		return
	}

	aCtx := a.getAuthContext(c)
	if a.errorResponse(c, aCtx.AuthZHasOrgLevel(prf.OrgId, auth.AUTHZ_LEVEL_OU)) {
		return
	}

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

	err = a.Dc.DeleteProfile(a.getAuthContext(c), prfId)
	if a.errorResponse(c, err) {
		return
	}
	c.Status(http.StatusNoContent)
}

// GET /persons/:persId
func (a *api) h_GET_persons_persId(c *gin.Context) {
	persId := c.Param("persId")

	// Parse query
	q := c.Request.URL.Query()
	includeDetails := parseBoolQueryParam("details", q, false)
	includeMeta := parseBoolQueryParam("meta", q, false)

	a.logger.Debug("GET /persons/", persId, "?details=", includeDetails, "&meta=", includeMeta)

	desc, err := a.Dc.DescribePerson(a.getAuthContext(c), persId, includeDetails, includeMeta)
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
	a.logger.Debug("PUT /persons/", persId)
	var p Person
	if a.errorResponse(c, bindAppJson(c, &p)) {
		return
	}

	aCtx := a.getAuthContext(c)
	if a.errorResponse(c, aCtx.AuthZCamAccess(ptr2int64(p.CamId, 0), auth.AUTHZ_LEVEL_OU)) {
		return
	}

	p.Id = persId
	mp, err := a.person2mperson(&p)
	if a.errorResponse(c, err) {
		return
	}
	if a.errorResponse(c, a.Dc.UpdatePerson(mp)) {
		return
	}
	c.Status(http.StatusNoContent)
}

// POST /persons/:persId/profiles - o, u, sa
func (a *api) h_POST_persons_persId_profiles(c *gin.Context) {
	persId := c.Param("persId")
	a.logger.Info("POST /persons/", persId, "/profiles")

	var p Profile
	if a.errorResponse(c, bindAppJson(c, &p)) {
		return
	}

	prf, err := a.profile2mprofile(&p)
	if a.errorResponse(c, err) {
		return
	}

	aCtx := a.getAuthContext(c)
	if a.errorResponse(c, aCtx.AuthZHasOrgLevel(prf.OrgId, auth.AUTHZ_LEVEL_OU)) {
		return
	}

	pid, err := a.Dc.InsertProfile(prf)
	if a.errorResponse(c, err) {
		return
	}

	desc, err := a.Dc.DescribePerson(aCtx, persId, false, false)
	if a.errorResponse(c, err) {
		return
	}
	desc.Person.ProfileId = pid
	if a.errorResponse(c, a.Dc.UpdatePerson(desc.Person)) {
		return
	}
	c.Status(http.StatusNoContent)
}

// DELETE /persons/:persId
func (a *api) h_DELETE_persons_persId(c *gin.Context) {
	persId := c.Param("persId")
	a.logger.Debug("DELETE /persons/", persId)
	aCtx := a.getAuthContext(c)
	if a.errorResponse(c, a.Dc.DeletePerson(aCtx, persId)) {
		return
	}
	c.Status(http.StatusNoContent)
}

// DELETE /persons/:persId/faces
func (a *api) h_DELETE_persons_persId_faces(c *gin.Context) {
	persId := c.Param("persId")
	a.logger.Debug("DELETE /persons/", persId, "/faces")

	faceIds := []string{}
	if a.errorResponse(c, bindAppJson(c, &faceIds)) {
		return
	}

	aCtx := a.getAuthContext(c)
	if a.errorResponse(c, a.Dc.DeletePersonFaces(aCtx, persId, faceIds)) {
		return
	}
	c.Status(http.StatusNoContent)
}

// GET /orgs/:orgId/cameras
func (a *api) h_GET_orgs_orgId_cameras(c *gin.Context) {
	orgId, err := parseInt64Param(c, "orgId")
	if a.errorResponse(c, err) {
		return
	}
	aCtx := a.getAuthContext(c)
	if a.errorResponse(c, aCtx.AuthZHasOrgLevel(orgId, auth.AUTHZ_LEVEL_OU)) {
		return
	}

	cams, err := a.Dc.GetAllCameras(orgId)
	if a.errorResponse(c, err) {
		return
	}
	c.JSON(http.StatusOK, a.mcams2cams(cams))
}

// POST /orgs/:orgId/cameras
func (a *api) h_POST_orgs_orgId_cameras(c *gin.Context) {
	orgId, err := parseInt64Param(c, "orgId")
	if a.errorResponse(c, err) {
		return
	}
	aCtx := a.getAuthContext(c)
	if a.errorResponse(c, aCtx.AuthZOrgAdmin(orgId)) {
		return
	}

	var cam Camera
	if a.errorResponse(c, bindAppJson(c, &cam)) {
		return
	}
	a.logger.Info("POST /orgs/", orgId, "/cameras ", cam)

	cam.OrgId = orgId
	camId, err := a.Dc.NewCamera(a.cam2mcam(&cam))
	if a.errorResponse(c, err) {
		return
	}

	w := c.Writer
	uri := composeURIWithPath(c.Request, "cameras", strconv.FormatInt(camId, 10))
	a.logger.Debug("New camera ", uri, " has been just created")
	w.Header().Set("Location", uri)
	c.Status(http.StatusCreated)
}

// GET /cameras/:camId
func (a *api) h_GET_cameras_camId(c *gin.Context) {
	camId, err := parseInt64Param(c, "camId")
	if a.errorResponse(c, err) {
		return
	}

	aCtx := a.getAuthContext(c)
	if a.errorResponse(c, aCtx.AuthZCamAccess(camId, auth.AUTHZ_LEVEL_OU)) {
		return
	}

	mcam, err := a.Dc.GetCameraById(camId)
	if a.errorResponse(c, err) {
		return
	}
	c.JSON(http.StatusOK, a.mcam2cam(mcam))
}

// POST /cameras/:camId/newkey
func (a *api) h_POST_cameras_camId_newkey(c *gin.Context) {
	camId, err := parseInt64Param(c, "camId")
	if a.errorResponse(c, err) {
		return
	}

	aCtx := a.getAuthContext(c)
	if a.errorResponse(c, aCtx.AuthZCamAccess(camId, auth.AUTHZ_LEVEL_OA)) {
		return
	}

	mc, sk, err := a.Dc.NewCameraKey(camId)
	if a.errorResponse(c, err) {
		return
	}
	cam := a.mcam2cam(mc)
	cam.SecretKey = toPtrString(sk)
	a.logger.Info("New secret key was requested and successfully generated for camId=", camId)
	c.JSON(http.StatusOK, cam)
}

// GET /images/:imgName
// the image name is encoded like <id>[_l_t_r_b].jpeg
//
// so the size part can be missed. Valid names are:
// asbasdfasdf-234.png	 - no region
// 1234987239487.png	 - no region
// 12341234_0_3_200_300.png - get the region(l:0, t:3, r:200, b:300) for 12341234.png
func (a *api) h_GET_images_png_download(c *gin.Context) {
	imgName := c.Param("imgName")
	a.logger.Debug("GET /images/", imgName)

	imd := &image.ImgDesc{}
	err := imd.ParseFileName(imgName)
	if a.errorResponse(c, err) {
		return
	}

	aCtx := a.getAuthContext(c)
	if a.errorResponse(c, aCtx.AuthZCamAccess(imd.CamId, auth.AUTHZ_LEVEL_OA)) {
		return
	}

	q := c.Request.URL.Query()
	wdth, _ := parseInt64QueryParam2("width", q, 0)
	hght, _ := parseInt64QueryParam2("height", q, 0)

	rdr, err := a.ImageService.GetImageByFileName(imd, int(wdth), int(hght))
	if a.errorResponse(c, err) {
		return
	}

	w := c.Writer
	r := c.Request
	w.Header().Set("Content-Disposition", "attachment; filename=\""+imgName+"\"")
	http.ServeContent(w, r, imgName, time.Now(), rdr.(io.ReadSeeker))
}

// ================================ Helpers ==================================
func (a *api) getAuthContext(c *gin.Context) auth.Context {
	return a.authMW.getAuthContext(c)
}

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
	c.Next()
}

func parseInt64QueryParam2(prmName string, vals url.Values, defVal int64) (int64, error) {
	v := vals[prmName]
	if len(v) == 0 {
		return defVal, nil
	}

	r, err := gorivets.ParseInt64(v[0], 0, math.MaxInt64, 0)
	if err != nil {
		return defVal, err
	}
	return r, nil
}

func parseInt64QueryParam(prmName string, vals url.Values) (int64, error) {
	v := vals[prmName]
	if len(v) == 0 {
		return 0, common.NewError(common.ERR_NOT_FOUND, "Query param="+prmName+" is expected, but not found.")
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
		return -1, common.NewError(common.ERR_NOT_FOUND, "Expecting some value for int URL path parameter("+prmName+")")
	}
	val, err := strconv.ParseInt(prm, 10, 64)
	if err != nil {
		return -1, common.NewError(common.ERR_NOT_FOUND, "Expecting an integer value, but got \""+prm+"\" for URL path parameter="+prmName)
	}
	if val < 0 {
		return -1, common.NewError(common.ERR_INVALID_VAL, "orgId must be positive")
	}
	return val, nil
}

func bindAppJson(c *gin.Context, inf interface{}) error {
	ct := c.ContentType()
	fmt.Println("content=", ct)
	if ct != "application/json" {
		return common.NewError(common.ERR_INVALID_VAL, "Expected content type for the request is 'application/json', but really is "+strconv.Quote(ct))
	}
	return c.Bind(inf)
}

func reqOp(c *gin.Context) string {
	return fmt.Sprint(c.Request.Method, " ", c.Request.URL)
}

func (a *api) errorResponse(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}

	if common.CheckError(err, common.ERR_NOT_FOUND) {
		a.logger.Warn("Not Found err=", err)
		c.JSON(http.StatusNotFound, &error_resp{http.StatusNotFound, err.Error()})
		return true
	}

	if common.CheckError(err, common.ERR_AUTH_REQUIRED) || common.CheckError(err, common.ERR_WRONG_CREDENTIALS) {
		a.logger.Warn("Auhtentication required for ", reqOp(c))
		c.Header("WWW-Authenticate", "Authorization Required")
		cookie := &http.Cookie{Name: "session", Value: "", Expires: time.Now()}
		http.SetCookie(c.Writer, cookie)
		c.Status(http.StatusUnauthorized)
		return true
	}

	if common.CheckError(err, common.ERR_UNAUTHORIZED) {
		a.logger.Warn("Unauthorized request for ", reqOp(c), " err=", err)
		c.JSON(http.StatusForbidden, &error_resp{http.StatusForbidden, err.Error()})
		return true
	}

	if common.CheckError(err, common.ERR_INVALID_VAL) || common.CheckError(err, common.ERR_LIMIT_VIOLATION) {
		a.logger.Warn("Bad request for ", reqOp(c), " err=", err)
		c.JSON(http.StatusBadRequest, &error_resp{http.StatusBadRequest, err.Error()})
		return true
	}

	a.logger.Warn("Bad request err=", err)
	c.JSON(http.StatusInternalServerError, &error_resp{http.StatusInternalServerError, err.Error()})
	return true
}

func composeURIWithPath(r *http.Request, pth, id string) string {
	return resolveScheme(r) + "://" + path.Join(resolveHost(r), pth, id)
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

//============================== Transformers ================================
func (a *api) imgURL(imgId string) string {
	if imgId == "" {
		return ""
	}
	return a.Config.ImgsPrefix + imgId
}

func (a *api) mcams2cams(mcams []*model.Camera) []*Camera {
	if mcams == nil {
		return []*Camera{}
	}
	res := make([]*Camera, len(mcams))
	for i, mc := range mcams {
		res[i] = a.mcam2cam(mc)
	}
	return res
}

func (a *api) mcam2cam(mcam *model.Camera) *Camera {
	cam := new(Camera)
	cam.Id = mcam.Id
	cam.DisplayName = mcam.Name
	cam.OrgId = mcam.OrgId
	cam.HasSecretKey = mcam.SecretKey != ""
	return cam
}

func (a *api) cam2mcam(cam *Camera) *model.Camera {
	mcam := new(model.Camera)
	mcam.Id = cam.Id
	mcam.Name = cam.DisplayName
	mcam.OrgId = cam.OrgId
	return mcam
}

func (a *api) prsnDescs2Persons(prsnDescs []*service.PersonDesc) []*Person {
	res := make([]*Person, len(prsnDescs))
	for i, pd := range prsnDescs {
		res[i] = a.prsnDesc2Person(pd)
	}
	return res
}

func (a *api) prsnDesc2Person(prsnDesc *service.PersonDesc) *Person {
	p := a.mperson2person(prsnDesc.Person, nil)
	if prsnDesc.Profiles != nil {
		mchs := make([]*Profile, 0, len(prsnDesc.Profiles))
		for _, mp := range prsnDesc.Profiles {
			pr := a.mprofile2profile(mp)
			if mp.Id == prsnDesc.Person.ProfileId {
				p.Profile = pr
			}
			mchs = append(mchs, pr)
		}
		if prsnDesc.Person.MatchGroup > 0 {
			p.Matches = mchs
			a.normalizeMatchesList(p)
		}
	}
	p.Pictures = a.facesToPictureInfos(prsnDesc.Faces)
	return p
}

func (a *api) toSceneTimeline(scnTl *scene.SceneTimeline) *SceneTimeline {
	prfMap := a.mprofiles2profiles(scnTl.Profiles)
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
			a.normalizeMatchesList(prsn)
		}
		stl.Persons[i] = prsn
	}

	stl.CamId = scnTl.CamId
	stl.Frame.Id = scnTl.LatestPicId
	stl.Frame.PicURL = a.imgURL(scnTl.LatestPicId)

	return stl
}

// Removes profile from matches list if the profile is already assigned to the person
func (a *api) normalizeMatchesList(p *Person) {
	prfId := ptr2int64(p.ProfileId, 0)
	if prfId == 0 || len(p.Matches) == 0 {
		return
	}
	for i, m := range p.Matches {
		if m.Id == prfId {
			b := make([]*Profile, len(p.Matches))
			copy(b, p.Matches)
			p.Matches = append(b[:i], b[i:]...)
			return
		}
	}
}

func (a *api) mperson2person(p *model.Person, profs map[int64]*Profile) *Person {
	ps := new(Person)
	ps.Id = p.Id
	ps.AvatarUrl = a.imgURL(p.PictureId)
	ps.LastSeenAt = common.Timestamp(p.LastSeenAt).ToISO8601Time()
	ps.ProfileId = toPtrInt64(p.ProfileId)

	if p.ProfileId > 0 {
		ps.MatchingResult = "identified"
	} else if p.MatchGroup > 0 {
		ps.MatchingResult = "ambiguous"
	} else {
		ps.MatchingResult = "matching"
	}

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
		ps.PictureId = p.AvatarUrl
	}
	ps.ProfileId = ptr2int64(p.ProfileId, -1)
	ps.CamId = ptr2int64(p.CamId, -1)
	return ps, nil
}

func (a *api) mprofiles2profiles(profiles map[int64]*model.Profile) map[int64]*Profile {
	res := make(map[int64]*Profile)
	if profiles != nil && len(profiles) > 0 {
		for pid, p := range profiles {
			res[pid] = a.mprofile2profile(p)
		}
	}
	return res
}

func (a *api) mprofile2profile(prf *model.Profile) *Profile {
	p := new(Profile)
	p.Id = prf.Id
	p.OrgId = prf.OrgId
	p.AvatarUrl = toPtrString(a.imgURL(prf.PictureId))
	p.Attributes = a.metasToAttributes(prf.Meta)
	p.MappedFields = prf.KeyVals
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
	pi.Id = strconv.FormatInt(face.Id, 10)
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

func (a *api) morgs2orgs(ods []*service.OrgDesc) []*Organization {
	if ods == nil {
		return []*Organization{}
	}
	res := make([]*Organization, len(ods))
	for i, od := range ods {
		res[i] = a.morg2org(od)
	}
	return res
}

func (a *api) morg2org(od *service.OrgDesc) *Organization {
	if od == nil {
		return nil
	}
	org := new(Organization)
	org.Id = od.Org.Id
	org.Name = od.Org.Name
	org.Meta = a.fieldInfos2MetaInfos(od.Fields)
	org.Cameras = a.mcams2cams(od.Cams)
	org.Users = a.muserRoles2userRoles(od.Users)
	return org
}

func (a *api) user2muser(user *User) *model.User {
	u := new(model.User)
	u.Login = user.Login
	u.Email = user.Email
	return u
}

func (a *api) muser2user(mu *model.User) *User {
	u := new(User)
	u.Email = mu.Email
	u.Login = mu.Login
	return u
}

func (a *api) muserRoles2userRoles(mus []*model.UserRole) []*UserRole {
	if mus == nil || len(mus) == 0 {
		return []*UserRole{}
	}
	res := make([]*UserRole, len(mus))
	for i, mu := range mus {
		res[i] = a.muserRole2userRole(mu)
	}
	return res
}

func (a *api) muserRole2userRole(mu *model.UserRole) *UserRole {
	ur := new(UserRole)
	ur.Login = mu.Login
	ur.OrgId = mu.OrgId
	ur.Role = (auth.AZLevel(mu.Role)).String()
	return ur
}

func (a *api) userRole2muserRole(ur *UserRole) *model.UserRole {
	mur := new(model.UserRole)
	mur.Login = ur.Login
	mur.OrgId = ur.OrgId
	mur.Role = int(auth.AZLevelParse(ur.Role))
	return mur
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

func (a *api) profile2mprofile(p *Profile) (*model.Profile, error) {
	prf := new(model.Profile)
	prf.Id = p.Id
	prf.OrgId = p.OrgId
	prf.KeyVals = p.MappedFields
	avtUrl := ptr2string(p.AvatarUrl, "")
	if avtUrl != "" {
		prf.PictureId = avtUrl
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
