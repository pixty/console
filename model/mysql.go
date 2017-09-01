package model

import (
	"context"
	"database/sql"
	"io/ioutil"
	"strconv"
	"strings"
	"sync"

	"github.com/go-sql-driver/mysql"
	"github.com/jrivets/log4g"
	"github.com/pixty/console/common"
)

// Implementation of common.Persister + LifeCycler
type (
	MysqlPersister struct {
		// The console configuration. Will be injected
		Config *common.ConsoleConfig `inject:""`
		logger log4g.Logger
		// Keep main connection all the time
		mainConn *msql_connection
	}

	// Connection to database, just establishes connection and keeps pool via
	// DB object
	msql_connection struct {
		lock    sync.Mutex
		logger  log4g.Logger
		conName string
		ds      string
		db      *sql.DB
	}

	// The object is provided for making requests to MySQL, it abstracts
	// whether sql.DB or sql.Tx objects. Decorates calls to sql.DB or sql.TX
	msql_executor interface {
		Exec(query string, args ...interface{}) (sql.Result, error)
		Query(query string, args ...interface{}) (*sql.Rows, error)
	}

	// The object keeps connection to DB and supports transaction management
	msql_tx struct {
		logger log4g.Logger
		// never nil
		db *sql.DB
		// keeps active transaction, if it exists
		tx *sql.Tx
	}

	msql_main_tx struct {
		*msql_tx
	}

	msql_part_tx struct {
		*msql_tx
	}
)

func NewMysqlPersister() *MysqlPersister {
	mp := new(MysqlPersister)
	mp.logger = log4g.GetLogger("pixty.mysql")
	return mp
}

// =========================== MysqlPersister ================================
// ----------------------------- LifeCycler ----------------------------------
func (mp *MysqlPersister) DiPhase() int {
	return common.CMP_PHASE_DB
}

func (mp *MysqlPersister) DiInit() error {
	mp.logger.Info("Initializing.")
	mp.mainConn = mp.newConnection(mp.Config.MysqlDatasource, log4g.GetLogger("pixty.mysql.main"))
	return nil
}

func (mp *MysqlPersister) DiShutdown() {
	mp.logger.Info("Shutting down.")
}

// ------------------------------ Persister ----------------------------------
func (mp *MysqlPersister) GetMainTx() (MainTx, error) {
	tx, err := mp.makeTx(mp.mainConn)
	if err != nil {
		return nil, err
	}
	return &msql_main_tx{msql_tx: tx}, nil
}

func (mp *MysqlPersister) GetPartitionTx(partId string) (PartTx, error) {
	tx, err := mp.makeTx(mp.mainConn)
	if err != nil {
		return nil, err
	}
	tx.logger = log4g.GetLogger("pixty.mysql." + partId)
	return &msql_part_tx{msql_tx: tx}, nil
}

// -------------------------------- Misc -------------------------------------
func (mp *MysqlPersister) makeTx(mc *msql_connection) (*msql_tx, error) {
	db, err := mc.getDb()
	if err != nil {
		return nil, err
	}
	return &msql_tx{db: db, logger: mc.logger}, nil
}

//The method cuts password to make it use in logs
func (mp *MysqlPersister) getFriendlyName(ds string) string {
	cfg, err := mysql.ParseDSN(ds)
	if err != nil {
		mp.logger.Error("Could not parse DSN properly: ", err)
		return "N/A"
	}
	return cfg.User + "@" + cfg.Addr + "/" + cfg.DBName
}

func (mp *MysqlPersister) newConnection(ds string, logger log4g.Logger) *msql_connection {
	conName := mp.getFriendlyName(ds)
	mp.logger.Info("new connection to ", conName)
	mc := new(msql_connection)
	mc.conName = conName
	mc.ds = ds
	mc.logger = logger
	return mc
}

// =========================== msql_connection ===============================
func (mc *msql_connection) close() {
	mc.lock.Lock()
	defer mc.lock.Unlock()
	mc.logger.Info("Close DB connection to ", mc.conName)
	if mc.db != nil {
		mc.db.Close()
		mc.db = nil
	}
}

