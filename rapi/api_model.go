package rapi

import (
	"github.com/pixty/console/common"
	"github.com/pixty/fpcp"
)

type (
	Organization struct {
		Id common.Id `json:"id"`
	}

	Camera struct {
		Id    common.Id `json:"id"`
		OrgId common.Id `json:"orgId"`
	}

	Profile struct {
		Id    common.Id `json:"id"`
		OrgId common.Id `json:"orgId"`

		// Key-Value pairs for the organization
		Attributes map[string]string `json:"attributes"`
	}

	Person struct {
		Id         common.Id          `json:"id"`
		CamId      common.Id          `json:"camId"`
		CapturedAt common.ISO8601Time `json:"capturedAt"`
		LostAt     common.ISO8601Time `json:"lostAt"`

		// Contains Person <-> profile association. Could be nil, if there is
		// no such association
		Profile  *Profile        `json:"profile"`
		Matches  []*ProfileMatch `json:"matches"`
		Pictures []*PictureInfo  `json:"pictures"`
	}

	ProfileMatch struct {
		// an integer, which indicates the occuracy in percentage [0..100]
		Occuracy int `json:"occuracy"`

		// Profile data
		Profile *Profile `json:"profile"`
	}

	PictureInfo struct {
		Id        common.Id          `json:"id"`
		Timestamp common.ISO8601Time `json:"timestamp"`
		Size      fpcp.RectSize      `json:"size"`

		// Identifies a rectangle on the picture. Can be populated when the
		// object is used for describing a face (in Person object for instance)
		Rect *fpcp.Rect `json:"rect"`
	}

	Scene struct {
		CamId     common.Id          `json:"camId"`
		Timestamp common.ISO8601Time `json:"timestamp"`
		Persons   []*Person          `json:"persons"`
	}
)
