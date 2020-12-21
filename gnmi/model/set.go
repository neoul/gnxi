package model

import (
	"github.com/neoul/gnxi/utilities/status"
	"github.com/neoul/gnxi/utilities/xpath"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ygot/ytypes"
	"google.golang.org/grpc/codes"
)

// SetInit initializes the Set transaction.
func (m *Model) SetInit() error {
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

func newDataAndPathMap(in []*DataAndPath) map[string]*DataAndPath {
	m := make(map[string]*DataAndPath)
	for _, entry := range in {
		if _, found := m[entry.Path]; !found {
			m[entry.Path] = entry
		}
	}
	return m
}

// SetCommit commit the changed configuration. it returns an error with error index.
func (m *Model) SetCommit() (int, error) {
	var err error
	if m.StateConfig == nil {
		return m.transaction.returnSetError(
			opNone, -1, status.TaggedErrorf(codes.Internal, status.TagOperationFail, "no state-config interface"))
	}

	// delete
	if err = m.StateConfig.UpdateStart(); err != nil {
		m.StateConfig.UpdateEnd()
		return m.transaction.returnSetError(opDelete, -1, err)
	}
	for _, opinfo := range m.transaction.delete {
		curlist := m.ListAll(opinfo.curval, nil, &AddFakePrefix{Prefix: opinfo.gpath})
		for p := range newDataAndPathMap(curlist) {
			err = m.StateConfig.UpdateDelete(p)
			if err != nil {
				m.StateConfig.UpdateEnd()
				return m.transaction.returnSetError(opDelete, opinfo.opseq, err)
			}
		}
	}
	if err = m.StateConfig.UpdateEnd(); err != nil {
		return m.transaction.returnSetError(opDelete, -1, err)
	}

	// replace (delete and then update)
	if err = m.StateConfig.UpdateStart(); err != nil {
		m.StateConfig.UpdateEnd()
		return m.transaction.returnSetError(opReplace, -1, err)
	}
	for _, opinfo := range m.transaction.replace {
		newlist := m.ListAll(m.GetRoot(), opinfo.gpath)
		curlist := m.ListAll(opinfo.curval, nil, &AddFakePrefix{Prefix: opinfo.gpath})
		cur := newDataAndPathMap(curlist)
		new := newDataAndPathMap(newlist)
		// Get difference between cur and new for C/R/D operations
		for p := range cur {
			if _, exists := new[p]; !exists {
				err = m.StateConfig.UpdateDelete(p)
				if err != nil {
					m.StateConfig.UpdateEnd()
					return m.transaction.returnSetError(opReplace, opinfo.opseq, err)
				}
			}
		}
		for p, entry := range new {
			if _, exists := cur[p]; exists {
				err = m.StateConfig.UpdateReplace(p, entry.GetValueString())
			} else {
				err = m.StateConfig.UpdateCreate(p, entry.GetValueString())
			}
			if err != nil {
				m.StateConfig.UpdateEnd()
				return m.transaction.returnSetError(opReplace, opinfo.opseq, err)
			}
		}
	}
	if err = m.StateConfig.UpdateEnd(); err != nil {
		return m.transaction.returnSetError(opReplace, -1, err)
	}
	// update
	if err = m.StateConfig.UpdateStart(); err != nil {
		m.StateConfig.UpdateEnd()
		return m.transaction.returnSetError(opUpdate, -1, err)
	}
	for _, opinfo := range m.transaction.update {
		newlist := m.ListAll(m.GetRoot(), opinfo.gpath)
		curlist := m.ListAll(opinfo.curval, nil, &AddFakePrefix{Prefix: opinfo.gpath})
		cur := newDataAndPathMap(curlist)
		new := newDataAndPathMap(newlist)
		// Get difference between cur and new for C/R/D operations
		for p := range cur {
			if _, exists := new[p]; !exists {
				err = m.StateConfig.UpdateDelete(p)
				if err != nil {
					m.StateConfig.UpdateEnd()
					return m.transaction.returnSetError(opUpdate, opinfo.opseq, err)
				}
			}
		}
		for p, entry := range new {
			if _, exists := cur[p]; exists {
				err = m.StateConfig.UpdateReplace(p, entry.GetValueString())
			} else {
				err = m.StateConfig.UpdateCreate(p, entry.GetValueString())
			}
			if err != nil {
				m.StateConfig.UpdateEnd()
				return m.transaction.returnSetError(opUpdate, opinfo.opseq, err)
			}
		}
	}
	if err = m.StateConfig.UpdateEnd(); err != nil {
		return m.transaction.returnSetError(opUpdate, -1, err)
	}
	return -1, nil
}

// SetDelete deletes the path from root if the path exists.
func (m *Model) SetDelete(prefix, path *gnmipb.Path) error {
	fullpath := xpath.GNMIFullPath(prefix, path)
	targets, _ := m.Get(fullpath)
	m.transaction.setSequnce()
	for _, target := range targets {
		targetPath, err := xpath.ToGNMIPath(target.Path)
		if err != nil {
			return status.TaggedErrorf(codes.Internal, status.TagInvalidPath,
				"xpath-to-gpath converting error for %s", target.Path)
		}
		m.transaction.addOperation(opDelete, &target.Path, targetPath, target.Value)
		if len(targetPath.GetElem()) == 0 {
			// root deletion
			if mo, err := m.NewRoot(nil); err == nil {
				m.MO = mo
			} else {
				return status.TaggedErrorf(codes.Internal,
					status.TagOperationFail, "root deleting error:: %v", err)
			}
		} else {
			if err = ytypes.DeleteNode(m.GetSchema(), m.GetRoot(), targetPath); err != nil {
				return status.TaggedErrorf(codes.InvalidArgument, status.TagBadData,
					"deleting error in %s:: %v", target.Path, err)
			}
		}
	}
	return nil
}

// SetReplace deletes the path from root if the path exists.
func (m *Model) SetReplace(prefix, path *gnmipb.Path, typedValue *gnmipb.TypedValue) error {
	var err error
	fullpath := xpath.GNMIFullPath(prefix, path)
	m.transaction.setSequnce()
	targets, ok := m.Get(fullpath)
	if ok {
		for _, target := range targets {
			targetPath, err := xpath.ToGNMIPath(target.Path)
			if err != nil {
				return status.TaggedErrorf(codes.Internal, status.TagInvalidPath,
					"xpath-to-gpath converting error for %s", target.Path)
			}
			m.transaction.addOperation(opReplace, &target.Path, targetPath, target.Value)
			err = ytypes.DeleteNode(m.GetSchema(), m.GetRoot(), targetPath)
			if err != nil {
				return status.TaggedErrorf(codes.InvalidArgument, status.TagBadData,
					"deleting error in %s:: %v", target.Path, err)
			}
			err = writeTypedValue(m, targetPath, typedValue)
			if err != nil {
				return status.TaggedErrorf(codes.InvalidArgument, status.TagBadData,
					"writing error in %s:: %v", target.Path, err)
			}
		}
		return nil
	}
	tpath := xpath.ToXPath(fullpath)
	m.transaction.addOperation(opReplace, &tpath, fullpath, nil)
	err = writeTypedValue(m, fullpath, typedValue)
	if err != nil {
		return status.TaggedErrorf(codes.InvalidArgument, status.TagBadData,
			"writing error in %s:: %v", tpath, err)
	}
	return nil
}

// SetUpdate deletes the path from root if the path exists.
func (m *Model) SetUpdate(prefix, path *gnmipb.Path, typedValue *gnmipb.TypedValue) error {
	var err error
	fullpath := xpath.GNMIFullPath(prefix, path)
	m.transaction.setSequnce()
	targets, ok := m.Get(fullpath)
	if ok {
		for _, target := range targets {
			targetPath, err := xpath.ToGNMIPath(target.Path)
			if err != nil {
				return status.TaggedErrorf(codes.Internal, status.TagInvalidPath,
					"xpath-to-gpath converting error for %s", target.Path)
			}
			m.transaction.addOperation(opUpdate, &target.Path, targetPath, target.Value)
			err = writeTypedValue(m, targetPath, typedValue)
			if err != nil {
				return status.TaggedErrorf(codes.InvalidArgument, status.TagBadData,
					"writing error in %s:: %v", target.Path, err)
			}
		}
		return nil
	}
	tpath := xpath.ToXPath(fullpath)
	m.transaction.addOperation(opUpdate, &tpath, fullpath, nil)
	err = writeTypedValue(m, fullpath, typedValue)
	if err != nil {
		return status.TaggedErrorf(codes.InvalidArgument, status.TagBadData,
			"writing error in %s:: %v", tpath, err)
	}
	return nil
}

type ignoringStateConfig struct{}

func (sc *ignoringStateConfig) UpdateStart() error {
	// return status.TaggedErrorf(codes.Unimplemented, status.TagNotSupport, "not implemented StateConfig interface")
	return nil
}
func (sc *ignoringStateConfig) UpdateCreate(path string, value string) error {
	// fmt.Println("ignoringStateConfig.UpdateCreate", "C", path, value)
	return nil
}
func (sc *ignoringStateConfig) UpdateReplace(path string, value string) error {
	// fmt.Println("ignoringStateConfig.UpdateReplace", "R", path, value)
	return nil
}
func (sc *ignoringStateConfig) UpdateDelete(path string) error {
	// fmt.Println("ignoringStateConfig.UpdateDelete", "D", path)
	return nil
}
func (sc *ignoringStateConfig) UpdateEnd() error {
	// fmt.Println("ignoringStateConfig.UpdateEnd")
	return nil
}
func (sc *ignoringStateConfig) UpdateSync(path ...string) error {
	// fmt.Println("ignoringStateConfig.UpdateSync", path)
	return nil
}