func (mc *msql_connection) getDb() (*sql.DB, error) {
	db := mc.db
	if db == nil {
		mc.lock.Lock()
		defer mc.lock.Unlock()

		if mc.db != nil {
			return mc.db, nil
		}
		mc.logger.Info("Connecting to ", mc.conName)

		var err error
		db, err = sql.Open("mysql", mc.ds)
		if err != nil {
			mc.logger.Error("Could not open connection, err=", err)
			return nil, err
		}
		mc.db = db
	}
	return db, nil
}

// ============================== msql_tx ====================================
func (mtx *msql_tx) executor() msql_executor {
	if mtx.tx == nil {
		return mtx.db
	}
	return mtx.tx
}

// Begins tx in serializable isolation level
func (mtx *msql_tx) BeginSerializable() error {
	return mtx.begin(&sql.TxOptions{Isolation: sql.LevelSerializable})
}

func (mtx *msql_tx) Begin() error {
	return mtx.begin(nil)
}

func (mtx *msql_tx) begin(ops *sql.TxOptions) error {
	mtx.Commit()
	tx, err := mtx.db.BeginTx(context.TODO(), ops)
	if err == nil {
		mtx.tx = tx
	}
	return err
}

func (mtx *msql_tx) Rollback() error {
	var err error
	if mtx.tx != nil {
		err = mtx.tx.Rollback()
		mtx.tx = nil
	}
	mtx.logger.Debug("Rollback() done. err=", err)
	return err
}

func (mtx *msql_tx) Commit() error {
	var err error
	if mtx.tx != nil {
		err = mtx.tx.Commit()
		mtx.tx = nil
	}
	mtx.logger.Debug("Commit() done. err=", err)
	return err
}

func (mtx *msql_tx) ExecQuery(sqlQuery string, params ...interface{}) error {
	_, err := mtx.executor().Exec(sqlQuery, params...)
	return err
}

func (mtx *msql_tx) ExecScript(sqlScript string) error {
	file, err := ioutil.ReadFile(sqlScript)

	if err != nil {
		mtx.logger.Warn("ExecScript(): Could not find/read script file ", sqlScript, ", err=", err)
		return err
	}

	requests := strings.Split(string(file), ";")

	for _, request := range requests {
		if strings.Trim(request, " ") == "" {
			continue
		}
		err := mtx.ExecQuery(request)
		if err != nil {
			mtx.logger.Warn("ExecScript(): Ooops, error while executing the following statement request=", request, ", err=", err)
			return err
		}
	}
	return nil
}

// ========================= msql_main_persister =============================
func (mmp *msql_main_tx) FindCameraById(camId string) (*Camera, error) {
	rows, err := mmp.executor().Query("SELECT org_id, secret_key FROM camera WHERE id=?", camId)
	if err != nil {
		mmp.logger.Warn("Could read camera by id=", camId, ", err=", err)
		return nil, err
	}
	defer rows.Close()

	if rows.Next() {
		cam := new(Camera)
		err := rows.Scan(&cam.OrgId, &cam.SecretKey)
		cam.Id = camId
		if err != nil {
			mmp.logger.Warn("FindCameraById(): could not scan result err=", err)
		}
		return cam, nil
	}
	return nil, nil
}

func (mmp *msql_main_tx) InsertOrg(org *Organization) (int64, error) {
	res, err := mmp.executor().Exec("INSERT INTO organization(name) VALUES (?)", org.Name)
	if err != nil {
		mmp.logger.Warn("InsertOrg(): Could not insert new organization ", org, ", got the err=", err)
		return -1, err
	}

	return res.LastInsertId()
}

func (mmp *msql_main_tx) GetOrg(orgId int64) (*Organization, error) {
	rows, err := mmp.executor().Query("SELECT name FROM organization WHERE id=?", orgId)
	if err != nil {
		mmp.logger.Warn("GetOrg(): Could not get organization by orgId=", orgId, ", got the err=", err)
		return nil, err
	}
	defer rows.Close()
	if rows.Next() {
		org := new(Organization)
		org.Id = orgId
		rows.Scan(&org.Name)
		return org, nil
	}

	mmp.logger.Warn("GetOrg(): No organization by orgId=", orgId)
	return nil, common.NewError(common.ERR_NOT_FOUND, "No org for orgId="+strconv.FormatInt(orgId, 10))
}

