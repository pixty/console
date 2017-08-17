package common

type (
	// Version, could be used by DB's supported CAS ops (like mongo)
	Version int64

	// Organization DO.
	Organization struct {
		Id   Id          `bson:"_id" json:"id"`
		Name string      `bson:"name" json:"name"`
		Meta OrgMetadata `bson:"meta" json:"meta"`
	}

	// User DO
	User struct {
		Id    Id     `bson:"_id" json:"id"`
		Login string `bson:"login" json:"login"`
	}

	// OrgMetadata contains profile data mapping information
	OrgMetadata struct {
		FieldsMappings []FieldInfo `bson:"fieldsMapping" json:"fieldsMapping"`
	}

	// Type of the field
	FieldType string

	// FieldInfo - describes a field of profile metadata
	FieldInfo struct {
		FieldId     int       `bson:"fieldId" json:"fieldId"`
		FieldType   FieldType `bson:"fieldType" json:"fieldType"`
		DisplayName string    `bson:"displayName" json:"displayName"`
	}

	// Camera DO
	Camera struct {
		Id         Id     `bson:"_id" json:"id"`
		OrgId      Id     `bson:"orgId" json:"orgId"`
		AcceessKey string `bson:"accessKey" json:"accessKey"`
		SecretKey  string `bson:"secretKey" json:"secretKey"`
	}

	// A person DO
	Person struct {
		// Person id is generated by Frame Processor
		Id         Id        `bson:"_id" json:"id"`
		CamId      Id        `bson:"camId" json:"camId"`
		LastSeenAt Timestamp `bson:"lastSeenAt" json:"lastSeenAt"`
		ProfileId  Id        `bson:"profileId" json:"profileId"`
		Match      Match     `bson:"match" json:"match"`
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

	// A face description DO
	Face struct {
		PersonId   Id        `bson:"personId" json:"personId"`
		CapturedAt Timestamp `bson:"capturedAt" json:"capturedAt"`
		ImageId    Id        `bson:"imgId" json:"imgId"`
		Rect       Rectangle `bson:"rect" json:"rect"`
		V128D      V128D     `bson:"v128D" json:"v128D"`
	}

	// The status of matching process, please see MTCH_ST_ group of constants
	MatchStatus int

	Match struct {
		Status  MatchStatus   `bson:"status" json:"status"`
		Matches []PersonMatch `bson:"matches" json:"matches"`
	}

	// A person match structure
	PersonMatch struct {
		ProfileId Id      `bson:"profileId" json:"profileId"`
		Distance  float32 `bson:"distance" json:"distance"`
	}

	// An organization profile's information
	ProfileMeta struct {
		ProfileId Id             `bson:"profileId" json:"profileId"`
		OrgId     Id             `bson:"orgId" json:"orgId"`
		Metadata  map[int]string `bson:"meta" json:"meta"`
	}

	// Persister is an interface which provides an access to persistent layer
	Persister interface {
		NewMainPersister() MainPersister
		NewPartPersister(partId Id) PartPersister
	}

	// An transactional persister (has context dependant time)
	MainPersister interface {
		GetOrganizationDAO() CrudExecutor
		GetUserDAO() CrudExecutor
		GetCameraDAO() CrudExecutor
		Close()
	}

	// Partitioned persister
	PartPersister interface {
		GetPersonDAO() CrudExecutor
		GetPersonFacesDAO() CrudExecutor
		GetProfileMetasDAO() CrudExecutor
		Close()
	}

	CrudExecutor interface {
		Create(o interface{}) error
		CreateMany(objs ...interface{}) error
		Read(id Id, res interface{}) error
		Update(id Id, o interface{}) error
		Delete(id Id) error
	}
)

// Known field types for profile fields description
const (
	FLD_TYPE_STRING FieldType = "string"
)

const (
	MTCH_ST_NOT_STARTED = 0
	MTCH_ST_RUNNING     = 1
	MTCH_ST_DONE        = 2
)

//func (sc *Scene) String() string {
//	return fmt.Sprintf("%+v", *sc)
//}

//func (q *SceneQuery) String() string {
//	return fmt.Sprintf("{camId=%s, limit=%d}", q.CamId, q.Limit)
//}
