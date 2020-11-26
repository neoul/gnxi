package server

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/neoul/gnxi/gnmi/model"
	"github.com/neoul/gnxi/utilities"
	"github.com/neoul/gnxi/utilities/xpath"
	"github.com/neoul/gtrie"
	"github.com/neoul/libydb/go/ydb"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ygot/ygot"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TelemID - Telemetry Session ID
type TelemID uint64

type telemEvent struct {
	telesub     *TelemetrySubscription
	updatedroot ygot.GoStruct
	updatedPath []*string
	deletedPath []*string
}

// gNMI Telemetry Control Block
type telemCtrl struct {
	// lookup map[TelemID]*TelemetrySubscription using a path
	lookup *gtrie.Trie
	ready  map[TelemID]*telemEvent
	mutex  *sync.Mutex
}

func newTeleCtrl() *telemCtrl {
	return &telemCtrl{
		lookup: gtrie.New(),
		ready:  make(map[TelemID]*telemEvent),
		mutex:  &sync.Mutex{},
	}
}

func (tcb *telemCtrl) register(m *model.Model, telesub *TelemetrySubscription) error {
	tcb.mutex.Lock()
	defer tcb.mutex.Unlock()
	for _, path := range telesub.Paths {
		fullpath := xpath.GNMIFullPath(telesub.Prefix, path)
		targetpaths, _ := m.FindAllPaths(fullpath)
		for _, p := range targetpaths {
			if subgroup, ok := tcb.lookup.Find(p); ok {
				subgroup.(map[TelemID]*TelemetrySubscription)[telesub.ID] = telesub
			} else {
				tcb.lookup.Add(p, map[TelemID]*TelemetrySubscription{telesub.ID: telesub})
			}
			glog.Infof("telemCtrl.telesub[%d][%d].subscribe(%v)", telesub.SessionID, telesub.ID, p)
		}
	}
	return nil
}

func (tcb *telemCtrl) unregister(telesub *TelemetrySubscription) {
	tcb.mutex.Lock()
	defer tcb.mutex.Unlock()
	lookup := tcb.lookup.All("")
	for p, subgroup := range lookup {
		subscriber := subgroup.(map[TelemID]*TelemetrySubscription)
		_, ok := subscriber[telesub.ID]
		if ok {
			glog.Infof("telemCtrl.telesub[%d][%d].unsubscribe(%v)", telesub.SessionID, telesub.ID, p)
			delete(subscriber, telesub.ID)
			if len(subscriber) == 0 {
				tcb.lookup.Remove(p)
			}
		}
	}
}

// ChangeNotification.ChangeStarted - callback for Telemetry subscription on data changes
func (tcb *telemCtrl) ChangeStarted(changes ygot.GoStruct) {
	tcb.mutex.Lock()
	defer tcb.mutex.Unlock()
	tcb.ready = make(map[TelemID]*telemEvent)
	// glog.Infof("telemCtrl.ChangeStarted")
}

func (tcb *telemCtrl) updateTelemEvent(op int, datapath, searchpath string) {
	for _, subgroup := range tcb.lookup.FindAll(searchpath) {
		subscribers := subgroup.(map[TelemID]*TelemetrySubscription)
		for _, telesub := range subscribers {
			if telesub.IsPolling {
				continue
			}
			// glog.Infof("telemCtrl.%c.path(%s).telesub[%d][%d]",
			// 	op, subscribedpath, telesub.SessionID, telesub.ID)
			event, ok := tcb.ready[telesub.ID]
			if !ok {
				event = &telemEvent{
					telesub:     telesub,
					updatedPath: make([]*string, 0, 64),
					deletedPath: make([]*string, 0, 64),
				}
				tcb.ready[telesub.ID] = event
			}
			switch op {
			case 'C':
				event.updatedPath = append(event.updatedPath, &datapath)
			case 'R':
				event.updatedPath = append(event.updatedPath, &datapath)
				event.deletedPath = append(event.deletedPath, &datapath)
			case 'D':
				event.deletedPath = append(event.deletedPath, &datapath)
			default:
				return
			}
		}
	}
}

