package model

import (
	"flag"
	"fmt"

	"github.com/golang/glog"
	"github.com/neoul/gnxi/utilities/xpath"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
)

// var (
// 	defaultSyncRequiredSchemaPath = []string{
// 		"/interfaces/interface/state/counters",
// 		"/interfaces/interface/time-sensitive-networking/state/statistics",
// 		"/interfaces/interface/radio-over-ethernet/state/statistics",
// 	}
// )

type syncPath []string

func (srpaths *syncPath) String() string {
	return fmt.Sprint(*srpaths)
}

func (srpaths *syncPath) Set(value string) error {
	*srpaths = append(*srpaths, value)
	return nil
}

var srpaths syncPath

func init() {
	flag.Var(&srpaths, "sync-path", "path requiring synchronization before read")
	// flag.Set("sync-path", "/interfaces/interface/state/counters")
}

func (m *Model) initStateSync() {
	for _, p := range srpaths {
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
func (m *Model) RequestStateSync(prefix *gnmipb.Path, paths []*gnmipb.Path) bool {
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
	if m.StateSync != nil && len(spaths) > 0 {
		m.StateSync.UpdateSync(spaths...)
		return true
	}
	return false
}
