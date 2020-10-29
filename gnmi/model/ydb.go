// YDB Go Interface for model data update

package model

import (
	"fmt"
	"reflect"

	"github.com/neoul/libydb/go/ydb"
	"github.com/openconfig/ygot/ygot"
	"github.com/openconfig/ygot/ytypes"
)

// dataMerge - constructs ModelData
func dataMerge(root ygot.GoStruct, keys []string, key string, tag string, value string) error {
	v := reflect.ValueOf(root)
	for _, k := range keys {
		cv, ok := ydb.ValFindOrInit(v, k, ydb.SearchByContent)
		if !ok || !cv.IsValid() {
			return fmt.Errorf("key %s not found", k)
		}
		v = cv
	}
	ct, ok := ydb.TypeFind(v.Type(), key)
	if ok && value != "" {
		cv, err := ytypes.StringToType(ct, value)
		if err == nil {
			_, err = ydb.ValChildDirectSet(v, key, cv)
			return err
		}
	}
	_, err := ydb.ValChildSet(v, key, value, ydb.SearchByContent)
	return err
}

// dataDelete - constructs ModelData
func dataDelete(root ygot.GoStruct, keys []string, key string) error {
	v := reflect.ValueOf(root)
	for _, k := range keys {
		cv, ok := ydb.ValFind(v, k, ydb.SearchByContent)
		if !ok || !cv.IsValid() {
			return fmt.Errorf("key %s not found", k)
		}
		v = cv
	}
	_, err := ydb.ValChildUnset(v, key, ydb.SearchByContent)
	return err
}

// Create - constructs ModelData
func (m *Model) Create(keys []string, key string, tag string, value string) error {
	// fmt.Printf("ModelData.Create %v %v %v %v {\n", keys, key, tag, value)
	err := dataMerge(m.dataroot, keys, key, tag, value)
	if err == nil {
		dataMerge(m.updatedroot, keys, key, tag, value)
		if m.Callback != nil {
			path := append(keys, key)
			onchangecb, ok := m.Callback.(ChangeNotification)
			if ok {
				onchangecb.ChangeCreated(path, m.updatedroot)
			}
		}
	}
	// fmt.Println("}", err)
	return err
}

// Replace - constructs ModelData
func (m *Model) Replace(keys []string, key string, tag string, value string) error {
	// fmt.Printf("ModelData.Replace %v %v %v %v {\n", keys, key, tag, value)
	err := dataMerge(m.dataroot, keys, key, tag, value)
	if err == nil {
		dataMerge(m.updatedroot, keys, key, tag, value)
		if onchangecb, ok := m.Callback.(ChangeNotification); ok {
			path := append(keys, key)
			onchangecb.ChangeReplaced(path, m.updatedroot)
		}
	}
	// fmt.Println("}", err)
	return err
}

// Delete - constructs ModelData
func (m *Model) Delete(keys []string, key string) error {
	// fmt.Printf("ModelData.Delete %v %v {\n", keys, key)
	err := dataDelete(m.dataroot, keys, key)
	if err == nil {
		if onchangecb, ok := m.Callback.(ChangeNotification); ok {
			path := append(keys, key)
			onchangecb.ChangeDeleted(path)
		}
	}
	// fmt.Println("}", err)
	return err
}

// UpdateStart - indicates the start of ModelData construction
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

// UpdateEnd - indicates the end of ModelData construction
func (m *Model) UpdateEnd() {
	if onchangecb, ok := m.Callback.(ChangeNotification); ok {
		onchangecb.ChangeFinished(m.updatedroot)
	}
	m.updatedroot = nil
}
