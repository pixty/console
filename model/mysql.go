package model

import (
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
		Config   *common.ConsoleConfig `inject:""`
		logger   log4g.Logger
		mainPers *msql_main_persister
		// same like main one, so far...
		partPers *msql_part_persister
	}

	msql_connection struct {
		lock    sync.Mutex
		logger  log4g.Logger
		conName string
		ds      string
		db      *sql.DB
	}

	msql_main_persister struct {
		logger log4g.Logger
		dbc    *msql_connection
	}

	msql_part_persister struct {
		logger log4g.Logger
		dbc    *msql_connection
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
	mc := mp.newConnection(mp.Config.MysqlDatasource, log4g.GetLogger("pixty.mysql.main"))
	mp.mainPers = &msql_main_persister{mc.logger, mc}
	pc := mp.newConnection(mp.Config.MysqlDatasource, log4g.GetLogger("pixty.mysql.part"))
	mp.partPers = &msql_part_persister{pc.logger, pc}
	return nil
}

func (mp *MysqlPersister) DiShutdown() {
	mp.logger.Info("Shutting down.")
}

// ------------------------------ Persister ----------------------------------
func (mp *MysqlPersister) GetMainPersister() MainPersister {
	return mp.mainPers
}

func (mp *MysqlPersister) GetPartPersister(partId string) PartPersister {
	// let's use one instance of part persister so far
	return mp.partPers
}

// -------------------------------- Misc -------------------------------------
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

func (mc *msql_connection) exec(sqlQuery string, params ...interface{}) error {
	db, err := mc.getDb()
	if err != nil {
		mc.logger.Warn("exec(): Could not get DB err=", err)
		return err
	}

	_, err = db.Exec(sqlQuery, params...)
	return err
}

func (mc *msql_connection) execScript(sqlScript string) error {
	file, err := ioutil.ReadFile(sqlScript)

	if err != nil {
		mc.logger.Warn("Could not find/read script file ", sqlScript, ", err=", err)
		return err
	}

	requests := strings.Split(string(file), ";")

	for _, request := range requests {
		if strings.Trim(request, " ") == "" {
			continue
		}
		err := mc.exec(request)
		if err != nil {
			mc.logger.Warn("Ooops, error while executing the following statement request=", request, ", err=", err)
			return err
		}
	}
	return nil
}

// ========================= msql_main_persister =============================
func (mmp *msql_main_persister) ExecQuery(sqlQuery string, params ...interface{}) error {
	return mmp.dbc.exec(sqlQuery)
}

func (mmp *msql_main_persister) ExecScript(pathToFile string) error {
	return mmp.dbc.execScript(pathToFile)
}

func (mmp *msql_main_persister) FindCameraById(camId string) (*Camera, error) {
	db, err := mmp.dbc.getDb()
	if err != nil {
		mmp.logger.Warn("FindCameraById(): Could not get DB err=", err)
		return nil, err
	}

	rows, err := db.Query("SELECT org_id, secret_key FROM camera WHERE id=?", camId)
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
			mmp.logger.Warn("FindCameraByAccessKey() could not scan result err=", err)
		}
		return cam, nil
	}
	return nil, nil
}

func (mmp *msql_main_persister) InsertOrg(org *Organization) (int64, error) {
	db, err := mmp.dbc.getDb()
	if err != nil {
		mmp.logger.Warn("InsertOrg(): Could not get DB err=", err)
		return -1, err
	}

	res, err := db.Exec("INSERT INTO organization(name) VALUES (?)", org.Name)
	if err != nil {
		mmp.logger.Warn("InsertOrg: Could not insert new organization ", org, ", got the err=", err)
		return -1, err
	}

	return res.LastInsertId()
}

func (mmp *msql_main_persister) GetOrg(orgId int64) (*Organization, error) {
	db, err := mmp.dbc.getDb()
	if err != nil {
		mmp.logger.Warn("GetOrg(): Could not get DB err=", err)
		return nil, err
	}

	rows, err := db.Query("SELECT name FROM organization WHERE id=?", orgId)
	if err != nil {
		mmp.logger.Warn("GetOrg: Could not get organization by orgId=", orgId, ", got the err=", err)
		return nil, err
	}
	defer rows.Close()
	if rows.Next() {
		org := new(Organization)
		org.Id = orgId
		rows.Scan(&org.Name)
		return org, nil
	}

	mmp.logger.Warn("GetOrg: No organization by orgId=", orgId)
	return nil, common.NewError(common.ERR_NOT_FOUND, "No org for orgId="+strconv.FormatInt(orgId, 10))
}

