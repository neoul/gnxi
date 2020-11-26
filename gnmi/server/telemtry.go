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
type present struct {
	duplicates uint32
}
type pathSet map[string]*present

type telemUpdateEvent struct {
	updatedroot ygot.GoStruct
	updatedPath pathSet
	deletedPath pathSet
}

// gNMI Telemetry Control Block
type telemCtrl struct {
	// lookup map[TelemID]*TelemetrySubscription using a path
	lookup  *gtrie.Trie
	ready   map[TelemID]*TelemetrySubscription
	updated map[TelemID]pathSet // paths
	deleted map[TelemID]pathSet // paths
	mutex   *sync.Mutex
}

func newTeleCtrl() *telemCtrl {
	return &telemCtrl{
		lookup:  gtrie.New(),
		ready:   map[TelemID]*TelemetrySubscription{},
		updated: map[TelemID]pathSet{},
		deleted: map[TelemID]pathSet{},
		mutex:   &sync.Mutex{},
	}
}

func (tcb *telemCtrl) register(m *model.Model, telesub *TelemetrySubscription) error {
	tcb.mutex.Lock()
	defer tcb.mutex.Unlock()
	for _, path := range telesub.Paths {
		fullpath := xpath.GNMIFullPath(telesub.Prefix, path)
		allpaths, _ := m.FindAllPaths(fullpath)
		for _, p := range allpaths {
			if subgroup, ok := tcb.lookup.Find(p); ok {
				subgroup.(map[TelemID]*TelemetrySubscription)[telesub.ID] = telesub
			} else {
				tcb.lookup.Add(p, map[TelemID]*TelemetrySubscription{telesub.ID: telesub})
			}
			glog.Infof("telemctrl.telemetry[%d][%d].subscribe(%v)", telesub.SessionID, telesub.ID, p)
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
			glog.Infof("telemctrl.telemetry[%d][%d].unsubscribe(%v)", telesub.SessionID, telesub.ID, p)
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
	tcb.ready = map[TelemID]*TelemetrySubscription{}
	tcb.updated = map[TelemID]pathSet{}
	tcb.deleted = map[TelemID]pathSet{}
	glog.Infof("telemctrl.ChangeStarted")
}

func (tcb *telemCtrl) updateTelemetryCtrl(datapath, path string, op int) {
	update := func(subscriber map[TelemID]*TelemetrySubscription, list map[TelemID]pathSet) {
		for _, telesub := range subscriber {
			if telesub.IsPolling {
				continue
			}
			glog.Infof("telemetry[%d][%d].onchange(%c).matched.path(%s)",
				telesub.SessionID, telesub.ID, op, path)
			tcb.ready[telesub.ID] = telesub
			if list[telesub.ID] == nil {
				list[telesub.ID] = pathSet{datapath: &present{duplicates: 1}}
			} else {
				if p, ok := list[telesub.ID][datapath]; !ok {
					list[telesub.ID][datapath] = &present{duplicates: 1}
				} else {
					p.duplicates++
				}
			}
		}
	}
	var tlist map[TelemID]pathSet
	switch op {
	case 'C', 'R':
		tlist = tcb.updated
	case 'D':
		tlist = tcb.deleted
	default:
		return
	}

	for _, subgroup := range tcb.lookup.FindAll(path) {
		subscriber := subgroup.(map[TelemID]*TelemetrySubscription)
		update(subscriber, tlist)
	}
}

// ChangeNotification.ChangeCreated - callback for Telemetry subscription on data changes
func (tcb *telemCtrl) ChangeCreated(datapath string, changes ygot.GoStruct) {
	tcb.mutex.Lock()
	defer tcb.mutex.Unlock()
	glog.Infof("telemctrl.ChangeCreated.path(%s)", datapath)
	gpath, err := xpath.ToGNMIPath(datapath)
	if err != nil {
		return
	}
	tcb.updateTelemetryCtrl(datapath, datapath, 'C')
	if !xpath.IsSchemaPath(gpath) {
		tcb.updateTelemetryCtrl(datapath, xpath.PathElemToXPATH(gpath.GetElem(), true), 'C')
	}
}

// ChangeNotification.ChangeReplaced - callback for Telemetry subscription on data changes
func (tcb *telemCtrl) ChangeReplaced(datapath string, changes ygot.GoStruct) {
	tcb.mutex.Lock()
	defer tcb.mutex.Unlock()
	glog.Infof("telemctrl.ChangeReplaced.path(%s)", datapath)
	gpath, err := xpath.ToGNMIPath(datapath)
	if err != nil {
		return
	}
	tcb.updateTelemetryCtrl(datapath, datapath, 'R')
	if !xpath.IsSchemaPath(gpath) {
		tcb.updateTelemetryCtrl(datapath, xpath.PathElemToXPATH(gpath.GetElem(), true), 'R')
	}
}

// ChangeNotification.ChangeDeleted - callback for Telemetry subscription on data changes
func (tcb *telemCtrl) ChangeDeleted(datapath string) {
	tcb.mutex.Lock()
	defer tcb.mutex.Unlock()
	glog.Infof("telemctrl.ChangeDeleted.path(%s)", datapath)
	gpath, err := xpath.ToGNMIPath(datapath)
	if err != nil {
		return
	}
	tcb.updateTelemetryCtrl(datapath, datapath, 'D')
	if !xpath.IsSchemaPath(gpath) {
		tcb.updateTelemetryCtrl(datapath, xpath.PathElemToXPATH(gpath.GetElem(), true), 'D')
	}
}

// ChangeNotification.ChangeCompleted - callback for Telemetry subscription on data changes
func (tcb *telemCtrl) ChangeCompleted(changes ygot.GoStruct) {
	tcb.mutex.Lock()
	defer tcb.mutex.Unlock()
	for telesubid, telesub := range tcb.ready {
		if telesub.eventque != nil {
			telesub.eventque <- &telemUpdateEvent{
				updatedPath: tcb.updated[telesubid],
				deletedPath: tcb.deleted[telesubid],
				updatedroot: changes,
			}
		}
		delete(tcb.ready, telesubid)
		delete(tcb.updated, telesubid)
		delete(tcb.deleted, telesubid)
	}
	glog.Infof("telemctrl.ChangeCompleted")
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
	mutex     sync.RWMutex
	server    *Server
}

var (
	sessID TelemID
	subID  TelemID
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

func newTelemetrySession(ctx context.Context, s *Server) *TelemetrySession {
	var address string
	var port int
	sessID++
	_, remoteaddr, _ := utilities.QueryAddr(ctx)
	addr := remoteaddr.String()
	end := strings.LastIndex(addr, ":")
	if end < 0 {
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
		server:    s,
	}
}

func (teleses *TelemetrySession) stopTelemetrySession() {
	delDynamicTeleSub(teleses.server.Model, teleses)
	teleses.lock()
	defer teleses.unlock()
	for _, telesub := range teleses.Telesub {
		teleses.server.unregister(telesub)
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
	Duplicates        map[string]uint32            `json:"duplicates,omitempty"` // Number of coalesced duplicates.
	// internal data
	session    *TelemetrySession
	Configured struct {
		SubscriptionMode  gnmipb.SubscriptionMode
		SampleInterval    uint64
		SuppressRedundant bool
		HeartbeatInterval uint64
	}
	IsPolling bool

	eventque    chan *telemUpdateEvent
	createdList *gtrie.Trie
	updatedList *gtrie.Trie
	deletedList *gtrie.Trie
	started     bool
	stop        chan struct{}

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
	var samplingTimer, heartbeatTimer *time.Ticker
	shutdown := teleses.shutdown
	waitgroup := teleses.waitgroup
	defer func() {
		glog.Warningf("telemetry[%d][%d].quit", telesub.SessionID, telesub.ID)
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
		glog.Errorf("telemetry[%d][%d].timer-failed", telesub.SessionID, telesub.ID)
		return
	}
	for {
		select {
		case event, ok := <-telesub.eventque:
			if !ok {
				glog.Errorf("telemetry[%d][%d].event-queue-closed", telesub.SessionID, telesub.ID)
				return
			}
			glog.Infof("telemetry[%d][%d].event-received", telesub.SessionID, telesub.ID)
			switch telesub.Configured.SubscriptionMode {
			case gnmipb.SubscriptionMode_ON_CHANGE, gnmipb.SubscriptionMode_SAMPLE:
				glog.Info("replaced,deleted ", event.updatedPath, event.deletedPath)
				for p, d := range event.updatedPath {
					if f, ok := telesub.updatedList.Find(p); ok {
						ps := f.(*present)
						ps.duplicates++
					} else {
						telesub.updatedList.Add(p, d)
					}
				}
				for p, d := range event.deletedPath {
					if f, ok := telesub.deletedList.Find(p); ok {
						ps := f.(*present)
						ps.duplicates++
					} else {
						telesub.deletedList.Add(p, d)
					}
				}
				if telesub.Configured.SubscriptionMode == gnmipb.SubscriptionMode_ON_CHANGE {
					err := teleses.telemetryUpdate(telesub, event.updatedroot)
					if err != nil {
						glog.Errorf("telemetry[%d][%d].failed(%v)", telesub.SessionID, telesub.ID, err)
						return
					}
					telesub.updatedList = gtrie.New()
					telesub.deletedList = gtrie.New()
				}
			}
		case <-samplingTimer.C:
			glog.Infof("telemetry[%d][%d].sampling-timer-expired", telesub.SessionID, telesub.ID)
			// suppress_redundant - skips the telemetry update if no changes
			if !telesub.Configured.SuppressRedundant ||
				telesub.updatedList.Size() > 0 ||
				telesub.deletedList.Size() > 0 {
				err := teleses.telemetryUpdate(telesub, nil)
				if err != nil {
					glog.Errorf("telemetry[%d][%d].failed(%v)", telesub.SessionID, telesub.ID, err)
					return
				}
			}
			telesub.updatedList = gtrie.New()
			telesub.deletedList = gtrie.New()
		case <-heartbeatTimer.C:
			glog.Infof("telemetry[%d][%d].heartbeat-timer-expired", telesub.SessionID, telesub.ID)
			err := teleses.telemetryUpdate(telesub, nil)
			if err != nil {
				glog.Errorf("telemetry[%d][%d].failed(%v)", telesub.SessionID, telesub.ID, err)
				return
			}
			telesub.updatedList = gtrie.New()
			telesub.deletedList = gtrie.New()
		case <-shutdown:
			glog.Infof("telemetry[%d][%d].shutdown", teleses.ID, telesub.ID)
			return
		case <-telesub.stop:
			glog.Infof("telemetry[%d][%d].stopped", teleses.ID, telesub.ID)
			return
		}
	}
}

// StartTelmetryUpdate - returns a key for telemetry comparison
func (teleses *TelemetrySession) StartTelmetryUpdate(telesub *TelemetrySubscription) error {
	teleses.lock()
	defer teleses.unlock()
	teleses.server.register(teleses.server.Model, telesub)
	if !telesub.started {
		telesub.started = true
		teleses.waitgroup.Add(1)
		go telesub.run(teleses)
	}
	return nil
}

// StopTelemetryUpdate - returns a key for telemetry comparison
func (teleses *TelemetrySession) StopTelemetryUpdate(telesub *TelemetrySubscription) error {
	teleses.lock()
	defer teleses.unlock()
	teleses.server.unregister(telesub)
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
	deletesNum := telesub.updatedList.Size() + telesub.deletedList.Size()
	deletes := make([]*gnmipb.Path, 0, deletesNum)
	if !deleteOnly {
		rpaths := telesub.updatedList.PrefixSearch(*path)
		for _, rpath := range rpaths {
			datapath, err := xpath.ToGNMIPath(rpath)
			if err != nil {
				return nil, fmt.Errorf("path-conversion-error(%s)", rpath)
			}
			deletes = append(deletes, datapath)
		}
	}
	dpaths := telesub.deletedList.PrefixSearch(*path)
	for _, dpath := range dpaths {
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
		paths := telesub.updatedList.PrefixSearch(data.Path)
		for _, p := range paths {
			if n, ok := telesub.updatedList.Find(p); ok {
				duplicates += n.(*present).duplicates
			}
		}
	}
	return &gnmipb.Update{Path: datapath, Val: typedValue, Duplicates: duplicates}, nil
}

// initTelemetryUpdate - Process and generate responses for a init update.
func (teleses *TelemetrySession) initTelemetryUpdate(req *gnmipb.SubscribeRequest) error {
	s := teleses.server
	bundling := !s.disableBundling
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
	paths := make([]*gnmipb.Path, 0, len(subList))
	for _, updateEntry := range subList {
		paths = append(paths, updateEntry.Path)
	}
	s.Model.RequestStateSync(prefix, paths)

	s.Model.RLock()
	defer s.Model.RUnlock()
	if err := xpath.ValidateGNMIPath(prefix); err != nil {
		return status.Errorf(codes.Unimplemented, "invalid-path(%s)", err.Error())
	}
	toplist, ok := s.Model.Find(s.Model.GetRoot(), prefix)
	if !ok || len(toplist) <= 0 {
		if ok = s.Model.ValidatePathSchema(prefix); ok {
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
			datalist, ok := s.Model.Find(branch, path)
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
	telesub.session.rlock()
	defer telesub.session.runlock()
	s := teleses.server
	bundling := !s.disableBundling
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
		// 1. lookup the prefix in the server.alias for server.alias.
		// prefix = nil
		// alias = xxx
	}
	if telesub.Configured.SubscriptionMode != gnmipb.SubscriptionMode_ON_CHANGE {
		s.Model.RequestStateSync(prefix, telesub.Paths)
	}

	s.Model.RLock()
	defer s.Model.RUnlock()
	if updatedroot == nil {
		updatedroot = s.Model.GetRoot()
	}
	if err := xpath.ValidateGNMIPath(prefix); err != nil {
		return status.Errorf(codes.Unimplemented, "invalid-path(%s)", err.Error())
	}
	toplist, ok := s.Model.Find(updatedroot, prefix)
	if !ok || len(toplist) <= 0 {
		if ok = s.Model.ValidatePathSchema(prefix); ok {
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
			datalist, ok := s.Model.Find(branch, path)
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
	sampleInterval uint64, suppressRedundant bool, heartbeatInterval uint64,
) (*TelemetrySubscription, error) {
	fmt.Println(heartbeatInterval)

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
		Duplicates:        make(map[string]uint32),
		updatedList:       gtrie.New(),
		deletedList:       gtrie.New(),
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
	telesub.eventque = make(chan *telemUpdateEvent, 64)
	teleses.lock()
	defer teleses.unlock()
	if t, ok := teleses.Telesub[key]; ok {
		// only updates the new path
		t.Paths = append(t.Paths, telesub.Paths...)
		telesub = t
		glog.Infof("telemetry[%d][%s].add-path(%s)", teleses.ID, key, xpath.ToXPath(telesub.Paths[len(telesub.Paths)-1]))
		return nil, nil
	}
	subID++
	id := subID
	telesub.ID = id
	telesub.session = teleses
	teleses.Telesub[key] = telesub
	glog.Infof("telemetry[%d][%d].new(%s)", teleses.ID, telesub.ID, *telesub.key)
	glog.Infof("telemetry[%d][%d].add-path(%s)", teleses.ID, telesub.ID, xpath.ToXPath(telesub.Paths[0]))
	return telesub, nil
}

// addPollSubscription - Create new TelemetrySubscription
func (teleses *TelemetrySession) addPollSubscription() error {
	telesub := TelemetrySubscription{
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
	teleses.lock()
	defer teleses.unlock()
	teleses.Telesub[key] = &telesub
	glog.Infof("telemetry[%d][%d].new(%s)", teleses.ID, telesub.ID, *telesub.key)
	return nil
}

func (teleses *TelemetrySession) updateAliases(aliaslist []*gnmipb.Alias) error {
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

	if err := teleses.server.Model.CheckModels(useModules); err != nil {
		return status.Errorf(codes.Unimplemented, err.Error())
	}
	if err := teleses.server.checkEncoding(encoding); err != nil {
		return err
	}

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
	startingList := []*TelemetrySubscription{}
	for _, updateEntry := range subList {
		path := updateEntry.GetPath()
		submod := updateEntry.GetMode()
		SampleInterval := updateEntry.GetSampleInterval()
		supressRedundant := updateEntry.GetSuppressRedundant()
		heartBeatInterval := updateEntry.GetHeartbeatInterval()
		telesub, err := teleses.addStreamSubscription(
			prefix, useAliases, gnmipb.SubscriptionList_STREAM,
			allowAggregation, encoding, []*gnmipb.Path{path}, submod,
			SampleInterval, supressRedundant, heartBeatInterval)
		if err != nil {
			return err
		}
		if telesub != nil {
			startingList = append(startingList, telesub)
		}
	}

	addDynamicTeleSub(teleses.server.Model, startingList)
	for _, telesub := range startingList {
		err = teleses.StartTelmetryUpdate(telesub)
		if err != nil {
			return err
		}
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