// ========================= msql_part_persister =============================

func (mpp *msql_part_tx) InsertCamera(cam *Camera) error {
	_, err := mpp.executor().Exec("INSERT INTO camera(id, org_id, secret_key) VALUES (?,?,?)",
		cam.Id, cam.OrgId, cam.SecretKey)
	if err != nil {
		mpp.logger.Warn("InsertCamera(): Could not insert new camera ", cam, ", got the err=", err)
		return err
	}
	return nil
}

func (mpp *msql_part_tx) GetCameraById(camId string) (*Camera, error) {
	rows, err := mpp.executor().Query("SELECT org_id, secret_key FROM camera WHERE id=?", camId)
	if err != nil {
		mpp.logger.Warn("GetCameraById(): Getting camera by id=", camId, ", got the err=", err)
		return nil, err
	}
	defer rows.Close()
	if rows.Next() {
		c := new(Camera)
		rows.Scan(&c.OrgId, &c.SecretKey)
		c.Id = camId
		return c, nil
	}
	return nil, common.NewError(common.ERR_NOT_FOUND, "Could not find camera with id="+camId)
}

func (mpp *msql_part_tx) UpdateCamera(cam *Camera) error {
	_, err := mpp.executor().Exec("UPDATE camera SET secret_key=? WHERE id=?",
		cam.SecretKey, cam.Id)
	if err != nil {
		mpp.logger.Warn("UpdateCamera(): Could not update camera ", cam, ", got the err=", err)
		return err
	}
	return nil
}

func (mpp *msql_part_tx) DeleteCamera(camId string) error {
	_, err := mpp.executor().Exec("DELETE camera WHERE id=?", camId)
	if err != nil {
		mpp.logger.Warn("DeleteCamera(): Could not delete camera with id=", camId, ", got the err=", err)
		return err
	}
	return nil
}

func (mpp *msql_part_tx) FindCameras(q *CameraQuery) ([]*Camera, error) {
	rows, err := mpp.executor().Query("SELECT id, org_id, secret_key FROM camera WHERE org_id=?", q.OrgId)
	if err != nil {
		mpp.logger.Warn("FindCameras(): Getting cameras by query=", q, ", got the err=", err)
		return nil, err
	}
	defer rows.Close()
	res := []*Camera{}
	for rows.Next() {
		c := new(Camera)
		rows.Scan(&c.Id, &c.OrgId, &c.SecretKey)
		res = append(res, c)
	}
	return res, nil
}

func (mpp *msql_part_tx) InsertFace(f *Face) (int64, error) {
	res, err := mpp.executor().Exec("INSERT INTO face(scene_id, person_id, captured_at, image_id, img_top, img_left, img_bottom, img_right, face_image_id, v128d) VALUES (?,?,?,?,?,?,?,?,?,?)",
		f.SceneId, f.PersonId, f.CapturedAt, f.ImageId, f.Rect.LeftTop.Y, f.Rect.LeftTop.X, f.Rect.RightBottom.Y, f.Rect.RightBottom.X, f.FaceImageId, f.V128D.ToByteSlice())
	if err != nil {
		mpp.logger.Warn("InsertFace(): Could not insert new face ", f, ", got the err=", err)
		return -1, err
	}

	return res.LastInsertId()
}

func (mpp *msql_part_tx) InsertFaces(faces []*Face) error {
	mpp.logger.Debug("InsertFaces() ", len(faces), " faces to DB: ", faces)
	if len(faces) > 0 {
		q := "INSERT INTO face(scene_id, person_id, captured_at, image_id, img_top, img_left, img_bottom, img_right, face_image_id, v128d) VALUES "
		vals := []interface{}{}
		for i, f := range faces {
			if i > 0 {
				q = q + ", "
			}
			q = q + "(?,?,?,?,?,?,?,?,?,?)"
			vals = append(vals, f.SceneId, f.PersonId, f.CapturedAt, f.ImageId, f.Rect.LeftTop.Y, f.Rect.LeftTop.X, f.Rect.RightBottom.Y, f.Rect.RightBottom.X, f.FaceImageId, f.V128D.ToByteSlice())
		}

		_, err := mpp.executor().Exec(q, vals...)
		if err != nil {
			mpp.logger.Warn("InserFaces(): Could not execute insert query ", q, ", err=", err)
			return err
		}
	}
	return nil
}

