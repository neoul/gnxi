package gnmi

import (
	"fmt"
	"strconv"
	"time"

	"github.com/neoul/gnxi/utils"
	pb "github.com/openconfig/gnmi/proto/gnmi"
	"golang.org/x/net/context"
)

// StreamProtocol - The type of the subscription protocol
type StreamProtocol int

const (
	// StreamUserDefined - Stream subscription over user-defined RPC
	StreamUserDefined StreamProtocol = iota
	// StreamSSH - Stream subscription over SSH
	StreamSSH
	// StreamGRPC - Stream subscription over GRPC
	StreamGRPC
	// StreamJSONRPC - Stream subscription over JSON RPC
	StreamJSONRPC
	// StreamThriftRPC - Stream subscription over ThriftRPC
	StreamThriftRPC
	// StreamWebsocketRPC - Stream subscription over WebsocketRPC
	StreamWebsocketRPC
)

var streamProtocolStr = [...]string{
	"STREAM_USER_DEFINED_RPC",
	"STREAM_SSH",
	"STREAM_GRPC",
	"STREAM_JSON_RPC",
	"STREAM_THRIFT_RPC",
	"STREAM_WEBSOCKET_RPC",
}

func (s StreamProtocol) String() string { return streamProtocolStr[s%5] }

// Session - gNMI gRPC Session information managed by server
type Session struct {
	ID                 uint64          `json:"id,omitempty"`
	SID                string          `json:"sid,omitempty"`
	Username           string          `json:"username,omitempty"`
	Password           string          `json:"password,omitempty"`
	GrpcVer            string          `json:"grpc-ver,omitempty"`
	ContentType        string          `json:"content-type,omitempty"`
	LoginTime          time.Time       `json:"login-time,omitempty"`
	DestinationAddress string          `json:"destination-address,omitempty"`
	DestinationPort    uint16          `json:"destination-port,omitempty"`
	Protocol           StreamProtocol  `json:"protocol,omitempty"`
	Subscriptions      []*Subscription `json:"subscriptions,omitempty"`
	alias              map[string]*pb.Alias
	server             *Server
	entrance           int
}

var (
	sessionID uint64
)

// NewSession - Get the Session. Create new Session if not exists.
func (s *Server) NewSession(ctx context.Context) (*Session, error) {
	if ctx == nil {
		return nil, errMissingMetadata
	}
	m, ok := utils.GetMetadata(ctx)
	if !ok {
		return nil, errMissingMetadata
	}
	peer := m["peer"]
	s.mu.Lock()
	s.dataBlock.Lock()
	defer s.mu.Unlock()
	defer s.dataBlock.Unlock()

	session, ok := s.Sessions[peer]
	if ok {
		session.entrance++
		return session, nil
	}
	sessionID++
	username, _ := m["username"]
	password, _ := m["password"]
	userAgent, _ := m["user-agent"]
	contentType, _ := m["content-type"]
	destinationAddress := m["peer-address"]
	destinationPort, _ := strconv.ParseUint(m["peer-port"], 0, 16)
	session = &Session{
		ID: sessionID, Username: username, Password: password,
		GrpcVer: userAgent, ContentType: contentType, LoginTime: time.Now(),
		DestinationAddress: destinationAddress, DestinationPort: uint16(destinationPort), Protocol: StreamGRPC,
		Subscriptions: []*Subscription{}, alias: map[string]*pb.Alias{},
		server: s, SID: peer,
	}
	session.entrance++
	s.Sessions[peer] = session
	fmt.Printf("session[%s] opened\n", peer)
	return session, nil
}

// CloseSession - Close the session
func (s *Server) CloseSession(session *Session) {
	s.mu.Lock()
	s.dataBlock.Lock()
	defer s.mu.Unlock()
	defer s.dataBlock.Unlock()
	session.entrance--
	if session.entrance <= 0 {
		peer := session.SID
		delete(s.Sessions, peer)
		fmt.Printf("session[%s] closed\n", peer)
	}
}
