package model

import (
	"database/sql"
	"sync"

	"github.com/go-sql-driver/mysql"
	"github.com/jrivets/log4g"
	"github.com/pixty/console/common"
)

// Implementation of common.Persister + LifeCycler
type (
	MysqlPersister struct {
		// The console configuration. Will be injected
		Config   *common.ConsoleConfig `inject:""`
		logger   log4g.Logger
		mainPers *msql_main_persister
		// same like main one, so far...
		partPers *msql_part_persister
	}

	msql_connection struct {
		lock    sync.Mutex
		logger  log4g.Logger
		conName string
		ds      string
		db      *sql.DB
	}

	msql_main_persister struct {
		logger log4g.Logger
		dbc    *msql_connection
	}

	msql_part_persister struct {
		logger log4g.Logger
		dbc    *msql_connection
	}
)

func NewMysqlPersister() *MysqlPersister {
	mp := new(MysqlPersister)
	mp.logger = log4g.GetLogger("pixty.mysql")
	return mp
}

// =========================== MysqlPersister ================================
// ----------------------------- LifeCycler ----------------------------------
func (mp *MysqlPersister) DiPhase() int {
	return common.CMP_PHASE_DB
}

func (mp *MysqlPersister) DiInit() error {
	mp.logger.Info("Initializing.")
	mc := mp.newConnection(mp.Config.MysqlDatasource, log4g.GetLogger("pixty.mysql.main"))
	mp.mainPers = &msql_main_persister{mc.logger, mc}
	pc := mp.newConnection(mp.Config.MysqlDatasource, log4g.GetLogger("pixty.mysql.part"))
	mp.partPers = &msql_part_persister{pc.logger, pc}
	return nil
}

func (mp *MysqlPersister) DiShutdown() {
	mp.logger.Info("Shutting down.")
}

// ------------------------------ Persister ----------------------------------
func (mp *MysqlPersister) GetMainPersister() MainPersister {
	return mp.mainPers
}

func (mp *MysqlPersister) GetPartPersister(partId string) PartPersister {
	// let's use one instance of part persister so far
	return mp.partPers
}

// -------------------------------- Misc -------------------------------------
//The method cuts password to make it use in logs
func (mp *MysqlPersister) getFriendlyName(ds string) string {
	cfg, err := mysql.ParseDSN(ds)
	if err != nil {
		mp.logger.Error("Could not parse DSN properly: ", err)
		return "N/A"
	}
	return cfg.User + "@" + cfg.Addr + "/" + cfg.DBName
}

func (mp *MysqlPersister) newConnection(ds string, logger log4g.Logger) *msql_connection {
	conName := mp.getFriendlyName(ds)
	mp.logger.Info("new connection to ", conName)
	mc := new(msql_connection)
	mc.conName = conName
	mc.ds = ds
	mc.logger = logger
	return mc
}

// =========================== msql_connection ===============================
func (mc *msql_connection) close() {
	mc.lock.Lock()
	defer mc.lock.Unlock()
	mc.logger.Info("Close DB connection to ", mc.conName)
	if mc.db != nil {
		mc.db.Close()
		mc.db = nil
	}
}

func (mc *msql_connection) getDb() (*sql.DB, error) {
	db := mc.db
	if db == nil {
		mc.lock.Lock()
		defer mc.lock.Unlock()

		if mc.db != nil {
			return mc.db, nil
		}
		mc.logger.Info("Connecting to ", mc.conName)

		var err error
		db, err = sql.Open("mysql", mc.ds)
		if err != nil {
			mc.logger.Error("Could not open connection, err=", err)
			return nil, err
		}
		mc.db = db
	}
	return db, nil
}

// ========================= msql_main_persister =============================
func (mmp *msql_main_persister) FindCameraById(camId string) (*Camera, error) {
	db, err := mmp.dbc.getDb()
	if err != nil {
		mmp.logger.Warn("FindCameraById(): Could not get DB err=", err)
		return nil, err
	}

	rows, err := db.Query("SELECT org_id, secret_key FROM camera WHERE id=?", camId)
	if err != nil {
		mmp.logger.Warn("Could read camera by id=", camId, ", err=", err)
		return nil, err
	}
	defer rows.Close()

	if rows.Next() {
		cam := new(Camera)
		err := rows.Scan(&cam.OrgId, &cam.SecretKey)
		cam.Id = camId
		if err != nil {
			mmp.logger.Warn("FindCameraByAccessKey() could not scan result err=", err)
		}
		return cam, nil
	}
	return nil, nil
}

