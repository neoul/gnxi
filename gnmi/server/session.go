package server

import (
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/neoul/gnxi/utilities"
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
	ID                 uint64         `json:"id,omitempty"`
	SID                string         `json:"sid,omitempty"`
	Username           string         `json:"username,omitempty"`
	Password           string         `json:"password,omitempty"`
	GrpcVer            string         `json:"grpc-ver,omitempty"`
	ContentType        string         `json:"content-type,omitempty"`
	LoginTime          time.Time      `json:"login-time,omitempty"`
	DestinationAddress string         `json:"destination-address,omitempty"`
	DestinationPort    uint16         `json:"destination-port,omitempty"`
	Protocol           StreamProtocol `json:"protocol,omitempty"`

	// telechan chan *pb.SubscribeResponse // The channel to send telemetry updates
	// teledone chan bool                  // The channel to signal telemetry updates complete
	alias  map[string]*pb.Alias
	server *Server
	valid  bool
}

var (
	sessionID uint64
)

// Started - netsession interface to receive the session started event
func (s *Server) Started(local, remote net.Addr) {
	remoteaddr := remote.String()
	session, ok := s.sessions[remoteaddr]
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
		Protocol:           StreamGRPC,
		alias:              map[string]*pb.Alias{},
		server:             s, SID: remoteaddr,
	}
	s.sessions[remoteaddr] = session
}

// Closed - netsession interface to receive the session closed event
func (s *Server) Closed(local, remote net.Addr) {
	remoteaddr := remote.String()
	delete(s.sessions, remoteaddr)
}

// updateSession - Updated and Validate the session user
func (s *Server) updateSession(ctx context.Context, SID string) (*Session, error) {
	session, ok := s.sessions[SID]
	if !ok {
		return nil, errInvalidSession
	}
	m, ok := utilities.GetMetadata(ctx)
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
	return session, nil
}

// getSession - Updated and Validate the session user
func (s *Server) getSession(ctx context.Context) (*Session, error) {
	peer, ok := utilities.QueryMetadata(ctx, "peer")
	if !ok {
		return nil, errMissingMetadata
	}
	session, ok := s.sessions[peer]
	if !ok {
		localaddr, remoteaddr, ok := utilities.QueryAddr(ctx)
		if !ok {
			return nil, errMissingMetadata
		}
		s.Started(localaddr, remoteaddr)
		s.updateSession(ctx, peer)
		session, ok = s.sessions[peer]
		if !ok {
			return nil, errInvalidSession
		}
	} else if !session.valid {
		s.updateSession(ctx, peer)
	}
	return session, nil
}