// ChangeNotification.ChangeCreated - callback for Telemetry subscription on data changes
func (tcb *telemCtrl) ChangeCreated(datapath string, changes ygot.GoStruct) {
	tcb.mutex.Lock()
	defer tcb.mutex.Unlock()
	glog.Infof("telemCtrl.ChangeCreated.path(%s)", datapath)
	gpath, err := xpath.ToGNMIPath(datapath)
	if err != nil {
		return
	}
	tcb.updateTelemEvent('C', datapath, datapath)
	if !xpath.IsSchemaPath(gpath) {
		tcb.updateTelemEvent('C', datapath, xpath.PathElemToXPATH(gpath.GetElem(), true))
	}
}

// ChangeNotification.ChangeReplaced - callback for Telemetry subscription on data changes
func (tcb *telemCtrl) ChangeReplaced(datapath string, changes ygot.GoStruct) {
	tcb.mutex.Lock()
	defer tcb.mutex.Unlock()
	glog.Infof("telemCtrl.ChangeReplaced.path(%s)", datapath)
	gpath, err := xpath.ToGNMIPath(datapath)
	if err != nil {
		return
	}
	tcb.updateTelemEvent('R', datapath, datapath)
	if !xpath.IsSchemaPath(gpath) {
		tcb.updateTelemEvent('R', datapath, xpath.PathElemToXPATH(gpath.GetElem(), true))
	}
}

// ChangeNotification.ChangeDeleted - callback for Telemetry subscription on data changes
func (tcb *telemCtrl) ChangeDeleted(datapath string) {
	tcb.mutex.Lock()
	defer tcb.mutex.Unlock()
	glog.Infof("telemCtrl.ChangeDeleted.path(%s)", datapath)
	gpath, err := xpath.ToGNMIPath(datapath)
	if err != nil {
		return
	}
	tcb.updateTelemEvent('D', datapath, datapath)
	if !xpath.IsSchemaPath(gpath) {
		tcb.updateTelemEvent('D', datapath, xpath.PathElemToXPATH(gpath.GetElem(), true))
	}
}

// ChangeNotification.ChangeCompleted - callback for Telemetry subscription on data changes
func (tcb *telemCtrl) ChangeCompleted(changes ygot.GoStruct) {
	tcb.mutex.Lock()
	defer tcb.mutex.Unlock()
	for telesubid, event := range tcb.ready {
		telesub := event.telesub
		if telesub.eventque != nil {
			event.updatedroot = changes
			telesub.eventque <- event
			glog.Infof("telemCtrl.send.event.to.telesub[%d][%d]", telesub.SessionID, telesub.ID)
		}
		delete(tcb.ready, telesubid)
	}
	// glog.Infof("telemCtrl.ChangeCompleted")
}

// TelemetrySession - gNMI gRPC Subscribe RPC (Telemetry) session information managed by server
type TelemetrySession struct {
	ID        TelemID
	Address   string
	Port      uint16
	Telesub   map[string]*TelemetrySubscription
	respchan  chan *gnmipb.SubscribeResponse
	shutdown  chan struct{}
	waitgroup *sync.WaitGroup
	alias     map[string]*gnmipb.Alias
	*Server
}

var (
	sessID TelemID
	subID  TelemID
)

func (teleses *TelemetrySession) String() string {
	return fmt.Sprintf("%s:%d", teleses.Address, teleses.Port)
}

func newTelemetrySession(ctx context.Context, s *Server) *TelemetrySession {
	var address string
	var port int
	sessID++
	_, remoteaddr, _ := utilities.QueryAddr(ctx)
	addr := remoteaddr.String()
	end := strings.LastIndex(addr, ":")
	if end >= 0 {
		address = addr[:end]
		port, _ = strconv.Atoi(addr[end+1:])
	}
	return &TelemetrySession{
		ID:        sessID,
		Address:   address,
		Port:      uint16(port),
		Telesub:   map[string]*TelemetrySubscription{},
		respchan:  make(chan *gnmipb.SubscribeResponse, 256),
		shutdown:  make(chan struct{}),
		waitgroup: new(sync.WaitGroup),
		alias:     map[string]*gnmipb.Alias{},
		Server:    s,
	}
}

func (teleses *TelemetrySession) stopTelemetrySession() {
	delDynamicTeleSub(teleses.Model, teleses)
	for _, telesub := range teleses.Telesub {
		teleses.unregister(telesub)
	}
	close(teleses.shutdown)
	teleses.waitgroup.Wait()
}

