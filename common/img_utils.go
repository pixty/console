package common

import (
	"fmt"
	"image"
	"strconv"
	"strings"
)

const (
	IMG_CAM_PREFIX     = "cm-"
	IMG_TMP_CAM_PREFIX = "tcm-"
)

func ImgMakeFileName(imgId string, rect *image.Rectangle) string {
	return ImgMakeId(imgId, rect) + ".png"
}

func ImgMakeCamId(camId int64, ts Timestamp) string {
	return IMG_CAM_PREFIX + strconv.FormatInt(camId, 10) + "-" + strconv.FormatUint(uint64(ts), 10)
}

func ImgMakeTmpCamId(camId int64, ts Timestamp) string {
	return IMG_TMP_CAM_PREFIX + strconv.FormatInt(camId, 10) + "-" + strconv.FormatUint(uint64(ts), 10)
}

func ImgIsTmpCamId(imgId string) bool {
	return strings.HasPrefix(imgId, IMG_TMP_CAM_PREFIX)
}

func ImgMakeId(imgId string, rect *image.Rectangle) string {
	if rect == nil {
		return string(imgId)
	}
	return fmt.Sprintf("%s_%d_%d_%d_%d", imgId, rect.Min.X, rect.Min.Y, rect.Max.X, rect.Max.Y)
}

// Parses imgFileName and returns parsed Id and rectangle, if present:
// In: abc.png Out: abc, nil, nil
// In: abc_1_2_3_4.png Out: abc, {1,2,3,4}, nil
// In: http://localhost:8080/images/abc.png Out: abc, nil, nil
func ImgParseFileName(imgFileName string) (string, *image.Rectangle, error) {
	var nilId string
	if !strings.HasSuffix(imgFileName, ".png") {
		return nilId, nil, NewError(ERR_INVALID_VAL, "Expecting .png filename, but received "+imgFileName)
	}

	idx := strings.LastIndex(imgFileName, "/")
	if idx > -1 {
		imgFileName = imgFileName[idx+1 : len(imgFileName)]
	}
	id := strings.TrimSuffix(imgFileName, ".png")
	return ImgParseId(id)
}

// Parses imgFileName and returns parsed Id:
// In: abc.png Out: abc, nil
// In: abc_1_2_3_4.png Out: abc_1_2_3_4, nil
// In: http://localhost:8080/images/abc.png Out: abc, nil
// In: http://localhost:8080/images/abc.jpeg Out: nil, <.png is expected>
func ImgParseFileNameNotDeep(imgFileName string) (string, error) {
	if !strings.HasSuffix(imgFileName, ".png") {
		return "", NewError(ERR_INVALID_VAL, "Expecting .png filename, but received "+imgFileName)
	}
	idx := strings.LastIndex(imgFileName, "/")
	if idx > -1 {
		imgFileName = imgFileName[idx+1 : len(imgFileName)]
	}
	return strings.TrimSuffix(imgFileName, ".png"), nil
}

func ImgParseId(id string) (string, *image.Rectangle, error) {
	var nilId string
	parts := strings.Split(id, "_")
	if len(parts) == 1 {
		// ok, no rectangle encoded
		return id, nil, nil
	}
	if len(parts) != 5 {
		return nilId, nil, NewError(ERR_INVALID_VAL, "Expecting image in <id>_<x0>_<y0>_<x1>_<y1>.png format")
	}

	x0, err := strconv.Atoi(parts[1])
	if err != nil {
		return nilId, nil, err
	}
	y0, err := strconv.Atoi(parts[2])
	if err != nil {
		return nilId, nil, err
	}
	x1, err := strconv.Atoi(parts[3])
	if err != nil {
		return nilId, nil, err
	}
	y1, err := strconv.Atoi(parts[4])
	if err != nil {
		return nilId, nil, err
	}
	rect := image.Rect(x0, y0, x1, y1)
	return id, &rect, nil
}
