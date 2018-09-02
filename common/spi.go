package common

import (
	crand "crypto/rand"
	"crypto/sha256"
	//"encoding/base32"
	"encoding/base64"
	"fmt"
	"math"
	mrand "math/rand"
	"os"
	"strconv"
	"time"

	"github.com/satori/go.uuid"
)

type (

	// Various Identificators
	//Id string

	// Timestamp is a time in milliseconds
	Timestamp int64

	// A 128 dimensional vector of the face
	V128D []float32

	ISO8601Time time.Time

	// returns orgId by camId
	CamId2OrgIdCache interface {
		GetOrgId(camId int64) int64
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
	ERR_AUTH_REQUIRED               = 5
	ERR_UNAUTHORIZED                = 6

	MAX_FACES_PER_PERSON = 10

	V128D_SIZE          = 512 // 128 values by 4 bytes each
	SECRET_KEY_ALPHABET = "0123456789QWERTYUIOPASDFGHJKLZXCVBNMqwertyuiopasdfghjklzxcvbnazx_^-()@#$%"
	SESSION_ALPHABET    = "0123456789QWERTYUIOPASDFGHJKLZXCVBNMqwertyuiopasdfghjklzxcvbnazx"
)

// ================================= Misc ====================================
func NewUUID() string {
	u, _ := uuid.NewV4()
	return u.String()
}

func DoesFileExist(fileName string) bool {
	_, err := os.Stat(fileName)
	return !os.IsNotExist(err)
}

func NewSecretKey(lng int) string {
	val := make([]byte, lng)
	Rand(val)

	return bytes2String(val, SECRET_KEY_ALPHABET, 7)
}

// turn bytes to string. val will be affected
func bytes2String(val []byte, abet string, bits int) string {
	cap := len(val) * 8 / bits
	abl := len(abet)
	res := make([]byte, 0, cap)
	mask := (1 << uint(bits)) - 1
	i := 0
	shft := 0
	for i < len(val) {
		b := int(val[i]) >> uint(shft)
		bSize := 8 - shft
		if bSize <= bits {
			i++
			if i < (len(val)) {
				shft = bits - bSize
				b |= int(val[i]) << uint(bSize)
			}
		} else {
			shft += bits
		}
		res = append(res, abet[(b&mask)%abl])
	}
	return string(res)
}

func NewSession() string {
	val := make([]byte, 32)
	Rand(val)
	h := sha256.Sum256(val)
	return bytes2String(h[:], SESSION_ALPHABET, 6)
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
		return NewError(ERR_INVALID_VAL, "Array is nil. Cannot assign it to V123D")
	}

	if len(b) != V128D_SIZE {
		return NewError(ERR_INVALID_VAL, "Size of bytes must be "+strconv.Itoa(V128D_SIZE)+" even, but it is "+strconv.Itoa(len(b)))
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
