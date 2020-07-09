package gnmi

import (
	"time"

	pb "github.com/openconfig/gnmi/proto/gnmi"
)

func buildSubscribeResponse(prefix *pb.Path, alias string, update []*pb.Update, disableBundling bool, isInitUpdate bool) ([]*pb.SubscribeResponse, error) {
	if !disableBundling {
		notification := pb.Notification{
			Timestamp: time.Now().UnixNano(),
			Prefix:    prefix,
			Alias:     alias,
			Update:    update,
		}

		updates := []*pb.SubscribeResponse{
			{Response: &pb.SubscribeResponse_Update{
				Update: &notification,
			}},
			{Response: &pb.SubscribeResponse_SyncResponse{
				SyncResponse: true,
			}},
		}
		return updates, nil
	}
	var num int = len(update)
	var responses []*pb.SubscribeResponse
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
