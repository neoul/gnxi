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
	// telemetry update bundling disabled.
	if *disableBundling {
		// var responses []*pb.SubscribeResponse
		// updateNum := len(update)
		// deleteNum := len(delete)
		// max := updateNum
		// if max < deleteNum {
		// 	max = deleteNum
		// }
		// if isInitUpdate {
		// 	max++
		// }
		// if max > 0 {
		// 	responses = make([]*pb.SubscribeResponse, max)
		// 	for i, u := range update {
		// 		dd := []*pb.Path{}
		// 		for j, d := range delete {
		// 			if d == u.Path {
		// 				delete[j] = nil
		// 			}
		// 		}
		// 		if isreplace {
		// 			responses[i] = &pb.SubscribeResponse{
		// 				Response: &pb.SubscribeResponse_Update{
		// 					Update: &pb.Notification{
		// 						Timestamp: time.Now().UnixNano(),
		// 						Prefix:    prefix,
		// 						Alias:     alias,
		// 						Update:    []*pb.Update{u},
		// 						Delete:    []*pb.Path{u.Path},
		// 					},
		// 				}}
		// 		} else {
		// 			responses[i] = &pb.SubscribeResponse{
		// 				Response: &pb.SubscribeResponse_Update{
		// 					Update: &pb.Notification{
		// 						Timestamp: time.Now().UnixNano(),
		// 						Prefix:    prefix,
		// 						Alias:     alias,
		// 						Update:    []*pb.Update{u},
		// 					},
		// 				}}
		// 		}
		// 	}

		// 	if isInitUpdate {
		// 		responses[max-1] = &pb.SubscribeResponse{
		// 			Response: &pb.SubscribeResponse_SyncResponse{
		// 				SyncResponse: true,
		// 			}}
		// 	}
		// }
		// return responses
	}

	notification := pb.Notification{
		Timestamp: time.Now().UnixNano(),
		Prefix:    prefix,
		Alias:     alias,
		Update:    update,
		Delete:    delete,
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
		return subscribeResponse
	}
	subscribeResponse := []*pb.SubscribeResponse{
		{Response: &pb.SubscribeResponse_Update{
			Update: &notification,
		}},
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
