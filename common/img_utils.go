package common

import (
	"errors"
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

func ImgMakeCamId(camId string, ts Timestamp) string {
	return IMG_CAM_PREFIX + camId + strconv.FormatUint(uint64(ts), 10)
}

func ImgMakeTmpCamId(camId string, ts Timestamp) string {
	return IMG_TMP_CAM_PREFIX + string(camId) + strconv.FormatUint(uint64(ts), 10)
}

func ImgIsTmpCamId(camId string) bool {
	return strings.HasPrefix(camId, IMG_TMP_CAM_PREFIX)
}

func ImgMakeId(imgId string, rect *image.Rectangle) string {
	if rect == nil {
		return string(imgId)
	}
	return fmt.Sprintf("%s_%d_%d_%d_%d", imgId, rect.Min.X, rect.Min.Y, rect.Max.X, rect.Max.Y)
}

func ImgParseFileName(imgFileName string) (string, *image.Rectangle, error) {
	var nilId string
	if !strings.HasSuffix(imgFileName, ".png") {
		return nilId, nil, errors.New("Expecting .png filename, but received " + imgFileName)
	}

	id := strings.TrimSuffix(imgFileName, ".png")
	return ImgParseId(id)
}

func ImgParseId(id string) (string, *image.Rectangle, error) {
	var nilId string
	parts := strings.Split(id, "_")
	if len(parts) == 1 {
		// ok, no rectangle encoded
		return id, nil, nil
	}
	if len(parts) != 5 {
		return nilId, nil, errors.New("Expecting image in <id>_<x0>_<y0>_<x1>_<y1>.png format")
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
	return parts[0], &rect, nil
}
