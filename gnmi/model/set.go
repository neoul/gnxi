package model

import (
	"github.com/golang/glog"
	"github.com/neoul/gnxi/utilities/status"
	"github.com/neoul/gnxi/utilities/xpath"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ygot/ytypes"
	"google.golang.org/grpc/codes"
)

type backupEntry struct {
	optype   gnmipb.UpdateResult_Operation
	xpath    *string
	gpath    *gnmipb.Path
	oldval   interface{}
	executed bool
}

func (be *backupEntry) markExecuted() {
	be.executed = true
}

func (m *Model) addBackupEntry(optype gnmipb.UpdateResult_Operation,
	xpath *string, gpath *gnmipb.Path, oldval interface{}) *backupEntry {
	entry := &backupEntry{
		optype: optype,
		xpath:  xpath,
		gpath:  gpath,
		oldval: oldval,
	}
	m.setBackup = append(m.setBackup, entry)
	return entry
}

// SetInit initializes the Set transaction.
func (m *Model) SetInit() error {
	m.setSeq++
	m.setBackup = nil
	return nil
}

// SetDone resets the Set transaction.
func (m *Model) SetDone() {
	if m.setBackup == nil {
		return
	}
	// ignore SetDone if there is not the StateConfig interface
	if m.StateConfig == nil {
		return
	}

	// Revert back the new data to the old data
	// to refresh the data by the StateUpdate interface for data sync.
	for _, entry := range m.setBackup {
		var err error
		if glog.V(10) {
			glog.Infof("set.done(%s)", *entry.xpath)
		}
		if entry.oldval == nil {
			err = m.DeleteValue(entry.gpath)
		} else {
			err = m.WriteValue(entry.gpath, entry.oldval)
		}
		if err != nil {
			if glog.V(10) {
				glog.Error(status.TaggedErrorf(codes.Internal, status.TagOperationFail,
					"set.done error(ignored*) in %s:: %v", *entry.xpath, err))
			}
			continue
		}
	}
	m.setBackup = nil
}

// SetRollback reverts the original configuration.
func (m *Model) SetRollback() {
	for _, entry := range m.setBackup {
		var err error
		if glog.V(10) {
			glog.Infof("set.rollback(%s)", *entry.xpath)
		}
		new := m.FindValue(entry.gpath)
		if entry.oldval == nil {
			err = m.DeleteValue(entry.gpath)
		} else {
			err = m.WriteValue(entry.gpath, entry.oldval)
		}
		if err != nil {
			if glog.V(10) {
				glog.Error(status.TaggedErrorf(codes.Internal, status.TagOperationFail,
					"rollback.delete error(ignored*) in %s:: %v", *entry.xpath, err))
			}
			continue
		}
		// skip entry if not executed
		if !entry.executed {
			continue
		}
		// error is ignored on rollback
		if err := m.executeStateConfig(entry.gpath, new); err != nil {
			if glog.V(10) {
				glog.Error(status.TaggedErrorf(codes.Internal, status.TagOperationFail,
					"rollback.delete error(ignored*) in %s:: %v", *entry.xpath, err))
			}
		}
	}
	m.setBackup = nil
}

// SetCommit commit the changed configuration. it returns an error.
func (m *Model) SetCommit() error {
	return nil
}

