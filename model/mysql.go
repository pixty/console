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
	return nil
}

func (mp *MysqlPersister) DiShutdown() {
	mp.logger.Info("Shutting down.")
}

// ------------------------------ Persister ----------------------------------
func (mp *MysqlPersister) GetMainPersister() MainPersister {
	return mp.mainPers
}

func (mp *MysqlPersister) GetPartPersister(partId Id) PartPersister {
	return nil
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
func (mmp *msql_main_persister) FindCameraByAccessKey(ak string) (*Camera, error) {
	db, err := mmp.dbc.getDb()
	if err != nil {
		return nil, err
	}

	rows, err := db.Query("SELECT id, org_id, secret_key FROM camera WHERE access_key=?", ak)
	if err != nil {
		mmp.logger.Warn("Could read camera by access_key=", ak, ", err=", err)
		return nil, err
	}
	defer rows.Close()

	if rows.Next() {
		cam := new(Camera)
		err := rows.Scan(&cam.Id, &cam.OrgId, &cam.SecretKey)
		cam.AcceessKey = ak
		if err != nil {
			mmp.logger.Warn("FindCameraByAccessKey() could not scan result err=", err)
		}
		return cam, nil
	}
	return nil, nil
}
