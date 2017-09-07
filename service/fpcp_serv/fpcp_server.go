package fpcp_serv

import (
	"errors"
	"net"
	"strconv"
	"sync"

	"github.com/jrivets/gorivets"
	"github.com/jrivets/log4g"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"

	"github.com/pixty/console/common"
	"github.com/pixty/console/common/fpcp"
	"github.com/pixty/console/model"
	"github.com/pixty/console/service/scene"
)

const (
	mtKeyError     = "error"
	mtKeySessionId = "session_id"

	mtErrVal_UnknonwSess = "1" //Unknown session id
	mtErrVal_AuthFailed  = "2" //Unknown credentials
	mtErrVal_UnableNow   = "3" //Unable run now. Please try again later
)

type (
	FPCPServer struct {
		// The console configuration. Will be injected
		Config     *common.ConsoleConfig `inject:""`
		Persister  model.Persister       `inject:"persister"`
		ScnService *scene.SceneProcessor `inject:"scnProcessor"`
		log        gorivets.Logger
		sessions   gorivets.LRU
		camId2sess map[int64]string // access keys to sess
		listener   net.Listener
		started    bool
		lock       sync.Mutex
	}
)

func NewFPCPServer() *FPCPServer {
	fs := new(FPCPServer)
	fs.log = log4g.GetLogger("pixty.fpcp")
	fs.started = false
	return fs
}

// =========================== MongoPersister ================================
// ----------------------------- LifeCycler ----------------------------------
func (fs *FPCPServer) DiPhase() int {
	return common.CMP_PHASE_FPCP
}

func (fs *FPCPServer) DiInit() error {
	fs.log.Info("Initializing.")
	port := fs.Config.GrpcFPCPPort
	fs.log.Info("Listening on ", port)

	lis, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		msg := "Could not open gRPC TCP socket for listening on " + strconv.Itoa(port)
		fs.log.Fatal(msg)
		return errors.New(msg)
	}

	fs.sessions = gorivets.NewLRU(int64(fs.Config.GrpcFPCPSessCapacity), fs.onDropSession)
	fs.camId2sess = make(map[int64]string)
	fs.listener = lis
	fs.run()
	return nil
}

func (fs *FPCPServer) DiShutdown() {
	fs.log.Info("Shutting down.")
	fs.close()
}

func getFirstValue(md metadata.MD, key string) string {
	vals, ok := md[key]
	if !ok {
		return ""
	}
	return vals[0]
}

func (fs *FPCPServer) run() {
	fs.log.Info("run() called")
	fs.started = true
	gs := grpc.NewServer()
	fpcp.RegisterSceneProcessorServiceServer(gs, fs)
	// Register reflection service on gRPC server.
	reflection.Register(gs)
	go func() {
		fs.log.Info("Starting go routine by listening gRPC FPCP connections")
		if err := gs.Serve(fs.listener); err != nil && fs.started {
			fs.log.Fatal("failed to serve: err=", err)
		}
		fs.log.Info("Finishing go routine by listening gRPC FPCP connections")
	}()
}

func (fs *FPCPServer) close() {
	fs.started = false
	fs.listener.Close()
}

func (fs *FPCPServer) onDropSession(sid, ci interface{}) {
	fs.log.Info("Dropping session_id=", sid, " for camId=", ci)
	go func() {
		fs.lock.Lock()
		defer fs.lock.Unlock()
		camId := ci.(int64)
		sess, ok := fs.camId2sess[camId]
		if ok && sess == sid.(string) {
			delete(fs.camId2sess, camId)
		}
	}()
}

func (fs *FPCPServer) checkSession(ctx context.Context) int64 {
	md, ok := metadata.FromContext(ctx)
	if ok {
		sess := getFirstValue(md, mtKeySessionId)
		ci, ok := fs.sessions.Get(sess)
		if ok {
			fs.log.Debug("Session check is ok for session_id=", sess, ", access_key=", ci)
			return ci.(int64)
		}
		fs.log.Warn("Unknown connection for session_id=", sess)
	} else {
		fs.log.Warn("Got a gRPC call expecting session id, but it is not provided")
	}
	return -1
}

func (fs *FPCPServer) authenticate(camId int64, authToken *fpcp.AuthToken) (string, error) {
	mpp, err := fs.Persister.GetPartitionTx("FAKE")
	if err != nil {
		return "", err
	}

	cam, err := mpp.GetCameraById(camId)
	if err != nil {
		return "", err
	}

	if cam == nil || cam.SecretKey != common.Hash(authToken.Secret) {
		fs.log.Info("Cannot authenticate by access_key=", authToken.Access, ", not found or wrong secret key")
		return "", nil
	}

	fs.lock.Lock()
	defer fs.lock.Unlock()
	sid := common.NewSession()
	oldSess, ok := fs.camId2sess[camId]
	if ok {
		delete(fs.camId2sess, camId)
		fs.sessions.Delete(oldSess)
		fs.log.Info("When authentication by access_key=", authToken.Access, ", found old session by the key sess=", oldSess)
	}
	fs.sessions.Add(sid, camId, 1)
	fs.camId2sess[camId] = sid
	return sid, nil
}

func setError(ctx context.Context, err string) {
	trailer := metadata.Pairs(mtKeyError, err)
	grpc.SetTrailer(ctx, trailer)
}

// -------------------------------- FPCP -------------------------------------
// We expect that access_key is camId, which is int64
func ak2camId(access_key string) int64 {
	camId, err := strconv.ParseInt(access_key, 10, 64)
	if err != nil {
		return -1
	}
	return camId
}

func (fs *FPCPServer) Authenticate(ctx context.Context, authToken *fpcp.AuthToken) (*fpcp.Void, error) {
	fs.log.Info("Got authentication request for access_key=", authToken.Access)
	if len(authToken.Access) == 0 || len(authToken.Secret) == 0 {
		fs.log.Warn("Empty credentials in authentication!")
		setError(ctx, mtErrVal_AuthFailed)
		return &fpcp.Void{}, nil
	}

	camId := ak2camId(authToken.Access)
	if camId < 0 {
		fs.log.Warn("Frame Proc uses not int value for the access_key=", authToken.Access, ", expecting camId which is int")
		setError(ctx, mtErrVal_AuthFailed)
		return &fpcp.Void{}, nil
	}

	sid, err := fs.authenticate(camId, authToken)
	if err != nil {
		fs.log.Warn("Unable authenticate. err=", err)
		setError(ctx, mtErrVal_UnableNow)
		return &fpcp.Void{}, nil
	}

	if sid == "" {
		fs.log.Info("Invalid credentials for access_key=", authToken.Access)
		setError(ctx, mtErrVal_AuthFailed)
		return &fpcp.Void{}, nil
	}

	fs.log.Info("Assigning session_id=", sid, " for access_key=", authToken.Access)
	trailer := metadata.Pairs(mtKeySessionId, sid)
	grpc.SetTrailer(ctx, trailer)
	grpc.SetHeader(ctx, trailer)
	return &fpcp.Void{}, nil
}

func (fs *FPCPServer) OnScene(ctx context.Context, scn *fpcp.Scene) (*fpcp.Void, error) {
	camId := fs.checkSession(ctx)
	if camId < 0 {
		fs.log.Warn("Unauthorized call to OnScene()")
		setError(ctx, mtErrVal_UnknonwSess)
		return &fpcp.Void{}, nil
	}
	fs.ScnService.OnFPCPScene(camId, scn)
	return &fpcp.Void{}, nil
}
