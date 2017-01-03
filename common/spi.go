package common

import "time"
import "fmt"
import "io"
import "github.com/satori/go.uuid"
import "os"

const (
	ID_NULL = ""
)

type Id string

// Timestamp is a time in nanoseconds...
type Timestamp int64
type ISO8601Time time.Time

type (
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
		Timestamp Timestamp
	}

	ImageService interface {
		New(id *ImageDescriptor) (Id, error)
		Read(imgId Id) *ImageDescriptor
	}

	BlobMeta struct {
		KVPairs map[string]interface{}
	}

	BlobStorage interface {
		// Saves BLOB to the store. The invoker should close the reader itself
		Add(r io.Reader, bMeta *BlobMeta) (Id, error)

		// Returns reader for the object ID. It is in voker responsibility to
		// close the reader after use.
		Read(objId Id) (io.ReadCloser, *BlobMeta)

		// Deletes an object by its id. Returns error != nil if the object is not found
		Delete(objId Id) error
	}
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

func ToTimestamp(t time.Time) Timestamp {
	return Timestamp(t.UnixNano())
}

func (t Timestamp) ToTime() time.Time {
	return time.Unix(0, int64(t))
}
