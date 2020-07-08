package gnmi

import (
	"fmt"
	"net"
	"strconv"
	"strings"
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
	valid              bool
}

var (
	sessionID uint64
)

// Started - netsession interface to receive the session started event
func (s *Server) Started(local, remote net.Addr) {
	s.mu.Lock()
	s.dataBlock.Lock()
	defer s.mu.Unlock()
	defer s.dataBlock.Unlock()
	remoteaddr := remote.String()
	session, ok := s.Sessions[remoteaddr]
	if ok {
		return
	}
	sessionID++
	index := strings.LastIndex(remoteaddr, ":")
	destinationAddress := remoteaddr[:index]
	destinationPort, _ := strconv.ParseUint(remoteaddr[index+1:], 0, 16)
	session = &Session{
		ID:                 sessionID,
		LoginTime:          time.Now(),
		DestinationAddress: destinationAddress,
		DestinationPort:    uint16(destinationPort),
		Protocol:           StreamUserDefined,
		Subscriptions:      []*Subscription{},
		alias:              map[string]*pb.Alias{},
		server:             s, SID: remoteaddr,
	}
	s.Sessions[remoteaddr] = session
	fmt.Printf("session[%s] Started\n", remoteaddr)
}

// Closed - netsession interface to receive the session closed event
func (s *Server) Closed(local, remote net.Addr) {
	remoteaddr := remote.String()
	fmt.Printf("session[%s] Closed\n", remoteaddr)
	s.mu.Lock()
	s.dataBlock.Lock()
	delete(s.Sessions, remoteaddr)
	defer s.mu.Unlock()
	defer s.dataBlock.Unlock()
}

// updateSession - Updated and Validate the session user
func (s *Server) updateSession(ctx context.Context, SID string) (*Session, error) {
	session, ok := s.Sessions[SID]
	if !ok {
		return nil, errInvalidSession
	}
	m, ok := utils.GetMetadata(ctx)
	if !ok {
		return nil, errMissingMetadata
	}

	username, _ := m["username"]
	password, _ := m["password"]
	userAgent, _ := m["user-agent"]
	contentType, _ := m["content-type"]
	session.Username = username
	session.Password = password
	session.GrpcVer = userAgent
	session.ContentType = contentType
	session.Protocol = StreamGRPC
	session.valid = true
	fmt.Printf("session[%s] updated\n", SID)
	return session, nil
}

// getSession - Updated and Validate the session user
func (s *Server) getSession(ctx context.Context) (*Session, error) {
	peer, ok := utils.QueryMetadata(ctx, "peer")
	if !ok {
		return nil, errMissingMetadata
	}
	session, ok := s.Sessions[peer]
	if !ok {
		localaddr, remoteaddr, ok := utils.QueryAddr(ctx)
		if !ok {
			return nil, errMissingMetadata
		}
		s.Started(localaddr, remoteaddr)
		s.updateSession(ctx, peer)
		session, ok = s.Sessions[peer]
		if !ok {
			return nil, errInvalidSession
		}
	} else if !session.valid {
		s.updateSession(ctx, peer)
	}
	return session, nil
}
