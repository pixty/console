package image

import (
	"fmt"
	"image"
	"strconv"
	"strings"

	"github.com/pixty/console/common"
)

type (
	ImgDesc struct {
		// Can be regular or temporary object
		// PFX_PERM or PFX_TMP values
		Prefix string
		// camera Id, an integer
		CamId int64
		// frame Id from the camera, an integer
		FrameId int64
		// Rectangle on the orginal Id, or the original size
		Rect *image.Rectangle
		// The byte code one of the images
		Size   byte
		Format byte
	}
)

const (
	// Prefix code
	PFX_PERM = "cm"
	PFX_TEMP = "tcm"

	// Image sizes
	IMG_SIZE_ORIGINAL = 'o'
	IMG_SIZE_800x600  = 'l'
	IMG_SIZE_640x480  = 'm'
	IMG_SIZE_320x240  = 's'
	IMG_SIZE_160x120  = 't'

	// Image format
	IMG_FRMT_JPEG = 'j'
	IMG_FRMT_PNG  = 'p'
)

var sizeCodesMap = map[byte]int{'t': 0, 's': 1, 'm': 2, 'l': 3, 'o': 4}
var sizeCodes = []byte{'t', 's', 'm', 'l', 'o'}

// Get a file name, parse it and fill the ImageDesc object fields by
// the file fields values
// The filename format is expected in the following form:
// <Prefix>_<CamId>_<FrameId>_[<Rectangle>].jpeg
// <Rectangle> is encoded like Left-Top-Right-Bottom and optional
func (imd *ImgDesc) ParseFileName(fn string) error {
	// only jpeg is acceptable
	id := strings.TrimSuffix(fn, ".jpeg")
	if len(id) == len(fn) {
		return common.NewError(common.ERR_INVALID_VAL, "Expecting .jpeg filename, but received "+fn)
	}

	parts := strings.Split(id, "_")
	if len(parts) < 3 || len(parts) > 4 {
		return common.NewError(common.ERR_INVALID_VAL, "Unexpected file-name format "+fn+", could not parse it.")
	}

	if parts[0] != PFX_PERM && parts[0] != PFX_TEMP {
		return common.NewError(common.ERR_INVALID_VAL, "Unexpected file-name format "+fn+": Unkonwn prefix "+parts[0])
	}

	camId, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return common.NewError(common.ERR_INVALID_VAL, "Unexpected file-name format "+fn+": Wrong camId="+parts[1])
	}

	frmId, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return common.NewError(common.ERR_INVALID_VAL, "Unexpected file-name format "+fn+": Wrong frameId="+parts[2])
	}

	if len(parts) == 4 {
		rParts := strings.Split(parts[3], "-")
		if len(rParts) != 4 {
			return common.NewError(common.ERR_INVALID_VAL, "Unexpected file-name format "+fn+": Wrong rectangle encoding="+parts[3])
		}

		x0, err := strconv.Atoi(rParts[0])
		if err != nil {
			return common.NewError(common.ERR_INVALID_VAL, "Unexpected file-name format "+fn+": Wrong rectangle encoding x0="+rParts[0])
		}
		y0, err := strconv.Atoi(rParts[1])
		if err != nil {
			return common.NewError(common.ERR_INVALID_VAL, "Unexpected file-name format "+fn+": Wrong rectangle encoding y0="+rParts[1])
		}
		x1, err := strconv.Atoi(rParts[2])
		if err != nil {
			return common.NewError(common.ERR_INVALID_VAL, "Unexpected file-name format "+fn+": Wrong rectangle encoding x1="+rParts[2])
		}
		y1, err := strconv.Atoi(rParts[3])
		if err != nil {
			return common.NewError(common.ERR_INVALID_VAL, "Unexpected file-name format "+fn+": Wrong rectangle encoding y1="+rParts[3])
		}
		rect := image.Rect(x0, y0, x1, y1)
		imd.Rect = &rect
	}
	imd.Prefix = parts[0]
	imd.CamId = camId
	imd.FrameId = frmId
	imd.Size = 0
	imd.Format = IMG_FRMT_JPEG
	return nil
}

func (imd *ImgDesc) check() {
	if imd.Prefix != PFX_PERM && imd.Prefix != PFX_TEMP {
		panic("Unknown prefix " + imd.Prefix)
	}

	if imd.Format != IMG_FRMT_JPEG && imd.Format != IMG_FRMT_PNG {
		panic("Unsupported format " + string(imd.Format))
	}

	if _, ok := sizeCodesMap[imd.Size]; !ok {
		panic("Unknown size " + string(imd.Size))
	}
}

// Formats file name based on the descriptor settings
func (imd *ImgDesc) getFileName() string {
	ext := ".jpeg"
	if imd.Format == IMG_FRMT_PNG {
		ext = ".png"
	}
	if imd.Rect != nil {
		return fmt.Sprint(imd.Prefix, "_", imd.CamId, "_", imd.FrameId, "_",
			imd.Rect.Min.X, "-", imd.Rect.Min.Y, "-", imd.Rect.Max.X, "-", imd.Rect.Max.Y, ext)
	}
	return fmt.Sprint(imd.Prefix, "_", imd.CamId, "_", imd.FrameId, ext)
}

func (imd *ImgDesc) String() string {
	return fmt.Sprint("ImageDesc:{Prefix=", imd.Prefix, ", CamId=", imd.CamId, ", FrameId=", imd.FrameId, ", Rect=", imd.Rect, ", Size=", imd.Size, ", Format=", imd.Format, "}")
}

func (imd *ImgDesc) getPossibleIDs() []string {
	res := make([]string, len(sizeCodes))
	for i, sc := range sizeCodes {
		res[i] = imd.getStoreIdForSize(sc)
	}
	return res
}

func (imd *ImgDesc) getStoreId() string {
	imd.check()
	return imd.getStoreIdForSize(imd.Size)
}

func (imd *ImgDesc) getStoreIdForSize(size byte) string {
	if imd.Rect != nil {
		return fmt.Sprintf("%s_%x_%x_%d-%d-%d-%d_%c%c%x", imd.Prefix, imd.CamId, imd.FrameId,
			imd.Rect.Min.X, imd.Rect.Min.Y, imd.Rect.Max.X, imd.Rect.Max.Y, imd.Format, size, imd.FrameId&255)
	}
	return fmt.Sprintf("%s_%x_%x_%c%c%x", imd.Prefix, imd.CamId, imd.FrameId, imd.Format, size, imd.FrameId&255)
}
