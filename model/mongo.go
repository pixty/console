package model

import (
	"reflect"
	"strings"
	"time"

	"github.com/jrivets/log4g"
	"github.com/pixty/console/common"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

const (
	cColCamera       = "cameras"
	cColOrganization = "organizations"
	cColPerson       = "persons"
	cColProfile      = "profiles"
	cColScene        = "scenes"
)

type MongoPersister struct {
	Config        *common.ConsoleConfig `inject:""`
	logger        log4g.Logger
	mlogger       log4g.Logger
	session       *mgo.Session
	crudExecutors []*crudExec
}

type txPersister struct {
	mp      *MongoPersister
	session *mgo.Session
	db      *mgo.Database
}

type crudExec struct {
	collection *mgo.Collection
	instType   reflect.Type
	logger     log4g.Logger
}

func NewMongoPersister() *MongoPersister {
	return &MongoPersister{logger: log4g.GetLogger("console.mongo"), mlogger: log4g.GetLogger("mongo.mgo")}
}

// ============================= LifeCycler ==================================
func (mp *MongoPersister) DiPhase() int {
	return common.CMP_PHASE_DB
}

func (mp *MongoPersister) DiInit() error {
	mp.logger.Info("Initializing.")

	addrs := strings.Split(mp.Config.MongoAddress, ",")

	mgo.SetDebug(mp.Config.DebugMode)
	if mp.Config.DebugMode {
		mgo.SetLogger(mp)
	}

	mongoDBDialInfo := &mgo.DialInfo{
		Addrs:    addrs,
		Timeout:  time.Duration(mp.Config.MongoTimeoutSec) * time.Second,
		Database: mp.Config.MongoDatabase,
		Username: mp.Config.MongoUser,
		Password: mp.Config.MongoPasswd,
	}

	var err error
	mp.session, err = mgo.DialWithInfo(mongoDBDialInfo)
	if err != nil {
		mp.logger.Fatal("Could not connect to MongoDB ", err)
		return err
	}

	mp.session.SetMode(mgo.Monotonic, true)
	mp.ensureIndexes()
	return nil
}

func (mp *MongoPersister) DiShutdown() {
	mp.logger.Info("Shutting down.")
}

func (mp *MongoPersister) Output(calldepth int, s string) error {
	mp.mlogger.Debug("Dpth: ", calldepth, " msg: ", s)
	return nil
}

// ============================= Persister ===================================
func (mp *MongoPersister) NewTxPersister() common.TxPersister {
	return mp.new_txPersister()
}

func (mp *MongoPersister) new_txPersister() *txPersister {
	session := mp.session.Copy()
	db := session.DB(mp.Config.MongoDatabase)
	return &txPersister{mp, session, db}
}

func (mp *MongoPersister) ensureIndexes() {
	mp.logger.Info("Ensure indexes")
	tx := mp.new_txPersister()
	defer tx.Close()

	// persons
	colPerson := tx.getMgoCollection(cColPerson)
	colPerson.EnsureIndex(mgo.Index{
		Key:        []string{"profileId"},
		Background: true,
	})

	// scenes
	colScene := tx.getMgoCollection(cColScene)
	colScene.EnsureIndex(mgo.Index{
		Key:        []string{"camId", "-timestamp"},
		Background: true,
	})
}

func (mp *MongoPersister) dropDatabase() {
	mp.logger.Warn("Dropping database: ", mp.Config.MongoDatabase)
	session := mp.session.Copy()
	defer session.Close()
	db := session.DB(mp.Config.MongoDatabase)
	db.DropDatabase()
}

// ============================ TxPersister ==================================
func (tx *txPersister) GetCrudExecutor(storage common.Storage) common.CrudExecutor {
	var colName string
	var tp reflect.Type
	switch storage {
	case common.STGE_CAMERA:
		colName = cColCamera
		tp = reflect.TypeOf(&common.Camera{})
	case common.STGE_ORGANIZATION:
		colName = cColOrganization
		tp = reflect.TypeOf(&common.Organization{})
	case common.STGE_PERSON:
		colName = cColPerson
		tp = reflect.TypeOf(&common.Person{})
	case common.STGE_PROFILE:
		colName = cColProfile
		tp = reflect.TypeOf(&common.Profile{})
	case common.STGE_SCENE:
		colName = cColScene
		tp = reflect.TypeOf(&common.Scene{})
	default:
		tx.mp.logger.Error("Unknown storage ", storage)
		return nil
	}

	collection := tx.getMgoCollection(colName)
	logname := tx.mp.logger.GetName() + "." + colName
	return &crudExec{collection, tp, log4g.GetLogger(logname)}
}

func (tx *txPersister) FindPersonsByIds(ids ...string) ([]*common.Person, error) {
	logger := tx.mp.logger
	logger.Debug("FindPersonsByIds(): looking for persons with Ids = ", ids)
	q := bson.M{"_id": bson.M{"$in": ids}}
	colPersons := tx.getMgoCollection(cColPerson)
	var res []*common.Person
	it := colPersons.Find(q).Iter()
	err := it.All(&res)
	it.Close()

	if err != nil {
		logger.Warn("FindPersonsByIds(): oops. Coannot iterate over result collection ", err)
		return nil, err
	}

	return res, nil
}

func (tx *txPersister) GetLatestScene(camId common.Id) (*common.Scene, error) {
	logger := tx.mp.logger
	logger.Debug("GetLatestScene(): looking for latest scene ", camId)
	return nil, nil
}

func (tx *txPersister) Close() {
	tx.session.Close()
	tx.session = nil
	tx.db = nil
}

func (tx *txPersister) getMgoCollection(colName string) *mgo.Collection {
	return tx.db.C(colName)
}

// =========================== CrudExecutor ==================================
func (ce *crudExec) Create(o interface{}) error {
	ce.logger.Debug("New object ", o)
	return ce.collection.Insert(o)
}

func (ce *crudExec) CreateMany(objs ...interface{}) error {
	ce.logger.Debug("New objects ", objs)
	return ce.collection.Insert(objs...)
}

func (ce *crudExec) Read(id common.Id) interface{} {
	result := reflect.New(ce.instType)
	err := ce.collection.FindId(id).One(&result)
	ce.logger.Debug("Read by id=", id, ", found=", result, " err=", err)
	if err != nil {
		return nil
	}
	return result
}

func (ce *crudExec) Update(o interface{}) error {
	ce.logger.Debug("Update ", o)
	id := reflect.ValueOf(o).Elem().FieldByName("Id")
	selector := bson.M{"_id": id}
	return ce.collection.Update(selector, o)
}

func (ce *crudExec) Delete(id common.Id) error {
	ce.logger.Debug("Delete by id=", id)
	selector := bson.M{"_id": id}
	return ce.collection.Remove(selector)
}
