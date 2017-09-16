package image

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"math"
	"time"

	"github.com/jrivets/log4g"
	"github.com/nfnt/resize"
	"github.com/pixty/console/common"
	"github.com/pixty/console/service/storage"
)

type (
	ImageService struct {
		BlobStorage storage.BlobStorage `inject:""`
		logger      log4g.Logger
		dfltSize    byte
	}
)

func NewImageService() *ImageService {
	ims := new(ImageService)
	ims.logger = log4g.GetLogger("pixty.ImageService")
	ims.dfltSize = IMG_SIZE_640x480
	return ims
}

// ============================== ImageService ===============================
// **** Public interface ****

// Returns the image by the desired width or height. If the desired dimensions
// width(w) and height(h) are less or equal 0, then the default size will be used
func (ims *ImageService) GetImageByFileName(fileName string, w, h int) (io.ReadCloser, error) {
	var imd img_desc
	err := imd.parseFileName(fileName)
	if err != nil {
		return nil, err
	}
	imd.format = IMG_FRMT_JPEG
	imd.size = ims.getSizeCode(w, h)

	for {
		id := imd.getStoreId()
		ims.logger.Debug("Get image by filename ", fileName, ", id=", id)
		rdr, bm := ims.BlobStorage.Read(id)
		if bm != nil {
			return rdr, nil
		}

		if imd.size == IMG_SIZE_ORIGINAL {
			return nil, common.NewError(common.ERR_NOT_FOUND, "Could not find image "+fileName)
		}
		imd.size = nextSizeCode(imd.size)
	}
}

func (ims *ImageService) DeleteImageByFile(fileName string) error {
	var imd img_desc
	err := imd.parseFileName(fileName)
	if err != nil {
		return err
	}
	return ims.BlobStorage.Delete(imd.getPossibleIDs()...)
}

// Stores the frame params:
// camId - the camera where the image comes from
// frameId - the original frame Id
// img - the pure image of the frame (not coded, just the image)
// rects - rectangles on the original frame that should be turned into a separated pictures as well
//
// Returns - list of file-names the frame and rectangles can be selected by
func (ims *ImageService) StoreNewFrame(camId, frameId int64, img image.Image, rects []image.Rectangle) ([]string, error) {
	res := make([]string, 0, len(rects)+1)

	prefix := PFX_TEMP
	if len(rects) > 0 {
		prefix = PFX_PERM
	}

	// Reduce original size if needed
	sImg, _ := ims.scaleImage(ims.dfltSize, img)
	// but save it like an original
	iDsc := &img_desc{prefix, camId, frameId, nil, IMG_SIZE_ORIGINAL, IMG_FRMT_JPEG}
	err := ims.storeImage(iDsc, sImg)
	if err != nil {
		return res, err
	}
	res = append(res, iDsc.getFileName())

	// Walk through all faces and make some pics
	for _, rect := range rects {
		si := img.(interface {
			SubImage(r image.Rectangle) image.Image
		}).SubImage(rect)

		// scale to default
		sImg, _ := ims.scaleImage(ims.dfltSize, si)
		// but save it like an original
		iDsc.size = IMG_SIZE_ORIGINAL
		iDsc.rect = &rect
		err := ims.storeImage(iDsc, sImg)
		if err != nil {
			return res, err
		}
		res = append(res, iDsc.getFileName())

		for i := sizeCodesMap[ims.dfltSize] - 1; i >= 0; i-- {
			sc := sizeCodes[i]
			iDsc.size = sc
			sImg, ok := ims.scaleImage(ims.dfltSize, img)
			if ok {
				err = ims.storeImage(iDsc, sImg)
				if err != nil {
					ims.logger.Error("Could not save image ", iDsc, ", err=", err)
				}
			}
		}
	}
	return res, nil
}

