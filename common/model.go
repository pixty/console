package common

import "fmt"

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
	// Identified Object
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
		X int `json:"x" bson:"x"`
		Y int `json:"y" bson:"y"`
	}

	Rectangle struct {
		LeftTop     Point `json:"leftTop" bson:"leftTop"`
		RightBottom Point `json:"rightBottom" bson:"rightBottom"`
	}

	SnapshotImage struct {
		ImageId Id `bson:"imgId"`

		// Indicates the time when the snapshot was made
		Timestamp Timestamp `bson:"ts"`
		Rect      Rectangle `bson:"rect"`
	}

	// The PersonLogRecord is a structure which describes a person, who is on the scene
	// The Structure is constructed by FrameProcessor and supposed to be persisted
	// by CameraService.
	PersonLogRecord struct {
		IO

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
		// person. The person with same DetectionId can appear on multiple scenes,
		// but same person never has different values of the filed on same scene.
		DetectionId Id            `bson:"dId"`
		CamId       Id            `bson:"camId"`
		OrgId       Id            `bson:"orgId"`
		Snapshot    SnapshotImage `bson:"snapshot"`

		// The FirstSeenAt contains a time when the person has been captured by the
		// camera first time. This value is always same with the SceneTs when the
		// person has appeared first.
		FirstSeenAt Timestamp `bson:"firstSeenAt"`

		// The SceneTs is a timestamp when the scene has been changed. The field
		// is filled by FrameProcessor and can be used to distinguish scene states
		// Scene state is changed every time when FrameProcessor concludes that
		// one of the following happens: (1) a new person appears on the scene
		// (2) a person is disappeared from the scene.
		// Every time, when FrameProcessor forms new scene it provides the whole list
		// of persons for the new scene. So some persons can come from the previous
		// scene
		SceneTs Timestamp `bson:"sceneTs"`
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
