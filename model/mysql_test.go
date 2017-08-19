package model

import (
	"testing"
	"time"

	"github.com/jrivets/log4g"
	"github.com/pixty/console/common"
)

func TestMysqlConnection(t *testing.T) {
	mp := NewMysqlPersister()
	mp.Config = common.NewConsoleConfig()
	mp.Config.MysqlDatasource = "pixty@/pixty?charset=utf8"
	mp.DiInit()

	for {
		db, err := mp.mainPers.dbc.getDb()
		if err == nil {
			_, err = db.Query("select * from camera")
			if err == nil {
				log4g.GetLogger("asdf").Info("Ping ok")
			} else {
				log4g.GetLogger("asdf").Info("Ping ne ok ", err, "  ping=", mp.mainPers.dbc.db.Ping())
				//mp.mainPers.dbc.reConnect()
			}
		} else {
			log4g.GetLogger("asdf").Info("Jopa")
		}

		//		p := gorivets.CheckPanic(func() {
		//			err := mp.mainPers.dbc.db.Ping()
		//			if err == nil {
		//				log4g.GetLogger("asdf").Info("Ping ok")
		//			} else {
		//				log4g.GetLogger("asdf").Info("Ping ne ok ", err)
		//			}
		//		})

		//		if p != nil {
		//			log4g.GetLogger("asdf").Info("it panics ", p)
		//		}

		time.Sleep(1000 * time.Millisecond)
	}

}