// SetDelete deletes the path from root if the path exists.
func (m *Model) SetDelete(prefix, path *gnmipb.Path) error {
	fullpath := xpath.GNMIFullPath(prefix, path)
	if glog.V(10) {
		glog.Infof("set.delete(%v)", xpath.ToXPath(fullpath))
	}
	targets, _ := m.Get(fullpath)
	for _, target := range targets {
		targetPath, err := xpath.ToGNMIPath(target.Path)
		if err != nil {
			return status.TaggedErrorf(codes.Internal, status.TagInvalidPath,
				"path.converting error for %s", target.Path)
		}
		if s := m.FindSchemaByGNMIPath(targetPath); s != nil {
			if s.ReadOnly() {
				return status.TaggedErrorf(codes.InvalidArgument, status.TagInvalidPath,
					"unable to set read-only data in %s", target.Path)
			}
		}
		e := m.addBackupEntry(gnmipb.UpdateResult_DELETE, &target.Path, targetPath, target.Value)
		if len(targetPath.GetElem()) == 0 {
			// root deletion
			m.MO = m.NewEmptyRoot()
		} else {
			if err = ytypes.DeleteNode(m.GetSchema(), m.GetRoot(), targetPath); err != nil {
				return status.TaggedErrorf(codes.InvalidArgument, status.TagBadData,
					"set.delete error in %s:: %v", target.Path, err)
			}
		}
		if err := m.executeStateConfig(targetPath, target.Value); err != nil {
			return status.TaggedErrorf(codes.Internal, status.TagOperationFail, "set error:: %v", err)
		}
		e.markExecuted()
	}
	return nil
}

// SetReplace deletes the path from root if the path exists.
func (m *Model) SetReplace(prefix, path *gnmipb.Path, typedValue *gnmipb.TypedValue) error {
	var err error
	fullpath := xpath.GNMIFullPath(prefix, path)
	if glog.V(10) {
		glog.Infof("set.replace(%v, %v)", xpath.ToXPath(fullpath), typedValue)
	}
	targets, ok := m.Get(fullpath)
	if !ok {
		tpath := xpath.ToXPath(fullpath)
		if s := m.FindSchemaByGNMIPath(fullpath); s != nil {
			if s.ReadOnly() {
				return status.TaggedErrorf(codes.InvalidArgument, status.TagInvalidPath,
					"unable to set read-only data in %s", tpath)
			}
		}
		e := m.addBackupEntry(gnmipb.UpdateResult_REPLACE, &tpath, fullpath, nil)
		err = m.WriteTypedValue(fullpath, typedValue)
		if err != nil {
			return status.TaggedErrorf(codes.InvalidArgument, status.TagBadData,
				"replace error in %s:: %v", tpath, err)
		}
		if err := m.executeStateConfig(fullpath, nil); err != nil {
			return status.TaggedErrorf(codes.Internal, status.TagOperationFail, "set error:: %v", err)
		}
		e.markExecuted()
		return nil
	}
	for _, target := range targets {
		targetPath, err := xpath.ToGNMIPath(target.Path)
		if err != nil {
			return status.TaggedErrorf(codes.Internal, status.TagInvalidPath,
				"path.converting error for %s", target.Path)
		}
		if s := m.FindSchemaByGNMIPath(targetPath); s != nil {
			if s.ReadOnly() {
				return status.TaggedErrorf(codes.InvalidArgument, status.TagInvalidPath,
					"unable to set read-only data in %s", target.Path)
			}
		}
		e := m.addBackupEntry(gnmipb.UpdateResult_REPLACE, &target.Path, targetPath, target.Value)
		err = ytypes.DeleteNode(m.GetSchema(), m.GetRoot(), targetPath)
		if err != nil {
			return status.TaggedErrorf(codes.InvalidArgument, status.TagBadData,
				"set.replace error in %s:: %v", target.Path, err)
		}
		err = m.WriteTypedValue(targetPath, typedValue)
		if err != nil {
			return status.TaggedErrorf(codes.InvalidArgument, status.TagBadData,
				"set.replace error in %s:: %v", target.Path, err)
		}
		if err := m.executeStateConfig(targetPath, target.Value); err != nil {
			return status.TaggedErrorf(codes.Internal, status.TagOperationFail, "set error:: %v", err)
		}
		e.markExecuted()
	}
	return nil
}

