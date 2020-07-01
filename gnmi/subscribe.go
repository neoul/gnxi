package gnmi

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/neoul/gnxi/utils"
	pb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmi/value"
	"github.com/openconfig/ygot/experimental/ygotutils"
	"github.com/openconfig/ygot/ygot"
	"golang.org/x/net/context"
	cpb "google.golang.org/genproto/googleapis/rpc/code"
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
	conn             *StreamConn
	server           *Server

	// // https://github.com/openconfig/gnmi/issues/45 - QoSMarking seems to be deprecated
	// Qos              *pb.QOSMarking           `json:"qos,omitempty"`          // DSCP marking to be used.
	// UseModels        []*pb.ModelData          `json:"use_models,omitempty"`   // (Check validate only in Request)
	// Alias            []*pb.Alias              `json:"alias,omitempty"`
}

// StreamConn - Connection information for Stream gRPC connection
type StreamConn struct {
	ID                 uint64          `json:"id,omitempty"`
	Username           string          `json:"username,omitempty"`
	Password           string          `json:"password,omitempty"`
	GrpcVer            string          `json:"grpc-ver,omitempty"`
	ContentType        string          `json:"content-type,omitempty"`
	LoginTime          time.Time       `json:"login-time,omitempty"`
	DestinationAddress string          `json:"destination-address,omitempty"`
	DestinationPort    uint16          `json:"destination-port,omitempty"`
	Protocol           StreamProtocol  `json:"protocol,omitempty"`
	Subscriptions      []*Subscription `json:"subscriptions,omitempty"`
	alias              map[string]*pb.Alias
	server             *Server
}

var (
	streamConnID   uint64
	streamConnList []*StreamConn
)

func init() {
	// [FIXME] subscriptionlist needs to lock before use.
	streamConnList = []*StreamConn{}
}

// newStreamConn - Create new Stream Connection
func newStreamConn(ctx context.Context, server *Server) *StreamConn {
	if ctx == nil {
		return nil
	}
	m, ok := utils.GetMetadata(ctx)
	if !ok {
		return nil
	}
	streamConnID++
	username, _ := m["username"]
	password, _ := m["password"]
	userAgent, _ := m["user-agent"]
	contentType, _ := m["content-type"]
	authority, _ := m["authority"]
	sconn := StreamConn{
		ID: streamConnID, Username: username, Password: password,
		GrpcVer: userAgent, ContentType: contentType, LoginTime: time.Now(),
		DestinationAddress: authority, DestinationPort: 0, Protocol: StreamGRPC,
		Subscriptions: []*Subscription{}, alias: map[string]*pb.Alias{},
		server: server,
	}
	streamConnList = append(streamConnList, &sconn)
	return &sconn
}

// deleteStreamConn - Delete the Stream Connection
func deleteStreamConn(sconn *StreamConn) {
	for i, s := range streamConnList {
		if s == sconn {
			streamConnList = append(streamConnList[:i], streamConnList[i+1:]...)
			break
		}
	}
}

func (sconn *StreamConn) addSubscription(sub *Subscription) {
	sconn.Subscriptions = append(sconn.Subscriptions, sub)
}

func (sconn *StreamConn) delSubscription(sub *Subscription) {
	for i, s := range sconn.Subscriptions {
		if s == sub {
			sconn.Subscriptions = append(sconn.Subscriptions[:i], sconn.Subscriptions[i+1:]...)
			break
		}
	}
}

// newSubscription - Create new Subscription
func newSubscription(server *Server) *Subscription {
	s := Subscription{
		Prefix: nil, UseAliases: false, Mode: pb.SubscriptionList_STREAM,
		AllowAggregation: false, Encoding: pb.Encoding_JSON_IETF, UpdatesOnly: false,
		SubscribedEntity: []*SubscriptionEntity{}, isPolling: false, conn: nil, server: server,
	}
	return &s
}

