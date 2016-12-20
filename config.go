package main

import "flag"
import "errors"

var cc *ConsoleConfig

type ConsoleConfig struct {
	// Logging configuration file name
	logConfigFN string

	// router http port
	httpPort int
}

// Set up default config values
func init() {
	cc = &ConsoleConfig{}
	cc.httpPort = 8080
}

// This function parses CL args and apply them on top of ConsoleConfig instance
func parseCLArgs() error {
	var help bool

	flag.StringVar(&cc.logConfigFN, "log-config", "", "The log4g configuration file name")
	flag.IntVar(&cc.httpPort, "port", cc.httpPort, "The http port the console will listen on")
	flag.BoolVar(&help, "help", false, "Prints the usage")

	flag.Parse()

	if help {
		flag.Usage()
		return errors.New("Exit after helping")
	}

	return nil
}
