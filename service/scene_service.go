package service

import (
	"errors"

	"github.com/jrivets/log4g"
	"github.com/pixty/console/common"
	"github.com/pixty/console/common/fpcp"
	"github.com/pixty/console/model"
)

type (
	SceneProcessor struct {
		Persister model.Persister `inject:"perisister"`
		logger    log4g.Logger
	}
)

func NewSceneProcessor() *SceneProcessor {
	sp := new(SceneProcessor)
	sp.logger = log4g.GetLogger("pixty.SceneProcessor")
	return sp
}

// =========================== SceneProcessor ================================
// ----------------------------- LifeCycler ----------------------------------
func (sp *SceneProcessor) DiPhase() int {
	return common.CMP_PHASE_SCENE_SERVICE
}

func (sp *SceneProcessor) DiInit() error {
	sp.logger.Info("DiInit()")
	return nil
}

func (sp *SceneProcessor) DiShutdown() {
	sp.logger.Info("Shutting down.")
}

// ---------------------------- SceneService ---------------------------------
func (sp *SceneProcessor) onFPCPScene(camId string, scene *fpcp.Scene) error {
	sp.logger.Debug("Got new scene from camId=", camId, " with ", scene.Persons, " persons on the scene")
	
	if scene.Faces != nil && len(scene.Faces) > 0 {
		faces := make([]*model.Face, len(scene.Faces))
		for i, f := range scene.Faces {
			faces[i], err := sp.toFace(f)
			if err != nil {
				sp.logger.Warn("Error while parsing face for camId=", camId, ", err=", err)
				return err
			}
		}
	}
	
	
}

// creates new face and fills it partially
func (sp *SceneProcessor) toFace(face *fpcp.Face) (*model.Face, error) {
	if face == nil {
		return nil, nil
	}
	f := new(model.Face)
	f.PersonId = face.Id
	toRect(face.Rect, &f.Rect)
	if face.Vector == nil || len(face.Vector) != 128 {
		sp.logger.Warn("We got a face for personId=", face.Id, ", but it doesn't have proper vector information (array is nil, or length is not 128 elements)")
		return nil, errors.New("128 dimensional vector of face is expected.")
	}
	f.V128D = V128D(face.Vector)
	return f, nil
}

func toRect(fr *fpcp.Rectangle, r *model.Rectangle) {
	if fr == nil || r == nil {
		return
	}
	r.LeftTop.X = fr.Left
	r.LeftTop.Y = fr.Top
	r.RightBottom.X = fr.Right
	r.RightBottom.Y = fr.Bottom
}
