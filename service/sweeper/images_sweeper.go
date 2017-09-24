package sweeper

import (
	"context"
	"fmt"
	"time"

	"github.com/jrivets/gorivets"
	"github.com/jrivets/log4g"
	"github.com/pixty/console/common"
	"github.com/pixty/console/model"
	"github.com/pixty/console/service/image"
)

type (
	ImagesSweeper interface {
	}

	images_sweeper struct {
		CConfig    *common.ConsoleConfig `inject:""`
		Persister  model.Persister       `inject:"persister"`
		MainCtx    context.Context       `inject:"mainCtx"`
		ImgService *image.ImageService   `inject:""`
		stats      *images_swpr_stats
		logger     log4g.Logger
	}

	images_swpr_stats struct {
		startedAt   time.Time
		from        common.Timestamp
		dbTrans     int
		picsDeleted int
		err         error
	}
)

func NewImagesSweeper() ImagesSweeper {
	return new(images_sweeper)
}

// ========================== PostConstructor ================================
func (is *images_sweeper) DiPostConstruct() {
	is.logger = log4g.GetLogger("pixty.ImagesSweeper")
	is.logger.Info("Post construct.")
	is.stats = new(images_swpr_stats)

	go func() {
		is.logger.Info("Entering job routine.")
		for {
			select {
			case <-is.MainCtx.Done():
				is.logger.Info("Leaving job routine.")
				return
			// kick the sweeping routine
			case <-time.After(time.Second * time.Duration(is.CConfig.SweepImagesToSec)):
				err := gorivets.CheckPanic(is.sweepImages)
				if err != nil {
					is.logger.Error("Got the panic in sweeping images: ", err)
				}
			}
		}
	}()
}

func (is *images_sweeper) sweepImages() {
	pxt, err := is.Persister.GetPartitionTx("FAKE")
	if err != nil {
		is.logger.Error("Could not get PartitionTx object err=", err)
		return
	}

	is.stats.start()
	for is.sweepImagesTx(pxt, is.CConfig.SweepImagesPackSize) {
		if is.CConfig.SweepImagesPackSizePauseMs > 0 {
			time.Sleep(time.Millisecond * time.Duration(is.CConfig.SweepImagesPackSizePauseMs))
		}
	}
	is.logger.Info("Done with sweeping images. Stats is \"", is.stats, "\"")
}

func (is *images_sweeper) sweepImagesTx(pxt model.PartTx, limit int) bool {
	err := pxt.Begin()
	if err != nil {
		is.logger.Error("Could not start transaction, err=", err)
		is.stats.onError(err)
		return false
	}
	is.stats.incDbTran()
	defer pxt.Commit()

	pics, err := pxt.FindZeroRefPics(limit)
	if err != nil {
		is.logger.Error("Could not read ", limit, " pictures, err=", err)
		pxt.Rollback()
		is.stats.onError(err)
		return false
	}

	for i, p := range pics {
		// TODO. Are we sure about making IDs the way?
		err = is.ImgService.DeleteImageByFile(p.Id)
		if err != nil {
			is.stats.onError(err)
			is.logger.Error("Could not delete picture by id=", p.Id, ", err=", err)
			pics = pics[:i]
			break
		}
		// Ok
	}

	err = pxt.DeletePics(pics)
	if err != nil {
		is.stats.onError(err)
		is.logger.Error("Could not delete pictures ", pics, ", err=", err)
		pxt.Rollback()
		return false
	}
	is.stats.onPicsDeleted(len(pics))
	return len(pics) > 0
}

func (iss *images_swpr_stats) String() string {
	return fmt.Sprint("transactions=", iss.dbTrans, ", picsDeleted=", iss.picsDeleted, ", for ", time.Now().Sub(iss.startedAt), ", err=", iss.err)
}

func (iss *images_swpr_stats) start() {
	iss.startedAt = time.Now()
	iss.picsDeleted = 0
	iss.dbTrans = 0
	iss.err = nil
}

func (iss *images_swpr_stats) onError(err error) {
	iss.err = err
}

func (iss *images_swpr_stats) incDbTran() {
	iss.dbTrans++
}

func (iss *images_swpr_stats) onPicsDeleted(pics int) {
	iss.picsDeleted += pics
}
