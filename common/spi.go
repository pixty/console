package common

import (
	crand "crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"math"
	mrand "math/rand"
	"os"
	"strconv"
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

		// Deletes an image by its id. Returns error != nil if operation is failed,
		// returns nil, if the object is not found (succeeded)
		Delete(imgId Id) error

		// Delete all pictures that starts from prefix
		DeleteAllWithPrefix(prefix Id) int
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

		// Deletes an object by its id. Returns error != nil if operation is failed
		Delete(objId Id) error

		// Deletes all ids with prefix
		DeleteAllWithPrefix(prefix Id) int
	}

	Error struct {
		code  int
		param interface{}
	}
)

const (
	ID_NULL                         = ""
	TIMESTAMP_NA          Timestamp = 0
	ERR_NOT_FOUND                   = 1
	ERR_INVALID_VAL                 = 2
	ERR_LIMIT_VIOLATION             = 3
	ERR_WRONG_CREDENTIALS           = 4
	ERR_UNAUTHORIZED                = 5

	V128D_SIZE          = 512 // 128 values by 4 bytes each
	SECRET_KEY_ALPHABET = "0123456789QWERTYUIOPASDFGHJKLZXCVBNMqwertyuiopasdfghjklzxcvbnm_^-()@#$%"
)

// ================================= Misc ====================================
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

func NewSecretKey(lng int) string {
	val := make([]byte, lng)
	Rand(val)

	ab := len(SECRET_KEY_ALPHABET)
	for i := 0; i < lng; i++ {
		idx := int(val[i]) % ab
		val[i] = SECRET_KEY_ALPHABET[idx]
	}
	return string(val)
}

func NewSession() string {
	val := make([]byte, 32)
	Rand(val)
	return HashBytes(val)
}

func Hash(password string) string {
	return HashBytes([]byte(password))
}

func HashBytes(bts []byte) string {
	h := sha256.Sum256(bts)
	return base64.StdEncoding.EncodeToString(h[:])
}

func Rand(bts []byte) {
	if _, err := crand.Read(bts); err != nil {
		panic(err)
	}
}

// ================================ Error ====================================
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
	return e.param.(string)
}

// ============================== BlobMeta ===================================
func NewBlobMeta() *BlobMeta {
	return &BlobMeta{ID_NULL, make(map[string]interface{}), CurrentTimestamp(), 0}
}

func (bm *BlobMeta) String() string {
	return fmt.Sprintf("{KVPairs: %v}", bm.KVPairs)
}

// ============================== Timestamp ==================================
func CurrentTimestamp() Timestamp {
	return ToTimestamp(time.Now())
}

func CurrentISO8601Time() ISO8601Time {
	return ISO8601Time(time.Now())
}

func (t ISO8601Time) MarshalJSON() ([]byte, error) {
	tm := time.Time(t)
	stamp := fmt.Sprintf("\"%s\"", tm.Format("2006-01-02T15:04:05-0700"))
	return []byte(stamp), nil
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

// ================================ V128D ====================================
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

func NewV128D() V128D {
	return V128D(make([]float32, 128, 128))
}

func (v V128D) ToByteSlice() []byte {
	res := make([]byte, V128D_SIZE, V128D_SIZE)
	idx := 0
	for _, val := range v {
		ui32 := math.Float32bits(val)
		res[idx] = byte(ui32)
		res[idx+1] = byte(ui32 >> 8)
		res[idx+2] = byte(ui32 >> 16)
		res[idx+3] = byte(ui32 >> 24)
		idx += 4
	}
	return res
}

func (v V128D) Assign(b []byte) error {
	if b == nil {
		return errors.New("Array is nil. Cannot assign it to V123D")
	}

	if len(b) != V128D_SIZE {
		return errors.New("Size of bytes must be " + strconv.Itoa(V128D_SIZE) + " even, but it is " + strconv.Itoa(len(b)))
	}

	i := 0
	for idx := 0; idx < 128; idx++ {
		var ui32 uint32
		ui32 = uint32(b[i]) | uint32(b[i+1])<<8 | uint32(b[i+2])<<16 | uint32(b[i+3])<<24
		v[idx] = math.Float32frombits(ui32)
		i += 4
	}

	return nil
}

func (v V128D) Equals(v2 V128D) bool {
	for i, vv := range v {
		if v2[i] != vv {
			return false
		}
	}
	return true
}

// For testing...
func (v V128D) FillRandom() V128D {
	s := mrand.NewSource(time.Now().UnixNano())
	r := mrand.New(s)
	for i := 0; i < 128; i++ {
		v[i] = r.Float32()
	}
	return v
}
