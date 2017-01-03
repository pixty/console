package service

import "testing"
import "github.com/pixty/console/common"
import "os"
import "path/filepath"
import "strings"
import "fmt"
import "reflect"
import "bytes"
import "io"

func TestDiInit(t *testing.T) {
	lbs := NewLfsBlobStorage()
	lbs.Config = &common.ConsoleConfig{}
	if err := lbs.DiInit(); err == nil {
		t.Fatal("Should fail due to no storage dir")
	}

	lbs.Config.LbsDir = getUniqueDir()
	defer removeDir(lbs.Config.LbsDir)

	if common.DoesFileExist(lbs.Config.LbsDir) {
		t.Fatal("The folder " + lbs.Config.LbsDir + " should not exist yet")
	}

	if err := lbs.DiInit(); err != nil {
		t.Fatal("Should be successfull")
	}

	if !common.DoesFileExist(lbs.Config.LbsDir) {
		t.Fatal("The folder " + lbs.Config.LbsDir + " should exist after init")
	}

	if common.DoesFileExist(lbs.metaFN) {
		t.Fatal("The file " + lbs.metaFN + " should not exist after init")
	}

	lbs.DiShutdown()

	if !common.DoesFileExist(lbs.metaFN) {
		t.Fatal("The file " + lbs.metaFN + " should exist after shutdown")
	}
}

func TestRWCycle(t *testing.T) {
	lbs := initLbs()

	defer removeDir(lbs.storeDir)

	meta := common.NewBlobMeta()
	meta.KVPairs["key"] = "val"
	data := "test value"
	r := strings.NewReader(data)
	id, _ := lbs.Add(r, meta)

	r2, meta2 := lbs.Read(id)
	if r2 == nil {
		t.Fatal("Data should be found")
	}

	if !reflect.DeepEqual(meta, meta2) {
		t.Fatal("Meta meta=" + meta.String() + ", meta2=" + meta2.String() + "doesn't look same")
	}

	data2 := readString(r2)
	if strings.Compare(data2, data) != 0 {
		t.Fatal("data2=\"" + data2 + "\" is not same like \"" + data + "\"")
	}

	lbs.DiShutdown()
}

func TestRWMetaNil(t *testing.T) {
	lbs := initLbs()

	defer removeDir(lbs.storeDir)

	data := "test value"
	r := strings.NewReader(data)
	id, _ := lbs.Add(r, nil)

	r2, meta2 := lbs.Read(id)
	if r2 == nil {
		t.Fatal("Data should be found")
	}

	if meta2 != nil {
		t.Fatal("Meta meta2=" + meta2.String() + "doesn't look it's equal to nil")
	}

	data2 := readString(r2)
	if strings.Compare(data2, data) != 0 {
		t.Fatal("data2=\"" + data2 + "\" is not same like \"" + data + "\"")
	}

	lbs.DiShutdown()
}

func TestStandartCycle(t *testing.T) {
	lbs := initLbs()

	defer removeDir(lbs.storeDir)

	meta := common.NewBlobMeta()
	meta.KVPairs["key"] = "val"
	data := "test value"
	r := strings.NewReader(data)
	id, _ := lbs.Add(r, meta)

	lbs.DiShutdown()

	lbs2 := NewLfsBlobStorage()
	lbs2.Config = lbs.Config
	lbs2.DiInit()

	r2, meta2 := lbs2.Read(id)
	if r2 == nil {
		t.Fatal("Data should be found")
	}

	if !reflect.DeepEqual(meta, meta2) {
		t.Fatal("Meta meta=" + meta.String() + ", meta2=" + meta2.String() + "doesn't look same")
	}

	data2 := readString(r2)
	if strings.Compare(data2, data) != 0 {
		t.Fatal("data2=\"" + data2 + "\" is not same like \"" + data + "\"")
	}

	lbs2.DiShutdown()

}

func TestDelete(t *testing.T) {
	lbs := initLbs()

	defer removeDir(lbs.storeDir)

	meta := common.NewBlobMeta()
	meta.KVPairs["key"] = "val"
	data := "test value"
	r := strings.NewReader(data)
	id, _ := lbs.Add(r, meta)

	if lbs.Delete(id) != nil {
		t.Fatal("Should delete")
	}

	if lbs.Delete(id) == nil {
		t.Fatal("Should be deleted")
	}

	lbs.DiShutdown()
}

func initLbs() *LfsBlobStorage {
	lbs := NewLfsBlobStorage()
	lbs.Config = &common.ConsoleConfig{}
	lbs.Config.LbsDir = getUniqueDir()
	if err := lbs.DiInit(); err != nil {
		panic("DiInit() Should be successfull")
	}
	return lbs
}

func readString(stream io.Reader) string {
	buf := new(bytes.Buffer)
	buf.ReadFrom(stream)
	return buf.String()
}

func getUniqueDir() string {
	uuid := common.NewUUID()
	d1 := uuid[0 : len(uuid)/2]
	d2 := uuid[len(uuid)/2 : len(uuid)]
	result := filepath.Join(os.TempDir(), d1, d2)
	fmt.Printf("getUniqueDir(): %s\n", result)
	return result
}

func removeDir(dir2remove string) {
	dirs := strings.Split(dir2remove, fmt.Sprintf("%c", os.PathSeparator))
	dir := filepath.Join(os.TempDir(), dirs[len(dirs)-2])
	fmt.Printf("Removing %s, actually will be %s removed\n", dir2remove, dir)
	os.RemoveAll(dir)
}