func (mpp *msql_part_tx) GetFaceById(fId int64) (*Face, error) {
	rows, err := mpp.executor().Query("SELECT scene_id, person_id, captured_at, image_id, img_top, img_left, img_bottom, img_right, face_image_id, v128d FROM face WHERE id=?", fId)
	if err != nil {
		mpp.logger.Warn("GetFaceById(): could read face by id=", fId, ", err=", err)
		return nil, err
	}
	defer rows.Close()

	if rows.Next() {
		f := new(Face)
		f.Id = fId
		f.V128D = common.NewV128D()
		vec := make([]byte, common.V128D_SIZE)
		err := rows.Scan(&f.SceneId, &f.PersonId, &f.CapturedAt, &f.ImageId, &f.Rect.LeftTop.Y, &f.Rect.LeftTop.X, &f.Rect.RightBottom.Y, &f.Rect.RightBottom.X, &f.FaceImageId, &vec)
		if err != nil {
			mpp.logger.Warn("GetFaceById(): could not scan result err=", err)
			return nil, err
		}
		f.V128D.Assign(vec)
		return f, nil
	}
	return nil, nil
}

func (mpp *msql_part_tx) FindFaces(fQuery *FacesQuery) ([]*Face, error) {
	mpp.logger.Debug("FindFaces: Requesting faces by ", fQuery)
	var q string
	if fQuery.Short {
		q = "SELECT id, scene_id, person_id, captured_at, image_id, img_top, img_left, img_bottom, img_right, face_image_id FROM face "
	} else {
		q = "SELECT id, scene_id, person_id, captured_at, image_id, img_top, img_left, img_bottom, img_right, face_image_id, v128d FROM face "
	}

	whereParams := []interface{}{}
	if fQuery.PersonIds != nil {
		if len(fQuery.PersonIds) == 0 {
			mpp.logger.Debug("FindFaces: empty, but not nil fQuery.PersonIds, returns empty result ")
			return []*Face{}, nil
		}
		q += "WHERE person_id IN("
		for i, pid := range fQuery.PersonIds {
			if i > 0 {
				q += ", ?"
			} else {
				q += "?"
			}
			whereParams = append(whereParams, pid)
		}
		q += ")"
	}

	q = q + " ORDER BY captured_at DESC"
	if fQuery.Limit > 0 {
		q = q + " LIMIT " + strconv.Itoa(fQuery.Limit)
	}

	mpp.logger.Debug("FindFaces: q=", q, " ", whereParams)
	rows, err := mpp.executor().Query(q, whereParams...)
	if err != nil {
		mpp.logger.Warn("FindFaces(): could read faces by query=", fQuery, ", err=", err)
		return nil, err
	}
	defer rows.Close()

	res := make([]*Face, 0, 10)
	for rows.Next() {
		f := new(Face)
		if fQuery.Short {
			err := rows.Scan(&f.Id, &f.SceneId, &f.PersonId, &f.CapturedAt, &f.ImageId, &f.Rect.LeftTop.Y, &f.Rect.LeftTop.X, &f.Rect.RightBottom.Y, &f.Rect.RightBottom.X, &f.FaceImageId)
			if err != nil {
				mpp.logger.Warn("FindFaces(): could not scan short result err=", err)
				return nil, err
			}
		} else {
			f.V128D = common.NewV128D()
			vec := make([]byte, common.V128D_SIZE)
			err := rows.Scan(&f.Id, &f.SceneId, &f.PersonId, &f.CapturedAt, &f.ImageId, &f.Rect.LeftTop.Y, &f.Rect.LeftTop.X, &f.Rect.RightBottom.Y, &f.Rect.RightBottom.X, &f.FaceImageId, &vec)
			if err != nil {
				mpp.logger.Warn("FindFaces(): could not scan full result err=", err)
				return nil, err
			}
			f.V128D.Assign(vec)
		}
		res = append(res, f)
	}
	return res, nil
}

