package gnmi

import (
	"time"

	pb "github.com/openconfig/gnmi/proto/gnmi"
)

func buildSubscribeResponse(prefix *pb.Path, alias string, update []*pb.Update, delete []*pb.Path, isInitUpdate bool) []*pb.SubscribeResponse {
	if update == nil && delete == nil {
		if isInitUpdate {
			return buildSyncResponse()
		}
		return []*pb.SubscribeResponse{}
	}
	notification := pb.Notification{
		Timestamp: time.Now().UnixNano(),
		Prefix:    prefix,
		Alias:     alias,
		Update:    update,
		Delete:    delete,
	}
	subscribeResponse := []*pb.SubscribeResponse{
		{Response: &pb.SubscribeResponse_Update{
			Update: &notification,
		}},
	}
	if isInitUpdate { // set SyncResponse if init update
		subscribeResponse = append(subscribeResponse, buildSyncResponse()...)
	}
	return subscribeResponse
}

func buildSyncResponse() []*pb.SubscribeResponse {
	return []*pb.SubscribeResponse{
		{Response: &pb.SubscribeResponse_SyncResponse{
			SyncResponse: true,
		}},
	}
}
