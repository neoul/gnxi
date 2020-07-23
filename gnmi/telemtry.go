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
type present struct{}
type pathSet map[string]present

type telemetryUpdateJob struct {
	mergedList  pathSet
	deletedList pathSet
	changes     ygot.GoStruct
}

type telemetryCB struct {
	// map[uint]*telemetrySubscription: uint = is subscription.id
	lookupTeleSub map[string]map[telemetryID]*telemetrySubscription
	readyToUpdate map[telemetryID]*telemetrySubscription
	mergedList    map[telemetryID]pathSet // paths
	deletedList   map[telemetryID]pathSet // paths
	mutex         sync.RWMutex
}

func newTelemetryCB() *telemetryCB {
	return &telemetryCB{
		lookupTeleSub: map[string]map[telemetryID]*telemetrySubscription{},
		readyToUpdate: map[telemetryID]*telemetrySubscription{},
		mergedList:    map[telemetryID]pathSet{},
		deletedList:   map[telemetryID]pathSet{},
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
	tcb.mutex.Lock()
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
	tcb.readyToUpdate = map[telemetryID]*telemetrySubscription{}
	tcb.mergedList = map[telemetryID]pathSet{}
	tcb.deletedList = map[telemetryID]pathSet{}
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
				telesub.Duplicates++
				if tcb.mergedList[telesub.id] == nil {
					tcb.mergedList[telesub.id] = pathSet{path: present{}}
				} else {
					tcb.mergedList[telesub.id][path] = present{}
				}
				tcb.readyToUpdate[telesub.id] = telesub
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
				telesub.Duplicates++
				if tcb.mergedList[telesub.id] == nil {
					tcb.mergedList[telesub.id] = pathSet{path: present{}}
				} else {
					tcb.mergedList[telesub.id][path] = present{}
				}
				if tcb.deletedList[telesub.id] == nil {
					tcb.deletedList[telesub.id] = pathSet{path: present{}}
				} else {
					tcb.mergedList[telesub.id][path] = present{}
				}
				tcb.readyToUpdate[telesub.id] = telesub
			}
		}
	}
}

// OnChangeDeleted - callback for Telemetry subscription on data changes
func (tcb *telemetryCB) OnChangeDeleted(path []string) {
	tcb.mutex.RLock()
	defer tcb.mutex.RUnlock()
	fmt.Println(tcb.lookupTeleSub)

	for i := len(path); i >= 0; i-- {
		path := "/" + strings.Join(path[:i], "/")
		// fmt.Println(path)
		subgroup, ok := tcb.lookupTeleSub[path]
		if ok {
			for _, telesub := range subgroup {
				if telesub.isPolling {
					continue
				}
				telesub.Duplicates++
				if tcb.deletedList[telesub.id] == nil {
					tcb.deletedList[telesub.id] = pathSet{path: present{}}
				} else {
					tcb.deletedList[telesub.id][path] = present{}
				}
				tcb.readyToUpdate[telesub.id] = telesub
			}
		}
	}
}

// OnStarted - callback for Telemetry subscription on data changes
func (tcb *telemetryCB) OnChangeFinished(changes ygot.GoStruct) {
	tcb.mutex.RLock()
	defer tcb.mutex.RUnlock()
	for telesubid, telesub := range tcb.readyToUpdate {
		if telesub.job != nil {
			telesub.job <- &telemetryUpdateJob{
				mergedList:  tcb.mergedList[telesubid],
				deletedList: tcb.deletedList[telesubid],
				changes:     changes,
			}
		}
		delete(tcb.readyToUpdate, telesubid)
		delete(tcb.mergedList, telesubid)
		delete(tcb.deletedList, telesubid)
	}
}

