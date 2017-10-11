package model

import (
	"context"
	"database/sql"
	"io/ioutil"
	"math"
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
		mtx.logger.Debug("Commit() done. err=", err)
	}
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
func (mmp *msql_main_tx) InsertOrg(org *Organization) (int64, error) {
	res, err := mmp.executor().Exec("INSERT INTO organization(name) VALUES (?)", org.Name)
	if err != nil {
		mmp.logger.Warn("InsertOrg(): Could not insert new organization ", org, ", got the err=", err)
		return -1, err
	}

	return res.LastInsertId()
}

func (mmp *msql_main_tx) GetOrgById(orgId int64) (*Organization, error) {
	rows, err := mmp.executor().Query("SELECT name FROM organization WHERE id=?", orgId)
	if err != nil {
		mmp.logger.Warn("GetOrgById(): Could not get organization by orgId=", orgId, ", got the err=", err)
		return nil, err
	}
	defer rows.Close()
	if rows.Next() {
		org := new(Organization)
		org.Id = orgId
		rows.Scan(&org.Name)
		return org, nil
	}

	mmp.logger.Warn("GetOrgById(): No organization by orgId=", orgId)
	return nil, common.NewError(common.ERR_NOT_FOUND, "No org for orgId="+strconv.FormatInt(orgId, 10))
}

func (mmp *msql_main_tx) FindOrgs(q *OrgQuery) ([]*Organization, error) {
	if q.OrgIds == nil || len(q.OrgIds) == 0 {
		return []*Organization{}, nil
	}
	query := "SELECT id, name FROM organization WHERE id IN ("
	params := []interface{}{}
	for i, oid := range q.OrgIds {
		if i > 0 {
			query += ", ?"
		} else {
			query += "?"
		}
		params = append(params, oid)
	}
	query += ")"

	mmp.logger.Debug("FindOrgs(): query=", query, ", params=", params)
	rows, err := mmp.executor().Query(query, params...)
	if err != nil {
		mmp.logger.Warn("FindOrgs(): Could not get organizations by query=", query, " params=", q.OrgIds, ", got the err=", err)
		return nil, err
	}
	defer rows.Close()
	res := make([]*Organization, 0, 1)
	for rows.Next() {
		org := new(Organization)
		rows.Scan(&org.Id, &org.Name)
		res = append(res, org)
	}

	return res, nil
}

func (mmp *msql_main_tx) InsertUser(user *User) error {
	_, err := mmp.executor().Exec("INSERT INTO user(login, email, salt, hash) VALUES (?,?,?,?)",
		user.Login, user.Email, user.Salt, user.Hash)
	if err != nil {
		mmp.logger.Warn("InsertUser(): Could not insert new user ", user, ", got the err=", err)
	}
	return err
}

func (mmp *msql_main_tx) GetUserByLogin(login string) (*User, error) {
	rows, err := mmp.executor().Query("SELECT login, email, salt, hash FROM user WHERE login=?", login)
	if err != nil {
		mmp.logger.Warn("GetUserByLogin(): Could not get user by login=", login, ", got the err=", err)
		return nil, err
	}
	defer rows.Close()
	if rows.Next() {
		user := new(User)
		rows.Scan(&user.Login, &user.Email, &user.Salt, &user.Hash)
		return user, nil
	}

	mmp.logger.Debug("GetUserByLogin(): No user with login=", login)
	return nil, common.NewError(common.ERR_NOT_FOUND, "No user with login="+login)
}

func (mmp *msql_main_tx) UpdateUser(user *User) error {
	_, err := mmp.executor().Exec("UPDATE user SET email=?, hash=? WHERE login=?",
		user.Email, user.Hash, user.Login)
	if err != nil {
		mmp.logger.Warn("UpdateUser(): Could not update user ", user, ", got the err=", err)
	}
	return nil
}

