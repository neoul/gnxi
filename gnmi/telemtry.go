package gnmi

import (
	"fmt"
	"strings"
	"sync"
	"time"

	log "github.com/golang/glog"
	"github.com/neoul/gnxi/gnmi/model"
	"github.com/neoul/gnxi/utils"
	"github.com/neoul/gnxi/utils/xpath"
	pb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ygot/ygot"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type telemetryID uint64

type telemetryCB struct {
	// map[uint]*telemetrySubscription: uint = is subscription.id
	lookupTeleSub   map[string]map[telemetryID]*telemetrySubscription
	onchangeMerged  map[telemetryID]*telemetrySubscription
	onchangeDeleted map[telemetryID][]*string
	mutex           sync.RWMutex
}

func newTelemetryCB() *telemetryCB {
	return &telemetryCB{
		lookupTeleSub:   map[string]map[telemetryID]*telemetrySubscription{},
		onchangeMerged:  map[telemetryID]*telemetrySubscription{},
		onchangeDeleted: map[telemetryID][]*string{},
	}
}

func (tcb *telemetryCB) registerTelemetry(m *model.Model, telesub *telemetrySubscription) error {
	tcb.mutex.Lock()
	defer tcb.mutex.Unlock()
	for _, path := range telesub.Paths {
		fullpath := utils.GNMIFullPath(telesub.Prefix, path)
		allpaths, ok := m.FindAllPaths(fullpath)
		if ok {
			for _, p := range allpaths {
				subgroup, ok := tcb.lookupTeleSub[p]
				if !ok || subgroup == nil {
					tcb.lookupTeleSub[p] = map[telemetryID]*telemetrySubscription{}
					subgroup = tcb.lookupTeleSub[p]
				}
				subgroup[telesub.id] = telesub
			}
		}
	}
	fmt.Println(tcb)
	return nil
}

func (tcb *telemetryCB) unregisterTelemetry(telesub *telemetrySubscription) {
	tcb.mutex.Unlock()
	defer tcb.mutex.Unlock()
	for _, subgroup := range tcb.lookupTeleSub {
		_, ok := subgroup[telesub.id]
		if ok {
			delete(subgroup, telesub.id)
		}
	}
}

// OnChangeStarted - callback for Telemetry subscription on data changes
func (tcb *telemetryCB) OnChangeStarted(changes ygot.GoStruct) {
	tcb.mutex.RLock()
	defer tcb.mutex.RUnlock()
}

// OnChangeCreated - callback for Telemetry subscription on data changes
func (tcb *telemetryCB) OnChangeCreated(path []string, changes ygot.GoStruct) {
	tcb.mutex.RLock()
	defer tcb.mutex.RUnlock()
	for i := len(path); i >= 0; i-- {
		path := "/" + strings.Join(path[:i], "/")
		// fmt.Println(path)
		subgroup, ok := tcb.lookupTeleSub[path]
		if ok {
			for _, telesub := range subgroup {
				if telesub.isPolling {
					continue
				}
				fmt.Println("OnCreated run:", telesub.id)
				tcb.onchangeMerged[telesub.id] = telesub
			}
		}
	}
}

// OnChangeReplaced - callback for Telemetry subscription on data changes
func (tcb *telemetryCB) OnChangeReplaced(path []string, changes ygot.GoStruct) {
	tcb.mutex.RLock()
	defer tcb.mutex.RUnlock()
	for i := len(path); i >= 0; i-- {
		path := "/" + strings.Join(path[:i], "/")
		// fmt.Println(path)
		subgroup, ok := tcb.lookupTeleSub[path]
		if ok {
			for _, telesub := range subgroup {
				if telesub.isPolling {
					continue
				}
				fmt.Println("OnReplaced run:", telesub.id)
				tcb.onchangeMerged[telesub.id] = telesub
			}
		}
	}
}

// OnChangeDeleted - callback for Telemetry subscription on data changes
func (tcb *telemetryCB) OnChangeDeleted(path []string) {
	tcb.mutex.RLock()
	defer tcb.mutex.RUnlock()
	for i := len(path); i >= 0; i-- {
		path := "/" + strings.Join(path[:i], "/")
		// fmt.Println(path)
		subgroup, ok := tcb.lookupTeleSub[path]
		if ok {
			for _, telesub := range subgroup {
				if telesub.isPolling {
					continue
				}
				fmt.Println("OnReplaced run:", telesub.id)
				deletedlist := tcb.onchangeDeleted[telesub.id]
				if deletedlist == nil {
					tcb.onchangeDeleted[telesub.id] = []*string{&path}
				} else {
					tcb.onchangeDeleted[telesub.id] = append(deletedlist, &path)
				}
			}
		}
	}
}

// OnStarted - callback for Telemetry subscription on data changes
func (tcb *telemetryCB) OnChangeFinished(changes ygot.GoStruct) {
	tcb.mutex.RLock()
	defer tcb.mutex.RUnlock()
	for _, telesub := range tcb.onchangeMerged {
		telesub.Duplicates++
	}
}

// telemetrySession - gNMI gRPC Subscribe RPC (Telemetry) session information managed by server
type telemetrySession struct {
	id        telemetryID
	telesub   map[string]*telemetrySubscription
	channel   chan *pb.SubscribeResponse
	shutdown  chan struct{}
	waitgroup *sync.WaitGroup
	alias     map[string]*pb.Alias
	mutex     sync.RWMutex
	server    *Server
}

var (
	sessID telemetryID
	subID  telemetryID
)

func (teleses *telemetrySession) lock() {
	teleses.mutex.Lock()
}

func (teleses *telemetrySession) unlock() {
	teleses.mutex.Unlock()
}

func (teleses *telemetrySession) rlock() {
	teleses.mutex.RLock()
}

func (teleses *telemetrySession) runlock() {
	teleses.mutex.RUnlock()
}

func newTelemetrySession(s *Server) *telemetrySession {
	sessID++
	return &telemetrySession{
		id:        sessID,
		telesub:   map[string]*telemetrySubscription{},
		channel:   make(chan *pb.SubscribeResponse, 256),
		shutdown:  make(chan struct{}),
		waitgroup: new(sync.WaitGroup),
		alias:     map[string]*pb.Alias{},
		server:    s,
	}
}

func (teleses *telemetrySession) stopTelemetrySession() {
	teleses.lock()
	defer teleses.unlock()
	for _, telesub := range teleses.telesub {
		teleses.server.unregisterTelemetry(telesub)
	}
	close(teleses.shutdown)
	teleses.waitgroup.Wait()
}

// telemetrySubscription - Default structure for Telemetry Update Subscription
type telemetrySubscription struct {
	id                telemetryID
	sessionid         telemetryID
	key               *string
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
	session            *telemetrySession

	// // https://github.com/openconfig/gnmi/issues/45 - QoSMarking seems to be deprecated
	// Qos              *pb.QOSMarking           `json:"qos,omitempty"`          // DSCP marking to be used.
	// UseModels        []*pb.ModelData          `json:"use_models,omitempty"`   // (Check validate only in Request)
	// Alias            []*pb.Alias              `json:"alias,omitempty"`
	// UpdatesOnly       bool                     `json:"updates_only,omitempty"` // not required to store
	// [FIXME]
	// 1. Ticker (Timer)
	// 2. keys (The path to the subscription data)
}

func goTelemetryUpdate(
	teleses *telemetrySession, telesub *telemetrySubscription,
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
				log.Infof("tsession[%d].sub[%s].samplingTimer.expired", teleses.id, telesub.key)
			} else {
				log.Infof("tsession[%d].sub[%s].heartbeatTimer.expired", teleses.id, telesub.key)
			}
			resps, err := teleses.telemetryUpdateSampled(telesub)
			if err != nil {
				return
			}
			for _, resp := range resps {
				telemetrychannel <- resp
			}
		case <-shutdown:
			log.Infof("tsession[%d].sub[%s].shutdown", teleses.id, telesub.key)
			return
		case <-stop:
			log.Infof("tsession[%d].sub[%s].stopped", teleses.id, telesub.key)
			return
		}
	}
}

