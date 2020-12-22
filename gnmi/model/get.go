package model

import (
	"github.com/golang/glog"
	"github.com/neoul/gnxi/utilities/xpath"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
)

func (m *Model) initStateSync(ss StateSync) {
	if ss == nil {
		return
	}
	for _, p := range ss.UpdateSyncPath() {
		entry := m.FindSchemaByXPath(p)
		if entry == nil {
			glog.Errorf("sync-path(invalid): %s", p)
		} else {
			glog.Infof("sync-path(added): %s", p)
			m.stateSyncPath.Add(p, true)
		}
	}
}

// RequestStateSync requests the data sync to the system before read.
// Do not use Model.Lock() before it.
func (m *Model) RequestStateSync(prefix *gnmipb.Path, paths []*gnmipb.Path) bool {
	if m.StateSync == nil {
		return false
	}
	spaths := make([]string, 0, 8)
	for _, path := range paths {
		fullpath := xpath.GNMIFullPath(prefix, path)
		// glog.Infof("StateSync check %s", xpath.ToXPath(fullpath))
		if len(fullpath.GetElem()) > 0 {
			schemaPaths, _ := m.FindSchemaPaths(fullpath)
			for _, spath := range schemaPaths {
				found := m.stateSyncPath.FindAll(spath)
				for p := range found {
					spaths = append(spaths, p)
				}
			}
		} else {
			requiredPath := m.stateSyncPath.PrefixSearch("/")
			spaths = append(spaths, requiredPath...)
		}
	}
	for _, sp := range spaths {
		glog.Infof("StateSync %s", sp)
	}
	if len(spaths) > 0 {
		m.StateSync.UpdateSync(spaths...)
		return true
	}
	return false
}
