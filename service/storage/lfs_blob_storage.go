package storage

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/jrivets/gorivets"
	"github.com/jrivets/log4g"
	"github.com/pixty/console/common"
)

const cMetaFileName = ".meta"

type LfsBlobStorage struct {
	logger   log4g.Logger
	objects  map[string]*BlobMeta
	lru      gorivets.LRU
	metaFN   string
	storeDir string
	rwLock   sync.RWMutex
	lastSave time.Time
}

func NewLfsBlobStorage(storeDir string, maxSize int64) *LfsBlobStorage {
	logger := log4g.GetLogger("pixty.service.LfsBlobStorage")
	result := &LfsBlobStorage{logger: logger, storeDir: storeDir}
	result.lru = gorivets.NewLRU(maxSize, result.onLRUDelete)
	err := result.init()
	if err != nil {
		panic(err)
	}
	return result
}

func newBlobMeta() *BlobMeta {
	return &BlobMeta{"", time.Now(), 0}
}

func (bm *BlobMeta) String() string {
	return fmt.Sprintf("{Id: ", bm.Id, ", ts=", bm.Timestamp.Format("01-02-2017 12:13:43"), ", size=", bm.Size, "}")
}

// ============================= LifeCycler ==================================
func (lbs *LfsBlobStorage) DiPhase() int {
	return common.CMP_PHASE_BLOB_STORE
}

func (lbs *LfsBlobStorage) DiInit() error {
	return nil
}

func (lbs *LfsBlobStorage) DiShutdown() {
	lbs.Shutdown()
}

func (lbs *LfsBlobStorage) String() string {
	return fmt.Sprint("LfsBlobStorage:{objects=", lbs.lru.Len(), ", Size=", lbs.lru.Size(), ", storeDir=", lbs.storeDir, "}")
}

func (lbs *LfsBlobStorage) init() error {
	lbs.rwLock.Lock()
	defer lbs.rwLock.Unlock()

	lbs.objects = make(map[string]*BlobMeta)

	path := lbs.storeDir
	lbs.logger.Info("Initializing. Loading data from ", path)
	if !common.DoesFileExist(path) {
		lbs.logger.Info("Could not find directory ", path, " creating new one...")
		err := os.MkdirAll(path, os.ModePerm)
		if err != nil {
			lbs.logger.Error("Could not create storage directory ", path, " got the error=", err)
			return err
		}
	}

	lbs.metaFN = filepath.Join(path, cMetaFileName)
	lbs.storeDir = path
	return lbs.readObjects()
}

func (lbs *LfsBlobStorage) Shutdown() {
	lbs.rwLock.Lock()
	defer lbs.rwLock.Unlock()

	lbs.saveObjects(false)
	lbs.objects = nil
}

// ============================= BlobStorage =================================
func (lbs *LfsBlobStorage) Add(r io.Reader, bMeta *BlobMeta) (string, error) {
	lbs.logger.Debug("Writing ", bMeta)
	defer lbs.logger.Debug("Done with writing ", bMeta)
	if r == nil {
		lbs.logger.Error("reader (r) is nil ")
		return common.ID_NULL, common.NewError(common.ERR_INVALID_VAL, "reader cannot be nil")
	}

	if bMeta == nil {
		bMeta = newBlobMeta()
	}

	id := bMeta.Id
	if id == "" {
		id = common.NewUUID()
	}

	fileName, err := lbs.getFilePath(string(id))
	if err != nil {
		lbs.logger.Error("Could not create path for id=", id)
		return "", err
	}

	file, err := os.Create(fileName)
	if err != nil {
		lbs.logger.Error("Could not create new file ", fileName)
		return "", err
	}
	defer file.Close()

	_, err = io.Copy(file, r)
	if err != nil {
		lbs.logger.Error("Could not copy data to fileName=", fileName, ", err=", err)
		return "", err
	}

	fi, err := os.Stat(fileName)
	if err != nil {
		lbs.logger.Error("Could not obtain fileName=", fileName, " stat err=", err)
		return "", err
	}
	bMeta.Id = id
	bMeta.Size = fi.Size()

	lbs.rwLock.Lock()
	defer lbs.rwLock.Unlock()

	lbs.objects[id] = bMeta
	lbs.logger.Debug("New BLOB with id=", id, " is created. file=", fileName)
	lbs.lru.Add(bMeta.Id, bMeta, bMeta.Size)
	//	lbs.saveObjects(true)
	return id, nil
}

