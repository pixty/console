package auth

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/jrivets/gorivets"
	"github.com/jrivets/log4g"
	"github.com/pixty/console/common"
)

type (
	sess2desc map[string]*sess_inmem

	sess_inmem struct {
		user    string
		session string
		since   time.Time
	}

	sess_inmem_service struct {
		Config *common.ConsoleConfig `inject:""`
		// main cache keeps session -> *sess_inmem relation in LRU
		sessCache gorivets.LRU
		// supplementary cache keeps user -> sess2desc relations
		usrCache map[string]sess2desc
		lock     sync.Mutex
		logger   log4g.Logger
	}
)

func NewInMemSessionService() SessionService {
	sis := new(sess_inmem_service)
	sis.logger = log4g.GetLogger("pixty.sess_inmem_service")
	return sis
}

// Construct the object when all DI is done
func (sis *sess_inmem_service) DiPostConstruct() {
	sessTTL := time.Second * time.Duration(sis.Config.AuthSessionTOSec)

	sis.logger.Info("Initializing...")

	// New LRU sessions cache. Limit it by size just for good styling.
	// We are in trouble if it is exceeded :)
	sis.initSessionCache(10000, sessTTL)
	sis.usrCache = make(map[string]sess2desc)
}

func (sis *sess_inmem_service) initSessionCache(size int64, ttl time.Duration) {
	sis.logger.Info("Creating LRU storage with ", size, " elerments and ", ttl, " duration")
	sis.sessCache = gorivets.NewTtlLRU(size, ttl, sis.onSessionRemoved)
}

func (sis *sess_inmem_service) NewSession(user string) (SessionDesc, error) {
	sis.lock.Lock()
	defer sis.lock.Unlock()

	s2d, ok := sis.usrCache[user]
	if !ok {
		s2d = make(sess2desc)
		sis.usrCache[user] = s2d
	}

	maxSess := sis.Config.AuthMaxSessions
	if len(s2d) >= maxSess && maxSess > 0 {
		sis.logger.Warn("Number of opened sessions for ", user, " reaches maximum(", maxSess, "), don't allow to open new one.")
		return nil, common.NewError(common.ERR_LIMIT_VIOLATION,
			"The number of opened session for the user "+user+" exceeds the maximum("+strconv.Itoa(sis.Config.AuthMaxSessions))
	}

	sessId := sis.newSessionId()
	sd := new(sess_inmem)
	sd.user = user
	sd.session = sessId
	sd.since = time.Now()

	sis.logger.Info("New session ", sd)
	s2d[sessId] = sd
	sis.sessCache.Add(sessId, sd, 1)
	return sd, nil
}

func (sis *sess_inmem_service) GetBySession(session string) SessionDesc {
	sis.lock.Lock()
	defer sis.lock.Unlock()

	sis.sessCache.Sweep()
	inf, _ := sis.sessCache.Get(session)
	if inf == nil {
		return nil
	}
	return inf.(SessionDesc)
}

func (sis *sess_inmem_service) DeleteSesion(session string) SessionDesc {
	sis.lock.Lock()
	defer sis.lock.Unlock()

	return sis.sessCache.Delete(session).(SessionDesc)
}

func (sis *sess_inmem_service) DeleteAllSessions(user string) []SessionDesc {
	sis.lock.Lock()
	defer sis.lock.Unlock()

	sis.sessCache.Sweep()
	s2d, ok := sis.usrCache[user]
	res := []SessionDesc{}
	if ok {
		for sess, sd := range s2d {
			sis.sessCache.DeleteWithCallback(sess, false)
			res = append(res, sd)
		}
		delete(sis.usrCache, user)
	}
	sis.logger.Info("ALL sessions for user=", user, " were deleted: ", res)
	return res
}

func (sis *sess_inmem_service) newSessionId() string {
	return common.NewSession()
}

func (sis *sess_inmem_service) onSessionRemoved(k, v interface{}) {
	sessId := k.(string)
	sd := v.(*sess_inmem)
	user := sd.user
	sis.logger.Info("Callback: The session ", sd, " is closed or removed from LRU.")
	s2d, ok := sis.usrCache[user]
	if !ok {
		sis.logger.Error("The session ", sessId, " could not be found in usrCache! Something is wrong!")
		return
	}
	delete(s2d, sessId)
	if len(s2d) == 0 {
		delete(sis.usrCache, user)
		sis.logger.Debug("No session for ", user, " anymore")
	}
}

// ============================ SessionDesc ==================================
func (sd *sess_inmem) User() string {
	return sd.user
}

func (sd *sess_inmem) Session() string {
	return sd.session
}

func (sd *sess_inmem) Since() time.Time {
	return sd.since
}

func (sd *sess_inmem) String() string {
	return fmt.Sprint("{User=", sd.user, ", SessId=", sd.session, ", Started=", sd.since.Format("01-02 15:04:05.000"))
}
