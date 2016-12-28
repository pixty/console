package model

import "github.com/pixty/console/common"
import "github.com/jrivets/log4g"
import "gopkg.in/mgo.v2"
import "gopkg.in/mgo.v2/bson"
import "time"
import "strings"
import "reflect"

type MongoPersister struct {
	Config        *common.ConsoleConfig `inject:""`
	logger        log4g.Logger
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
}

func NewMongoPersister() *MongoPersister {
	return &MongoPersister{logger: log4g.GetLogger("console.mongo")}
}

// ============================= LifeCycler ==================================
func (mp *MongoPersister) DiPhase() int {
	return common.CMP_PHASE_DB
}

func (mp *MongoPersister) DiInit() error {
	mp.logger.Info("Initializing.")

	addrs := strings.Split(mp.Config.MongoAddress, ",")
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
	return nil
}

func (mp *MongoPersister) DiShutdown() {
	mp.logger.Info("Shutting down.")
}

// ============================= Persister ===================================
func (mp *MongoPersister) NewTxPersister() common.TxPersister {
	session := mp.session.Copy()
	db := session.DB(mp.Config.MongoDatabase)
	return &txPersister{mp, session, db}
}

// ============================ TxPersister ==================================
func (tx *txPersister) GetCrudExecutor(storage common.Storage) common.CrudExecutor {
	var colName string
	var tp reflect.Type
	switch storage {
	case common.STGE_CAMERA:
		colName = "camera"
		tp = reflect.TypeOf(&common.Camera{})
	case common.STGE_ORGANIZATION:
		colName = "organization"
		tp = reflect.TypeOf(&common.Organization{})
	case common.STGE_PERSON:
		colName = "person"
		tp = reflect.TypeOf(&common.Person{})
	case common.STGE_PERSON_LOG:
		colName = "personLog"
		tp = reflect.TypeOf(&common.PersonLog{})
	case common.STGE_PERSON_ORG:
		colName = "personOrgInfo"
		tp = reflect.TypeOf(&common.PersonOrgInfo{})
	default:
		tx.mp.logger.Error("Unknown storage ", storage)
		return nil
	}

	collection := tx.db.C(colName)
	return &crudExec{collection, tp}
}

func (tx *txPersister) GetPersonLogs(query *common.PersonLogQuery) []*common.PersonLog {
	//TODO
	return nil
}

func (tx *txPersister) Close() {
	tx.session.Close()
	tx.session = nil
	tx.db = nil
}

// =========================== CrudExecutor ==================================
func (ce *crudExec) NewId() common.Id {
	return common.Id(bson.NewObjectId())
}

func (ce *crudExec) Create(o interface{}) error {
	return ce.collection.Insert(o)
}

func (ce *crudExec) Read(id common.Id) interface{} {
	result := reflect.New(ce.instType)
	err := ce.collection.FindId(id).One(&result)
	if err != nil {
		return nil
	}
	return result
}

func (ce *crudExec) Update(o interface{}) error {
	id := reflect.ValueOf(o).Elem().FieldByName("Id")
	selector := bson.M{"_id": id}
	return ce.collection.Update(selector, o)
}

func (ce *crudExec) Delete(id common.Id) error {
	selector := bson.M{"_id": id}
	return ce.collection.Remove(selector)
}
