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
func (mdata *ModelData) Create(keys []string, key string, tag string, value string) error {
	// fmt.Printf("ModelData.Create %v %v %v %v {\n", keys, key, tag, value)
	err := dataMerge(mdata.dataroot, keys, key, tag, value)
	if err == nil {
		dataMerge(mdata.updatedroot, keys, key, tag, value)
		if mdata.callback != nil {
			path := append(keys, key)
			onchangecb, ok := mdata.callback.(OnChangeCallback)
			if ok {
				onchangecb.OnChangeCreated(path, mdata.updatedroot)
			}
		}
	}
	// fmt.Println("}", err)
	return err
}

// Replace - constructs ModelData
func (mdata *ModelData) Replace(keys []string, key string, tag string, value string) error {
	// fmt.Printf("ModelData.Replace %v %v %v %v {\n", keys, key, tag, value)
	err := dataMerge(mdata.dataroot, keys, key, tag, value)
	if err == nil {
		dataMerge(mdata.updatedroot, keys, key, tag, value)
		if mdata.callback != nil {
			path := append(keys, key)
			onchangecb, ok := mdata.callback.(OnChangeCallback)
			if ok {
				onchangecb.OnChangeReplaced(path, mdata.updatedroot)
			}
		}
	}
	// fmt.Println("}", err)
	return err
}

// Delete - constructs ModelData
func (mdata *ModelData) Delete(keys []string, key string) error {
	// fmt.Printf("ModelData.Delete %v %v {\n", keys, key)
	err := dataDelete(mdata.dataroot, keys, key)
	if err == nil {
		if mdata.callback != nil {
			path := append(keys, key)
			onchangecb, ok := mdata.callback.(OnChangeCallback)
			if ok {
				onchangecb.OnChangeDeleted(path)
			}
		}
	}
	// fmt.Println("}", err)
	return err
}

// UpdateStart - indicates the start of ModelData construction
func (mdata *ModelData) UpdateStart() {
	// updatedroot is used to save the changes of the model data.
	updatedroot, err := NewGoStruct(mdata.model, nil)
	if err != nil {
		return
	}
	mdata.updatedroot = updatedroot
	if mdata.callback != nil {
		onchangecb, ok := mdata.callback.(OnChangeCallback)
		if ok {
			onchangecb.OnChangeStarted(mdata.updatedroot)
		}
	}
}

// UpdateEnd - indicates the end of ModelData construction
func (mdata *ModelData) UpdateEnd() {
	if mdata.callback != nil {
		onchangecb, ok := mdata.callback.(OnChangeCallback)
		if ok {
			onchangecb.OnChangeFinished(mdata.updatedroot)
		}
	}
	mdata.updatedroot = nil
}
