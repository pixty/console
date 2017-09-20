package main

import (
	"github.com/jrivets/inject"
	"github.com/jrivets/log4g"
	"github.com/pixty/console/common"
	"github.com/pixty/console/model"
	"github.com/pixty/console/rapi"
	"github.com/pixty/console/service"
	"github.com/pixty/console/service/auth"
	"github.com/pixty/console/service/email"
	"github.com/pixty/console/service/fpcp_serv"
	"github.com/pixty/console/service/image"
	"github.com/pixty/console/service/scene"
	"github.com/pixty/console/service/storage"
	"golang.org/x/net/context"
)

func main() {
	cc := initConsoleConfig()
	if cc == nil {
		return
	}

	initLoging(cc)
	logger := log4g.GetLogger("pixty")
	if cc.DebugMode {
		log4g.SetLogLevel("pixty", log4g.DEBUG)
		logger.Info("Running in DEBUG mode")
	}
	logger.Info("Config, eventually is ", cc.NiceString())

	injector := inject.NewInjector(log4g.GetLogger("pixty.injector"), log4g.GetLogger("fb.injector"))

	defer injector.Shutdown()
	defer log4g.Shutdown()

	mainCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	imgSrvc := image.NewImageService()
	restApi := rapi.NewAPI()
	msqlPersist := model.NewMysqlPersister()
	lbs := storage.NewLfsBlobStorage(cc.LbsDir, cc.GetLbsMaxSizeBytes())
	fpcp := fpcp_serv.NewFPCPServer()
	scnProc := scene.NewSceneProcessor()
	dtaCtrlr := service.NewDataController()
	authService := auth.NewAuthService()
	sessService := auth.NewInMemSessionService()
	esender := email.NewEmailSender()

	injector.RegisterMany(cc, restApi, fpcp, dtaCtrlr, authService, sessService, lbs, esender, imgSrvc)
	injector.RegisterOne(msqlPersist, "persister")
	injector.RegisterOne(mainCtx, "mainCtx")
	injector.RegisterOne(scnProc, "scnProcessor")
	injector.Construct()

	restApi.Run()
}

func initConsoleConfig() *common.ConsoleConfig {
	cc := common.NewConsoleConfig()
	if !cc.ParseCLArgs() {
		return nil
	}
	return cc
}

func initLoging(cc *common.ConsoleConfig) {
	if cc.LogConfigFN != "" {
		log4g.GetLogger("pixty").Info("Loading log4g configuartion from ", cc.LogConfigFN)
		err := log4g.ConfigF(cc.LogConfigFN)
		if err != nil {
			panic(err)
		}
	}
}
