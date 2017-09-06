package auth

type (
	Context interface {
		// must be org admin, or fail
		AuthZOrgAdmin(orgId int64) error
		// must have the level
		AuthZHasOrgLevel(orgId int64, level AZLevel) error
		// must be the user
		AuthZUser(userLogin string) error

		// returns authenticated user login or "" if not authenticated
		UserLogin() string
	}
)
