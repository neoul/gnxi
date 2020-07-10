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
	waitgroup *sync.WaitGroup
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
		waitgroup: new(sync.WaitGroup),
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

	// internal data
	_subscriptionMode  pb.SubscriptionMode
	_sampleInterval    uint64
	_suppressRedundant bool
	_heartbeatInterval uint64
	samplingTimer      *time.Ticker
	heartbeatTimer     *time.Ticker
	stop               chan struct{}
	isPolling          bool
	session            *TelemetrySession

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

func telemetryUpdate(
	teleses *TelemetrySession, telesub *TelemetrySubscription,
	samplingTimer *time.Ticker, shutdown chan struct{},
	stop chan struct{}, waitgroup *sync.WaitGroup,
	telemetrychannel chan *pb.SubscribeResponse,
) {
	defer waitgroup.Done()
	for {
		select {
		case <-samplingTimer.C:
			// Send TelemetryUpdate
			if samplingTimer == telesub.samplingTimer {
				log.Infof("tsession[%d].sub[%s].samplingTimer.expired", teleses.id, telesub.GetKey())
			} else {
				log.Infof("tsession[%d].sub[%s].heartbeatTimer.expired", teleses.id, telesub.GetKey())
			}
			resps, err := teleses.telemetryUpdate(telesub)
			if err != nil {
				return
			}
			for _, resp := range resps {
				telemetrychannel <- resp
			}
		case <-shutdown:
			log.Infof("tsession[%d].sub[%s].shutdown", teleses.id, telesub.GetKey())
			return
		case <-stop:
			log.Infof("tsession[%d].sub[%s].stopped", teleses.id, telesub.GetKey())
			return
		}
	}
}

// StartTelmetryUpdate - returns a key for telemetry comparison
func (teleses *TelemetrySession) StartTelmetryUpdate(telesub *TelemetrySubscription) error {
	teleses.lock()
	defer teleses.unlock()
	fmt.Println(telesub)
	if telesub.samplingTimer == nil && telesub._sampleInterval > 0 {
		tick := time.Duration(telesub._sampleInterval)
		telesub.samplingTimer = time.NewTicker(tick * time.Nanosecond)
		if telesub.samplingTimer == nil {
			return status.Errorf(codes.Internal, "sampling rate control failed")
		}
		teleses.waitgroup.Add(1)
		go telemetryUpdate(
			teleses, telesub, telesub.samplingTimer,
			teleses.shutdown, telesub.stop,
			teleses.waitgroup, teleses.channel)
	}
	if telesub.heartbeatTimer == nil && telesub._heartbeatInterval > 0 {
		tick := time.Duration(telesub._heartbeatInterval)
		telesub.heartbeatTimer = time.NewTicker(tick * time.Nanosecond)
		if telesub.heartbeatTimer == nil {
			return status.Errorf(codes.Internal, "sampling rate control failed")
		}
		teleses.waitgroup.Add(1)
		go telemetryUpdate(
			teleses, telesub, telesub.heartbeatTimer,
			teleses.shutdown, telesub.stop,
			teleses.waitgroup, teleses.channel)
	}
	return nil
}

