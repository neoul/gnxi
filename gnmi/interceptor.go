package gnmi

import (
	"fmt"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

// UnaryInterceptor - used to intercept the unary gRPC requests and responses
func UnaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	fmt.Println("server info:", info)

	gnmis, ok := info.Server.(*Server)
	if ok {
		fmt.Println("gNMI Server !!!!")
		fmt.Println(gnmis)
	}
	fmt.Printf("%T", info.Server)

	return handler(ctx, req)
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

// StreamInterceptor - used to intercept the unary gRPC Streams
func StreamInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	return handler(srv, newWrappedStream(ss))
}
