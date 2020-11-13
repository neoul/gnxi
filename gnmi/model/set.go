package model

import (
	"flag"
	"fmt"
	"time"

	"github.com/golang/glog"
	"github.com/neoul/gnxi/utilities/xpath"
	"github.com/neoul/libydb/go/ydb"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/goyang/pkg/yang"
	"github.com/openconfig/ygot/ygot"
	"github.com/openconfig/ygot/ytypes"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// var (
// 	defaultSyncRequiredSchemaPath = []string{
// 		"/interfaces/interface/state/counters",
// 		"/interfaces/interface/time-sensitive-networking/state/statistics",
// 		"/interfaces/interface/radio-over-ethernet/state/statistics",
// 	}
// )

type syncRequiredPaths []string

func (srpaths *syncRequiredPaths) String() string {
	return fmt.Sprint(*srpaths)
}

func (srpaths *syncRequiredPaths) Set(value string) error {
	*srpaths = append(*srpaths, value)
	return nil
}

var srpaths syncRequiredPaths
var disableYdbChannel = flag.Bool("disable-ydb", false, "disable YAML Datablock interface")

func init() {
	flag.Var(&srpaths, "sync-required-path", "path required YDB sync operation to update data")
	// flag.Set("sync-required-path", "/interfaces/interface/state/counters")
}

// GetYDB - Get YAML DataBlock
func (m *Model) GetYDB() *ydb.YDB {
	return m.block
}

// Lock - Lock the YDB instance for use.
func (m *Model) Lock() {
	m.block.Lock()
}

// Unlock - Unlock of the YDB instance.
func (m *Model) Unlock() {
	m.block.Unlock()
}

// RLock - Lock the YDB instance for read.
func (m *Model) RLock() {
	m.block.RLock()
}

// RUnlock - Unlock of the YDB instance for read.
func (m *Model) RUnlock() {
	m.block.RUnlock()
}

// Close the connected YDB instance
func (m *Model) Close() {
	m.block.Close()
}

func buildSyncUpdatePath(entries []*yang.Entry, elems []*gnmipb.PathElem) string {
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

// GetSyncUpdatePath - synchronizes the data in the path
func (m *Model) GetSyncUpdatePath(prefix *gnmipb.Path, paths []*gnmipb.Path) []string {
	syncPaths := make([]string, 0, 8)
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
							syncPaths = append(syncPaths, buildSyncUpdatePath(entires, fullpath.GetElem()))
						}
					}
				}
				if rpath, ok := m.syncRequired.FindLongestMatch(spath); ok {
					if n, ok := m.syncRequired.Find(rpath); ok {
						entires := n.Meta().([]*yang.Entry)
						if entires != nil {
							syncPaths = append(syncPaths, buildSyncUpdatePath(entires, fullpath.GetElem()))
						}
					}
				}
			}
		} else {
			requiredPath := m.syncRequired.PrefixSearch("/")
			syncPaths = append(syncPaths, requiredPath...)
		}
	}
	return syncPaths
}

// RunSyncUpdate - synchronizes & update the data in the path. It locks model data.
func (m *Model) RunSyncUpdate(syncIgnoreTime time.Duration, syncPaths []string) {
	if syncPaths == nil || len(syncPaths) == 0 {
		return
	}
	for _, sp := range syncPaths {
		glog.Infof("sync-update %s", sp)
	}
	m.block.SyncTo(syncIgnoreTime, true, syncPaths...)
}

// WriteTypedValue - Write the TypedValue to the model instance
func (m *Model) WriteTypedValue(path *gnmipb.Path, typedValue *gnmipb.TypedValue) error {
	// var err error
	schema := m.GetSchema()
	base := m.GetRoot()
	tValue, tSchema, err := ytypes.GetOrCreateNode(schema, base, path)
	if err != nil {
		return err
	}
	if tSchema.IsDir() {
		target := tValue.(ygot.GoStruct)
		if err := m.Unmarshal(typedValue.GetJsonIetfVal(), target); err != nil {
			return status.Errorf(codes.InvalidArgument, "unmarshaling json failed: %v", err)
		}
	} else { // (schema.IsLeaf() || schema.IsLeafList())
		err = ytypes.SetNode(schema, base, path, typedValue, &ytypes.InitMissingElements{})
	}
	return err
}

// SetInit initializes the Set transaction.
func (m *Model) SetInit() error {
	if m.transaction != nil {
		return status.Errorf(codes.Unavailable, "Already running")
	}
	m.transaction = startTransaction()
	return nil
}

