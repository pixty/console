package rapi

import "github.com/pixty/console/common"

type (
	Scene struct {
		CamId     common.Id          `json:"camId"`
		Timestamp common.ISO8601Time `json:"timestamp"`
		Persons   []*ScenePerson     `json:"persons"`
	}

	ScenePerson struct {
		Person     *Person            `json:"person"`
		CapturedAt common.ISO8601Time `json:"capturedAt"`
		PicId      common.Id          `json:"picId"`
		PicTime    common.ISO8601Time `json:"picTime"`
		PicPos     *common.Point      `json:"picPos"`
	}

	Person struct {
		Id common.Id `json:"id"`

		// Contains list of organiations where the person has attributes
		OrgsData []*OrgAttributes `json:"orgsData"`
	}

	OrgAttributes struct {
		OrgId      common.Id         `json:"orgId"`
		Attributes map[string]string `json:"attributes"`
		QueryStats map[string]string `json:"queryStats"`
	}
)
