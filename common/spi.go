package common

import (
	"fmt"
	"io"
	"math"
	"os"
	"time"

	"github.com/satori/go.uuid"
)

type (

	// Various Identificators
	Id string

	// Timestamp is a time in milliseconds
	Timestamp int64

	// A 128 dimensional vector of the face
	V128D []float32

	ISO8601Time time.Time

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

	Error struct {
		code  int
		param interface{}
	}
)

const (
	ID_NULL                 = ""
	TIMESTAMP_NA  Timestamp = 0
	ERR_NOT_FOUND           = 1
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
	tm := time.Time(t)
	stamp := fmt.Sprintf("\"%s\"", tm.Format("2006-01-02T15:04:05-0700"))
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

// The call is optimized for the 128 dimensional vectors with lenght 1
func MatchAdvancedV128D(v1, v2 V128D, d float64) bool {
	var sum float64 = 0.0
	dd := d * d
	for i := 0; i < 128; i++ {
		v := float64(v1[i]) - float64(v2[i])
		sum += v * v
		if sum > dd {
			return false
		}
	}
	return math.Sqrt(sum) < d
}

func MatchAdvanced2V128D(v1, v2 V128D, dd float64) bool {
	var sum float64 = 0.0
	for i := 0; i < 128; i++ {
		v := float64(v1[i]) - float64(v2[i])
		sum += v * v
		if sum > dd {
			return false
		}
	}
	return sum < dd
}

func MatchV128D(v1, v2 V128D, d float64) bool {
	var sum float64 = 0.0
	for i := 0; i < 128; i++ {
		v := float64(v1[i]) - float64(v2[i])
		sum += v * v
	}
	return math.Sqrt(sum) < d

}
