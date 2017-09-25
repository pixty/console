package scene

import (
	"sync"
	"time"

	"github.com/jrivets/gorivets"
	"github.com/pixty/console/model"
)

type (
	persons_cache struct {
		lock  sync.Mutex
		cache gorivets.LRU
	}

	person_desc struct {
		// the flag indicates that the person is new and first time seen
		// in the system. The flag is changed by scene processor when it checks
		// the person availability in the DB
		newPerson bool
		lastFace  *model.Face
		faces     int
	}
)

func new_persons_cache(cache_ttl time.Duration) *persons_cache {
	pc := new(persons_cache)
	pc.cache = gorivets.NewTtlLRU(10000, cache_ttl, nil)
	return pc
}

func (pc *persons_cache) should_be_added(face *model.Face) bool {
	pc.lock.Lock()
	defer pc.lock.Unlock()

	var pd *person_desc
	inf, ok := pc.cache.Get(face.PersonId)
	if !ok {
		// this is the first time we met the face for the LRU TTL, so add it no problem
		pd = new(person_desc)
		pc.cache.Add(face.PersonId, pd, 1)
	} else {
		pd = inf.(*person_desc)
	}

	return pd.addIfNeeded(face)
}

func (pc *persons_cache) mark_person_as_new(personId string) {
	pc.lock.Lock()
	defer pc.lock.Unlock()

	inf, ok := pc.cache.Peek(personId)
	if ok {
		pd := inf.(*person_desc)
		pd.newPerson = true
	}
}

func (pd *person_desc) newFace(face *model.Face) {
	pd.lastFace = face
	pd.faces++
}

func (pd *person_desc) addIfNeeded(face *model.Face) bool {
	if pd.faces == 0 || (pd.newPerson && pd.faces < 3) {
		pd.newFace(face)
		return true
	}

	// hardcoded 1 minute diff
	if pd.faces > 5 && face.CapturedAt-pd.lastFace.CapturedAt < 60000 {
		return false
	}

	if face.Rect.Area() > pd.lastFace.Rect.Area() {
		pd.newFace(face)
		return true
	}

	return false
}
