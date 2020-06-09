package gostruct

import (
	"fmt"
	"os"
	"reflect"

	"github.com/neoul/libydb/go/ydb"
	"github.com/op/go-logging"
	"github.com/openconfig/ygot/ytypes"
)

// Generate m6424 model
//go:generate sh -c "cd $GOPATH/src && go run github.com/openconfig/ygot/generator/generator.go -generate_fakeroot -output_file github.com/neoul/gnxi/gnmi/modeldata/gostruct/generated.go -package_name gostruct -exclude_modules ietf-interfaces -path github.com/neoul/gnxi/gnmi/modeldata/yang github.com/neoul/gnxi/gnmi/modeldata/yang/hfr-oc-dev.yang github.com/neoul/gnxi/gnmi/modeldata/yang/hfr-oc-roe.yang github.com/neoul/gnxi/gnmi/modeldata/yang/openconfig-interfaces.yang github.com/neoul/gnxi/gnmi/modeldata/yang/hfr-oc-tsn.yang github.com/neoul/gnxi/gnmi/modeldata/yang/openconfig-messages.yang"

var log *logging.Logger

func init() {
	log = ydb.SetLog("gostruct", os.Stderr, logging.DEBUG, "%{message}")
}

// YDB Go Interface

// Merge - constructs Device
func Merge(device *Device, keys []string, key string, tag string, value string) error {
	var pkey string
	var pv, cv reflect.Value
	var err error
	var ok bool

	v := reflect.ValueOf(device)
	cv = v
	if len(keys) > 0 {
		pv, cv, pkey, ok = ydb.FindValueWithParent(cv, cv, keys...)
		if !cv.IsValid() {
			return fmt.Errorf("Invalid parent value")
		}
		if !ok {
			return fmt.Errorf("Parent value not found: %s", pkey)
		}
	}

	cv = ydb.GetNonInterfaceValueDeep(cv)
	if ydb.IsYamlSeq(tag) {
		nv := reflect.ValueOf([]interface{}{})
		cv, err = ydb.SetValueChild(cv, nv, key)
		if err != nil {
			return err
		}
	} else {
		var values []interface{}
		// Try to update value via ygot
		v = ydb.FindValue(cv, key)
		if v.IsValid() && value != "" {
			v, err = ytypes.StringToType(v.Type(), value)
			if err == nil {
				if cv.Kind() == reflect.Ptr {
					cv = cv.Elem()
				}
				log.Debugf("%T %s", v.Interface(), v)
				cv, err = ydb.SetValueChild(cv, v, key)
				return err
			}
		}

		switch tag {
		case "!!map", "!!imap", "!!omap":
			values = []interface{}{key, value}
		case "!!set":
			values = []interface{}{key}
		case "!!seq":
			values = []interface{}{value}
		default: // other scalar types:
			if ydb.IsValueSlice(cv) {
				if key == "" {
					values = []interface{}{value}
				} else {
					values = []interface{}{key}
				}
			} else {
				values = []interface{}{key, value}
			}
		}

		nv := ydb.SetValue(cv, values...)
		if nv.Kind() == reflect.Slice {
			pv, err = ydb.SetValueChild(pv, nv, pkey)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// Create - constructs Device
func (device *Device) Create(keys []string, key string, tag string, value string) error {
	log.Debugf("Device.Create %v %v %v %v {\n", keys, key, tag, value)
	err := Merge(device, keys, key, tag, value)
	log.Debug("}", err, "\n")
	return err
}

// Replace - constructs Device
func (device *Device) Replace(keys []string, key string, tag string, value string) error {
	log.Debugf("Device.Replace %v %v %v %v {\n", keys, key, tag, value)
	err := Merge(device, keys, key, tag, value)
	log.Debug("}", err, "\n")
	return err
}

// Delete - constructs Device
func (device *Device) Delete(keys []string, key string) error {
	log.Debugf("Device.Delete %v %v {\n", keys, key)
	v := reflect.ValueOf(device)
	err := ydb.UnsetValueDeep(v, v, keys, key)
	log.Debug("}", err, "\n")
	return err
}