// TelemetrySubscription - Default structure for Telemetry Update Subscription
type TelemetrySubscription struct {
	ID                TelemID
	SessionID         TelemID
	key               *string
	Prefix            *gnmipb.Path                 `json:"prefix,omitempty"`
	UseAliases        bool                         `json:"use_aliases,omitempty"`
	StreamingMode     gnmipb.SubscriptionList_Mode `json:"stream_mode,omitempty"`
	AllowAggregation  bool                         `json:"allow_aggregation,omitempty"`
	Encoding          gnmipb.Encoding              `json:"encoding,omitempty"`
	Paths             []*gnmipb.Path               `json:"path,omitempty"`              // The data tree path.
	SubscriptionMode  gnmipb.SubscriptionMode      `json:"subscription_mode,omitempty"` // Subscription mode to be used.
	SampleInterval    uint64                       `json:"sample_interval,omitempty"`   // ns between samples in SAMPLE mode.
	SuppressRedundant bool                         `json:"suppress_redundant,omitempty"`
	HeartbeatInterval uint64                       `json:"heartbeat_interval,omitempty"`

	Configured struct {
		SubscriptionMode  gnmipb.SubscriptionMode
		SampleInterval    uint64
		SuppressRedundant bool
		HeartbeatInterval uint64
	}
	IsPolling bool

	// internal data
	session     *TelemetrySession
	stateSync   bool // StateSync is required before telemetry update?
	eventque    chan *telemEvent
	updatedList *gtrie.Trie
	deletedList *gtrie.Trie
	started     bool
	stop        chan struct{}
	mutex       *sync.Mutex

	// // https://github.com/openconfig/gnmi/issues/45 - QoSMarking seems to be deprecated
	// Qos              *gnmipb.QOSMarking           `json:"qos,omitempty"`          // DSCP marking to be used.
	// UseModels        []*gnmipb.ModelData          `json:"use_models,omitempty"`   // (Check validate only in Request)
	// Alias            []*gnmipb.Alias              `json:"alias,omitempty"`
	// UpdatesOnly       bool                     `json:"updates_only,omitempty"` // not required to store
	// [FIXME]
	// 1. Ticker (Timer)
	// 2. keys (The path to the subscription data)
}

