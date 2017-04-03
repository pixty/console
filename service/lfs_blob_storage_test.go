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

func TestRWCycle(t *testing.T) {
	lbs := initLbs()
	defer lbs.Shutdown()
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

}

func TestRWMetaNil(t *testing.T) {
	lbs := initLbs()
	defer lbs.Shutdown()
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
}

func TestStandartCycle(t *testing.T) {
	lbs := initLbs()
	defer removeDir(lbs.storeDir)

	meta := common.NewBlobMeta()
	meta.KVPairs["key"] = "val"
	data := "test value"
	r := strings.NewReader(data)
	id, _ := lbs.Add(r, meta)

	lbs.Shutdown()

	lbs2 := NewLfsBlobStorage(lbs.storeDir)

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

	lbs2.Shutdown()
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

	lbs.Shutdown()
}

func initLbs() *LfsBlobStorage {
	lbs := NewLfsBlobStorage(getUniqueDir())
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
