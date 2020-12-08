package server

import (
	"time"

	"github.com/neoul/gnxi/utilities/status"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/grpc/codes"
)

// SubscriberResponse message for the target-defined aliases (server aliases)
func buildAliasResponse(prefix *gnmipb.Path, alias string) []*gnmipb.SubscribeResponse {
	if prefix == nil || alias == "" {
		return nil
	}
	notification := gnmipb.Notification{
		Timestamp: time.Now().UnixNano(),
		Prefix:    prefix,
		Alias:     alias,
	}
	subscribeResponse := []*gnmipb.SubscribeResponse{
		{Response: &gnmipb.SubscribeResponse_Update{
			Update: &notification,
		}},
	}
	return subscribeResponse
}

func buildSubscribeResponse(prefix *gnmipb.Path, update []*gnmipb.Update, delete []*gnmipb.Path) []*gnmipb.SubscribeResponse {
	if update == nil && delete == nil {
		return nil
	}
	notification := gnmipb.Notification{
		Timestamp: time.Now().UnixNano(),
		Prefix:    prefix,
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

func newUpdateResultError(err error) *gnmipb.Error {
	if err == nil {
		return nil
	}
	gerr, _ := status.FromError(err)
	return gerr.UpdateError()
}

var aborted *gnmipb.Error = &gnmipb.Error{Code: uint32(codes.Aborted), Message: "unable to progress by another error"}

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