func (telesub *TelemetrySubscription) run(teleses *TelemetrySession) {
	var samplingTimer *time.Ticker
	var heartbeatTimer *time.Ticker
	shutdown := teleses.shutdown
	waitgroup := teleses.waitgroup
	defer func() {
		glog.Warningf("telesub[%d][%d].quit", telesub.SessionID, telesub.ID)
		telesub.started = false
		waitgroup.Done()
	}()
	if telesub.Configured.SampleInterval > 0 {
		tick := time.Duration(telesub.Configured.SampleInterval)
		samplingTimer = time.NewTicker(tick * time.Nanosecond)
	} else {
		tick := time.Duration(defaultInterval)
		samplingTimer = time.NewTicker(tick * time.Nanosecond)
		samplingTimer.Stop() // stop
	}
	if telesub.Configured.HeartbeatInterval > 0 {
		tick := time.Duration(telesub.Configured.HeartbeatInterval)
		heartbeatTimer = time.NewTicker(tick * time.Nanosecond)
	} else {
		tick := time.Duration(defaultInterval)
		heartbeatTimer = time.NewTicker(tick * time.Nanosecond)
		heartbeatTimer.Stop() // stop
	}
	if samplingTimer == nil || heartbeatTimer == nil {
		glog.Errorf("telesub[%d][%d].timer-failed", telesub.SessionID, telesub.ID)
		return
	}
	timerExpired := make(chan bool, 2)

	for {
		select {
		case event, ok := <-telesub.eventque:
			if !ok {
				glog.Errorf("telesub[%d][%d].event-queue-closed", telesub.SessionID, telesub.ID)
				return
			}
			glog.Infof("telesub[%d][%d].event-received", telesub.SessionID, telesub.ID)
			switch telesub.Configured.SubscriptionMode {
			case gnmipb.SubscriptionMode_ON_CHANGE, gnmipb.SubscriptionMode_SAMPLE:
				for _, p := range event.updatedPath {
					duplicates := uint32(1)
					if v, ok := telesub.updatedList.Find(*p); ok {
						duplicates = v.(uint32)
						duplicates++
					}
					telesub.updatedList.Add(*p, duplicates)
				}
				for _, p := range event.deletedPath {
					duplicates := uint32(1)
					if v, ok := telesub.deletedList.Find(*p); ok {
						duplicates = v.(uint32)
						duplicates++
					}
					telesub.deletedList.Add(*p, duplicates)
				}
				if telesub.Configured.SubscriptionMode == gnmipb.SubscriptionMode_ON_CHANGE {
					err := teleses.telemetryUpdate(telesub, event.updatedroot)
					if err != nil {
						glog.Errorf("telesub[%d][%d].failed(%v)", telesub.SessionID, telesub.ID, err)
						return
					}
					telesub.updatedList = gtrie.New()
					telesub.deletedList = gtrie.New()
				}
			}
		case <-samplingTimer.C:
			glog.Infof("telesub[%d][%d].sampling-timer-expired", telesub.SessionID, telesub.ID)
			if telesub.stateSync {
				telesub.mutex.Lock()
				telesub.session.RequestStateSync(telesub.Prefix, telesub.Paths)
				telesub.mutex.Unlock()
			}
			timerExpired <- false
		case <-heartbeatTimer.C:
			glog.Infof("telesub[%d][%d].heartbeat-timer-expired", telesub.SessionID, telesub.ID)
			timerExpired <- true
		case sendUpdate := <-timerExpired:
			if !sendUpdate {
				// suppress_redundant - skips the telemetry update if no changes
				if !telesub.Configured.SuppressRedundant ||
					telesub.updatedList.Size() > 0 ||
					telesub.deletedList.Size() > 0 {
					sendUpdate = true
				}
			}
			if sendUpdate {
				glog.Infof("telesub[%d][%d].send.telemetry-update.to.%v",
					telesub.SessionID, telesub.ID, telesub.session)
				telesub.mutex.Lock() // block to modify telesub.Path
				err := teleses.telemetryUpdate(telesub, nil)
				telesub.mutex.Unlock()
				if err != nil {
					glog.Errorf("telesub[%d][%d].failed(%v)", telesub.SessionID, telesub.ID, err)
					return
				}
				telesub.updatedList = gtrie.New()
				telesub.deletedList = gtrie.New()
			}
		case <-shutdown:
			glog.Infof("telesub[%d][%d].shutdown", teleses.ID, telesub.ID)
			return
		case <-telesub.stop:
			glog.Infof("telesub[%d][%d].stopped", teleses.ID, telesub.ID)
			return
		}
	}
}

// StartTelmetryUpdate - returns a key for telemetry comparison
func (teleses *TelemetrySession) StartTelmetryUpdate(telesub *TelemetrySubscription) error {
	teleses.Server.telemCtrl.register(teleses.Model, telesub)
	if !telesub.started {
		telesub.started = true
		teleses.waitgroup.Add(1)
		go telesub.run(teleses)
	}
	return nil
}

// StopTelemetryUpdate - returns a key for telemetry comparison
func (teleses *TelemetrySession) StopTelemetryUpdate(telesub *TelemetrySubscription) error {
	teleses.Server.telemCtrl.unregister(telesub)
	close(telesub.stop)
	if telesub.eventque != nil {
		close(telesub.eventque)
	}
	return nil
}

func (teleses *TelemetrySession) sendTelemetryUpdate(responses []*gnmipb.SubscribeResponse) error {
	for _, response := range responses {
		teleses.respchan <- response
	}
	return nil
}

func getDeletes(telesub *TelemetrySubscription, path *string, deleteOnly bool) ([]*gnmipb.Path, error) {
	deletes := make([]*gnmipb.Path, 0, telesub.deletedList.Size())
	dpaths := telesub.deletedList.PrefixSearch(*path)
	for _, dpath := range dpaths {
		if deleteOnly {
			if _, ok := telesub.updatedList.Find(dpath); ok {
				continue
			}
		}
		datapath, err := xpath.ToGNMIPath(dpath)
		if err != nil {
			return nil, fmt.Errorf("path-conversion-error(%s)", dpath)
		}
		deletes = append(deletes, datapath)
	}
	return deletes, nil
}

