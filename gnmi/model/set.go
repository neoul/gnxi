package model

import (
	"github.com/golang/glog"
	"github.com/neoul/gnxi/utilities/status"
	"github.com/neoul/gnxi/utilities/xpath"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ygot/ytypes"
	"google.golang.org/grpc/codes"
)

type rollbackEntry struct {
	optype gnmipb.UpdateResult_Operation
	xpath  *string
	gpath  *gnmipb.Path
	oldval interface{}
}

func (m *Model) addRollbackEntry(optype gnmipb.UpdateResult_Operation,
	xpath *string, gpath *gnmipb.Path, oldval interface{}) {
	unit := &rollbackEntry{
		optype: optype,
		xpath:  xpath,
		gpath:  gpath,
		oldval: oldval,
	}
	m.setRollback = append(m.setRollback, unit)
	return
}

// SetInit initializes the Set transaction.
func (m *Model) SetInit() error {
	m.setSeq++
	m.setRollback = nil
	return nil
}

// SetDone resets the Set transaction.
func (m *Model) SetDone() {
	m.setRollback = nil
}

// SetRollback reverts the original configuration.
func (m *Model) SetRollback() {
	for _, entry := range m.setRollback {
		var err error
		if glog.V(2) {
			glog.Infof("set.rollback(%s)", *entry.xpath)
		}
		new := m.FindValue(entry.gpath)
		if entry.oldval == nil {
			err = m.DeleteValue(entry.gpath)
		} else {
			err = m.WriteValue(entry.gpath, entry.oldval)
		}
		if err != nil {
			glog.Error(status.TaggedErrorf(codes.Internal, status.TagOperationFail,
				"rollback.delete error(ignored*) in %s:: %v", *entry.xpath, err))
			continue
		}
		// error is ignored on rollback
		if err := m.executeStateConfig(entry.gpath, new); err != nil {
			glog.Error(status.TaggedErrorf(codes.Internal, status.TagOperationFail,
				"rollback.delete error(ignored*) in %s:: %v", *entry.xpath, err))
			continue
		}
	}
}

// SetCommit commit the changed configuration. it returns an error.
func (m *Model) SetCommit() error {
	return nil
}

// SetDelete deletes the path from root if the path exists.
func (m *Model) SetDelete(prefix, path *gnmipb.Path) error {
	fullpath := xpath.GNMIFullPath(prefix, path)
	if glog.V(2) {
		glog.Infof("set.delete(%v)", xpath.ToXPath(fullpath))
	}
	targets, _ := m.Get(fullpath)
	for _, target := range targets {
		targetPath, err := xpath.ToGNMIPath(target.Path)
		if err != nil {
			return status.TaggedErrorf(codes.Internal, status.TagInvalidPath,
				"path.converting error for %s", target.Path)
		}
		if len(targetPath.GetElem()) == 0 {
			// root deletion
			m.MO = m.NewEmptyRoot()
		} else {
			if err = ytypes.DeleteNode(m.GetSchema(), m.GetRoot(), targetPath); err != nil {
				return status.TaggedErrorf(codes.InvalidArgument, status.TagBadData,
					"set.delete error in %s:: %v", target.Path, err)
			}
		}
		m.addRollbackEntry(gnmipb.UpdateResult_DELETE, &target.Path, targetPath, target.Value)
		if err := m.executeStateConfig(targetPath, target.Value); err != nil {
			return status.TaggedErrorf(codes.Internal, status.TagOperationFail, "set error:: %v", err)
		}
	}
	return nil
}

// SetReplace deletes the path from root if the path exists.
func (m *Model) SetReplace(prefix, path *gnmipb.Path, typedValue *gnmipb.TypedValue) error {
	var err error
	fullpath := xpath.GNMIFullPath(prefix, path)
	if glog.V(2) {
		glog.Infof("set.replace(%v, %v)", xpath.ToXPath(fullpath), typedValue)
	}
	targets, ok := m.Get(fullpath)
	if !ok {
		tpath := xpath.ToXPath(fullpath)
		err = m.WriteTypedValue(fullpath, typedValue)
		if err != nil {
			return status.TaggedErrorf(codes.InvalidArgument, status.TagBadData,
				"replace error in %s:: %v", tpath, err)
		}
		m.addRollbackEntry(gnmipb.UpdateResult_REPLACE, &tpath, fullpath, nil)
		if err := m.executeStateConfig(fullpath, nil); err != nil {
			return status.TaggedErrorf(codes.Internal, status.TagOperationFail, "set error:: %v", err)
		}
		return nil
	}
	for _, target := range targets {
		targetPath, err := xpath.ToGNMIPath(target.Path)
		if err != nil {
			return status.TaggedErrorf(codes.Internal, status.TagInvalidPath,
				"path.converting error for %s", target.Path)
		}
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
		m.addRollbackEntry(gnmipb.UpdateResult_REPLACE, &target.Path, targetPath, target.Value)
		if err := m.executeStateConfig(targetPath, target.Value); err != nil {
			return status.TaggedErrorf(codes.Internal, status.TagOperationFail, "set error:: %v", err)
		}
	}
	return nil
}

// SetUpdate deletes the path from root if the path exists.
func (m *Model) SetUpdate(prefix, path *gnmipb.Path, typedValue *gnmipb.TypedValue) error {
	var err error
	fullpath := xpath.GNMIFullPath(prefix, path)
	if glog.V(2) {
		glog.Infof("set.update(%v, %v)", xpath.ToXPath(fullpath), typedValue)
	}
	targets, ok := m.Get(fullpath)
	if !ok {
		tpath := xpath.ToXPath(fullpath)
		err = m.WriteTypedValue(fullpath, typedValue)
		if err != nil {
			return status.TaggedErrorf(codes.InvalidArgument, status.TagBadData,
				"set.update error in %s:: %v", tpath, err)
		}
		m.addRollbackEntry(gnmipb.UpdateResult_UPDATE, &tpath, fullpath, nil)
		if err := m.executeStateConfig(fullpath, nil); err != nil {
			return status.TaggedErrorf(codes.Internal, status.TagOperationFail, "set error:: %v", err)
		}
		return nil
	}
	for _, target := range targets {
		targetPath, err := xpath.ToGNMIPath(target.Path)
		if err != nil {
			return status.TaggedErrorf(codes.Internal, status.TagInvalidPath,
				"path.converting error for %s", target.Path)
		}
		err = m.WriteTypedValue(targetPath, typedValue)
		if err != nil {
			return status.TaggedErrorf(codes.InvalidArgument, status.TagBadData,
				"set.update error in %s:: %v", target.Path, err)
		}
		m.addRollbackEntry(gnmipb.UpdateResult_UPDATE, &target.Path, targetPath, target.Value)
		if err := m.executeStateConfig(targetPath, target.Value); err != nil {
			return status.TaggedErrorf(codes.Internal, status.TagOperationFail, "set error:: %v", err)
		}
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

type ignoringStateConfig struct{}

func (sc *ignoringStateConfig) UpdateStart() error {
	// return status.TaggedErrorf(codes.Unimplemented, status.TagNotSupport, "not implemented StateConfig interface")
	return nil
}
func (sc *ignoringStateConfig) UpdateCreate(path string, value string) error {
	return nil
}
func (sc *ignoringStateConfig) UpdateReplace(path string, value string) error {
	return nil
}
func (sc *ignoringStateConfig) UpdateDelete(path string) error {
	return nil
}
func (sc *ignoringStateConfig) UpdateEnd() error {
	return nil
}
func (sc *ignoringStateConfig) UpdateSync(path ...string) error {
	return nil
}
