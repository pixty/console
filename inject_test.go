package main

import "testing"
import "errors"

var initC []interface{} = make([]interface{}, 0)
var deinit []interface{} = make([]interface{}, 0)

type c1 struct {
	C3Ref *c3 `inject:""`
	err   bool
	panic bool
}

type c2 struct {
	C1Ref *c1 `inject:""`
	err   bool
	panic bool
}

type c3 struct {
	C2Ref *c2 `inject:"c2"`
	err   bool
	panic bool
}

func (*c1) DiPhase() int {
	return 10
}

func (c *c1) DiInit() error {
	if c.err {
		return errors.New("c1 error")
	}
	if c.panic {
		panic("c1 panic")
	}
	initC = append(initC, c)
	return nil
}

func (c *c1) DiShutdown() {
	deinit = append(deinit, c)
}

func (*c3) DiPhase() int {
	return 1
}

func (c *c3) DiInit() error {
	if c.err {
		return errors.New("c3 error")
	}
	if c.panic {
		panic("c3 panic")
	}
	initC = append(initC, c)
	return nil
}

func (c *c3) DiShutdown() {
	deinit = append(deinit, c)
}

func TestInjectStraight(t *testing.T) {
	inj := newInjector()

	c1Inst := &c1{}
	c2Inst := &c2{}
	c3Inst := &c3{}

	inj.Register(&Component{Component: c1Inst})
	inj.Register(&Component{Component: c2Inst, Name: "c2"}, &Component{Component: c3Inst})
	initC = make([]interface{}, 0)
	deinit = make([]interface{}, 0)
	inj.construct()

	if c1Inst.C3Ref != c3Inst {
		t.Fatal("c1 doesn't refer to c3")
	}

	if c2Inst.C1Ref != c1Inst {
		t.Fatal("c2 doesn't refer to c1")
	}

	if c3Inst.C2Ref != c2Inst {
		t.Fatal("c3 doesn't refer to c2")
	}

	if len(initC) != 2 || initC[0] != c3Inst || initC[1] != c1Inst {
		t.Fatal("2 inits should happen")
	}

	if len(deinit) != 0 {
		t.Fatal("0 deinits should happen")
	}

	inj.shutdown()

	if len(deinit) != 2 || deinit[0] != c1Inst || deinit[1] != c3Inst {
		t.Fatal("2 de-inits should happen")
	}
}

func TestInjectNew(t *testing.T) {
	inj := newInjector()

	c1Inst := &c1{}
	c2Inst := &c2{}
	c22Inst := &c2{}

	inj.Register(&Component{Component: c1Inst, Name: "aaaa"})
	inj.Register(&Component{Component: c2Inst, Name: "bbbb"})
	inj.Register(&Component{Component: c22Inst, Name: "c2"})

	initC = make([]interface{}, 0)
	inj.construct()

	if c1Inst.C3Ref == nil {
		t.Fatal("c1 is not initialized")
	}

	if c2Inst.C1Ref == nil {
		t.Fatal("c2 is not initialized")
	}

	if c1Inst.C3Ref.C2Ref != c22Inst {
		t.Fatal("c3 doesn't refer to c2")
	}

	if len(initC) != 2 || initC[0] != c1Inst.C3Ref || initC[1] != c1Inst {
		t.Fatal("2 inits should happen!")
	}

	inj.shutdown()
}
