package common

import "time"
import "fmt"

type Id string
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
)

func (t ISO8601Time) MarshalJSON() ([]byte, error) {
	stamp := fmt.Sprintf("\"%s\"", time.Time(t).Format("2006-01-02T15:04:05-0700"))
	return []byte(stamp), nil
}
