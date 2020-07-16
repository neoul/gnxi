package gnmi

import (
	"time"

	pb "github.com/openconfig/gnmi/proto/gnmi"
)

func buildSubscribeResponse(prefix *pb.Path, alias string, update []*pb.Update, disableBundling bool, isInitUpdate bool) ([]*pb.SubscribeResponse, error) {
	var responses []*pb.SubscribeResponse
	if update == nil || len(update) == 0 {
		if isInitUpdate {
			subscribeResponse := []*pb.SubscribeResponse{
				{Response: &pb.SubscribeResponse_SyncResponse{
					SyncResponse: true,
				}},
			}
			return subscribeResponse, nil
		}
		return []*pb.SubscribeResponse{}, nil
	}

	if !disableBundling {
		notification := pb.Notification{
			Timestamp: time.Now().UnixNano(),
			Prefix:    prefix,
			Alias:     alias,
			Update:    update,
		}
		if isInitUpdate { // set SyncResponse if init update
			subscribeResponse := []*pb.SubscribeResponse{
				{Response: &pb.SubscribeResponse_Update{
					Update: &notification,
				}},
				{Response: &pb.SubscribeResponse_SyncResponse{
					SyncResponse: true,
				}},
			}
			return subscribeResponse, nil
		}
		subscribeResponse := []*pb.SubscribeResponse{
			{Response: &pb.SubscribeResponse_Update{
				Update: &notification,
			}},
		}
		return subscribeResponse, nil
	}
	// telemetry update bundling disabled.
	num := len(update)
	if isInitUpdate {
		responses = make([]*pb.SubscribeResponse, num+1)
	} else {
		responses = make([]*pb.SubscribeResponse, num)
	}
	for i, u := range update {
		notification := pb.Notification{
			Timestamp: time.Now().UnixNano(),
			Prefix:    prefix,
			Alias:     alias,
			Update:    []*pb.Update{u},
		}
		responses[i] = &pb.SubscribeResponse{
			Response: &pb.SubscribeResponse_Update{
				Update: &notification,
			}}
	}
	if isInitUpdate {
		responses[num] = &pb.SubscribeResponse{
			Response: &pb.SubscribeResponse_SyncResponse{
				SyncResponse: true,
			}}
	}
	return responses, nil
}

func buildSyncResponse() ([]*pb.SubscribeResponse, error) {
	return []*pb.SubscribeResponse{
		{Response: &pb.SubscribeResponse_SyncResponse{
			SyncResponse: true,
		}},
	}, nil
}
