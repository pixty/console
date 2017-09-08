package storage

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/pixty/console/common"
)

func TestRWCycle(t *testing.T) {
	lbs := initLbs()
	defer lbs.Shutdown()
	defer removeDir(lbs.storeDir)

	meta := newBlobMeta()
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

	r2, _ := lbs.Read(id)
	if r2 == nil {
		t.Fatal("Data should be found")
	}

	data2 := readString(r2)
	if strings.Compare(data2, data) != 0 {
		t.Fatal("data2=\"" + data2 + "\" is not same like \"" + data + "\"")
	}
}

func TestStandartCycle(t *testing.T) {
	lbs := initLbs()
	defer removeDir(lbs.storeDir)

	meta := newBlobMeta()
	data := "test value"
	r := strings.NewReader(data)
	id, _ := lbs.Add(r, meta)

	lbs.Shutdown()

	lbs2 := NewLfsBlobStorage(lbs.storeDir, 1000000000)

	r2, meta2 := lbs2.Read(id)
	if r2 == nil {
		t.Fatal("Data should be found")
	}

	if meta.Id != meta2.Id || meta.Size != meta2.Size || compBlobMeta(meta, meta2) != 0 {
		t.Fatal("Meta meta=" + meta.String() + ", meta2=" + meta2.String() + "doesn't look same")
	}

	data2 := readString(r2)
	if strings.Compare(data2, data) != 0 {
		t.Fatal("data2=\"" + data2 + "\" is not same like \"" + data + "\"")
	}

	lbs2.Shutdown()

	// remove .meta
	os.Remove(filepath.Join(lbs.storeDir, cMetaFileName))
	lbs3 := NewLfsBlobStorage(lbs.storeDir, 1000000000)

	r2, meta2 = lbs3.Read(id)
	if r2 == nil {
		t.Fatal("Data should be found")
	}

	// Time will be probably lost, don't check it
	if meta.Id != meta2.Id || meta.Size != meta2.Size {
		t.Fatal("Meta meta=" + meta.String() + ", meta2=" + meta2.String() + "doesn't look same")
	}

	data2 = readString(r2)
	if strings.Compare(data2, data) != 0 {
		t.Fatal("data2=\"" + data2 + "\" is not same like \"" + data + "\"")
	}

	lbs3.Shutdown()

}

func TestSize(t *testing.T) {
	lbs := NewLfsBlobStorage(getUniqueDir(), 15)

	defer removeDir(lbs.storeDir)

	meta := newBlobMeta()
	data := "0123456789"
	r := strings.NewReader(data)
	id, _ := lbs.Add(r, meta)

	if lbs.ReadMeta(id) == nil {
		t.Fatal("Should Be there")
	}

	meta = newBlobMeta()
	data = "0123456789"
	r = strings.NewReader(data)
	id2, _ := lbs.Add(r, meta)

	if lbs.ReadMeta(id) != nil {
		t.Fatal("Should Be dropped")
	}
	if lbs.ReadMeta(id2) == nil {
		t.Fatal("Should Be there id2")
	}

	if lbs.Delete(id2) != nil {
		t.Fatal("id2 should be safely deleted")
	}

	lbs.Shutdown()
}

func initLbs() *LfsBlobStorage {
	lbs := NewLfsBlobStorage(getUniqueDir(), 1000000000)
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