func (sub *Subscription) updateSubscription(sconn *StreamConn, request *pb.SubscribeRequest) ([]*pb.SubscribeResponse, error) {
	server := sub.server
	// Poll Subscription indication
	pollMode := request.GetPoll()
	if pollMode != nil {
		sub.isPolling = true
		sub.Mode = pb.SubscriptionList_POLL
		sconn.addSubscription(sub)
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
	if err := server.checkEncodingAndModel(encoding, useModules); err != nil {
		return nil, status.Error(codes.Unimplemented, err.Error())
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
			subscription: sub, Path: path, Mode: submod, SampleInterval: sampleInterval,
			SuppressRedundant: supressRedundant, HeartbeatInterval: heartBeatInterval}
		sub.SubscribedEntity = append(sub.SubscribedEntity, &subentity)
		if mode == pb.SubscriptionList_STREAM {
			err := subentity.registerStreamSubscription()
			if err != nil {
				return nil, status.Error(codes.InvalidArgument, err.Error())
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
		sconn.addSubscription(sub)
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
		sub.conn.alias[name] = alias
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
		fullPath := path
		if prefix != nil {
			fullPath = gnmiFullPath(prefix, path)
		}

		if fullPath.GetElem() == nil && fullPath.GetElement() != nil {
			return nil, status.Error(codes.Unimplemented, "deprecated path element")
		}
		fmt.Println("fullPath", fullPath)
		node, stat := ygotutils.GetNode(server.model.schemaTreeRoot, server.config, fullPath)
		if isNil(node) || stat.GetCode() != int32(cpb.Code_OK) {
			return nil, status.Errorf(codes.NotFound, "path %v not found", fullPath)
		}

		nodeStruct, ok := node.(ygot.GoStruct)
		// Return leaf node.
		if !ok {
			var val *pb.TypedValue
			switch kind := reflect.ValueOf(node).Kind(); kind {
			case reflect.Ptr, reflect.Interface:
				var err error
				val, err = value.FromScalar(reflect.ValueOf(node).Elem().Interface())
				if err != nil {
					msg := fmt.Sprintf("leaf node %v does not contain a scalar type value: %v", path, err)
					return nil, status.Error(codes.Internal, msg)
				}
			case reflect.Int64:
				enumMap, ok := server.model.enumData[reflect.TypeOf(node).Name()]
				if !ok {
					return nil, status.Error(codes.Internal, "not a GoStruct enumeration type")
				}
				val = &pb.TypedValue{
					Value: &pb.TypedValue_StringVal{
						StringVal: enumMap[reflect.ValueOf(node).Int()].Name,
					},
				}
			default:
				return nil, status.Errorf(codes.Internal, "unexpected kind of leaf node type: %v %v", node, kind)
			}
			update[i] = &pb.Update{Path: path, Val: val}
			continue
		}

		// Return IETF JSON by default.
		jsonEncoder := func() (map[string]interface{}, error) {
			return ygot.ConstructIETFJSON(nodeStruct, &ygot.RFC7951JSONConfig{AppendModuleName: true})
		}
		jsonType := "IETF"
		buildUpdate := func(b []byte) *pb.Update {
			return &pb.Update{Path: path, Val: &pb.TypedValue{Value: &pb.TypedValue_JsonIetfVal{JsonIetfVal: b}}}
		}

		if encoding == pb.Encoding_JSON {
			jsonEncoder = func() (map[string]interface{}, error) {
				return ygot.ConstructInternalJSON(nodeStruct)
			}
			jsonType = "Internal"
			buildUpdate = func(b []byte) *pb.Update {
				return &pb.Update{Path: path, Val: &pb.TypedValue{Value: &pb.TypedValue_JsonVal{JsonVal: b}}}
			}
		}

		jsonTree, err := jsonEncoder()
		if err != nil {
			msg := fmt.Sprintf("error in constructing %s JSON tree from requested node: %v", jsonType, err)
			return nil, status.Error(codes.Internal, msg)
		}

		jsonDump, err := json.Marshal(jsonTree)
		if err != nil {
			msg := fmt.Sprintf("error in marshaling %s JSON tree to bytes: %v", jsonType, err)
			return nil, status.Error(codes.Internal, msg)
		}
		update[i] = buildUpdate(jsonDump)
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

	return nil
}