// Scales the image to the provided size. Will do nothing if the original size
// is less than requested. The second returned param shows whether the transformation
// was done or not
func (ims *ImageService) scaleImage(size byte, img image.Image) (image.Image, bool) {
	// Original dimensions
	w := img.Bounds().Dx()
	h := img.Bounds().Dy()

	nsImg := img

	// First resize if needed
	if size != IMG_SIZE_ORIGINAL {
		dw, dh := getDimensionsBySizeCode(size)

		if w == 0 || h == 0 || dw == 0 || dh == 0 {
			ims.logger.Warn("Cannot save an image with 0 width or height: w=", w, ", h=", h, ", dw=", dw, ", dh=", dh)
			return nsImg, false
		}

		ddw := float64(dw) / float64(w)
		ddh := float64(dh) / float64(h)

		if ddw >= 1.0 && ddh >= 1.0 {
			// desired size is bigger than original, do nothing, we don't streach
			ims.logger.Debug("Orginal size less than desired, don't streach: ddw=", ddw, ", ddh=", ddh)
			return nsImg, false
		}

		nw := uint(dw)
		nh := uint(dh)
		if ddw < ddh {
			nh = uint(math.Max(1.0, float64(h)*ddw))
		} else {
			nw = uint(math.Max(1.0, float64(w)*ddh))
		}

		ims.logger.Debug("Scale picutre w=", w, ", h=", h, " newW=", nw, ", newH=", nh)
		nsImg = resize.Resize(nw, nh, img, resize.Bilinear)
	}

	return nsImg, false
}

// Stores the provided image by the descriptor. If the operation
// is successful then nil will be returned
func (ims *ImageService) storeImage(iDsc *img_desc, img image.Image) error {
	bb := bytes.NewBuffer([]byte{})
	var err error
	if iDsc.format == IMG_FRMT_JPEG {
		err = jpeg.Encode(bb, img, nil)
	} else {
		err = png.Encode(bb, img)
	}

	// Store image
	bm := &storage.BlobMeta{Id: iDsc.getStoreId(), Timestamp: time.Now()}
	_, err = ims.BlobStorage.Add(bytes.NewReader(bb.Bytes()), bm)
	if err != nil {
		ims.logger.Error("Could not write data to data store, err=", err)
		return err
	}
	return nil
}

// **** Private interface ****
func (ims *ImageService) getSizeCode(w, h int) byte {
	if w <= 0 && h <= 0 {
		return ims.dfltSize
	}

	if w <= 0 {
		return getSizeCodeByHeight(h)
	}

	if h <= 0 {
		return getSizeCodeByWidth(w)
	}

	hs := getSizeCodeByHeight(h)
	ws := getSizeCodeByWidth(h)
	if sizeCodesMap[hs] > sizeCodesMap[ws] {
		return hs
	}

	return ws
}

func nextSizeCode(c byte) byte {
	i := sizeCodesMap[c]
	i++
	if i >= len(sizeCodes) {
		panic("something goes wrong. Cannot increase code size for last one " + string(c))
	}
	return sizeCodes[i]
}

func getSizeCodeByHeight(h int) byte {
	if h <= 100 {
		return IMG_SIZE_160x100
	}
	if h <= 200 {
		return IMG_SIZE_320x200
	}
	if h <= 480 {
		return IMG_SIZE_640x480
	}
	if h <= 600 {
		return IMG_SIZE_800x600
	}
	return IMG_SIZE_ORIGINAL
}

func getSizeCodeByWidth(w int) byte {
	if w <= 160 {
		return IMG_SIZE_160x100
	}
	if w <= 320 {
		return IMG_SIZE_320x200
	}
	if w <= 640 {
		return IMG_SIZE_640x480
	}
	if w <= 800 {
		return IMG_SIZE_800x600
	}
	return IMG_SIZE_ORIGINAL
}

// returns dime
func getDimensionsBySizeCode(sc byte) (int, int) {
	switch sc {
	case IMG_SIZE_160x100:
		return 160, 100
	case IMG_SIZE_320x200:
		return 320, 200
	case IMG_SIZE_640x480:
		return 640, 480
	case IMG_SIZE_800x600:
		return 800, 600
	default:
		return 0, 0
	}
}
