package gnmi

import (
	"fmt"
	"strings"
	"sync"
	"time"

	log "github.com/golang/glog"
	"github.com/neoul/gnxi/utils/xpath"
	pb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ygot/experimental/ygotutils"
	"github.com/openconfig/ygot/ygot"
	cpb "google.golang.org/genproto/googleapis/rpc/code"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TelemetrySession - gNMI gRPC Subscribe RPC (Telemetry) session information managed by server
type TelemetrySession struct {
	id        uint
	telesub   map[string]*TelemetrySubscription
	channel   chan *pb.SubscribeResponse
	shutdown  chan struct{}
	waitgroup sync.WaitGroup
	alias     map[string]*pb.Alias
	mutex     sync.RWMutex
	server    *Server
}

var (
	telesesID uint
)

func (teleses *TelemetrySession) lock() {
	teleses.mutex.Lock()
}

func (teleses *TelemetrySession) unlock() {
	teleses.mutex.Unlock()
}

func (teleses *TelemetrySession) rlock() {
	teleses.mutex.RLock()
}

func (teleses *TelemetrySession) runlock() {
	teleses.mutex.RUnlock()
}

func newTelemetrySession(s *Server) *TelemetrySession {
	telesesID++
	return &TelemetrySession{
		id:        telesesID,
		telesub:   map[string]*TelemetrySubscription{},
		channel:   make(chan *pb.SubscribeResponse, 256),
		shutdown:  make(chan struct{}),
		waitgroup: sync.WaitGroup{},
		alias:     map[string]*pb.Alias{},
		server:    s,
	}
}

// TelemetrySubscription - Default structure for Telemetry Update Subscription
type TelemetrySubscription struct {
	Prefix            *pb.Path                 `json:"prefix,omitempty"`
	UseAliases        bool                     `json:"use_aliases,omitempty"`
	StreamingMode     pb.SubscriptionList_Mode `json:"stream_mode,omitempty"`
	AllowAggregation  bool                     `json:"allow_aggregation,omitempty"`
	Encoding          pb.Encoding              `json:"encoding,omitempty"`
	Paths             []*pb.Path               `json:"path,omitempty"`              // The data tree path.
	SubscriptionMode  pb.SubscriptionMode      `json:"subscription_mode,omitempty"` // Subscription mode to be used.
	SampleInterval    uint64                   `json:"sample_interval,omitempty"`   // ns between samples in SAMPLE mode.
	SuppressRedundant bool                     `json:"suppress_redundant,omitempty"`
	HeartbeatInterval uint64                   `json:"heartbeat_interval,omitempty"`
	Duplicates        uint32                   `json:"duplicates,omitempty"` // Number of coalesced duplicates.

	ticker    *time.Ticker
	stop      chan bool
	isPolling bool
	session   *TelemetrySession

	// // https://github.com/openconfig/gnmi/issues/45 - QoSMarking seems to be deprecated
	// Qos              *pb.QOSMarking           `json:"qos,omitempty"`          // DSCP marking to be used.
	// UseModels        []*pb.ModelData          `json:"use_models,omitempty"`   // (Check validate only in Request)
	// Alias            []*pb.Alias              `json:"alias,omitempty"`
	// UpdatesOnly       bool                     `json:"updates_only,omitempty"` // not required to store
	// [FIXME]
	// 1. Ticker (Timer)
	// 2. keys (The path to the subscription data)
}

// GetKey - returns a key for telemetry comparison
func (telesub *TelemetrySubscription) GetKey() string {
	return fmt.Sprintf("%s-%s-%s-%s-%d-%d-%t-%t-%t",
		xpath.ToXPATH(telesub.Prefix), telesub.StreamingMode, telesub.Encoding,
		telesub.SubscriptionMode, telesub.SampleInterval, telesub.HeartbeatInterval,
		telesub.UseAliases, telesub.AllowAggregation, telesub.SuppressRedundant,
	)
}

// StartTelmetryUpdate - returns a key for telemetry comparison
func (telesub *TelemetrySubscription) StartTelmetryUpdate() error {
	teleses := telesub.session
	teleses.lock()
	defer teleses.unlock()

	if telesub.SubscriptionMode == pb.SubscriptionMode_SAMPLE ||
		telesub.SubscriptionMode == pb.SubscriptionMode_TARGET_DEFINED {
		if telesub.ticker == nil {
			interval := telesub.SampleInterval
			if telesub.SampleInterval == 0 {
				// Set minimal sampling interval
				interval = 1000000000
			}
			if telesub.SubscriptionMode == pb.SubscriptionMode_TARGET_DEFINED {
				interval = 5000000000
			}
			if interval < 1000000000 {
				return status.Errorf(codes.InvalidArgument, "sample_interval under 1sec is not supported")
			}
			tick := time.Duration(interval)
			telesub.ticker = time.NewTicker(tick * time.Nanosecond)
			teleses.waitgroup.Add(1)
			go func(telesub *TelemetrySubscription) {
				teleses := telesub.session
				defer teleses.waitgroup.Done()
				for {
					select {
					case <-telesub.ticker.C:
						telesub.session.rlock()
						// Send TelemetryUpdate
						log.Infof("telemetry-session[%d] subscription(%s) expired", teleses.id, telesub.GetKey())
						telesub.session.runlock()
					case <-telesub.session.shutdown:
						telesub.ticker = nil
						return
					case <-telesub.stop:
						telesub.ticker = nil
						return
					}
				}
			}(telesub)
		}
	}
	return nil
}

// StopTelemetryUpdate - returns a key for telemetry comparison
func (telesub *TelemetrySubscription) StopTelemetryUpdate() error {
	teleses := telesub.session
	teleses.lock()
	defer teleses.unlock()
	if telesub.ticker != nil {
		telesub.ticker.Stop()
		telesub.stop <- true
	}
	return nil
}

func (telesub *TelemetrySubscription) updateClientAliases(aliases *pb.AliasList) error {
	teleses := telesub.session
	teleses.lock()
	defer teleses.unlock()
	aliaslist := aliases.GetAlias()
	for _, alias := range aliaslist {
		name := alias.GetAlias()
		if !strings.HasPrefix(name, "#") {
			msg := fmt.Sprintf("invalid alias(Alias): Alias must start with '#'")
			return status.Error(codes.InvalidArgument, msg)
		}
		teleses.server.alias[name] = alias
	}
	return nil
}

func (telesub *TelemetrySubscription) updateSeverliases(aliases *pb.AliasList) error {
	return nil
}

// initTelemetryUpdate - Process and generate responses for a init update.
func (teleses *TelemetrySession) initTelemetryUpdate(req *pb.SubscribeRequest) ([]*pb.SubscribeResponse, error) {
	s := teleses.server
	subscriptionList := req.GetSubscribe()
	subList := subscriptionList.GetSubscription()
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
	s.mu.RLock()
	s.dataBlock.Lock()
	defer s.mu.RUnlock()
	defer s.dataBlock.Unlock()
	update := make([]*pb.Update, len(subList))
	for i, updateEntry := range subList {
		// Get schema node for path from config struct.
		path := updateEntry.Path
		fullPath := xpath.GNMIFullPath(prefix, path)
		if fullPath.GetElem() == nil && fullPath.GetElement() != nil {
			return nil, status.Error(codes.Unimplemented, "deprecated path element used")
		}
		// fmt.Println("path:::", xpath.ToXPATH(fullPath))
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

	return buildSubscribeResponse(prefix, alias, update, *disableBundling, true)
}

func newTelemetrySubscription(
	prefix *pb.Path, useAliases bool, streamingMode pb.SubscriptionList_Mode, allowAggregation bool,
	encoding pb.Encoding, paths []*pb.Path, subscriptionMode pb.SubscriptionMode,
	sampleInterval uint64, suppressRedundant bool, heartbeatInterval uint64,
) *TelemetrySubscription {

	telesub := TelemetrySubscription{
		Prefix:            prefix,
		UseAliases:        useAliases,
		StreamingMode:     streamingMode,
		AllowAggregation:  allowAggregation,
		Encoding:          encoding,
		Paths:             paths,
		SubscriptionMode:  subscriptionMode,
		SampleInterval:    sampleInterval,
		SuppressRedundant: suppressRedundant,
		HeartbeatInterval: heartbeatInterval,
	}
	if streamingMode == pb.SubscriptionList_POLL {
		telesub.isPolling = true
		telesub.Paths = []*pb.Path{}
		telesub.Prefix = nil
	}
	return &telesub
}

// addSubscription - Create new TelemetrySubscription
func (teleses *TelemetrySession) addSubscription(telesub *TelemetrySubscription) (*TelemetrySubscription, error) {
	key := telesub.GetKey()
	teleses.lock()
	defer teleses.unlock()
	if t, ok := teleses.telesub[key]; ok {
		// only add the path
		log.Infof("telemetry-session[%d] update subscription(%s)", teleses.id, key)
		t.Paths = append(t.Paths, telesub.Paths...)
		telesub = t
	} else {
		log.Infof("telemetry-session[%d] add subscription(%s)", teleses.id, key)
		telesub.session = teleses
		teleses.telesub[key] = telesub
	}
	return telesub, nil
}

// addPollingSubscription - Create new TelemetrySubscription
func (teleses *TelemetrySession) addPollingSubscription() {
	teleses.lock()
	defer teleses.unlock()
	telesub := TelemetrySubscription{
		StreamingMode: pb.SubscriptionList_POLL,
		Paths:         []*pb.Path{},
		isPolling:     true,
		session:       teleses,
	}
	teleses.telesub[telesub.GetKey()] = &telesub
}

func (teleses *TelemetrySession) updateAliases(aliaslist []*pb.Alias) error {
	teleses.lock()
	defer teleses.unlock()
	for _, alias := range aliaslist {
		name := alias.GetAlias()
		if !strings.HasPrefix(name, "#") {
			msg := fmt.Sprintf("invalid alias(Alias): Alias must start with '#'")
			return status.Error(codes.InvalidArgument, msg)
		}
		teleses.alias[name] = alias
	}
	return nil
}

func processSR(teleses *TelemetrySession, req *pb.SubscribeRequest) error {
	// SubscribeRequest for poll Subscription indication
	pollMode := req.GetPoll()
	if pollMode != nil {
		teleses.addPollingSubscription()
		return nil
	}
	// SubscribeRequest for aliases update
	aliases := req.GetAliases()
	if aliases != nil {
		// process client aliases
		aliaslist := aliases.GetAlias()
		return teleses.updateAliases(aliaslist)
	}
	// extension := req.GetExtension()
	subscriptionList := req.GetSubscribe()
	if subscriptionList == nil {
		return status.Errorf(codes.InvalidArgument, "no subscribe(SubscriptionList)")
	}
	subList := subscriptionList.GetSubscription()
	subListLength := len(subList)
	if subList == nil || subListLength <= 0 {
		err := fmt.Errorf("no subscription field(Subscription)")
		return status.Error(codes.InvalidArgument, err.Error())
	}
	encoding := subscriptionList.GetEncoding()
	useModules := subscriptionList.GetUseModels()
	if err := teleses.server.checkEncodingAndModel(encoding, useModules); err != nil {
		return err
	}
	resps, err := teleses.initTelemetryUpdate(req)
	for _, resp := range resps {
		teleses.channel <- resp
	}
	mode := subscriptionList.GetMode()
	if mode == pb.SubscriptionList_ONCE ||
		mode == pb.SubscriptionList_POLL ||
		err != nil {
		return err
	}

	prefix := subscriptionList.GetPrefix()
	useAliases := subscriptionList.GetUseAliases()
	allowAggregation := subscriptionList.GetAllowAggregation()
	for _, updateEntry := range subList {
		path := updateEntry.GetPath()
		submod := updateEntry.GetMode()
		sampleInterval := updateEntry.GetSampleInterval()
		supressRedundant := updateEntry.GetSuppressRedundant()
		heartBeatInterval := updateEntry.GetHeartbeatInterval()
		telesub := newTelemetrySubscription(
			prefix, useAliases, pb.SubscriptionList_STREAM,
			allowAggregation, encoding, []*pb.Path{path}, submod,
			sampleInterval, supressRedundant, heartBeatInterval)
		telesub, err := teleses.addSubscription(telesub)
		if err != nil {
			return err
		}
		err = telesub.StartTelmetryUpdate()
		if err != nil {
			return err
		}
	}
	return nil
}