// telemetrySession - gNMI gRPC Subscribe RPC (Telemetry) session information managed by server
type telemetrySession struct {
	id        telemetryID
	telesub   map[string]*telemetrySubscription
	respchan  chan *pb.SubscribeResponse
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
		respchan:  make(chan *pb.SubscribeResponse, 256),
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
	session            *telemetrySession
	_subscriptionMode  pb.SubscriptionMode
	_sampleInterval    uint64
	_suppressRedundant bool
	_heartbeatInterval uint64

	job         chan *telemetryUpdateJob
	mergedList  pathSet
	deletedList pathSet
	started     bool
	stop        chan struct{}
	isPolling   bool

	// // https://github.com/openconfig/gnmi/issues/45 - QoSMarking seems to be deprecated
	// Qos              *pb.QOSMarking           `json:"qos,omitempty"`          // DSCP marking to be used.
	// UseModels        []*pb.ModelData          `json:"use_models,omitempty"`   // (Check validate only in Request)
	// Alias            []*pb.Alias              `json:"alias,omitempty"`
	// UpdatesOnly       bool                     `json:"updates_only,omitempty"` // not required to store
	// [FIXME]
	// 1. Ticker (Timer)
	// 2. keys (The path to the subscription data)
}

func (telesub *telemetrySubscription) run(teleses *telemetrySession) {
	var samplingTimer, heartbeatTimer *time.Ticker
	shutdown := teleses.shutdown
	waitgroup := teleses.waitgroup
	defer func() {
		telesub.started = false
		waitgroup.Done()
	}()
	if telesub._sampleInterval > 0 {
		tick := time.Duration(telesub._sampleInterval)
		samplingTimer = time.NewTicker(tick * time.Nanosecond)
	} else {
		tick := time.Duration(defaultInterval)
		samplingTimer = time.NewTicker(tick * time.Nanosecond)
		samplingTimer.Stop() // stop
	}
	if telesub._heartbeatInterval > 0 {
		tick := time.Duration(telesub._heartbeatInterval)
		heartbeatTimer = time.NewTicker(tick * time.Nanosecond)
	} else {
		tick := time.Duration(defaultInterval)
		heartbeatTimer = time.NewTicker(tick * time.Nanosecond)
		heartbeatTimer.Stop() // stop
	}
	if samplingTimer == nil || heartbeatTimer == nil {
		log.Errorf("telemetry[%d][%d].timer-failed", telesub.sessionid, telesub.id)
		return
	}
	for {
		select {
		case job, ok := <-telesub.job:
			if !ok {
				log.Errorf("telemetry[%d][%d].job-queue-closed", telesub.sessionid, telesub.id)
				return
			}
			log.Infof("telemetry[%d][%d].job(mergedlist:%s, deletedlist:%s)",
				telesub.sessionid, telesub.id, job.deletedList, job.mergedList)
			switch telesub._subscriptionMode {
			case pb.SubscriptionMode_ON_CHANGE:
				// err := teleses.onChangeTelemetryUpdate(telesub)
				// if err != nil {
				// 	log.Errorf("telemetry[%d][%d].failed(%v)", telesub.sessionid, telesub.id, err)
				// 	return
				// }
			case pb.SubscriptionMode_SAMPLE:
				for p := range job.mergedList {
					telesub.mergedList[p] = present{}
				}
				for p := range job.deletedList {
					telesub.deletedList[p] = present{}
				}
			}
		case <-samplingTimer.C:
			log.Infof("telemetry[%d][%d].sampling-timer-expired", telesub.sessionid, telesub.id)
			err := teleses.SampledTelemetryUpdate(telesub)
			if err != nil {
				log.Errorf("telemetry[%d][%d].failed(%v)", telesub.sessionid, telesub.id, err)
				return
			}
		case <-heartbeatTimer.C:
			log.Infof("telemetry[%d][%d].heartbeat-timer-expired", telesub.sessionid, telesub.id)
			err := teleses.SampledTelemetryUpdate(telesub)
			if err != nil {
				log.Errorf("telemetry[%d][%d].failed(%v)", telesub.sessionid, telesub.id, err)
				return
			}
		case <-shutdown:
			log.Infof("telemetry[%d][%d].shutdown", teleses.id, telesub.id)
			return
		case <-telesub.stop:
			log.Infof("telemetry[%d][%d].stopped", teleses.id, telesub.id)
			return
		}
	}
}

