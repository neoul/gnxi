package gnmi

import (
	"fmt"
	"time"

	"github.com/neoul/gnxi/utils"
	"golang.org/x/net/context"
)

// Session - gNMI session control bock
type Session struct {
	ID          int       `json:"id,omitempty"`
	Username    string    `json:"username,omitempty"`
	Password    string    `json:"password,omitempty"`
	GrpcVer     string    `json:"grpc-ver,omitempty"`
	ContentType string    `json:"content-type,omitempty"`
	SourceHost  string    `json:"source-host,omitempty"`
	LoginTime   time.Time `json:"login-time,omitempty"`
}

var ids int

// NewSession - Create and Update new session
func NewSession(ctx context.Context) *Session {
	if ctx == nil {
		return nil
	}
	m, ok := utils.GetMetadata(ctx)
	if !ok {
		return nil
	}
	ids++
	s := Session{
		ID: ids, Username: m["username"], Password: m["password"],
		GrpcVer: m["user-agent"], ContentType: m["content-type"],
		SourceHost: m["authority"], LoginTime: time.Now()}
	fmt.Println(s)
	return &s
}