// StopTelemetryUpdate - returns a key for telemetry comparison
func (teleses *TelemetrySession) StopTelemetryUpdate(telesub *TelemetrySubscription) error {
	teleses.lock()
	defer teleses.unlock()
	closeSignal := false
	if telesub.samplingTimer != nil || telesub.heartbeatTimer != nil {
		telesub.samplingTimer.Stop()
		telesub.samplingTimer = nil
		closeSignal = true
	}
	if telesub.heartbeatTimer != nil {
		telesub.heartbeatTimer.Stop()
		telesub.heartbeatTimer = nil
		closeSignal = true
	}
	if closeSignal {
		close(telesub.stop)
	}
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

// initTelemetryUpdate - Process and generate responses for a init update.
func (teleses *TelemetrySession) telemetryUpdate(telesub *TelemetrySubscription) ([]*pb.SubscribeResponse, error) {
	telesub.session.rlock()
	defer telesub.session.runlock()
	s := teleses.server

	prefix := telesub.Prefix
	encoding := telesub.Encoding
	useAliases := telesub.UseAliases
	mode := telesub.StreamingMode
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
	update := make([]*pb.Update, len(telesub.Paths))
	for i, path := range telesub.Paths {
		// Get schema node for path from config struct.
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

	return buildSubscribeResponse(prefix, alias, update, *disableBundling, false)
}

func newTelemetrySubscription(
	prefix *pb.Path, useAliases bool, streamingMode pb.SubscriptionList_Mode, allowAggregation bool,
	encoding pb.Encoding, paths []*pb.Path, subscriptionMode pb.SubscriptionMode,
	sampleInterval uint64, suppressRedundant bool, heartbeatInterval uint64) (*TelemetrySubscription, error) {

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
		// internal data
		// _subscriptionMode:  _subscriptionMode,
		// _sampleInterval:    _sampleInterval,
		// _suppressRedundant: _suppressRedundant,
		// _heartbeatInterval: _heartbeatInterval,
	}
	if streamingMode == pb.SubscriptionList_POLL {
		telesub.isPolling = true
		telesub.Paths = []*pb.Path{}
		telesub.Prefix = nil
	}
	// 3.5.1.5.2 STREAM Subscriptions Must be satisfied for telemetry update starting.
	switch telesub.SubscriptionMode {
	case pb.SubscriptionMode_TARGET_DEFINED:
		// vendor specific mode
		telesub._subscriptionMode = pb.SubscriptionMode_ON_CHANGE
		telesub._sampleInterval = 0
		telesub._suppressRedundant = false
		telesub._heartbeatInterval = 60000000000 // 60sec
	case pb.SubscriptionMode_ON_CHANGE:
		if telesub.HeartbeatInterval < 1000000000 && telesub.HeartbeatInterval != 0 {
			return nil, status.Errorf(codes.InvalidArgument,
				"heartbeat_interval(< 1sec) is not supported")
		}
		telesub._subscriptionMode = pb.SubscriptionMode_ON_CHANGE
		telesub._sampleInterval = 0
		telesub._suppressRedundant = false
		telesub._heartbeatInterval = telesub.HeartbeatInterval

	case pb.SubscriptionMode_SAMPLE:
		if telesub.SampleInterval < 1000000000 && telesub.SampleInterval != 0 {
			return nil, status.Errorf(codes.InvalidArgument,
				"sample_interval(< 1sec) is not supported")
		}
		if telesub.HeartbeatInterval < 1000000000 && telesub.HeartbeatInterval != 0 {
			return nil, status.Errorf(codes.InvalidArgument,
				"heartbeat_interval(< 1sec) is not supported")
		}
		telesub._subscriptionMode = pb.SubscriptionMode_SAMPLE
		telesub._sampleInterval = telesub.SampleInterval
		if telesub.SampleInterval == 0 {
			// Set minimal sampling interval (1sec)
			telesub._sampleInterval = 1000000000
		}
		telesub._suppressRedundant = telesub.SuppressRedundant
		telesub._heartbeatInterval = telesub.HeartbeatInterval
	}
	return &telesub, nil
}

// addSubscription - Create new TelemetrySubscription
func (teleses *TelemetrySession) addSubscription(newsub *TelemetrySubscription) *TelemetrySubscription {
	teleses.lock()
	defer teleses.unlock()
	key := newsub.GetKey()
	if t, ok := teleses.telesub[key]; ok {
		// only add the path
		log.Infof("tsession[%d].sub[%s].append(%s)", teleses.id, key, xpath.ToXPATH(newsub.Paths[0]))
		t.Paths = append(t.Paths, newsub.Paths...)
		newsub = t
	} else {
		log.Infof("tsession[%d].sub[%s].add(%s)", teleses.id, key, xpath.ToXPATH(newsub.Paths[0]))
		newsub.session = teleses
		teleses.telesub[key] = newsub
	}
	return newsub
}

// addPollingSubscription - Create new TelemetrySubscription
func (teleses *TelemetrySession) addPollingSubscription() error {
	teleses.lock()
	defer teleses.unlock()
	telesub := TelemetrySubscription{
		StreamingMode: pb.SubscriptionList_POLL,
		Paths:         []*pb.Path{},
		isPolling:     true,
		session:       teleses,
	}
	key := telesub.GetKey()
	teleses.telesub[key] = &telesub
	log.Infof("tsession[%d].sub[%s].add(polling)", teleses.id, key)
	return nil
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
		return teleses.addPollingSubscription()
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
		_sampleInterval := updateEntry.GetSampleInterval()
		supressRedundant := updateEntry.GetSuppressRedundant()
		heartBeatInterval := updateEntry.GetHeartbeatInterval()
		telesub, err := newTelemetrySubscription(
			prefix, useAliases, pb.SubscriptionList_STREAM,
			allowAggregation, encoding, []*pb.Path{path}, submod,
			_sampleInterval, supressRedundant, heartBeatInterval)
		if err != nil {
			return err
		}
		telesub = teleses.addSubscription(telesub)
		if err != nil {
			return err
		}
		err = teleses.StartTelmetryUpdate(telesub)
		if err != nil {
			return err
		}
	}
	return nil
}
