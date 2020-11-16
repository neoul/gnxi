package model

import (
	"fmt"

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

func newDataAndPathMap(in []*DataAndPath) map[string]*DataAndPath {
	m := make(map[string]*DataAndPath)
	for _, entry := range in {
		if _, found := m[entry.Path]; !found {
			m[entry.Path] = entry
		}
	}
	return m
}

// SetCommit commit the changed configuration.
func (m *Model) SetCommit() error {
	if m.StateConfig == nil {
		m.transaction = nil
		return fmt.Errorf("no StateConfig interface configured")
	}
	m.StateConfig.UpdateStart()
	// delete
	for _, opinfo := range m.transaction.delete {
		curlist := m.ListAll(opinfo.curval, nil, &AddFakePrefix{Prefix: opinfo.gpath}, &FindAndSort{})
		for p := range newDataAndPathMap(curlist) {
			m.StateConfig.UpdateDelete(p)
		}
	}
	// replace (delete and then update)
	for _, opinfo := range m.transaction.replace {
		newlist := m.ListAll(m.GetRoot(), opinfo.gpath, &FindAndSort{})
		curlist := m.ListAll(opinfo.curval, nil, &AddFakePrefix{Prefix: opinfo.gpath}, &FindAndSort{})
		cur := newDataAndPathMap(curlist)
		new := newDataAndPathMap(newlist)
		// Get difference between cur and new for C/R/D operations
		for p := range cur {
			if _, exists := new[p]; !exists {
				m.StateConfig.UpdateDelete(p)
			}
		}
		for p, entry := range new {
			if _, exists := cur[p]; exists {
				m.StateConfig.UpdateReplace(p, entry.GetValueString())
			} else {
				m.StateConfig.UpdateCreate(p, entry.GetValueString())
			}
		}
	}
	// update
	for _, opinfo := range m.transaction.update {
		newlist := m.ListAll(m.GetRoot(), opinfo.gpath, &FindAndSort{})
		curlist := m.ListAll(opinfo.curval, nil, &AddFakePrefix{Prefix: opinfo.gpath}, &FindAndSort{})
		cur := newDataAndPathMap(curlist)
		new := newDataAndPathMap(newlist)
		// Get difference between cur and new for C/R/D operations
		for p := range cur {
			if _, exists := new[p]; !exists {
				m.StateConfig.UpdateDelete(p)
			}
		}
		for p, entry := range new {
			if _, exists := cur[p]; exists {
				m.StateConfig.UpdateReplace(p, entry.GetValueString())
			} else {
				m.StateConfig.UpdateCreate(p, entry.GetValueString())
			}
		}
	}
	m.StateConfig.UpdateEnd()
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

type emptySource struct{}

func (sc *emptySource) UpdateStart() {
	// fmt.Println("emptySource.UpdateStart")
}
func (sc *emptySource) UpdateCreate(path string, value string) error {
	// fmt.Println("emptySource.UpdateCreate", "C", path, value)
	return nil
}
func (sc *emptySource) UpdateReplace(path string, value string) error {
	// fmt.Println("emptySource.UpdateReplace", "R", path, value)
	return nil
}
func (sc *emptySource) UpdateDelete(path string) error {
	// fmt.Println("emptySource.UpdateDelete", "D", path)
	return nil
}
func (sc *emptySource) UpdateEnd() {
	// fmt.Println("emptySource.UpdateEnd")
}
func (sc *emptySource) UpdateSync(path string) error {
	// fmt.Println("emptySource.UpdateSync", path)
	return nil
}