func (mpp *msql_part_tx) GetPersonById(pId string) (*Person, error) {
	rows, err := mpp.executor().Query("SELECT cam_id, last_seen, profile_id, picture_id, match_group FROM person WHERE id=?", pId)
	if err != nil {
		mpp.logger.Warn("GetPersonById(): could read by id=", pId, ", err=", err)
		return nil, err
	}
	defer rows.Close()

	if rows.Next() {
		p := new(Person)
		p.Id = pId
		err := rows.Scan(&p.CamId, &p.LastSeenAt, &p.ProfileId, &p.PictureId, &p.MatchGroup)
		if err != nil {
			mpp.logger.Warn("GetPersonById(): could not scan result err=", err)
		}
		return p, nil
	}
	return nil, common.NewError(common.ERR_NOT_FOUND, "Could not find person by id="+pId)
}

func (mpp *msql_part_tx) CheckPersonInOrg(pId string, orgId int64) (bool, error) {
	rows, err := mpp.executor().Query("SELECT p.id FROM person AS p WHERE p.id=? AND (SELECT id FROM camera WHERE org_id=? AND id=p.cam_id) IS NOT NULL", pId, orgId)
	if err != nil {
		mpp.logger.Warn("CheckPersonInOrg(): err=", err)
		return false, err
	}
	defer rows.Close()
	return rows.Next(), nil
}

func (mpp *msql_part_tx) FindPersons(pQuery *PersonsQuery) ([]*Person, error) {
	mpp.logger.Debug("FindPersons: Requesting faces by ", pQuery)
	q := "SELECT id, cam_id, last_seen, profile_id, picture_id, match_group FROM person "
	whereCond := []string{}
	whereParams := []interface{}{}

	if pQuery.CamId != "" {
		whereCond = append(whereCond, "cam_id=?")
		whereParams = append(whereParams, pQuery.CamId)
	}

	if pQuery.MaxLastSeenAt != common.TIMESTAMP_NA {
		whereCond = append(whereCond, "last_seen <= ?")
		whereParams = append(whereParams, int64(pQuery.MaxLastSeenAt))
	}

	if pQuery.PersonIds != nil {
		if len(pQuery.PersonIds) == 0 {
			mpp.logger.Debug("FindPersons: empty, but not nil pQuery.PersonIds, returns empty result ")
			return []*Person{}, nil
		}
		inc := "id IN("
		for i, pid := range pQuery.PersonIds {
			if i > 0 {
				inc += ", ?"
			} else {
				inc += "?"
			}
			whereParams = append(whereParams, pid)
		}
		inc += ")"
		whereCond = append(whereCond, inc)
	}

	if len(whereCond) > 0 {
		q = q + " WHERE "
		for i, w := range whereCond {
			if i > 0 {
				q = q + " AND " + w
			} else {
				q = q + w
			}
		}
	}

	q = q + " ORDER BY last_seen DESC"
	if pQuery.Limit > 0 {
		q = q + " LIMIT " + strconv.Itoa(pQuery.Limit)
	}

	mpp.logger.Debug("FindPersons(): q=", q, whereParams)
	rows, err := mpp.executor().Query(q, whereParams...)
	if err != nil {
		mpp.logger.Warn("FindPersons(): could read faces by query=", pQuery, ", err=", err)
		return nil, err
	}
	defer rows.Close()

	res := make([]*Person, 0, 10)
	for rows.Next() {
		p := new(Person)
		err := rows.Scan(&p.Id, &p.CamId, &p.LastSeenAt, &p.ProfileId, &p.PictureId, &p.MatchGroup)
		if err != nil {
			mpp.logger.Warn("FindPersons(): could not scan result err=", err)
			return nil, err
		}
		res = append(res, p)
	}
	return res, nil

}

func (mpp *msql_part_tx) InsertPerson(p *Person) error {
	_, err := mpp.executor().Exec("INSERT INTO person(id, cam_id, last_seen, profile_id, picture_id, match_group) VALUES (?,?,?,?,?,?)",
		p.Id, p.CamId, p.LastSeenAt, p.ProfileId, p.PictureId, p.MatchGroup)
	if err != nil {
		mpp.logger.Warn("InsertPerson(): Could not insert new person ", p, ", got the err=", err)
		return err
	}

	return nil
}