func getUpdates(telesub *TelemetrySubscription, data *model.DataAndPath, encoding gnmipb.Encoding) (*gnmipb.Update, error) {
	typedValue, err := ygot.EncodeTypedValue(data.Value, encoding)
	if err != nil {
		return nil, fmt.Errorf("encoding-error(%s)", err.Error())
	}
	if typedValue == nil {
		return nil, nil
	}
	datapath, err := xpath.ToGNMIPath(data.Path)
	if err != nil {
		return nil, fmt.Errorf("update-path-conversion-error(%s)", data.Path)
	}
	var duplicates uint32
	if telesub != nil {
		for _, v := range telesub.updatedList.All(data.Path) {
			duplicates += v.(uint32)
		}
	}
	return &gnmipb.Update{Path: datapath, Val: typedValue, Duplicates: duplicates}, nil
}

func (teleses *TelemetrySession) requestStateSync(prefix *gnmipb.Path, paths []*gnmipb.Path) bool {
	return teleses.Model.RequestStateSync(prefix, paths)
}

// initTelemetryUpdate - Process and generate responses for a init update.
func (teleses *TelemetrySession) initTelemetryUpdate(req *gnmipb.SubscribeRequest) error {
	bundling := !teleses.disableBundling
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
	case gnmipb.SubscriptionList_POLL:
	case gnmipb.SubscriptionList_ONCE:
	case gnmipb.SubscriptionList_STREAM:
	}
	if useAliases {
		// 1. lookup the prefix in the session.alias for client.alias.
		// 1. lookup the prefix in the server.alias for server.alias.
		// prefix = nil
		// alias = xxx
	}

	teleses.RLock()
	defer teleses.RUnlock()
	if err := xpath.ValidateGNMIPath(prefix); err != nil {
		return status.Errorf(codes.Unimplemented, "invalid-path(%s)", err.Error())
	}
	toplist, ok := teleses.Find(teleses.GetRoot(), prefix)
	if !ok || len(toplist) <= 0 {
		if ok = teleses.ValidatePathSchema(prefix); ok {
			// data-missing is not an error in SubscribeRPC
			// doest send any of messages ahead of the sync response.
			return teleses.sendTelemetryUpdate(buildSyncResponse())
		}
		return status.Errorf(codes.NotFound, "unknown-schema(%s)", xpath.ToXPath(prefix))
	}

	updates := []*gnmipb.Update{}
	for _, top := range toplist {
		bpath := top.Path
		branch := top.Value.(ygot.GoStruct)
		bprefix, err := xpath.ToGNMIPath(bpath)
		if err != nil {
			return status.Errorf(codes.Internal, "path-conversion-error(%s)", bprefix)
		}
		if bundling {
			updates = make([]*gnmipb.Update, 0, 16)
		}
		for _, updateEntry := range subList {
			path := updateEntry.Path
			if err := xpath.ValidateGNMIFullPath(prefix, path); err != nil {
				return status.Errorf(codes.Unimplemented, "invalid-path(%s)", err.Error())
			}
			datalist, ok := teleses.Find(branch, path)
			if !ok || len(datalist) <= 0 {
				continue
			}
			for _, data := range datalist {
				u, err := getUpdates(nil, data, encoding)
				if err != nil {
					return status.Error(codes.Internal, err.Error())
				}
				if u != nil {
					if bundling {
						updates = append(updates, u)
					} else {
						err = teleses.sendTelemetryUpdate(
							buildSubscribeResponse(prefix, alias, []*gnmipb.Update{u}, nil))
						if err != nil {
							return err
						}
					}
				}
			}
		}
		if bundling && len(updates) > 0 {
			err = teleses.sendTelemetryUpdate(
				buildSubscribeResponse(prefix, alias, updates, nil))
			if err != nil {
				return err
			}
		}
	}
	return teleses.sendTelemetryUpdate(buildSyncResponse())
}

