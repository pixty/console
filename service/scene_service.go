package service

import (
	"bytes"
	"errors"
	"image"
	"image/png"
	"sync"

	"github.com/jrivets/gorivets"

	"github.com/jrivets/log4g"
	"github.com/pixty/console/common"
	"github.com/pixty/console/common/fpcp"
	"github.com/pixty/console/model"
)

type (
	SceneProcessor struct {
		Persister  model.Persister     `inject:"perisister"`
		ImgService common.ImageService `inject:"imgService"`
		logger     log4g.Logger
		cpCache    cam_pictures_cache
		// Cutting faces border size
		border int
	}

	// A cache object which is used for storing last updated camera picture
	cam_pictures_cache struct {
		lock    sync.Mutex
		camPics map[string]string
	}
)

func NewSceneProcessor() *SceneProcessor {
	sp := new(SceneProcessor)
	sp.logger = log4g.GetLogger("pixty.SceneProcessor")
	sp.border = 20
	sp.cpCache.camPics = make(map[string]string)
	return sp
}

// =========================== SceneProcessor ================================
// ----------------------------- LifeCycler ----------------------------------
func (sp *SceneProcessor) DiPhase() int {
	return common.CMP_PHASE_SCENE_SERVICE
}

func (sp *SceneProcessor) DiInit() error {
	sp.logger.Info("DiInit()")
	sp.ImgService.DeleteAllWithPrefix(common.IMG_TMP_CAM_PREFIX)
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
			var err error
			// toFace put PersonId, Rect and V128D
			faces[i], err = sp.toFace(f)
			if err != nil {
				sp.logger.Warn("Error while parsing face for camId=", camId, ", err=", err)
				return err
			}
			faces[i].CapturedAt = scene.Frame.Timestamp
			faces[i].SceneId = scene.Id
		}

		err := sp.saveFaceImages(camId, scene.Frame, faces)
		if err != nil {
			sp.logger.Warn("Got the error while saving images: err=", err, ", ignoring the scene :(")
			return nil
		}

		// Looks good now, trying to store the data to DB
		err = sp.persistSceneFaces(camId, faces)
		if err != nil {
			sp.logger.Warn("Got the error while saving faces(", len(faces), ") to DB: err=", err, ", ignoring the scene :(")
		}
	} else {
		imgId := common.ImgMakeTmpCamId(camId, common.Timestamp(scene.Frame.Timestamp))
		err := sp.saveFrameImage(imgId, camId, scene.Frame)
		if err != nil {
			sp.logger.Warn("Could not save the frame image into a temporary file, err=", err)
		}
	}

	return nil
}

func (sp *SceneProcessor) persistSceneFaces(camId string, faces []*model.Face) error {
	sp.logger.Debug("Updating ", len(faces), " faces into DB")
	pp := sp.Persister.GetPartPersister("FAKE")
	persIds := make([]string, len(faces))
	persIdMap := make(map[string]*model.Face)
	for i, f := range faces {
		persIds[i] = f.PersonId
		persIdMap[f.PersonId] = f
	}

	persons, err := pp.FindPersons(&model.PersonsQuery{PersonIds: persIds})
	if err != nil {
		sp.logger.Error("Could not find persons by ids=", persIds, ", err=", err)
		return err
	}

	if len(persons) > 0 {
		exists := make([]string, len(persons))
		for _, p := range persons {
			exists = append(exists, p.Id)
			delete(persIdMap, p.Id)
		}
		err := pp.UpdatePersonsLastSeenAt(exists, faces[0].CapturedAt)
		if err != nil {
			sp.logger.Error("Could not update last seen at time for ids=", exists, ", err=", err)
			return err
		}
	}

	npc := len(persIdMap)
	if npc > 0 {
		sp.logger.Info("Found ", npc, " new person(s) on ", camId, ", will persist them...")
		newPers := make([]*model.Person, npc)
		for pid, f := range persIdMap {
			p := new(model.Person)
			p.Id = pid
			p.CamId = camId
			p.LastSeenAt = f.CapturedAt
			p.PictureId = f.FaceImageId
			newPers = append(newPers, p)
		}
		err := pp.InsertPersons(newPers)
		if err != nil {
			sp.logger.Error("Could not insert new persons, err=", err)
			return err
		}
	}

	err = pp.InsertFaces(faces)
	if err != nil {
		sp.logger.Error("Could not insert new faces, err=", err)
		return err
	}
	return nil
}

