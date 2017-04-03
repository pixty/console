package service

import (
	"io"
	"strconv"

	"github.com/jrivets/log4g"
	"github.com/pixty/console/common"
)

type DefaultImageService struct {
	logger      log4g.Logger
	BlobStorage common.BlobStorage `inject:"blobStorage"`
}

const (
	ckFileName  = "fn"
	ckCamId     = "cid"
	ckTimestamp = "ts"
	ckWidth     = "w"
	ckHeight    = "h"
)

func NewDefaultImageService() *DefaultImageService {
	return &DefaultImageService{logger: log4g.GetLogger("console.service.imageService")}
}

func (imgS *DefaultImageService) New(id *common.ImageDescriptor) (common.Id, error) {
	imgS.logger.Debug("New image: ", id)
	bMeta := toBlobMeta(id)

	imgId, err := imgS.BlobStorage.Add(id.Reader, bMeta)
	return imgId, err
}

func (imgS *DefaultImageService) Read(imgId common.Id, noData bool) *common.ImageDescriptor {
	var r io.ReadCloser
	var b *common.BlobMeta
	if noData {
		b = imgS.BlobStorage.ReadMeta(imgId)
	} else {
		r, b = imgS.BlobStorage.Read(imgId)
	}

	if b == nil {
		return nil
	}

	res := toImageDesc(b)
	res.Id = imgId
	res.Reader = r
	return res
}

func toBlobMeta(id *common.ImageDescriptor) *common.BlobMeta {
	bMeta := common.NewBlobMeta()
	bMeta.KVPairs[ckFileName] = id.FileName
	bMeta.KVPairs[ckCamId] = id.CamId
	bMeta.KVPairs[ckTimestamp] = strconv.FormatInt(int64(id.Timestamp), 10)
	bMeta.KVPairs[ckWidth] = strconv.FormatInt(int64(id.Width), 10)
	bMeta.KVPairs[ckHeight] = strconv.FormatInt(int64(id.Height), 10)
	return bMeta
}

func toImageDesc(b *common.BlobMeta) *common.ImageDescriptor {
	id := new(common.ImageDescriptor)
	id.FileName = b.KVPairs[ckFileName].(string)
	id.CamId = common.Id(b.KVPairs[ckCamId].(string))

	ts, _ := strconv.ParseInt(b.KVPairs[ckTimestamp].(string), 10, 64)
	w, _ := strconv.ParseInt(b.KVPairs[ckWidth].(string), 10, 64)
	h, _ := strconv.ParseInt(b.KVPairs[ckWidth].(string), 10, 64)

	id.Timestamp = common.Timestamp(ts)
	id.Width = int(w)
	id.Height = int(h)
	return id
}
