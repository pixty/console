package service

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/jrivets/gorivets"
	"github.com/jrivets/log4g"
	"github.com/pixty/console/common"
)

const cMetaFileName = ".meta"

type LfsBlobStorage struct {
	logger   log4g.Logger
	objects  map[common.Id]*common.BlobMeta
	lru      *gorivets.Lru
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

func (lbs *LfsBlobStorage) init() error {
	lbs.rwLock.Lock()
	defer lbs.rwLock.Unlock()

	lbs.objects = make(map[common.Id]*common.BlobMeta)

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
func (lbs *LfsBlobStorage) Add(r io.Reader, bMeta *common.BlobMeta) (common.Id, error) {
	lbs.logger.Debug("Writing ", bMeta)
	defer lbs.logger.Debug("Done with writing ", bMeta)
	if r == nil {
		lbs.logger.Error("reader (r) is nil ")
		return common.ID_NULL, errors.New("reader cannot be nil")
	}

	if bMeta == nil {
		bMeta = common.NewBlobMeta()
	}

	id := bMeta.Id
	if id == common.ID_NULL {
		id = common.NewId()
	}
	fileName, err := lbs.getFilePath(string(id))
	if err != nil {
		lbs.logger.Error("Could not create path for id=", id)
		return common.ID_NULL, err
	}

	file, err := os.Create(fileName)
	if err != nil {
		lbs.logger.Error("Could not create new file ", fileName)
		return common.ID_NULL, err
	}
	defer file.Close()

	_, err = io.Copy(file, r)
	if err != nil {
		lbs.logger.Error("Could not copy data to fileName=", fileName, ", err=", err)
		return common.ID_NULL, err
	}

	fi, err := os.Stat(fileName)
	if err != nil {
		lbs.logger.Error("Could not obtain fileName=", fileName, " stat err=", err)
		return common.ID_NULL, err
	}
	bMeta.Id = id
	bMeta.Size = fi.Size()

	lbs.rwLock.Lock()
	defer lbs.rwLock.Unlock()

	lbs.objects[id] = bMeta
	lbs.logger.Info("New BLOB with id=", id, " is created. file=", fileName)
	lbs.lru.Add(bMeta.Id, bMeta, bMeta.Size)
	lbs.saveObjects(true)
	return id, nil
}

func (lbs *LfsBlobStorage) Read(objId common.Id) (io.ReadCloser, *common.BlobMeta) {
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
		os.Remove(fileName)
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

func (lbs *LfsBlobStorage) ReadMeta(objId common.Id) *common.BlobMeta {
	lbs.rwLock.RLock()
	defer lbs.rwLock.RUnlock()

	bMeta, ok := lbs.objects[objId]
	if !ok {
		lbs.logger.Debug("Could not find BLOB by id=", objId)
	}
	return bMeta
}

func (lbs *LfsBlobStorage) Delete(objId common.Id) error {
	fileName, err := lbs.getFilePath(string(objId))
	if err != nil {
		lbs.logger.Error("Could not form path for id=", objId, ", err=", err)
		return err
	}

	lbs.rwLock.Lock()
	defer lbs.rwLock.Unlock()

	lbs.logger.Info("Deleting BLOB by id=", objId)
	_, ok := lbs.objects[objId]
	if !ok {
		lbs.logger.Warn("Could not find BLOB by id=", objId)
		return errors.New("Could not find object by id=" + string(objId))
	}

	delete(lbs.objects, objId)
	os.Remove(fileName)
	lbs.lru.DeleteWithCallback(objId, false)
	return nil
}

// =============================== Private ===================================
func compBlobMeta(a, b interface{}) int {
	bm1 := a.(*common.BlobMeta)
	bm2 := b.(*common.BlobMeta)
	switch {
	case bm1.Timestamp < bm2.Timestamp:
		return -1
	case bm1.Timestamp > bm2.Timestamp:
		return 1
	default:
		return 0
	}
}

func (lbs *LfsBlobStorage) onLRUDelete(k, v interface{}) {
	id := k.(common.Id)
	lbs.logger.Debug("LRU deleting object id=", id)
	fp, err := lbs.getFilePath(string(id))
	if err != nil {
		lbs.logger.Warn("Could not obtain filePath for id=", id)
	} else {
		os.Remove(fp)
	}
	delete(lbs.objects, id)
}

func (lbs *LfsBlobStorage) readObjects() error {
	file, err := os.Open(lbs.metaFN)
	if os.IsNotExist(err) {
		lbs.logger.Info("Could not find ", lbs.metaFN, " file, considering the storage is empty.")
		return nil
	}

	if err != nil {
		lbs.logger.Error("Could not open file ", lbs.metaFN, " seems unrecoverable error=", err)
		return err
	}

	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.UseNumber()
	err = decoder.Decode(&lbs.objects)
	if err != nil {
		lbs.logger.Error("Could not deserialize JSON content of ", lbs.metaFN, " seems unrecoverable error=", err)
		return err
	}

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
		bm := sl.At(i).(*common.BlobMeta)
		lbs.lru.Add(bm.Id, bm, bm.Size)
	}

	lbs.logger.Info(len(lbs.objects), " objects have been just read from ", lbs.metaFN)
	return nil
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

	if checkTime {
		lbs.logger.Debug("Saving ", len(lbs.objects), " objects to ", lbs.metaFN)
	} else {
		lbs.logger.Info("Saving ", len(lbs.objects), " objects to ", lbs.metaFN)
	}

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
