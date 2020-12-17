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
	"github.com/neoul/gnxi/utilities/status"
	"github.com/neoul/gnxi/utilities/xpath"
	"github.com/neoul/gtrie"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ygot/ygot"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
)

// TeleID - Telemetry Session ID
type TeleID uint64

type teleEvent struct {
	sub         *Subscription
	updatedroot ygot.GoStruct
	updatedPath []*string
	deletedPath []*string
}

// gNMI Telemetry Control Block
type teleCtrl struct {
	// lookup map[TeleID]*Subscription using a path
	lookup *gtrie.Trie
	ready  map[TeleID]*teleEvent
	mutex  *sync.Mutex
}

func newTeleCtrl() *teleCtrl {
	return &teleCtrl{
		lookup: gtrie.New(),
		ready:  make(map[TeleID]*teleEvent),
		mutex:  &sync.Mutex{},
	}
}

func (tcb *teleCtrl) register(m *model.Model, sub *Subscription) error {
	tcb.mutex.Lock()
	defer tcb.mutex.Unlock()
	for _, path := range sub.Paths {
		fullpath := xpath.GNMIFullPath(sub.Prefix, path)
		targetpaths, _ := m.FindAllPaths(fullpath)
		for _, p := range targetpaths {
			if subgroup, ok := tcb.lookup.Find(p); ok {
				subgroup.(map[TeleID]*Subscription)[sub.ID] = sub
			} else {
				tcb.lookup.Add(p, map[TeleID]*Subscription{sub.ID: sub})
			}
			glog.Infof("teleCtrl.sub[%d][%d].subscribe(%v)", sub.SessionID, sub.ID, p)
		}
	}
	return nil
}

func (tcb *teleCtrl) unregister(sub *Subscription) {
	tcb.mutex.Lock()
	defer tcb.mutex.Unlock()
	lookup := tcb.lookup.All("")
	for p, subgroup := range lookup {
		subscriber := subgroup.(map[TeleID]*Subscription)
		_, ok := subscriber[sub.ID]
		if ok {
			glog.Infof("teleCtrl.sub[%d][%d].unsubscribe(%v)", sub.SessionID, sub.ID, p)
			delete(subscriber, sub.ID)
			if len(subscriber) == 0 {
				tcb.lookup.Remove(p)
			}
		}
	}
}

// ChangeNotification.ChangeStarted - callback for Telemetry subscription on data changes
func (tcb *teleCtrl) ChangeStarted(changes ygot.GoStruct) {
	tcb.mutex.Lock()
	defer tcb.mutex.Unlock()
	tcb.ready = make(map[TeleID]*teleEvent)
	// glog.Infof("teleCtrl.ChangeStarted")
}

