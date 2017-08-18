package fpcp

import (
	"errors"
	"net"
	"strconv"

	"github.com/jrivets/gorivets"
	"github.com/jrivets/log4g"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"

	"github.com/pixty/console/common"
)

const (
	mtKeyError     = "error"
	mtKeySessionId = "session_id"

	mtErrVal_UnknonwSess = "1" //Unknown session id
	mtErrVal_AuthFailed  = "2" //Unknown credentials
)

type (
	FPCPServer struct {
		// The console configuration. Will be injected
		Config   *common.ConsoleConfig `inject:""`
		log      gorivets.Logger
		sessions *gorivets.Lru
		listener net.Listener
		started  bool
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
	RegisterSceneProcessorServiceServer(gs, fs)
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

func (fs *FPCPServer) onDropSession(sid, ak interface{}) {
	fs.log.Info("Dropping session_id=", sid, " for access_key=", ak)
}

func (fs *FPCPServer) checkSession(ctx context.Context) bool {
	md, ok := metadata.FromContext(ctx)
	if ok {
		sess := getFirstValue(md, mtKeySessionId)
		ak, ok := fs.sessions.Get(sess)
		if ok {
			fs.log.Debug("Session check is ok for session_id=", sess, ", access_key=", ak)
			return true
		}
		fs.log.Warn("Unknown connection for session_id=", sess)
	} else {
		fs.log.Warn("Got a gRPC call expecting session id, but it is not provided")
	}
	return false
}

func setError(ctx context.Context, err string) {
	trailer := metadata.Pairs(mtKeyError, err)
	grpc.SetTrailer(ctx, trailer)
}

// -------------------------------- FPCP -------------------------------------
func (fs *FPCPServer) Authenticate(ctx context.Context, authToken *AuthToken) (*Void, error) {
	fs.log.Info("Got authentication request for access_key=", authToken.Access)
	if len(authToken.Access) == 0 || len(authToken.Secret) == 0 {
		fs.log.Warn("Empty credentials in authentication!")
		setError(ctx, mtErrVal_AuthFailed)
		return &Void{}, nil
	}

	sid := common.NewUUID()
	fs.sessions.Add(sid, authToken.Access, 1)
	fs.log.Info("Assigning session_id=", sid, " for access_key=", authToken.Access)

	trailer := metadata.Pairs(mtKeySessionId, sid)
	grpc.SetTrailer(ctx, trailer)
	grpc.SetHeader(ctx, trailer)
	return &Void{}, nil
}

func (fs *FPCPServer) OnScene(ctx context.Context, scn *Scene) (*Void, error) {
	if !fs.checkSession(ctx) {
		fs.log.Warn("Unauthorized call to OnScene()")
		setError(ctx, mtErrVal_UnknonwSess)
		return &Void{}, nil
	}
	fs.log.Info("got OnScene() ", len(scn.Frame.Data))
	return &Void{}, nil
}
