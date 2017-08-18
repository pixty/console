package model

import (
	"strconv"
	"testing"

	"github.com/jrivets/log4g"
	"github.com/pixty/console/common"
)

func initMongoPersister(t *testing.T) *MongoPersister {
	mp := NewMongoPersister()
	mp.Config = common.NewConsoleConfig()
	mp.Config.MongoDatabase = "__pixty__test__"

	log4g.SetLogLevel("pixty.mongo", log4g.DEBUG)

	if err := mp.DiInit(); err != nil {
		t.Fatal("Could not initialize ", err)
		return nil
	}
	return mp
}

func TestGetPersonsById(t *testing.T) {
	mp := initMongoPersister(t)
	log := log4g.GetLogger("pixty.mongo")

	// for the test only
	defer mp.main_mds.dropDb()

	tx := mp.NewPartPersister(common.Id("234"))
	createPersons(1, tx, 5)

	pd := tx.GetPersonDAO()
	id := common.Id("Person-3")
	p := &common.Person{LastSeenAt: 12341435}
	err := pd.Read(id, p)
	if err != nil {
		t.Fatal("Could not find err=", err)
	}

	log.Info("Read p=", p)

	err = pd.Update(p.Id, p)
	if err != nil {
		t.Fatal("Could not update, err=", err)
	}

	err = pd.Delete(id)
	if err != nil {
		t.Fatal("Could not delete err=", err)
	}

	if pd.Read(id, p) == nil {
		t.Fatal(id, " should be deleted.")
	}
}

func TestSearchPersons(t *testing.T) {
	mp := initMongoPersister(t)
	//	log := log4g.GetLogger("pixty.mongo.TestSearchPersons")
	// for the test only
	defer mp.main_mds.dropDb()
	tx := mp.NewPartPersister(common.Id("234"))
	ids1 := createPersons(1, tx, 5)
	searchPersonsByIds(t, tx, []common.Id{"Person-2", "Person-4", "Person-asdf"}, 2)
	searchPersonsByIds(t, tx, ids1, 5)
	ids2 := createPersons(6, tx, 5)
	searchPersonsByIds(t, tx, ids2, 5)
	ids3 := append(ids1, ids2...)
	searchPersonsByIds(t, tx, ids3, 10)
	ids3 = append(ids3, ids2...)
	searchPersonsByIds(t, tx, ids3, 10)

	search3Persons(t, tx, &common.PersonsQuery{CamId: "abc", MaxLastSeenAt: 10000, Limit: 10}, 0)
	search3Persons(t, tx, &common.PersonsQuery{MaxLastSeenAt: 10000, Limit: 10}, 10)
	search3Persons(t, tx, &common.PersonsQuery{MaxLastSeenAt: 5, Limit: 10}, 5)
	search3Persons(t, tx, &common.PersonsQuery{CamId: "cam-1", MaxLastSeenAt: 5, Limit: 2}, 2)
	search3Persons(t, tx, &common.PersonsQuery{CamId: "cam-11", MaxLastSeenAt: 5, Limit: 2}, 0)
}

func createPersons(startId int, tx common.PartPersister, persons int) []common.Id {
	objs := make([]interface{}, 0, 10)
	res := make([]common.Id, 0, 10)
	log := log4g.GetLogger("pixty.mongo")

	col := tx.GetPersonDAO()

	for ; persons > 0; persons-- {
		p := &common.Person{CamId: "cam-1", LastSeenAt: common.Timestamp(startId)}
		p.Id = common.Id("Person-" + strconv.Itoa(startId))
		startId += 1
		p.Match = common.Match{Status: common.MTCH_ST_NOT_STARTED, Matches: []common.PersonMatch{common.PersonMatch{ProfileId: "1", Distance: 0.234}}}
		objs = append(objs, p)
		res = append(res, p.Id)
	}

	err := col.CreateMany(objs...)
	if err != nil {
		log.Error("Something goes wrong: ", err)
		return nil
	}
	log.Info("Create new records: ", res)
	return res
}

func search3Persons(t *testing.T, tx common.PartPersister, q *common.PersonsQuery, expected int) {
	res, err := tx.SearchPersons(q)
	if err != nil {
		t.Fatal("Failed with error=" + err.Error())
		return
	}

	if len(res) != expected {
		t.Fatal("Expected " + strconv.Itoa(expected) + " but found " + strconv.Itoa(len(res)))
	}

	if expected > 1 {
		ts := q.MaxLastSeenAt
		for _, p := range res {
			if p.LastSeenAt > ts {
				t.Fatalf("Person %v comes inorder of expected ts=%d", p, ts)
			}
			ts = p.LastSeenAt
		}
	}
}

func searchPersonsByIds(t *testing.T, tx common.PartPersister, idsSearch []common.Id, expected int) {
	ids := append([]common.Id(nil), idsSearch...)
	res, err := tx.SearchPersons(&common.PersonsQuery{PersonIds: ids})
	if err != nil {
		t.Fatal("Failed with error=" + err.Error())
		return
	}

	if len(res) != expected {
		t.Fatal("Expected " + strconv.Itoa(expected) + " but found " + strconv.Itoa(len(res)))
	}

	for _, p := range res {
		found := false
		for i, id := range ids {
			if p.Id == id {
				found = true
				ids = append(ids[:i], ids[i+1:]...)
				break
			}
		}

		if !found {
			t.Fatalf("id=%s is not found in requested one %v", p.Id, ids)
			return
		}
	}
}
