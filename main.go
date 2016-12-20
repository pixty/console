package main

import "github.com/jrivets/log4g"

func main() {
	injector := newInjector()

	defer injector.shutdown()
	defer log4g.Shutdown()

	if cc.logConfigFN != "" {
		log4g.ConfigF(cc.logConfigFN)
	}

	injector.Register(&Component{Component: cc})
	injector.construct()

	for {
	}
}