func (mpp *msql_part_tx) InsertPersons(persons []*Person) error {
	mpp.logger.Debug("Inserting ", len(persons), " persons to DB: ", persons)
	if len(persons) > 0 {
		q := "INSERT INTO person(id, cam_id, last_seen, profile_id, picture_id, match_group) VALUES "
		vals := []interface{}{}
		for i, p := range persons {
			if i > 0 {
				q = q + ", "
			}
			q = q + "(?,?,?,?,?,?)"
			vals = append(vals, p.Id, p.CamId, p.LastSeenAt, p.ProfileId, p.PictureId, p.MatchGroup)
		}

		_, err := mpp.executor().Exec(q, vals...)
		if err != nil {
			mpp.logger.Warn("Could not execute insert query ", q, ", err=", err)
			return err
		}
	}
	return nil

}

func (mpp *msql_part_tx) UpdatePerson(p *Person) error {
	_, err := mpp.executor().Exec("UPDATE person SET last_seen=?, profile_id=?, picture_id=?, match_group=?",
		p.LastSeenAt, p.ProfileId, p.PictureId, p.MatchGroup)
	if err != nil {
		mpp.logger.Warn("UpdatePerson(): Could not update person ", p, ", got the err=", err)
		return err
	}
	return nil
}

func (mpp *msql_part_tx) UpdatePersonsLastSeenAt(pids []string, lastSeenAt uint64) error {
	if pids == nil || len(pids) == 0 {
		return nil
	}

	q := "UPDATE person SET last_seen=? WHERE id IN ("
	args := make([]interface{}, len(pids)+1)
	args[0] = lastSeenAt
	for i, pid := range pids {
		if i > 0 {
			q = q + ", ?"
		} else {
			q = q + "?"
		}
		args[i+1] = pid
	}
	q = q + ")"

	mpp.logger.Debug("UpdatePersonsLastSeenAt(): q=", q, " ", args)
	_, err := mpp.executor().Exec(q, args...)
	return err
}

// =========== Field Infos
func (mpp *msql_part_tx) GetFieldInfo(fldId int64) (*FieldInfo, error) {
	rows, err := mpp.executor().Query("SELECT id, org_id, field_type, display_name FROM field_info WHERE id=?", fldId)
	if err != nil {
		mpp.logger.Warn("GetFieldInfo(): Could not select field info by fldId=", fldId, ", got the err=", err)
		return nil, err
	}
	defer rows.Close()

	if rows.Next() {
		fi := new(FieldInfo)
		rows.Scan(&fi.Id, &fi.OrgId, &fi.FieldType, &fi.DisplayName)
		return fi, nil
	}

	mpp.logger.Warn("GetFieldInfo(): No field by fldId=", fldId)
	return nil, common.NewError(common.ERR_NOT_FOUND, "No field info by fldId="+strconv.FormatInt(fldId, 10))
}

func (mpp *msql_part_tx) GetFieldInfos(orgId int64) ([]*FieldInfo, error) {
	rows, err := mpp.executor().Query("SELECT id, field_type, display_name FROM field_info WHERE org_id=?", orgId)
	if err != nil {
		mpp.logger.Warn("GetFieldInfos: Could not select field infos by orgId=", orgId, ", got the err=", err)
		return nil, err
	}
	defer rows.Close()

	res := make([]*FieldInfo, 0, 3)
	for rows.Next() {
		fi := new(FieldInfo)
		rows.Scan(&fi.Id, &fi.FieldType, &fi.DisplayName)
		fi.OrgId = orgId
		res = append(res, fi)
	}
	return res, nil
}

func (mpp *msql_part_tx) InsertFieldInfos(fldInfos []*FieldInfo) error {
	if fldInfos == nil || len(fldInfos) == 0 {
		mpp.logger.Warn("InsertFieldInfos(): got an empty list, adds nothing")
		return nil
	}
	q := "INSERT INTO field_info(org_id, field_type, display_name) VALUES "
	params := make([]interface{}, 0, len(fldInfos)*3)
	for i, fi := range fldInfos {
		if i > 0 {
			q += ", (?, ?, ?)"
		} else {
			q += " (?, ?, ?)"
		}
		params = append(params, fi.OrgId, fi.FieldType, fi.DisplayName)
	}

	mpp.logger.Debug("InsertFieldInfos(): Inserting q=", q, ", params=", params)
	_, err := mpp.executor().Exec(q, params...)
	if err != nil {
		mpp.logger.Warn("InsertFieldInfos(): Could not insert new fieldInfos ", fldInfos, ", got the err=", err)
		return err
	}

	return nil
}

