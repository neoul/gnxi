package gnmi

import (
	"fmt"
	"time"

	"github.com/neoul/gnxi/utils"
	pb "github.com/openconfig/gnmi/proto/gnmi"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// DynamicSubscriptionType - Subscription RPC (Request) Type
type DynamicSubscriptionType int

const (
	DynamicSubscriptionTypeUnknown DynamicSubscriptionType = iota
	DynamicSubscriptionTypePool
	DynamicSubscriptionTypeOnce
	DynamicSubscriptionTypeStream
)

var dynamicSubscriptionTypeStr = [...]string{
	"UNKOWN",
	"POOL",
	"ONCE",
	"STREAM",
}

func (s DynamicSubscriptionType) String() string { return dynamicSubscriptionTypeStr[s%4] }

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

// DynamicSubscription - Interface for all types of DynamicSubscription
type DynamicSubscription interface {
	IsDynamicSubscription()
}

var (
	dynamicSubscriptionID         uint64
	dynamicSubscriptionlist       []DynamicSubscription
	dynamicSubscriptioinAliaslist []*Alias
)

func init() {
	// [FIXME] dynamicSubscriptionlist and dynamicSubscriptioinAliaslist need to lock before use.
	dynamicSubscriptionlist = []DynamicSubscription{}
	dynamicSubscriptioinAliaslist = []*Alias{}
}

// DefaultDynamicSubscription - Default structure for DynamicSubscription
type DefaultDynamicSubscription struct {
	ID                 uint64                  `json:"id,omitempty"`
	Username           string                  `json:"username,omitempty"`
	Password           string                  `json:"password,omitempty"`
	GrpcVer            string                  `json:"grpc-ver,omitempty"`
	ContentType        string                  `json:"content-type,omitempty"`
	LoginTime          time.Time               `json:"login-time,omitempty"`
	DestinationAddress string                  `json:"destination-address,omitempty"`
	DestinationPort    uint16                  `json:"destination-port,omitempty"`
	Protocol           StreamProtocol          `json:"protocol,omitempty"`
	Type               DynamicSubscriptionType `json:"type,omitempty"`
}

// IsDynamicSubscription - used to check it is a DynamicSubscription
func (*DefaultDynamicSubscription) IsDynamicSubscription() {
}

// CreateSubscription - Create new DefaultDynamicSubscription
func CreateSubscription(ctx context.Context) *DefaultDynamicSubscription {
	if ctx == nil {
		return nil
	}
	m, ok := utils.GetMetadata(ctx)
	if !ok {
		return nil
	}
	dynamicSubscriptionID++
	username, _ := m["username"]
	password, _ := m["password"]
	userAgent, _ := m["user-agent"]
	contentType, _ := m["content-type"]
	authority, _ := m["authority"]
	s := DefaultDynamicSubscription{
		ID: dynamicSubscriptionID, Username: username, Password: password,
		GrpcVer: userAgent, ContentType: contentType, LoginTime: time.Now(),
		DestinationAddress: authority, DestinationPort: 0}
	s.Protocol = StreamGRPC
	s.Type = DynamicSubscriptionTypeUnknown
	dynamicSubscriptionlist = append(dynamicSubscriptionlist, &s)
	return &s
}

// DeleteSubscription - Delete the Subscription
func DeleteSubscription(subscription DynamicSubscription) {
	for i, s := range dynamicSubscriptionlist {
		if s == subscription {
			dynamicSubscriptionlist = append(dynamicSubscriptionlist[:i], dynamicSubscriptionlist[i+1:]...)
			break
		}
	}
}

// PollSubscription - used for Polling subscription.
type PollSubscription struct {
	DefaultDynamicSubscription
}

// PollSubscriptionInterface - used to check the Subscription is Poll subscription
type PollSubscriptionInterface interface {
	IsPollSubscription()
}

// IsDynamicSubscription - used to check it is a DynamicSubscription
func (*PollSubscription) IsDynamicSubscription() {
}

// IsPollSubscription - used to check the Subscription is a Poll Subscription
func (subscription *PollSubscription) IsPollSubscription() {
}

// CreatePollSubscription - Update the default Subscription to a Poll Subscription.
func CreatePollSubscription(sub *DefaultDynamicSubscription) *PollSubscription {
	ps := PollSubscription{}
	ps.DefaultDynamicSubscription = *sub
	ps.Type = DynamicSubscriptionTypePool
	for i, s := range dynamicSubscriptionlist {
		if s == sub {
			dynamicSubscriptionlist[i] = &ps
			break
		}
	}
	return &ps
}

// SubscriptionEntity - The entities of the Subscription
type SubscriptionEntity struct {
	Path              *pb.Path            `json:"path,omitempty"`            // The data tree path.
	Mode              pb.SubscriptionMode `json:"mode,omitempty"`            // Subscription mode to be used.
	SampleInterval    uint64              `json:"sample_interval,omitempty"` // ns between samples in SAMPLE mode.
	SuppressRedundant bool                `json:"suppress_redundant,omitempty"`
	HeartbeatInterval uint64              `json:"heartbeat_interval,omitempty"`
	// [FIXME]
	// 1. Ticker (Timer)
	// 2. keys (The path to the subscription data)
}

// StreamSubscription - Stream Subscription data structure
type StreamSubscription struct {
	DefaultDynamicSubscription
	Prefix           *pb.Path                 `json:"prefix,omitempty"` // Prefix used for paths.
	UseAliases       bool                     `json:"use_aliases,omitempty"`
	Qos              *pb.QOSMarking           `json:"qos,omitempty"` // DSCP marking to be used.
	Mode             pb.SubscriptionList_Mode `json:"mode,omitempty"`
	AllowAggregation bool                     `json:"allow_aggregation,omitempty"`
	UseModels        []*pb.ModelData          `json:"use_models,omitempty"`
	Encoding         pb.Encoding              `json:"encoding,omitempty"`
	UpdatesOnly      bool                     `json:"updates_only,omitempty"`
	Subscription     []*SubscriptionEntity    `json:"subscription,omitempty"` // Set of subscriptions to create.
}

// StreamSubscriptionInterface - used to check the Subscription is Stream subscription
type StreamSubscriptionInterface interface {
	IsStreamSubscription()
}

// IsDynamicSubscription - used to check it is a DynamicSubscription
func (*StreamSubscription) IsDynamicSubscription() {
}

// IsStreamSubscription - used to check the Subscription is a Stream Subscription
func (subscription *StreamSubscription) IsStreamSubscription() {
}

// CreateStreamSubscription - Update the default Subscription to a Stream Subscription.
func CreateStreamSubscription(subscription *DefaultDynamicSubscription, subscriptionList *pb.SubscriptionList) (*StreamSubscription, error) {
	prefix := subscriptionList.GetPrefix()
	allowAggregation := subscriptionList.GetAllowAggregation()
	updateOnly := subscriptionList.GetUpdatesOnly()
	encoding := subscriptionList.GetEncoding()
	qos := subscriptionList.GetQos()
	useAliases := subscriptionList.GetUseAliases()
	useModules := subscriptionList.GetUseModels()
	mode := subscriptionList.GetMode()
	// fmt.Println("Stream Subscription:", prefix, allowAggregation, updateOnly, encoding, mode, qos, useAliases, useModules)
	ss := StreamSubscription{}
	ss.DefaultDynamicSubscription = *subscription
	ss.Type = DynamicSubscriptionTypeStream
	ss.Prefix = prefix
	ss.AllowAggregation = allowAggregation
	ss.UpdatesOnly = updateOnly
	ss.Encoding = encoding
	ss.Qos = qos
	ss.UseAliases = useAliases
	ss.UseModels = useModules
	ss.Mode = mode

	subs := subscriptionList.GetSubscription()
	if subs == nil || len(subs) <= 0 {
		msg := fmt.Sprintf("no subscription field(Subscription)")
		return nil, status.Error(codes.InvalidArgument, msg)
	}
	for i, sub := range subs {
		path := sub.GetPath()
		submod := sub.GetMode()
		sampleInterval := sub.GetSampleInterval()
		supressRedundant := sub.GetSuppressRedundant()
		heartBeatInterval := sub.GetHeartbeatInterval()
		fmt.Println("Stream SubscriptionList:", i, path, submod, sampleInterval, supressRedundant, heartBeatInterval)
		se := SubscriptionEntity{
			Path: path, Mode: submod, SampleInterval: sampleInterval,
			SuppressRedundant: supressRedundant, HeartbeatInterval: heartBeatInterval}
		ss.Subscription = append(ss.Subscription, &se)
	}

	for i, s := range dynamicSubscriptionlist {
		if s == subscription {
			dynamicSubscriptionlist[i] = &ss
			break
		}
	}
	utils.PrintStruct(&ss, "LoginTime")
	return &ss, nil
}

// 0A ->  1B  -> C
// 0A ->  2D  -> C
// 0A ->  *  -> C
// 0A -> ... -> C
// 0A -> 10X -> 20Y -> C

// Alias - used to compress the complicated path.
type Alias struct {
	path *pb.Path
	name string
}

func UpdateAliases(dynamicSubscriptioinAliaslist *pb.AliasList) {
	for i, a := range dynamicSubscriptioinAliaslist.GetAlias() {
		fmt.Println("Alias:", i, a)
	}
}

// SubscribeResponse: # !!oneof
//   update(Notification):
//     timestamp(int64):
//     prefix(Path):
//     alias(string):
//     update(Update): # list
//     delete(Path): # list
//     atomic(boo): # no description how to use.
//   sync_response(bool):
//   error(Error): # deprecated, not used
//     ...

// reply init update
func replyInitUpdate(subscription *DynamicSubscription, request *pb.SubscribeRequest) (*pb.SubscribeResponse, error) {
	// send init update
	// send sync_response
	// close gRPC if ONCE Subscription

	return nil, nil
}

// reply update
func replyUpdate(request *pb.SubscribeRequest) (*pb.SubscribeResponse, error) {
	// send update after sync_response periodically
	return nil, nil
}
