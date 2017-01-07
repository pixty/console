package model

import "testing"
import "github.com/pixty/console/common"
import "github.com/jrivets/log4g"
import "strconv"

func TestGetLatestScene(t *testing.T) {
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

	createScene(mp, 5, "cam1")
	createScene(mp, 2, "cam2")

	tx := mp.NewTxPersister()
	checkLastScene(t, tx, "cam1", 5)
	checkLastScene(t, tx, "cam2", 2)

	createScene(mp, 1, "cam1")
	checkLastScene(t, tx, "cam1", 1)
	tx.Close()
}

func createScene(mp *MongoPersister, persons int, camId common.Id) {
	ts := common.CurrentTimestamp()
	objs := make([]interface{}, 0, 10)
	tx := mp.NewTxPersister()
	defer tx.Close()

	for ; persons > 0; persons-- {
		p := &common.PersonLog{CamId: camId, SceneTs: ts, OrgId: "org"}
		objs = append(objs, p)
	}
	err := tx.GetCrudExecutor(common.STGE_PERSON_LOG).CreateMany(objs...)
	if err != nil {
		mp.logger.Error("Error: ", err)
	}
}

func checkLastScene(t *testing.T, tx common.TxPersister, camId common.Id, expected int) {
	res, err := tx.GetLatestScene(camId)
	if err != nil {
		t.Fatal("Failed with error=" + err.Error())
		return
	}

	if len(res) != expected {
		t.Fatal("Expected " + strconv.Itoa(expected) + " but found " + strconv.Itoa(len(res)))
	}

	for _, p := range res {
		if p.CamId != camId {
			t.Fatalf("Expected camId=%s, but received %v", camId, p)
		}
	}
}
