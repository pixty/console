package common

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"math"

	"github.com/jrivets/gorivets"
	"github.com/jrivets/log4g"
)

const (
	CnConsoleConfig = "ConsoleConfig"

	// *** Component phases ***
	CMP_PHASE_BLOB_STORE    = 0
	CMP_PHASE_DB            = 1
	CMP_PHASE_SCENE_SERVICE = 2
	CMP_PHASE_FPCP          = 3
)

type ConsoleConfig struct {
	// Logging configuration file name
	LogConfigFN string

	// router http port
	HttpPort      int
	HttpDebugMode bool

	// grpc (FPCP) config
	GrpcFPCPPort int
	// how many sessions (connection) can be kept in the FPCP at a time
	GrpcFPCPSessCapacity int

	// Debug mode
	DebugMode bool

	// Please refer to https://github.com/go-sql-driver/mysql about DSN
	// example: "id:password@tcp(your-amazonaws-uri.com:3306)/dbname" etc.
	MysqlDatasource string

	// Local File System Blob Storage
	LbsDir     string
	LbsMaxSize string

	// HTTP images endpoint prefix
	ImgsPrefix string
	// how long to keep temporary images
	ImgsTmpTTLSec int

	// AuthN, AuthZ
	AuthMaxSessions  int
	AuthSessionTOSec int

	// Email
	EmailSmtpServer string
	EmailSmtpUser   string
	EmailSmtpPasswd string

	logger log4g.Logger
}

func (cc *ConsoleConfig) NiceString() string {
	return fmt.Sprint("{\n\tLogConfigFN=", cc.LogConfigFN, ",\n\tHttpPort=", cc.HttpPort, ",\n\tHttpDebugMode=", cc.HttpDebugMode,
		",\n\tGrpcFPCPPort=", cc.GrpcFPCPPort, ",\n\tGrpcFPCPSessCapacity=", cc.GrpcFPCPSessCapacity, ",\n\tDebugMode=",
		cc.DebugMode, ",\n\tMysqlDatasource=", cc.MysqlDatasource, ",\n\tLbsDir=", cc.LbsDir, ",\n\tLbsMaxSize=", cc.LbsMaxSize,
		"(", cc.GetLbsMaxSizeBytes(), "bytes)", ",\n\tImgsPrefix=", cc.ImgsPrefix, ",\n\tImgsTmpTTLSec=", cc.ImgsTmpTTLSec, "\n}")
}

func (cc *ConsoleConfig) String() string {
	return fmt.Sprint("ConsoleConfig: {HttpDebugMode=", cc.HttpDebugMode, ", DebugMode=",
		cc.DebugMode, "}")
}

// Set up default config values
func NewConsoleConfig() *ConsoleConfig {
	cc := &ConsoleConfig{}
	cc.HttpPort = 8080
	cc.GrpcFPCPPort = 50051
	cc.GrpcFPCPSessCapacity = 10000
	cc.MysqlDatasource = "pixty@/pixty?charset=utf8"
	cc.LbsDir = "/opt/pixty/store"
	cc.LbsMaxSize = "20G"
	cc.ImgsPrefix = "http://127.0.0.1:8080/images/"
	cc.ImgsTmpTTLSec = 60
	cc.AuthMaxSessions = 3               // same user can open up to 3 sessions (so far, then will reduce)
	cc.AuthSessionTOSec = 300            // kick it out in 5 minutes
	cc.EmailSmtpServer = "mail.name.com" // mail.name.com:465?
	cc.EmailSmtpUser = "support@pixty.io"
	cc.logger = log4g.GetLogger("pixty.ConsoleConfig")
	return cc
}

func (cc *ConsoleConfig) apply(cc1 *ConsoleConfig) {
	if cc1.HttpPort > 0 {
		cc.HttpPort = cc1.HttpPort
	}
	if cc1.GrpcFPCPPort > 0 {
		cc.GrpcFPCPPort = cc1.GrpcFPCPPort
	}
	if cc1.GrpcFPCPSessCapacity > 0 {
		cc.GrpcFPCPSessCapacity = cc1.GrpcFPCPSessCapacity
	}
	if cc1.MysqlDatasource != "" {
		cc.MysqlDatasource = cc1.MysqlDatasource
	}
	if cc1.LbsDir != "" {
		cc.LbsDir = cc1.LbsDir
	}
	if cc1.LbsMaxSize != "" {
		cc.LbsMaxSize = cc1.LbsMaxSize
	}
	if cc1.ImgsPrefix != "" {
		cc.ImgsPrefix = cc1.ImgsPrefix
	}
	if cc1.ImgsTmpTTLSec != 0 {
		cc.ImgsTmpTTLSec = cc1.ImgsTmpTTLSec
	}
	if cc1.logger != nil {
		cc.logger = cc1.logger
	}
	if cc1.DebugMode {
		cc.DebugMode = true
	}
	if cc1.HttpDebugMode {
		cc.HttpDebugMode = true
	}
	if cc1.AuthMaxSessions > 0 {
		cc.AuthMaxSessions = cc1.AuthMaxSessions
	}
	if cc1.AuthSessionTOSec > 0 {
		cc.AuthSessionTOSec = cc1.AuthSessionTOSec
	}
}

// This function parses CL args and apply them on top of ConsoleConfig instance
func (ccFinal *ConsoleConfig) ParseCLArgs() bool {
	cc := &ConsoleConfig{logger: ccFinal.logger}

	var help bool
	var cfgFile string

	flag.StringVar(&cfgFile, "config-file", "./pixty_console.json", "The console configuration file")
	flag.StringVar(&cc.LogConfigFN, "log-config", "", "The log4g configuration file name")
	flag.IntVar(&cc.HttpPort, "port", cc.HttpPort, "The http port the console will listen on")
	flag.IntVar(&cc.GrpcFPCPPort, "fpcp-port", cc.GrpcFPCPPort, "The gRPC port for serving FPCP from cameras")
	flag.BoolVar(&help, "help", false, "Prints the usage")
	flag.BoolVar(&cc.DebugMode, "debug", false, "Run in debug mode")
	flag.BoolVar(&cc.HttpDebugMode, "http-debug", false, "Run in http-debug mode")

	flag.Parse()

	// read config from file, if provided
	ccf := &ConsoleConfig{logger: ccFinal.logger}
	ccf.readFromFile(cfgFile)

	// overwrite the config file settings by command-line params
	ccf.apply(cc)

	// Apply finally to the original config
	ccFinal.apply(ccf)

	if help {
		flag.Usage()
		return false
	}

	return true
}

func (cc *ConsoleConfig) readFromFile(filename string) {
	if filename == "" {
		return
	}
	cfgData, err := ioutil.ReadFile(filename)
	if err != nil {
		cc.logger.Warn("Could not read configuration file ", filename, ", err=", err)
		return
	}

	cfg := &ConsoleConfig{}
	err = json.Unmarshal(cfgData, cfg)
	if err != nil {
		cc.logger.Warn("Could not unmarshal data from ", filename, ", err=", err)
		return
	}
	cc.logger.Info("Configuration read from ", filename)
	cc.apply(cfg)
}

func (cc *ConsoleConfig) GetLbsMaxSizeBytes() int64 {
	res, err := gorivets.ParseInt64(cc.LbsMaxSize, 1000000, math.MaxInt64, 1000000000)
	if err != nil {
		cc.logger.Fatal("Could not parse LBS size=", cc.LbsMaxSize, " panicing!")
		panic(err)
	}
	return res
}
