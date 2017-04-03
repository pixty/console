package common

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/satori/go.uuid"
)

type (

	// Various Identificators
	Id string

	// Timestamp is a time in milliseconds
	Timestamp int64

	ISO8601Time time.Time

	OrgService interface {
		GetById(OrgId Id) *Organization
	}

	CameraService interface {
		GetById(CamId Id) *Camera
		GetByOrgId(OrgId Id) []*Camera
	}

	ImageDescriptor struct {
		Id        Id
		Reader    io.ReadCloser
		FileName  string
		CamId     Id
		Width     int
		Height    int
		Timestamp Timestamp
	}

	ImageService interface {
		New(id *ImageDescriptor) (Id, error)
		// Returns image descriptor by an image Id. If noData == true, then
		// only image metadata is filled, but no data reader is returned.
		// Returns nil if the image is not found
		Read(imgId Id, noData bool) *ImageDescriptor
	}

	BlobMeta struct {
		KVPairs map[string]interface{}
	}

	BlobStorage interface {
		// Saves BLOB to the store. The invoker should close the reader itself
		Add(r io.Reader, bMeta *BlobMeta) (Id, error)

		// Returns reader for the object ID. It is invoker responsibility to
		// close the reader after use.
		Read(objId Id) (io.ReadCloser, *BlobMeta)

		// Reads meta data for the object. Returns nil if not found.
		ReadMeta(objId Id) *BlobMeta

		// Deletes an object by its id. Returns error != nil if the object is not found
		Delete(objId Id) error
	}
)

const (
	ID_NULL = ""
)

func (t ISO8601Time) MarshalJSON() ([]byte, error) {
	stamp := fmt.Sprintf("\"%s\"", time.Time(t).Format("2006-01-02T15:04:05-0700"))
	return []byte(stamp), nil
}

func NewBlobMeta() *BlobMeta {
	return &BlobMeta{make(map[string]interface{})}
}

func (bm *BlobMeta) String() string {
	return fmt.Sprintf("{KVPairs: %v}", bm.KVPairs)
}

func NewUUID() string {
	return uuid.NewV4().String()
}

func NewId() Id {
	return Id(uuid.NewV4().String())
}

func DoesFileExist(fileName string) bool {
	_, err := os.Stat(fileName)
	return !os.IsNotExist(err)
}

func CurrentTimestamp() Timestamp {
	return ToTimestamp(time.Now())
}

func CurrentISO8601Time() ISO8601Time {
	return ISO8601Time(time.Now())
}

func ToTimestamp(t time.Time) Timestamp {
	return Timestamp(t.UnixNano() / int64(time.Millisecond))
}

func (t Timestamp) ToTime() time.Time {
	return time.Unix(0, int64(t*Timestamp(time.Millisecond)))
}

func (t Timestamp) ToISO8601Time() ISO8601Time {
	return ISO8601Time(t.ToTime())
}
