package auth

import (
	"fmt"
	"regexp"
	"sync"
	"time"

	"github.com/jrivets/gorivets"
	"github.com/jrivets/log4g"
	"github.com/pixty/console/common"
	"github.com/pixty/console/model"
)

type (
	AZLevel int

	AuthService interface {
		// Checks user password. Returns error if the user is not found or the
		// password was not checked by other error. Returns whether the password
		// matches or not for the user credentials provided.
		AuthN(user, passwd string) (bool, error)
		AuthZ(user string, orgId int64) (AZLevel, error)

		// creates new user. Expects the following fields to be not-null:
		// - login
		// - email
		CreateUser(user *model.User) error

		// Updates the following fields for the user:
		// - e-mail address
		UpdateUser(user *model.User) error
		SetUserPasswd(user, passwd string) error
		UpdateUserRoles(orgId int64, login string, urs []*model.UserRole) error

		// Finds users whether by orgId, login or both
		GetUserRoles(orgId int64, login string) ([]*model.UserRole, error)
	}

	org2levels map[int64]AZLevel

	auth_service struct {
		Persister model.Persister `inject:"persister"`
		urCache   gorivets.LRU
		logger    log4g.Logger
		lock      sync.Mutex
	}
)

const (
	// Authorization levels:
	// SA - superadmin
	// OA - org admin
	// OU - org user
	// NA - Not available
	AUTHZ_LEVEL_SA     = 15
	AUTHZ_LEVEL_OA     = 10
	AUTHZ_LEVEL_OU     = 5
	AUTHZ_LEVEL_NA     = 0
	AUTHZ_LEVEL_UNKNWN = -1
)

var loginRegexp = regexp.MustCompile(`^[a-zA-Z]{1}([0-9a-zA-Z-_]+){2,39}$`)

func NewAuthService() AuthService {
	as := new(auth_service)
	as.logger = log4g.GetLogger("pixty.auth")
	as.urCache = gorivets.NewTtlLRU(1000, time.Minute*10, nil)
	return as
}

func (as *auth_service) String() string {
	cached := 0
	if as.urCache != nil {
		cached = as.urCache.Len()
	}
	return fmt.Sprint("AuthService{cached=", cached, "}")
}

func (as *auth_service) AuthN(login, passwd string) (bool, error) {
	usr, err := as.getUser(login)
	if err != nil {
		return false, err
	}

	return (getHash(usr, passwd) == usr.Hash), nil
}

func (as *auth_service) AuthZ(login string, orgId int64) (AZLevel, error) {
	azl := as.getAZLevelFromCache(login, orgId)
	if azl != AUTHZ_LEVEL_UNKNWN {
		return azl, nil
	}

	// Ok there is no data in cache yet
	mmp, err := as.Persister.GetMainTx()
	if err != nil {
		return AUTHZ_LEVEL_UNKNWN, err
	}

	urs, err := mmp.FindUserRoles(&model.UserRoleQuery{Login: login})
	if err != nil {
		return AUTHZ_LEVEL_UNKNWN, err
	}

	res := make(org2levels)
	for _, ur := range urs {
		if ur.Role == AUTHZ_LEVEL_SA {
			res = make(org2levels)
			res[0] = AUTHZ_LEVEL_SA
			break
		}
		res[ur.OrgId] = AZLevel(ur.Role)
	}

	as.lock.Lock()
	defer as.lock.Unlock()

	as.urCache.Add(login, res, 1)
	return as.getAZLevelByOrg2Levels(orgId, res), nil
}

func (as *auth_service) getAZLevelFromCache(login string, orgId int64) AZLevel {
	as.lock.Lock()
	defer as.lock.Unlock()

	o2l := as.getOrg2Levels(login)
	return as.getAZLevelByOrg2Levels(orgId, o2l)
}

func (as *auth_service) getAZLevelByOrg2Levels(orgId int64, o2l org2levels) AZLevel {
	if o2l != nil {
		ul, ok := o2l[orgId]
		if ok {
			return ul
		}

		// is superadmin?
		ul, ok = o2l[0]
		if ok {
			return ul
		}
		return AUTHZ_LEVEL_NA
	}
	return AUTHZ_LEVEL_UNKNWN
}

func (as *auth_service) getOrg2Levels(login string) org2levels {
	as.urCache.Sweep()
	o2l, ok := as.urCache.Get(login)
	if !ok {
		return nil
	}
	return o2l.(org2levels)
}

func (as *auth_service) getUser(login string) (*model.User, error) {
	mmp, err := as.Persister.GetMainTx()
	if err != nil {
		return nil, err
	}
	return mmp.GetUserByLogin(login)
}

func getHash(user *model.User, passwd string) string {
	return common.Hash(user.Salt + passwd)
}

func (as *auth_service) CreateUser(user *model.User) error {
	if !loginRegexp.MatchString(user.Login) {
		return common.NewError(common.ERR_INVALID_VAL, "Invalid login="+user.Login+", expected string up to 40 chars long.")
	}

	mmp, err := as.Persister.GetMainTx()
	if err != nil {
		return err
	}

	// No password for the new user
	user.Salt = common.NewSecretKey(16)
	user.Hash = getHash(user, "")
	as.logger.Info("Inserting new user ", user)
	err = mmp.InsertUser(user)
	return err
}

func (as *auth_service) UpdateUser(usr *model.User) error {
	mmp, err := as.Persister.GetMainTx()
	if err != nil {
		return err
	}

	err = mmp.Begin()
	if err != nil {
		return err
	}
	defer mmp.Commit()

	user, err := mmp.GetUserByLogin(usr.Login)
	if err != nil {
		return err
	}

	user.Email = usr.Email
	return mmp.UpdateUser(user)
}

func (as *auth_service) SetUserPasswd(login, passwd string) error {
	as.logger.Info("Changing user password for ", login)
	mmp, err := as.Persister.GetMainTx()
	if err != nil {
		return err
	}
	err = mmp.Begin()
	if err != nil {
		return err
	}
	defer mmp.Commit()

	user, err := mmp.GetUserByLogin(login)
	if err != nil {
		return err
	}

	user.Hash = getHash(user, passwd)
	return mmp.UpdateUser(user)
}

func (as *auth_service) UpdateUserRoles(orgId int64, login string, urs []*model.UserRole) error {
	mmp, err := as.Persister.GetMainTx()
	if err != nil {
		return err
	}

	err = mmp.Begin()
	if err != nil {
		return err
	}
	defer mmp.Commit()

	err = mmp.DeleteUserRoles(&model.UserRoleQuery{OrgId: orgId, Login: login})
	if err != nil {
		mmp.Rollback()
		return err
	}
	return mmp.InsertUserRoles(urs)
}

func (as *auth_service) GetUserRoles(orgId int64, login string) ([]*model.UserRole, error) {
	mmp, err := as.Persister.GetMainTx()
	if err != nil {
		return nil, err
	}

	return mmp.FindUserRoles(&model.UserRoleQuery{OrgId: orgId, Login: login})
}