// StartTelmetryUpdate - returns a key for telemetry comparison
func (teleses *telemetrySession) StartTelmetryUpdate(telesub *telemetrySubscription) error {
	teleses.lock()
	defer teleses.unlock()
	teleses.server.registerTelemetry(teleses.server.model, telesub)

	if telesub.samplingTimer == nil && telesub._sampleInterval > 0 {
		tick := time.Duration(telesub._sampleInterval)
		telesub.samplingTimer = time.NewTicker(tick * time.Nanosecond)
		if telesub.samplingTimer == nil {
			return status.Errorf(codes.Internal, "sampling rate control failed")
		}
		teleses.waitgroup.Add(1)
		go goTelemetryUpdate(
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
		go goTelemetryUpdate(
			teleses, telesub, telesub.heartbeatTimer,
			teleses.shutdown, telesub.stop,
			teleses.waitgroup, teleses.channel)
	}
	return nil
}

// StopTelemetryUpdate - returns a key for telemetry comparison
func (teleses *telemetrySession) StopTelemetryUpdate(telesub *telemetrySubscription) error {
	teleses.lock()
	defer teleses.unlock()
	teleses.server.unregisterTelemetry(telesub)

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
func (teleses *telemetrySession) initTelemetryUpdate(req *pb.SubscribeRequest) ([]*pb.SubscribeResponse, error) {
	s := teleses.server
	subscriptionList := req.GetSubscribe()
	subList := subscriptionList.GetSubscription()
	updateOnly := subscriptionList.GetUpdatesOnly()
	if updateOnly {
		return buildSyncResponse()
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
	s.modeldata.RLock()
	defer s.modeldata.RUnlock()
	if err := utils.ValidateGNMIPath(prefix); err != nil {
		return nil, status.Errorf(codes.Unimplemented, "invalid-path(%s)", err.Error())
	}
	toplist, ok := model.FindAllData(s.modeldata.GetRoot(), prefix)
	if !ok || len(toplist) <= 0 {
		_, ok = model.FindAllSchemaTypes(s.modeldata.GetRoot(), prefix)
		if ok {
			// data-missing is not an error in SubscribeRPC
			// doest send any of messages before sync response.
			return buildSyncResponse()
		}
		return nil, status.Errorf(codes.NotFound, "unknown-schema(%s)", xpath.ToXPATH(prefix))
	}
	allresponses := []*pb.SubscribeResponse{}
	for _, top := range toplist {
		bpath := top.Path
		branch := top.Value.(ygot.GoStruct)
		bprefix, err := xpath.ToGNMIPath(bpath)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "path-conversion-error(%s)", bprefix)
		}
		allupdates := []*pb.Update{}
		for _, updateEntry := range subList {
			path := updateEntry.Path
			if err := utils.ValidateGNMIFullPath(prefix, path); err != nil {
				return nil, status.Errorf(codes.Unimplemented, "invalid-path(%s)", err.Error())
			}
			datalist, ok := model.FindAllData(branch, path)
			if !ok || len(datalist) <= 0 {
				continue
			}
			j := 0
			update := make([]*pb.Update, len(datalist))
			for _, data := range datalist {
				typedValue, err := ygot.EncodeTypedValue(data.Value, encoding)
				if err != nil {
					return nil, status.Errorf(codes.Internal, "encoding-error(%s)", err.Error())
				}
				if typedValue == nil {
					continue
				}
				datapath, err := xpath.ToGNMIPath(data.Path)
				if err != nil {
					return nil, status.Errorf(codes.Internal, "path-conversion-error(%s)", data.Path)
				}
				update[j] = &pb.Update{Path: datapath, Val: typedValue}
				j++
			}
			if j > 0 {
				allupdates = append(allupdates, update[:j]...)
			}
		}
		responses, err := buildSubscribeResponse(prefix, alias, allupdates, *disableBundling, true)
		if err != nil {
			return []*pb.SubscribeResponse{}, err
		}
		allresponses = append(allresponses, responses...)
	}
	return allresponses, nil
}

// telemetryUpdateSampled - Process and generate responses for a telemetry update.
func (teleses *telemetrySession) telemetryUpdateSampled(telesub *telemetrySubscription) ([]*pb.SubscribeResponse, error) {
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
	s.modeldata.RLock()
	defer s.modeldata.RUnlock()
	if err := utils.ValidateGNMIPath(prefix); err != nil {
		return nil, status.Errorf(codes.Unimplemented, "invalid-path(%s)", err.Error())
	}
	toplist, ok := model.FindAllData(s.modeldata.GetRoot(), prefix)
	if !ok || len(toplist) <= 0 {
		_, ok = model.FindAllSchemaTypes(s.modeldata.GetRoot(), prefix)
		if ok {
			// data-missing is not an error in SubscribeRPC
			// doest send any of messages before sync response.
			return buildSyncResponse()
		}
		return nil, status.Errorf(codes.NotFound, "unknown-schema(%s)", xpath.ToXPATH(prefix))
	}

	allresponses := []*pb.SubscribeResponse{}
	for _, top := range toplist {
		bpath := top.Path
		branch := top.Value.(ygot.GoStruct)
		bprefix, err := xpath.ToGNMIPath(bpath)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "path-conversion-error(%s)", bprefix)
		}
		allupdates := []*pb.Update{}
		for _, path := range telesub.Paths {
			if err := utils.ValidateGNMIFullPath(prefix, path); err != nil {
				return nil, status.Errorf(codes.Unimplemented, "invalid-path(%s)", err.Error())
			}
			datalist, ok := model.FindAllData(branch, path)
			if !ok || len(datalist) <= 0 {
				continue
			}
			j := 0
			update := make([]*pb.Update, len(datalist))
			for _, data := range datalist {
				typedValue, err := ygot.EncodeTypedValue(data.Value, encoding)
				if err != nil {
					return nil, status.Errorf(codes.Internal, "encoding-error(%s)", err.Error())
				}
				if typedValue == nil {
					continue
				}
				datapath, err := xpath.ToGNMIPath(data.Path)
				if err != nil {
					return nil, status.Errorf(codes.Internal, "path-conversion-error(%s)", data.Path)
				}
				update[j] = &pb.Update{Path: datapath, Val: typedValue}
				j++
			}
			if j > 0 {
				allupdates = append(allupdates, update...)
			}
		}
		responses, err := buildSubscribeResponse(prefix, alias, allupdates, *disableBundling, false)
		if err != nil {
			return []*pb.SubscribeResponse{}, err
		}
		allresponses = append(allresponses, responses...)
	}
	return allresponses, nil
}

