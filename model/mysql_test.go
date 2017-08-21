package model

import (
	"testing"

	"github.com/pixty/console/common"
)

func TestFacePutGet(t *testing.T) {
	mp := NewMysqlPersister()
	mp.Config = common.NewConsoleConfig()
	mp.Config.MysqlDatasource = "pixty@/pixty_test?charset=utf8"
	mp.DiInit()

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
