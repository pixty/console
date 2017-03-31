package common

import (
	"fmt"

	"golang.org/x/net/context"
	"gopkg.in/mgo.v2/bson"
)

type (
	// Version, could be used by DB's supported CAS ops (like mongo)
	Version int64

	// Organization DO.
	Organization struct {
		Id       bson.ObjectId `bson:"_id" json:"id"`
		Name     string        `bson:"name"`
		Metadata []string      `bson:"metadata"`
	}

	// Camera DO
	Camera struct {
		Id    bson.ObjectId `bson:"_id" json:"id"`
		OrgId Id            `bson:"orgId" json:"orgId"`
		Name  string        `bson:"name" json:"name"`
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

	// A face picture reference
	FacePic struct {
		ImageId Id        `bson:"imgId"`
		Rect    Rectangle `bson:"rect"`
	}

	Person struct {
		Id string `bson:"_id" json:"id"`
		//Id    bson.ObjectId `bson:"_id" json:"id"`
		CamId Id `bson:"camId"`

		// Associated profile. It can be nil if the profile is not found or
		// not created yet.
		ProfileId Id        `bson:"profileId"`
		Faces     []FacePic `bson:"faces"`

		// First time the person has been seen at
		SeenAt Timestamp `bson:"seenAt"`

		// Populated by SP if it is done with the person completely.
		LostAt Timestamp `bson:"lostAt"`
	}

	Profile struct {
		Id      bson.ObjectId         `bson:"_id" json:"id"`
		OrgInfo map[Id]ProfileOrgInfo `bson:"orgInfo"`
	}

	// An organization person's information
	ProfileOrgInfo struct {
		ProfileId Id                `bson:"profileId"`
		OrgId     Id                `bson:"orgId"`
		Metadata  map[string]string `bson:"meta"`
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
		Id bson.ObjectId `bson:"_id" json:"id"`

		// Scene beginning time. Populated by FP as soon as scene catched up.
		Timestamp Timestamp `bson:"timestamp"`

		// List of Persons IDs found by FP.
		PersonsIds []Id `bson:"persons"`

		CamId Id `bson:"camId"`
		OrgId Id `bson:"orgId"`
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
		FindPersonsByIds(ids ...string) ([]*Person, error)

		// Scene specific
		GetLatestScene(camId Id) (*Scene, error)
	}

	CrudExecutor interface {
		Create(o interface{}) error
		CreateMany(objs ...interface{}) error
		Read(id Id) interface{}
		Update(o interface{}) error
		Delete(id Id) error
	}
)

// Storage types. Used to receive CrudExecutor instance for the sotrage
const (
	STGE_ORGANIZATION = iota + 1
	STGE_CAMERA
	STGE_PERSON
	STGE_PROFILE
	STGE_SCENE
)

func (sc *Scene) String() string {
	return fmt.Sprintf("%+v", *sc)
}