// telemetryUpdateOnChange - Process and generate responses for a telemetry update.
func (teleses *telemetrySession) telemetryUpdateOnChange(telesub *telemetrySubscription, basePath, changedPath *string) ([]*pb.SubscribeResponse, error) {
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
	// already locked....
	// s.modeldata.RLock()
	// defer s.modeldata.RUnlock()
	if err := utils.ValidateGNMIPath(prefix); err != nil {
		return nil, status.Errorf(codes.Unimplemented, "invalid-path(%s)", err.Error())
	}
	toplist, ok := model.FindAllData(s.modeldata.GetRoot(), prefix)
	if !ok || len(toplist) <= 0 {
		_, ok = model.FindAllSchemaTypes(s.modeldata.GetRoot(), prefix)
		if ok {
			// data-missing is not an error in SubscribeRPC
			// doest send any of messages before sync response.
			return buildSyncResponse()
		}
		return nil, status.Errorf(codes.NotFound, "unknown-schema(%s)", xpath.ToXPATH(prefix))
	}

	allresponses := []*pb.SubscribeResponse{}
	for _, top := range toplist {
		bpath := top.Path
		branch := top.Value.(ygot.GoStruct)
		bprefix, err := xpath.ToGNMIPath(bpath)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "path-conversion-error(%s)", bprefix)
		}
		if !strings.HasPrefix(*changedPath, bpath) {
			continue
		}
		allupdates := []*pb.Update{}
		path, err := xpath.ToGNMIPath((*changedPath)[len(bpath):])
		// path, err := ygot.StringToStructuredPath(*changedPath)
		if err != nil {
			return nil, status.Errorf(codes.Unimplemented, "invalid-path(%s)", err.Error())
		}
		if err := utils.ValidateGNMIFullPath(prefix, path); err != nil {
			return nil, status.Errorf(codes.Unimplemented, "invalid-path(%s)", err.Error())
		}
		datalist, ok := model.FindAllData(branch, path)
		if !ok || len(datalist) <= 0 {
			continue
		}
		update := make([]*pb.Update, len(datalist))
		for j, data := range datalist {
			typedValue, err := ygot.EncodeTypedValue(data.Value, encoding)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "encoding-error(%s)", err.Error())
			}
			datapath, err := xpath.ToGNMIPath(data.Path)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "path-conversion-error(%s)", data.Path)
			}
			update[j] = &pb.Update{Path: datapath, Val: typedValue}
		}
		allupdates = append(allupdates, update...)

		responses, err := buildSubscribeResponse(prefix, alias, allupdates, *disableBundling, false)
		if err != nil {
			return []*pb.SubscribeResponse{}, err
		}
		allresponses = append(allresponses, responses...)
	}
	return allresponses, nil
}

