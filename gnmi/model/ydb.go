// YDB Go Interface for model data update

package model

import (
	"github.com/golang/glog"
)

// Create - constructs the model instance
func (m *Model) Create(keys []string, key string, tag string, value string) error {
	// fmt.Printf("m.Create %v %v %v %v {\n", keys, key, tag, value)
	keys = append(keys, key)
	schema := m.FindSchema(m.dataroot)
	err := ValWrite(schema, m.dataroot, keys, value)
	if err == nil {
		ValWrite(schema, m.updatedroot, keys, value)
		if onchangecb, ok := m.Callback.(ChangeNotification); ok {
			onchangecb.ChangeCreated(keys, m.updatedroot)
		}
	} else {
		glog.Errorf("%v", err)
	}
	// fmt.Println("}")
	return err
}

// Replace - constructs the model instance
func (m *Model) Replace(keys []string, key string, tag string, value string) error {
	// fmt.Printf("m.Replace %v %v %v %v {\n", keys, key, tag, value)
	keys = append(keys, key)
	schema := m.FindSchema(m.dataroot)
	err := ValWrite(schema, m.dataroot, keys, value)
	if err == nil {
		ValWrite(schema, m.updatedroot, keys, value)
		if onchangecb, ok := m.Callback.(ChangeNotification); ok {
			onchangecb.ChangeReplaced(keys, m.updatedroot)
		}
	} else {
		glog.Errorf("%v", err)
	}
	// fmt.Println("}")
	return err
}

// Delete - constructs the model instance
func (m *Model) Delete(keys []string, key string) error {
	// fmt.Printf("m.Delete %v %v {\n", keys, key)
	keys = append(keys, key)
	schema := m.FindSchema(m.dataroot)
	err := ValDelete(schema, m.dataroot, keys)
	if err == nil {
		if onchangecb, ok := m.Callback.(ChangeNotification); ok {
			onchangecb.ChangeDeleted(keys)
		}
	} else {
		glog.Errorf("%v", err)
	}
	// dump.Print(m.dataroot)
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
		onchangecb.ChangeStarted(m.updatedroot)
	}
}

// UpdateEnd - indicates the end of the model instance update
func (m *Model) UpdateEnd() {
	if onchangecb, ok := m.Callback.(ChangeNotification); ok {
		onchangecb.ChangeFinished(m.updatedroot)
	}
	m.updatedroot = nil
}
