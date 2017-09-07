package auth

type (
	Context interface {
		// must me superadmin
		AuthZSuperadmin() error
		// must be org admin, or fail
		AuthZOrgAdmin(orgId int64) error
		// must have the level
		AuthZHasOrgLevel(orgId int64, level AZLevel) error
		// must be the user
		AuthZUser(userLogin string) error

		// user is authorized to access the camera
		AuthZCamAccess(camId string, lvl AZLevel) error

		// returns authenticated user login or "" if not authenticated
		UserLogin() string
	}
)
