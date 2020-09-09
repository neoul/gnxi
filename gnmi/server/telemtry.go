package server

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/neoul/gnxi/gnmi/model"
	"github.com/neoul/gnxi/utilities"
	"github.com/neoul/gnxi/utilities/xpath"
	"github.com/neoul/libydb/go/ydb"
	"github.com/neoul/trie"
	pb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ygot/ygot"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TelemetryID - Telemetry Session ID
type TelemetryID uint64
type present struct {
	duplicates uint32
}
type pathSet map[string]*present

type telemetryUpdateEvent struct {
	updatedroot  ygot.GoStruct
	createdList  pathSet
	replacedList pathSet
	deletedList  pathSet
}

type telemetryCtrl struct {
	// map[uint]*TelemetrySubscription: uint = is subscription.ID
	lookupTeleSub map[string]map[TelemetryID]*TelemetrySubscription
	readyToUpdate map[TelemetryID]*TelemetrySubscription
	createdList   map[TelemetryID]pathSet // paths
	replacedList  map[TelemetryID]pathSet // paths
	deletedList   map[TelemetryID]pathSet // paths
	mutex         sync.RWMutex
}

func newTelemetryCB() *telemetryCtrl {
	return &telemetryCtrl{
		lookupTeleSub: map[string]map[TelemetryID]*TelemetrySubscription{},
		readyToUpdate: map[TelemetryID]*TelemetrySubscription{},
		createdList:   map[TelemetryID]pathSet{},
		replacedList:  map[TelemetryID]pathSet{},
		deletedList:   map[TelemetryID]pathSet{},
	}
}

func (tcb *telemetryCtrl) registerTelemetry(m *model.Model, telesub *TelemetrySubscription) error {
	tcb.mutex.Lock()
	defer tcb.mutex.Unlock()
	for _, path := range telesub.Paths {
		fullpath := utilities.GNMIFullPath(telesub.Prefix, path)
		allpaths, ok := m.FindAllPaths(fullpath)
		if ok {
			for _, p := range allpaths {
				subgroup, ok := tcb.lookupTeleSub[p]
				if !ok || subgroup == nil {
					tcb.lookupTeleSub[p] = map[TelemetryID]*TelemetrySubscription{}
					subgroup = tcb.lookupTeleSub[p]
				}
				subgroup[telesub.ID] = telesub
			}
		}
	}
	fmt.Println(tcb)
	return nil
}

func (tcb *telemetryCtrl) unregisterTelemetry(telesub *TelemetrySubscription) {
	tcb.mutex.Lock()
	defer tcb.mutex.Unlock()
	for _, subgroup := range tcb.lookupTeleSub {
		_, ok := subgroup[telesub.ID]
		if ok {
			delete(subgroup, telesub.ID)
		}
	}
}

// OnChangeStarted - callback for Telemetry subscription on data changes
func (tcb *telemetryCtrl) OnChangeStarted(changes ygot.GoStruct) {
	tcb.mutex.RLock()
	defer tcb.mutex.RUnlock()
	tcb.readyToUpdate = map[TelemetryID]*TelemetrySubscription{}
	tcb.createdList = map[TelemetryID]pathSet{}
	tcb.replacedList = map[TelemetryID]pathSet{}
	tcb.deletedList = map[TelemetryID]pathSet{}
}

