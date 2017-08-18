package model

import (
	"testing"

	"github.com/pixty/console/common"
)

func TestMysqlConnection(t *testing.T) {
	mp := NewMysqlPersister()
	mp.Config = common.NewConsoleConfig()
	mp.Config.MysqlDatasource = "pixty@/pixty?charset=utf8"
	mp.DiInit()

}
