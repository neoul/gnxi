package server

import (
	"time"

	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func buildSubscribeResponse(prefix *gnmipb.Path, alias string, update []*gnmipb.Update, delete []*gnmipb.Path) []*gnmipb.SubscribeResponse {
	if update == nil && delete == nil {
		return []*gnmipb.SubscribeResponse{}
	}
	notification := gnmipb.Notification{
		Timestamp: time.Now().UnixNano(),
		Prefix:    prefix,
		Alias:     alias,
		Update:    update,
		Delete:    delete,
	}
	subscribeResponse := []*gnmipb.SubscribeResponse{
		{Response: &gnmipb.SubscribeResponse_Update{
			Update: &notification,
		}},
	}
	return subscribeResponse
}

func buildSyncResponse() []*gnmipb.SubscribeResponse {
	return []*gnmipb.SubscribeResponse{
		{Response: &gnmipb.SubscribeResponse_SyncResponse{
			SyncResponse: true,
		}},
	}
}

func initUpdateResult(deletePath []*gnmipb.Path, replacePath, updatePath []*gnmipb.Update) ([]*gnmipb.UpdateResult, error) {
	numDelete := len(deletePath)
	numReplace := len(replacePath)
	numUpdate := len(updatePath)
	numOp := numDelete + numReplace + numUpdate
	if numOp == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "no-value")
	}
	index := 0
	err := &gnmipb.Error{Code: uint32(codes.Aborted), Message: "unable to progress by the other error"} // default error
	result := make([]*gnmipb.UpdateResult, numOp)
	for _, path := range deletePath {
		updateResult := &gnmipb.UpdateResult{
			Path:    path,
			Op:      gnmipb.UpdateResult_DELETE,
			Message: err,
		}
		result[index] = updateResult
		index++
	}
	for _, u := range replacePath {
		updateResult := &gnmipb.UpdateResult{
			Path:    u.Path,
			Op:      gnmipb.UpdateResult_DELETE,
			Message: err,
		}
		result[index] = updateResult
		index++
	}
	for _, u := range updatePath {
		updateResult := &gnmipb.UpdateResult{
			Path:    u.Path,
			Op:      gnmipb.UpdateResult_DELETE,
			Message: err,
		}
		result[index] = updateResult
		index++
	}
	return result, nil
}

func newUpdateResultError(err error) *gnmipb.Error {
	if err == nil {
		return nil
	}
	grpcError, _ := status.FromError(err)
	return &gnmipb.Error{Code: uint32(grpcError.Code()), Message: grpcError.Message()}
}

var aborted *gnmipb.Error = &gnmipb.Error{Code: uint32(codes.Aborted), Message: "unable to progress by the other error"}

func buildUpdateResult(op gnmipb.UpdateResult_Operation, path *gnmipb.Path, err error) *gnmipb.UpdateResult {
	return &gnmipb.UpdateResult{
		Path:    path,
		Op:      op,
		Message: newUpdateResultError(err),
	}
}

func buildUpdateResultAborted(op gnmipb.UpdateResult_Operation, path *gnmipb.Path) *gnmipb.UpdateResult {
	return &gnmipb.UpdateResult{
		Path:    path,
		Op:      op,
		Message: aborted,
	}
}
