package fpcp

import (
	"net"

	"github.com/jrivets/gorivets"

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

type server struct {
	log      gorivets.Logger
	sessions *gorivets.Lru
}

func newFpcpServer(log gorivets.Logger, sessPoolSize int64) *server {
	log.Info("Creating new server with sessPoolSize=", sessPoolSize)
	s := new(server)
	s.log = log
	s.sessions = gorivets.NewLRU(sessPoolSize, s.onDropSession)
	return s
}

func getFirstValue(md metadata.MD, key string) string {
	vals, ok := md[key]
	if !ok {
		return ""
	}
	return vals[0]
}

func setError(ctx context.Context, err string) {
	trailer := metadata.Pairs(mtKeyError, err)
	grpc.SetTrailer(ctx, trailer)
}

func (s *server) onDropSession(sid, ak interface{}) {
	s.log.Info("Dropping session_id=", sid, " for access_key=", ak)
}

func (s *server) checkSession(ctx context.Context) bool {
	md, ok := metadata.FromContext(ctx)
	if ok {
		sess := getFirstValue(md, mtKeySessionId)
		ak, ok := s.sessions.Get(sess)
		if ok {
			s.log.Debug("Session check is ok for session_id=", sess, ", access_key=", ak)
			return true
		}
		s.log.Warn("Unknown connection for session_id=", sess)
	} else {
		s.log.Warn("Got a gRPC call expecting session id, but it is not provided")
	}
	return false
}

func (s *server) Authenticate(ctx context.Context, authToken *AuthToken) (*Void, error) {
	s.log.Info("Got authentication request for access_key=", authToken.Access)
	if len(authToken.Access) == 0 || len(authToken.Secret) == 0 {
		s.log.Warn("Empty credentials in authentication!")
		setError(ctx, mtErrVal_AuthFailed)
		return &Void{}, nil
	}

	sid := common.NewUUID()
	s.sessions.Add(sid, authToken.Access, 1)
	s.log.Info("Assigning session_id=", sid, " for access_key=", authToken.Access)

	trailer := metadata.Pairs(mtKeySessionId, sid)
	grpc.SetTrailer(ctx, trailer)
	grpc.SetHeader(ctx, trailer)
	return &Void{}, nil
}

func (s *server) OnScene(ctx context.Context, scn *Scene) (*Void, error) {
	if !s.checkSession(ctx) {
		s.log.Warn("Unauthorized call to OnScene()")
		setError(ctx, mtErrVal_UnknonwSess)
		return &Void{}, nil
	}
	s.log.Info("got OnScene() ", len(scn.Frame.Data))
	return &Void{}, nil
}

func RunFPCPServer(port string, logger gorivets.Logger) {
	log := logger.WithName("pixty.fpcp_server").(gorivets.Logger)
	log.Info("Listening on ", port)
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatal("Could not open TCP socket for listening on ", port)
	}

	srv := newFpcpServer(log, 10000)
	s := grpc.NewServer()
	RegisterSceneProcessorServiceServer(s, srv)
	// Register reflection service on gRPC server.
	reflection.Register(s)
	if err := s.Serve(lis); err != nil {
		log.Fatal("failed to serve: err=", err)
	}
}
