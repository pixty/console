package common

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
	STGE_ORGANIZATION = iota + 1
	STGE_CAMERA
	STGE_PERSON
	STGE_PERSON_ORG
	STGE_PERSON_LOG
)

type (
	Organization struct {
		Id       Id       `bson:"_id"`
		Name     string   `bson:"name"`
		Metadata []string `bson:"metadata"`
	}

	Camera struct {
		Id    Id     `bson:"_id" json:"id"`
		OrgId Id     `bson:"orgId" json:"orgId"`
		Name  string `bson:"name" json:"name"`
	}

	Person struct {
		Id Id `bson:"_id"`
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

	PersonLog struct {
		PersonId  Id            `bson:"pId"`
		CamId     Id            `bson:"camId"`
		OrgId     Id            `bson:"orgId"`
		Snapshot  SnapshotImage `bson:"snapshot"`
		Timestamp Timestamp     `bson:"ts"`
		State     int           `bson:"state"`
	}
)

type CrudExecutor interface {
	NewId() Id
	Create(o interface{}) error
	Read(id Id) interface{}
	Update(o interface{}) error
	Delete(id Id) error
}

type PersonLogQuery struct {
}

type Persister interface {
	NewTxPersister() TxPersister
}

type TxPersister interface {
	GetCrudExecutor(storage Storage) CrudExecutor
	GetPersonLogs(query *PersonLogQuery) []*PersonLog
	Close()
}
