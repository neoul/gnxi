package server

import (
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/neoul/gnxi/utilities"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

	alias map[string]*gnmipb.Alias
	valid bool
}

var (
	sessionID          uint64
	errMissingMetadata = status.Errorf(codes.InvalidArgument, "missing metadata")
	errInvalidSession  = status.Errorf(codes.InvalidArgument, "invalid session")
)

// Started - netsession interface to receive the session started event
func (s *Server) Started(local, remote net.Addr) {
	remoteaddr := remote.String()
	_, ok := s.sessions[remoteaddr]
	if ok {
		return
	}
	sessionID++
	index := strings.LastIndex(remoteaddr, ":")
	destinationAddress := remoteaddr[:index]
	destinationPort, _ := strconv.ParseUint(remoteaddr[index+1:], 0, 16)
	session := &Session{
		ID:                 sessionID,
		SID:                remoteaddr,
		LoginTime:          time.Now(),
		DestinationAddress: destinationAddress,
		DestinationPort:    uint16(destinationPort),
		Protocol:           StreamGRPC,
		alias:              map[string]*gnmipb.Alias{},
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
