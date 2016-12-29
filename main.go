package main

import "github.com/jrivets/log4g"
import "github.com/jrivets/inject"
import "github.com/pixty/console/rapi"
import "github.com/pixty/console/common"
import "github.com/pixty/console/service"

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
	camService := service.NewDefaultCameraService()
	orgService := service.NewDefaultOrgService()
	injector.RegisterMany(cc, restApi)
	injector.RegisterOne(camService, "camService")
	injector.RegisterOne(orgService, "orgService")
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