// StartTelmetryUpdate - returns a key for telemetry comparison
func (teleses *telemetrySession) StartTelmetryUpdate(telesub *telemetrySubscription) error {
	teleses.lock()
	defer teleses.unlock()
	teleses.server.registerTelemetry(teleses.server.model, telesub)
	if !telesub.started {
		telesub.started = true
		teleses.waitgroup.Add(1)
		go telesub.run(teleses)
	}
	return nil
}

// StopTelemetryUpdate - returns a key for telemetry comparison
func (teleses *telemetrySession) StopTelemetryUpdate(telesub *telemetrySubscription) error {
	teleses.lock()
	defer teleses.unlock()
	teleses.server.unregisterTelemetry(telesub)

	close(telesub.stop)
	if telesub.job != nil {
		close(telesub.job)
	}
	return nil
}

func (teleses *telemetrySession) sendTelemetryUpdate(responses []*pb.SubscribeResponse) error {
	for _, response := range responses {
		teleses.respchan <- response
	}
	return nil
}

// initTelemetryUpdate - Process and generate responses for a init update.
func (teleses *telemetrySession) initTelemetryUpdate(req *pb.SubscribeRequest) error {
	s := teleses.server
	subscriptionList := req.GetSubscribe()
	subList := subscriptionList.GetSubscription()
	updateOnly := subscriptionList.GetUpdatesOnly()
	if updateOnly {
		return teleses.sendTelemetryUpdate(buildSyncResponse())
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
		return status.Errorf(codes.Unimplemented, "invalid-path(%s)", err.Error())
	}
	toplist, ok := model.FindAllData(s.modeldata.GetRoot(), prefix)
	if !ok || len(toplist) <= 0 {
		_, ok = model.FindAllSchemaTypes(s.modeldata.GetRoot(), prefix)
		if ok {
			// data-missing is not an error in SubscribeRPC
			// doest send any of messages ahead of the sync response.
			return teleses.sendTelemetryUpdate(buildSyncResponse())
		}
		return status.Errorf(codes.NotFound, "unknown-schema(%s)", xpath.ToXPATH(prefix))
	}
	allresponses := []*pb.SubscribeResponse{}
	for _, top := range toplist {
		bpath := top.Path
		branch := top.Value.(ygot.GoStruct)
		bprefix, err := xpath.ToGNMIPath(bpath)
		if err != nil {
			return status.Errorf(codes.Internal, "path-conversion-error(%s)", bprefix)
		}
		allupdates := []*pb.Update{}
		for _, updateEntry := range subList {
			path := updateEntry.Path
			if err := utils.ValidateGNMIFullPath(prefix, path); err != nil {
				return status.Errorf(codes.Unimplemented, "invalid-path(%s)", err.Error())
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
					return status.Errorf(codes.Internal, "encoding-error(%s)", err.Error())
				}
				if typedValue == nil {
					continue
				}
				datapath, err := xpath.ToGNMIPath(data.Path)
				if err != nil {
					return status.Errorf(codes.Internal, "path-conversion-error(%s)", data.Path)
				}
				update[j] = &pb.Update{Path: datapath, Val: typedValue}
				j++
			}
			if j > 0 {
				allupdates = append(allupdates, update[:j]...)
			}
		}
		allresponses = append(allresponses,
			buildSubscribeResponse(prefix, alias, allupdates, nil, true)...)
	}
	return teleses.sendTelemetryUpdate(allresponses)
}

