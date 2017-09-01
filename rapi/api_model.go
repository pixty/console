package rapi

import (
	"github.com/pixty/console/common"
	"github.com/pixty/console/model"
)

type (
	PersonStatus string

	Organization struct {
		Id   int64          `json:"id"`
		Name string         `json:"name"`
		Meta OrgMetaInfoArr `json:"metaInfo"`
	}

	OrgMetaInfoArr []*OrgMetaInfo

	OrgMetaInfo struct {
		Id        int64  `json:"id"`
		FieldName string `json:"fieldName"`
		FieldType string `json:"fieldType"`
	}

	Camera struct {
		Id           string  `json:"id"`
		OrgId        int64   `json:"orgId"`
		HasSecretKey bool    `json:"hasSecretKey"`
		SecretKey    *string `json:"secretKey,omitempty"`
	}

	Profile struct {
		Id        int64   `json:"id, omitempty"`
		OrgId     int64   `json:"orgId,omitempty"`
		AvatarUrl *string `json:"avatarUrl,omitempty"`

		// Key-Value pairs for the organization
		Attributes []*ProfileAttribute `json:"attributes,omitempty"`
	}

	ProfileAttribute struct {
		FieldId *int64  `json:"fieldId, omitempty"`
		Name    *string `json:"name, omitempty"`
		Value   *string `json:"value, omitempty"`
	}

	Person struct {
		Id         string             `json:"id"`
		CamId      *string            `json:"camId,omitempty"`
		LastSeenAt common.ISO8601Time `json:"lastSeenAt"`
		AvatarUrl  string             `json:"avatarUrl"`
		ProfileId  *int64             `json:"profileId,omitempty"`

		// Contains Person <-> profile association. Could be nil, if there is
		// no such association. This assignment is done manually only.
		Profile  *Profile       `json:"profile"`
		Matches  []*Profile     `json:"matches,omitempty"`
		Pictures []*PictureInfo `json:"pictures,omitempty"`
	}

	PictureInfo struct {
		Id        string              `json:"id"`
		CamId     *string             `json:"camId,omitempty"`
		Timestamp *common.ISO8601Time `json:"timestamp,omitempty"`
		Size      *model.Size         `json:"size,omitempty"`

		// Identifies a rectangle on the picture. Can be populated when the
		// object is used for describing a face (in Person object for instance)
		Rect    *model.Rectangle `json:"rect,omitempty"`
		PicURL  string           `json:"picURL"`
		FaceURL *string          `json:"url,omitempty"`
	}

	SceneTimeline struct {
		CamId   common.Id   `json:"camId"`
		Persons []*Person   `json:"persons"`
		Frame   PictureInfo `json:"frame"`
	}
)
