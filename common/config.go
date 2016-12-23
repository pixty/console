package common

import "flag"

const (
	CnConsoleConfig = "ConsoleConfig"

	// *** Component phases ***
)

type ConsoleConfig struct {
	// Logging configuration file name
	LogConfigFN string

	// router http port
	HttpPort int

	// Debug mode
	DebugMode bool
}

// Set up default config values
func NewConsoleConfig() *ConsoleConfig {
	cc := &ConsoleConfig{}
	cc.HttpPort = 8080
	return cc
}

// This function parses CL args and apply them on top of ConsoleConfig instance
func (cc *ConsoleConfig) ParseCLArgs() bool {
	var help bool

	flag.StringVar(&cc.LogConfigFN, "log-config", "", "The log4g configuration file name")
	flag.IntVar(&cc.HttpPort, "port", cc.HttpPort, "The http port the console will listen on")
	flag.BoolVar(&help, "help", false, "Prints the usage")
	flag.BoolVar(&cc.DebugMode, "debug", false, "Run in debug mode")

	flag.Parse()

	if help {
		flag.Usage()
		return false
	}

	return true
}
