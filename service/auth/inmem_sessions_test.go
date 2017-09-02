package auth

import (
	"testing"
	"time"

	"github.com/jrivets/log4g"
	"github.com/pixty/console/common"
)

func TestSessionTimeout(t *testing.T) {
	sis := NewInMemSessionService().(*sess_inmem_service)
	log4g.SetLogLevel(sis.logger.GetName(), log4g.DEBUG)
	sis.Config = common.NewConsoleConfig()
	sis.DiPostConstruct()
	ttl := 50 * time.Millisecond
	sis.initSessionCache(100, ttl)

	sd, _ := sis.NewSession("user")
	if sd == nil {
		t.Fatal("Oops, could not create session")
	}

	stopTime := time.Now().Add(ttl + ttl/10)
	for time.Now().Before(stopTime) {
		sd2 := sis.GetBySession(sd.Session())
		if sd2 == nil || sd2.Session() != sd.Session() || sd2.User() != sd.User() {
			t.Fatal("Oops, got nil, or wrong object ", sd2)
		}
	}

	is := (*sess_inmem_service)(sis)
	if len(is.usrCache) != 1 || is.sessCache.Len() != 1 {
		t.Fatal("Caches must contain 1 session")
	}
	time.Sleep(ttl + 1)
	sd2 := sis.GetBySession(sd.Session())
	if sd2 != nil {
		t.Fatal("Oops, got wrong object (nil is expected) ", sd2)
	}

	if len(is.usrCache) > 0 || is.sessCache.Len() > 0 {
		t.Fatal("Caches must me empty")
	}
}

func TestMaxSessionsLimit(t *testing.T) {
	sis := NewInMemSessionService().(*sess_inmem_service)
	sis.Config = common.NewConsoleConfig()
	sis.Config.AuthMaxSessions = 1
	sis.DiPostConstruct()
	sd, _ := sis.NewSession("user")
	if sd == nil {
		t.Fatal("Oops, could not create session")
	}
	sd, _ = sis.NewSession("user")
	if sd != nil {
		t.Fatal("Oops, must not create session")
	}
}

func TestDeleteAllSessions(t *testing.T) {
	sis := NewInMemSessionService().(*sess_inmem_service)
	sis.Config = common.NewConsoleConfig()
	sis.Config.AuthMaxSessions = 3
	sis.DiPostConstruct()
	sd1, _ := sis.NewSession("user")
	if sd1 == nil {
		t.Fatal("Oops, could not create session1")
	}

	sd2, _ := sis.NewSession("user")
	if sd2 == nil {
		t.Fatal("Oops, could not create session2")
	}

	sd3, _ := sis.NewSession("user")
	if sd3 == nil {
		t.Fatal("Oops, could not create session3")
	}

	sis.DeleteAllSessions("user")
	if sis.GetBySession(sd1.Session()) != nil || sis.GetBySession(sd2.Session()) != nil ||
		sis.GetBySession(sd3.Session()) != nil {
		t.Fatal("Must clean up all sessions")
	}
}