// ========================= msql_part_persister =============================
func (mpp *msql_part_persister) InsertFace(f *Face) (int64, error) {
	db, err := mpp.dbc.getDb()
	if err != nil {
		mpp.logger.Warn("InsertFace(): Could not get DB err=", err)
		return -1, err
	}

	res, err := db.Exec("INSERT INTO face(scene_id, person_id, captured_at, image_id, img_top, img_left, img_bottom, img_right, face_image_id, v128d) VALUES (?,?,?,?,?,?,?,?,?,?)",
		f.SceneId, f.PersonId, f.CapturedAt, f.ImageId, f.Rect.LeftTop.Y, f.Rect.LeftTop.X, f.Rect.RightBottom.Y, f.Rect.RightBottom.X, f.FaceImageId, f.V128D.ToByteSlice())
	if err != nil {
		mpp.logger.Warn("Could not insert new face ", f, ", got the err=", err)
		return -1, err
	}

	return res.LastInsertId()
}

func (mpp *msql_part_persister) GetFaceById(fId int64) (*Face, error) {
	db, err := mpp.dbc.getDb()
	if err != nil {
		mpp.logger.Warn("GetFaceById(): Could not get DB err=", err)
		return nil, err
	}

	rows, err := db.Query("SELECT scene_id, person_id, captured_at, image_id, img_top, img_left, img_bottom, img_right, face_image_id, v128d FROM face WHERE id=?", fId)
	if err != nil {
		mpp.logger.Warn("GetFaceById(): could read face by id=", fId, ", err=", err)
		return nil, err
	}
	defer rows.Close()

	if rows.Next() {
		f := new(Face)
		f.Id = fId
		f.V128D = common.NewV128D()
		vec := make([]byte, common.V128D_SIZE)
		err := rows.Scan(&f.SceneId, &f.PersonId, &f.CapturedAt, &f.ImageId, &f.Rect.LeftTop.Y, &f.Rect.LeftTop.X, &f.Rect.RightBottom.Y, &f.Rect.RightBottom.X, &f.FaceImageId, &vec)
		if err != nil {
			mpp.logger.Warn("GetFaceById(): could not scan result err=", err)
		}
		f.V128D.Assign(vec)
		return f, nil
	}
	return nil, nil
}

func (mpp *msql_part_persister) GetPersonById(pId string) (*Person, error) {
	db, err := mpp.dbc.getDb()
	if err != nil {
		mpp.logger.Warn("GetPersonById(): Could not get DB err=", err)
		return nil, err
	}
	rows, err := db.Query("SELECT cam_id, last_seen, profile_id, picture_id, match_group FROM person WHERE id=?", pId)
	if err != nil {
		mpp.logger.Warn("GetPersonById(): could read by id=", pId, ", err=", err)
		return nil, err
	}
	defer rows.Close()

	if rows.Next() {
		p := new(Person)
		p.Id = pId
		err := rows.Scan(&p.CamId, &p.LastSeenAt, &p.ProfileId, &p.PictureId, &p.MatchGroup)
		if err != nil {
			mpp.logger.Warn("GetPersonById(): could not scan result err=", err)
		}
		return p, nil
	}
	return nil, nil
}

func (mpp *msql_part_persister) InsertPerson(p *Person) (int64, error) {
	db, err := mpp.dbc.getDb()
	if err != nil {
		mpp.logger.Warn("InsertPerson(): Could not get DB err=", err)
		return -1, err
	}
	res, err := db.Exec("INSERT INTO person(id, cam_id, last_seen, profile_id, picture_id, match_group) VALUES (?,?,?,?,?,?)",
		p.Id, p.CamId, p.LastSeenAt, p.ProfileId, p.PictureId, p.MatchGroup)
	if err != nil {
		mpp.logger.Warn("InsertPerson(): Could not insert new person ", p, ", got the err=", err)
		return -1, err
	}

	return res.LastInsertId()
}

func (mpp *msql_part_persister) UpdatePerson(p *Person) error {
	db, err := mpp.dbc.getDb()
	if err != nil {
		mpp.logger.Warn("UpdatePerson(): Could not get DB err=", err)
		return err
	}

	_, err = db.Exec("UPDATE person SET last_seen=?, profile_id=?, picture_id=?, match_group=?",
		p.LastSeenAt, p.ProfileId, p.PictureId, p.MatchGroup)
	if err != nil {
		mpp.logger.Warn("UpdatePerson(): Could not update person ", p, ", got the err=", err)
		return err
	}
	return nil
}