// telemetryUpdate - Process and generate responses for a telemetry update.
func (teleses *TelemetrySession) telemetryUpdate(telesub *TelemetrySubscription, updatedroot ygot.GoStruct) error {
	bundling := !teleses.disableBundling
	prefix := telesub.Prefix
	encoding := telesub.Encoding
	useAliases := telesub.UseAliases
	mode := telesub.StreamingMode
	alias := ""
	// [FIXME] Are they different?
	switch mode {
	case gnmipb.SubscriptionList_POLL:
	case gnmipb.SubscriptionList_ONCE:
	case gnmipb.SubscriptionList_STREAM:
	}
	if useAliases {
		// 1. lookup the prefix in the session.alias for client.alias.
		// 2. lookup the prefix in the server.alias for server.alias.
		// prefix = nil
		// alias = xxx
	}

	teleses.RLock()
	defer teleses.RUnlock()
	if updatedroot == nil {
		updatedroot = teleses.GetRoot()
	}
	if err := xpath.ValidateGNMIPath(prefix); err != nil {
		return status.Errorf(codes.Unimplemented, "invalid-path(%s)", err.Error())
	}
	toplist, ok := teleses.Find(updatedroot, prefix)
	if !ok || len(toplist) <= 0 {
		if ok = teleses.ValidatePathSchema(prefix); ok {
			// data-missing is not an error in SubscribeRPC
			// doest send any of messages.
			return nil
		}
		return status.Errorf(codes.NotFound, "unknown-schema(%s)", xpath.ToXPath(prefix))
	}

	deletes := []*gnmipb.Path{}
	updates := []*gnmipb.Update{}

	for _, top := range toplist {
		bpath := top.Path
		branch := top.Value.(ygot.GoStruct)
		bprefix, err := xpath.ToGNMIPath(bpath)
		if err != nil {
			return status.Errorf(codes.Internal, "path-conversion-error(%s)", bprefix)
		}

		if bundling {
			updates = make([]*gnmipb.Update, 0, 16)
			// get all replaced, deleted paths relative to the prefix
			deletes, err = getDeletes(telesub, &bpath, false)
			if err != nil {
				return status.Error(codes.Internal, err.Error())
			}
		}

		for _, path := range telesub.Paths {
			if err := xpath.ValidateGNMIFullPath(prefix, path); err != nil {
				return status.Errorf(codes.Unimplemented, "invalid-path(%s)", err.Error())
			}
			datalist, ok := teleses.Find(branch, path)
			if !ok || len(datalist) <= 0 {
				continue
			}
			for _, data := range datalist {
				u, err := getUpdates(telesub, data, encoding)
				if err != nil {
					return status.Error(codes.Internal, err.Error())
				}
				if u != nil {
					if bundling {
						updates = append(updates, u)
					} else {
						fullpath := bpath + data.Path
						deletes, err = getDeletes(telesub, &fullpath, false)
						if err != nil {
							return status.Error(codes.Internal, err.Error())
						}
						err = teleses.sendTelemetryUpdate(
							buildSubscribeResponse(prefix, alias, []*gnmipb.Update{u}, nil))
						if err != nil {
							return err
						}
					}
				}
			}
		}
		if bundling {
			err = teleses.sendTelemetryUpdate(
				buildSubscribeResponse(prefix, alias, updates, deletes))
			if err != nil {
				return err
			}
		} else {
			deletes, err = getDeletes(telesub, &bpath, true)
			if err != nil {
				return status.Error(codes.Internal, err.Error())
			}
			for _, d := range deletes {
				err = teleses.sendTelemetryUpdate(
					buildSubscribeResponse(prefix, alias, nil, []*gnmipb.Path{d}))
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

const (
	defaultInterval = 60000000000
	minimumInterval = 1000000000
)

func (teleses *TelemetrySession) addStreamSubscription(
	prefix *gnmipb.Path, useAliases bool, streamingMode gnmipb.SubscriptionList_Mode, allowAggregation bool,
	encoding gnmipb.Encoding, paths []*gnmipb.Path, subscriptionMode gnmipb.SubscriptionMode,
	sampleInterval uint64, suppressRedundant bool, heartbeatInterval uint64, stateSync bool,
) (*TelemetrySubscription, error) {

	telesub := &TelemetrySubscription{
		SessionID:         teleses.ID,
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
		updatedList:       gtrie.New(),
		deletedList:       gtrie.New(),
		stateSync:         stateSync,
		mutex:             &sync.Mutex{},
	}
	if streamingMode == gnmipb.SubscriptionList_POLL {
		return nil, status.Errorf(codes.InvalidArgument,
			"poll subscription configured as streaming subscription")
	}
	// 3.5.1.5.2 STREAM Subscriptions Must be satisfied for telemetry update starting.
	switch telesub.SubscriptionMode {
	case gnmipb.SubscriptionMode_TARGET_DEFINED:
		// vendor specific mode
		telesub.Configured.SubscriptionMode = gnmipb.SubscriptionMode_SAMPLE
		telesub.Configured.SampleInterval = defaultInterval / 10
		telesub.Configured.SuppressRedundant = true
		telesub.Configured.HeartbeatInterval = 0
	case gnmipb.SubscriptionMode_ON_CHANGE:
		if telesub.HeartbeatInterval < minimumInterval && telesub.HeartbeatInterval != 0 {
			return nil, status.Errorf(codes.InvalidArgument,
				"heartbeat_interval(< 1sec) is not supported")
		}
		telesub.Configured.SubscriptionMode = gnmipb.SubscriptionMode_ON_CHANGE
		telesub.Configured.SampleInterval = 0
		telesub.Configured.SuppressRedundant = false
		telesub.Configured.HeartbeatInterval = telesub.HeartbeatInterval
	case gnmipb.SubscriptionMode_SAMPLE:
		if telesub.SampleInterval < minimumInterval && telesub.SampleInterval != 0 {
			return nil, status.Errorf(codes.InvalidArgument,
				"sample_interval(< 1sec) is not supported")
		}
		if telesub.HeartbeatInterval < minimumInterval && telesub.HeartbeatInterval != 0 {
			return nil, status.Errorf(codes.InvalidArgument,
				"heartbeat_interval(< 1sec) is not supported")
		}
		telesub.Configured.SubscriptionMode = gnmipb.SubscriptionMode_SAMPLE
		telesub.Configured.SampleInterval = telesub.SampleInterval
		if telesub.SampleInterval == 0 {
			// Set minimal sampling interval (1sec)
			telesub.Configured.SampleInterval = minimumInterval
		}
		telesub.Configured.SuppressRedundant = telesub.SuppressRedundant
		telesub.Configured.HeartbeatInterval = telesub.HeartbeatInterval
	}
	key := fmt.Sprintf("%d-%s-%s-%s-%s-%d-%d-%t-%t-%t",
		telesub.SessionID,
		telesub.StreamingMode, telesub.Encoding, telesub.SubscriptionMode,
		xpath.ToXPath(telesub.Prefix), telesub.SampleInterval, telesub.HeartbeatInterval,
		telesub.UseAliases, telesub.AllowAggregation, telesub.SuppressRedundant,
	)
	telesub.key = &key
	telesub.eventque = make(chan *telemEvent, 64)
	if t, ok := teleses.Telesub[key]; ok {
		// only updates the new path if the telesub exists.
		telesub = t
		telesub.mutex.Lock()
		defer telesub.mutex.Unlock()
		telesub.Paths = append(telesub.Paths, paths...)
		if !telesub.stateSync {
			telesub.stateSync = stateSync
		}
		glog.Infof("telemetry[%d][%d].add.path(%s)", teleses.ID, telesub.ID, xpath.ToXPath(paths[0]))
		return telesub, nil
	}
	subID++
	id := subID
	telesub.ID = id
	telesub.session = teleses
	teleses.Telesub[key] = telesub
	glog.Infof("telemetry[%d][%d].new(%s)", teleses.ID, telesub.ID, *telesub.key)
	glog.Infof("telemetry[%d][%d].add.path(%s)", teleses.ID, telesub.ID, xpath.ToXPath(paths[0]))
	return telesub, nil
}

// addPollSubscription - Create new TelemetrySubscription
func (teleses *TelemetrySession) addPollSubscription() error {
	telesub := &TelemetrySubscription{
		StreamingMode: gnmipb.SubscriptionList_POLL,
		Paths:         []*gnmipb.Path{},
		IsPolling:     true,
		session:       teleses,
	}
	subID++
	id := subID
	key := fmt.Sprintf("%s", telesub.StreamingMode)
	telesub.ID = id
	telesub.key = &key
	teleses.Telesub[key] = telesub
	glog.Infof("telemetry[%d][%d].new(%s)", teleses.ID, telesub.ID, *telesub.key)
	return nil
}

func (teleses *TelemetrySession) updateAliases(aliaslist []*gnmipb.Alias) error {
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

func processSR(teleses *TelemetrySession, req *gnmipb.SubscribeRequest) error {
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

	if err := teleses.CheckModels(useModules); err != nil {
		return status.Errorf(codes.Unimplemented, err.Error())
	}
	if err := teleses.checkEncoding(encoding); err != nil {
		return err
	}
	paths := make([]*gnmipb.Path, 0, subListLength)
	for _, updateEntry := range subList {
		paths = append(paths, updateEntry.Path)
	}
	stateSync := teleses.RequestStateSync(subscriptionList.Prefix, paths)

	err := teleses.initTelemetryUpdate(req)
	mode := subscriptionList.GetMode()
	if mode == gnmipb.SubscriptionList_ONCE ||
		mode == gnmipb.SubscriptionList_POLL ||
		err != nil {
		return err
	}

	prefix := subscriptionList.GetPrefix()
	useAliases := subscriptionList.GetUseAliases()
	allowAggregation := subscriptionList.GetAllowAggregation()
	startingList := make([]*TelemetrySubscription, 0, subListLength)
	for _, updateEntry := range subList {
		path := updateEntry.GetPath()
		submod := updateEntry.GetMode()
		SampleInterval := updateEntry.GetSampleInterval()
		supressRedundant := updateEntry.GetSuppressRedundant()
		heartBeatInterval := updateEntry.GetHeartbeatInterval()
		fullpath := xpath.GNMIFullPath(prefix, path)
		_, ok := teleses.FindAllPaths(fullpath)
		if !ok {
			return status.Errorf(codes.InvalidArgument,
				"schema not found for %s", xpath.ToXPath(fullpath))
		}
		telesub, err := teleses.addStreamSubscription(
			prefix, useAliases, gnmipb.SubscriptionList_STREAM,
			allowAggregation, encoding, []*gnmipb.Path{path}, submod,
			SampleInterval, supressRedundant, heartBeatInterval, stateSync)
		if err != nil {
			return err
		}
		startingList = append(startingList, telesub)
	}

	addDynamicTeleSub(teleses.Model, startingList)
	for _, telesub := range startingList {
		teleses.StartTelmetryUpdate(telesub)
	}
	return nil
}

var dynamicTeleSubInfoFormat string = `
telemetry-system:
 subscriptions:
  dynamic-subscriptions:
   dynamic-subscription[id=%d]:
    id: %d
    state:
     id: %d
     destination-address: %s
     destination-port: %d
     sample-interval: %d
     heartbeat-interval: %d
     suppress-redundant: %v
     protocol: %s
     encoding: ENC_%s
    sensor-paths:`

var dynamicTeleSubInfoPathFormat string = `
     sensor-path[path=%s]:
      state:
       path: %s`

func addDynamicTeleSub(m *model.Model, telesubs []*TelemetrySubscription) {
	switch y := m.StateSync.(type) {
	case *ydb.YDB:
		s := ""
		for _, telesub := range telesubs {
			s += fmt.Sprintf(dynamicTeleSubInfoFormat,
				telesub.ID, telesub.ID, telesub.ID,
				telesub.session.Address,
				telesub.session.Port,
				telesub.Configured.SampleInterval,
				telesub.Configured.HeartbeatInterval,
				telesub.Configured.SuppressRedundant,
				"STREAM_GRPC",
				telesub.Encoding,
			)
			for i := range telesub.Paths {
				p := xpath.ToXPath(telesub.Paths[i])
				s += fmt.Sprintf(dynamicTeleSubInfoPathFormat, p, p)
			}
		}
		if s != "" {
			y.Write([]byte(s))
		}
	}
}

func delDynamicTeleSub(m *model.Model, teleses *TelemetrySession) {
	switch y := m.StateSync.(type) {
	case *ydb.YDB:
		s := ""
		for _, telesub := range teleses.Telesub {
			s += fmt.Sprintf(`
telemetry-system:
 subscriptions:
  dynamic-subscriptions:
   dynamic-subscription[id=%d]:
`, telesub.ID)
		}
		if s != "" {
			y.Delete([]byte(s))
		}
	}
}
