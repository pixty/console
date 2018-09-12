package scene

import (
	"bytes"
	"container/list"
	"image"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jrivets/gorivets"
	imageSrv "github.com/pixty/console/service/image"

	"github.com/jrivets/log4g"
	"github.com/pixty/console/common"
	"github.com/pixty/console/common/fpcp"
	"github.com/pixty/console/model"
	"github.com/pixty/console/service/matcher"
	"golang.org/x/net/context"
)

type (
	SceneProcessor struct {
		Persister    model.Persister        `inject:"persister"`
		MainCtx      context.Context        `inject:"mainCtx"`
		CConfig      *common.ConsoleConfig  `inject:""`
		ImageService *imageSrv.ImageService `inject:""`
		Matcher      matcher.Matcher        `inject:"matcher"`
		logger       log4g.Logger
		cpCache      *cam_pictures_cache
		persCache    *persons_cache
		// Cutting faces border size
		border int
	}

	SceneTimeline struct {
		CamId       int64
		LatestPicId string
		Persons     []*model.Person
		// Map of personId -> []faces
		Faces map[string][]*model.Face
		// Map of profileId -> Profile
		Profiles map[int64]*model.Profile
		// Map of profileId -> MG ids
		Prof2MGs map[int64]int64
	}

	// A cache object which is used for storing last updated camera picture
	cam_pictures_cache struct {
		lock    sync.Mutex
		camPics gorivets.LRU
		deleted *list.List
		dead    *list.List
	}
)

func NewSceneProcessor() *SceneProcessor {
	sp := new(SceneProcessor)
	sp.logger = log4g.GetLogger("pixty.SceneProcessor")
	sp.border = 20
	sp.cpCache = new(cam_pictures_cache)
	sp.cpCache.camPics = gorivets.NewTtlLRU(1000, time.Minute, sp.cpCache.on_delete)
	sp.cpCache.deleted = list.New()
	sp.cpCache.dead = list.New()
	// keep a person information for 5 minutes to reduce the number of faces to be stored
	sp.persCache = new_persons_cache(time.Minute * time.Duration(5))
	return sp
}

// =========================== SceneProcessor ================================
// ----------------------------- LifeCycler ----------------------------------
func (sp *SceneProcessor) DiPhase() int {
	return common.CMP_PHASE_SCENE_SERVICE
}

func (sp *SceneProcessor) DiInit() error {
	sp.logger.Info("DiInit()")
	sp.ImageService.DeleteAllTmpFiles()

	go func() {
		sleepTime := time.Duration(sp.CConfig.ImgsTmpTTLSec) * time.Second
		sp.logger.Info("Runing cleaning images loop to check every ", sp.CConfig.ImgsTmpTTLSec, " seconds")
		for {
			select {
			case <-time.After(sleepTime):
				sp.cpCache.on_sweep(sp.ImageService)
			case <-sp.MainCtx.Done():
				sp.logger.Info("Shutting down cleaning temporary images loop")
				return
			}
		}
	}()
	return nil
}

func (sp *SceneProcessor) DiShutdown() {
	sp.logger.Info("Shutting down.")
}