func (mmp *msql_main_tx) InsertUserRoles(urs []*UserRole) error {
	if urs == nil || len(urs) == 0 {
		return nil
	}

	q := "INSERT INTO user_role(login, org_id, role) VALUES "
	params := make([]interface{}, 0, len(urs)*3)
	for i, ur := range urs {
		if i > 0 {
			q += ", (?,?,?)"
		} else {
			q += " (?,?,?)"
		}
		params = append(params, ur.Login, ur.OrgId, ur.Role)
	}

	mmp.logger.Debug("Runing q=", q, " params=", params)
	_, err := mmp.executor().Exec(q, params...)
	if err != nil {
		mmp.logger.Warn("InsertUserRoles(): Could not insert new user roles ", urs, ", got the err=", err)
	}
	return err
}

func (q *UserRoleQuery) getWhereCondition() (string, []interface{}) {
	var query string
	params := []interface{}{}

	if q.Login != "" {
		query += " WHERE login=?"
		params = append(params, q.Login)
	}

	if q.OrgId > 0 {
		if len(params) > 0 {
			query += " AND org_id=?"
		} else {
			query += " WHERE org_id=?"
		}
		params = append(params, q.OrgId)
	}
	return query, params
}

func (mmp *msql_main_tx) DeleteUserRoles(q *UserRoleQuery) error {
	where, params := q.getWhereCondition()
	query := "DELETE FROM user_role " + where

	mmp.logger.Debug("DeleteUserRoles(): query=", query, " params=", params)
	_, err := mmp.executor().Exec(query, params...)
	if err != nil {
		mmp.logger.Warn("DeleteUserRoles(): Could not delet user roles for q=", q, ", got the err=", err)
	}
	return err
}

func (mmp *msql_main_tx) FindUserRoles(q *UserRoleQuery) ([]*UserRole, error) {
	where, params := q.getWhereCondition()
	query := "SELECT login, org_id, role FROM user_role " + where

	mmp.logger.Debug("FindUserRoles(): query=", query, " params=", params)
	rows, err := mmp.executor().Query(query, params...)
	if err != nil {
		mmp.logger.Warn("FindUserRoles(): Could not get user roles by user Query=", q, ", got the err=", err)
		return nil, err
	}
	defer rows.Close()

	res := []*UserRole{}
	for rows.Next() {
		ur := new(UserRole)
		rows.Scan(&ur.Login, &ur.OrgId, &ur.Role)
		res = append(res, ur)
	}
	return res, nil
}

// ========================= msql_part_persister =============================

func (mpp *msql_part_tx) InsertCamera(cam *Camera) (int64, error) {
	res, err := mpp.executor().Exec("INSERT INTO camera(name, org_id, secret_key) VALUES (?,?,?)",
		cam.Name, cam.OrgId, cam.SecretKey)
	if err != nil {
		mpp.logger.Warn("InsertCamera(): Could not insert new camera ", cam, ", got the err=", err)
		return -1, err
	}
	return res.LastInsertId()
}

func (mpp *msql_part_tx) GetCameraById(camId int64) (*Camera, error) {
	mpp.logger.Debug("GetCameraById(): Getting camera by id=", camId)
	rows, err := mpp.executor().Query("SELECT name, org_id, secret_key FROM camera WHERE id=?", camId)
	if err != nil {
		mpp.logger.Warn("GetCameraById(): Getting camera by id=", camId, ", got the err=", err)
		return nil, err
	}
	defer rows.Close()
	if rows.Next() {
		c := new(Camera)
		rows.Scan(&c.Name, &c.OrgId, &c.SecretKey)
		c.Id = camId
		return c, nil
	}
	return nil, common.NewError(common.ERR_NOT_FOUND, "Could not find camera with id="+strconv.FormatInt(camId, 10))
}

func (mpp *msql_part_tx) UpdateCamera(cam *Camera) error {
	_, err := mpp.executor().Exec("UPDATE camera SET secret_key=?, name=? WHERE id=?",
		cam.SecretKey, cam.Name, cam.Id)
	if err != nil {
		mpp.logger.Warn("UpdateCamera(): Could not update camera ", cam, ", got the err=", err)
		return err
	}
	return nil
}

