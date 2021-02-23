package server

import (
	"fmt"
	"sort"
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
	updatedPath []string
	deletedPath []string
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
				subscriber := subgroup.(map[TeleID]*Subscription)
				subscriber[sub.ID] = sub
				// gdump.Print(subgroup.(map[TeleID]*Subscription)[sub.ID])
			} else {
				tcb.lookup.Add(p, map[TeleID]*Subscription{sub.ID: sub})
			}

			if glog.V(11) {
				glog.Infof("teleCtrl.sub[%d][%d].registered(%v)", sub.SessionID, sub.ID, p)
			}
		}
	}
	return nil
}

func (tcb *teleCtrl) unregister(sub *Subscription) {
	tcb.mutex.Lock()
	defer tcb.mutex.Unlock()
	all := tcb.lookup.All("")
	for p, subgroup := range all {
		subscriber := subgroup.(map[TeleID]*Subscription)
		_, ok := subscriber[sub.ID]
		if ok {
			if glog.V(11) {
				glog.Infof("teleCtrl.sub[%d][%d].unregistered(%v)", sub.SessionID, sub.ID, p)
			}
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
	// glog.V(11).Infof("teleCtrl.ChangeStarted")
}

func (tcb *teleCtrl) updateTeleEvent(op int, datapath, searchpath string) {
	for _, subgroup := range tcb.lookup.FindAll(searchpath) {
		subscribers := subgroup.(map[TeleID]*Subscription)
		for _, sub := range subscribers {
			if glog.V(11) {
				glog.Infof("teleCtrl.%c.path(%s).sub[%d][%d]",
					op, datapath, sub.SessionID, sub.ID)
			}
			event, ok := tcb.ready[sub.ID]
			if !ok {
				event = &teleEvent{
					sub:         sub,
					updatedPath: make([]string, 0, 64),
					deletedPath: make([]string, 0, 64),
				}
				tcb.ready[sub.ID] = event
			}
			switch op {
			case 'C':
				event.updatedPath = append(event.updatedPath, datapath)
			case 'R':
				event.updatedPath = append(event.updatedPath, datapath)
				event.deletedPath = append(event.deletedPath, datapath)
			case 'D':
				event.deletedPath = append(event.deletedPath, datapath)
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
	if glog.V(11) {
		glog.Infof("teleCtrl.ChangeCreated.path(%s)", datapath)
	}
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
	if glog.V(11) {
		glog.Infof("teleCtrl.ChangeReplaced.path(%s)", datapath)
	}
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
	if glog.V(11) {
		glog.Infof("teleCtrl.ChangeDeleted.path(%s)", datapath)
	}
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
			if glog.V(11) {
				glog.Infof("teleCtrl.send.event.to.sub[%d][%d]", sub.SessionID, sub.ID)
			}
		}
		delete(tcb.ready, telesubid)
	}
	// glog.V(11).Infof("teleCtrl.ChangeCompleted")
}

// PollSubscription - for Poll mode subscription
type PollSubscription struct {
	ID        TeleID
	SessionID TeleID
	SubList   *gnmipb.SubscriptionList
	session   *SubSession
}

// SubSession - gNMI gRPC Subscribe RPC (Telemetry) session information managed by server
type SubSession struct {
	ID        TeleID
	Address   string
	Port      uint16
	StreamSub map[string]*Subscription
	PollSub   []*PollSubscription
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

func startSubSession(ctx context.Context, s *Server) *SubSession {
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
	if glog.V(11) {
		glog.Infof("subses[%d].started", sessID)
	}
	return &SubSession{
		ID:            sessID,
		Address:       address,
		Port:          uint16(port),
		StreamSub:     map[string]*Subscription{},
		respchan:      make(chan *gnmipb.SubscribeResponse, 256),
		shutdown:      make(chan struct{}),
		waitgroup:     new(sync.WaitGroup),
		clientAliases: newClientAliases(),
		Server:        s,
	}
}

func (subses *SubSession) stopSubSession() {
	for i := range subses.PollSub {
		subses.deletePollDynamicSubscriptionInfo(subses.PollSub[i])
		subses.PollSub[i] = nil
	}
	for key, sub := range subses.StreamSub {
		subses.deleteStreamDynamicSubscriptionInfo(sub)
		subses.unregister(sub)
		delete(subses.StreamSub, key)
	}
	close(subses.respchan)
	close(subses.shutdown)
	subses.waitgroup.Wait()
	if glog.V(11) {
		glog.Infof("subses[%d].stopped", subses.ID)
	}
}

// Subscription - Default structure for Telemetry Update Subscription
type Subscription struct {
	ID                TeleID
	SessionID         TeleID
	key               string
	Prefix            *gnmipb.Path                 `json:"prefix,omitempty"`
	UseAliases        bool                         `json:"use_aliases,omitempty"`
	Mode              gnmipb.SubscriptionList_Mode `json:"stream_mode,omitempty"`
	AllowAggregation  bool                         `json:"allow_aggregation,omitempty"`
	Encoding          gnmipb.Encoding              `json:"encoding,omitempty"`
	Paths             []*gnmipb.Path               `json:"path,omitempty"`              // The data tree path.
	StreamMode        gnmipb.SubscriptionMode      `json:"subscription_mode,omitempty"` // Subscription mode to be used.
	SampleInterval    uint64                       `json:"sample_interval,omitempty"`   // ns between samples in SAMPLE mode.
	SuppressRedundant bool                         `json:"suppress_redundant,omitempty"`
	HeartbeatInterval uint64                       `json:"heartbeat_interval,omitempty"`

	Configured struct {
		StreamMode        gnmipb.SubscriptionMode
		SampleInterval    uint64
		SuppressRedundant bool
		HeartbeatInterval uint64
	}

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
	timerExpired := make(chan bool, 2)
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
	defer func() {
		if glog.V(11) {
			glog.Warningf("sub[%d][%d].quit", sub.SessionID, sub.ID)
		}
		sub.started = false
		samplingTimer.Stop()
		heartbeatTimer.Stop()
		subses.waitgroup.Done()
		close(timerExpired)
	}()
	if samplingTimer == nil || heartbeatTimer == nil {
		if glog.V(11) {
			glog.Errorf("sub[%d][%d].timer-failed", sub.SessionID, sub.ID)
		}
		return
	}

	for {
		select {
		case event, ok := <-sub.eventque:
			if !ok {
				if glog.V(11) {
					glog.Errorf("sub[%d][%d].event-queue-closed", sub.SessionID, sub.ID)
				}
				return
			}
			if glog.V(11) {
				glog.Infof("sub[%d][%d].event-received", sub.SessionID, sub.ID)
			}
			switch sub.Configured.StreamMode {
			case gnmipb.SubscriptionMode_ON_CHANGE, gnmipb.SubscriptionMode_SAMPLE:
				for _, p := range event.updatedPath {
					duplicates := uint32(1)
					if v, ok := sub.updatedList.Find(p); ok {
						duplicates = v.(uint32)
						duplicates++
					}
					sub.updatedList.Add(p, duplicates)
				}
				for _, p := range event.deletedPath {
					duplicates := uint32(1)
					if v, ok := sub.deletedList.Find(p); ok {
						duplicates = v.(uint32)
						duplicates++
					}
					sub.deletedList.Add(p, duplicates)
				}
				if sub.Configured.StreamMode == gnmipb.SubscriptionMode_ON_CHANGE {
					err := subses.telemetryUpdate(sub, event.updatedroot)
					if err != nil {
						if glog.V(11) {
							glog.Errorf("sub[%d][%d].failed(%v)", sub.SessionID, sub.ID, err)
						}
						return
					}
					sub.updatedList = gtrie.New()
					sub.deletedList = gtrie.New()
				}
			}
		case <-samplingTimer.C:
			if glog.V(11) {
				glog.Infof("sub[%d][%d].sampling-timer-expired", sub.SessionID, sub.ID)
			}
			if sub.stateSync {
				sub.mutex.Lock()
				sub.session.RequestStateSync(sub.Prefix, sub.Paths)
				sub.mutex.Unlock()
			}
			timerExpired <- false
		case <-heartbeatTimer.C:
			if glog.V(11) {
				glog.Infof("sub[%d][%d].heartbeat-timer-expired", sub.SessionID, sub.ID)
			}
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
				if glog.V(11) {
					glog.Infof("sub[%d][%d].send.telemetry-update.to.%v",
						sub.SessionID, sub.ID, sub.session)
				}
				sub.mutex.Lock() // block to modify sub.Path
				err := subses.telemetryUpdate(sub, nil)
				sub.mutex.Unlock()
				if err != nil {
					if glog.V(11) {
						glog.Errorf("sub[%d][%d].failed(%v)", sub.SessionID, sub.ID, err)
					}
					return
				}
				sub.updatedList = gtrie.New()
				sub.deletedList = gtrie.New()
			}
		case <-subses.shutdown:
			if glog.V(11) {
				glog.Infof("sub[%d][%d].shutdown", subses.ID, sub.ID)
			}
			return
		case <-sub.stop:
			if glog.V(11) {
				glog.Infof("sub[%d][%d].stopped", subses.ID, sub.ID)
			}
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
				"path.converting error for %s", dpath)
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
			"path.converting error for %s", data.Path)
	}
	var duplicates uint32
	if sub != nil {
		for _, v := range sub.updatedList.All(data.Path) {
			duplicates += v.(uint32)
		}
	}
	return &gnmipb.Update{Path: datapath, Val: typedValue, Duplicates: duplicates}, nil
}

func (subses *SubSession) clientAliasesUpdate(aliaslist *gnmipb.AliasList) error {
	aliasnames, err := subses.updateClientAliases(aliaslist.GetAlias())
	for _, name := range aliasnames {
		subses.sendTelemetryUpdate(
			buildAliasResponse(subses.ToPath(name, true).(*gnmipb.Path), name))
	}
	return err
}

func (subses *SubSession) serverAliasesUpdate() {
	aliases := subses.updateServerAliases(subses.serverAliases, true)
	sort.Slice(aliases, func(i, j int) bool {
		return aliases[i] < aliases[j]
	})
	for _, alias := range aliases {
		subses.sendTelemetryUpdate(
			buildAliasResponse(subses.ToPath(alias, true).(*gnmipb.Path), alias))
	}
}

// initTelemetryUpdate - Process and generate responses for a init update.
func (subses *SubSession) initTelemetryUpdate(subscriptionList *gnmipb.SubscriptionList) error {
	subList := subscriptionList.GetSubscription()
	updatesOnly := subscriptionList.GetUpdatesOnly()
	if updatesOnly {
		return subses.sendTelemetryUpdate(buildSyncResponse())
	}
	prefix := subscriptionList.GetPrefix()
	prefix = subses.ToPath(prefix, false).(*gnmipb.Path)
	prefixAlias := subses.ToAlias(prefix, false).(*gnmipb.Path)

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
	if err := subses.ValidateGNMIPath(prefix); err != nil {
		return err
	}
	toplist, ok := subses.Find(subses.GetRoot(), prefix)
	if !ok || len(toplist) <= 0 {
		return subses.sendTelemetryUpdate(buildSyncResponse())
	}

	for _, top := range toplist {
		branch := top.Value.(ygot.GoStruct)
		updates := make([]*gnmipb.Update, 0, 16)
		for _, updateEntry := range subList {
			path := updateEntry.Path
			if err := subses.ValidateGNMIPath(prefix, path); err != nil {
				return err
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
					updates = append(updates, u)
				}
			}
		}
		if len(updates) > 0 {
			err := subses.sendTelemetryUpdate(
				buildSubscribeResponse(prefixAlias, updates, nil))
			if err != nil {
				return err
			}
		}
	}
	return subses.sendTelemetryUpdate(buildSyncResponse())
}

// telemetryUpdate - Process and generate responses for a telemetry update.
func (subses *SubSession) telemetryUpdate(sub *Subscription, updatedroot ygot.GoStruct) error {
	prefix := sub.Prefix
	prefixAlias := subses.ToAlias(prefix, false).(*gnmipb.Path)
	encoding := sub.Encoding
	mode := sub.Mode

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
	if err := subses.ValidateGNMIPath(prefix); err != nil {
		return err
	}
	toplist, ok := subses.Find(updatedroot, prefix)
	if !ok || len(toplist) <= 0 {
		// data-missing is not an error in SubscribeRPC
		// does not send any of messages.
		return nil
	}

	for _, top := range toplist {
		var err error
		var deletes []*gnmipb.Path
		var updates []*gnmipb.Update
		bpath := top.Path
		branch := top.Value.(ygot.GoStruct)
		// get all replaced, deleted paths relative to the prefix
		deletes, err = getDeletes(sub, &bpath, false)
		if err != nil {
			return err
		}
		updates = make([]*gnmipb.Update, 0, 16)
		for _, path := range sub.Paths {
			if err := subses.ValidateGNMIPath(prefix, path); err != nil {
				return err
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
					updates = append(updates, u)
				}
			}
		}
		err = subses.sendTelemetryUpdate(
			buildSubscribeResponse(prefixAlias, updates, deletes))
		if err != nil {
			return err
		}
	}
	return nil
}

