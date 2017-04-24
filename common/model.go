package common

import (
	"fmt"

	"golang.org/x/net/context"
)

type (
	// Version, could be used by DB's supported CAS ops (like mongo)
	Version int64

	// Organization DO.
	Organization struct {
		Id       Id       `bson:"_id" json:"id"`
		Name     string   `bson:"name"`
		Metadata []string `bson:"metadata"`
	}

	// Camera DO
	Camera struct {
		Id    Id     `bson:"_id" json:"id"`
		OrgId Id     `bson:"orgId" json:"orgId"`
		Name  string `bson:"name" json:"name"`
	}

	// Point on a 2D plane
	Point struct {
		X int `json:"x" bson:"x"`
		Y int `json:"y" bson:"y"`
	}

	// A rectangle on a 2D plane
	Rectangle struct {
		LeftTop     Point `json:"leftTop" bson:"leftTop"`
		RightBottom Point `json:"rightBottom" bson:"rightBottom"`
	}

	// A size on 2D plane
	Size struct {
		Width  int `json:"Width" bson:"width"`
		Height int `json:"Height" bson:"height"`
	}

	// A face picture reference
	FacePic struct {
		ImageId Id        `bson:"imgId"`
		Rect    Rectangle `bson:"rect"`
	}

	Person struct {
		// Person id is generated by Frame Processor
		Id    Id `bson:"_id"`
		CamId Id `bson:"camId"`

		// Associated profile. It can be nil if the profile is not found or
		// not created yet.
		Profile *PersonMatch `bson:"profile"`

		Faces []FacePic `bson:"faces"`

		// First time the person has been seen at
		SeenAt Timestamp `bson:"seenAt"`

		// Populated by SP if it is done with the person completely.
		LostAt Timestamp `bson:"lostAt"`
	}

	PersonMatch struct {
		PersonId  Id `bson:"personId" json:"personId"`
		ProfileId Id `bson:"profileId" json:"profileId"`

		// 10000 means 100.00%, 999 means 9.99%, 5000 means 50.00% etc.
		Occuracy int `bson:"occuracy" json:"occuracy"`
	}

	Profile struct {
		Id      Id                    `bson:"_id" json:"id"`
		OrgInfo map[Id]ProfileOrgInfo `bson:"orgInfo"`
	}

	// An organization person's information
	ProfileOrgInfo struct {
		Metadata map[string]string `bson:"meta"`
	}

	// The Scene DO describes scenes reported by a Frame Processor (FP). The
	// structure contains some information received directly from FP and some
	// information, which will be populated after. Scene Processor (SP) will
	// monitor this structures and will handle them controlling the "State"
	// field.
	//
	// The typical scheme is as following: New scene record appears as soon as
	// FP reports it. It is persisted and left untouched until SP comes and checks it.
	// The SP will build associations between persons found by FP and Pixty DB,
	// it will populate PersonsIds fields and update the record Status. For
	// more details see how the SP works.
	Scene struct {
		Id Id `bson:"_id" json:"id"`

		// Scene beginning time. Populated by FP as soon as scene catched up.
		Timestamp Timestamp `bson:"timestamp"`

		// List of Persons IDs found by FP.
		PersonsIds []Id `bson:"persons"`

		CamId Id `bson:"camId"`
		OrgId Id `bson:"orgId"`

		// Image of the scene. Optional.
		ImgId Id `bson:"imgId"`
	}

	// Represents storage types. Can be mapped to DB tables, please see
	// STGE_ - constants below.
	Storage int

	// Scene state. Used by state processor to handle persons on the scene and
	// build persons association.
	SceneState int

	// Persister is an interface which provides an access to persistent layer
	Persister interface {
		NewTxPersister(ctx context.Context) TxPersister
	}

	// An transactional persister (has context dependant time)
	TxPersister interface {
		GetCrudExecutor(storage Storage) CrudExecutor
		Close()

		// Persons specific
		FindPersons(query *PersonsQuery) ([]*Person, error)

		// Scene specific
		GetScenes(q *SceneQuery) ([]*Scene, error)

		// Find all matches
		GetMatches(personId Id) ([]*PersonMatch, error)
	}

	PersonsQuery struct {
		PersonIds []Id
		ProfileId Id
		FromTime  Timestamp
		Limit     int
	}

	SceneQuery struct {
		CamId Id
		Limit int
	}

	CrudExecutor interface {
		Create(o interface{}) error
		CreateMany(objs ...interface{}) error
		Read(id Id, res interface{}) error
		Update(id Id, o interface{}) error
		Delete(id Id) error
	}
)

// Storage types. Used to receive CrudExecutor instance for the sotrage
const (
	STGE_ORGANIZATION = iota + 1
	STGE_CAMERA
	STGE_PERSON
	STGE_PERSON_MATCH
	STGE_PROFILE
	STGE_SCENE
)

func (sc *Scene) String() string {
	return fmt.Sprintf("%+v", *sc)
}

func (q *SceneQuery) String() string {
	return fmt.Sprintf("{camId=%s, limit=%d}", q.CamId, q.Limit)
}
