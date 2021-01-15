package model

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/openconfig/gnmi/proto/gnmi"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/goyang/pkg/yang"
)

// SchemaXPath returns a schema xapth using *yang.Entry
func SchemaXPath(schema *yang.Entry) string {
	p := []string{}
	for ; schema != nil && schema.Parent != nil; schema = schema.Parent {
		p = append([]string{schema.Name}, p...)
	}
	return strings.Join(p, "/")
}

// SchemaGNMIPath returns a schema xapth using *yang.Entry
func SchemaGNMIPath(schema *yang.Entry) *gnmipb.Path {
	p := &gnmipb.Path{
		Elem: []*gnmipb.PathElem{},
	}
	for ; schema != nil && schema.Parent != nil; schema = schema.Parent {
		p.Elem = append([]*gnmi.PathElem{&gnmi.PathElem{Name: schema.Name}}, p.Elem...)
	}
	return p
}

// typeFind - finds a child type from the struct, map or slice value using the key.
func typeFind(pt reflect.Type, key *string) (reflect.Type, bool) {
	if pt == reflect.TypeOf(nil) {
		return pt, false
	}
	if key == nil || *key == "" {
		return pt, true
	}

	switch pt.Kind() {
	case reflect.Interface:
		return pt, false
	case reflect.Ptr:
		return typeFind(pt.Elem(), key)
	case reflect.Struct:
		for i := 0; i < pt.NumField(); i++ {
			ft := pt.Field(i)
			if n, ok := ft.Tag.Lookup("path"); ok && n == *key {
				return ft.Type, true
			}
		}
	case reflect.Map, reflect.Slice, reflect.Array:
		return typeFind(pt.Elem(), key)
	}
	return pt, false
}

func updateType(mo *MO, curSchema *yang.Entry, curType reflect.Type) error {
	if curSchema.Annotation == nil {
		curSchema.Annotation = map[string]interface{}{}
	}
	curSchema.Annotation["type"] = curType
	for name, schema := range curSchema.Dir {
		if t, ok := typeFind(curType, &name); ok {
			if err := updateType(mo, schema, t); err != nil {
				return err
			}
		} else {
			return fmt.Errorf("type update to schema tree failed for %s", SchemaXPath(schema))
		}
	}
	return nil
}

// UpdateType updates the schema for the fast type searching..
func (mo *MO) UpdateType() error {
	curSchema := mo.GetSchema()
	curType := mo.GetRootType()
	return updateType(mo, curSchema, curType)
}