// ------------------------------- Public ------------------------------------
// Handles scene object which is sent by FP. Returns error only in case of the
// packet is not properly formed.
func (sp *SceneProcessor) OnFPCPScene(camId int64, scene *fpcp.Scene) error {
	sp.logger.Debug("Got new scene from camId=", camId, " with ", scene.Persons, " persons on the scene")

	if scene == nil || scene.Frame == nil {
		sp.logger.Error("Got wrong Scene packet scene of the frame is nil ", scene)
		return common.NewError(common.ERR_INVALID_VAL, "Wrong packet")
	}

	frameId, err := strconv.ParseInt(scene.Frame.Id, 10, 64)
	if err != nil {
		sp.logger.Error("Got wrong Scene packet: cannot transform frameId to int err=", err)
		return err
	}

	// Filtering faces through the cache. Some faces can be rejected due to the cache rules
	f2f := make(map[int]*model.Face)
	if len(scene.Faces) > 0 {
		skpdPers := make([]string, 0, 1)
		for i, f := range scene.Faces {
			// toFace sets PersonId, Rect and V128D
			face, err := sp.toFace(f)
			if err != nil {
				sp.logger.Warn("Error while parsing face for camId=", camId, ", err=", err)
				return err
			}
			face.CapturedAt = scene.Frame.Timestamp
			face.SceneId = scene.Id

			// check whether we have to persist the face?
			if sp.persCache.should_be_added(face) {
				f2f[i] = face
			} else {
				sp.logger.Debug("Drop the face for personId=", face.PersonId, ", by the cache rules.")
				skpdPers = append(skpdPers, face.PersonId)
			}
		}

		if len(skpdPers) > 0 {
			// update last seen time
			sp.updateLastSeenTime(skpdPers, scene.Frame.Timestamp)
		}
	}

	// Now we checks should we store the frame picture permanently or just temporary
	pfx := imageSrv.PFX_TEMP
	if len(f2f) > 0 {
		pfx = imageSrv.PFX_PERM
	}

	imgFrameFN, err := sp.savePictures(pfx, camId, frameId, nil, scene.Frame.Pictures)
	if err != nil {
		sp.logger.Warn("Could not save frame pictures err=", err)
		return nil
	}
	sp.cpCache.set_cam_image(camId, imgFrameFN)

	if len(f2f) > 0 {
		faces := make([]*model.Face, 0, len(f2f))
		for i, f := range scene.Faces {
			face, ok := f2f[i]
			if !ok {
				continue
			}

			faces = append(faces, face)
			mr := face.Rect
			r := image.Rect(mr.LeftTop.X, mr.LeftTop.Y, mr.RightBottom.X, mr.RightBottom.Y)

			// save face pics
			imgFn, err := sp.savePictures(pfx, camId, frameId, &r, f.Pictures)
			if err != nil {
				sp.logger.Warn("Could not save a face pictures err=", err)
				return nil
			}
			face.ImageId = imgFrameFN
			face.FaceImageId = imgFn
		}

		// Looks good now, trying to store the data to DB
		err = sp.persistSceneFaces(camId, faces)
		if err != nil {
			sp.logger.Warn("Got the error while saving faces(", len(faces), ") to DB: err=", err, ", ignoring the scene :(")
		}
	}

	return nil
}

// Returns scene timeline object
func (sp *SceneProcessor) GetTimelineView(camId int64, maxTs common.Timestamp, limit int) (*SceneTimeline, error) {
	pp, err := sp.Persister.GetPartitionTx("FAKE")
	if err != nil {
		return nil, err
	}
	//transaction
	err = pp.Begin()
	if err != nil {
		return nil, err
	}
	defer pp.Commit()

	persons, err := pp.FindPersons(&model.PersonsQuery{CamId: camId, MaxLastSeenAt: maxTs, Order: model.PQO_LAST_SEEN_DESC, Limit: limit})
	if err != nil {
		return nil, err
	}

	// collecting selected persons ids
	pids := make([]string, len(persons))
	for i, p := range persons {
		pids[i] = p.Id
	}

	// collecting faces
	faces, err := pp.FindFaces(&model.FacesQuery{PersonIds: pids, Short: true})
	if err != nil {
		return nil, err
	}
	facesMap := make(map[string][]*model.Face)
	for _, f := range faces {
		farr, ok := facesMap[f.PersonId]
		if !ok {
			farr = make([]*model.Face, 0, 3)
		}
		farr = append(farr, f)
		facesMap[f.PersonId] = farr
	}
	sp.logger.Debug("faces=", faces, ", facesMap=", facesMap)

	// collecting MatchGroups and Profile Id
	mgArr := make([]int64, 0, len(persons))
	prArr := make([]int64, 0, len(persons))
	profRes := make(map[int64]*model.Profile) // will use it in result
	for _, p := range persons {
		if p.ProfileId > 0 {
			prArr = append(prArr, p.ProfileId)
			profRes[p.ProfileId] = nil // so far
		}
		if p.MatchGroup > 0 {
			mgArr = append(mgArr, p.MatchGroup)
		}
	}

	// First, getting profiles for the match groups. It can contain profiles,
	// that are not in prArr, because not all persons for the MGs could be selected
	// to persons
	prof2MGs, err := pp.GetProfilesByMGs(mgArr)
	if err != nil {
		return nil, err
	}
	for pid, _ := range prof2MGs {
		if _, ok := profRes[pid]; !ok {
			prArr = append(prArr, pid)
			profRes[pid] = nil
		}
	}

	profs, err := pp.GetProfiles(&model.ProfileQuery{ProfileIds: prArr, AllMeta: false})
	if err != nil {
		return nil, err
	}
	for _, prof := range profs {
		profRes[prof.Id] = prof
	}

	stl := new(SceneTimeline)
	stl.CamId = camId
	stl.Persons = persons
	stl.Faces = facesMap
	stl.Prof2MGs = prof2MGs
	stl.Profiles = profRes
	stl.LatestPicId = sp.cpCache.get_cam_image(camId)
	return stl, nil
}