// SetUpdate deletes the path from root if the path exists.
func (m *Model) SetUpdate(prefix, path *gnmipb.Path, typedValue *gnmipb.TypedValue) error {
	var err error
	fullpath := xpath.GNMIFullPath(prefix, path)
	if glog.V(10) {
		glog.Infof("set.update(%v, %v)", xpath.ToXPath(fullpath), typedValue)
	}
	targets, ok := m.Get(fullpath)
	if !ok {
		tpath := xpath.ToXPath(fullpath)
		if s := m.FindSchemaByGNMIPath(fullpath); s != nil {
			if s.ReadOnly() {
				return status.TaggedErrorf(codes.InvalidArgument, status.TagInvalidPath,
					"unable to set read-only data in %s", tpath)
			}
		}
		e := m.addBackupEntry(gnmipb.UpdateResult_UPDATE, &tpath, fullpath, nil)
		err = m.WriteTypedValue(fullpath, typedValue)
		if err != nil {
			return status.TaggedErrorf(codes.InvalidArgument, status.TagBadData,
				"set.update error in %s:: %v", tpath, err)
		}
		if err := m.executeStateConfig(fullpath, nil); err != nil {
			return status.TaggedErrorf(codes.Internal, status.TagOperationFail, "set error:: %v", err)
		}
		e.markExecuted()
		return nil
	}
	for _, target := range targets {
		targetPath, err := xpath.ToGNMIPath(target.Path)
		if err != nil {
			return status.TaggedErrorf(codes.Internal, status.TagInvalidPath,
				"path.converting error for %s", target.Path)
		}
		if s := m.FindSchemaByGNMIPath(targetPath); s != nil {
			if s.ReadOnly() {
				return status.TaggedErrorf(codes.InvalidArgument, status.TagInvalidPath,
					"unable to set read-only data in %s", target.Path)
			}
		}
		e := m.addBackupEntry(gnmipb.UpdateResult_UPDATE, &target.Path, targetPath, target.Value)
		err = m.WriteTypedValue(targetPath, typedValue)
		if err != nil {
			return status.TaggedErrorf(codes.InvalidArgument, status.TagBadData,
				"set.update error in %s:: %v", target.Path, err)
		}
		if err := m.executeStateConfig(targetPath, target.Value); err != nil {
			return status.TaggedErrorf(codes.Internal, status.TagOperationFail, "set error:: %v", err)
		}
		e.markExecuted()
	}
	return nil
}

func mapDataAndPath(in []*DataAndPath) map[string]*DataAndPath {
	m := make(map[string]*DataAndPath)
	for _, entry := range in {
		if _, found := m[entry.Path]; !found {
			m[entry.Path] = entry
		}
	}
	return m
}

func (m *Model) executeStateConfig(gpath *gnmipb.Path, oldval interface{}) error {
	if m.StateConfig == nil {
		return nil
	}
	curlist := m.ListAll(oldval, nil, &AddFakePrefix{Prefix: gpath})
	newlist := m.ListAll(m.GetRoot(), gpath)
	cur := mapDataAndPath(curlist)
	new := mapDataAndPath(newlist)

	if err := m.StateConfig.UpdateStart(); err != nil {
		m.StateConfig.UpdateEnd()
		return err
	}
	// Get difference between cur and new for C/R/D operations
	for _, entry := range curlist {
		if _, exists := new[entry.Path]; !exists {
			err := m.StateConfig.UpdateDelete(entry.Path)
			if err != nil {
				m.StateConfig.UpdateEnd()
				return err
			}
		}
	}
	for _, entry := range newlist {
		var err error
		if _, exists := cur[entry.Path]; exists {
			err = m.StateConfig.UpdateReplace(entry.Path, entry.GetValueString())
		} else {
			err = m.StateConfig.UpdateCreate(entry.Path, entry.GetValueString())
		}
		if err != nil {
			m.StateConfig.UpdateEnd()
			return err
		}
	}
	if err := m.StateConfig.UpdateEnd(); err != nil {
		return err
	}
	return nil
}
