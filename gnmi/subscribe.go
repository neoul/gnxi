package gnmi

import (
	"fmt"
	"strings"
	"time"

	"github.com/neoul/gnxi/utils"
	pb "github.com/openconfig/gnmi/proto/gnmi"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// StreamProtocol - The type of the subscription protocol
type StreamProtocol int

const (
	// StreamUserDefined - Stream subscription over user-defined RPC
	StreamUserDefined StreamProtocol = iota
	// StreamSSH - Stream subscription over SSH
	StreamSSH
	// StreamGRPC - Stream subscription over GRPC
	StreamGRPC
	// StreamJSONRPC - Stream subscription over JSON RPC
	StreamJSONRPC
	// StreamThriftRPC - Stream subscription over ThriftRPC
	StreamThriftRPC
	// StreamWebsocketRPC - Stream subscription over WebsocketRPC
	StreamWebsocketRPC
)

var streamProtocolStr = [...]string{
	"STREAM_USER_DEFINED_RPC",
	"STREAM_SSH",
	"STREAM_GRPC",
	"STREAM_JSON_RPC",
	"STREAM_THRIFT_RPC",
	"STREAM_WEBSOCKET_RPC",
}

func (s StreamProtocol) String() string { return streamProtocolStr[s%5] }

// SubscriptionEntity - The entities of the Subscription
type SubscriptionEntity struct {
	Prefix            *pb.Path            `json:"prefix,omitempty"`          // Prefix used for paths.
	Path              *pb.Path            `json:"path,omitempty"`            // The data tree path.
	Mode              pb.SubscriptionMode `json:"mode,omitempty"`            // Subscription mode to be used.
	SampleInterval    uint64              `json:"sample_interval,omitempty"` // ns between samples in SAMPLE mode.
	SuppressRedundant bool                `json:"suppress_redundant,omitempty"`
	HeartbeatInterval uint64              `json:"heartbeat_interval,omitempty"`
	// [FIXME]
	// 1. Ticker (Timer)
	// 2. keys (The path to the subscription data)
}

// Subscription - Default structure for DynamicSubscription
type Subscription struct {
	ID                 uint64                   `json:"id,omitempty"`
	Username           string                   `json:"username,omitempty"`
	Password           string                   `json:"password,omitempty"`
	GrpcVer            string                   `json:"grpc-ver,omitempty"`
	ContentType        string                   `json:"content-type,omitempty"`
	LoginTime          time.Time                `json:"login-time,omitempty"`
	DestinationAddress string                   `json:"destination-address,omitempty"`
	DestinationPort    uint16                   `json:"destination-port,omitempty"`
	Protocol           StreamProtocol           `json:"protocol,omitempty"`
	UseAliases         bool                     `json:"use_aliases,omitempty"`
	Mode               pb.SubscriptionList_Mode `json:"mode,omitempty"`
	AllowAggregation   bool                     `json:"allow_aggregation,omitempty"`
	Encoding           pb.Encoding              `json:"encoding,omitempty"`
	UpdatesOnly        bool                     `json:"updates_only,omitempty"`
	Subscription       []*SubscriptionEntity    `json:"subscription,omitempty"` // Set of subscriptions to create.
	// Prefix             *pb.Path       `json:"prefix,omitempty"` // (moved to Subscription)
	// https://github.com/openconfig/gnmi/issues/45 - QoSMarking seems to be deprecated
	// Qos              *pb.QOSMarking           `json:"qos,omitempty"` // DSCP marking to be used.
	// UseModels        []*pb.ModelData          `json:"use_models,omitempty"` (Check validate only in Request)
	// Alias              []*pb.Alias              `json:"alias,omitempty"`
	alias     map[string]*pb.Alias
	isPolling bool
	server    *Server
}

var (
	subscriptionID   uint64
	subscriptionlist []*Subscription
)

func init() {
	// [FIXME] subscriptionlist needs to lock before use.
	subscriptionlist = []*Subscription{}
}

// newSubscription - Create new Subscription
func newSubscription(ctx context.Context, server *Server) *Subscription {
	if ctx == nil {
		return nil
	}
	m, ok := utils.GetMetadata(ctx)
	if !ok {
		return nil
	}
	subscriptionID++
	username, _ := m["username"]
	password, _ := m["password"]
	userAgent, _ := m["user-agent"]
	contentType, _ := m["content-type"]
	authority, _ := m["authority"]
	s := Subscription{
		ID: subscriptionID, Username: username, Password: password,
		GrpcVer: userAgent, ContentType: contentType, LoginTime: time.Now(),
		DestinationAddress: authority, DestinationPort: 0, Protocol: StreamGRPC,
		UseAliases: false, Mode: pb.SubscriptionList_STREAM,
		AllowAggregation: false, Encoding: pb.Encoding_JSON_IETF, UpdatesOnly: false,
		Subscription: []*SubscriptionEntity{}, alias: map[string]*pb.Alias{},
		isPolling: false, server: server,
	}
	// [FIXME] update server's aliases if server.useAliases is enabled.
	subscriptionlist = append(subscriptionlist, &s)
	return &s
}

func (sub *Subscription) updateSubscription(request *pb.SubscribeRequest) ([]*pb.SubscribeResponse, error) {
	server := sub.server
	// Poll Subscription indication
	pollMode := request.GetPoll()
	if pollMode != nil {
		sub.isPolling = true
		sub.Mode = pb.SubscriptionList_POLL
		// reset all Subscription data and timers
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
		msg := fmt.Sprintf("no subscribe field(SubscriptionList)")
		return nil, status.Error(codes.InvalidArgument, msg)
	}
	encoding := subscriptionList.GetEncoding()
	useModules := subscriptionList.GetUseModels()
	if useModules != nil {
		if err := server.checkEncodingAndModel(encoding, useModules); err != nil {
			return nil, status.Error(codes.Unimplemented, err.Error())
		}
	}
	prefix := subscriptionList.GetPrefix()
	mode := subscriptionList.GetMode()
	sub.Encoding = encoding
	sub.UseAliases = subscriptionList.GetUseAliases()
	sub.AllowAggregation = subscriptionList.GetAllowAggregation()
	sub.UpdatesOnly = subscriptionList.GetUpdatesOnly()
	subList := subscriptionList.GetSubscription()
	if subList == nil || len(subList) <= 0 {
		msg := fmt.Sprintf("no subscription field(Subscription)")
		return nil, status.Error(codes.InvalidArgument, msg)
	}
	for i, entity := range subList {
		path := entity.GetPath()
		submod := entity.GetMode()
		sampleInterval := entity.GetSampleInterval()
		supressRedundant := entity.GetSuppressRedundant()
		heartBeatInterval := entity.GetHeartbeatInterval()
		fmt.Println("SubscriptionList:", i, prefix, path, submod, sampleInterval, supressRedundant, heartBeatInterval)
		subentity := SubscriptionEntity{
			Prefix: prefix, Path: path, Mode: submod, SampleInterval: sampleInterval,
			SuppressRedundant: supressRedundant, HeartbeatInterval: heartBeatInterval}

		// SubscribeResponse per Subscription(Path) entity
		if mode == pb.SubscriptionList_ONCE {
			sub.onceSubscription(request)
		} else if mode == pb.SubscriptionList_POLL {
			sub.pollSubscription(request)
		} else {
			sub.Subscription = append(sub.Subscription, &subentity)
			sub.registerStreamSubscription(request)
			sub.streamSubscription(request)
		}
	}
	return nil, nil
}

// deleteSubscription - Delete the Subscription
func deleteSubscription(subscription *Subscription) {
	for i, s := range subscriptionlist {
		if s == subscription {
			subscriptionlist = append(subscriptionlist[:i], subscriptionlist[i+1:]...)
			break
		}
	}
}

func (sub *Subscription) updateClientAliases(aliases *pb.AliasList) error {
	aliaslist := aliases.GetAlias()
	for _, alias := range aliaslist {
		name := alias.GetAlias()
		if !strings.HasPrefix(name, "#") {
			msg := fmt.Sprintf("invalid alias(Alias): Alias must start with '#'")
			return status.Error(codes.InvalidArgument, msg)
		}
		sub.alias[name] = alias
	}
	return nil
}

func (sub *Subscription) updateSeverliases(aliases *pb.AliasList) error {
	return nil
}

func (sub *Subscription) onceSubscription(request *pb.SubscribeRequest) ([]*pb.SubscribeResponse, error) {
	// server := sub.server
	// subscriptionList := request.GetSubscribe()
	// subList := subscriptionList.GetSubscription()
	// prefix := subscriptionList.GetPrefix()
	// encoding := subscriptionList.GetEncoding()
	// updates := make([]*pb., len(subList))
	// for i, sentity := range subList {
	// 	path := sentity.GetPath()
	// 	fullPath := path
	// 	if prefix != nil {
	// 		fullPath = gnmiFullPath(prefix, path)
	// 	}
	// 	if fullPath.GetElem() == nil && fullPath.GetElement() != nil {
	// 		return nil, status.Error(codes.Unimplemented, "deprecated path element used")
	// 	}
	// 	node, stat := ygotutils.GetNode(server.model.schemaTreeRoot, server.config, fullPath)
	// 	if isNil(node) || stat.GetCode() != int32(cpb.Code_OK) {
	// 		return nil, status.Errorf(codes.NotFound, "path %v not found", fullPath)
	// 	}
	return nil, nil
}

func (sub *Subscription) pollSubscription(request *pb.SubscribeRequest) ([]*pb.SubscribeResponse, error) {

	return nil, nil
}

func (sub *Subscription) streamSubscription(request *pb.SubscribeRequest) ([]*pb.SubscribeResponse, error) {

	return nil, nil
}

func (sub *Subscription) registerStreamSubscription(request *pb.SubscribeRequest) ([]*pb.SubscribeResponse, error) {

	return nil, nil
}