// ------------------------------ Private ------------------------------------
func (sp *SceneProcessor) updateLastSeenTime(persIds []string, captAt uint64) {
	pp, err := sp.Persister.GetPartitionTx("FAKE")
	if err != nil {
		return
	}
	pp.UpdatePersonsLastSeenAt(persIds, captAt)
}

func (sp *SceneProcessor) removeDuplicatesUnordered(elements []string) []string {
    encountered := map[string]bool{}

    for v:= range elements {
        encountered[elements[v]] = true
    }

    result := []string{}
    for key, _ := range encountered {
        result = append(result, key)
    }
    return result
}

func (sp *SceneProcessor) findProfileForFaces(currentFaces []*model.Face) (int64, error) {

	sp.logger.Debug("Looking for currentFaces ", len(currentFaces))
	pp, err := sp.Persister.GetPartitionTx("faces")
	faces, err := pp.FindFaces(&model.FacesQuery{})
	if err != nil {
		return 0, err;
	}

	persons := []string{}

	for _, face := range faces {
		for _, currentFace := range currentFaces {
			if (common.MatchAdvancedV128D(face.V128D, currentFace.V128D, sp.CConfig.MchrDistance)) {
				persons = append(persons, face.PersonId);
			}
		}
   }

   // persons = sp.removeDuplicatesUnordered(persons)
   m := make(map[int64]int64)

   for _, personId := range persons {

   	profile, err := pp.GetPersonById(personId)

   	if err != nil {
			return 0, err;
		}

		m[profile.ProfileId] += 1
  	sp.logger.Debug("Found same persons ", personId, " profileId ", profile.ProfileId)
 	}

 	var maxId, maxValue int64
 	maxId = 0
 	maxValue = 0
 	for k, v := range m {
 		if maxValue < v {
 			maxId = k
 			maxValue = v
 		}
    sp.logger.Debug("k ", k, " v ", v)
  }

  sp.logger.Debug("maxId ", maxId, " maxValue ", maxValue)
  return maxId, nil;

}

