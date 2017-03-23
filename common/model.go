package common

import (
	"fmt"

	"github.com/pixty/fpcp"
	"golang.org/x/net/context"
)

type (
	// Version, could be used by DB's supported CAS ops (like mongo)
	Version int64

	// Identified Object. Most DO inherits it
	IO struct {
		Id Id `bson:"_id" json:"id"`
	}

	// Organization DO.
	Organization struct {
		IO
		Name     string   `bson:"name"`
		Metadata []string `bson:"metadata"`
	}

	// Camera DO
	Camera struct {
		IO
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

	// A face picture reference
	FacePic struct {
		ImageId Id        `bson:"imgId"`
		Rect    Rectangle `bson:"rect"`
	}

	// Person DO. The Person describes an known person persisted in the Pixty
	// DB. This object is not one, which we receives from Frame Processor (FP),
	// but what Pixty knows and will recognize among faces received from FP
	Person struct {
		IO
	}

	// An organization person's information
	PersonOrgInfo struct {
		PersonId Id                `bson:"pId"`
		OrgId    Id                `bson:"orgId"`
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
		IO

		// Scene beginning time. Populated by FP as soon as scene catched up.
		Timestamp Timestamp `bson:"timestamp"`

		// List of Persons found by FP. The field can be updated after the scene
		// is created. FP can report new images for found persons. All information
		// in the field relates to FP DB. For instance, person Ids are not Pixty's
		// Person's Ids, but FP's ones. SP will recognize this persons
		// and build associations between the FP persons and Pixty's DB.
		FpPersons []fpcp.Person `bson:"fpPersons"`

		// Pixty persons associated with the scene
		PersonsIds []Id       `bson:"personsIds"`
		CamId      Id         `bson:"camId"`
		OrgId      Id         `bson:"orgId"`
		State      SceneState `bson:"state"`
		Version    Version    `bson:"version"`
	}

	// The Fp2Sp DO keeps association between FP to SP data. This is a temporary
	// information which is used by SP for building association between data received
	// froom FP and Pixty data.
	Fp2Sp struct {
		CamId      Id
		FpPersonId Id
		PersonId   Id
		ImagesMap  map[Id]Id
		// Populated by SP if the person is not found in last reported scene
		FpClosed bool
		// Populated by SP if it is done with the person completely.
		SpClosedAt Timestamp
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
	}

	CrudExecutor interface {
		NewId() Id
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
	STGE_PERSON_ORG
	STGE_SCENE
	STGE_FP2SP
)

// Scene states. Used by State Processor to check and process scenes
const (
	// New scene, some actions are required
	SS_NEW = iota + 1
	// All people associations are found, but scene could be updated
	SS_IDENTIFIED
	// The Scene Processor closed the scene and no updates are expected
	SS_COMPLETED
)

func (pl *PersonLog) String() string {
	return fmt.Sprintf("%+v", *pl)
}
