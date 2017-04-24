package common

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/pixty/fpcp"
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
		Reader    io.Reader
		FileName  string
		CamId     Id
		Width     int
		Height    int
		Timestamp Timestamp
	}

	ImageService interface {
		// Store the image. id can be provided. If not, the new one will be generated
		New(id *ImageDescriptor) (Id, error)

		// Returns image descriptor by an image Id. If noData == true, then
		// only image metadata is filled, but no data reader is returned.
		// Returns nil if the image is not found
		Read(imgId Id, noData bool) *ImageDescriptor
	}

	BlobMeta struct {
		Id        Id
		KVPairs   map[string]interface{}
		Timestamp Timestamp
		Size      int64
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

	SceneService interface {
		// Returns Http Scene Processor implementation to wire up with gin
		GetHttpSceneProcessor() *fpcp.HttpSceneProcessor
		GetScene(ctx CtxHolder, camId Id) (*Scene, error)
	}

	Error struct {
		code  int
		param interface{}
	}
)

const (
	ID_NULL       = ""
	ERR_NOT_FOUND = 1
)

func CheckError(e error, code int) bool {
	if e == nil {
		return false
	}
	err, ok := e.(*Error)
	if !ok {
		return false
	}
	return err.code == code
}

func NewError(code int, param interface{}) *Error {
	return &Error{code, param}
}

func (e *Error) Error() string {
	switch e.code {
	case ERR_NOT_FOUND:
		return fmt.Sprint(e.param, " not found.")
	}
	return ""
}

func (t ISO8601Time) MarshalJSON() ([]byte, error) {
	stamp := fmt.Sprintf("\"%s\"", time.Time(t).Format("2006-01-02T15:04:05-0700"))
	return []byte(stamp), nil
}

func NewBlobMeta() *BlobMeta {
	return &BlobMeta{ID_NULL, make(map[string]interface{}), CurrentTimestamp(), 0}
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
