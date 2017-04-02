package model

import (
	"strconv"
	"testing"

	"github.com/jrivets/log4g"
	"github.com/pixty/console/common"
	"gopkg.in/mgo.v2/bson"
)

func TestGetPersonsById(t *testing.T) {
	mp := NewMongoPersister()
	mp.Config = common.NewConsoleConfig()
	mp.Config.MongoDatabase = "__pixty__test__"

	log4g.SetLogLevel("console.mongo", log4g.DEBUG)

	if err := mp.DiInit(); err != nil {
		t.Fatal("Could not initialize ", err)
		return
	}

	// for the test only
	defer mp.dropDatabase()

	tx := mp.NewTxPersister()
	ids1 := createPersons(tx, 5)
	findPersons(t, tx, ids1, 5)
	ids2 := createPersons(tx, 5)
	findPersons(t, tx, ids2, 5)
	ids3 := append(ids1, ids2...)
	findPersons(t, tx, ids3, 10)
	ids3 = append(ids3, ids2...)
	findPersons(t, tx, ids3, 10)
}

func createPersons(tx common.TxPersister, persons int) []common.Id {
	objs := make([]interface{}, 0, 10)
	res := make([]common.Id, 0, 10)

	col := tx.GetCrudExecutor(common.STGE_PERSON)

	for ; persons > 0; persons-- {
		p := &common.Person{}
		p.Id = common.Id(bson.NewObjectId().Hex())
		p.ProfileId = "1234"
		objs = append(objs, p)
		res = append(res, p.Id)
	}

	err := col.CreateMany(objs...)
	if err != nil {
		log4g.GetLogger("console.mongo").Error("Something goes wrong: ", err)
		return nil
	}
	return res
}

func findPersons(t *testing.T, tx common.TxPersister, ids []common.Id, expected int) {
	res, err := tx.FindPersonsByIds(ids...)
	if err != nil {
		t.Fatal("Failed with error=" + err.Error())
		return
	}

	if len(res) != expected {
		t.Fatal("Expected " + strconv.Itoa(expected) + " but found " + strconv.Itoa(len(res)))
	}

	for _, p := range res {
		found := false
		for _, id := range ids {
			if p.Id == id {
				found = true
				break
			}
		}

		if !found {
			t.Fatalf("id=%s is not found in requested one %v", p.Id, ids)
		}
	}
}