func (mpp *msql_part_tx) DeleteCamera(camId int64) error {
	_, err := mpp.executor().Exec("DELETE FROM camera WHERE id=?", camId)
	if err != nil {
		mpp.logger.Warn("DeleteCamera(): Could not delete camera with id=", camId, ", got the err=", err)
		return err
	}
	return nil
}

func (mpp *msql_part_tx) FindCameras(q *CameraQuery) ([]*Camera, error) {
	rows, err := mpp.executor().Query("SELECT id, name, org_id, secret_key FROM camera WHERE org_id=?", q.OrgId)
	if err != nil {
		mpp.logger.Warn("FindCameras(): Getting cameras by query=", q, ", got the err=", err)
		return nil, err
	}
	defer rows.Close()
	res := []*Camera{}
	for rows.Next() {
		c := new(Camera)
		rows.Scan(&c.Id, &c.Name, &c.OrgId, &c.SecretKey)
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
	return nil, common.NewError(common.ERR_NOT_FOUND, "No face with id="+strconv.FormatInt(fId, 10))
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

func (mpp *msql_part_tx) DeleteFaces(fcIds []int64) error {
	if len(fcIds) == 0 {
		return nil
	}

	q := "DELETE FROM face WHERE id IN("
	whereParams := []interface{}{}
	for i, fcid := range fcIds {
		if i > 0 {
			q += ", ?"
		} else {
			q += "?"
		}
		whereParams = append(whereParams, fcid)
	}
	q += ")"

	mpp.logger.Debug("DeleteFaces(): q=", q, " ", fcIds)
	_, err := mpp.executor().Exec(q, whereParams...)
	return err
}

func (mpp *msql_part_tx) FindZeroRefPics(limit int) ([]*Picture, error) {
	rows, err := mpp.executor().Query("SELECT id, refs FROM picture WHERE refs=0 LIMIT ?", limit)
	if err != nil {
		mpp.logger.Warn("FindZeroRefPics(): coule not run the query, err=", err, ", limit=", limit)
		return nil, err
	}
	defer rows.Close()
	res := []*Picture{}
	for rows.Next() {
		p := new(Picture)
		err = rows.Scan(&p.Id, &p.Refs)
		if err != nil {
			mpp.logger.Warn("FindZeroRefPics(): coule not scan picture, err=", err)
			return nil, err
		}
		res = append(res, p)
	}
	return res, nil
}

func (mpp *msql_part_tx) DeletePics(pics []*Picture) error {
	if len(pics) == 0 {
		mpp.logger.Debug("DeletePics(): nothing to delete.")
		return nil
	}

	q := "DELETE FROM picture WHERE id IN("
	whereParams := []interface{}{}
	for i, p := range pics {
		if i > 0 {
			q += ", ?"
		} else {
			q += "?"
		}
		whereParams = append(whereParams, p.Id)
	}
	q += ")"
	mpp.logger.Debug("DeletePics(): q=", q, " params=", whereParams)
	_, err := mpp.executor().Exec(q, whereParams...)
	return err
}

func (mpp *msql_part_tx) GetPersonById(pId string) (*Person, error) {
	rows, err := mpp.executor().Query("SELECT cam_id, created_at, last_seen, profile_id, picture_id, match_group FROM person WHERE id=?", pId)
	if err != nil {
		mpp.logger.Warn("GetPersonById(): could read by id=", pId, ", err=", err)
		return nil, err
	}
	defer rows.Close()

	if rows.Next() {
		p := new(Person)
		p.Id = pId
		err := rows.Scan(&p.CamId, &p.CreatedAt, &p.LastSeenAt, &p.ProfileId, &p.PictureId, &p.MatchGroup)
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
	q := "SELECT p.id, p.cam_id, p.created_at, p.last_seen, p.profile_id, p.picture_id, p.match_group FROM person AS p "
	whereCond := []string{}
	whereParams := []interface{}{}

	if pQuery.CamId > 0 {
		whereCond = append(whereCond, "p.cam_id=?")
		whereParams = append(whereParams, pQuery.CamId)
	}

	if pQuery.MaxLastSeenAt != common.TIMESTAMP_NA {
		whereCond = append(whereCond, "p.last_seen <= ?")
		whereParams = append(whereParams, int64(pQuery.MaxLastSeenAt))
	}

	if pQuery.MinCreatedAt != nil {
		whereCond = append(whereCond, "p.created_at >= ?")
		whereParams = append(whereParams, uint64(*pQuery.MinCreatedAt))
	}

	if pQuery.MaxCreatedAt != nil {
		whereCond = append(whereCond, "p.created_at <= ?")
		whereParams = append(whereParams, uint64(*pQuery.MaxCreatedAt))
	}

	if pQuery.MinFacesCount > 0 {
		whereCond = append(whereCond, "(SELECT count(*) FROM face WHERE person_id=p.id) >= ?")
		whereParams = append(whereParams, uint64(pQuery.MinFacesCount))
	}

	if pQuery.MatchGroup != nil {
		whereCond = append(whereCond, "p.match_group = ?")
		whereParams = append(whereParams, *pQuery.MatchGroup)
	}

	if pQuery.MinId != nil {
		whereCond = append(whereCond, "p.id > ?")
		whereParams = append(whereParams, *pQuery.MinId)
	}

	if pQuery.PersonIds != nil {
		if len(pQuery.PersonIds) == 0 {
			mpp.logger.Debug("FindPersons: empty, but not nil pQuery.PersonIds, returns empty result ")
			return []*Person{}, nil
		}
		inc := "p.id IN("
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

	switch pQuery.Order {
	case PQO_LAST_SEEN_DESC:
		q = q + " ORDER BY p.last_seen DESC"
	case PQO_CREATED_AT_ASC:
		q = q + " ORDER BY p.created_at ASC"
	case PQO_ID_ASC:
		q = q + " ORDER BY p.id ASC"
	default:
	}

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
		err := rows.Scan(&p.Id, &p.CamId, &p.CreatedAt, &p.LastSeenAt, &p.ProfileId, &p.PictureId, &p.MatchGroup)
		if err != nil {
			mpp.logger.Warn("FindPersons(): could not scan result err=", err)
			return nil, err
		}
		res = append(res, p)
	}
	return res, nil
}

func (mpp *msql_part_tx) InsertPerson(p *Person) error {
	_, err := mpp.executor().Exec("INSERT INTO person(id, cam_id, created_at, last_seen, profile_id, picture_id, match_group) VALUES (?,?,?,?,?,?)",
		p.Id, p.CamId, p.CreatedAt, p.LastSeenAt, p.ProfileId, p.PictureId, p.MatchGroup)
	if err != nil {
		mpp.logger.Warn("InsertPerson(): Could not insert new person ", p, ", got the err=", err)
		return err
	}

	return nil
}

func (mpp *msql_part_tx) InsertPersons(persons []*Person) error {
	mpp.logger.Debug("Inserting ", len(persons), " persons to DB: ", persons)
	if len(persons) > 0 {
		q := "INSERT INTO person(id, cam_id, created_at, last_seen, profile_id, picture_id, match_group) VALUES "
		vals := []interface{}{}
		for i, p := range persons {
			if i > 0 {
				q = q + ", "
			}
			q = q + "(?,?,?,?,?,?,?)"
			vals = append(vals, p.Id, p.CamId, p.CreatedAt, p.LastSeenAt, p.ProfileId, p.PictureId, p.MatchGroup)
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
	_, err := mpp.executor().Exec("UPDATE person SET last_seen=?, profile_id=?, picture_id=?, match_group=? WHERE id=?",
		p.LastSeenAt, p.ProfileId, p.PictureId, p.MatchGroup, p.Id)
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

// Invoked when a profile is deleted
func (mpp *msql_part_tx) UpdatePersonsProfileId(prfId, newPrfId int64) error {
	mpp.logger.Debug("Updating persons with profileId=", prfId, " to newProfileId=", newPrfId)
	_, err := mpp.executor().Exec("UPDATE person SET profile_id=? WHERE profile_id=?", newPrfId, prfId)
	return err
}

func (mpp *msql_part_tx) UpdatePersonMatchGroup(persId string, mg int64) error {
	mpp.logger.Debug("Updating person with person_id=", persId, " to match_group=", mg)
	_, err := mpp.executor().Exec("UPDATE person SET match_group=? WHERE id=?", mg, persId)
	return err
}

func (mpp *msql_part_tx) FindPersonsForMatchCache(orgId, startMg int64, limit int) (*MatcherRecords, error) {
	rows, err := mpp.executor().Query("SELECT p.id, p.match_group, f.id, f.v128d FROM person AS p JOIN face AS f ON p.id=f.person_id WHERE p.cam_id IN (SELECT id FROM camera WHERE org_id=?) AND p.match_group>=? AND p.match_group > 0 ORDER BY p.match_group LIMIT ?",
		orgId, startMg, limit)
	if err != nil {
		mpp.logger.Warn("FindPersonsForMatchCache(): Could not select data by orgId=", orgId, ", startMg=", startMg, ", limit=", limit, ", got the err=", err)
		return nil, err
	}
	defer rows.Close()
	res := &MatcherRecords{Records: make([]*MatcherRecord, 0, 1), MinMG: math.MaxInt64}

	mapRecs := make(map[string]*MatcherRecord)
	for rows.Next() {
		var pId string
		var pMG, fId int64
		vec := make([]byte, common.V128D_SIZE)
		err := rows.Scan(&pId, &pMG, &fId, &vec)
		if err != nil {
			mpp.logger.Warn("FindPersonsForMatchCache(): scan err=", err)
			return nil, err
		}

		mr, ok := mapRecs[pId]
		if !ok {
			mr = new(MatcherRecord)
			mr.Person = new(Person)
			mr.Person.Id = pId
			mr.Person.MatchGroup = pMG
			mr.Faces = make([]*Face, 0, 1)
			mapRecs[pId] = mr
			res.Records = append(res.Records, mr)

			if res.MaxMG < pMG {
				res.MaxMG = pMG
			}
			if res.MinMG > pMG {
				res.MinMG = pMG
			}
		}

		f := new(Face)
		f.Id = fId
		f.V128D = common.NewV128D()
		f.V128D.Assign(vec)
		mr.Faces = append(mr.Faces, f)
		res.FacesCnt++ // counting faces in the result
	}
	return res, nil
}

func (mpp *msql_part_tx) DeletePerson(personId string) error {
	mpp.logger.Debug("Delete person with person_id=", personId)
	_, err := mpp.executor().Exec("DELETE FROM person WHERE id=?", personId)
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

func (mpp *msql_part_tx) CheckProfileInOrgWithCam(prId, camId int64) (bool, error) {
	rows, err := mpp.executor().Query("SELECT p.id FROM profile AS p WHERE p.id=? AND p.org_id= (SELECT org_id FROM camera WHERE id=?)", prId, camId)
	if err != nil {
		mpp.logger.Warn("CheckProfileInOrgWithCam(): err=", err)
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
	mpp.logger.Debug("UpdateProfile(): updating profile(setting only picture_id actually) ", prf)
	_, err := mpp.executor().Exec("UPDATE profile SET picture_id=? WHERE id=? ", prf.PictureId, prf.Id)
	return err
}

func (mpp *msql_part_tx) GetProfileById(pId int64) (*Profile, error) {
	mpp.logger.Debug("GetProfileById(): Getting profile by id=", pId)
	rows, err := mpp.executor().Query("SELECT org_id, picture_id FROM profile WHERE id=?", pId)
	if err != nil {
		mpp.logger.Warn("GetProfileById: Requesting profile by id=", pId, ", err=", err)
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, common.NewError(common.ERR_NOT_FOUND, "Cannot find profile by id="+strconv.FormatInt(pId, 10))
	}

	p := new(Profile)
	p.Id = pId
	err = rows.Scan(&p.OrgId, &p.PictureId)
	if err != nil {
		mpp.logger.Warn("GetProfileById(): Could not scan result err=", err)
		return nil, err
	}
	// If we don't do it in an opened transaction, following calls will fail!
	rows.Close()
	metas, err := mpp.GetProfileMetas([]int64{pId})
	if err != nil {
		return nil, err
	}
	p.Meta = metas
	return p, mpp.GetProfileKVs(p)
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

	if prQuery.AllMeta {
		err = mpp.GetProfilesKVs(res)
		if err != nil {
			mpp.logger.Warn("GetProfiles(): could not read k-v pairs err=", err)
			return nil, err
		}
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

func (mpp *msql_part_tx) InsertProfileKVs(prof *Profile) error {
	if prof.KeyVals == nil || len(prof.KeyVals) == 0 {
		return nil
	}
	q := "INSERT INTO profile_kvs(`profile_id`, `key`, `value`) VALUES "

	params := []interface{}{}
	first := true
	for k, v := range prof.KeyVals {
		if first {
			q += "(?,?,?)"
		} else {
			q += ", (?,?,?)"
		}
		first = false
		params = append(params, prof.Id, k, v)
	}
	mpp.logger.Debug("InsertProfileKVs(): q=", q, " params=", params)
	_, err := mpp.executor().Exec(q, params...)
	return err
}

func (mpp *msql_part_tx) DeleteProfileKVs(prfId int64) error {
	mpp.logger.Debug("DeleteProfileKVs(): deleting all keys for prfId=", prfId)
	_, err := mpp.executor().Exec("DELETE FROM profile_kvs WHERE profile_id=?", prfId)
	return err
}

func (mpp *msql_part_tx) GetProfileKVs(prof *Profile) error {
	res, err := mpp.executor().Query("SELECT `key`, `value` FROM profile_kvs WHERE profile_id=?", prof.Id)
	if err != nil {
		mpp.logger.Warn("GetProfileKVs(): could not read key-value pairs for profileI=", prof.Id, ", err=", err)
		return err
	}
	defer res.Close()

	prof.KeyVals = make(map[string]string)
	for res.Next() {
		var k, v string
		res.Scan(&k, &v)
		prof.KeyVals[k] = v
	}
	return nil
}

func (mpp *msql_part_tx) GetProfilesKVs(profs []*Profile) error {
	if profs == nil || len(profs) == 0 {
		return nil
	}

	q := "SELECT profile_id, `key`, `value` FROM profile_kvs WHERE profile_id IN("
	whereParams := []interface{}{}
	pMap := make(map[int64]*Profile)
	for i, p := range profs {
		if i > 0 {
			q += ", ?"
		} else {
			q += "?"
		}
		whereParams = append(whereParams, p.Id)
		pMap[p.Id] = p
	}
	q += ")"

	mpp.logger.Debug("GetProfileKVs(): q=", q, ", whereParams=", whereParams)
	res, err := mpp.executor().Query(q, whereParams...)
	if err != nil {
		mpp.logger.Warn("GetProfileKVs(): could not read key-values for q=", q, " params=", whereParams, ", err=", err)
		return err
	}
	defer res.Close()

	for res.Next() {
		var pid int64
		var k, v string
		res.Scan(&pid, &k, &v)
		p := pMap[pid]
		if p.KeyVals == nil {
			p.KeyVals = make(map[string]string)
		}
		p.KeyVals[k] = v
	}
	return nil
}
