package model

import (
	"errors"
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
	cColPersonMatch  = "matches"
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
		Key:        []string{"profileId", "-seenAt"},
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
	case common.STGE_PERSON_MATCH:
		colName = cColPersonMatch
		tp = reflect.TypeOf(&common.PersonMatch{})
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

func (tx *txPersister) FindPersons(query *common.PersonsQuery) ([]*common.Person, error) {
	logger := tx.mp.logger
	logger.Debug("FindPersons(): looking for persons query = ", query)
	colPersons := tx.getMgoCollection(cColPerson)

	var mq *mgo.Query
	if query.PersonIds != nil && len(query.PersonIds) > 0 {
		q := bson.M{"_id": bson.M{"$in": query.PersonIds}}
		mq = colPersons.Find(q)
	} else if query.ProfileId != common.ID_NULL {
		q := bson.M{"$and": []bson.M{bson.M{"seenAt": bson.M{"$lte": query.FromTime}}, bson.M{"profileId": query.ProfileId}}}
		mq = colPersons.Find(q).Sort("-seenAt").Limit(query.Limit)
	} else {
		logger.Warn("FindPersons(): bad query = ", query)
		return nil, errors.New("list of person Ids or profile Id should be specified")
	}

	var res []*common.Person
	it := mq.Iter()
	err := it.All(&res)
	it.Close()
	return res, err

}

func (tx *txPersister) GetScenes(query *common.SceneQuery) ([]*common.Scene, error) {
	logger := tx.mp.logger
	logger.Debug("GetScenes(): looking for query=", query)
	q := bson.M{"camId": query.CamId}
	col := tx.getMgoCollection(cColScene)
	var res []*common.Scene
	it := col.Find(q).Sort("-timestamp").Limit(query.Limit).Iter()
	err := it.All(&res)
	it.Close()
	return res, err
}

func (tx *txPersister) GetMatches(personId common.Id) ([]*common.PersonMatch, error) {
	logger := tx.mp.logger
	logger.Debug("GetMatches(): looking for person=", personId)
	q := bson.M{"personId": personId}
	comPersMatches := tx.getMgoCollection(cColPersonMatch)
	var res []*common.PersonMatch
	it := comPersMatches.Find(q).Iter()
	err := it.All(&res)
	it.Close()
	return res, err
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
