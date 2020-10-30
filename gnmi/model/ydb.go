// YDB Go Interface for model data update

package model

import (
	"reflect"
	"strings"

	"github.com/golang/glog"
	"github.com/neoul/gnxi/utilities/xpath"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ygot/ygot"
	"github.com/openconfig/ygot/ytypes"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ValWrite - Write the value to the model instance
func (m *Model) ValWrite(base ygot.GoStruct, keys []string, value string) error {
	schema := m.FindSchema(base)
	if schema == nil {
		return status.Errorf(codes.Internal, "schema (%T) not found", base)
	}
	path, err := xpath.SlicePathToGNMIPath(keys)
	if err != nil {
		return err
	}
	target, tSchema, err := ytypes.GetOrCreateNode(schema, base, path)
	if err != nil {
		return err
	}
	if !tSchema.IsDir() {
		// v := reflect.ValueOf(target)
		vt := reflect.TypeOf(target)
		if vt.Kind() == reflect.Ptr {
			vt = vt.Elem()
		}
		vv, err := ytypes.StringToType(vt, value)
		if err != nil {
			return err
		}
		typedValue, err := ygot.EncodeTypedValue(vv.Interface(), gnmipb.Encoding_JSON_IETF)
		if err != nil {
			return status.Errorf(codes.Internal, "encoding error(%s)", err.Error())
		}
		err = ytypes.SetNode(schema, base, path, typedValue, &ytypes.InitMissingElements{})
		if err != nil {
			return err
		}
	}
	return err
}

// ValDelete - Delete the value from the model instance
func (m *Model) ValDelete(base ygot.GoStruct, keys []string) error {
	schema := m.FindSchema(base)
	if schema == nil {
		return status.Errorf(codes.Internal, "schema (%T) not found", base)
	}
	path, err := xpath.SlicePathToGNMIPath(keys)
	if err != nil {
		return err
	}
	tSchema := m.FindSchemaByGNMIPath(path)
	if tSchema == nil {
		return status.Errorf(codes.Internal, "schema (%s) not found", xpath.ToXPath(path))
	}
	// The key field deletion is not allowed.
	if pSchema := tSchema.Parent; pSchema != nil {
		if strings.Contains(pSchema.Key, keys[len(keys)-1]) {
			return nil
		}
	}
	return ytypes.DeleteNode(schema, base, path)
}

// Create - constructs the model instance
func (m *Model) Create(keys []string, key string, tag string, value string) error {
	// fmt.Printf("m.Create %v %v %v %v {\n", keys, key, tag, value)
	keys = append(keys, key)
	err := m.ValWrite(m.dataroot, keys, value)
	if err == nil {
		m.ValWrite(m.updatedroot, keys, value)
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
	err := m.ValWrite(m.dataroot, keys, value)
	if err == nil {
		m.ValWrite(m.updatedroot, keys, value)
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
	err := m.ValDelete(m.dataroot, keys)
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
