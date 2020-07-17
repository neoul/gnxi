// YDB Go Interface for model data update

package model

import (
	"fmt"
	"reflect"

	"github.com/neoul/libydb/go/ydb"
	"github.com/openconfig/ygot/ytypes"
)

// ydbMerge - constructs Device
func ydbMerge(mdata *ModelData, keys []string, key string, tag string, value string) error {
	// fmt.Printf("Device.merge %v %v %v %v\n", keys, key, tag, value)
	v := reflect.ValueOf(mdata.dataroot)
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
	if err == nil {
		// ydb.DebugValueString(nv.Interface(), 1, func(x ...interface{}) { fmt.Print(x...) })
	} else {
		fmt.Println(err)
	}
	return nil
}

// ydbDelete - constructs Device
func ydbDelete(mdata *ModelData, keys []string, key string) error {
	// fmt.Printf("Device.delete %v %v\n", keys, key)
	v := reflect.ValueOf(mdata.dataroot)
	for _, k := range keys {
		cv, ok := ydb.ValFind(v, k, ydb.SearchByContent)
		if !ok || !cv.IsValid() {
			return fmt.Errorf("key %s not found", k)
		}
		v = cv
	}
	_, err := ydb.ValChildUnset(v, key, ydb.SearchByContent)
	if err == nil {
		// ydb.DebugValueString(v.Interface(), 1, func(x ...interface{}) { fmt.Print(x...) })
	} else {
		fmt.Println(err)
	}
	return nil
}

// Create - constructs Device
func (mdata *ModelData) Create(keys []string, key string, tag string, value string) error {
	// fmt.Printf("Device.Create %v %v %v %v {\n", keys, key, tag, value)
	err := ydbMerge(mdata, keys, key, tag, value)
	// fmt.Println("}", err)
	return err
}

// Replace - constructs Device
func (mdata *ModelData) Replace(keys []string, key string, tag string, value string) error {
	// fmt.Printf("Device.Replace %v %v %v %v {\n", keys, key, tag, value)
	err := ydbMerge(mdata, keys, key, tag, value)
	// fmt.Println("}", err)
	return err
}

// Delete - constructs Device
func (mdata *ModelData) Delete(keys []string, key string) error {
	// fmt.Printf("Device.Delete %v %v {\n", keys, key)
	err := ydbDelete(mdata, keys, key)
	// fmt.Println("}", err)
	return err
}