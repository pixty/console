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
		dbc *msql_connection
	}

	msql_part_persister struct {
		dbc *msql_connection
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
	mp.mainPers = &msql_main_persister{mc}
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
	mc.logger = logger
	mc.reConnect()
	return mc
}

// =========================== msql_connection ===============================
func (mc *msql_connection) connect() {
	mc.logger.Info("Connecting to ", mc.conName)

	db, err := sql.Open("mysql", mc.ds)
	if err != nil {
		mc.logger.Error("Could not open connection, err=", err)
		return
	}

	err = db.Ping()
	if err != nil {
		mc.logger.Error("Could not ping server, err=", err)
		return
	}

	mc.db = db
	mc.logger.Info("Ping ok for ", mc.conName)
	return
}

func (mc *msql_connection) reConnect() {
	mc.lock.Lock()
	defer mc.lock.Unlock()

	if mc.db != nil {
		err := mc.db.Ping()
		if err == nil {
			mc.logger.Debug("reConnect is fine")
			return
		}
		mc.logger.Warn("Cannot ping DB connection err=", err)
		mc.db.Close()
		mc.db = nil
	}
	mc.connect()
}

func (mc *msql_connection) close() {
	mc.lock.Lock()
	defer mc.lock.Unlock()
	mc.logger.Info("Close DB connection to ", mc.conName)
	if mc.db != nil {
		mc.db.Close()
		mc.db = nil
	}
}
