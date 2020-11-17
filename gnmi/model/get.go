package model

import (
	"flag"
	"fmt"

	"github.com/golang/glog"
	"github.com/neoul/gnxi/utilities/xpath"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/goyang/pkg/yang"
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
		// glog.Infof("sync-path %s", p)
		entry := m.FindSchemaByXPath(p)
		if entry == nil {
			continue
		}
		sentries := []*yang.Entry{}
		for entry != nil {
			sentries = append([]*yang.Entry{entry}, sentries...)
			entry = entry.Parent
		}
		m.syncRequired.Add(p, sentries)
	}
}

// RequestStateSync - synchronizes the data in the path
func (m *Model) RequestStateSync(prefix *gnmipb.Path, paths []*gnmipb.Path) {
	buildPaths := func(entries []*yang.Entry, elems []*gnmipb.PathElem) string {
		entrieslen := len(entries)
		elemslen := len(elems)
		if entrieslen > elemslen {
			for i := elemslen + 1; i < entrieslen; i++ {
				elems = append(elems, &gnmipb.PathElem{Name: entries[i].Name})
			}
			return xpath.PathElemToXPATH(elems, false)
		}
		return xpath.PathElemToXPATH(elems[:entrieslen], false)
	}
	spaths := make([]string, 0, 8)
	for _, path := range paths {
		// glog.Info(":::SynUpdate:::", xpath.ToXPath(xpath.GNMIFullPath(prefix, path)))
		fullpath := xpath.GNMIFullPath(prefix, path)
		if len(fullpath.GetElem()) > 0 {
			schemaPaths, ok := m.FindSchemaPaths(fullpath)
			if !ok {
				continue
			}
			for _, spath := range schemaPaths {
				requiredPath := m.syncRequired.PrefixSearch(spath)
				for _, rpath := range requiredPath {
					if n, ok := m.syncRequired.Find(rpath); ok {
						entires := n.Meta().([]*yang.Entry)
						if entires != nil {
							spaths = append(spaths, buildPaths(entires, fullpath.GetElem()))
						}
					}
				}
				if rpath, ok := m.syncRequired.FindLongestMatch(spath); ok {
					if n, ok := m.syncRequired.Find(rpath); ok {
						entires := n.Meta().([]*yang.Entry)
						if entires != nil {
							spaths = append(spaths, buildPaths(entires, fullpath.GetElem()))
						}
					}
				}
			}
		} else {
			requiredPath := m.syncRequired.PrefixSearch("/")
			spaths = append(spaths, requiredPath...)
		}
	}
	for _, sp := range spaths {
		glog.Infof("sync-update %s", sp)
	}
	if m.StateSync != nil {
		m.StateSync.UpdateSync(spaths...)
		// syncIgnoreTime := time.Second * 3
		// m.UpdateSync(syncIgnoreTime, true, spaths...)
		// m.UpdateSync(spaths...)
	}
}
