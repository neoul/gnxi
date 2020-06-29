package gnmi

import (
	"time"

	"github.com/neoul/gnxi/utils"
	pb "github.com/openconfig/gnmi/proto/gnmi"
	"golang.org/x/net/context"
)

// StreamProtocol - The type of the subscription protocol
type StreamProtocol string

const (
	// StreamSSH - Stream subscription over SSH
	StreamSSH StreamProtocol = "STREAM_SSH"
	// StreamGRPC - Stream subscription over GRPC
	StreamGRPC StreamProtocol = "STREAM_GRPC"
	// StreamJSONRPC - Stream subscription over JSON RPC
	StreamJSONRPC StreamProtocol = "STREAM_JSON_RPC"
	// StreamThriftRPC - Stream subscription over ThriftRPC
	StreamThriftRPC StreamProtocol = "STREAM_THRIFT_RPC"
	// StreamWebsocketRPC - Stream subscription over WebsocketRPC
	StreamWebsocketRPC StreamProtocol = "STREAM_WEBSOCKET_RPC"
	// StreamUserDefined - Stream subscription over user-defined RPC
	StreamUserDefined StreamProtocol = "STREAM_USER_DEFINED_RPC"
)

// Encoding - The type of value encoding
type Encoding string

const (
	EncodingJSON     Encoding = "JSON"      // EncodingJSON - JSON encoded text.
	EncodingBYTES    Encoding = "BYTES"     // Arbitrarily encoded bytes.
	EncodingPROTO    Encoding = "PROTO"     // Encoded according to out-of-band agreed Protobuf.
	EncodingASCII    Encoding = "ASCII"     // ASCII text of an out-of-band agreed format.
	EncodingJSONIETF Encoding = "JSON_IETF" // JSON encoded text as per RFC7951.
)

// Subscription - gNMI subscription metadata control bock
type Subscription struct {
	Username             string    `json:"username,omitempty"`
	Password             string    `json:"password,omitempty"`
	GrpcVer              string    `json:"grpc-ver,omitempty"`
	ContentType          string    `json:"content-type,omitempty"`
	LoginTime            time.Time `json:"login-time,omitempty"`
	DestinationAddress   string    `json:"destination-address,omitempty"`
	DestinationPort      uint16    `json:"destination-port,omitempty"`
	Encoding             Encoding  `json:"encondig,omitempty"`
	encoding             pb.Encoding
	HeartbeatInterval    uint64         `json:"heartbeat-interval,omitempty"`
	ID                   uint64         `json:"id,omitempty"`
	OriginatedQosMarking uint8          `json:"originated-qos-marking,omitempty"`
	Protocol             StreamProtocol `json:"protocol,omitempty"`
	SampleInterval       uint64         `json:"sample-interval,omitempty"`
	SuppressRedundant    bool           `json:"suppress-redundant,omitempty"`
	// *pb.SubscribeRequest
}

var ids uint64
var subscribeList []*Subscription

func init() {
	subscribeList = []*Subscription{}
}

// addSubscription - Create new Subscription
func addSubscription(ctx context.Context) *Subscription {
	if ctx == nil {
		return nil
	}
	m, ok := utils.GetMetadata(ctx)
	if !ok {
		return nil
	}
	ids++
	username, _ := m["username"]
	password, _ := m["password"]
	userAgent, _ := m["user-agent"]
	contentType, _ := m["content-type"]
	authority, _ := m["authority"]
	s := Subscription{
		Username: username, Password: password,
		GrpcVer: userAgent, ContentType: contentType,
		LoginTime: time.Now(), DestinationAddress: authority,
		DestinationPort: 0, Encoding: EncodingJSONIETF,
		encoding: pb.Encoding_JSON_IETF, HeartbeatInterval: 0,
		ID: ids, OriginatedQosMarking: 0, Protocol: StreamGRPC,
		SampleInterval: 0, SuppressRedundant: false,
	}
	subscribeList = append(subscribeList, &s)
	return &s
}

// deleteSubscription - Delete the Subscription
func deleteSubscription(sub *Subscription) {
	for i, s := range subscribeList {
		if s == sub {
			subscribeList = append(subscribeList[:i], subscribeList[i+1:]...)
			break
		}
	}
}

// 0A ->  1B  -> C
// 0A ->  2D  -> C
// 0A ->  *  -> C
// 0A -> ... -> C
// 0A -> 10X -> 20Y -> C
