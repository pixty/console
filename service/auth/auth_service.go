package auth

import (
	"fmt"
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

func PasswdHash(user *model.User, passwd string) string {
	return common.Hash(user.Salt + passwd)
}

func NewAuthService() AuthService {
	as := new(auth_service)
	as.logger = log4g.GetLogger("pixty.auth")
	as.urCache = gorivets.NewTtlLRU(1000, time.Minute*10, nil)
	return as
}

func (azl AZLevel) String() string {
	switch azl {
	case AUTHZ_LEVEL_SA:
		return "superadmin"
	case AUTHZ_LEVEL_OA:
		return "orgadmin"
	case AUTHZ_LEVEL_OU:
		return "orguser"
	case AUTHZ_LEVEL_NA:
		return "notassigned"
	default:
		return "unknown"
	}
}

func AZLevelParse(lvl string) AZLevel {
	switch lvl {
	case "superadmin":
		return AUTHZ_LEVEL_SA
	case "orgadmin":
		return AUTHZ_LEVEL_OA
	case "orguser":
		return AUTHZ_LEVEL_OU
	case "notassigned":
		return AUTHZ_LEVEL_NA
	default:
		return AUTHZ_LEVEL_UNKNWN
	}
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

	return (PasswdHash(usr, passwd) == usr.Hash), nil
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
