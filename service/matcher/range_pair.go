package matcher

import (
	"github.com/jrivets/gorivets"
)

type (
	range_pair struct {
		min int64
		max int64
	}
)

func cmp_range_pair(a, b interface{}) int {
	rpa := a.(*range_pair)
	rpb := b.(*range_pair)
	if rpa.min < rpb.min {
		return -1
	}
	if rpa.min > rpb.min {
		return 1
	}
	return 0
}
