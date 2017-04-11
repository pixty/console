package main

import (
	"github.com/jrivets/inject"
	"github.com/jrivets/log4g"
	"github.com/pixty/console/common"
	"github.com/pixty/console/model"
	"github.com/pixty/console/rapi"
	"github.com/pixty/console/service"
	"golang.org/x/net/context"
)

func main() {
	cc := initConsoleConfig()
	if cc == nil {
		return
	}

	initLoging(cc)
	logger := log4g.GetLogger("console")
	if cc.DebugMode {
		log4g.SetLogLevel("console", log4g.DEBUG)
		logger.Info("Running in DEBUG mode")
	}

	injector := inject.NewInjector(log4g.GetLogger("console.injector"), log4g.GetLogger("fb.injector"))

	defer injector.Shutdown()
	defer log4g.Shutdown()

	mainCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	restApi := rapi.NewAPI()
	mongo := model.NewMongoPersister()
	imgService := service.NewDefaultImageService()
	lbs := service.NewLfsBlobStorage(cc.LbsDir)

	injector.RegisterMany(cc, restApi)
	injector.RegisterOne(imgService, "imgService")
	injector.RegisterOne(lbs, "blobStorage")
	injector.RegisterOne(mongo, "persister")
	injector.RegisterOne(mainCtx, "mainCtx")
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
		log4g.ConfigF(cc.LogConfigFN)
	}
}
