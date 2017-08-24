package rapi

import (
	"github.com/pixty/console/common"
	"github.com/pixty/console/model"
)

type (
	PersonStatus string

	Organization struct {
		Id   common.Id     `json:"id"`
		Meta []OrgMetaInfo `json:"metaInfo"`
	}

	OrgMetaInfo struct {
		FieldName string `json:"fieldName"`
		FieldType string `json:"fieldType"`
	}

	Camera struct {
		Id    common.Id `json:"id"`
		OrgId common.Id `json:"orgId"`
	}

	Profile struct {
		Id        common.Id `json:"id"`
		AvatarUrl string    `json:"avatarUrl"`

		// Key-Value pairs for the organization
		Attributes map[string]string `json:"attributes,omitempty"`
	}

	Person struct {
		Id         string             `json:"id"`
		CamId      *string            `json:"camId,omitempty"`
		LastSeenAt common.ISO8601Time `json:"lastSeenAt"`
		AvatarUrl  string             `json:"avatarUrl"`

		// Contains Person <-> profile association. Could be nil, if there is
		// no such association. This assignment is done manually only.
		Profile  *Profile       `json:"profile"`
		Matches  []*Profile     `json:"matches"`
		Pictures []*PictureInfo `json:"pictures"`
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
		CamId     common.Id          `json:"camId"`
		Timestamp common.ISO8601Time `json:"timestamp,omitempty"`
		Persons   []*Person          `json:"persons"`
		Frame     PictureInfo        `json:"frame"`
	}
)
