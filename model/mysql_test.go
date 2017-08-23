package model

import (
	"testing"

	"github.com/pixty/console/common"
)

func initMysqlPersister() *MysqlPersister {
	mp := NewMysqlPersister()
	mp.Config = common.NewConsoleConfig()
	mp.Config.MysqlDatasource = "pixty@/pixty_test?charset=utf8"
	mp.DiInit()

	pp := mp.GetPartPersister("ttt")
	pp.ExecQuery("DROP DATABASE pixty_test")
	pp.ExecScript("scheme.sql")
	return mp
}

func TestFacePutGet(t *testing.T) {
	mp := initMysqlPersister()

	pp := mp.GetPartPersister("ttt")
	f := new(Face)
	f.V128D = common.NewV128D()
	f.V128D.FillRandom()
	id, err := pp.InsertFace(f)
	if err != nil {
		t.Fatal("Fail when inserting face, err=", err)
	}

	t.Log("Created new face, id=", id, ", vec=", f.V128D)
	ff, err := pp.GetFaceById(id)
	if err != nil {
		t.Fatal("Fail when getting face, err=", err)
	}
	t.Log("Read face, id=", id, ", vec=", ff.V128D)

	if !ff.V128D.Equals(f.V128D) {
		t.Fatal("Vectors are not equal!")
	}

	ff, err = pp.GetFaceById(id + 1)
	if ff != nil || err != nil {
		t.Fatal("Unexpected result for pp.GetFaceById(id + 1)")
	}
}

func TestFacePutGetMany(t *testing.T) {
	mp := initMysqlPersister()
	pp := mp.GetPartPersister("ttt")

	f1 := new(Face)
	f1.PersonId = "P1"
	f1.V128D = common.NewV128D()
	f1.V128D.FillRandom()

	f2 := new(Face)
	f2.PersonId = "P2"
	f2.V128D = common.NewV128D()
	f2.V128D.FillRandom()

	f3 := new(Face)
	f3.PersonId = "P1"
	f3.V128D = common.NewV128D()
	f3.V128D.FillRandom()

	err := pp.InsertFaces([]*Face{f1, f2, f3})
	if err != nil {
		t.Fatal("Fail when inserting face, err=", err)
	}

	t.Log("Created new faces reading P2 now")
	faces, err := pp.FindFaces(&FacesQuery{PersonId: "P2"})
	if err != nil {
		t.Fatal("Fail when getting face, err=", err)
	}
	t.Log("Read faces, len=", len(faces))

	if len(faces) != 1 {
		t.Fatal("Wrong result, expected 1 element, but actually ", len(faces))
	}

	if !faces[0].V128D.Equals(f2.V128D) {
		t.Fatal("Result vector and f2 vector are not equal")
	}

	if f1.V128D.Equals(f2.V128D) != f1.V128D.Equals(faces[0].V128D) {
		t.Fatal("Insert doesn't store vector properly")
	}

	faces, err = pp.FindFaces(&FacesQuery{})
	if err != nil {
		t.Fatal("Fail when getting faces, err=", err)
	}
	if len(faces) != 3 {
		t.Fatal("Wrong result, expected 3 elements, but actually ", len(faces), " ", faces)
	}

	faces, err = pp.FindFaces(&FacesQuery{Limit: 2})
	if err != nil {
		t.Fatal("Fail when getting faces, err=", err)
	}
	if len(faces) != 2 {
		t.Fatal("Wrong result, expected 2 elements, but actually ", faces)
	}

}
