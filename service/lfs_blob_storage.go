package service

import "github.com/pixty/console/common"
import "github.com/jrivets/log4g"
import "io"
import "os"
import "path/filepath"
import "encoding/json"
import "errors"
import "sync"

const cMetaFileName = ".meta"

type LfsBlobStorage struct {
	logger   log4g.Logger
	Config   *common.ConsoleConfig `inject:""`
	objects  map[common.Id]*common.BlobMeta
	metaFN   string
	storeDir string
	init     bool
	rwLock   sync.RWMutex
}

func NewLfsBlobStorage() *LfsBlobStorage {
	logger := log4g.GetLogger("console.service.LFSBlobStorage")
	return &LfsBlobStorage{logger: logger}
}

// ============================= LifeCycler ==================================
func (lbs *LfsBlobStorage) DiPhase() int {
	return common.CMP_PHASE_BLOB_STORE
}

func (lbs *LfsBlobStorage) DiInit() error {
	lbs.rwLock.Lock()
	defer lbs.rwLock.Unlock()

	if lbs.init {
		lbs.logger.Error("Unexpected state. Already initialized ", lbs)
		return errors.New("LfsBlobStorage already initialized")
	}

	lbs.objects = make(map[common.Id]*common.BlobMeta)

	lbs.logger.Info("Initializing...")
	if lbs.Config.LbsDir == "" {
		lbs.logger.Error("Expecting not empty storage directory")
		return errors.New("LfsBlobStorage expects not empty storage directory")
	}

	path := lbs.Config.LbsDir
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
	lbs.init = true
	return lbs.readObjects()
}

func (lbs *LfsBlobStorage) DiShutdown() {
	lbs.rwLock.Lock()
	defer lbs.rwLock.Unlock()

	if !lbs.init {
		lbs.logger.Error("Unexpected state. Not initialized ", lbs)
	}
	lbs.saveObjects()
	lbs.objects = nil
}

// ============================= BlobStorage =================================
func (lbs *LfsBlobStorage) Add(r io.Reader, bMeta *common.BlobMeta) (common.Id, error) {
	if r == nil {
		lbs.logger.Error("reader (r) is nil ")
		return common.ID_NULL, errors.New("reader cannot be nil")
	}

	lbs.rwLock.Lock()
	defer lbs.rwLock.Unlock()

	lbs.checkStarted()

	id := common.NewId()
	fileName := filepath.Join(lbs.storeDir, string(id))
	file, err := os.Create(fileName)
	if err != nil {
		lbs.logger.Error("Could not create new file ", fileName)
		return common.ID_NULL, err
	}

	defer file.Close()

	_, err = io.Copy(file, r)
	if err != nil {
		lbs.logger.Error("Could not copy data to file ", fileName)
		return common.ID_NULL, err
	}

	lbs.objects[id] = bMeta
	lbs.logger.Info("New BLOB with id=", id, " is created. file=", fileName)
	return id, nil
}

func (lbs *LfsBlobStorage) Read(objId common.Id) (io.ReadCloser, *common.BlobMeta) {
	lbs.rwLock.RLock()
	defer lbs.rwLock.RUnlock()

	lbs.checkStarted()

	fileName := filepath.Join(lbs.storeDir, string(objId))

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

func (lbs *LfsBlobStorage) Delete(objId common.Id) error {
	lbs.rwLock.Lock()
	defer lbs.rwLock.Unlock()

	lbs.logger.Info("Deleting BLOB by id=", objId)

	fileName := filepath.Join(lbs.storeDir, string(objId))
	_, ok := lbs.objects[objId]
	if !ok {
		lbs.logger.Warn("Could not find BLOB by id=", objId)
		return errors.New("Could not find object by id=" + string(objId))
	}

	delete(lbs.objects, objId)
	os.Remove(fileName)
	return nil
}

// =============================== Private ===================================
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

	lbs.logger.Info(len(lbs.objects), " objects have been just read from ", lbs.metaFN)
	return nil
}

func (lbs *LfsBlobStorage) saveObjects() {
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

	lbs.logger.Info("Saving ", len(lbs.objects), " objects to ", lbs.metaFN)
	encoder := json.NewEncoder(file)
	err = encoder.Encode(lbs.objects)
	if err != nil {
		lbs.logger.Error("Could not save objects infos to ", lbs.metaFN, " error=", err)
	}
}

func (lbs *LfsBlobStorage) checkStarted() {
	if !lbs.init {
		panic("The LfsBlobStorage is not initialized.")
	}
}