// OnChangeCreated - callback for Telemetry subscription on data changes
func (tcb *telemetryCtrl) OnChangeCreated(slicepath []string, changes ygot.GoStruct) {
	tcb.mutex.RLock()
	defer tcb.mutex.RUnlock()
	datapath := "/" + strings.Join(slicepath, "/")
	glog.Infof("telectrl.onchange(created).path(%s)", datapath)
	for i := len(slicepath); i >= 0; i-- {
		path := "/" + strings.Join(slicepath[:i], "/")
		subgroup, ok := tcb.lookupTeleSub[path]
		if ok {
			for _, telesub := range subgroup {
				if telesub.IsPolling {
					continue
				}
				glog.Infof("telemetry[%d][%d].onchange(created).matched.path(%s)",
					telesub.SessionID, telesub.ID, path)
				tcb.readyToUpdate[telesub.ID] = telesub
				if tcb.createdList[telesub.ID] == nil {
					tcb.createdList[telesub.ID] = pathSet{datapath: &present{duplicates: 1}}
				} else {
					if p, ok := tcb.createdList[telesub.ID][datapath]; !ok {
						tcb.createdList[telesub.ID][datapath] = &present{duplicates: 1}
					} else {
						p.duplicates++
					}
				}
			}
		}
	}
	sliceschema, isEqual := xpath.ToSchemaSlicePath(slicepath)
	if !isEqual {
		for i := len(sliceschema); i >= 0; i-- {
			path := "/" + strings.Join(sliceschema[:i], "/")
			subgroup, ok := tcb.lookupTeleSub[path]
			if ok {
				for _, telesub := range subgroup {
					if telesub.IsPolling {
						continue
					}
					glog.Infof("telemetry[%d][%d].onchange(created).matched.path(%s)",
						telesub.SessionID, telesub.ID, path)
					tcb.readyToUpdate[telesub.ID] = telesub
					if tcb.createdList[telesub.ID] == nil {
						tcb.createdList[telesub.ID] = pathSet{datapath: &present{duplicates: 1}}
					} else {
						if p, ok := tcb.createdList[telesub.ID][datapath]; !ok {
							tcb.createdList[telesub.ID][datapath] = &present{duplicates: 1}
						} else {
							p.duplicates++
						}
					}
				}
			}
		}
	}
}

// OnChangeReplaced - callback for Telemetry subscription on data changes
func (tcb *telemetryCtrl) OnChangeReplaced(slicepath []string, changes ygot.GoStruct) {
	tcb.mutex.RLock()
	defer tcb.mutex.RUnlock()
	datapath := "/" + strings.Join(slicepath, "/")
	glog.Infof("telectrl.onchange(replaced).path(%s)", datapath)
	for i := len(slicepath); i >= 0; i-- {
		path := "/" + strings.Join(slicepath[:i], "/")
		subgroup, ok := tcb.lookupTeleSub[path]
		if ok {
			for _, telesub := range subgroup {
				if telesub.IsPolling {
					continue
				}
				glog.Infof("telemetry[%d][%d].onchange(replaced).matched.path(%s)",
					telesub.SessionID, telesub.ID, path)
				tcb.readyToUpdate[telesub.ID] = telesub
				if tcb.replacedList[telesub.ID] == nil {
					tcb.replacedList[telesub.ID] = pathSet{datapath: &present{duplicates: 1}}
				} else {
					if p, ok := tcb.replacedList[telesub.ID][datapath]; !ok {
						tcb.replacedList[telesub.ID][datapath] = &present{duplicates: 1}
					} else {
						p.duplicates++
						fmt.Println("p.duplicates++", p.duplicates)
					}
				}
			}
		}
	}
	sliceschema, isEqual := xpath.ToSchemaSlicePath(slicepath)
	if !isEqual {
		for i := len(sliceschema); i >= 0; i-- {
			path := "/" + strings.Join(sliceschema[:i], "/")
			subgroup, ok := tcb.lookupTeleSub[path]
			if ok {
				for _, telesub := range subgroup {
					if telesub.IsPolling {
						continue
					}
					glog.Infof("telemetry[%d][%d].onchange(replaced).matched.path(%s)",
						telesub.SessionID, telesub.ID, path)
					tcb.readyToUpdate[telesub.ID] = telesub
					if tcb.replacedList[telesub.ID] == nil {
						tcb.replacedList[telesub.ID] = pathSet{datapath: &present{duplicates: 1}}
					} else {
						if p, ok := tcb.replacedList[telesub.ID][datapath]; !ok {
							tcb.replacedList[telesub.ID][datapath] = &present{duplicates: 1}
						} else {
							p.duplicates++
						}
					}
				}
			}
		}
	}
}

