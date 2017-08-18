package rapi

import (
	"github.com/pixty/console/common"
)

type (
	PersonStatus string

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
		CamId      *common.Id         `json:"camId,omitempty"`
		CapturedAt common.ISO8601Time `json:"capturedAt"`
		LostAt     common.ISO8601Time `json:"lostAt"`

		// Status of the person. Please see explanations below. Possible values
		// are "matching", "assigned" and "unassigned".
		Status PersonStatus `json:"status"`

		// Contains Person <-> profile association. Could be nil, if there is
		// no such association
		Profile  *Profile       `json:"profile"`
		Matches  []*Profile     `json:"matches"`
		Pictures []*PictureInfo `json:"pictures"`
	}

	PictureInfo struct {
		Id        common.Id           `json:"id"`
		CamId     *common.Id          `json:"camId,omitempty"`
		Timestamp *common.ISO8601Time `json:"timestamp,omitempty"`
		Size      *common.Size        `json:"size,omitempty"`

		// Identifies a rectangle on the picture. Can be populated when the
		// object is used for describing a face (in Person object for instance)
		Rect    *common.Rectangle `json:"rect,omitempty"`
		PicURL  string            `json:"picURL"`
		FaceURL *string           `json:"url,omitempty"`
	}

	Scene struct {
		PicURL    string             `json:"url"`
		CamId     common.Id          `json:"camId"`
		Timestamp common.ISO8601Time `json:"timestamp,omitempty"`
		Persons   []*Person          `json:"persons"`
	}
)

const (
	// The person is in process of searching most best profile match. In the status
	// the matches list can be updated, new profiles can be found and added to the
	// list.
	cPS_MATCHING = "matching"

	// The person has a profile assigned (associated). The state means that a profile
	// is assigned to the person (profile field is not nil) and no matching process
	// is runing anymore. Matching list can be whether empty or not, but it is not
	// going to be updated.
	cPS_ASSIGNED = "assigned"

	// There is no profile to the person association (profile field is nil), but
	// matching process is over. The matching list is not going to be updated anymore.
	cPS_UNASSIGNED = "unassigned"
)
