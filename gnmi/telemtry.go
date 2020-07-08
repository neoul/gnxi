package gnmi

import (
	"fmt"
	"strings"
	"time"

	"github.com/neoul/gnxi/utils"
	pb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ygot/experimental/ygotutils"
	"github.com/openconfig/ygot/ygot"
	cpb "google.golang.org/genproto/googleapis/rpc/code"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// SubscriptionEntity - The entities of the Subscription
type SubscriptionEntity struct {
	Path              *pb.Path            `json:"path,omitempty"`            // The data tree path.
	Mode              pb.SubscriptionMode `json:"mode,omitempty"`            // Subscription mode to be used.
	SampleInterval    uint64              `json:"sample_interval,omitempty"` // ns between samples in SAMPLE mode.
	SuppressRedundant bool                `json:"suppress_redundant,omitempty"`
	HeartbeatInterval uint64              `json:"heartbeat_interval,omitempty"`
	tsub              *TelemetrySubscription
	// [FIXME]
	// 1. Ticker (Timer)
	// 2. keys (The path to the subscription data)
}

// TelemetrySubscription - Default structure for Telemetry Update Subscription
type TelemetrySubscription struct {
	Prefix           *pb.Path                 `json:"prefix,omitempty"`
	UseAliases       bool                     `json:"use_aliases,omitempty"`
	Mode             pb.SubscriptionList_Mode `json:"mode,omitempty"`
	AllowAggregation bool                     `json:"allow_aggregation,omitempty"`
	Encoding         pb.Encoding              `json:"encoding,omitempty"`
	UpdatesOnly      bool                     `json:"updates_only,omitempty"`
	SubscribedEntity []*SubscriptionEntity    `json:"subscribed_entity,omitempty"` // Set of subscriptions to create.
	isPolling        bool
	session          *Session
	// server           *Server

	// // https://github.com/openconfig/gnmi/issues/45 - QoSMarking seems to be deprecated
	// Qos              *pb.QOSMarking           `json:"qos,omitempty"`          // DSCP marking to be used.
	// UseModels        []*pb.ModelData          `json:"use_models,omitempty"`   // (Check validate only in Request)
	// Alias            []*pb.Alias              `json:"alias,omitempty"`
}

func (s *Server) addSubscription(sub *TelemetrySubscription) {
	s.TSub = append(s.TSub, sub)
}

func (s *Server) delSubscription(sub *TelemetrySubscription) {
	for i, se := range s.TSub {
		if se == sub {
			s.TSub =
				append(s.TSub[:i], s.TSub[i+1:]...)
			break
		}
	}
}

// newSubscription - Create new TelemetrySubscription
func newSubscription(session *Session) *TelemetrySubscription {
	s := TelemetrySubscription{
		Prefix: nil, UseAliases: false, Mode: pb.SubscriptionList_STREAM,
		AllowAggregation: false, Encoding: pb.Encoding_JSON_IETF, UpdatesOnly: false,
		SubscribedEntity: []*SubscriptionEntity{}, isPolling: false, session: session,
	}
	return &s
}

func (sub *TelemetrySubscription) handleSubscription(request *pb.SubscribeRequest) ([]*pb.SubscribeResponse, error) {
	server := sub.session.server
	// Poll Subscription indication
	pollMode := request.GetPoll()
	if pollMode != nil {
		sub.isPolling = true
		sub.Mode = pb.SubscriptionList_POLL
		server.addSubscription(sub)
		return nil, nil
	}
	// Subscription requests for aliases update
	aliases := request.GetAliases()
	if aliases != nil {
		return nil, sub.updateClientAliases(aliases)
	}
	// extension := request.GetExtension()
	subscriptionList := request.GetSubscribe()
	if subscriptionList == nil {
		return nil, status.Errorf(codes.InvalidArgument, "no subscribe(SubscriptionList)")
	}
	encoding := subscriptionList.GetEncoding()
	useModules := subscriptionList.GetUseModels()
	if err := server.checkEncodingAndModel(encoding, useModules); err != nil {
		return nil, err
	}
	mode := subscriptionList.GetMode()
	prefix := subscriptionList.GetPrefix()
	sub.Encoding = encoding
	sub.Prefix = prefix
	sub.UseAliases = subscriptionList.GetUseAliases()
	sub.AllowAggregation = subscriptionList.GetAllowAggregation()
	sub.UpdatesOnly = subscriptionList.GetUpdatesOnly()
	subList := subscriptionList.GetSubscription()
	if subList == nil || len(subList) <= 0 {
		err := fmt.Errorf("no subscription field(Subscription)")
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	for i, entity := range subList {
		path := entity.GetPath()
		submod := entity.GetMode()
		sampleInterval := entity.GetSampleInterval()
		supressRedundant := entity.GetSuppressRedundant()
		heartBeatInterval := entity.GetHeartbeatInterval()
		fmt.Println("SubscriptionList:", i, prefix, path, submod, sampleInterval, supressRedundant, heartBeatInterval)
		subentity := SubscriptionEntity{
			tsub: sub, Path: path, Mode: submod, SampleInterval: sampleInterval,
			SuppressRedundant: supressRedundant, HeartbeatInterval: heartBeatInterval}
		sub.SubscribedEntity = append(sub.SubscribedEntity, &subentity)
		if mode == pb.SubscriptionList_STREAM {
			if err := subentity.registerStreamSubscription(); err != nil {
				return nil, err
			}
		}
	}
	// Build SubscribeResponses using the Subscription object
	if mode == pb.SubscriptionList_ONCE {
		// subscribeResponses, err := sub.onceSubscription(request)
		// return subscribeResponses, err
	} else if mode == pb.SubscriptionList_POLL {
		sub.pollSubscription(request)
	} else {
		server.addSubscription(sub)
		sub.streamSubscription(request)
	}
	return nil, nil
}

func (sub *TelemetrySubscription) updateClientAliases(aliases *pb.AliasList) error {
	aliaslist := aliases.GetAlias()
	for _, alias := range aliaslist {
		name := alias.GetAlias()
		if !strings.HasPrefix(name, "#") {
			msg := fmt.Sprintf("invalid alias(Alias): Alias must start with '#'")
			return status.Error(codes.InvalidArgument, msg)
		}
		sub.session.server.alias[name] = alias
	}
	return nil
}

func (sub *TelemetrySubscription) updateSeverliases(aliases *pb.AliasList) error {
	return nil
}

// initTelemetryUpdate - Process and generate responses for a init update.
func (ss *Session) initTelemetryUpdate(req *pb.SubscribeRequest) ([]*pb.SubscribeResponse, error) {
	s := ss.server
	subscriptionList := req.GetSubscribe()
	subList := subscriptionList.GetSubscription()
	if subList == nil || len(subList) <= 0 {
		err := fmt.Errorf("no subscription field(Subscription)")
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	updateOnly := subscriptionList.GetUpdatesOnly()
	if updateOnly {
		updates := []*pb.SubscribeResponse{
			{Response: &pb.SubscribeResponse_SyncResponse{
				SyncResponse: true,
			}},
		}
		return updates, nil
	}
	prefix := subscriptionList.GetPrefix()
	encoding := subscriptionList.GetEncoding()
	useAliases := subscriptionList.GetUseAliases()
	mode := subscriptionList.GetMode()
	alias := ""
	// [FIXME] Are they different?
	switch mode {
	case pb.SubscriptionList_POLL:
	case pb.SubscriptionList_ONCE:
	case pb.SubscriptionList_STREAM:
	}
	if useAliases {
		// 1. lookup the prefix in the session.alias for client.alias.
		// 1. lookup the prefix in the server.alias for server.alias.
		// prefix = nil
		// alias = xxx
	}
	update := make([]*pb.Update, len(subList))
	for i, updateEntry := range subList {
		// Get schema node for path from config struct.
		path := updateEntry.Path
		fullPath := utils.GetGNMIFullPath(prefix, path)
		if fullPath.GetElem() == nil && fullPath.GetElement() != nil {
			return nil, status.Error(codes.Unimplemented, "deprecated path element used")
		}
		node, stat := ygotutils.GetNode(s.model.schemaTreeRoot, s.config, fullPath)
		if isNil(node) || stat.GetCode() != int32(cpb.Code_OK) {
			return nil, status.Errorf(codes.NotFound, "path %v not found", fullPath)
		}
		typedValue, err := ygot.EncodeTypedValue(node, encoding)
		if err != nil {
			return nil, status.Errorf(codes.Internal, err.Error())
		}
		update[i] = &pb.Update{Path: path, Val: typedValue}
	}
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

func (sub *TelemetrySubscription) pollSubscription(request *pb.SubscribeRequest) ([]*pb.SubscribeResponse, error) {
	// updates := pb.SubscribeResponse{Response: &pb.SubscribeResponse_Update{
	// 	Update: &pb.Notification{
	// 		Timestamp: 200,
	// 		Update: []*pb.Update{
	// 			{
	// 				Path: &pb.Path{Element: []string{"a"}},
	// 				Val:  &pb.TypedValue{Value: &pb.TypedValue_IntVal{5}},
	// 			},
	// 		},
	// 	},
	// }}
	return nil, nil
}

func (sub *TelemetrySubscription) streamSubscription(request *pb.SubscribeRequest) ([]*pb.SubscribeResponse, error) {

	return nil, nil
}

func (sentity *SubscriptionEntity) registerStreamSubscription() error {
	// status.Error(codes.InvalidArgument, err.Error())
	return nil
}

// newSubscription - Create new TelemetrySubscription
func newTelemetrySubscription(ss *Session) *TelemetrySubscription {
	s := TelemetrySubscription{
		Prefix: nil, UseAliases: false, Mode: pb.SubscriptionList_STREAM,
		AllowAggregation: false, Encoding: pb.Encoding_JSON_IETF, UpdatesOnly: false,
		SubscribedEntity: []*SubscriptionEntity{}, isPolling: false, session: ss,
	}
	return &s
}

func (ss *Session) updateAliases(aliaslist []*pb.Alias) error {
	for _, alias := range aliaslist {
		name := alias.GetAlias()
		if !strings.HasPrefix(name, "#") {
			msg := fmt.Sprintf("invalid alias(Alias): Alias must start with '#'")
			return status.Error(codes.InvalidArgument, msg)
		}
		ss.alias[name] = alias
	}
	return nil
}

func (ss *Session) processSR(req *pb.SubscribeRequest, rchan chan *pb.SubscribeResponse) error {
	// SubscribeRequest for poll Subscription indication
	pollMode := req.GetPoll()
	if pollMode != nil {
		tsub := newTelemetrySubscription(ss)
		tsub.isPolling = true
		tsub.Mode = pb.SubscriptionList_POLL
		ss.server.addSubscription(tsub)
		return nil
	}
	// SubscribeRequest for aliases update
	aliases := req.GetAliases()
	if aliases != nil {
		// process client aliases
		aliaslist := aliases.GetAlias()
		return ss.updateAliases(aliaslist)
	}
	// extension := req.GetExtension()
	subscriptionList := req.GetSubscribe()
	if subscriptionList == nil {
		return status.Errorf(codes.InvalidArgument, "no subscribe(SubscriptionList)")
	}
	encoding := subscriptionList.GetEncoding()
	useModules := subscriptionList.GetUseModels()
	if err := ss.server.checkEncodingAndModel(encoding, useModules); err != nil {
		return err
	}
	mode := subscriptionList.GetMode()
	if mode == pb.SubscriptionList_ONCE || mode == pb.SubscriptionList_POLL {
		resps, err := ss.initTelemetryUpdate(req)
		for _, resp := range resps {
			rchan <- resp
		}
		return err
	}

	// prefix := subscriptionList.GetPrefix()
	// useAliases := subscriptionList.GetUseAliases()
	// allowAggregation = subscriptionList.GetAllowAggregation()
	// updatesOnly = subscriptionList.GetUpdatesOnly()

	// subList := subscriptionList.GetSubscription()
	// if subList == nil || len(subList) <= 0 {
	// 	err := fmt.Errorf("no subscription field(Subscription)")
	// 	return nil, status.Error(codes.InvalidArgument, err.Error())
	// }
	// for i, entity := range subList {
	// 	path := entity.GetPath()
	// 	submod := entity.GetMode()
	// 	sampleInterval := entity.GetSampleInterval()
	// 	supressRedundant := entity.GetSuppressRedundant()
	// 	heartBeatInterval := entity.GetHeartbeatInterval()
	// 	fmt.Println("SubscriptionList:", i, prefix, path, submod, sampleInterval, supressRedundant, heartBeatInterval)
	// 	subentity := SubscriptionEntity{
	// 		tsub: sub, Path: path, Mode: submod, SampleInterval: sampleInterval,
	// 		SuppressRedundant: supressRedundant, HeartbeatInterval: heartBeatInterval}
	// 	sub.SubscribedEntity = append(sub.SubscribedEntity, &subentity)
	// 	if mode == pb.SubscriptionList_STREAM {
	// 		if err := subentity.registerStreamSubscription(); err != nil {
	// 			return nil, err
	// 		}
	// 	}
	// }
	// // Build SubscribeResponses using the Subscription object
	// if mode == pb.SubscriptionList_ONCE {
	// 	subscribeResponses, err := sub.onceSubscription(request)
	// 	return subscribeResponses, err
	// } else if mode == pb.SubscriptionList_POLL {
	// 	sub.pollSubscription(request)
	// } else {
	// 	server.addSubscription(sub)
	// 	sub.streamSubscription(request)
	// }
	return nil
}