// OnChangeDeleted - callback for Telemetry subscription on data changes
func (tcb *telemetryCtrl) OnChangeDeleted(slicepath []string) {
	tcb.mutex.RLock()
	defer tcb.mutex.RUnlock()
	datapath := "/" + strings.Join(slicepath, "/")
	glog.Infof("telectrl.onchange(deleted).path(%s)", datapath)
	for i := len(slicepath); i >= 0; i-- {
		path := "/" + strings.Join(slicepath[:i], "/")
		subgroup, ok := tcb.lookupTeleSub[path]
		if ok {
			for _, telesub := range subgroup {
				if telesub.IsPolling {
					continue
				}
				glog.Infof("telemetry[%d][%d].onchange(deleted).matched.path(%s)",
					telesub.SessionID, telesub.ID, path)
				tcb.readyToUpdate[telesub.ID] = telesub
				if tcb.deletedList[telesub.ID] == nil {
					tcb.deletedList[telesub.ID] = pathSet{datapath: &present{duplicates: 1}}
				} else {
					if p, ok := tcb.deletedList[telesub.ID][datapath]; !ok {
						tcb.deletedList[telesub.ID][datapath] = &present{duplicates: 1}
					} else {
						p.duplicates++
					}
				}
			}
		}
	}
	sliceschema, isEqual := xpath.ToSchemaSlicePath(slicepath)
	if !isEqual {
		for i := len(sliceschema); i >= 0; i-- {
			path := "/" + strings.Join(sliceschema[:i], "/")
			subgroup, ok := tcb.lookupTeleSub[path]
			if ok {
				for _, telesub := range subgroup {
					if telesub.IsPolling {
						continue
					}
					glog.Infof("telemetry[%d][%d].onchange(deleted).matched.path(%s)",
						telesub.SessionID, telesub.ID, path)
					tcb.readyToUpdate[telesub.ID] = telesub
					if tcb.deletedList[telesub.ID] == nil {
						tcb.deletedList[telesub.ID] = pathSet{datapath: &present{duplicates: 1}}
					} else {
						if p, ok := tcb.deletedList[telesub.ID][datapath]; !ok {
							tcb.deletedList[telesub.ID][datapath] = &present{duplicates: 1}
						} else {
							p.duplicates++
						}
					}
				}
			}
		}
	}
}

// OnStarted - callback for Telemetry subscription on data changes
func (tcb *telemetryCtrl) OnChangeFinished(changes ygot.GoStruct) {
	tcb.mutex.RLock()
	defer tcb.mutex.RUnlock()
	for telesubid, telesub := range tcb.readyToUpdate {
		if telesub.eventque != nil {
			telesub.eventque <- &telemetryUpdateEvent{
				createdList:  tcb.createdList[telesubid],
				replacedList: tcb.replacedList[telesubid],
				deletedList:  tcb.deletedList[telesubid],
				updatedroot:  changes,
			}
		}
		delete(tcb.readyToUpdate, telesubid)
		delete(tcb.createdList, telesubid)
		delete(tcb.replacedList, telesubid)
		delete(tcb.deletedList, telesubid)
	}
}

// TelemetrySession - gNMI gRPC Subscribe RPC (Telemetry) session information managed by server
type TelemetrySession struct {
	ID        TelemetryID
	Telesub   map[string]*TelemetrySubscription
	respchan  chan *pb.SubscribeResponse
	shutdown  chan struct{}
	waitgroup *sync.WaitGroup
	alias     map[string]*pb.Alias
	mutex     sync.RWMutex
	server    *Server
}

var (
	sessID TelemetryID
	subID  TelemetryID
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
	sessID++
	return &TelemetrySession{
		ID:        sessID,
		Telesub:   map[string]*TelemetrySubscription{},
		respchan:  make(chan *pb.SubscribeResponse, 256),
		shutdown:  make(chan struct{}),
		waitgroup: new(sync.WaitGroup),
		alias:     map[string]*pb.Alias{},
		server:    s,
	}
}

func (teleses *TelemetrySession) stopTelemetrySession() {
	block := teleses.server.ModelData.GetYDB()
	delDynamicTeleSubscriptionInfo(block, teleses)
	teleses.lock()
	defer teleses.unlock()
	for _, telesub := range teleses.Telesub {
		teleses.server.unregisterTelemetry(telesub)
	}
	close(teleses.shutdown)
	teleses.waitgroup.Wait()
}

