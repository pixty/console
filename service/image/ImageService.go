package image

import (
	"io"

	"github.com/jrivets/log4g"
	"github.com/pixty/console/common"
	"github.com/pixty/console/service/storage"
)

type (
	ImageService struct {
		BlobStorage storage.BlobStorage `inject:""`
		logger      log4g.Logger
		dfltHeight  int
	}
)

const (
	IMG_HEIGHT_ORIGINAL = 1200 // selecting a number wich is different than constants below
	IMG_HEIGHT_600      = 600  // 800x600
	IMG_HEIGHT_480      = 480  // 640x480
	IMG_HEIGHT_200      = 200  // 320x200
	IMG_HEIGHT_100      = 100  // 160x100
)

func NewImageService() *ImageService {
	ims := new(ImageService)
	ims.logger = log4g.GetLogger("pixty.ImageService")
	ims.dfltHeight = IMG_HEIGHT_480
	return ims
}

// ============================== ImageService ===============================
// **** Public interface ****

// Returns the image by the desired height. If the desired height is nil, or it has
// a senseless value like Height <= 0, then the picture with default height will be returned.
func (ims *ImageService) GetImageByFileName(fileName string, dsrdHght int) (io.ReadCloser, error) {
	sz := ims.normalizeHeight(dsrdHght)
	rdr, err := ims.getImageReader(fileName, sz)
	if err != nil {
		return nil, err
	}
	if rdr == nil {
		if sz == IMG_HEIGHT_ORIGINAL {
			return nil, common.NewError(common.ERR_NOT_FOUND, "Could not find image "+fileName)
		}

	}
	return nil, nil
}

// Returns recommended height by the width
func (ims *ImageService) RecommendHeightByWidth(width int) int {
	if width <= 0 {
		return ims.dfltHeight
	}
	if width <= 160 {
		return IMG_HEIGHT_100
	}
	if width <= 320 {
		return IMG_HEIGHT_200
	}
	if width <= 640 {
		return IMG_HEIGHT_480
	}
	if width <= 800 {
		return IMG_HEIGHT_600
	}
	return IMG_HEIGHT_ORIGINAL
}

// **** Private interface ****

// normalize height to our scale
func (ims *ImageService) normalizeHeight(dsrdHght int) int {
	if dsrdHght <= 0 {
		return ims.dfltHeight
	}
	if dsrdHght < IMG_HEIGHT_100 {
		return IMG_HEIGHT_100
	}
	if dsrdHght < IMG_HEIGHT_200 {
		return IMG_HEIGHT_200
	}
	if dsrdHght < IMG_HEIGHT_480 {
		return IMG_HEIGHT_480
	}
	if dsrdHght < IMG_HEIGHT_600 {
		return IMG_HEIGHT_600
	}
	return IMG_HEIGHT_ORIGINAL
}

func (ims *ImageService) getImageReader(fileName string, height int) (io.ReadCloser, error) {
	return nil, nil
}
