package model

import (
	"github.com/neoul/gnxi/utilities/xpath"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ygot/ygot"
	"github.com/openconfig/ygot/ytypes"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

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
	var opseq int = -1
	if m.StateConfig == nil {
		return m.transaction.returnSetErr(
			opNone, opseq, status.Errorf(codes.Internal, "no StateConfig interface"))
	}

	// delete
	if err = m.StateConfig.UpdateStart(); err != nil {
		m.StateConfig.UpdateEnd()
		return m.transaction.returnSetErr(opDelete, opseq, err)
	}
	for _, opinfo := range m.transaction.delete {
		curlist := m.ListAll(opinfo.curval, nil, &AddFakePrefix{Prefix: opinfo.gpath})
		for p := range newDataAndPathMap(curlist) {
			err = m.StateConfig.UpdateDelete(p)
			if err != nil {
				m.StateConfig.UpdateEnd()
				return m.transaction.returnSetErr(opDelete, opinfo.opseq, err)
			}
		}
	}
	if err = m.StateConfig.UpdateEnd(); err != nil {
		return m.transaction.returnSetErr(opDelete, opseq, err)
	}

	// replace (delete and then update)
	if err = m.StateConfig.UpdateStart(); err != nil {
		m.StateConfig.UpdateEnd()
		return m.transaction.returnSetErr(opReplace, opseq, err)
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
					return m.transaction.returnSetErr(opReplace, opinfo.opseq, err)
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
				return m.transaction.returnSetErr(opReplace, opinfo.opseq, err)
			}
		}
	}
	if err = m.StateConfig.UpdateEnd(); err != nil {
		return m.transaction.returnSetErr(opReplace, opseq, err)
	}
	// update
	if err = m.StateConfig.UpdateStart(); err != nil {
		m.StateConfig.UpdateEnd()
		return m.transaction.returnSetErr(opUpdate, opseq, err)
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
					return m.transaction.returnSetErr(opUpdate, opinfo.opseq, err)
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
				return m.transaction.returnSetErr(opUpdate, opinfo.opseq, err)
			}
		}
	}
	if err = m.StateConfig.UpdateEnd(); err != nil {
		return m.transaction.returnSetErr(opUpdate, opseq, err)
	}
	return opseq, err
}

// SetDelete deletes the path from root if the path exists.
func (m *Model) SetDelete(prefix, path *gnmipb.Path) error {
	fullpath := xpath.GNMIFullPath(prefix, path)
	targets, _ := m.Get(fullpath)
	m.transaction.setSequnce(opDelete, fullpath)
	for _, target := range targets {
		targetPath, err := xpath.ToGNMIPath(target.Path)
		if err != nil {
			return status.Errorf(codes.Internal, "conversion-error(%s)", target.Path)
		}
		m.transaction.addOperation(opDelete, &target.Path, targetPath, target.Value)
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
	m.transaction.setSequnce(opReplace, fullpath)
	targets, ok := m.Get(fullpath)
	if ok {
		for _, target := range targets {
			targetPath, err := xpath.ToGNMIPath(target.Path)
			if err != nil {
				return status.Errorf(codes.Internal, "conversion-error(%s)", target.Path)
			}
			m.transaction.addOperation(opReplace, &target.Path, targetPath, target.Value)
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
	m.transaction.addOperation(opReplace, &tpath, fullpath, nil)
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
	m.transaction.setSequnce(opUpdate, fullpath)
	targets, ok := m.Get(fullpath)
	if ok {
		for _, target := range targets {
			targetPath, err := xpath.ToGNMIPath(target.Path)
			if err != nil {
				return status.Errorf(codes.Internal, "conversion-error(%s)", target.Path)
			}
			m.transaction.addOperation(opUpdate, &target.Path, targetPath, target.Value)
			err = m.WriteTypedValue(targetPath, typedValue)
			if err != nil {
				return err
			}
		}
		return nil
	}
	tpath := xpath.ToXPath(fullpath)
	m.transaction.addOperation(opUpdate, &tpath, fullpath, nil)
	err = m.WriteTypedValue(fullpath, typedValue)
	if err != nil {
		return err
	}
	return nil
}

type emptyStateConfig struct{}

func (sc *emptyStateConfig) UpdateStart() error {
	// fmt.Println("emptyStateConfig.UpdateStart")
	return nil
}
func (sc *emptyStateConfig) UpdateCreate(path string, value string) error {
	// fmt.Println("emptyStateConfig.UpdateCreate", "C", path, value)
	return nil
}
func (sc *emptyStateConfig) UpdateReplace(path string, value string) error {
	// fmt.Println("emptyStateConfig.UpdateReplace", "R", path, value)
	return nil
}
func (sc *emptyStateConfig) UpdateDelete(path string) error {
	// fmt.Println("emptyStateConfig.UpdateDelete", "D", path)
	return nil
}
func (sc *emptyStateConfig) UpdateEnd() error {
	// fmt.Println("emptyStateConfig.UpdateEnd")
	return nil
}
func (sc *emptyStateConfig) UpdateSync(path ...string) error {
	// fmt.Println("emptyStateConfig.UpdateSync", path)
	return nil
}
