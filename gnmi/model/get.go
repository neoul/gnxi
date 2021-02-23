package model

import (
	"github.com/golang/glog"
	"github.com/neoul/gnxi/utilities/xpath"
	"github.com/neoul/gtrie"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/goyang/pkg/yang"
)

func (m *Model) initStateSync() {
	if m.StateSync == nil {
		return
	}
	m.stateSyncSchema = make(map[*yang.Entry]map[string]bool)
	m.stateSyncSchemaPath = gtrie.New()
	for _, p := range m.StateSync.UpdateSyncPath() {
		entry := m.FindSchemaByXPath(p)
		if entry == nil {
			if glog.V(10) {
				glog.Errorf("sync-path(invalid): %s", p)
			}
			continue
		}
		if glog.V(10) {
			glog.Infof("sync-path(added): %s", p)
		}
		m.stateSyncSchema[entry] = make(map[string]bool)
		m.stateSyncSchemaPath.Add(p, entry)
	}
}

func (m *Model) addStateSync(entry *yang.Entry, path string) {
	if m.StateSync == nil || entry == nil {
		return
	}
	if smap, ok := m.stateSyncSchema[entry]; ok {
		smap[path] = true
	}
}

func (m *Model) delStateSync(entry *yang.Entry, path string) {
	if m.StateSync == nil || entry == nil {
		return
	}
	if smap, ok := m.stateSyncSchema[entry]; ok {
		delete(smap, path)
	}
}

// RequestStateSync requests the data sync to the system before read.
// Do not use Model.Lock() before it.
func (m *Model) RequestStateSync(prefix *gnmipb.Path, paths []*gnmipb.Path) bool {
	if m.StateSync == nil {
		return false
	}
	syncpath := make([]string, 0, 8)
	for _, path := range paths {
		fullpath := xpath.GNMIFullPath(prefix, path)
		if glog.V(10) {
			glog.Infof("statesync: input %s", xpath.ToXPath(fullpath))
		}
		xpath := m.FindPaths(fullpath)
		for i := range xpath {
			entry := m.FindSchemaByXPath(xpath[i])
			if entry == nil {
				continue
			}
			if smap, ok := m.stateSyncSchema[entry]; ok {
				for p := range smap {
					syncpath = append(syncpath, p)
				}
			} else {
				found := m.stateSyncSchemaPath.All(xpath[i])
				for _, e := range found {
					entry = e.(*yang.Entry)
					if smap, ok := m.stateSyncSchema[entry]; ok {
						for p := range smap {
							syncpath = append(syncpath, p)
						}
					}
				}
			}
		}
	}
	for _, sp := range syncpath {
		if glog.V(10) {
			glog.Infof("statesync: output %s", sp)
		}
	}
	if len(syncpath) > 0 {
		m.StateSync.UpdateSync(syncpath...)
		return true
	}
	return false
}

// RequestStateSyncBySubscriptionList requests the data sync to the system before read.
// Do not use Model.Lock() before it.
func (m *Model) RequestStateSyncBySubscriptionList(subscriptionList *gnmipb.SubscriptionList) bool {
	if m.StateSync == nil {
		return false
	}
	syncpath := make([]string, 0, 8)
	for _, sub := range subscriptionList.Subscription {
		fullpath := xpath.GNMIFullPath(subscriptionList.Prefix, sub.Path)
		if glog.V(10) {
			glog.Infof("statesync: input %s", xpath.ToXPath(fullpath))
		}
		xpath := m.FindPaths(fullpath)
		for i := range xpath {
			entry := m.FindSchemaByXPath(xpath[i])
			if entry == nil {
				continue
			}
			if smap, ok := m.stateSyncSchema[entry]; ok {
				for p := range smap {
					syncpath = append(syncpath, p)
				}
			} else {
				found := m.stateSyncSchemaPath.All(xpath[i])
				for _, e := range found {
					entry = e.(*yang.Entry)
					if smap, ok := m.stateSyncSchema[entry]; ok {
						for p := range smap {
							syncpath = append(syncpath, p)
						}
					}
				}
			}
		}
	}
	for _, sp := range syncpath {
		if glog.V(10) {
			glog.Infof("statesync: output %s", sp)
		}
	}
	if len(syncpath) > 0 {
		m.StateSync.UpdateSync(syncpath...)
		return true
	}
	return false
}
