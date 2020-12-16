package status

import (
	"fmt"
	"testing"

	"google.golang.org/grpc/codes"
)

func TestStatus(t *testing.T) {

	err := fmt.Errorf("invalid path")
	s := Error(codes.InvalidArgument, err)
	t.Log(s)
	t.Logf("%v: %v", s.(GStatus).Code(), s.(GStatus).Message())
}
