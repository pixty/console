package common

import "flag"

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

	// Persistence settings
	MongoAddress    string
	MongoTimeoutSec int
	MongoDatabase   string
	MongoUser       string
	MongoPasswd     string
	MongoDebugMode  bool

	// Please refer to https://github.com/go-sql-driver/mysql about DSN
	// example: "id:password@tcp(your-amazonaws-uri.com:3306)/dbname" etc.
	MysqlDatasource string

	// Local File System Blob Storage
	LbsDir     string
	LbsMaxSize int64

	// HTTP images endpoint prefix
	ImgsPrefix string
}

// Set up default config values
func NewConsoleConfig() *ConsoleConfig {
	cc := &ConsoleConfig{}
	cc.HttpPort = 8080
	cc.GrpcFPCPPort = 50051
	cc.GrpcFPCPSessCapacity = 10000
	cc.MongoAddress = "127.0.0.1:27017"
	cc.MongoTimeoutSec = 60
	cc.MongoDatabase = "pixty"
	cc.MysqlDatasource = "pixty@/pixty?charset=utf8"
	cc.LbsDir = "/tmp/lfsBlobStorage"
	cc.LbsMaxSize = 1000000000 // 1gig
	cc.ImgsPrefix = "http://127.0.0.1:8080/images/"
	return cc
}

// This function parses CL args and apply them on top of ConsoleConfig instance
func (cc *ConsoleConfig) ParseCLArgs() bool {
	var help bool

	flag.StringVar(&cc.LogConfigFN, "log-config", "", "The log4g configuration file name")
	flag.IntVar(&cc.HttpPort, "port", cc.HttpPort, "The http port the console will listen on")
	flag.IntVar(&cc.GrpcFPCPPort, "fpcp-port", cc.GrpcFPCPPort, "The gRPC port for serving FPCP from cameras")
	flag.BoolVar(&help, "help", false, "Prints the usage")
	flag.BoolVar(&cc.DebugMode, "debug", false, "Run in debug mode")

	flag.Parse()

	if help {
		flag.Usage()
		return false
	}

	return true
}
