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
	ckFileName = "fn"
	ckCamId    = "cid"
	ckWidth    = "w"
	ckHeight   = "h"
)

func NewDefaultImageService() *DefaultImageService {
	return &DefaultImageService{logger: log4g.GetLogger("pixty.service.images")}
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

func (imgS *DefaultImageService) DeleteAllWithPrefix(prefix common.Id) int {
	return imgS.BlobStorage.DeleteAllWithPrefix(prefix)
}

func (imgS *DefaultImageService) Delete(imgId common.Id) error {
	return imgS.BlobStorage.Delete(imgId)
}

func toBlobMeta(id *common.ImageDescriptor) *common.BlobMeta {
	bMeta := common.NewBlobMeta()
	bMeta.Id = id.Id
	bMeta.Timestamp = id.Timestamp
	bMeta.KVPairs[ckFileName] = id.FileName
	bMeta.KVPairs[ckCamId] = string(id.CamId)
	bMeta.KVPairs[ckWidth] = strconv.FormatInt(int64(id.Width), 10)
	bMeta.KVPairs[ckHeight] = strconv.FormatInt(int64(id.Height), 10)
	return bMeta
}

func toImageDesc(b *common.BlobMeta) *common.ImageDescriptor {
	id := new(common.ImageDescriptor)
	id.FileName = b.KVPairs[ckFileName].(string)
	id.CamId = common.Id(b.KVPairs[ckCamId].(string))
	id.Id = b.Id
	id.Timestamp = b.Timestamp

	w, _ := strconv.ParseInt(b.KVPairs[ckWidth].(string), 10, 64)
	h, _ := strconv.ParseInt(b.KVPairs[ckHeight].(string), 10, 64)

	id.Width = int(w)
	id.Height = int(h)
	return id
}