func (tcb *teleCtrl) updateTeleEvent(op int, datapath, searchpath string) {
	for _, subgroup := range tcb.lookup.FindAll(searchpath) {
		subscribers := subgroup.(map[TeleID]*Subscription)
		for _, sub := range subscribers {
			if sub.IsPolling {
				continue
			}
			// glog.Infof("teleCtrl.%c.path(%s).sub[%d][%d]",
			// 	op, subscribedpath, sub.SessionID, sub.ID)
			event, ok := tcb.ready[sub.ID]
			if !ok {
				event = &teleEvent{
					sub:         sub,
					updatedPath: make([]*string, 0, 64),
					deletedPath: make([]*string, 0, 64),
				}
				tcb.ready[sub.ID] = event
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
func (tcb *teleCtrl) ChangeCreated(datapath string, changes ygot.GoStruct) {
	tcb.mutex.Lock()
	defer tcb.mutex.Unlock()
	glog.Infof("teleCtrl.ChangeCreated.path(%s)", datapath)
	gpath, err := xpath.ToGNMIPath(datapath)
	if err != nil {
		return
	}
	tcb.updateTeleEvent('C', datapath, datapath)
	if !xpath.IsSchemaPath(gpath) {
		tcb.updateTeleEvent('C', datapath, xpath.PathElemToXPATH(gpath.GetElem(), true))
	}
}

// ChangeNotification.ChangeReplaced - callback for Telemetry subscription on data changes
func (tcb *teleCtrl) ChangeReplaced(datapath string, changes ygot.GoStruct) {
	tcb.mutex.Lock()
	defer tcb.mutex.Unlock()
	glog.Infof("teleCtrl.ChangeReplaced.path(%s)", datapath)
	gpath, err := xpath.ToGNMIPath(datapath)
	if err != nil {
		return
	}
	tcb.updateTeleEvent('R', datapath, datapath)
	if !xpath.IsSchemaPath(gpath) {
		tcb.updateTeleEvent('R', datapath, xpath.PathElemToXPATH(gpath.GetElem(), true))
	}
}

// ChangeNotification.ChangeDeleted - callback for Telemetry subscription on data changes
func (tcb *teleCtrl) ChangeDeleted(datapath string) {
	tcb.mutex.Lock()
	defer tcb.mutex.Unlock()
	glog.Infof("teleCtrl.ChangeDeleted.path(%s)", datapath)
	gpath, err := xpath.ToGNMIPath(datapath)
	if err != nil {
		return
	}
	tcb.updateTeleEvent('D', datapath, datapath)
	if !xpath.IsSchemaPath(gpath) {
		tcb.updateTeleEvent('D', datapath, xpath.PathElemToXPATH(gpath.GetElem(), true))
	}
}

// ChangeNotification.ChangeCompleted - callback for Telemetry subscription on data changes
func (tcb *teleCtrl) ChangeCompleted(changes ygot.GoStruct) {
	tcb.mutex.Lock()
	defer tcb.mutex.Unlock()
	for telesubid, event := range tcb.ready {
		sub := event.sub
		if sub.eventque != nil {
			event.updatedroot = changes
			sub.eventque <- event
			glog.Infof("teleCtrl.send.event.to.sub[%d][%d]", sub.SessionID, sub.ID)
		}
		delete(tcb.ready, telesubid)
	}
	// glog.Infof("teleCtrl.ChangeCompleted")
}

// SubSession - gNMI gRPC Subscribe RPC (Telemetry) session information managed by server
type SubSession struct {
	ID        TeleID
	Address   string
	Port      uint16
	SubList   map[string]*Subscription
	respchan  chan *gnmipb.SubscribeResponse
	shutdown  chan struct{}
	waitgroup *sync.WaitGroup
	*clientAliases
	*Server
}

var (
	sessID TeleID
	subID  TeleID
)

func (subses *SubSession) String() string {
	return fmt.Sprintf("%s:%d", subses.Address, subses.Port)
}

func newSubSession(ctx context.Context, s *Server) *SubSession {
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
	return &SubSession{
		ID:            sessID,
		Address:       address,
		Port:          uint16(port),
		SubList:       map[string]*Subscription{},
		respchan:      make(chan *gnmipb.SubscribeResponse, 256),
		shutdown:      make(chan struct{}),
		waitgroup:     new(sync.WaitGroup),
		clientAliases: newClientAliases(),
		Server:        s,
	}
}

func (subses *SubSession) stopSubSession() {
	subses.deleteDynamicSubscriptionInfo(subses)
	for _, sub := range subses.SubList {
		subses.unregister(sub)
	}
	close(subses.shutdown)
	subses.waitgroup.Wait()
}

// Subscription - Default structure for Telemetry Update Subscription
type Subscription struct {
	ID                TeleID
	SessionID         TeleID
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
	session     *SubSession
	stateSync   bool // StateSync is required before telemetry update?
	eventque    chan *teleEvent
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

func (sub *Subscription) run(subses *SubSession) {
	var samplingTimer *time.Ticker
	var heartbeatTimer *time.Ticker
	shutdown := subses.shutdown
	waitgroup := subses.waitgroup
	defer func() {
		glog.Warningf("sub[%d][%d].quit", sub.SessionID, sub.ID)
		sub.started = false
		waitgroup.Done()
	}()
	if sub.Configured.SampleInterval > 0 {
		tick := time.Duration(sub.Configured.SampleInterval)
		samplingTimer = time.NewTicker(tick * time.Nanosecond)
	} else {
		tick := time.Duration(defaultInterval)
		samplingTimer = time.NewTicker(tick * time.Nanosecond)
		samplingTimer.Stop() // stop
	}
	if sub.Configured.HeartbeatInterval > 0 {
		tick := time.Duration(sub.Configured.HeartbeatInterval)
		heartbeatTimer = time.NewTicker(tick * time.Nanosecond)
	} else {
		tick := time.Duration(defaultInterval)
		heartbeatTimer = time.NewTicker(tick * time.Nanosecond)
		heartbeatTimer.Stop() // stop
	}
	if samplingTimer == nil || heartbeatTimer == nil {
		glog.Errorf("sub[%d][%d].timer-failed", sub.SessionID, sub.ID)
		return
	}
	timerExpired := make(chan bool, 2)

	for {
		select {
		case event, ok := <-sub.eventque:
			if !ok {
				glog.Errorf("sub[%d][%d].event-queue-closed", sub.SessionID, sub.ID)
				return
			}
			glog.Infof("sub[%d][%d].event-received", sub.SessionID, sub.ID)
			switch sub.Configured.SubscriptionMode {
			case gnmipb.SubscriptionMode_ON_CHANGE, gnmipb.SubscriptionMode_SAMPLE:
				for _, p := range event.updatedPath {
					duplicates := uint32(1)
					if v, ok := sub.updatedList.Find(*p); ok {
						duplicates = v.(uint32)
						duplicates++
					}
					sub.updatedList.Add(*p, duplicates)
				}
				for _, p := range event.deletedPath {
					duplicates := uint32(1)
					if v, ok := sub.deletedList.Find(*p); ok {
						duplicates = v.(uint32)
						duplicates++
					}
					sub.deletedList.Add(*p, duplicates)
				}
				if sub.Configured.SubscriptionMode == gnmipb.SubscriptionMode_ON_CHANGE {
					err := subses.telemetryUpdate(sub, event.updatedroot)
					if err != nil {
						glog.Errorf("sub[%d][%d].failed(%v)", sub.SessionID, sub.ID, err)
						return
					}
					sub.updatedList = gtrie.New()
					sub.deletedList = gtrie.New()
				}
			}
		case <-samplingTimer.C:
			glog.Infof("sub[%d][%d].sampling-timer-expired", sub.SessionID, sub.ID)
			if sub.stateSync {
				sub.mutex.Lock()
				sub.session.RequestStateSync(sub.Prefix, sub.Paths)
				sub.mutex.Unlock()
			}
			timerExpired <- false
		case <-heartbeatTimer.C:
			glog.Infof("sub[%d][%d].heartbeat-timer-expired", sub.SessionID, sub.ID)
			timerExpired <- true
		case sendUpdate := <-timerExpired:
			if !sendUpdate {
				// suppress_redundant - skips the telemetry update if no changes
				if !sub.Configured.SuppressRedundant ||
					sub.updatedList.Size() > 0 ||
					sub.deletedList.Size() > 0 {
					sendUpdate = true
				}
			}
			if sendUpdate {
				glog.Infof("sub[%d][%d].send.telemetry-update.to.%v",
					sub.SessionID, sub.ID, sub.session)
				sub.mutex.Lock() // block to modify sub.Path
				err := subses.telemetryUpdate(sub, nil)
				sub.mutex.Unlock()
				if err != nil {
					glog.Errorf("sub[%d][%d].failed(%v)", sub.SessionID, sub.ID, err)
					return
				}
				sub.updatedList = gtrie.New()
				sub.deletedList = gtrie.New()
			}
		case <-shutdown:
			glog.Infof("sub[%d][%d].shutdown", subses.ID, sub.ID)
			return
		case <-sub.stop:
			glog.Infof("sub[%d][%d].stopped", subses.ID, sub.ID)
			return
		}
	}
}

// StartTelmetryUpdate - returns a key for telemetry comparison
func (subses *SubSession) StartTelmetryUpdate(sub *Subscription) error {
	subses.Server.teleCtrl.register(subses.Model, sub)
	if !sub.started {
		sub.started = true
		subses.waitgroup.Add(1)
		go sub.run(subses)
	}
	return nil
}

// StopTelemetryUpdate - returns a key for telemetry comparison
func (subses *SubSession) StopTelemetryUpdate(sub *Subscription) error {
	subses.Server.teleCtrl.unregister(sub)
	close(sub.stop)
	if sub.eventque != nil {
		close(sub.eventque)
	}
	return nil
}

func (subses *SubSession) sendTelemetryUpdate(responses []*gnmipb.SubscribeResponse) error {
	for _, response := range responses {
		subses.respchan <- response
	}
	return nil
}

func getDeletes(sub *Subscription, path *string, deleteOnly bool) ([]*gnmipb.Path, error) {
	deletes := make([]*gnmipb.Path, 0, sub.deletedList.Size())
	dpaths := sub.deletedList.PrefixSearch(*path)
	for _, dpath := range dpaths {
		if deleteOnly {
			if _, ok := sub.updatedList.Find(dpath); ok {
				continue
			}
		}
		datapath, err := xpath.ToGNMIPath(dpath)
		if err != nil {
			return nil, status.TaggedErrorf(codes.Internal, status.TagInvalidPath,
				"xpath-to-gpath converting error for %s", dpath)
		}
		deletes = append(deletes, datapath)
	}
	return deletes, nil
}

func getUpdates(sub *Subscription, data *model.DataAndPath, encoding gnmipb.Encoding) (*gnmipb.Update, error) {
	typedValue, err := ygot.EncodeTypedValue(data.Value, encoding)
	if err != nil {
		return nil, status.TaggedErrorf(codes.Internal, status.TagBadData,
			"typed-value encoding error in %s: %v", data.Path, err)
	}
	if typedValue == nil {
		return nil, nil
	}
	datapath, err := xpath.ToGNMIPath(data.Path)
	if err != nil {
		return nil, status.TaggedErrorf(codes.Internal, status.TagInvalidPath,
			"xpath-to-gpath converting error for %s", data.Path)
	}
	var duplicates uint32
	if sub != nil {
		for _, v := range sub.updatedList.All(data.Path) {
			duplicates += v.(uint32)
		}
	}
	return &gnmipb.Update{Path: datapath, Val: typedValue, Duplicates: duplicates}, nil
}

func (subses *SubSession) aliasesUpdate() {
	aliases := subses.UpdateAliases(subses.serverAliases, true)
	for _, alias := range aliases {
		subses.sendTelemetryUpdate(
			buildAliasResponse(subses.ToPath(alias, true).(*gnmipb.Path), alias))
	}
}

// initTelemetryUpdate - Process and generate responses for a init update.
func (subses *SubSession) initTelemetryUpdate(req *gnmipb.SubscribeRequest) error {
	bundling := !subses.disableBundling
	subscriptionList := req.GetSubscribe()
	subList := subscriptionList.GetSubscription()
	updateOnly := subscriptionList.GetUpdatesOnly()
	if updateOnly {
		return subses.sendTelemetryUpdate(buildSyncResponse())
	}
	prefix := subscriptionList.GetPrefix()
	encoding := subscriptionList.GetEncoding()
	mode := subscriptionList.GetMode()
	// [FIXME] Are they different?
	switch mode {
	case gnmipb.SubscriptionList_POLL:
	case gnmipb.SubscriptionList_ONCE:
	case gnmipb.SubscriptionList_STREAM:
	}

	subses.RLock()
	defer subses.RUnlock()
	if err := xpath.ValidateGNMIPath(prefix); err != nil {
		return status.TaggedErrorf(codes.InvalidArgument, status.TagInvalidPath,
			"prefix validation failed: %s", err)
	}
	toplist, ok := subses.Find(subses.GetRoot(), prefix)
	if !ok || len(toplist) <= 0 {
		if ok = subses.ValidatePathSchema(prefix); ok {
			// data-missing is not an error in SubscribeRPC
			// doest send any of messages ahead of the sync response.
			return subses.sendTelemetryUpdate(buildSyncResponse())
		}
		return status.TaggedErrorf(codes.NotFound, status.TagUnknownPath,
			"unable to find %s from the schema tree", xpath.ToXPath(prefix))
	}

	updates := []*gnmipb.Update{}
	for _, top := range toplist {
		branch := top.Value.(ygot.GoStruct)
		if bundling {
			updates = make([]*gnmipb.Update, 0, 16)
		}
		for _, updateEntry := range subList {
			path := updateEntry.Path
			if err := xpath.ValidateGNMIFullPath(prefix, path); err != nil {
				return status.TaggedErrorf(codes.Unimplemented, status.TagInvalidPath,
					"full path (%s + %s) validation failed: %v",
					xpath.ToXPath(prefix), xpath.ToXPath(path), err)
			}
			datalist, ok := subses.Find(branch, path)
			if !ok || len(datalist) <= 0 {
				continue
			}
			for _, data := range datalist {
				u, err := getUpdates(nil, data, encoding)
				if err != nil {
					return err
				}
				if u != nil {
					if bundling {
						updates = append(updates, u)
					} else {
						aliasPrefix := subses.ToAlias(prefix, false).(*gnmipb.Path)
						err = subses.sendTelemetryUpdate(
							buildSubscribeResponse(aliasPrefix, []*gnmipb.Update{u}, nil))
						if err != nil {
							return err
						}
					}
				}
			}
		}
		if bundling && len(updates) > 0 {
			err := subses.sendTelemetryUpdate(
				buildSubscribeResponse(prefix, updates, nil))
			if err != nil {
				return err
			}
		}
	}
	return subses.sendTelemetryUpdate(buildSyncResponse())
}

// telemetryUpdate - Process and generate responses for a telemetry update.
func (subses *SubSession) telemetryUpdate(sub *Subscription, updatedroot ygot.GoStruct) error {
	bundling := !subses.disableBundling
	prefix := sub.Prefix
	encoding := sub.Encoding
	mode := sub.StreamingMode

	// [FIXME] Are they different?
	switch mode {
	case gnmipb.SubscriptionList_POLL:
	case gnmipb.SubscriptionList_ONCE:
	case gnmipb.SubscriptionList_STREAM:
	}

	subses.RLock()
	defer subses.RUnlock()
	if updatedroot == nil {
		updatedroot = subses.GetRoot()
	}
	if err := xpath.ValidateGNMIPath(prefix); err != nil {
		return status.TaggedErrorf(codes.InvalidArgument, status.TagInvalidPath,
			"prefix validation failed: %s", err)
	}
	toplist, ok := subses.Find(updatedroot, prefix)
	if !ok || len(toplist) <= 0 {
		if ok = subses.ValidatePathSchema(prefix); ok {
			// data-missing is not an error in SubscribeRPC
			// does not send any of messages.
			return nil
		}
		return status.TaggedErrorf(codes.NotFound, status.TagUnknownPath,
			"unable to find %s from the schema tree", xpath.ToXPath(prefix))
	}

	deletes := []*gnmipb.Path{}
	updates := []*gnmipb.Update{}

	for _, top := range toplist {
		var err error
		bpath := top.Path
		branch := top.Value.(ygot.GoStruct)
		if bundling {
			updates = make([]*gnmipb.Update, 0, 16)
			// get all replaced, deleted paths relative to the prefix
			deletes, err = getDeletes(sub, &bpath, false)
			if err != nil {
				return err
			}
		}

		for _, path := range sub.Paths {
			if err := xpath.ValidateGNMIFullPath(prefix, path); err != nil {
				return status.TaggedErrorf(codes.Unimplemented, status.TagInvalidPath,
					"full path (%s + %s) validation failed: %v",
					xpath.ToXPath(prefix), xpath.ToXPath(path), err)
			}
			datalist, ok := subses.Find(branch, path)
			if !ok || len(datalist) <= 0 {
				continue
			}
			for _, data := range datalist {
				u, err := getUpdates(sub, data, encoding)
				if err != nil {
					return err
				}
				if u != nil {
					if bundling {
						updates = append(updates, u)
					} else {
						fullpath := bpath + data.Path
						deletes, err = getDeletes(sub, &fullpath, false)
						if err != nil {
							return err
						}
						aliasPrefix := subses.ToAlias(prefix, false).(*gnmipb.Path)
						err = subses.sendTelemetryUpdate(
							buildSubscribeResponse(aliasPrefix, []*gnmipb.Update{u}, nil))
						if err != nil {
							return err
						}
					}
				}
			}
		}
		if bundling {
			aliasPrefix := subses.ToAlias(prefix, false).(*gnmipb.Path)
			err = subses.sendTelemetryUpdate(
				buildSubscribeResponse(aliasPrefix, updates, deletes))
			if err != nil {
				return err
			}
		} else {
			deletes, err = getDeletes(sub, &bpath, true)
			if err != nil {
				return err
			}
			for _, d := range deletes {
				aliasPrefix := subses.ToAlias(prefix, false).(*gnmipb.Path)
				err = subses.sendTelemetryUpdate(
					buildSubscribeResponse(aliasPrefix, nil, []*gnmipb.Path{d}))
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

func (subses *SubSession) addStreamSubscription(
	prefix *gnmipb.Path, useAliases bool, streamingMode gnmipb.SubscriptionList_Mode, allowAggregation bool,
	encoding gnmipb.Encoding, paths []*gnmipb.Path, subscriptionMode gnmipb.SubscriptionMode,
	sampleInterval uint64, suppressRedundant bool, heartbeatInterval uint64, stateSync bool,
) (*Subscription, error) {

	sub := &Subscription{
		SessionID:         subses.ID,
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
		return nil, status.TaggedErrorf(codes.InvalidArgument, status.TagMalformedMessage,
			"streaming subscription not allowed on poll mode")
	}
	// 3.5.1.5.2 STREAM Subscriptions Must be satisfied for telemetry update starting.
	switch sub.SubscriptionMode {
	case gnmipb.SubscriptionMode_TARGET_DEFINED:
		// vendor specific mode
		sub.Configured.SubscriptionMode = gnmipb.SubscriptionMode_SAMPLE
		sub.Configured.SampleInterval = defaultInterval / 10
		sub.Configured.SuppressRedundant = true
		sub.Configured.HeartbeatInterval = 0
	case gnmipb.SubscriptionMode_ON_CHANGE:
		if sub.HeartbeatInterval < minimumInterval && sub.HeartbeatInterval != 0 {
			return nil, status.TaggedErrorf(codes.OutOfRange, status.TagInvalidConfig,
				"heartbeat_interval(!= 0sec and < 1sec) is not supported")
		}
		sub.Configured.SubscriptionMode = gnmipb.SubscriptionMode_ON_CHANGE
		sub.Configured.SampleInterval = 0
		sub.Configured.SuppressRedundant = false
		sub.Configured.HeartbeatInterval = sub.HeartbeatInterval
	case gnmipb.SubscriptionMode_SAMPLE:
		if sub.SampleInterval < minimumInterval && sub.SampleInterval != 0 {
			return nil, status.TaggedErrorf(codes.OutOfRange, status.TagInvalidConfig,
				"sample_interval(!= 0sec and < 1sec) is not supported")
		}
		if sub.HeartbeatInterval < minimumInterval && sub.HeartbeatInterval != 0 {
			return nil, status.TaggedErrorf(codes.OutOfRange, status.TagInvalidConfig,
				"heartbeat_interval(!= 0sec and < 1sec) is not supported")
		}
		sub.Configured.SubscriptionMode = gnmipb.SubscriptionMode_SAMPLE
		sub.Configured.SampleInterval = sub.SampleInterval
		if sub.SampleInterval == 0 {
			// Set minimal sampling interval (1sec)
			sub.Configured.SampleInterval = minimumInterval
		}
		sub.Configured.SuppressRedundant = sub.SuppressRedundant
		sub.Configured.HeartbeatInterval = sub.HeartbeatInterval
	}
	key := fmt.Sprintf("%d-%s-%s-%s-%s-%d-%d-%t-%t-%t",
		sub.SessionID,
		sub.StreamingMode, sub.Encoding, sub.SubscriptionMode,
		xpath.ToXPath(sub.Prefix), sub.SampleInterval, sub.HeartbeatInterval,
		sub.UseAliases, sub.AllowAggregation, sub.SuppressRedundant,
	)
	sub.key = &key
	sub.eventque = make(chan *teleEvent, 64)
	if t, ok := subses.SubList[key]; ok {
		// only updates the new path if the sub exists.
		sub = t
		sub.mutex.Lock()
		defer sub.mutex.Unlock()
		sub.Paths = append(sub.Paths, paths...)
		if !sub.stateSync {
			sub.stateSync = stateSync
		}
		glog.Infof("telemetry[%d][%d].add.path(%s)", subses.ID, sub.ID, xpath.ToXPath(paths[0]))
		return sub, nil
	}
	subID++
	id := subID
	sub.ID = id
	sub.session = subses
	subses.SubList[key] = sub
	glog.Infof("telemetry[%d][%d].new(%s)", subses.ID, sub.ID, *sub.key)
	glog.Infof("telemetry[%d][%d].add.path(%s)", subses.ID, sub.ID, xpath.ToXPath(paths[0]))
	return sub, nil
}

// addPollSubscription - Create new Subscription
func (subses *SubSession) addPollSubscription() error {
	sub := &Subscription{
		StreamingMode: gnmipb.SubscriptionList_POLL,
		Paths:         []*gnmipb.Path{},
		IsPolling:     true,
		session:       subses,
	}
	subID++
	id := subID
	key := fmt.Sprintf("%s", sub.StreamingMode)
	sub.ID = id
	sub.key = &key
	subses.SubList[key] = sub
	glog.Infof("telemetry[%d][%d].new(%s)", subses.ID, sub.ID, *sub.key)
	return nil
}

func (subses *SubSession) processSubscribeRequest(req *gnmipb.SubscribeRequest) error {
	// SubscribeRequest for poll Subscription indication
	pollMode := req.GetPoll()
	if pollMode != nil {
		return subses.addPollSubscription()
	}
	// SubscribeRequest for aliases update
	aliases := req.GetAliases()
	if aliases != nil {
		return subses.SetAliases(aliases.GetAlias())
	}

	// extension := req.GetExtension()
	subscriptionList := req.GetSubscribe()
	if subscriptionList == nil {
		return status.TaggedErrorf(codes.InvalidArgument, status.TagMalformedMessage,
			"no subscription info (SubscriptionList field)")
	}
	// check & update the server aliases and use_aliases
	useAliases := subscriptionList.GetUseAliases()
	if useAliases {
		subses.aliasesUpdate()
	}

	subList := subscriptionList.GetSubscription()
	subListLength := len(subList)
	if subList == nil || subListLength <= 0 {
		return status.TaggedErrorf(codes.InvalidArgument, status.TagMalformedMessage,
			"no subscription info (subscription field)")
	}
	encoding := subscriptionList.GetEncoding()
	useModules := subscriptionList.GetUseModels()

	if err := subses.CheckModels(useModules); err != nil {
		return err
	}
	if err := subses.CheckEncoding(encoding); err != nil {
		return err
	}
	prefix := subscriptionList.GetPrefix()
	paths := make([]*gnmipb.Path, 0, subListLength)
	for _, updateEntry := range subList {
		paths = append(paths, updateEntry.Path)
	}
	stateSync := subses.RequestStateSync(prefix, paths)

	err := subses.initTelemetryUpdate(req)
	mode := subscriptionList.GetMode()
	if mode == gnmipb.SubscriptionList_ONCE ||
		mode == gnmipb.SubscriptionList_POLL ||
		err != nil {
		return err
	}

	allowAggregation := subscriptionList.GetAllowAggregation()
	startingList := make([]*Subscription, 0, subListLength)
	for _, updateEntry := range subList {
		path := updateEntry.GetPath()
		submod := updateEntry.GetMode()
		SampleInterval := updateEntry.GetSampleInterval()
		supressRedundant := updateEntry.GetSuppressRedundant()
		heartBeatInterval := updateEntry.GetHeartbeatInterval()
		fullpath := xpath.GNMIFullPath(prefix, path)
		_, ok := subses.FindAllPaths(fullpath)
		if !ok {
			return status.TaggedErrorf(codes.NotFound, status.TagUnknownPath,
				"unable to find %s from the schema tree", xpath.ToXPath(fullpath))
		}
		sub, err := subses.addStreamSubscription(
			prefix, useAliases, gnmipb.SubscriptionList_STREAM,
			allowAggregation, encoding, []*gnmipb.Path{path}, submod,
			SampleInterval, supressRedundant, heartBeatInterval, stateSync)
		if err != nil {
			return err
		}
		startingList = append(startingList, sub)
	}

	subses.addDynamicSubscriptionInfo(startingList)
	for _, sub := range startingList {
		subses.StartTelmetryUpdate(sub)
	}
	return nil
}
