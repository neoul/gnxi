package model

import (
	"fmt"

	"github.com/golang/glog"
	"github.com/neoul/gnxi/utilities/xpath"
	"github.com/neoul/gtrie"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/goyang/pkg/yang"
)

type stateSyncCtrl struct {
	pathtrees    []*gtrie.Trie
	syncRequired bool
}

func addStateSyncSchema(stateSyncPath map[*yang.Entry]*stateSyncCtrl, entry *yang.Entry, pathtree *gtrie.Trie) {
	for _, e := range entry.Dir {
		if s := stateSyncPath[e]; s == nil {
			stateSyncPath[e] = &stateSyncCtrl{
				syncRequired: false,
			}
		}
		stateSyncPath[e].pathtrees = append(stateSyncPath[e].pathtrees, pathtree)
		addStateSyncSchema(stateSyncPath, e, pathtree)
	}
}

func (m *Model) initStateSync() {
	if m.StateSync == nil {
		return
	}
	m.stateSyncPath = make(map[*yang.Entry]*stateSyncCtrl)
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
		pathtree := gtrie.New()
		m.stateSyncPath[entry] = &stateSyncCtrl{
			pathtrees:    []*gtrie.Trie{pathtree},
			syncRequired: true,
		}

		for e := entry.Parent; e != nil; e = e.Parent {
			if s := m.stateSyncPath[e]; s == nil {
				m.stateSyncPath[e] = &stateSyncCtrl{
					syncRequired: false,
				}
			}
			m.stateSyncPath[e].pathtrees = append(m.stateSyncPath[e].pathtrees, pathtree)
			fmt.Println(m.stateSyncPath[e])
		}
		addStateSyncSchema(m.stateSyncPath, entry, pathtree)
		m.stateSyncSchemaPath.Add(p, pathtree)
	}
}

func (m *Model) addStateSyncPath(entry *yang.Entry, path string) {
	if m.StateSync == nil || entry == nil {
		return
	}
	if syncctrl, ok := m.stateSyncPath[entry]; ok && syncctrl.syncRequired {
		if n := syncctrl.pathtrees[0]; ok {
			n.Add(path, true)
		}
	}
}

func (m *Model) deleteStateSyncPath(entry *yang.Entry, path string) {
	if m.StateSync == nil || entry == nil {
		return
	}
	if syncctrl, ok := m.stateSyncPath[entry]; ok && syncctrl.syncRequired {
		if n := syncctrl.pathtrees[0]; ok {
			n.Remove(path)
		}
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
		isSchemaPath := xpath.IsSchemaPath(fullpath)
		strpath := m.FindPaths(fullpath)
		for i := range strpath {
			if isSchemaPath {
				pathtrees := m.stateSyncSchemaPath.FindAll(strpath[i])
				for _, data := range pathtrees {
					if pathtree, ok := data.(*gtrie.Trie); ok {
						_syncpath := pathtree.FindAll("")
						for p := range _syncpath {
							syncpath = append(syncpath, p)
						}
					}
				}
			} else {
				entry := m.FindSchemaByXPath(strpath[i])
				syncnode := m.stateSyncPath[entry]
				if syncnode == nil {
					continue
				}
				for _, pathtree := range syncnode.pathtrees {
					_syncpath := pathtree.FindAll(strpath[i])
					for p := range _syncpath {
						syncpath = append(syncpath, p)
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
		isSchemaPath := xpath.IsSchemaPath(fullpath)
		strpath := m.FindPaths(fullpath)
		for i := range strpath {
			if isSchemaPath {
				pathtrees := m.stateSyncSchemaPath.FindAll(strpath[i])
				for _, data := range pathtrees {
					if pathtree, ok := data.(*gtrie.Trie); ok {
						_syncpath := pathtree.FindAll("")
						for p := range _syncpath {
							syncpath = append(syncpath, p)
						}
					}
				}
			} else {
				entry := m.FindSchemaByXPath(strpath[i])
				syncnode := m.stateSyncPath[entry]
				if syncnode == nil {
					continue
				}
				for i := range syncnode.pathtrees {
					_syncpath := syncnode.pathtrees[i].FindAll(strpath[i])
					for p := range _syncpath {
						syncpath = append(syncpath, p)
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
