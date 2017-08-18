package model

import (
	"errors"
	"math"
	"reflect"
	"strings"
	"time"

	"github.com/jrivets/log4g"
	"github.com/pixty/console/common"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

const (
	cColOrganization = "organizations"
	cColUser         = "users"
	cColCamera       = "cameras"
	cColPerson       = "persons"
	cColPersonFaces  = "personFaces"
	cColProfileMetas = "profileMetas"
)

// Implementation of common.Persister + LifeCycler
type MongoPersister struct {
	// The console configuration. Will be injected
	Config *common.ConsoleConfig `inject:""`

	logger log4g.Logger
	// Mongo logger
	mlogger  log4g.Logger
	main_mds *mongo_ds
}

// A mongo connection data source (one per database)
type mongo_ds struct {
	session *mgo.Session
	db      *mgo.Database
}

// MainPersister Mongo implementation
type main_persister struct {
	logger log4g.Logger
	mds    *mongo_ds
}

// PartPersister Mongo implementation
type part_persister struct {
	logger log4g.Logger
	mds    *mongo_ds
}

// CrudExecutor Mongo implementation
type crud struct {
	collection *mgo.Collection
	instType   reflect.Type
	logger     log4g.Logger
}

func NewMongoPersister() *MongoPersister {
	mp := new(MongoPersister)
	mp.mlogger = log4g.GetLogger("pixty.mongo.mgo")
	mp.logger = log4g.GetLogger("pixty.mongo")
	return mp
}

// =========================== MongoPersister ================================
// ----------------------------- LifeCycler ----------------------------------
func (mp *MongoPersister) DiPhase() int {
	return common.CMP_PHASE_DB
}

func (mp *MongoPersister) DiInit() error {
	mp.logger.Info("Initializing.")
	mgo.SetDebug(mp.Config.MongoDebugMode)
	if mp.Config.MongoDebugMode {
		mp.logger.Info("Setting mgo to debug mode")
		mgo.SetLogger(mp)
	}

	addrs := strings.Split(mp.Config.MongoAddress, ",")
	mdi := &mgo.DialInfo{
		Addrs:    addrs,
		Timeout:  time.Duration(mp.Config.MongoTimeoutSec) * time.Second,
		Database: mp.Config.MongoDatabase,
		Username: mp.Config.MongoUser,
		Password: mp.Config.MongoPasswd,
	}

	mds, err := mp.connect_to_mongo(mdi)
	if err != nil {
		mp.logger.Fatal("Could not connect to main DB. Probably cannot start the console ", err)
		return err
	}
	mp.main_mds = mds
	mds.ensureMainIndexes()
	return nil
}

func (mp *MongoPersister) DiShutdown() {
	mp.logger.Info("Shutting down.")
}

// ------------------------- Persister & others ------------------------------
func (mp *MongoPersister) NewMainPersister() common.MainPersister {
	main_p := new(main_persister)
	main_p.mds = mp.main_mds.clone()
	main_p.logger = mp.logger.WithId("{main}").(log4g.Logger)
	return main_p
}

func (mp *MongoPersister) NewPartPersister(partId common.Id) common.PartPersister {
	part_p := new(part_persister)
	// everything in one DB so far
	part_p.mds = mp.main_mds.clone()
	part_p.logger = mp.logger.WithId("{part=" + partId + "}").(log4g.Logger)
	return part_p
}

// Mongo logging function
func (mp *MongoPersister) Output(calldepth int, s string) error {
	mp.mlogger.Debug("Dpth: ", calldepth, " msg: ", s)
	return nil
}

func (mp *MongoPersister) connect_to_mongo(mdi *mgo.DialInfo) (*mongo_ds, error) {
	mp.logger.Info("Trying to connect to ", mdi.Addrs)
	mp.logger.Debug("Using ", mdi, " for the connection")

	session, err := mgo.DialWithInfo(mdi)
	if err != nil {
		mp.logger.Error("Could not connect to MongoDB ", err)
		return nil, err
	}

	session.SetMode(mgo.Monotonic, true)
	mds := new(mongo_ds)
	mds.session = session
	mds.db = session.DB(mp.Config.MongoDatabase)
	return mds, nil
}

// ============================== mongo_ds ===================================
func (mds *mongo_ds) clone() *mongo_ds {
	sess := mds.session.Copy()
	db := sess.DB(mds.db.Name)

	res := new(mongo_ds)
	res.session = sess
	res.db = db
	return res
}

func (mds *mongo_ds) close() {
	sess := mds.session
	if sess == nil {
		return
	}
	sess.Close()
	mds.session = nil
	mds.db = nil
}

// ensurances the main collection of indexes in place
func (mds *mongo_ds) ensureMainIndexes() {
	colCamera := mds.collection(cColCamera)
	colCamera.EnsureIndex(mgo.Index{
		Key:        []string{"orgId"},
		Background: true,
	})
	colCamera.EnsureIndex(mgo.Index{
		Key:        []string{"accessKey"},
		Background: true,
	})
}

// ensurances the main collection of indexes in place
func (mds *mongo_ds) ensurePartIndexes() {
	colPerson := mds.collection(cColPerson)
	colPerson.EnsureIndex(mgo.Index{
		Key:        []string{"camId", "-lastSeenAt"},
		Background: true,
	})
	colPerson.EnsureIndex(mgo.Index{
		Key:        []string{"profileId"},
		Background: true,
	})

	colFace := mds.collection(cColPersonFaces)
	colFace.EnsureIndex(mgo.Index{
		Key:        []string{"personId"},
		Background: true,
	})
	colFace.EnsureIndex(mgo.Index{
		Key:        []string{"capturedAt"},
		Background: true,
	})
	colFace.EnsureIndex(mgo.Index{
		Key:        []string{"imgId"},
		Background: true,
	})

	colProfileMeta := mds.collection(cColProfileMetas)
	colProfileMeta.EnsureIndex(mgo.Index{
		Key:        []string{"profileId", "orgId"},
		Background: true,
	})
}

func (mds *mongo_ds) collection(name string) *mgo.Collection {
	return mds.db.C(name)
}

func (mds *mongo_ds) dropDb() {
	mds.db.DropDatabase()
}

func (mds *mongo_ds) new_crud(colName string, tp reflect.Type, logger log4g.Logger) common.CrudExecutor {
	c := mds.collection(colName)
	logname := logger.GetName() + "." + colName
	return &crud{c, tp, log4g.GetLogger(logname)}
}

// =========================== main_persister ================================
func (mp *main_persister) GetOrganizationDAO() common.CrudExecutor {
	return mp.mds.new_crud(cColOrganization, reflect.TypeOf(&common.Organization{}), mp.logger)
}

func (mp *main_persister) GetUserDAO() common.CrudExecutor {
	return mp.mds.new_crud(cColUser, reflect.TypeOf(&common.User{}), mp.logger)
}

func (mp *main_persister) GetCameraDAO() common.CrudExecutor {
	return mp.mds.new_crud(cColCamera, reflect.TypeOf(&common.Camera{}), mp.logger)
}

func (mp *main_persister) Close() {
	mp.mds.close()
}

// =========================== part_persister ================================
func (pp *part_persister) GetPersonDAO() common.CrudExecutor {
	return pp.mds.new_crud(cColPerson, reflect.TypeOf(&common.Person{}), pp.logger)
}

func (pp *part_persister) GetPersonFacesDAO() common.CrudExecutor {
	return pp.mds.new_crud(cColPersonFaces, reflect.TypeOf(&common.Face{}), pp.logger)
}

func (pp *part_persister) GetProfileMetasDAO() common.CrudExecutor {
	return pp.mds.new_crud(cColProfileMetas, reflect.TypeOf(&common.ProfileMeta{}), pp.logger)
}

func (pp *part_persister) SearchPersons(query *common.PersonsQuery) ([]*common.Person, error) {
	pp.logger.Debug("SearchPersons(): looking for persons query = ", query)
	colPersons := pp.mds.collection(cColPerson)

	var mq *mgo.Query
	if query.PersonIds != nil && len(query.PersonIds) > 0 {
		q := bson.M{"_id": bson.M{"$in": query.PersonIds}}
		mq = colPersons.Find(q)
	} else if (query.CamId != common.ID_NULL || query.MaxLastSeenAt != common.TIMESTAMP_NA) && query.Limit > 0 {
		cond := []bson.M{}
		if query.CamId != common.ID_NULL {
			cond = append(cond, bson.M{"camId": query.CamId})
		}
		if query.MaxLastSeenAt != common.TIMESTAMP_NA {
			cond = append(cond, bson.M{"lastSeenAt": bson.M{"$lte": query.MaxLastSeenAt}})
		}

		q := cond[0]
		if len(cond) > 1 {
			q = bson.M{"$and": cond}
		}

		mq = colPersons.Find(q).Sort("-lastSeenAt").Limit(normLimit(query.Limit))
	} else {
		pp.logger.Warn("SearchPersons(): bad query = ", query)
		return nil, errors.New("list of person Ids or camera/limits should be specified")
	}

	var res []*common.Person
	it := mq.Iter()
	err := it.All(&res)
	it.Close()
	return res, err
}

func (pp *part_persister) Close() {
	pp.mds.close()
}

// ================================ crud =====================================
func (ce *crud) Create(o interface{}) error {
	ce.logger.Debug("New object ", o)
	return ce.collection.Insert(o)
}

func (ce *crud) CreateMany(objs ...interface{}) error {
	ce.logger.Debug("New objects ", objs)
	return ce.collection.Insert(objs...)
}

func (ce *crud) Read(id common.Id, res interface{}) error {
	err := ce.collection.FindId(id).One(res)
	ce.logger.Debug("Read by id=", id, ", found=", res, " err=", err)
	return err
}

func (ce *crud) Update(id common.Id, o interface{}) error {
	ce.logger.Debug("Update ", o, ", id=", id)
	selector := bson.M{"_id": id}
	_, err := ce.collection.Upsert(selector, o)
	return err
}

func (ce *crud) Delete(id common.Id) error {
	ce.logger.Debug("Delete by id=", id)
	selector := bson.M{"_id": id}
	return ce.collection.Remove(selector)
}

func normLimit(limit int) int {
	if limit <= 0 {
		return math.MaxInt32
	}
	return limit
}

//type MongoPersister struct {
//	Config        *common.ConsoleConfig `inject:""`
//	logger        log4g.Logger
//	mlogger       log4g.Logger
//	session       *mgo.Session
//	crudExecutors []*crudExec
//}

//type txPersister struct {
//	mp      *MongoPersister
//	session *mgo.Session
//	db      *mgo.Database
//}

//type crudExec struct {
//	collection *mgo.Collection
//	instType   reflect.Type
//	logger     log4g.Logger
//}

//func NewMongoPersister() *MongoPersister {
//	return &MongoPersister{logger: log4g.GetLogger("pixty.mongo"), mlogger: log4g.GetLogger("mongo.mgo")}
//}

// ============================= LifeCycler ==================================
//func (mp *MongoPersister) DiPhase() int {
//	return common.CMP_PHASE_DB
//}

//func (mp *MongoPersister) DiInit() error {
//	mp.logger.Info("Initializing.")

//	addrs := strings.Split(mp.Config.MongoAddress, ",")

//	mgo.SetDebug(mp.Config.MongoDebugMode)
//	if mp.Config.MongoDebugMode {
//		mgo.SetLogger(mp)
//	}

//	mongoDBDialInfo := &mgo.DialInfo{
//		Addrs:    addrs,
//		Timeout:  time.Duration(mp.Config.MongoTimeoutSec) * time.Second,
//		Database: mp.Config.MongoDatabase,
//		Username: mp.Config.MongoUser,
//		Password: mp.Config.MongoPasswd,
//	}

//	var err error
//	mp.session, err = mgo.DialWithInfo(mongoDBDialInfo)
//	if err != nil {
//		mp.logger.Fatal("Could not connect to MongoDB ", err)
//		return err
//	}

//	mp.session.SetMode(mgo.Monotonic, true)
//	mp.ensureIndexes()
//	return nil
//}

//func (mp *MongoPersister) DiShutdown() {
//	mp.logger.Info("Shutting down.")
//}

//func (mp *MongoPersister) Output(calldepth int, s string) error {
//	mp.mlogger.Debug("Dpth: ", calldepth, " msg: ", s)
//	return nil
//}

// ============================= Persister ===================================
//func (mp *MongoPersister) NewTxPersister(ctx context.Context) common.TxPersister {
//	return mp.new_txPersister()
//}

//func (mp *MongoPersister) new_txPersister() *txPersister {
//	session := mp.session.Copy()
//	db := session.DB(mp.Config.MongoDatabase)
//	return &txPersister{mp, session, db}
//}

//func (mp *MongoPersister) ensureIndexes() {
//	mp.logger.Info("Ensure indexes")
//	tx := mp.new_txPersister()
//	defer tx.Close()

//	// persons
//	colPerson := tx.getMgoCollection(cColPerson)
//	colPerson.EnsureIndex(mgo.Index{
//		Key:        []string{"profileId", "-seenAt"},
//		Background: true,
//	})

//	// scenes
//	colScene := tx.getMgoCollection(cColScene)
//	colScene.EnsureIndex(mgo.Index{
//		Key:        []string{"camId", "-timestamp"},
//		Background: true,
//	})
//}

//func (mp *MongoPersister) dropDatabase() {
//	mp.logger.Warn("Dropping database: ", mp.Config.MongoDatabase)
//	session := mp.session.Copy()
//	defer session.Close()
//	db := session.DB(mp.Config.MongoDatabase)
//	db.DropDatabase()
//}

// ============================ TxPersister ==================================
//func (tx *txPersister) GetCrudExecutor(storage common.Storage) common.CrudExecutor {
//	var colName string
//	var tp reflect.Type
//	switch storage {
//	case common.STGE_CAMERA:
//		colName = cColCamera
//		tp = reflect.TypeOf(&common.Camera{})
//	case common.STGE_ORGANIZATION:
//		colName = cColOrganization
//		tp = reflect.TypeOf(&common.Organization{})
//	case common.STGE_PERSON:
//		colName = cColPerson
//		tp = reflect.TypeOf(&common.Person{})
//	case common.STGE_PERSON_MATCH:
//		colName = cColPersonMatch
//		tp = reflect.TypeOf(&common.PersonMatch{})
//	case common.STGE_PROFILE:
//		colName = cColProfile
//		tp = reflect.TypeOf(&common.Profile{})
//	case common.STGE_SCENE:
//		colName = cColScene
//		tp = reflect.TypeOf(&common.Scene{})
//	default:
//		tx.mp.logger.Error("Unknown storage ", storage)
//		return nil
//	}

//	collection := tx.getMgoCollection(colName)
//	logname := tx.mp.logger.GetName() + "." + colName
//	return &crudExec{collection, tp, log4g.GetLogger(logname)}
//}

//func (tx *txPersister) FindPersons(query *common.PersonsQuery) ([]*common.Person, error) {
//	logger := tx.mp.logger
//	logger.Debug("FindPersons(): looking for persons query = ", query)
//	colPersons := tx.getMgoCollection(cColPerson)

//	var mq *mgo.Query
//	if query.PersonIds != nil && len(query.PersonIds) > 0 {
//		q := bson.M{"_id": bson.M{"$in": query.PersonIds}}
//		mq = colPersons.Find(q)
//	} else if query.ProfileId != common.ID_NULL {
//		q := bson.M{"$and": []bson.M{bson.M{"seenAt": bson.M{"$lte": query.FromTime}}, bson.M{"profileId": query.ProfileId}}}
//		mq = colPersons.Find(q).Sort("-seenAt").Limit(normLimit(query.Limit))
//	} else {
//		logger.Warn("FindPersons(): bad query = ", query)
//		return nil, errors.New("list of person Ids or profile Id should be specified")
//	}

//	var res []*common.Person
//	it := mq.Iter()
//	err := it.All(&res)
//	it.Close()
//	return res, err

//}

//func (tx *txPersister) GetScenes(query *common.SceneQuery) ([]*common.Scene, error) {
//	logger := tx.mp.logger
//	logger.Debug("GetScenes(): looking for query=", query)
//	q := bson.M{"camId": query.CamId}
//	col := tx.getMgoCollection(cColScene)
//	var res []*common.Scene
//	it := col.Find(q).Sort("-timestamp").Limit(normLimit(query.Limit)).Iter()
//	err := it.All(&res)
//	it.Close()
//	return res, err
//}

//func (tx *txPersister) GetMatches(personId common.Id) ([]*common.PersonMatch, error) {
//	logger := tx.mp.logger
//	logger.Debug("GetMatches(): looking for person=", personId)
//	q := bson.M{"personId": personId}
//	comPersMatches := tx.getMgoCollection(cColPersonMatch)
//	var res []*common.PersonMatch
//	it := comPersMatches.Find(q).Iter()
//	err := it.All(&res)
//	it.Close()
//	return res, err
//}

//func (tx *txPersister) Close() {
//	tx.session.Close()
//	tx.session = nil
//	tx.db = nil
//}

//func (tx *txPersister) getMgoCollection(colName string) *mgo.Collection {
//	return tx.db.C(colName)
//}
