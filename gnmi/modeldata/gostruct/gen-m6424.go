package gostruct

import (
	"os"
	"reflect"

	"github.com/neoul/libydb/go/ydb"
	"github.com/op/go-logging"
)

// Generate m6424 model
//go:generate sh -c "cd $GOPATH/src && go run github.com/openconfig/ygot/generator/generator.go -generate_fakeroot -output_file github.com/neoul/gnxi/gnmi/modeldata/gostruct/generated.go -package_name gostruct -exclude_modules ietf-interfaces -path github.com/neoul/gnxi/gnmi/modeldata/yang github.com/neoul/gnxi/gnmi/modeldata/yang/hfr-oc-dev.yang github.com/neoul/gnxi/gnmi/modeldata/yang/hfr-oc-roe.yang github.com/neoul/gnxi/gnmi/modeldata/yang/openconfig-interfaces.yang github.com/neoul/gnxi/gnmi/modeldata/yang/hfr-oc-tsn.yang github.com/neoul/gnxi/gnmi/modeldata/yang/openconfig-messages.yang"

var log *logging.Logger

func init() {
	// log = ydb.SetLog("ydb2ygot", os.Stderr, logging.DEBUG, "%{message}")
	log = ydb.SetLog("ydb2ygot", os.Stderr, logging.DEBUG, "")
}

// YDB Go Interface

// Create - constructs schema.Example
func (device *Device) Create(keys []string, key string, tag string, value string) error {
	log.Debugf("Device.Create %v %v %v %v {", keys, key, tag, value)
	v := reflect.ValueOf(device)
	err := ydb.SetInterfaceValue(v, v, keys, key, tag, value)
	log.Debugf("}")
	return err
}

// Replace - constructs schema.Example
func (device *Device) Replace(keys []string, key string, tag string, value string) error {
	log.Debugf("Device.Replace %v %v %v %v {", keys, key, tag, value)
	v := reflect.ValueOf(device)
	err := ydb.SetInterfaceValue(v, v, keys, key, tag, value)
	log.Debugf("}")
	return err
}

// Delete - constructs schema.Example
func (device *Device) Delete(keys []string, key string) error {
	log.Debugf("Device.Delete %v %v %v %v {", keys, key)
	v := reflect.ValueOf(device)
	err := ydb.UnsetInterfaceValue(v, v, keys, key)
	log.Debugf("}")
	return err
	// return nil
}
