package model

import (
	"github.com/pixty/console/common"
)

// Implementation of common.Persister + LifeCycler
type MysqlPersister struct {
	// The console configuration. Will be injected
	Config *common.ConsoleConfig `inject:""`

	logger log4g.Logger
	// Mongo logger
	mlogger  log4g.Logger
	main_mds *mongo_ds
}