// TelemetrySubscription - Default structure for Telemetry Update Subscription
type TelemetrySubscription struct {
	ID                TelemetryID
	SessionID         TelemetryID
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
	Duplicates        map[string]uint32        `json:"duplicates,omitempty"` // Number of coalesced duplicates.
	// internal data
	session    *TelemetrySession
	Configured struct {
		SubscriptionMode  pb.SubscriptionMode
		SampleInterval    uint64
		SuppressRedundant bool
		HeartbeatInterval uint64
	}
	IsPolling bool

	eventque     chan *telemetryUpdateEvent
	createdList  *trie.Trie
	replacedList *trie.Trie
	deletedList  *trie.Trie
	started      bool
	stop         chan struct{}

	// // https://github.com/openconfig/gnmi/issues/45 - QoSMarking seems to be deprecated
	// Qos              *pb.QOSMarking           `json:"qos,omitempty"`          // DSCP marking to be used.
	// UseModels        []*pb.ModelData          `json:"use_models,omitempty"`   // (Check validate only in Request)
	// Alias            []*pb.Alias              `json:"alias,omitempty"`
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
			case pb.SubscriptionMode_ON_CHANGE, pb.SubscriptionMode_SAMPLE:
				// glog.Info("replaced,deleted ", event.replacedList, event.deletedList)
				for p, d := range event.createdList {
					if f, ok := telesub.createdList.Find(p); ok {
						ps := f.Meta().(*present)
						ps.duplicates++
					} else {
						telesub.createdList.Add(p, d)
					}
				}
				for p, d := range event.replacedList {
					if f, ok := telesub.replacedList.Find(p); ok {
						ps := f.Meta().(*present)
						ps.duplicates++
					} else {
						telesub.replacedList.Add(p, d)
					}
				}
				for p, d := range event.deletedList {
					if f, ok := telesub.deletedList.Find(p); ok {
						ps := f.Meta().(*present)
						ps.duplicates++
					} else {
						telesub.deletedList.Add(p, d)
					}
				}
				if telesub.Configured.SubscriptionMode == pb.SubscriptionMode_ON_CHANGE {
					// utilities.PrintStruct(event.updatedroot)
					err := teleses.telemetryUpdate(telesub, event.updatedroot)
					if err != nil {
						glog.Errorf("telemetry[%d][%d].failed(%v)", telesub.SessionID, telesub.ID, err)
						return
					}
					telesub.createdList = trie.New()
					telesub.replacedList = trie.New()
					telesub.deletedList = trie.New()
				}
			}
		case <-samplingTimer.C:
			glog.Infof("telemetry[%d][%d].sampling-timer-expired", telesub.SessionID, telesub.ID)
			// suppress_redundant - skips the telemetry update if no changes
			if !telesub.Configured.SuppressRedundant ||
				telesub.replacedList.Size() > 0 ||
				telesub.deletedList.Size() > 0 {
				err := teleses.telemetryUpdate(telesub, nil)
				if err != nil {
					glog.Errorf("telemetry[%d][%d].failed(%v)", telesub.SessionID, telesub.ID, err)
					return
				}
			}
			telesub.createdList = trie.New()
			telesub.replacedList = trie.New()
			telesub.deletedList = trie.New()
		case <-heartbeatTimer.C:
			glog.Infof("telemetry[%d][%d].heartbeat-timer-expired", telesub.SessionID, telesub.ID)
			err := teleses.telemetryUpdate(telesub, nil)
			if err != nil {
				glog.Errorf("telemetry[%d][%d].failed(%v)", telesub.SessionID, telesub.ID, err)
				return
			}
			telesub.createdList = trie.New()
			telesub.replacedList = trie.New()
			telesub.deletedList = trie.New()
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
	teleses.server.registerTelemetry(teleses.server.Model, telesub)
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
	teleses.server.unregisterTelemetry(telesub)
	close(telesub.stop)
	if telesub.eventque != nil {
		close(telesub.eventque)
	}
	return nil
}