// Cut faces and store them images. Also pupulates the following fields
// - ImageId
// - FaceImageId
// for the faces array provided
func (sp *SceneProcessor) saveFaceImages(camId string, frame *fpcp.Frame, faces []*model.Face) error {
	if frame.Data == nil || len(frame.Data) == 0 {
		sp.logger.Warn("saveFaceImages(): No frame data for camId=", camId)
		return errors.New("Expecting image in the frame, but not found it.")
	}

	img, err := png.Decode(bytes.NewReader(frame.Data))
	if err != nil {
		sp.logger.Warn("Cannot decode png image err=", err)
		return err
	}

	// making Image Id here, and store the frame
	imgId := common.ImgMakeCamId(camId, common.Timestamp(frame.Timestamp))
	imgIdRef := imgId
	err = sp.saveFrameImage(imgId, camId, frame)
	if err != nil {
		sp.logger.Error("Could not write the frame image to imgService, err=", err)
		imgIdRef = ""
	}

	var rect image.Rectangle
	for _, face := range faces {
		// Update reference to frame
		face.ImageId = string(imgIdRef)

		// Cutting png image
		toImgRect(&face.Rect, &rect, frame.Size, sp.border)
		si := img.(interface {
			SubImage(r image.Rectangle) image.Image
		}).SubImage(rect)

		bb := bytes.NewBuffer([]byte{})
		err = png.Encode(bb, si)
		if err != nil {
			sp.logger.Warn("Cannot encode png image err=", err)
			continue
		}

		// Store the face to image store
		idesc := &common.ImageDescriptor{
			Id:        common.Id(common.ImgMakeId(imgId, &rect)),
			Reader:    bytes.NewReader(bb.Bytes()),
			FileName:  common.ImgMakeFileName(imgId, &rect),
			CamId:     common.Id(camId),
			Width:     rect.Dx(),
			Height:    rect.Dy(),
			Timestamp: common.Timestamp(frame.Timestamp),
		}
		_, err = sp.ImgService.New(idesc)
		if err != nil {
			sp.logger.Error("Could not write image to imgService, err=", err)
			continue
		}
		face.FaceImageId = string(idesc.Id)
	}
	return nil
}

func (sp *SceneProcessor) saveFrameImage(imgId string, camId string, frame *fpcp.Frame) error {
	idesc := &common.ImageDescriptor{
		Id:        common.Id(imgId),
		Reader:    bytes.NewReader(frame.Data),
		FileName:  common.ImgMakeFileName(imgId, nil),
		CamId:     common.Id(camId),
		Width:     int(frame.Size.Width),
		Height:    int(frame.Size.Height),
		Timestamp: common.Timestamp(frame.Timestamp),
	}
	_, err := sp.ImgService.New(idesc)
	if err == nil {
		oldImgId := sp.cpCache.set_cam_image(camId, imgId)
		if common.ImgIsTmpCamId(oldImgId) {
			sp.ImgService.Delete(common.Id(oldImgId))
		}
	}
	return err
}

// creates new face and fills it partially by populating:
// - PersonId
// - Rect
// - V128D
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
	f.V128D = common.V128D(face.Vector)
	return f, nil
}

func (cpc cam_pictures_cache) set_cam_image(camId string, imgId string) string {
	cpc.lock.Lock()
	defer cpc.lock.Unlock()

	prev, _ := cpc.camPics[camId]
	cpc.camPics[camId] = imgId
	return prev
}

func (cpc cam_pictures_cache) get_cam_image(camId string) string {
	return cpc.camPics[camId]
}

func toImgRect(r *model.Rectangle, res *image.Rectangle, sz *fpcp.Size, expand int) {
	res.Min.X = gorivets.Max(0, r.LeftTop.X-expand)
	res.Min.Y = gorivets.Max(0, r.LeftTop.Y-expand)
	res.Max.X = gorivets.Min(int(sz.Width), r.RightBottom.X+expand)
	res.Max.Y = gorivets.Min(int(sz.Height), r.RightBottom.Y+expand)
}

func toRect(fr *fpcp.Rectangle, r *model.Rectangle) {
	if fr == nil || r == nil {
		return
	}
	r.LeftTop.X = int(fr.Left)
	r.LeftTop.Y = int(fr.Top)
	r.RightBottom.X = int(fr.Right)
	r.RightBottom.Y = int(fr.Bottom)
}
