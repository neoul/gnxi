// YDB Go Interface for model data update

package model

import (
	"github.com/golang/glog"
)

// UpdateCreate - constructs the model instance for newly created data
func (m *Model) UpdateCreate(path string, value string) error {
	// fmt.Printf("m.UpdateCreate %v %v {\n", path, value)
	schema := m.RootSchema()
	err := ValWrite(schema, m.GetRoot(), path, value)
	if err == nil {
		fakeRoot := m.updatedroot.GetRoot()
		ValWrite(schema, fakeRoot, path, value)
		if onchangecb, ok := m.Callback.(ChangeNotification); ok {
			onchangecb.ChangeCreated(path, fakeRoot)
		}
	} else {
		glog.Errorf("%v", err)
	}
	// fmt.Println("}")
	return err
}

// UpdateReplace - constructs the model instance
func (m *Model) UpdateReplace(path string, value string) error {
	// fmt.Printf("m.UpdateCreate %v %v {\n", path, value)
	schema := m.RootSchema()
	err := ValWrite(schema, m.GetRoot(), path, value)
	if err == nil {
		fakeRoot := m.updatedroot.GetRoot()
		ValWrite(schema, fakeRoot, path, value)
		if onchangecb, ok := m.Callback.(ChangeNotification); ok {
			onchangecb.ChangeReplaced(path, fakeRoot)
		}
	} else {
		glog.Errorf("%v", err)
	}
	// fmt.Println("}")
	return err
}

// UpdateDelete - constructs the model instance
func (m *Model) UpdateDelete(path string) error {
	// fmt.Printf("m.UpdateDelete %v {\n", path)
	schema := m.RootSchema()
	err := ValDelete(schema, m.GetRoot(), path)
	if err == nil {
		if onchangecb, ok := m.Callback.(ChangeNotification); ok {
			onchangecb.ChangeDeleted(path)
		}
	} else {
		glog.Errorf("%v", err)
	}
	// fmt.Println("}")
	return err
}

// UpdateStart - indicates the start of the model instance update
func (m *Model) UpdateStart() {
	// updatedroot is used to save the changes of the model data.
	updatedroot, err := m.NewRoot(nil)
	if err != nil {
		return
	}
	m.updatedroot = updatedroot
	if onchangecb, ok := m.Callback.(ChangeNotification); ok {
		onchangecb.ChangeStarted(m.updatedroot.GetRoot())
	}
}

// UpdateEnd - indicates the end of the model instance update
func (m *Model) UpdateEnd() {
	if onchangecb, ok := m.Callback.(ChangeNotification); ok {
		onchangecb.ChangeFinished(m.updatedroot.GetRoot())
	}
	m.updatedroot = nil
}
