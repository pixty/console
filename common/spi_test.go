package common

import (
	"testing"

	"github.com/jrivets/log4g"
)

func TestPerformance(t *testing.T) {
	log := log4g.GetLogger("pt")
	count := 10000
	log.Info("Making ", count, " vectors...")
	vects := make([]V128D, count, count)
	for i := 0; i < count; i++ {
		vects[i] = newTestV128D()
	}
	log.Info("Done...")

	v := newTestV128D()
	edge := 3.734
	cnt := 10
	log.Info("doing match MatchAdvancedV128D() over... ", count*cnt, " comparisons")
	matches := 0
	start := CurrentTimestamp()
	for j := 0; j < cnt; j++ {
		for i := 0; i < count; i++ {
			if MatchAdvancedV128D(vects[i], v, edge) {
				matches++
			}
		}
	}
	log.Info("completed in ", CurrentTimestamp()-start, "ms, matches=", matches)

	log.Info("doing match MatchAdvanced2V128D() over... ", count*cnt, " comparisons")
	matches = 0
	start = CurrentTimestamp()
	for j := 0; j < cnt; j++ {
		for i := 0; i < count; i++ {
			if MatchAdvanced2V128D(vects[i], v, edge*edge) {
				matches++
			}
		}
	}
	log.Info("completed in ", CurrentTimestamp()-start, "ms, matches=", matches)

	log.Info("doing match MatchV128D() over... ", count*cnt, " comparisons")
	start = CurrentTimestamp()
	matches = 0
	for j := 0; j < cnt; j++ {
		for i := 0; i < count; i++ {
			if MatchV128D(vects[i], v, edge) {
				matches++
			}
		}
	}
	log.Info("completed in ", CurrentTimestamp()-start, "ms, matches=", matches)
}

func TestV128Conv(t *testing.T) {
	v := newTestV128D()
	b := v.ToByteSlice()
	v2 := NewV128D()
	v2.Assign(b)
	if !v.Equals(v2) {
		t.Fatal("v1 should be equal to v2")
	}
	v2[2] = -v2[2]
	if v.Equals(v2) {
		t.Fatal("v1 should NOT be equal to v2")
	}
}

func TestNewSecretKey(t *testing.T) {
	log := log4g.GetLogger("sk")
	for i := 0; i < 100; i++ {
		nsc := NewSecretKey(7)
		log.Info(nsc, " and it's hash=", Hash(nsc))
	}
}

func TestBytes2String(t *testing.T) {
	log := log4g.GetLogger("bytesHash")
	log.Info("ABCD result gives - ", bytes2String([]byte{0xAB, 0xCD}, "0123456789ABCDEF", 4))
	log.Info("10238A result gives - ", bytes2String([]byte{0x10, 0x23, 0x8A}, "0123456789ABCDEF", 6))
	log4g.Shutdown()
}

func TestNewSession(t *testing.T) {
	log := log4g.GetLogger("sessId")
	for i := 0; i < 10; i++ {
		sessId := NewSession()
		log.Info(sessId)
	}
}

func newTestV128D() V128D {
	res := NewV128D()
	return res.FillRandom()
}
