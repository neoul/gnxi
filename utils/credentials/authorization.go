// Package credentials loads certificates and validates user credentials.
package credentials

import (
	"github.com/golang/glog"
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
func validateUser(m map[string]string) error {
	return nil
	// if _, ok := m["username"]; !ok {
	// 	return errMissingUsername
	// }
	// if _, ok := m["password"]; !ok {
	// 	return errMissingpassword
	// }
	// // [FIXME] Need to add user validation
	// if len(m["username"]) > 0 && len(m["username"]) > 0 {
	// 	return nil
	// }
	// return errAuthFailed
}

// UnaryInterceptor - used to validate authentication
func UnaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	meta, ok := utils.GetMetadata(ctx)
	if !ok {
		return nil, errMissingMetadata
	}
	address, ok := meta["peer"]
	if !ok {
		address = "unknown"
	}
	err := validateUser(meta)
	if err != nil {
		glog.Errorf("[%s] %v", address, err)
		return nil, err
	}
	resp, err := handler(ctx, req)
	if err != nil {
		glog.Errorf("[%s] %v", address, err)
	}
	return resp, err
}

// wrappedStream wraps around the embedded grpc.ServerStream, and intercepts the RecvMsg and
// SendMsg method call.
type wrappedStream struct {
	grpc.ServerStream
}

func (w *wrappedStream) RecvMsg(m interface{}) error {
	return w.ServerStream.RecvMsg(m)
}

func (w *wrappedStream) SendMsg(m interface{}) error {
	return w.ServerStream.SendMsg(m)
}

func newWrappedStream(s grpc.ServerStream) grpc.ServerStream {
	return &wrappedStream{s}
}

// StreamInterceptor - used to validate authentication
func StreamInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	meta, ok := utils.GetMetadata(ss.Context())
	if !ok {
		return errMissingMetadata
	}
	address, ok := meta["peer"]
	if !ok {
		address = "unknown"
	}
	err := validateUser(meta)
	if err != nil {
		glog.Errorf("[%s] %v", address, err)
		return err
	}
	err = handler(srv, newWrappedStream(ss))
	if err != nil {
		glog.Errorf("[%s] %v", address, err)
	}
	return err
}
