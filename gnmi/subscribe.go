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
	subscription      *Subscription
	// [FIXME]
	// 1. Ticker (Timer)
	// 2. keys (The path to the subscription data)
}

// Subscription - Default structure for DynamicSubscription
type Subscription struct {
	Prefix           *pb.Path                 `json:"prefix,omitempty"`
	UseAliases       bool                     `json:"use_aliases,omitempty"`
	Mode             pb.SubscriptionList_Mode `json:"mode,omitempty"`
	AllowAggregation bool                     `json:"allow_aggregation,omitempty"`
	Encoding         pb.Encoding              `json:"encoding,omitempty"`
	UpdatesOnly      bool                     `json:"updates_only,omitempty"`
	SubscribedEntity []*SubscriptionEntity    `json:"subscribed_entity,omitempty"` // Set of subscriptions to create.
	isPolling        bool
	session          *Session
	server           *Server

	// // https://github.com/openconfig/gnmi/issues/45 - QoSMarking seems to be deprecated
	// Qos              *pb.QOSMarking           `json:"qos,omitempty"`          // DSCP marking to be used.
	// UseModels        []*pb.ModelData          `json:"use_models,omitempty"`   // (Check validate only in Request)
	// Alias            []*pb.Alias              `json:"alias,omitempty"`
}

func (s *Server) addSubscription(sub *Subscription) {
	s.Subscriptions = append(s.Subscriptions, sub)
}

func (s *Server) delSubscription(sub *Subscription) {
	for i, se := range s.Subscriptions {
		if se == sub {
			s.Subscriptions =
				append(s.Subscriptions[:i], s.Subscriptions[i+1:]...)
			break
		}
	}
}

// newSubscription - Create new Subscription
func newSubscription(session *Session) *Subscription {
	s := Subscription{
		Prefix: nil, UseAliases: false, Mode: pb.SubscriptionList_STREAM,
		AllowAggregation: false, Encoding: pb.Encoding_JSON_IETF, UpdatesOnly: false,
		SubscribedEntity: []*SubscriptionEntity{}, isPolling: false, session: session,
		server: session.server,
	}
	return &s
}

func (sub *Subscription) handleSubscription(request *pb.SubscribeRequest) ([]*pb.SubscribeResponse, error) {
	server := sub.server
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
			subscription: sub, Path: path, Mode: submod, SampleInterval: sampleInterval,
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
		subscribeResponses, err := sub.onceSubscription(request)
		return subscribeResponses, err
	} else if mode == pb.SubscriptionList_POLL {
		sub.pollSubscription(request)
	} else {
		server.addSubscription(sub)
		sub.streamSubscription(request)
	}
	return nil, nil
}

func (sub *Subscription) updateClientAliases(aliases *pb.AliasList) error {
	aliaslist := aliases.GetAlias()
	for _, alias := range aliaslist {
		name := alias.GetAlias()
		if !strings.HasPrefix(name, "#") {
			msg := fmt.Sprintf("invalid alias(Alias): Alias must start with '#'")
			return status.Error(codes.InvalidArgument, msg)
		}
		sub.server.alias[name] = alias
	}
	return nil
}

func (sub *Subscription) updateSeverliases(aliases *pb.AliasList) error {
	return nil
}

func (sub *Subscription) onceSubscription(request *pb.SubscribeRequest) ([]*pb.SubscribeResponse, error) {
	server := sub.server
	alias := ""
	prefix := sub.Prefix
	encoding := sub.Encoding
	timepstamp := time.Now().UnixNano()
	// if sub.UseAliases {
	// 	prefix = nil
	// }
	update := make([]*pb.Update, len(sub.SubscribedEntity))
	for i, subentity := range sub.SubscribedEntity {
		// Get schema node for path from config struct.
		path := subentity.Path
		fullPath := utils.GetGNMIFullPath(prefix, path)
		if fullPath.GetElem() == nil && fullPath.GetElement() != nil {
			return nil, status.Error(codes.Unimplemented, "deprecated path element")
		}
		node, stat := ygotutils.GetNode(server.model.schemaTreeRoot, server.config, fullPath)
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
		Timestamp: timepstamp,
		Prefix:    prefix,
		Alias:     alias,
		Update:    update,
	}

	updates := []*pb.SubscribeResponse{
		{Response: &pb.SubscribeResponse_Update{
			Update: &notification,
		}},
	}
	return updates, nil
}

func (sub *Subscription) pollSubscription(request *pb.SubscribeRequest) ([]*pb.SubscribeResponse, error) {
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

func (sub *Subscription) streamSubscription(request *pb.SubscribeRequest) ([]*pb.SubscribeResponse, error) {

	return nil, nil
}

func (sentity *SubscriptionEntity) registerStreamSubscription() error {
	// status.Error(codes.InvalidArgument, err.Error())
	return nil
}
