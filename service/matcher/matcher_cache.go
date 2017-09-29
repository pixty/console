package matcher

import (
	"fmt"

	"github.com/pixty/console/model"
)

type (
	cache struct {
		Persister model.Persister `inject:""`
		orgBlocks map[int64]*matcher_block
	}

	cache_block struct {
		// records sorted in ascending MG order
		records []*cache_block_rec
		// for first block it is 0!
		startIdx  int64
		endIdx    int64
		lastBlock bool
	}

	cache_block_rec struct {
		person *model.Person
		faces  []*model.Face
	}
)

func (cb *cache_block) String() string {
	fmt.Sprint("{records=", len(cb.records), ", startIdx=", cb.startIdx, ", endIdx=", cb.endIdx, ", lastBlock=", cb.lastBlock, "}")
}

func (cbr *cache_block_rec) String() string {
	fmt.Sprint("{personId=", cbr.person.Id, ", faces=", len(cbr.faces), "}")
}

// read the next cache block and return it when it is ready. returns nil, when over
func (ch *cache) nextCacheBlock(orgId int64) *cahce_block {

}

// when a match happens
func (cbf *cache_block) onMatch(cbr *cache_block_rec, pd *person_desc) {
	// TODO: assign MG, update block, write into DB...
}

// all faces were checked and nothing was found, now assign new MG
func (cbf *cache_block) onNewMG(pd *person_desc) {
}