func (teleses *TelemetrySession) sendTelemetryUpdate(responses []*pb.SubscribeResponse) error {
	for _, response := range responses {
		teleses.respchan <- response
	}
	return nil
}

func getDeletes(telesub *TelemetrySubscription, path *string, deleteOnly bool) ([]*pb.Path, error) {
	deletesNum := telesub.replacedList.Size() + telesub.deletedList.Size()
	deletes := make([]*pb.Path, 0, deletesNum)
	if !deleteOnly {
		rpaths := telesub.replacedList.PrefixSearch(*path)
		for _, rpath := range rpaths {
			datapath, err := ToGNMIPath(rpath)
			if err != nil {
				return nil, fmt.Errorf("path-conversion-error(%s)", rpath)
			}
			deletes = append(deletes, datapath)
		}
	}
	dpaths := telesub.deletedList.PrefixSearch(*path)
	for _, dpath := range dpaths {
		datapath, err := ToGNMIPath(dpath)
		if err != nil {
			return nil, fmt.Errorf("path-conversion-error(%s)", dpath)
		}
		deletes = append(deletes, datapath)
	}
	return deletes, nil
}

func getUpdate(telesub *TelemetrySubscription, data *model.DataAndPath, encoding pb.Encoding) (*pb.Update, error) {
	typedValue, err := ygot.EncodeTypedValue(data.Value, encoding)
	if err != nil {
		return nil, fmt.Errorf("encoding-error(%s)", err.Error())
	}
	if typedValue == nil {
		return nil, nil
	}
	datapath, err := ToGNMIPath(data.Path)
	if err != nil {
		return nil, fmt.Errorf("update-path-conversion-error(%s)", data.Path)
	}
	var duplicates uint32
	if telesub != nil {
		paths := telesub.createdList.PrefixSearch(data.Path)
		for _, p := range paths {
			if n, ok := telesub.createdList.Find(p); ok {
				duplicates += n.Meta().(*present).duplicates
			}
		}
		paths = telesub.replacedList.PrefixSearch(data.Path)
		for _, p := range paths {
			if n, ok := telesub.replacedList.Find(p); ok {
				duplicates += n.Meta().(*present).duplicates
			}
		}
	}
	return &pb.Update{Path: datapath, Val: typedValue, Duplicates: duplicates}, nil
}

