// Package credentials loads certificates and validates user credentials.
package credentials

import (
	log "github.com/golang/glog"
	"github.com/neoul/gnxi/utils"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// gNMI server authorization using interceptor

var (
	errMissingMetadata = status.Errorf(codes.InvalidArgument, "missing metadata")
	errMissingUsername = status.Errorf(codes.InvalidArgument, "missing username")
	errMissingpassword = status.Errorf(codes.InvalidArgument, "missing password")
	errAuthFailed      = status.Errorf(codes.Unauthenticated, "authentication failed")
)

// validateUser validates the user.
func validateUser(ctx context.Context) error {
	m, ok := utils.GetMetadata(ctx)
	if !ok {
		return errMissingMetadata
	}
	if _, ok := m["username"]; !ok {
		return errMissingUsername
	}
	if _, ok := m["password"]; !ok {
		return errMissingpassword
	}
	// [FIXME] Need to add user validation
	if len(m["username"]) > 0 && len(m["username"]) > 0 {
		return nil
	}
	return errAuthFailed
}

// UnaryInterceptor - used to validate authentication
func UnaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	err := validateUser(ctx)
	if err != nil {
		log.Error(err)
	}
	resp, err := handler(ctx, req)
	// if err != nil {
	// 	log.Error(err)
	// }
	return resp, err
}

// wrappedStream wraps around the embedded grpc.ServerStream, and intercepts the RecvMsg and
// SendMsg method call.
type wrappedStream struct {
	grpc.ServerStream
}

func (w *wrappedStream) RecvMsg(m interface{}) error {
	// logger("Receive a message (Type: %T) at %s", m, time.Now().Format(time.RFC3339))
	return w.ServerStream.RecvMsg(m)
}

func (w *wrappedStream) SendMsg(m interface{}) error {
	// logger("Send a message (Type: %T) at %v", m, time.Now().Format(time.RFC3339))
	return w.ServerStream.SendMsg(m)
}

func newWrappedStream(s grpc.ServerStream) grpc.ServerStream {
	return &wrappedStream{s}
}

// StreamInterceptor - used to validate authentication
func StreamInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	err := validateUser(ss.Context())
	if err != nil {
		log.Error(err)
	}
	err = handler(srv, newWrappedStream(ss))
	// if err != nil {
	// 	log.Error(err)
	// }
	return err
}
