package gostruct

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/neoul/gnxi/utils"
	"github.com/neoul/libydb/go/ydb"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/goyang/pkg/yang"
	"github.com/openconfig/ygot/ygot"
)

// DataAndPath - used to retrieve of the data and gNMI path from ygot go structure.
type DataAndPath struct {
	Value interface{}
	Path  string
}

// FindAllData - finds all nodes matched to the gNMI Path.
func FindAllData(gs ygot.GoStruct, path *gpb.Path) ([]*DataAndPath, bool) {
	elems := path.GetElem()
	if len(elems) <= 0 {
		dataAndGNMIPath := &DataAndPath{
			Value: gs, Path: "/",
		}
		return []*DataAndPath{dataAndGNMIPath}, true
	}
	v := reflect.ValueOf(gs)
	datapath := &dataAndPath{Value: v, Key: []string{""}}
	founds := findAllData(datapath, elems)
	// fmt.Println(founds)
	num := len(founds)
	if num <= 0 {
		return []*DataAndPath{}, false
	}
	i := 0
	rvalues := make([]*DataAndPath, num)
	for _, each := range founds {
		if each.Value.CanInterface() {
			dataAndGNMIPath := &DataAndPath{
				Value: each.Value.Interface(),
				Path:  strings.Join(each.Key, "/"),
			}
			rvalues[i] = dataAndGNMIPath
			i++
		}
	}
	if i > 0 {
		return rvalues[:i], true
	}
	return []*DataAndPath{}, false
}

// dataAndPath - used to retrieve of the data and path from ygot go structure internally.
type dataAndPath struct {
	Value reflect.Value
	Key   []string
}

// findYangEntry - find the yang.Entry for schema info.
func findYangEntry(v reflect.Value) *yang.Entry {
	if !v.IsValid() {
		return nil
	}
	for v.Kind() == reflect.Ptr {
		v = v.Elem()
		if !v.IsValid() {
			return nil
		}
	}
	return SchemaTree[v.Type().Name()]
}

func appendCopy(curkey []string, addkey string) []string {
	length := len(curkey)
	dest := make([]string, length+1)
	copy(dest, curkey)
	dest[length] = addkey
	return dest
}

func (dpath *dataAndPath) getAllChildren() ([]*dataAndPath, bool) {
	if !dpath.Value.IsValid() {
		return []*dataAndPath{}, false
	}
	switch dpath.Value.Kind() {
	case reflect.Ptr, reflect.Interface:
		// child := &dataAndPath{Value: dpath.Value.Elem(), Key: dpath.Key}
		dpath.Value = dpath.Value.Elem()
		return dpath.getAllChildren()
	case reflect.Struct:
		length := dpath.Value.NumField()
		more := []*dataAndPath{}
		rval := make([]*dataAndPath, length)
		rlen := 0
		for i := 0; i < length; i++ {
			sft := dpath.Value.Type().Field(i)
			fv := dpath.Value.Field(i)
			if !fv.IsValid() {
				continue
			}
			key := ""
			if ydb.EnableTagLookup {
				if tag, ok := sft.Tag.Lookup(ydb.TagLookupKey); ok {
					key = tag
				}
			}
			if key == "" {
				key = sft.Name
			}
			if ydb.IsValMap(fv) { // find more values
				child := &dataAndPath{Value: fv, Key: dpath.Key}
				_rval, _ok := child.getAllChildren()
				if _ok {
					more = append(more, _rval...)
				}
			} else {

				rval[rlen] = &dataAndPath{Value: fv, Key: appendCopy(dpath.Key, key)}
				rlen++
			}
		}
		ok := false
		if rlen > 0 || len(more) > 0 {
			ok = true
		}
		return append(rval[:rlen], more...), ok
	case reflect.Map:
		rval := make([]*dataAndPath, dpath.Value.Len())
		iter := dpath.Value.MapRange()
		i := 0
		var schemaEntry *yang.Entry
		for iter.Next() {
			// [FIXME] - Need to check another way to get the key of a list.
			// ev := iter.Value()
			// if !ev.CanInterface() {
			// 	continue
			// }
			// keydata, ok := ev.Interface().(ygot.KeyHelperGoStruct)
			// if !ok {
			// 	continue
			// }
			if i == 0 {
				schemaEntry = findYangEntry(iter.Value())
				if schemaEntry == nil {
					return []*dataAndPath{}, false
				}
			}
			key, err := ydb.StrKeyGen(iter.Key(), schemaEntry.Name, schemaEntry.Key)
			if err != nil {
				return []*dataAndPath{}, false
			}
			rval[i] = &dataAndPath{Value: iter.Value(), Key: appendCopy(dpath.Key, key)}
			i++
		}
		ok := false
		if i > 0 {
			ok = true
		}
		return rval, ok
	case reflect.Slice:
		length := dpath.Value.Len()
		rval := make([]*dataAndPath, length)
		for i := 0; i < length; i++ {
			rval[i] = &dataAndPath{Value: dpath.Value.Index(i), Key: dpath.Key}
		}
		ok := false
		if len(rval) > 0 {
			ok = true
		}
		return rval, ok
	default:
		return []*dataAndPath{}, false
	}
}