func (lbs *LfsBlobStorage) Read(objId string) (io.ReadCloser, *BlobMeta) {
	fileName, err := lbs.getFilePath(string(objId))
	if err != nil {
		lbs.logger.Error("Could not form path for id=", objId, ", err=", err)
		return nil, nil
	}

	lbs.rwLock.RLock()
	defer lbs.rwLock.RUnlock()

	bMeta, ok := lbs.objects[objId]
	if !ok {
		lbs.logger.Debug("Could not find BLOB by id=", objId)
		return nil, nil
	}

	file, err := os.Open(fileName)
	if err != nil {
		lbs.logger.Warn("Found metadata, but could not find the file with the content. Removing metadata... fileNmae=", fileName, ", id=", objId)
		delete(lbs.objects, objId)
		return nil, nil
	}

	lbs.logger.Debug("Found BLOB id=", objId, " meta=", bMeta)
	return file, bMeta
}

func (lbs *LfsBlobStorage) ReadMeta(objId string) *BlobMeta {
	lbs.rwLock.RLock()
	defer lbs.rwLock.RUnlock()

	bMeta, ok := lbs.objects[objId]
	if !ok {
		lbs.logger.Debug("Could not find BLOB by id=", objId)
	}
	return bMeta
}

func (lbs *LfsBlobStorage) Delete(objIds ...string) error {
	lbs.rwLock.Lock()
	defer lbs.rwLock.Unlock()

	for _, objId := range objIds {
		fileName, err := lbs.getFilePath(objId)
		if err != nil {
			continue
		}

		lbs.logger.Info("Deleting BLOB by id=", objId)
		_, ok := lbs.objects[objId]
		if !ok {
			lbs.logger.Warn("Could not find BLOB by id=", objId)
			continue
		}

		delete(lbs.objects, objId)
		go func() {
			os.Remove(fileName)
		}()
		lbs.lru.DeleteWithCallback(objId, false)
	}
	return nil
}

func (lbs *LfsBlobStorage) DeleteAllWithPrefix(prefix string) int {
	lbs.rwLock.Lock()
	defer lbs.rwLock.Unlock()

	deleted := 0
	toDel := make([]string, 0, 10)
	for id := range lbs.objects {
		if !strings.HasPrefix(id, prefix) {
			continue
		}
		fileName, _ := lbs.getFilePath(id)
		delete(lbs.objects, id)
		toDel = append(toDel, fileName)
		lbs.lru.DeleteWithCallback(id, false)
		deleted++
	}

	go func() {
		for _, fn := range toDel {
			os.Remove(fn)
		}
	}()

	return deleted
}

// =============================== Private ===================================
func compBlobMeta(a, b interface{}) int {
	bm1 := a.(*BlobMeta)
	bm2 := b.(*BlobMeta)
	switch {
	case bm1.Timestamp.Before(bm2.Timestamp):
		return -1
	case bm1.Timestamp.After(bm2.Timestamp):
		return 1
	default:
		return 0
	}
}

func (lbs *LfsBlobStorage) onLRUDelete(k, v interface{}) {
	id := k.(string)
	lbs.logger.Debug("LRU deleting object id=", id)
	fp, err := lbs.getFilePath(id)
	if err != nil {
		lbs.logger.Warn("Could not obtain filePath for id=", id)
	} else {
		go os.Remove(fp)
	}
	delete(lbs.objects, id)
}

