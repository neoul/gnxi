package model

import (
	"reflect"

	"github.com/neoul/gnxi/utilities/xpath"
	"github.com/neoul/libydb/go/ydb"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/goyang/pkg/yang"
	"github.com/openconfig/ygot/ygot"
	"github.com/openconfig/ygot/ytypes"
)

// MO (ModelObject) for wrapping ygot.GoStruct and schema
type MO ytypes.Schema

// RootSchema returns the YANG entry schema corresponding to the type of the root within
// the schema.
func (mo *MO) RootSchema() *yang.Entry {
	return mo.SchemaTree[reflect.TypeOf(mo.Root).Elem().Name()]
}

// GetName returns the name of the model.
func (mo *MO) GetName() string {
	// schema := (*ytypes.Schema)(mo)
	// rootSchema := schema.RootSchema()
	rootSchema := mo.GetSchema()
	if rootSchema == nil {
		return "unknown"
	}
	return rootSchema.Name
}

// GetSchema - returns the root yang.Entry of the model.
func (mo *MO) GetSchema() *yang.Entry {
	return mo.RootSchema()
}

// GetRootType returns the reflect.Type of the root
func (mo *MO) GetRootType() reflect.Type {
	return reflect.TypeOf(mo.Root).Elem()
}

// GetRoot returns the Root
func (mo *MO) GetRoot() ygot.ValidatedGoStruct {
	return mo.Root
}

// FindSchemaByType - find the yang.Entry by Type for schema info.
func (mo *MO) FindSchemaByType(t reflect.Type) *yang.Entry {
	if t == reflect.TypeOf(nil) {
		return nil
	}
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
		if t == reflect.TypeOf(nil) {
			return nil
		}
	}
	return mo.SchemaTree[t.Name()]
}

// FindSchema finds the *yang.Entry by value(ygot.GoStruct) for schema info.
func (mo *MO) FindSchema(value interface{}) *yang.Entry {
	return mo.FindSchemaByType(reflect.TypeOf(value))
}

// FindSchemaByXPath finds *yang.Entry by XPath string from root model
func (mo *MO) FindSchemaByXPath(path string) *yang.Entry {
	return mo.FindSchemaByRelativeXPath(mo.Root, path)
}

// FindSchemaByRelativeXPath finds *yang.Entry by XPath string from base(ygot.GoStruct)
func (mo *MO) FindSchemaByRelativeXPath(base interface{}, path string) *yang.Entry {
	bSchema := mo.FindSchema(base)
	if bSchema == nil {
		return nil
	}
	findSchemaByXPath := func(entry *yang.Entry, path string) *yang.Entry {
		slicedPath, err := xpath.ParseStringPath(path)
		if err != nil {
			return nil
		}
		for _, elem := range slicedPath {
			switch v := elem.(type) {
			case string:
				entry = entry.Dir[v]
				if entry == nil {
					return nil
				}
			case map[string]string:
				// skip keys
			default:
				return nil
			}
		}
		return entry
	}
	return findSchemaByXPath(bSchema, path)
}

// FindSchemaByGNMIPath finds the schema(*yang.Entry) using the gNMI Path
func (mo *MO) FindSchemaByGNMIPath(path *gnmipb.Path) *yang.Entry {
	return mo.FindSchemaByRelativeGNMIPath(mo.Root, path)
}

// FindSchemaByRelativeGNMIPath finds the schema(*yang.Entry) using the relative path
func (mo *MO) FindSchemaByRelativeGNMIPath(base interface{}, path *gnmipb.Path) *yang.Entry {
	bSchema := mo.FindSchema(base)
	if bSchema == nil || path == nil {
		return nil
	}
	findSchema := func(entry *yang.Entry, path *gnmipb.Path) *yang.Entry {
		for _, e := range path.GetElem() {
			entry = entry.Dir[e.GetName()]
			if entry == nil {
				return nil
			}
		}
		return entry
	}
	return findSchema(bSchema, path)
}

// NewRoot returns new root
func (mo *MO) NewRoot(startup []byte) (*MO, error) {
	vgs := reflect.New(mo.GetRootType()).Interface()
	root := vgs.(ygot.ValidatedGoStruct)
	newMO := &MO{
		Root:       root,
		SchemaTree: mo.SchemaTree,
		Unmarshal:  mo.Unmarshal,
	}
	if startup != nil {
		if err := newMO.Unmarshal(startup, newMO.GetRoot()); err != nil {
			block, close := ydb.Open("NewRoot")
			defer close()
			if err := block.Parse(startup); err != nil {
				return nil, err
			}
			if _, err := block.Convert(ydb.RetrieveAll(), ydb.RetrieveStruct(newMO)); err != nil {
				return nil, err
			}
		}
		// [FIXME] - error in creating gnmid: /device/interfaces: /device/interfaces/interface: list interface contains more than max allowed elements: 2 > 0
		// if err := root.Validate(); err != nil {
		// 	return nil, err
		// }
	}

	return newMO, nil
}

// Create constructs the Model instance
func (mo *MO) Create(keys []string, key string, tag string, value string) error {
	keys = append(keys, key)
	return ValWrite(mo.RootSchema(), mo.Root, keys, value)
}

// Replace constructs the Model instance
func (mo *MO) Replace(keys []string, key string, tag string, value string) error {
	keys = append(keys, key)
	return ValWrite(mo.RootSchema(), mo.Root, keys, value)
}

// Delete constructs the Model instance
func (mo *MO) Delete(keys []string, key string) error {
	keys = append(keys, key)
	return ValDelete(mo.RootSchema(), mo.Root, keys)
}