func (dpath *dataAndPath) findChildren(key string) ([]*dataAndPath, bool) {
	cv, ok := ydb.ValFind(dpath.Value, key, ydb.SearchByContent)
	if !ok || !cv.IsValid() {
		return []*dataAndPath{}, false
	}
	if ydb.IsValMap(cv) {
		dpath.Value = cv
		// fmt.Println("findChild MAP:", dpath.Key)
		return dpath.getAllChildren()
	}
	child := &dataAndPath{Value: cv, Key: appendCopy(dpath.Key, key)}
	// fmt.Println("findChild :", child.Key)
	return []*dataAndPath{child}, true
}

// findAllData - finds all nodes matched to the gNMI Path.
func findAllData(dpath *dataAndPath, elems []*gpb.PathElem) []*dataAndPath {
	if len(elems) <= 0 {
		return []*dataAndPath{dpath}
	}
	if ydb.IsValScalar(dpath.Value) {
		return []*dataAndPath{}
	}
	elem := elems[0]
	fmt.Println("** Search", elem.GetName(), "from", ydb.DebugValueStringInline(dpath.Value.Interface(), 0, nil))
	if elem.GetName() == "*" {
		rpath := []*dataAndPath{}
		childlist, ok := dpath.getAllChildren()
		if ok {
			celems := elems[1:]
			for _, child := range childlist {
				rpath = append(rpath, findAllData(child, celems)...)
			}
		}
		return rpath
	} else if elem.GetName() == "..." {
		rpath := []*dataAndPath{}
		childlist, ok := dpath.getAllChildren()

		if ok {
			celems := elems[1:]
			for _, child := range childlist {
				rpath = append(rpath, findAllData(child, celems)...)
			}
			for _, child := range childlist {
				rpath = append(rpath, findAllData(child, elems)...)
			}
		}
		// for _, r := range rpath {
		// 	fmt.Println("... found:", r.Key, ydb.DebugValueStringInline(r.Value.Interface(), 1, nil))
		// }
		return rpath
	}

	ke := []string{elem.GetName()}
	for k, kv := range elem.GetKey() {
		if kv == "*" {
			ke = []string{elem.GetName()}
			break
		}
		ke = append(ke, fmt.Sprintf("[%s=%s]", k, kv))
	}
	key := strings.Join(ke, "")
	rpath := []*dataAndPath{}
	childlist, ok := dpath.findChildren(key)
	if ok {
		celems := elems[1:]
		for _, child := range childlist {
			rpath = append(rpath, findAllData(child, celems)...)
		}
	}
	return rpath
}

// FindAllDataNodes - finds all nodes matched to the gNMI Path.
func FindAllDataNodes(gs ygot.GoStruct, path *gpb.Path) ([]interface{}, bool) {
	elems := path.GetElem()
	if len(elems) <= 0 {
		return []interface{}{gs}, true
	}
	v := reflect.ValueOf(gs)
	rvlist := findAllDataNodes(v, elems)
	// fmt.Println(rvlist)
	num := len(rvlist)
	if num <= 0 {
		return []interface{}{}, false
	}
	rvalues := []interface{}{}
	for _, rv := range rvlist {
		if rv.CanInterface() {
			rvalues = append(rvalues, rv.Interface())
		}
	}
	if len(rvalues) > 0 {
		return rvalues, true
	}
	return rvalues, false
}