// SetDone resets the Set transaction.
func (m *Model) SetDone() {
	m.transaction = nil
}

// SetRollback reverts the original configuration.
func (m *Model) SetRollback() {

}

// SetCommit commit the changed configuration.
func (m *Model) SetCommit() error {
	// delete
	for _, opinfo := range m.transaction.delete {
		curlist := m.ListAll(opinfo.curval, nil, &AddFakePrefix{Prefix: opinfo.gpath}, &FindAndSort{})
		fmt.Println("Delete curlist:", curlist)
	}
	// replace (delete and then update)
	for _, opinfo := range m.transaction.replace {
		newlist := m.ListAll(m.GetRoot(), opinfo.gpath, &FindAndSort{})
		curlist := m.ListAll(opinfo.curval, nil, &AddFakePrefix{Prefix: opinfo.gpath}, &FindAndSort{})
		fmt.Println("Replace curlist:", curlist)
		fmt.Println("Replace newlist:", newlist)
	}
	// update
	for _, opinfo := range m.transaction.update {
		newlist := m.ListAll(m.GetRoot(), opinfo.gpath, &FindAndSort{})
		curlist := m.ListAll(opinfo.curval, nil, &AddFakePrefix{Prefix: opinfo.gpath}, &FindAndSort{})
		fmt.Println("Update curlist:", curlist)
		fmt.Println("Update newlist:", newlist)
	}
	m.transaction = nil
	return nil
}

// SetDelete deletes the path from root if the path exists.
func (m *Model) SetDelete(prefix, path *gnmipb.Path) error {
	fullpath := xpath.GNMIFullPath(prefix, path)
	targets, _ := m.Get(fullpath)
	for _, target := range targets {
		targetPath, err := xpath.ToGNMIPath(target.Path)
		if err != nil {
			return status.Errorf(codes.Internal, "conversion-error(%s)", target.Path)
		}
		m.transaction.add(opDelete, &target.Path, targetPath, target.Value, nil)
		if len(targetPath.GetElem()) == 0 {
			// root deletion
			if mo, err := m.NewRoot(nil); err == nil {
				m.MO = mo
			} else {
				return err
			}
		} else {
			if err = ytypes.DeleteNode(m.GetSchema(), m.GetRoot(), targetPath); err != nil {
				return err
			}
		}
	}
	return nil
}

// SetReplace deletes the path from root if the path exists.
func (m *Model) SetReplace(prefix, path *gnmipb.Path, typedValue *gnmipb.TypedValue) error {
	var err error
	fullpath := xpath.GNMIFullPath(prefix, path)
	targets, ok := m.Get(fullpath)
	if ok {
		for _, target := range targets {
			targetPath, err := xpath.ToGNMIPath(target.Path)
			if err != nil {
				return status.Errorf(codes.Internal, "conversion-error(%s)", target.Path)
			}
			m.transaction.add(opReplace, &target.Path, targetPath, target.Value, typedValue)
			err = ytypes.DeleteNode(m.GetSchema(), m.GetRoot(), targetPath)
			if err != nil {
				return err
			}
			err = m.WriteTypedValue(targetPath, typedValue)
			if err != nil {
				return err
			}
		}
		return nil
	}
	tpath := xpath.ToXPath(fullpath)
	m.transaction.add(opReplace, &tpath, fullpath, nil, typedValue)
	err = m.WriteTypedValue(fullpath, typedValue)
	if err != nil {
		return err
	}
	return nil
}

// SetUpdate deletes the path from root if the path exists.
func (m *Model) SetUpdate(prefix, path *gnmipb.Path, typedValue *gnmipb.TypedValue) error {
	var err error
	fullpath := xpath.GNMIFullPath(prefix, path)
	targets, ok := m.Get(fullpath)
	if ok {
		for _, target := range targets {
			targetPath, err := xpath.ToGNMIPath(target.Path)
			if err != nil {
				return status.Errorf(codes.Internal, "conversion-error(%s)", target.Path)
			}
			m.transaction.add(opUpdate, &target.Path, targetPath, target.Value, typedValue)
			err = m.WriteTypedValue(targetPath, typedValue)
			if err != nil {
				return err
			}
		}
		return nil
	}
	tpath := xpath.ToXPath(fullpath)
	m.transaction.add(opUpdate, &tpath, fullpath, nil, typedValue)
	err = m.WriteTypedValue(fullpath, typedValue)
	if err != nil {
		return err
	}
	return nil
}
