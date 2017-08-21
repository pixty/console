package model

import (
	"fmt"

	"github.com/pixty/console/common"
)

type (
	// Organization DO.
	Organization struct {
		Id   int64
		Name string
	}

	// FieldInfo - describes a field of profile metadata
	FieldInfo struct {
		OrgId       int64
		FieldId     int64
		FieldType   string
		DisplayName string
	}

	// Type of the field please see FLD_TYPE_ constants below
	FieldType string

	// User DO
	User struct {
		Id    int64
		Login string
	}

	// Camera DO
	Camera struct {
		Id        string // friendly camera name
		OrgId     int64
		SecretKey string
	}

	// A person DO
	Person struct {
		// Person id is generated by Frame Processor
		Id         string
		CamId      string
		LastSeenAt int64
		ProfileId  int64
		PictureId  string // avatar
		MatchGroup string
	}

	// Point on a 2D plane
	Point struct {
		X int `json:"x"`
		Y int `json:"y"`
	}

	// A rectangle on a 2D plane
	Rectangle struct {
		LeftTop     Point `json:"leftTop"`
		RightBottom Point `json:"rightBottom"`
	}

	// A size on 2D plane
	Size struct {
		Width  int `json:"Width" bson:"width"`
		Height int `json:"Height" bson:"height"`
	}

	// A face description DO
	Face struct {
		Id          int64
		SceneId     string
		PersonId    string
		CapturedAt  int64
		ImageId     string
		Rect        Rectangle //composite, dao has transformations
		FaceImageId int64
		V128D       common.V128D //composite, dao has transformations
	}

	// An organization profile's information
	Profile struct {
		ProfileId int64
		OrgId     int64
		PictureId string // avatar
	}

	ProfileMeta struct {
		ProfileId int64
		FieldId   int64
		Value     string
	}

	// Persister is an interface which provides an access to persistent layer
	Persister interface {
		GetMainPersister() MainPersister
		GetPartPersister(partId common.Id) PartPersister
	}

	// An transactional persister (has context dependant time)
	MainPersister interface {
		FindCameraById(camId string) (*Camera, error)
	}

	// Partitioned persister
	PartPersister interface {
		// ==== Faces ====
		// returns Face by its Id, or error
		GetFaceById(pId int64) (*Face, error)
		// insert new face, returns the new record id, or error, if it happens
		InsertFace(face *Face) (int64, error)

		// ==== Persons ====
		// returns Person by its Id, or error
		GetPersonById(pId string) (*Person, error)
		// insert new person, returns the new record id, or error, if it happens
		InsertPerson(person *Person) (int64, error)
		UpdatePerson(person *Person) error
	}

	PersonsQuery struct {
		// Camera Id the request done for
		CamId string
		// The maximum allowable max time.
		MaxLastSeenAt common.Timestamp
		// Request only this persons (MaxLastSeenAt and Limit will be disregarded)
		PersonIds []string
		// How many to select
		Limit int
	}
)

// Known field types for profile fields description
const (
	FLD_TYPE_STRING FieldType = "string"
)

func (c *Camera) String() string {
	return fmt.Sprintf("{Id=%s, OrgId=%d, SecretKey=%s}", c.Id, c.OrgId, c.SecretKey)
}

func (q *PersonsQuery) String() string {
	return fmt.Sprintf("{CamId=%s, PersonsIds=%v, MaxLastSeenAt=%d, Limit=%d}", q.CamId, q.PersonIds, q.MaxLastSeenAt, q.Limit)
}
