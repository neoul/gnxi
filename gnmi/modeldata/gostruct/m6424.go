package gostruct

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/neoul/libydb/go/ydb"
	"github.com/openconfig/ygot/ytypes"
)

// Generate m6424 model
//go:generate sh -c "cd $GOPATH/src && go run github.com/openconfig/ygot/generator/generator.go -generate_fakeroot -output_file github.com/neoul/gnxi/gnmi/modeldata/gostruct/generated.go -package_name gostruct -exclude_modules ietf-interfaces -path github.com/neoul/gnxi/gnmi/modeldata/yang github.com/neoul/gnxi/gnmi/modeldata/yang/hfr-oc-dev.yang github.com/neoul/gnxi/gnmi/modeldata/yang/hfr-oc-roe.yang github.com/neoul/gnxi/gnmi/modeldata/yang/openconfig-interfaces.yang github.com/neoul/gnxi/gnmi/modeldata/yang/hfr-oc-tsn.yang github.com/neoul/gnxi/gnmi/modeldata/yang/openconfig-messages.yang"

// YDB Go Interface

func keyListing(keys []string, key string) ([]string, string) {
	var keylist []string
	if len(key) > 0 {
		for _, k := range keys {
			i := strings.Index(k, "[")
			if i > 0 {
				ename := k[:i]
				kname := strings.Trim(k[i:], "[]")
				i = strings.Index(kname, "=")
				if i > 0 {
					kname = kname[i+1:]
					keylist = append(keylist, ename)
					keylist = append(keylist, kname)
					continue
				}
			}
			keylist = append(keylist, k)
		}
	}
	i := strings.Index(key, "[")
	if i > 0 {
		ename := key[:i]
		kname := strings.Trim(key[i:], "[]")
		i = strings.Index(kname, "=")
		if i > 0 {
			kname = kname[i+1:]
			keylist = append(keylist, ename)
			key = kname
		}
	}
	return keylist, key
}

// Merge - constructs Device
func merge(device *Device, keys []string, key string, tag string, value string) error {
	keys, key = keyListing(keys, key)
	fmt.Printf("Device.merge %v %v %v %v\n", keys, key, tag, value)
	v := reflect.ValueOf(device)
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
			ydb.DebugValueString(cv.Interface(), 1, func(x ...interface{}) { fmt.Print(x...) })
			return err
		} else {
			fmt.Println(err)
		}
	}
	nv, err := ydb.ValChildSet(v, key, value, ydb.SearchByContent)
	if err == nil {
		// ydb.DebugValueString(v.Interface(), 1, func(x ...interface{}) { fmt.Print(x...) })
		ydb.DebugValueString(nv.Interface(), 1, func(x ...interface{}) { fmt.Print(x...) })
	} else {
		fmt.Println(err)
	}
	return nil
}

// Merge - constructs Device
func delete(device *Device, keys []string, key string) error {
	keys, key = keyListing(keys, key)
	fmt.Printf("Device.delete %v %v\n", keys, key)
	v := reflect.ValueOf(device)
	for _, k := range keys {
		cv, ok := ydb.ValFind(v, k, ydb.SearchByContent)
		if !ok || !cv.IsValid() {
			return fmt.Errorf("key %s not found", k)
		}
		v = cv
	}
	_, err := ydb.ValChildUnset(v, key, ydb.SearchByContent)
	if err == nil {
		ydb.DebugValueString(v.Interface(), 1, func(x ...interface{}) { fmt.Print(x...) })
	} else {
		fmt.Println(err)
	}
	return nil
}

// Create - constructs Device
func (device *Device) Create(keys []string, key string, tag string, value string) error {
	fmt.Printf("Device.Create %v %v %v %v {\n", keys, key, tag, value)
	err := merge(device, keys, key, tag, value)
	fmt.Println("}", err)
	return err
}

// Replace - constructs Device
func (device *Device) Replace(keys []string, key string, tag string, value string) error {
	fmt.Printf("Device.Replace %v %v %v %v {\n", keys, key, tag, value)
	err := merge(device, keys, key, tag, value)
	fmt.Println("}", err)
	return err
}

// Delete - constructs Device
func (device *Device) Delete(keys []string, key string) error {
	fmt.Printf("Device.Delete %v %v {\n", keys, key)
	err := delete(device, keys, key)
	fmt.Println("}", err)
	return err
}