const (
	defaultInterval = 60000000000
	minimumInterval = 1000000000
)

func (subses *SubSession) addStreamSubscription(
	prefix *gnmipb.Path, useAliases bool, Mode gnmipb.SubscriptionList_Mode, allowAggregation bool,
	Encoding gnmipb.Encoding, Path *gnmipb.Path, StreamMode gnmipb.SubscriptionMode,
	SampleInterval uint64, SuppressRedundant bool, HeartbeatInterval uint64, stateSync bool,
) (*Subscription, error) {
	key := fmt.Sprintf("%d-%s-%s-%s-%s-%d-%d-%t-%t-%t",
		subses.ID, Mode, Encoding, StreamMode,
		xpath.ToXPath(prefix), SampleInterval, HeartbeatInterval,
		useAliases, allowAggregation, SuppressRedundant,
	)

	if subctrl, ok := subses.StreamSub[key]; ok {
		// only updates the new path if the sub exists.
		subctrl.mutex.Lock()
		defer subctrl.mutex.Unlock()
		subctrl.Paths = append(subctrl.Paths, Path)
		if !subctrl.stateSync {
			subctrl.stateSync = stateSync
		}
		if glog.V(11) {
			glog.Infof("telemetry[%d][%d].add.path(%v)", subses.ID, subctrl.ID, Path)
		}
		return subctrl, nil
	}
	subID++
	subctrl := &Subscription{
		ID:                subID,
		SessionID:         subses.ID,
		Prefix:            prefix,
		Paths:             []*gnmipb.Path{Path},
		UseAliases:        useAliases,
		Mode:              Mode,
		AllowAggregation:  allowAggregation,
		Encoding:          Encoding,
		StreamMode:        StreamMode,
		SampleInterval:    SampleInterval,
		SuppressRedundant: SuppressRedundant,
		HeartbeatInterval: HeartbeatInterval,
		updatedList:       gtrie.New(),
		deletedList:       gtrie.New(),
		stateSync:         stateSync,
		eventque:          make(chan *teleEvent, 64),
		mutex:             &sync.Mutex{},
		session:           subses,
		key:               key,
	}
	subses.StreamSub[key] = subctrl
	if glog.V(11) {
		glog.Infof("telemetry[%d][%d].new(%s)", subses.ID, subctrl.ID, subctrl.key)
		glog.Infof("telemetry[%d][%d].add.path(%v)", subses.ID, subctrl.ID, Path)
	}

	// 3.5.1.5.2 STREAM Subscriptions Must be satisfied for telemetry update starting.
	switch subctrl.StreamMode {
	case gnmipb.SubscriptionMode_TARGET_DEFINED:
		// vendor specific mode
		subctrl.Configured.StreamMode = gnmipb.SubscriptionMode_SAMPLE
		subctrl.Configured.SampleInterval = defaultInterval
		subctrl.Configured.SuppressRedundant = true
		subctrl.Configured.HeartbeatInterval = 0
	case gnmipb.SubscriptionMode_ON_CHANGE:
		if subctrl.HeartbeatInterval < minimumInterval && subctrl.HeartbeatInterval != 0 {
			return nil, status.TaggedErrorf(codes.OutOfRange, status.TagInvalidConfig,
				"heartbeat_interval(!= 0sec and < 1sec) is not supported")
		}
		if subctrl.SampleInterval != 0 {
			return nil, status.TaggedErrorf(codes.InvalidArgument, status.TagInvalidConfig,
				"sample_interval not allowed on on_change mode")
		}
		subctrl.Configured.StreamMode = gnmipb.SubscriptionMode_ON_CHANGE
		subctrl.Configured.SampleInterval = 0
		subctrl.Configured.SuppressRedundant = false
		subctrl.Configured.HeartbeatInterval = subctrl.HeartbeatInterval
	case gnmipb.SubscriptionMode_SAMPLE:
		if subctrl.SampleInterval < minimumInterval && subctrl.SampleInterval != 0 {
			return nil, status.TaggedErrorf(codes.OutOfRange, status.TagInvalidConfig,
				"sample_interval(!= 0sec and < 1sec) is not supported")
		}
		if subctrl.HeartbeatInterval != 0 {
			if subctrl.HeartbeatInterval < minimumInterval {
				return nil, status.TaggedErrorf(codes.OutOfRange, status.TagInvalidConfig,
					"heartbeat_interval(!= 0sec and < 1sec) is not supported")
			}
			if subctrl.SampleInterval > subctrl.HeartbeatInterval {
				return nil, status.TaggedErrorf(codes.OutOfRange, status.TagInvalidConfig,
					"heartbeat_interval should be larger than sample_interval")
			}
		}

		subctrl.Configured.StreamMode = gnmipb.SubscriptionMode_SAMPLE
		subctrl.Configured.SampleInterval = subctrl.SampleInterval
		if subctrl.SampleInterval == 0 {
			// Set minimal sampling interval (1sec)
			subctrl.Configured.SampleInterval = minimumInterval
		}
		subctrl.Configured.SuppressRedundant = subctrl.SuppressRedundant
		subctrl.Configured.HeartbeatInterval = subctrl.HeartbeatInterval
	}
	return subctrl, nil
}