// SampledTelemetryUpdate - Process and generate responses for a telemetry update.
func (teleses *telemetrySession) SampledTelemetryUpdate(telesub *telemetrySubscription) error {
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
		return status.Errorf(codes.Unimplemented, "invalid-path(%s)", err.Error())
	}
	toplist, ok := model.FindAllData(s.modeldata.GetRoot(), prefix)
	if !ok || len(toplist) <= 0 {
		_, ok = model.FindAllSchemaTypes(s.modeldata.GetRoot(), prefix)
		if ok {
			// data-missing is not an error in SubscribeRPC
			// doest send any of messages before sync response.
			return teleses.sendTelemetryUpdate(buildSyncResponse())
		}
		return status.Errorf(codes.NotFound, "unknown-schema(%s)", xpath.ToXPATH(prefix))
	}

	allresponses := []*pb.SubscribeResponse{}
	for _, top := range toplist {
		bpath := top.Path
		branch := top.Value.(ygot.GoStruct)
		bprefix, err := xpath.ToGNMIPath(bpath)
		if err != nil {
			return status.Errorf(codes.Internal, "path-conversion-error(%s)", bprefix)
		}
		allupdates := []*pb.Update{}
		for _, path := range telesub.Paths {
			if err := utils.ValidateGNMIFullPath(prefix, path); err != nil {
				return status.Errorf(codes.Unimplemented, "invalid-path(%s)", err.Error())
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
					return status.Errorf(codes.Internal, "encoding-error(%s)", err.Error())
				}
				if typedValue == nil {
					continue
				}
				datapath, err := xpath.ToGNMIPath(data.Path)
				if err != nil {
					return status.Errorf(codes.Internal, "path-conversion-error(%s)", data.Path)
				}
				update[j] = &pb.Update{Path: datapath, Val: typedValue}
				j++
			}
			if j > 0 {
				allupdates = append(allupdates, update...)
			}
		}
		allresponses = append(allresponses,
			buildSubscribeResponse(prefix, alias, allupdates, nil, false)...)
	}
	return teleses.sendTelemetryUpdate(allresponses)
}

// onChangeTelemetryUpdate - Process and generate responses for a telemetry update.
func (teleses *telemetrySession) onChangeTelemetryUpdate(telesub *telemetrySubscription) error {
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
	// already locked
	// s.modeldata.RLock()
	// defer s.modeldata.RUnlock()
	if err := utils.ValidateGNMIPath(prefix); err != nil {
		return status.Errorf(codes.Unimplemented, "invalid-path(%s)", err.Error())
	}
	toplist, ok := model.FindAllData(s.modeldata.GetFakeRoot(), prefix)
	if !ok || len(toplist) <= 0 {
		_, ok = model.FindAllSchemaTypes(s.modeldata.GetFakeRoot(), prefix)
		if ok {
			// data-missing is not an error in SubscribeRPC
			// doest send any of messages before sync response.
			return teleses.sendTelemetryUpdate(buildSyncResponse())
		}
		return status.Errorf(codes.NotFound, "unknown-schema(%s)", xpath.ToXPATH(prefix))
	}

	allresponses := []*pb.SubscribeResponse{}
	for _, top := range toplist {
		bpath := top.Path
		branch := top.Value.(ygot.GoStruct)
		bprefix, err := xpath.ToGNMIPath(bpath)
		if err != nil {
			return status.Errorf(codes.Internal, "path-conversion-error(%s)", bprefix)
		}
		allupdates := []*pb.Update{}
		for _, path := range telesub.Paths {
			if err := utils.ValidateGNMIFullPath(prefix, path); err != nil {
				return status.Errorf(codes.Unimplemented, "invalid-path(%s)", err.Error())
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
					return status.Errorf(codes.Internal, "encoding-error(%s)", err.Error())
				}
				if typedValue == nil {
					continue
				}
				datapath, err := xpath.ToGNMIPath(data.Path)
				if err != nil {
					return status.Errorf(codes.Internal, "path-conversion-error(%s)", data.Path)
				}
				update[j] = &pb.Update{Path: datapath, Val: typedValue}
				j++
			}
			if j > 0 {
				allupdates = append(allupdates, update...)
			}

		}
		allresponses = append(allresponses,
			buildSubscribeResponse(prefix, alias, allupdates, nil, false)...)
	}
	return teleses.sendTelemetryUpdate(allresponses)
}

const (
	defaultInterval = 60000000000
	minimumInterval = 1000000000
)

