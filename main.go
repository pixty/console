package main

import "github.com/jrivets/log4g"
import "github.com/jrivets/inject"
import "github.com/pixty/console/rapi"
import "github.com/pixty/console/common"
import "github.com/pixty/console/service"
import "github.com/pixty/console/model"

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

	injector := inject.NewInjector(log4g.GetLogger("console.injector"))

	defer injector.Shutdown()
	defer log4g.Shutdown()

	restApi := rapi.NewAPI()
	mongo := model.NewMongoPersister()
	camService := service.NewDefaultCameraService()
	orgService := service.NewDefaultOrgService()
	imgService := service.NewDefaultImageService()
	lbs := service.NewLfsBlobStorage()
	defContextFactory := common.NewContextFactory()

	injector.RegisterMany(cc, restApi)
	injector.RegisterOne(camService, "camService")
	injector.RegisterOne(orgService, "orgService")
	injector.RegisterOne(imgService, "imgService")
	injector.RegisterOne(lbs, "blobStorage")
	injector.RegisterOne(mongo, "persister")
	injector.RegisterOne(defContextFactory, "ctxFactory")
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
