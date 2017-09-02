package auth

import (
	"github.com/pixty/console/model"
)

type (
	AZLevel int

	AuthService interface {
		AuthN(user, passwd string) (bool, error)
		AuthZ(user string, orgId int64) AZLevel

		CreateUser(user *model.User) error
		UpdateUser(user *model.User) error
		SetUserPasswd(user, passwd string) error
		InsertUserRoles(sessUsr string, urs []*model.UserRole) error
		DeleteUserRoles(sessUsr string, urs []*model.UserRole) error
		GetUserRoles(sessUsr, user string) ([]*model.UserRole, error)
	}
)

const (
	// Authorization levels:
	// SA - superadmin
	// OA - org admin
	// OU - org user
	// NA - Not available
	AUTHZ_LEVEL_SA = 15
	AUTHZ_LEVEL_OA = 10
	AUTHZ_LEVEL_OU = 5
	AUTHZ_LEVEL_NA = 0
)