// findAllDataNodes - finds all nodes matched to the gNMI Path.
func findAllDataNodes(v reflect.Value, elems []*gpb.PathElem) []reflect.Value {
	// select all child nodes if the current node is a list.
	if v.Kind() == reflect.Map {
		rv := []reflect.Value{}
		cvlist, ok := ydb.ValGetAll(v)
		if ok {
			for _, cv := range cvlist {
				rv = append(rv, findAllDataNodes(cv, elems)...)
			}
		}
		return rv
	}
	if len(elems) <= 0 {
		return []reflect.Value{v}
	}
	if ydb.IsValScalar(v) {
		return []reflect.Value{}
	}
	elem := elems[0]
	fmt.Println("** Search", elem.GetName(), "from", utils.SprintStructInline(v.Interface()))
	if elem.GetName() == "*" {
		rv := []reflect.Value{}
		cvlist, ok := ydb.ValGetAll(v)
		if ok {
			celems := elems[1:]
			for _, cv := range cvlist {
				rv = append(rv, findAllDataNodes(cv, celems)...)
			}
		}
		return rv
	} else if elem.GetName() == "..." {
		rv := []reflect.Value{}
		cvlist, ok := ydb.ValGetAll(v)
		if ok {
			celems := elems[1:]
			for _, cv := range cvlist {
				ccvlist := findAllDataNodes(cv, celems)
				if len(ccvlist) > 0 {
					rv = append(rv, ccvlist...)
				}
				rv = append(rv, findAllDataNodes(cv, elems)...)
			}
		}
		return rv
	}

	ke := []string{elem.GetName()}
	for k, kv := range elem.GetKey() {
		if kv == "*" {
			ke = []string{elem.GetName()}
			break
		}
		ke = append(ke, fmt.Sprintf("[%s=%s]", k, kv))
	}
	key := strings.Join(ke, "")
	cv, ok := ydb.ValFind(v, key, ydb.SearchByContent)
	if !ok || !cv.IsValid() {
		return []reflect.Value{}
	}
	v = cv
	return findAllDataNodes(v, elems[1:])
}

// FindAllSchemaTypes - finds all schema nodes matched to the gNMI Path.
func FindAllSchemaTypes(gs ygot.GoStruct, path *gpb.Path) ([]reflect.Type, bool) {
	elems := path.GetElem()
	t := reflect.TypeOf(gs)
	if len(elems) <= 0 {
		return []reflect.Type{t}, true
	}

	rtlist := findAllSchemaTypes(t, elems)
	num := len(rtlist)
	if num <= 0 {
		return []reflect.Type{}, false
	}
	return rtlist, true
}

// findAllSchemaTypes - finds all schema nodes matched to the gNMI Path.
func findAllSchemaTypes(t reflect.Type, elems []*gpb.PathElem) []reflect.Type {
	// select all child nodes if the current node is a list.
	fmt.Println(t)
	if t.Kind() == reflect.Map {
		rv := []reflect.Type{}
		ctlist, ok := ydb.TypeGetAll(t)
		if ok {
			for _, ct := range ctlist {
				rv = append(rv, findAllSchemaTypes(ct, elems)...)
			}
		}
		return rv
	}
	if len(elems) <= 0 {
		return []reflect.Type{t}
	}
	if ydb.IsTypeScalar(t) {
		return []reflect.Type{}
	}
	elem := elems[0]
	fmt.Println("** Search", elem.GetName(), "from", t)
	if elem.GetName() == "*" {
		rv := []reflect.Type{}
		ctlist, ok := ydb.TypeGetAll(t)
		if ok {
			celems := elems[1:]
			for _, ct := range ctlist {
				rv = append(rv, findAllSchemaTypes(ct, celems)...)
			}
		}
		return rv
	} else if elem.GetName() == "..." {
		rv := []reflect.Type{}
		ctlist, ok := ydb.TypeGetAll(t)
		if ok {
			celems := elems[1:]
			for _, ct := range ctlist {
				cctlist := findAllSchemaTypes(ct, celems)
				if len(cctlist) > 0 {
					rv = append(rv, cctlist...)
				}
				rv = append(rv, findAllSchemaTypes(ct, elems)...)
			}
		}
		return rv
	}

	key := elem.GetName()
	ct, ok := ydb.TypeFind(t, key)
	if !ok {
		return []reflect.Type{}
	}
	t = ct
	return findAllSchemaTypes(t, elems[1:])
}

// FindAllSchemaPaths - finds all schema nodes matched to the gNMI Path.
func FindAllSchemaPaths(gs ygot.GoStruct, path *gpb.Path) ([]string, bool) {
	elems := path.GetElem()
	if len(elems) <= 0 {
		return []string{"/"}, true
	}
	p := ""
	sp := schemaPathElem{
		t:    reflect.TypeOf(gs),
		path: &p,
	}

	rsplist := findAllSchemaPaths(sp, elems)
	num := len(rsplist)
	if num <= 0 {
		return []string{}, false
	}
	rlist := make([]string, num)
	for i := 0; i < num; i++ {
		rlist[i] = *rsplist[i].path
	}
	return rlist, true
}