func (mmp *msql_main_persister) GetFieldInfo(fldId int64) (*FieldInfo, error) {
	db, err := mmp.dbc.getDb()
	if err != nil {
		mmp.logger.Warn("GetFieldInfo(): Could not get DB err=", err)
		return nil, err
	}

	rows, err := db.Query("SELECT id, org_id, field_type, display_name FROM field_info WHERE id=?", fldId)
	if err != nil {
		mmp.logger.Warn("GetFieldInfo(): Could not select field info by fldId=", fldId, ", got the err=", err)
		return nil, err
	}
	defer rows.Close()

	if rows.Next() {
		fi := new(FieldInfo)
		rows.Scan(&fi.Id, &fi.OrgId, &fi.FieldType, &fi.DisplayName)
		return fi, nil
	}

	mmp.logger.Warn("GetOrg: No field by fldId=", fldId)
	return nil, common.NewError(common.ERR_NOT_FOUND, "No field info by fldId="+strconv.FormatInt(fldId, 10))
}

func (mmp *msql_main_persister) GetFieldInfos(orgId int64) ([]*FieldInfo, error) {
	db, err := mmp.dbc.getDb()
	if err != nil {
		mmp.logger.Warn("GetFieldInfos(): Could not get DB err=", err)
		return nil, err
	}

	rows, err := db.Query("SELECT id, field_type, display_name FROM field_info WHERE org_id=?", orgId)
	if err != nil {
		mmp.logger.Warn("GetFieldInfos: Could not select field infos by orgId=", orgId, ", got the err=", err)
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

func (mmp *msql_main_persister) InsertFieldInfos(fldInfos []*FieldInfo) error {
	if fldInfos == nil || len(fldInfos) == 0 {
		mmp.logger.Warn("InsertFieldInfos(): got an empty list, adds nothing")
		return nil
	}
	db, err := mmp.dbc.getDb()
	if err != nil {
		mmp.logger.Warn("InsertFieldInfos(): Could not get DB err=", err)
		return err
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

	mmp.logger.Debug("InsertFieldInfos: Inserting q=", q, ", params=", params)
	_, err = db.Exec(q, params...)
	if err != nil {
		mmp.logger.Warn("InsertFieldInfos: Could not insert new fieldInfos ", fldInfos, ", got the err=", err)
		return err
	}

	return nil
}

func (mmp *msql_main_persister) UpdateFiledInfo(fldInfo *FieldInfo) error {
	db, err := mmp.dbc.getDb()
	if err != nil {
		mmp.logger.Warn("UpdateFiledInfo(): Could not get DB err=", err)
		return err
	}

	_, err = db.Exec("UPDATE field_info SET display_name=? WHERE id=?", fldInfo.DisplayName, fldInfo.Id)
	if err != nil {
		mmp.logger.Warn("UpdateFiledInfo: Could not update fieldInfo ", fldInfo, ", got the err=", err)
		return err
	}
	return nil
}

func (mmp *msql_main_persister) DeleteFieldInfo(fldInfo *FieldInfo) error {
	db, err := mmp.dbc.getDb()
	if err != nil {
		mmp.logger.Warn("DeleteFieldInfo(): Could not get DB err=", err)
		return err
	}

	_, err = db.Exec("DELETE FROM field_info WHERE id=?", fldInfo.Id)
	if err != nil {
		mmp.logger.Warn("DeleteFieldInfo: Could not delete fieldInfo ", fldInfo, ", got the err=", err)
		return err
	}
	return nil
}

// ========================= msql_part_persister =============================
func (mpp *msql_part_persister) ExecQuery(sqlQuery string, params ...interface{}) error {
	return mpp.dbc.exec(sqlQuery)
}

func (mpp *msql_part_persister) ExecScript(pathToFile string) error {
	return mpp.dbc.execScript(pathToFile)
}

func (mpp *msql_part_persister) InsertFace(f *Face) (int64, error) {
	db, err := mpp.dbc.getDb()
	if err != nil {
		mpp.logger.Warn("InsertFace(): Could not get DB err=", err)
		return -1, err
	}

	res, err := db.Exec("INSERT INTO face(scene_id, person_id, captured_at, image_id, img_top, img_left, img_bottom, img_right, face_image_id, v128d) VALUES (?,?,?,?,?,?,?,?,?,?)",
		f.SceneId, f.PersonId, f.CapturedAt, f.ImageId, f.Rect.LeftTop.Y, f.Rect.LeftTop.X, f.Rect.RightBottom.Y, f.Rect.RightBottom.X, f.FaceImageId, f.V128D.ToByteSlice())
	if err != nil {
		mpp.logger.Warn("Could not insert new face ", f, ", got the err=", err)
		return -1, err
	}

	return res.LastInsertId()
}

func (mpp *msql_part_persister) InsertFaces(faces []*Face) error {
	mpp.logger.Debug("Inserting ", len(faces), " faces to DB: ", faces)
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

		db, err := mpp.dbc.getDb()
		if err != nil {
			mpp.logger.Warn("InsertFaces(): Could not get DB err=", err)
			return err
		}

		_, err = db.Exec(q, vals...)
		if err != nil {
			mpp.logger.Warn("Could not execute insert query ", q, ", err=", err)
			return err
		}
	}
	return nil
}

func (mpp *msql_part_persister) GetFaceById(fId int64) (*Face, error) {
	db, err := mpp.dbc.getDb()
	if err != nil {
		mpp.logger.Warn("GetFaceById(): Could not get DB err=", err)
		return nil, err
	}

	rows, err := db.Query("SELECT scene_id, person_id, captured_at, image_id, img_top, img_left, img_bottom, img_right, face_image_id, v128d FROM face WHERE id=?", fId)
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

func (mpp *msql_part_persister) FindFaces(fQuery *FacesQuery) ([]*Face, error) {
	db, err := mpp.dbc.getDb()
	if err != nil {
		mpp.logger.Warn("FindFaces(): Could not get DB err=", err)
		return nil, err
	}

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
	rows, err := db.Query(q, whereParams...)
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

func (mpp *msql_part_persister) GetPersonById(pId string) (*Person, error) {
	db, err := mpp.dbc.getDb()
	if err != nil {
		mpp.logger.Warn("GetPersonById(): Could not get DB err=", err)
		return nil, err
	}
	rows, err := db.Query("SELECT cam_id, last_seen, profile_id, picture_id, match_group FROM person WHERE id=?", pId)
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
	return nil, nil
}

func (mpp *msql_part_persister) FindPersons(pQuery *PersonsQuery) ([]*Person, error) {
	db, err := mpp.dbc.getDb()
	if err != nil {
		mpp.logger.Warn("FindPersons(): Could not get DB err=", err)
		return nil, err
	}

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

	mpp.logger.Debug("FindPersons: q=", q, whereParams)
	rows, err := db.Query(q, whereParams...)
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

func (mpp *msql_part_persister) InsertPerson(p *Person) error {
	db, err := mpp.dbc.getDb()
	if err != nil {
		mpp.logger.Warn("InsertPerson(): Could not get DB err=", err)
		return err
	}
	_, err = db.Exec("INSERT INTO person(id, cam_id, last_seen, profile_id, picture_id, match_group) VALUES (?,?,?,?,?,?)",
		p.Id, p.CamId, p.LastSeenAt, p.ProfileId, p.PictureId, p.MatchGroup)
	if err != nil {
		mpp.logger.Warn("InsertPerson(): Could not insert new person ", p, ", got the err=", err)
		return err
	}

	return nil
}

func (mpp *msql_part_persister) InsertPersons(persons []*Person) error {
	mpp.logger.Debug("Inserting ", len(persons), " persons to DB: ", persons)
	if len(persons) > 0 {
		q := "INSERT INTO person(id, cam_id, last_seen, profile_id, picture_id, match_group) VALUES "
		vals := []interface{}{}
		for i, p := range persons {
			if i > 0 {
				q = q + ", "
			}
			q = q + "(?,?,?,?,?,?)"
			mpp.logger.Info("p=", p)
			vals = append(vals, p.Id, p.CamId, p.LastSeenAt, p.ProfileId, p.PictureId, p.MatchGroup)
		}

		db, err := mpp.dbc.getDb()
		if err != nil {
			mpp.logger.Warn("InsertPersons(): Could not get DB err=", err)
			return err
		}

		_, err = db.Exec(q, vals...)
		if err != nil {
			mpp.logger.Warn("Could not execute insert query ", q, ", err=", err)
			return err
		}
	}
	return nil

}

func (mpp *msql_part_persister) UpdatePerson(p *Person) error {
	db, err := mpp.dbc.getDb()
	if err != nil {
		mpp.logger.Warn("UpdatePerson(): Could not get DB err=", err)
		return err
	}

	_, err = db.Exec("UPDATE person SET last_seen=?, profile_id=?, picture_id=?, match_group=?",
		p.LastSeenAt, p.ProfileId, p.PictureId, p.MatchGroup)
	if err != nil {
		mpp.logger.Warn("UpdatePerson(): Could not update person ", p, ", got the err=", err)
		return err
	}
	return nil
}

func (mpp *msql_part_persister) UpdatePersonsLastSeenAt(pids []string, lastSeenAt uint64) error {
	db, err := mpp.dbc.getDb()
	if err != nil {
		mpp.logger.Warn("UpdatePersonsLastSeenAt(): Could not get DB err=", err)
		return err
	}
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
	_, err = db.Exec(q, args...)
	return err
}

func (mpp *msql_part_persister) GetProfiles(prQuery *ProfileQuery) ([]*Profile, error) {
	db, err := mpp.dbc.getDb()
	if err != nil {
		mpp.logger.Warn("GetProfiles(): Could not get DB err=", err)
		return nil, err
	}

	mpp.logger.Debug("GetProfiles: Requesting profiles by ", prQuery)
	var q string
	where := false
	if prQuery.NoMeta {
		q = "SELECT p.id, p.org_id, p.picture_id FROM profile AS p "
	} else {
		q = "SELECT (SELECT display_name FROM field_info WHERE org_id=p.org_id AND id=pm.field_id), pm.field_id, pm.value, p.id, p.org_id, p.picture_id FROM profile AS p, profile_meta AS pm WHERE p.id=pm.profile_id "
		where = true
	}

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
	rows, err := db.Query(q, whereParams...)
	if err != nil {
		mpp.logger.Warn("GetProfiles(): could read profiles by query=", prQuery, ", err=", err)
		return nil, err
	}
	defer rows.Close()

	if prQuery.NoMeta {
		res := make([]*Profile, 0, len(prQuery.ProfileIds))
		for rows.Next() {
			p := new(Profile)
			err := rows.Scan(&p.Id, &p.OrgId, &p.PictureId)
			if err != nil {
				mpp.logger.Warn("GetProfiles(): could not scan result (No Meta) err=", err)
				return nil, err
			}
			res = append(res, p)
		}
		return res, nil
	}

	pMap := make(map[int64]*Profile)
	for rows.Next() {
		var fn, v, picId string
		var pid, fldId, orgId int64
		err := rows.Scan(&fn, &fldId, &v, &pid, &orgId, &picId)
		if err != nil {
			mpp.logger.Warn("GetProfiles(): could not scan result(with Meta!) err=", err)
			return nil, err
		}

		p, ok := pMap[pid]
		if !ok {
			p = new(Profile)
			p.Id = pid
			p.PictureId = picId
			p.OrgId = orgId
			p.Meta = make([]*ProfileMeta, 0, 2)
			pMap[pid] = p
		}
		pm := new(ProfileMeta)
		pm.FieldId = fldId
		pm.Value = v
		pm.ProfileId = pid
		pm.DisplayName = fn
	}

	res := make([]*Profile, 0, len(pMap))
	for _, p := range pMap {
		res = append(res, p)
	}
	return res, nil
}

func (mpp *msql_part_persister) GetProfilesByMGs(matchGroups []int64) (map[int64]int64, error) {
	db, err := mpp.dbc.getDb()
	if err != nil {
		mpp.logger.Warn("GetProfilesByMGs(): Could not get DB err=", err)
		return nil, err
	}

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
	rows, err := db.Query(q, whereParams...)
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