func (teleses *telemetrySession) addStreamSubscription(
	prefix *pb.Path, useAliases bool, streamingMode pb.SubscriptionList_Mode, allowAggregation bool,
	encoding pb.Encoding, paths []*pb.Path, subscriptionMode pb.SubscriptionMode,
	sampleInterval uint64, suppressRedundant bool, heartbeatInterval uint64,
) (*telemetrySubscription, error) {

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
		mergedList:        pathSet{},
		deletedList:       pathSet{},
	}
	if streamingMode == pb.SubscriptionList_POLL {
		return nil, status.Errorf(codes.InvalidArgument,
			"poll subscription configured as streaming subscription")
	}
	// 3.5.1.5.2 STREAM Subscriptions Must be satisfied for telemetry update starting.
	switch telesub.SubscriptionMode {
	case pb.SubscriptionMode_TARGET_DEFINED:
		// vendor specific mode
		telesub._subscriptionMode = pb.SubscriptionMode_ON_CHANGE
		telesub._sampleInterval = 0
		telesub._suppressRedundant = false
		telesub._heartbeatInterval = defaultInterval // 60sec
	case pb.SubscriptionMode_ON_CHANGE:
		if telesub.HeartbeatInterval < minimumInterval && telesub.HeartbeatInterval != 0 {
			return nil, status.Errorf(codes.InvalidArgument,
				"heartbeat_interval(< 1sec) is not supported")
		}
		telesub._subscriptionMode = pb.SubscriptionMode_ON_CHANGE
		telesub._sampleInterval = 0
		telesub._suppressRedundant = false
		telesub._heartbeatInterval = telesub.HeartbeatInterval
	case pb.SubscriptionMode_SAMPLE:
		if telesub.SampleInterval < minimumInterval && telesub.SampleInterval != 0 {
			return nil, status.Errorf(codes.InvalidArgument,
				"sample_interval(< 1sec) is not supported")
		}
		if telesub.HeartbeatInterval < minimumInterval && telesub.HeartbeatInterval != 0 {
			return nil, status.Errorf(codes.InvalidArgument,
				"heartbeat_interval(< 1sec) is not supported")
		}
		telesub._subscriptionMode = pb.SubscriptionMode_SAMPLE
		telesub._sampleInterval = telesub.SampleInterval
		if telesub.SampleInterval == 0 {
			// Set minimal sampling interval (1sec)
			telesub._sampleInterval = minimumInterval
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
	telesub.job = make(chan *telemetryUpdateJob, 64)
	teleses.lock()
	defer teleses.unlock()
	if t, ok := teleses.telesub[key]; ok {
		// only updates the new path
		t.Paths = append(t.Paths, telesub.Paths...)
		telesub = t
		log.Infof("telemetry[%d][%s].add-path(%s)", teleses.id, key, xpath.ToXPATH(telesub.Paths[len(telesub.Paths)-1]))
		return nil, nil
	}
	subID++
	id := subID
	telesub.id = id
	telesub.session = teleses
	teleses.telesub[key] = telesub
	log.Infof("telemetry[%d][%d].new(%s)", teleses.id, telesub.id, *telesub.key)
	log.Infof("telemetry[%d][%d].add-path(%s)", teleses.id, telesub.id, xpath.ToXPATH(telesub.Paths[0]))
	return telesub, nil
}

// addPollSubscription - Create new telemetrySubscription
func (teleses *telemetrySession) addPollSubscription() error {
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
	teleses.lock()
	defer teleses.unlock()
	teleses.telesub[key] = &telesub
	log.Infof("telemetry[%d][%d].new(%s)", teleses.id, telesub.id, *telesub.key)
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

	err := teleses.initTelemetryUpdate(req)
	mode := subscriptionList.GetMode()
	if mode == pb.SubscriptionList_ONCE ||
		mode == pb.SubscriptionList_POLL ||
		err != nil {
		return err
	}

	prefix := subscriptionList.GetPrefix()
	useAliases := subscriptionList.GetUseAliases()
	allowAggregation := subscriptionList.GetAllowAggregation()
	startingList := []*telemetrySubscription{}
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
		if telesub != nil {
			startingList = append(startingList, telesub)
		}
	}
	for _, telesub := range startingList {
		err = teleses.StartTelmetryUpdate(telesub)
		if err != nil {
			return err
		}
	}
	return nil
}