func (lbs *LfsBlobStorage) readObjects() error {
	lbs.logger.Info("Reading data objects from ", lbs.storeDir, " ...")
	scanned, err := lbs.scanFolder(lbs.storeDir)
	if err != nil {
		lbs.logger.Error("Stop reading - error while scanning ", err)
		return err
	}

	meta, err := lbs.readMetaFile()
	if err != nil {
		lbs.logger.Warn("Seems like meta file corrupted, will skip it information err=", err)
		meta = make(map[string]*BlobMeta)
	}

	// merge
	merged := 0
	for id, bm := range scanned {
		bmm, ok := meta[id]
		if ok {
			bm.Timestamp = bmm.Timestamp
			merged++
		}
	}

	lbs.logger.Info(len(scanned), " objects found in the store, ", len(meta), " found in meta file, ", merged, " meta-objects were merged.")
	lbs.objects = scanned

	sl, err := gorivets.NewSortedSliceByComp(compBlobMeta, gorivets.Max(len(lbs.objects), 1))
	if err != nil {
		lbs.logger.Error("Could not create new sorted slice, err=", err)
		return err
	}
	for _, v := range lbs.objects {
		sl.Add(v)
	}
	lbs.lru.Clear()
	for i := 0; i < sl.Len(); i++ {
		bm := sl.At(i).(*BlobMeta)
		lbs.lru.Add(bm.Id, bm, bm.Size)
	}

	lbs.logger.Info(len(lbs.objects), " objects in the store ")
	return nil

}

func (lbs *LfsBlobStorage) scanFolder(folder string) (map[string]*BlobMeta, error) {
	lbs.logger.Info("Scanning folder for existing objects ... ")
	res := make(map[string]*BlobMeta)
	scanned := 0
	err := filepath.Walk(folder, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		scanned++
		dir, file := filepath.Split(path)
		ln := len(file)
		if ln < 2 {
			return nil
		}

		suffix := file[ln-2:]
		if strings.HasSuffix(filepath.Base(dir), suffix) {
			bm := new(BlobMeta)
			bm.Id = file
			bm.Size = info.Size()
			bm.Timestamp = info.ModTime()
			res[file] = bm
		}
		return nil
	})

	if err != nil {
		lbs.logger.Error("Could not scan ", folder, ", got the error=", err)
		return res, err
	}

	lbs.logger.Info("Scanned folder ", folder, " found ", len(res), " objects, ", scanned, " files were considered")
	return res, nil
}

// read objects from metafile
func (lbs *LfsBlobStorage) readMetaFile() (map[string]*BlobMeta, error) {
	res := make(map[string]*BlobMeta)
	file, err := os.Open(lbs.metaFN)
	if os.IsNotExist(err) {
		lbs.logger.Info("Could not find ", lbs.metaFN, " file, considering the storage is empty.")
		return res, nil
	}

	if err != nil {
		lbs.logger.Error("Could not open file ", lbs.metaFN, " seems unrecoverable error=", err)
		return res, err
	}

	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.UseNumber()
	err = decoder.Decode(&res)
	if err != nil {
		lbs.logger.Error("Could not deserialize JSON content of ", lbs.metaFN, " seems unrecoverable error=", err)
		return res, err
	}

	return res, nil
}

func (lbs *LfsBlobStorage) saveObjects(checkTime bool) {
	now := time.Now()
	if checkTime && now.Sub(lbs.lastSave) < time.Minute {
		lbs.logger.Debug("Skip save metadata due to timeout")
		return
	}
	lbs.lastSave = now

	_, err := os.Stat(lbs.metaFN)
	if err == nil {
		err := os.Remove(lbs.metaFN)
		if err != nil {
			lbs.logger.Error("Could not remove file ", lbs.metaFN, " seems unrecoverable error=", err)
			panic(err)
		}
	}

	file, err := os.Create(lbs.metaFN)
	if err != nil {
		lbs.logger.Error("Could not save objects infos to ", lbs.metaFN)
		return
	}

	defer file.Close()
	lbs.logger.Debug("Saving ", len(lbs.objects), " objects to ", lbs.metaFN)

	encoder := json.NewEncoder(file)
	err = encoder.Encode(lbs.objects)
	if err != nil {
		lbs.logger.Error("Could not save objects infos to ", lbs.metaFN, " error=", err)
	}
}

func (lbs *LfsBlobStorage) getFilePath(id string) (string, error) {
	length := len(id)
	suffix := id[length-2 : length]
	path := filepath.Join(lbs.storeDir, suffix)
	if !common.DoesFileExist(path) {
		lbs.logger.Info("Could not find directory ", path, " creating new one...")
		err := os.MkdirAll(path, os.ModePerm)
		if err != nil {
			lbs.logger.Error("Could not create storage directory ", path, " got the error=", err)
			return "", err
		}
	}
	return filepath.Join(path, id), nil
}
