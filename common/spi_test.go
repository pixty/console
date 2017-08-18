package common

import (
	"math/rand"
	"testing"

	"github.com/jrivets/log4g"
)

func TestPerformance(t *testing.T) {
	log := log4g.GetLogger("pt")
	count := 100000
	log.Info("Making ", count, " vectors...")
	vects := make([]V128D, count, count)
	for i := 0; i < count; i++ {
		vects[i] = newV128D()
	}
	log.Info("Done...")

	v := newV128D()
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

func newV128D() V128D {
	var res V128D = make([]float32, 128, 128)
	for i := 0; i < 128; i++ {
		res[i] = rand.Float32()
	}
	return res
}