// initTelemetryUpdate - Process and generate responses for a init update.
func (teleses *TelemetrySession) initTelemetryUpdate(req *pb.SubscribeRequest) error {
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
	paths := make([]*pb.Path, 0, len(subList))
	for _, updateEntry := range subList {
		paths = append(paths, updateEntry.Path)
	}
	syncPaths := s.ModelData.GetSyncUpdatePath(prefix, paths)
	s.ModelData.RunSyncUpdate(time.Second*3, syncPaths)

	s.ModelData.RLock()
	defer s.ModelData.RUnlock()
	if err := utilities.ValidateGNMIPath(prefix); err != nil {
		return status.Errorf(codes.Unimplemented, "invalid-path(%s)", err.Error())
	}
	toplist, ok := s.Model.FindAllData(s.ModelData.GetRoot(), prefix)
	if !ok || len(toplist) <= 0 {
		if ok = s.Model.ValidatePathSchema(prefix); ok {
			// data-missing is not an error in SubscribeRPC
			// doest send any of messages ahead of the sync response.
			return teleses.sendTelemetryUpdate(buildSyncResponse())
		}
		return status.Errorf(codes.NotFound, "unknown-schema(%s)", xpath.ToXPATH(prefix))
	}

	updates := []*pb.Update{}
	for _, top := range toplist {
		bpath := top.Path
		branch := top.Value.(ygot.GoStruct)
		bprefix, err := ToGNMIPath(bpath)
		if err != nil {
			return status.Errorf(codes.Internal, "path-conversion-error(%s)", bprefix)
		}
		if bundling {
			updates = make([]*pb.Update, 0, 16)
		}
		for _, updateEntry := range subList {
			path := updateEntry.Path
			if err := utilities.ValidateGNMIFullPath(prefix, path); err != nil {
				return status.Errorf(codes.Unimplemented, "invalid-path(%s)", err.Error())
			}
			datalist, ok := s.Model.FindAllData(branch, path)
			if !ok || len(datalist) <= 0 {
				continue
			}
			for _, data := range datalist {
				u, err := getUpdate(nil, data, encoding)
				if err != nil {
					return status.Error(codes.Internal, err.Error())
				}
				if u != nil {
					if bundling {
						updates = append(updates, u)
					} else {
						err = teleses.sendTelemetryUpdate(
							buildSubscribeResponse(prefix, alias, []*pb.Update{u}, nil))
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
	if telesub.Configured.SubscriptionMode != pb.SubscriptionMode_ON_CHANGE {
		syncPaths := s.ModelData.GetSyncUpdatePath(prefix, telesub.Paths)
		s.ModelData.RunSyncUpdate(time.Second*3, syncPaths)
	}

	s.ModelData.RLock()
	defer s.ModelData.RUnlock()
	if updatedroot == nil {
		updatedroot = s.ModelData.GetRoot()
	}
	if err := utilities.ValidateGNMIPath(prefix); err != nil {
		return status.Errorf(codes.Unimplemented, "invalid-path(%s)", err.Error())
	}
	toplist, ok := s.Model.FindAllData(updatedroot, prefix)
	if !ok || len(toplist) <= 0 {
		if ok = s.Model.ValidatePathSchema(prefix); ok {
			// data-missing is not an error in SubscribeRPC
			// doest send any of messages.
			return nil
		}
		return status.Errorf(codes.NotFound, "unknown-schema(%s)", xpath.ToXPATH(prefix))
	}

	deletes := []*pb.Path{}
	updates := []*pb.Update{}

	for _, top := range toplist {
		bpath := top.Path
		branch := top.Value.(ygot.GoStruct)
		bprefix, err := ToGNMIPath(bpath)
		if err != nil {
			return status.Errorf(codes.Internal, "path-conversion-error(%s)", bprefix)
		}

		if bundling {
			updates = make([]*pb.Update, 0, 16)
			// get all replaced, deleted paths relative to the prefix
			deletes, err = getDeletes(telesub, &bpath, false)
			if err != nil {
				return status.Error(codes.Internal, err.Error())
			}
		}

		for _, path := range telesub.Paths {
			if err := utilities.ValidateGNMIFullPath(prefix, path); err != nil {
				return status.Errorf(codes.Unimplemented, "invalid-path(%s)", err.Error())
			}
			datalist, ok := s.Model.FindAllData(branch, path)
			if !ok || len(datalist) <= 0 {
				continue
			}
			for _, data := range datalist {
				u, err := getUpdate(telesub, data, encoding)
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
							buildSubscribeResponse(prefix, alias, []*pb.Update{u}, nil))
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
					buildSubscribeResponse(prefix, alias, nil, []*pb.Path{d}))
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
	prefix *pb.Path, useAliases bool, streamingMode pb.SubscriptionList_Mode, allowAggregation bool,
	encoding pb.Encoding, paths []*pb.Path, subscriptionMode pb.SubscriptionMode,
	sampleInterval uint64, suppressRedundant bool, heartbeatInterval uint64,
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
		Duplicates:        make(map[string]uint32),
		createdList:       trie.New(),
		replacedList:      trie.New(),
		deletedList:       trie.New(),
	}
	if streamingMode == pb.SubscriptionList_POLL {
		return nil, status.Errorf(codes.InvalidArgument,
			"poll subscription configured as streaming subscription")
	}
	// 3.5.1.5.2 STREAM Subscriptions Must be satisfied for telemetry update starting.
	switch telesub.SubscriptionMode {
	case pb.SubscriptionMode_TARGET_DEFINED:
		// vendor specific mode
		telesub.Configured.SubscriptionMode = pb.SubscriptionMode_SAMPLE
		telesub.Configured.SampleInterval = defaultInterval / 10
		telesub.Configured.SuppressRedundant = true
		telesub.Configured.HeartbeatInterval = 0
	case pb.SubscriptionMode_ON_CHANGE:
		if telesub.HeartbeatInterval < minimumInterval && telesub.HeartbeatInterval != 0 {
			return nil, status.Errorf(codes.InvalidArgument,
				"heartbeat_interval(< 1sec) is not supported")
		}
		telesub.Configured.SubscriptionMode = pb.SubscriptionMode_ON_CHANGE
		telesub.Configured.SampleInterval = 0
		telesub.Configured.SuppressRedundant = false
		telesub.Configured.HeartbeatInterval = telesub.HeartbeatInterval
	case pb.SubscriptionMode_SAMPLE:
		if telesub.SampleInterval < minimumInterval && telesub.SampleInterval != 0 {
			return nil, status.Errorf(codes.InvalidArgument,
				"sample_interval(< 1sec) is not supported")
		}
		if telesub.HeartbeatInterval < minimumInterval && telesub.HeartbeatInterval != 0 {
			return nil, status.Errorf(codes.InvalidArgument,
				"heartbeat_interval(< 1sec) is not supported")
		}
		telesub.Configured.SubscriptionMode = pb.SubscriptionMode_SAMPLE
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
		xpath.ToXPATH(telesub.Prefix), telesub.SampleInterval, telesub.HeartbeatInterval,
		telesub.UseAliases, telesub.AllowAggregation, telesub.SuppressRedundant,
	)
	telesub.key = &key
	telesub.eventque = make(chan *telemetryUpdateEvent, 64)
	teleses.lock()
	defer teleses.unlock()
	if t, ok := teleses.Telesub[key]; ok {
		// only updates the new path
		t.Paths = append(t.Paths, telesub.Paths...)
		telesub = t
		glog.Infof("telemetry[%d][%s].add-path(%s)", teleses.ID, key, xpath.ToXPATH(telesub.Paths[len(telesub.Paths)-1]))
		return nil, nil
	}
	subID++
	id := subID
	telesub.ID = id
	telesub.session = teleses
	teleses.Telesub[key] = telesub
	glog.Infof("telemetry[%d][%d].new(%s)", teleses.ID, telesub.ID, *telesub.key)
	glog.Infof("telemetry[%d][%d].add-path(%s)", teleses.ID, telesub.ID, xpath.ToXPATH(telesub.Paths[0]))
	return telesub, nil
}

// addPollSubscription - Create new TelemetrySubscription
func (teleses *TelemetrySession) addPollSubscription() error {
	telesub := TelemetrySubscription{
		StreamingMode: pb.SubscriptionList_POLL,
		Paths:         []*pb.Path{},
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
	if mode == pb.SubscriptionList_ONCE ||
		mode == pb.SubscriptionList_POLL ||
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
			prefix, useAliases, pb.SubscriptionList_STREAM,
			allowAggregation, encoding, []*pb.Path{path}, submod,
			SampleInterval, supressRedundant, heartBeatInterval)
		if err != nil {
			return err
		}
		if telesub != nil {
			startingList = append(startingList, telesub)
		}
	}
	block := teleses.server.ModelData.GetYDB()
	addDynamicTeleSubscriptionInfo(block, startingList)
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

func addDynamicTeleSubscriptionInfo(targetDataBlock *ydb.YDB, telesubs []*TelemetrySubscription) {
	s := ""
	for _, telesub := range telesubs {
		s += fmt.Sprintf(dynamicTeleSubInfoFormat,
			telesub.ID, telesub.ID, telesub.ID,
			"127.0.0.1", 55555,
			telesub.Configured.SampleInterval,
			telesub.Configured.HeartbeatInterval,
			telesub.Configured.SuppressRedundant,
			"STREAM_GRPC",
			telesub.Encoding,
		)
		for i := range telesub.Paths {
			p := xpath.ToXPATH(telesub.Paths[i])
			s += fmt.Sprintf(dynamicTeleSubInfoPathFormat, p, p)
		}
	}
	if s != "" {
		targetDataBlock.Write([]byte(s))
	}
}

func delDynamicTeleSubscriptionInfo(targetDataBlock *ydb.YDB, teleses *TelemetrySession) {
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
		targetDataBlock.Delete([]byte(s))
	}
}