func (sp *SceneProcessor) persistSceneFaces(camId int64, faces []*model.Face) error {
	sp.logger.Debug("Updating ", len(faces), " faces into DB")
	pp, err := sp.Persister.GetPartitionTx("FAKE")
	if err != nil {
		return err
	}
	err = pp.Begin()
	if err != nil {
		return err
	}
	defer pp.Commit()

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
		newPers := make([]*model.Person, 0, npc)
		createdAt := common.CurrentTimestamp()
		for pid, f := range persIdMap {
			p := new(model.Person)
			p.Id = pid
			foundId, err := sp.findProfileForFaces(faces)
			if err != nil {
				sp.persCache.mark_person_as_new(p.Id)
				sp.logger.Info("Not found mark as new....")
			} else {
				p.ProfileId = foundId
				sp.logger.Info("Set ProfileId to....... ", foundId)
			}

			p.CamId = camId
			p.CreatedAt = uint64(createdAt)
			p.LastSeenAt = f.CapturedAt
			p.PictureId = f.FaceImageId
			persons = append(persons, p)
			newPers = append(newPers, p)
			// marks the person as seen first time on the scene (affects faces filtering)
			// sp.persCache.mark_person_as_new(p.Id)
		}
		err := pp.InsertPersons(newPers)
		if err != nil {
			sp.logger.Error("Could not insert new persons, err=", err)
			pp.Rollback()
			return err
		}
	}

	err = pp.InsertFaces(faces)
	if err != nil {
		sp.logger.Error("Could not insert new faces, err=", err)
		pp.Rollback()
		return err
	}

	pp.Commit()
	sp.Matcher.OnNewFaces(camId, persons, faces)

	return nil
}

func (sp *SceneProcessor) savePictures(pfx string, camId, frameId int64, rect *image.Rectangle, pics []*fpcp.Picture) (string, error) {
	imDesc := &imageSrv.ImgDesc{Prefix: pfx, CamId: camId, FrameId: frameId, Rect: rect}
	res := ""
	for _, pic := range pics {
		var err error
		res, err = sp.savePicture(imDesc, pic)
		if err != nil {
			sp.logger.Warn("could not save file by descriptor ", imDesc)
			sp.ImageService.DeleteImageByFile(res)
			return "", err
		}
	}
	return res, nil
}

func (sp *SceneProcessor) savePicture(imDesc *imageSrv.ImgDesc, pic *fpcp.Picture) (string, error) {
	imDesc.Size = byte(pic.SizeCode)
	if pic.Format == fpcp.Picture_JPG {
		imDesc.Format = imageSrv.IMG_FRMT_JPEG
	} else {
		imDesc.Format = imageSrv.IMG_FRMT_PNG
	}
	return sp.ImageService.StoreImage(imDesc, bytes.NewReader(pic.Data))
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
		sp.logger.Warn("We got a face for personId=", face.Id, ", but it doesn't have proper vector information (array is nil, or length is not 128 elements) face.Vector=", face.Vector)
		return nil, common.NewError(common.ERR_INVALID_VAL, "128 dimensional vector of face is expected.")
	}
	f.V128D = common.V128D(face.Vector)
	return f, nil
}

func (cpc *cam_pictures_cache) set_cam_image(camId int64, imgFile string) {
	cpc.lock.Lock()
	defer cpc.lock.Unlock()

	cpc.camPics.Add(camId, imgFile, 1)
}

func (cpc *cam_pictures_cache) get_cam_image(camId int64) string {
	cpc.lock.Lock()
	defer cpc.lock.Unlock()

	if imgFile, ok := cpc.camPics.Peek(camId); ok {
		return imgFile.(string)
	}
	return ""
}

func (cpc *cam_pictures_cache) on_delete(k, v interface{}) {
	if strings.HasPrefix(v.(string), imageSrv.PFX_TEMP) {
		cpc.deleted.PushBack(v)
	}
}

func (cpc *cam_pictures_cache) on_sweep(imgS *imageSrv.ImageService) {
	cpc.lock.Lock()
	defer cpc.lock.Unlock()

	cpc.camPics.Sweep()

	for cpc.dead.Len() > 0 {
		imgFile := cpc.dead.Remove(cpc.dead.Front()).(string)
		imgS.DeleteImageByFile(imgFile)
	}

	// swap lists
	tmp := cpc.dead
	cpc.dead = cpc.deleted
	cpc.deleted = tmp
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