// addPollSubscription - Create new Subscription
func (subses *SubSession) addPollSubscription(sublist *gnmipb.SubscriptionList) (*PollSubscription, error) {
	subID++
	psub := &PollSubscription{
		ID:        subID,
		SessionID: subses.ID,
		SubList:   sublist,
		session:   subses,
	}
	subses.PollSub = append(subses.PollSub, psub)
	if glog.V(11) {
		glog.Infof("telemetry[%d][%d].new(poll)", subses.ID, psub.ID)
	}
	return psub, nil
}

func (subses *SubSession) processSubscribeRequest(req *gnmipb.SubscribeRequest) error {
	// SubscribeRequest for poll Subscription indication
	pollMode := req.GetPoll()
	if pollMode != nil {
		for _, psub := range subses.PollSub {
			sublist := psub.SubList
			subses.RequestStateSyncBySubscriptionList(sublist)
			if err := subses.initTelemetryUpdate(sublist); err != nil {
				if glog.V(11) {
					glog.Errorf("telemetry[%d][%d].new(poll)", subses.ID, psub.ID)
				}
			}
		}
		return nil
	}
	// SubscribeRequest for aliases update
	aliases := req.GetAliases()
	if aliases != nil {
		return subses.clientAliasesUpdate(aliases)
	}

	// extension := req.GetExtension()
	subscriptionList := req.GetSubscribe()
	if subscriptionList == nil {
		return status.TaggedErrorf(codes.InvalidArgument, status.TagMalformedMessage,
			"no subscription info (SubscriptionList field)")
	}
	// check & update the server aliases if use_aliases is true
	useAliases := subscriptionList.GetUseAliases()
	if useAliases {
		subses.serverAliasesUpdate()
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
	mode := subscriptionList.GetMode()
	subscriptionList.Prefix = subses.ToPath(subscriptionList.GetPrefix(), false).(*gnmipb.Path)
	prefix := subscriptionList.GetPrefix()

	var stateSync bool
	switch mode {
	case gnmipb.SubscriptionList_ONCE:
		stateSync = subses.RequestStateSyncBySubscriptionList(subscriptionList)
		return subses.initTelemetryUpdate(subscriptionList)
	case gnmipb.SubscriptionList_POLL:
		psub, err := subses.addPollSubscription(subscriptionList)
		if err != nil {
			return err
		}
		return subses.addPollDynamicSubscription(psub)
	case gnmipb.SubscriptionList_STREAM:
		stateSync = subses.RequestStateSyncBySubscriptionList(subscriptionList)
		if err := subses.initTelemetryUpdate(subscriptionList); err != nil {
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
			if err := subses.ValidateGNMIPath(prefix, path); err != nil {
				return err
			}
			sub, err := subses.addStreamSubscription(
				prefix, useAliases, mode, allowAggregation,
				encoding, path, submod, SampleInterval,
				supressRedundant, heartBeatInterval, stateSync)
			if err != nil {
				return err
			}
			startingList = append(startingList, sub)
		}
		subses.addStreamDynamicSubscription(startingList)
		for _, sub := range startingList {
			subses.StartTelmetryUpdate(sub)
		}
	}
	return nil
}
