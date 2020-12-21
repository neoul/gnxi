package status

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/golang/protobuf/proto"
	spb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
)

func TestStatus(t *testing.T) {

	err := fmt.Errorf("invalid path: /path/to/data")
	s := Error(codes.InvalidArgument, err)
	t.Log(s)
	t.Logf("%v: %v", s.(*Status).Code, s.(*Status).Message)
	// s = WithInfo(s, "WRONG_PATH", "/path/to/data", "127.0.0.1:13322")
	ss := s.(*Status)
	t.Log(ss.String())
	gs := ss.GRPCStatus()
	j, _ := json.Marshal(gs.Proto())

	fmt.Println(string(j))
	xs := &spb.Status{
		Code:    int32(ss.Code),
		Message: ss.Message,
	}
	fmt.Println(proto.MarshalTextString(xs))
	// fmt.Println(proto.MarshalTextString(gs.Proto()))
}
