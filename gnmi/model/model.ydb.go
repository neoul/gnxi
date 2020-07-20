// YDB Go Interface for model data update

package model

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/neoul/libydb/go/ydb"
	"github.com/openconfig/ygot/ytypes"
)

// ydbMerge - constructs Device
func ydbMerge(op Operation, mdata *ModelData, keys []string, key string, tag string, value string) error {
	// fmt.Printf("Device.merge %v %v %v %v\n", keys, key, tag, value)
	v := reflect.ValueOf(mdata.dataroot)
	dataPathKeys := []string{""}
	schemaPathKeys := []string{""}
	for _, k := range keys {
		cv, ok := ydb.ValFindOrInit(v, k, ydb.SearchByContent)
		if !ok || !cv.IsValid() {
			return fmt.Errorf("key %s not found", k)
		}
		schemaEntry := mdata.model.FindSchemaData(cv)
		if schemaEntry == nil {
			return fmt.Errorf("no schema data (%s)", k)
		}
		dataPathKeys = append(dataPathKeys, k)
		schemaPathKeys = append(schemaPathKeys, schemaEntry.Name)
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
	rv, err := ydb.ValChildSet(v, key, value, ydb.SearchByContent)
	if err == nil {
		cv, ok := ydb.ValFindOrInit(rv, key, ydb.SearchByContent)
		if !ok || !cv.IsValid() {
			return fmt.Errorf("key %s not found", key)
		}
		if ydb.IsValScalar(cv) {
			schemaPathKeys = append(schemaPathKeys, key)
		} else {
			schemaEntry := mdata.model.FindSchemaData(cv)
			if schemaEntry == nil {
				return fmt.Errorf("no schema data (%s)", key)
			}
			schemaPathKeys = append(schemaPathKeys, schemaEntry.Name)
		}

		dataPathKeys = append(dataPathKeys, key)
		execOnChangeCallback(mdata.callback, op,
			strings.Join(schemaPathKeys, "/"), strings.Join(dataPathKeys, "/"), cv.Interface())
		// ydb.DebugValueString(nv.Interface(), 1, func(x ...interface{}) { fmt.Print(x...) })
	}
	return err
}

// ydbDelete - constructs Device
func ydbDelete(mdata *ModelData, keys []string, key string) error {
	// fmt.Printf("Device.delete %v %v\n", keys, key)
	v := reflect.ValueOf(mdata.dataroot)
	dataPathKeys := []string{""}
	schemaPathKeys := []string{""}
	for _, k := range keys {
		cv, ok := ydb.ValFind(v, k, ydb.SearchByContent)
		if !ok || !cv.IsValid() {
			return fmt.Errorf("key %s not found", k)
		}
		schemaEntry := mdata.model.FindSchemaData(cv)
		if schemaEntry == nil {
			return fmt.Errorf("no schema data (%s)", k)
		}
		dataPathKeys = append(dataPathKeys, k)
		schemaPathKeys = append(schemaPathKeys, schemaEntry.Name)
		v = cv
	}
	_, err := ydb.ValChildUnset(v, key, ydb.SearchByContent)
	if err == nil {
		if i := strings.Index(key, "["); i < 0 {
			schemaPathKeys = append(schemaPathKeys, key)
		} else {
			schemaPathKeys = append(schemaPathKeys, key[:i])
		}
		dataPath := "/" + strings.Join(keys, "/") + "/" + key
		execOnChangeCallback(mdata.callback, OpDelete, "", dataPath, nil)
		// ydb.DebugValueString(v.Interface(), 1, func(x ...interface{}) { fmt.Print(x...) })
	} else {
		fmt.Println(err)
	}
	return nil
}

// Create - constructs Device
func (mdata *ModelData) Create(keys []string, key string, tag string, value string) error {
	// fmt.Printf("Device.Create %v %v %v %v {\n", keys, key, tag, value)
	err := ydbMerge(OpCreate, mdata, keys, key, tag, value)
	// fmt.Println("}", err)
	return err
}

// Replace - constructs Device
func (mdata *ModelData) Replace(keys []string, key string, tag string, value string) error {
	// fmt.Printf("Device.Replace %v %v %v %v {\n", keys, key, tag, value)
	err := ydbMerge(OpReplace, mdata, keys, key, tag, value)
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