type schemaPathElem struct {
	t    reflect.Type
	path *string
}

// findAllSchemaPaths - finds all schema nodes' path matched to the gNMI Path.
// It is used to find all schema nodes matched to a wildcard path.
func findAllSchemaPaths(sp schemaPathElem, elems []*gpb.PathElem) []schemaPathElem {
	// select all child nodes if the current node is a list.
	if sp.t.Kind() == reflect.Map {
		rv := []schemaPathElem{}
		csplist, ok := getAllSchemaPaths(sp)
		if ok {
			for _, csp := range csplist {
				rv = append(rv, findAllSchemaPaths(csp, elems)...)
			}
		}
		return rv
	}
	if len(elems) <= 0 {
		return []schemaPathElem{sp}
	}
	if ydb.IsTypeScalar(sp.t) {
		return []schemaPathElem{}
	}
	elem := elems[0]
	fmt.Println("** Search", elem.GetName(), "from", sp.t)
	if elem.GetName() == "*" {
		rv := []schemaPathElem{}
		csplist, ok := getAllSchemaPaths(sp)
		if ok {
			celems := elems[1:]
			for _, csp := range csplist {
				rv = append(rv, findAllSchemaPaths(csp, celems)...)
			}
		}
		return rv
	} else if elem.GetName() == "..." {
		rv := []schemaPathElem{}
		csplist, ok := getAllSchemaPaths(sp)
		if ok {
			celems := elems[1:]
			for _, csp := range csplist {
				ccsplist := findAllSchemaPaths(csp, celems)
				if len(ccsplist) > 0 {
					rv = append(rv, ccsplist...)
				}
				rv = append(rv, findAllSchemaPaths(csp, elems)...)
			}
		}
		return rv
	}

	key := elem.GetName()
	csp, ok := findSchemaPath(sp, key)
	if !ok {
		return []schemaPathElem{}
	}
	sp = csp
	return findAllSchemaPaths(sp, elems[1:])
}

// TypeGetAll - Get All child values from the struct, map or slice value
func getAllSchemaPaths(sp schemaPathElem) ([]schemaPathElem, bool) {
	nt := reflect.TypeOf(nil)
	if sp.t == nt {
		return []schemaPathElem{}, false
	}

	switch sp.t.Kind() {
	case reflect.Interface:
		// Interface type doesn't have any dedicated type assigned!!.
		return []schemaPathElem{}, false
	case reflect.Ptr:
		sp.t = sp.t.Elem()
		return getAllSchemaPaths(sp)
	case reflect.Struct:
		length := sp.t.NumField()
		rtype := make([]schemaPathElem, 0)
		for i := 0; i < length; i++ {
			sft := sp.t.Field(i)
			st := sft.Type
			if st != nt && ydb.IsStartedWithUpper(sft.Name) {
				p, ok := sft.Tag.Lookup("path")
				if ok {
					np := *sp.path + "/" + p
					rtype = append(rtype, schemaPathElem{t: st, path: &np})
				}
			}
		}
		return rtype, true
	case reflect.Map, reflect.Slice:
		sp.t = sp.t.Elem()
		return []schemaPathElem{sp}, true
	default:
		return []schemaPathElem{}, false
	}
}

// findSchemaPath - finds a child type from the struct, map or slice value using the key.
func findSchemaPath(sp schemaPathElem, key string) (schemaPathElem, bool) {
	if sp.t == reflect.TypeOf(nil) {
		return sp, false
	}
	if key == "" {
		return sp, true
	}

	switch sp.t.Kind() {
	case reflect.Interface:
		return sp, false
	case reflect.Ptr:
		// nsp := &schemaPathElem{t: sp.t.Elem(), path: sp.path}
		// return findSchemaPath(nsp, key)
		sp.t = sp.t.Elem()
		return findSchemaPath(sp, key)
	case reflect.Struct:
		// ft, ok := sp.t.FieldByName(key)
		// if ok {
		// 	p, ok := ft.Tag.Lookup("path")
		// 	if !ok {
		// 		return sp, false
		// 	}
		// 	np := *sp.path + p
		// 	return schemaPathElem{t: ft.Type, path: &np}, false
		// }
		for i := 0; i < sp.t.NumField(); i++ {
			ft := sp.t.Field(i)
			if n, ok := ft.Tag.Lookup("path"); ok && n == key {
				np := *sp.path + "/" + n
				return schemaPathElem{t: ft.Type, path: &np}, true
			}
		}
	case reflect.Map, reflect.Slice, reflect.Array:
		sp.t = sp.t.Elem()
		return sp, true
	}
	return sp, false
}