func (mpp *msql_part_tx) UpdateFiledInfo(fldInfo *FieldInfo) error {
	_, err := mpp.executor().Exec("UPDATE field_info SET display_name=? WHERE id=?", fldInfo.DisplayName, fldInfo.Id)
	if err != nil {
		mpp.logger.Warn("UpdateFiledInfo(): Could not update fieldInfo ", fldInfo, ", got the err=", err)
		return err
	}
	return nil
}

func (mpp *msql_part_tx) DeleteFieldInfo(fldInfo *FieldInfo) error {
	_, err := mpp.executor().Exec("DELETE FROM field_info WHERE id=?", fldInfo.Id)
	if err != nil {
		mpp.logger.Warn("DeleteFieldInfo(): Could not delete fieldInfo ", fldInfo, ", got the err=", err)
	}
	return err
}

func (mpp *msql_part_tx) InsertProfile(prf *Profile) (int64, error) {
	res, err := mpp.executor().Exec("INSERT INTO profile(org_id, picture_id) VALUES (?,?)",
		prf.OrgId, prf.PictureId)
	if err != nil {
		mpp.logger.Warn("InsertProfile: Could not insert new profile ", prf, ", got the err=", err)
		return -1, err
	}

	return res.LastInsertId()
}

func (mpp *msql_part_tx) InsertProfleMetas(pms []*ProfileMeta) error {
	if pms == nil || len(pms) == 0 {
		return nil
	}

	q := "INSERT INTO profile_meta(profile_id, field_id, value) VALUES "
	vals := []interface{}{}
	for i, pm := range pms {
		if i > 0 {
			q = q + ", "
		}
		q = q + "(?,?,?)"
		vals = append(vals, pm.ProfileId, pm.FieldId, pm.Value)
	}
	_, err := mpp.executor().Exec(q, vals...)
	if err != nil {
		mpp.logger.Warn("InsertProfleMetas: Could not insert new profile metas, got the err=", err)
		return err
	}
	return err
}

func (mpp *msql_part_tx) GetProfileMetas(prfIds []int64) ([]*ProfileMeta, error) {
	mpp.logger.Debug("GetProfileMetas(): Requesting profiles ", prfIds)
	if prfIds == nil || len(prfIds) == 0 {
		mpp.logger.Warn("GetProfileMetas(): Empty profiles list, returns nothing. ")
	}
	var q = "SELECT (SELECT display_name FROM field_info WHERE id=pm.field_id), pm.field_id, pm.value, pm.profile_id FROM profile_meta AS pm WHERE pm.profile_id IN ("

	whereParams := []interface{}{}
	for i, pid := range prfIds {
		if i > 0 {
			q += ", ?"
		} else {
			q += "?"
		}
		whereParams = append(whereParams, pid)
	}

	q += ")"
	mpp.logger.Debug("GetProfileMetas(): q=", q, " ", whereParams)
	rows, err := mpp.executor().Query(q, whereParams...)
	if err != nil {
		mpp.logger.Warn("GetProfileMetas(): could read profiles by query=", q, ", err=", err)
		return nil, err
	}
	defer rows.Close()

	res := make([]*ProfileMeta, 0, 2)
	for rows.Next() {
		pm := new(ProfileMeta)
		err := rows.Scan(&pm.DisplayName, &pm.FieldId, &pm.Value, &pm.ProfileId)
		if err != nil {
			return nil, err
		}
		res = append(res, pm)
	}
	return res, nil
}

func (mpp *msql_part_tx) CheckProfileInOrg(prId, orgId int64) (bool, error) {
	rows, err := mpp.executor().Query("SELECT p.id FROM profile AS p WHERE p.id=? AND p.org_id=?", prId, orgId)
	if err != nil {
		mpp.logger.Warn("CheckProfileInOrg(): err=", err)
		return false, err
	}
	defer rows.Close()
	return rows.Next(), nil
}