func (teleses *telemetrySession) addStreamSubscription(
	prefix *pb.Path, useAliases bool, streamingMode pb.SubscriptionList_Mode, allowAggregation bool,
	encoding pb.Encoding, paths []*pb.Path, subscriptionMode pb.SubscriptionMode,
	sampleInterval uint64, suppressRedundant bool, heartbeatInterval uint64) (*telemetrySubscription, error) {

	telesub := &telemetrySubscription{
		sessionid:         teleses.id,
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
	key := fmt.Sprintf("%d-%s-%s-%s-%s-%d-%d-%t-%t-%t",
		telesub.sessionid,
		telesub.StreamingMode, telesub.Encoding, telesub.SubscriptionMode,
		xpath.ToXPATH(telesub.Prefix), telesub.SampleInterval, telesub.HeartbeatInterval,
		telesub.UseAliases, telesub.AllowAggregation, telesub.SuppressRedundant,
	)
	telesub.key = &key
	teleses.lock()
	defer teleses.unlock()
	if t, ok := teleses.telesub[key]; ok {
		// only add the path
		t.Paths = append(t.Paths, telesub.Paths...)
		telesub = t
		log.Infof("tsession[%d].sub[%s].append(%s)", teleses.id, key, xpath.ToXPATH(telesub.Paths[0]))
	} else {
		subID++
		id := subID
		telesub.id = id
		telesub.session = teleses
		teleses.telesub[key] = telesub
		log.Infof("tsession[%d].sub[%s].add(%s)", teleses.id, key, xpath.ToXPATH(telesub.Paths[0]))
	}
	return telesub, nil
}

// addPollSubscription - Create new telemetrySubscription
func (teleses *telemetrySession) addPollSubscription() error {
	teleses.lock()
	defer teleses.unlock()
	telesub := telemetrySubscription{
		StreamingMode: pb.SubscriptionList_POLL,
		Paths:         []*pb.Path{},
		isPolling:     true,
		session:       teleses,
	}
	subID++
	id := subID
	key := fmt.Sprintf("%s", telesub.StreamingMode)
	telesub.id = id
	telesub.key = &key
	teleses.telesub[key] = &telesub
	log.Infof("tsession[%d].sub[%s].add(polling)", teleses.id, key)
	return nil
}

func (teleses *telemetrySession) updateAliases(aliaslist []*pb.Alias) error {
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

func processSR(teleses *telemetrySession, req *pb.SubscribeRequest) error {
	// SubscribeRequest for poll Subscription indication
	pollMode := req.GetPoll()
	if pollMode != nil {
		return teleses.addPollSubscription()
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

	if err := teleses.server.model.CheckModels(useModules); err != nil {
		return status.Errorf(codes.Unimplemented, err.Error())
	}
	if err := teleses.server.checkEncoding(encoding); err != nil {
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
		telesub, err := teleses.addStreamSubscription(
			prefix, useAliases, pb.SubscriptionList_STREAM,
			allowAggregation, encoding, []*pb.Path{path}, submod,
			_sampleInterval, supressRedundant, heartBeatInterval)
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
