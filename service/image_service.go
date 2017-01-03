package service

import "github.com/pixty/console/common"
import "github.com/jrivets/log4g"

type DefaultImageService struct {
	logger      log4g.Logger
	BlobStorage common.BlobStorage `inject:"blobStorage"`
}

const (
	ckFileName  = "fn"
	ckCamId     = "cid"
	ckTimestamp = "ts"
)

func NewDefaultImageService() *DefaultImageService {
	return &DefaultImageService{logger: log4g.GetLogger("console.service.imageService")}
}

func (imgS *DefaultImageService) New(id *common.ImageDescriptor) (common.Id, error) {
	imgS.logger.Debug("New image: ", id)
	bMeta := common.NewBlobMeta()
	bMeta.KVPairs[ckFileName] = id.FileName
	bMeta.KVPairs[ckCamId] = id.CamId
	bMeta.KVPairs[ckTimestamp] = id.Timestamp

	imgId, err := imgS.BlobStorage.Add(id.Reader, bMeta)
	return imgId, err
}

func (imgS *DefaultImageService) Read(imgId common.Id) *common.ImageDescriptor {
	r, b := imgS.BlobStorage.Read(imgId)
	if r == nil {
		return nil
	}
	fn := b.KVPairs[ckFileName].(string)
	camId := b.KVPairs[ckCamId].(common.Id)
	ts := b.KVPairs[ckTimestamp].(common.Timestamp)

	return &common.ImageDescriptor{Id: imgId, Reader: r, FileName: fn, CamId: camId, Timestamp: ts}
}