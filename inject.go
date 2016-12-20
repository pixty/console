package main

import "github.com/facebookgo/inject"
import "github.com/jrivets/log4g"
import "github.com/jrivets/gorivets"
import "errors"
import "bytes"
import "strconv"

type Injector struct {
	logger      log4g.Logger
	fbInjector  *inject.Graph
	lcComps     *gorivets.SortedSlice
	constructed bool
}

type Component struct {
	Component interface{}
	Name      string
}

// A component can implement the interface when the component has a life-cycle
// All life-cyclers are ordered by DiPhase value after creation and injection.
// Components with lower value are initialized first
type LifeCycler interface {
	DiPhase() int
	DiInit() error
	DiShutdown()
}

func (c *Component) lifeCycler() LifeCycler {
	lc, ok := c.Component.(LifeCycler)
	if ok {
		return lc
	}
	return nil
}

func (c *Component) getPhase() int {
	return c.lifeCycler().DiPhase()
}

func (c *Component) String() string {
	lc := c.lifeCycler()
	var buffer bytes.Buffer
	buffer.WriteString("Component: {\"")
	buffer.WriteString("\", lifeCycler: ")
	if !gorivets.IsNil(lc) {
		buffer.WriteString("yes, phase: ")
		buffer.WriteString(strconv.FormatInt(int64(lc.DiPhase()), 10))
	} else {
		buffer.WriteString("no")
	}
	buffer.WriteString("}")
	return buffer.String()
}

var lfCompare = func(a, b interface{}) int {
	p1 := a.(*Component).getPhase()
	p2 := b.(*Component).getPhase()
	return gorivets.CompareInt(p1, p2)
}

func newInjector() *Injector {
	fbInjector := &inject.Graph{}
	injector := &Injector{logger: log4g.GetLogger("console.injector"), fbInjector: fbInjector}
	fbInjector.Logger = injector
	return injector
}

func (i *Injector) Debugf(format string, v ...interface{}) {
	i.logger.Logf(log4g.DEBUG, format, v)
}

func (i *Injector) Register(comps ...*Component) {
	var objects []*inject.Object = make([]*inject.Object, len(comps))
	for idx, c := range comps {
		objects[idx] = &inject.Object{Value: c.Component, Name: c.Name}
		i.logger.Debug("Registering ", c)
	}
	i.fbInjector.Provide(objects...)
}

func (i *Injector) construct() {
	i.logger.Info("Initializing...")
	if i.constructed {
		panic("The dependency Inector already initialized. Injector.Construct() can be called once!")
	}
	i.constructed = true
	if err := i.fbInjector.Populate(); !gorivets.IsNil(err) {
		panic(err)
	}

	i.afterPopulation()
	if err := i.initLcs(); !gorivets.IsNil(err) {
		panic(err)
	}
}

func (i *Injector) shutdown() {
	i.shutdownLcs()
}

func (i *Injector) newLcComps() {
	size := 10
	if i.lcComps != nil {
		size = i.lcComps.Len()
	}
	i.lcComps, _ = gorivets.NewSortedSliceByComp(lfCompare, size)
}

// Scans all FB objects and make components from them.
// Build sorted list of life cyclers.
func (i *Injector) afterPopulation() {
	i.newLcComps()
	cmpMap := make(map[interface{}]*Component)
	i.logger.Debug("Scanning all objects after population")
	for _, o := range i.fbInjector.Objects() {
		if gorivets.IsNil(o.Value) {
			continue
		}

		if _, ok := cmpMap[o.Value]; ok {
			continue
		}
		comp := &Component{Component: o.Value, Name: o.Name}
		cmpMap[o.Value] = comp

		lfCycler := comp.lifeCycler()
		if lfCycler != nil {
			i.logger.Debug("Found LifeCycler ", comp)
			i.lcComps.Add(comp)
		}
	}
}

func (i *Injector) initLcs() error {
	i.logger.Info("Initializing life-cyclers (", i.lcComps.Len(), " will be initialized)")
	lcComps := i.lcComps.Copy()
	i.newLcComps()
	for _, comp := range lcComps {
		i.logger.Info("Initializing ", comp)
		c := comp.(*Component)
		lc := c.lifeCycler()
		if err := initLc(lc); !gorivets.IsNil(err) {
			i.shutdownLcs()
			return errors.New("Error while initializing ")
		}
		i.lcComps.Add(c)
	}
	return nil
}

func (i *Injector) shutdownLcs() {
	lcComps := i.lcComps.Copy()
	i.lcComps = nil
	for idx := len(lcComps) - 1; idx >= 0; idx-- {
		c := lcComps[idx].(*Component)
		i.logger.Info("Shutting down ", c)
		c.lifeCycler().DiShutdown()
	}
}

func initLc(lc LifeCycler) interface{} {
	return gorivets.CheckPanic(func() {
		if err := lc.DiInit(); !gorivets.IsNil(err) {
			panic(err)
		}
	})
}
