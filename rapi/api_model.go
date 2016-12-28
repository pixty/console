package rapi

import "github.com/pixty/console/common"

type (
	Scene struct {
		CamId     common.Id          `json:"camId"`
		Timestamp common.ISO8601Time `json:"timestamp"`
		Persons   []*ScenePerson     `json:"persons"`
	}

	ScenePerson struct {
		Person       *Person            `json:"person"`
		OnSceneSince common.ISO8601Time `json:"onSceneSince"`
		PicId        common.Id          `json:"picId"`
		PicTime      common.ISO8601Time `json:"picTime"`
		PicPos       *Position          `json:"picPos"`
	}

	Position struct {
		X int `json:"x"`
		Y int `json:"y"`
	}

	Person struct {
		Id       common.Id        `json:"id"`
		OrgsData []*OrgAttributes `json:"orgsData"`
	}

	OrgAttributes struct {
		OrgId      common.Id         `json:"orgId"`
		Attributes map[string]string `json:"attributes"`
		QueryStats map[string]string `json:"queryStats"`
	}
)