func (mpp *msql_part_tx) DeleteProfile(prfId int64) error {
	mpp.logger.Debug("DeleteProfile(): deleting profile prfId=", prfId)
	_, err := mpp.executor().Exec("DELETE FROM profile WHERE id=?", prfId)
	return err

}

func (mpp *msql_part_tx) DeleteAllProfileMetas(prfId int64) error {
	mpp.logger.Debug("DeleteAllProfileMetas(): deleting all profile metas by prfId=", prfId)
	_, err := mpp.executor().Exec("DELETE FROM profile_meta WHERE profile_id=?", prfId)
	return err
}

func (mpp *msql_part_tx) UpdateProfile(prf *Profile) error {
	mpp.logger.Debug("UpdateProfile(): updating profile ", prf)
	_, err := mpp.executor().Exec("UPDATE profile WHERE id=? SET PictureId=?", prf.Id, prf.PictureId)
	return err
}

func (mpp *msql_part_tx) GetProfiles(prQuery *ProfileQuery) ([]*Profile, error) {
	mpp.logger.Debug("GetProfiles: Requesting profiles by ", prQuery)
	q := "SELECT p.id, p.org_id, p.picture_id FROM profile AS p "
	where := false

	whereParams := []interface{}{}
	if prQuery.ProfileIds != nil {
		if len(prQuery.ProfileIds) == 0 {
			mpp.logger.Debug("GetProfiles: empty, but not nil prQuery.ProfileIds, returns empty result ")
			return []*Profile{}, nil
		}

		if where {
			q += " AND "
		} else {
			q += " WHERE "
		}
		where = true
		q += "p.id IN("
		for i, pid := range prQuery.ProfileIds {
			if i > 0 {
				q += ", ?"
			} else {
				q += "?"
			}
			whereParams = append(whereParams, pid)
		}
		q += ")"
	}

	mpp.logger.Debug("GetProfiles: q=", q, " ", whereParams)
	rows, err := mpp.executor().Query(q, whereParams...)
	if err != nil {
		mpp.logger.Warn("GetProfiles(): could read profiles by query=", prQuery, ", err=", err)
		return nil, err
	}
	defer rows.Close()

	res := make([]*Profile, 0, len(prQuery.ProfileIds))
	pMap := make(map[int64]*Profile)
	for rows.Next() {
		p := new(Profile)
		err := rows.Scan(&p.Id, &p.OrgId, &p.PictureId)
		if err != nil {
			mpp.logger.Warn("GetProfiles(): could not scan result (No Meta) err=", err)
			return nil, err
		}
		res = append(res, p)
		pMap[p.Id] = p
	}

	if prQuery.NoMeta {
		return res, nil
	}
	pms, err := mpp.GetProfileMetas(prQuery.ProfileIds)
	if err != nil {
		mpp.logger.Warn("GetProfiles(): could not read profile Metas err=", err)
		return nil, err
	}

	for _, pm := range pms {
		pr, ok := pMap[pm.ProfileId]
		if !ok {
			continue
		}
		if pr.Meta == nil {
			pr.Meta = make([]*ProfileMeta, 0, 2)
		}
		pr.Meta = append(pr.Meta, pm)
	}

	return res, nil
}

func (mpp *msql_part_tx) GetProfilesByMGs(matchGroups []int64) (map[int64]int64, error) {
	// pid -> mg
	prfMap := make(map[int64]int64)
	if matchGroups == nil || len(matchGroups) == 0 {
		return prfMap, nil
	}

	q := "SELECT profile_id, match_group FROM person WHERE profile_id > 0 AND match_group IN("
	whereParams := []interface{}{}
	for i, mg := range matchGroups {
		if i > 0 {
			q += ", ?"
		} else {
			q += "?"
		}
		whereParams = append(whereParams, mg)
	}
	q += ")"

	mpp.logger.Debug("GetProfilesByMGs: q=", q, " ", whereParams)
	rows, err := mpp.executor().Query(q, whereParams...)
	if err != nil {
		mpp.logger.Warn("GetProfilesByMGs(): could read profiles by match groups, err=", err)
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var pid, mg int64
		err := rows.Scan(&pid, &mg)
		if err != nil {
			mpp.logger.Warn("GetProfilesByMGs(): could not scan result err=", err)
			return nil, err
		}
		prfMap[pid] = mg
	}

	return prfMap, nil
}
