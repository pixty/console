package common

import "fmt"

// A person state on a scene. This states are used in Person log message
// to identify status of the the record
const (
	// This is when the person was captured first time on the scene
	PS_CAPTURE = iota + 1
	// This is an intermediate snapshot of the person on the scene
	PS_SCENE
	// This state indicates a moment when the person was gone from the scene
	PS_GONE
)

// Represents storage types
type Storage int

const (
	// Storage types. Used to receive CrudExecutor instance for the sotrage
	STGE_ORGANIZATION = iota + 1
	STGE_CAMERA
	STGE_PERSON
	STGE_PERSON_ORG
	STGE_PERSON_LOG
)

type (
	IO struct {
		Id Id `bson:"_id" json:"id"`
	}

	Organization struct {
		IO
		Name     string   `bson:"name"`
		Metadata []string `bson:"metadata"`
	}

	Camera struct {
		IO
		OrgId Id     `bson:"orgId" json:"orgId"`
		Name  string `bson:"name" json:"name"`
	}

	Person struct {
		IO
	}

	PersonOrgInfo struct {
		PersonId Id                `bson:"pId"`
		OrgId    Id                `bson:"orgId"`
		Metadata map[string]string `bson:"meta"`
	}

	Point struct {
		X int `bson:"x"`
		Y int `bson:"y"`
	}

	SnapshotImage struct {
		ImageId  string `bson:"imgId"`
		Position Point  `bson:"pos"`
	}

	// The PersonLog is a structure that describes a person, who is on the scene
	// The Structure is constructed by FrameProcessor and supposed to be persisted
	// by CameraService.
	PersonLog struct {

		// PersonId contains information about the recognized person. The field
		// is always nil immediately after creation and it is not filled by FrameProcessor
		// The FrameProcessor assigns DetectionId instead as a first step of
		// process recognition person. The PersonId will be found and assigned
		// later by special recurring process which tries to recognize persons
		PersonId Id `json:"pId" bson:"pId"`

		// DetectionId contains information about detected person. The value is
		// filled by FrameProcessor and can be used to distinguish same unrecognized
		// persons yet. This value indicates an initial attempt to identify
		// a person. FrameProcessor will try to keep same value for the same
		// person on a scene, but it is not always true. So same persons can have
		// different values for the DetectionId
		DetectionId Id            `bson:"dId"`
		CamId       Id            `bson:"camId"`
		OrgId       Id            `bson:"orgId"`
		Snapshot    SnapshotImage `bson:"snapshot"`

		// The SnapshotTs is a timestamp when the snapshot has been made
		SnapshotTs Timestamp `bson:"snTs"`

		// The SceneTs is a timestamp when the scene has been changed. The field
		// is filled by FrameProcessor and can be used to distinguish scene states
		// Scene state is changed every time when FrameProcessor concludes that
		// one of the following happens: (1) a new person appears on the scene
		// (2) a person is disappeared from the scene
		SceneTs Timestamp `bson:"sceneTs"`
		State   int       `bson:"state"`
	}
)

type CrudExecutor interface {
	NewId() Id
	Create(o interface{}) error
	CreateMany(objs ...interface{}) error
	Read(id Id) interface{}
	Update(o interface{}) error
	Delete(id Id) error
}

type Persister interface {
	NewTxPersister() TxPersister
}

type TxPersister interface {
	GetCrudExecutor(storage Storage) CrudExecutor
	GetLatestScene(camId Id) ([]*PersonLog, error)
	Close()
}

func (pl *PersonLog) String() string {
	return fmt.Sprintf("%+v", *pl)
}
