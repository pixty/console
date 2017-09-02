package auth

import "time"

type (
	SessionDesc interface {
		User() string
		Session() string
		Since() time.Time
	}

	SessionService interface {
		NewSession(user string) (SessionDesc, error)
		GetBySession(session string) SessionDesc
		DeleteSesion(session string) SessionDesc
		DeleteAllSessions(user string) []SessionDesc
	}
)
