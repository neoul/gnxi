package model

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/neoul/gnxi/utilities/status"
	"github.com/neoul/gnxi/utilities/xpath"
	"github.com/neoul/libydb/go/ydb"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/goyang/pkg/yang"
	"github.com/openconfig/ygot/ygot"
	"github.com/openconfig/ygot/ytypes"
)

// DataAndPath - used to retrieve of the data and gNMI path from ygot go structure.
type DataAndPath struct {
	Value interface{}
	Path  string
}

// GetValueString returns DataAndPath string
func (dap *DataAndPath) GetValueString() string {
	if dap.Value == nil {
		return ""
	}
	t := reflect.TypeOf(dap.Value)
	if ydb.IsTypeScalar(t) {
		if dap.Value == nil {
			return ""
		}
		typedValue, err := ygot.EncodeTypedValue(dap.Value, gnmipb.Encoding_JSON_IETF)
		if err != nil || typedValue == nil || typedValue.Value == nil {
			return ""
		}
		v := reflect.ValueOf(typedValue.Value)
		if v.Kind() == reflect.Ptr {
			v = v.Elem()
		}
		if v.Kind() == reflect.Struct {
			return fmt.Sprintf("%v", v.Field(0))
		}
		return fmt.Sprintf("%v", v)
	}
	return ""
}

// String returns DataAndPath string
func (dap *DataAndPath) String() string {
	t := reflect.TypeOf(dap.Value)
	if ydb.IsTypeScalar(t) {
		if dap.Value == nil {
			return fmt.Sprintf("%s=", dap.Path)
		}
		typedValue, err := ygot.EncodeTypedValue(dap.Value, gnmipb.Encoding_JSON_IETF)
		if err != nil || typedValue == nil || typedValue.Value == nil {
			return fmt.Sprintf("%s=", dap.Path)
		}
		v := reflect.ValueOf(typedValue.Value)
		if v.Kind() == reflect.Ptr {
			v = v.Elem()
		}
		if v.Kind() == reflect.Struct {
			return fmt.Sprintf("%s=%v", dap.Path, v.Field(0))
		}
		return fmt.Sprintf("%s=%v", dap.Path, v)
	}
	return fmt.Sprintf("%s", dap.Path)
}

// FindAllData - finds all nodes matched to the gNMI Path.
func FindAllData(gs ygot.GoStruct, path *gnmipb.Path) ([]*DataAndPath, bool) {
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
	rvalues := make([]*DataAndPath, 0, num)
	for _, each := range founds {
		if each.Value.CanInterface() {
			dataAndGNMIPath := &DataAndPath{
				Value: each.Value.Interface(),
				Path:  strings.Join(each.Key, "/"),
			}
			rvalues = append(rvalues, dataAndGNMIPath)
		}
	}
	if len(rvalues) > 0 {
		return rvalues, true
	}
	return rvalues, false
}

// dataAndPath - used to retrieve of the data and path from ygot go structure internally.
type dataAndPath struct {
	Value reflect.Value
	Key   []string
}

func appendCopy(curkey []string, addkey string) []string {
	length := len(curkey)
	dest := make([]string, length+1)
	copy(dest, curkey)
	dest[length] = addkey
	return dest
}

func (dpath *dataAndPath) getAllChildren(prefix string, opt ...FindOption) ([]*dataAndPath, bool) {
	if !dpath.Value.IsValid() {
		return []*dataAndPath{}, false
	}
	switch dpath.Value.Kind() {
	case reflect.Ptr, reflect.Interface:
		// child := &dataAndPath{Value: dpath.Value.Elem(), Key: dpath.Key}
		dpath.Value = dpath.Value.Elem()
		return dpath.getAllChildren(prefix, opt...)
	case reflect.Struct:
		length := dpath.Value.NumField()
		more := []*dataAndPath{}
		rval := make([]*dataAndPath, 0, length)
		for i := 0; i < length; i++ {
			sft := dpath.Value.Type().Field(i)
			fv := dpath.Value.Field(i)
			if !fv.IsValid() {
				continue
			}
			key := prefix
			if tag, ok := sft.Tag.Lookup(ydb.TagLookupKey); ok {
				key = prefix + tag
			}
			if key == prefix {
				key = sft.Name
			}
			if ydb.IsValMap(fv) { // find more values
				child := &dataAndPath{Value: fv, Key: dpath.Key}
				_rval, _ok := child.getAllChildren(prefix+key, opt...)
				if _ok {
					more = append(more, _rval...)
				}
			} else {
				rval = append(rval, &dataAndPath{Value: fv, Key: appendCopy(dpath.Key, key)})
			}
		}
		ok := false
		if len(rval) > 0 || len(more) > 0 {
			ok = true
		}
		return append(rval, more...), ok
	case reflect.Map:
		rval := make([]*dataAndPath, 0, dpath.Value.Len())
		iter := dpath.Value.MapRange()
		for iter.Next() {
			// [FIXME] - Need to check another way to get the key of a list.
			ev := iter.Value()
			if !ev.CanInterface() {
				continue
			}
			listStruct, ok := ev.Interface().(ygot.KeyHelperGoStruct)
			if !ok {
				continue
			}
			keymap, err := listStruct.ΛListKeyMap()
			if err != nil {
				continue
			}
			key := prefix
			for kname, kvalue := range keymap {
				key = key + fmt.Sprintf("[%s=%v]", kname, kvalue)
			}
			// var schemaEntry *yang.Entry
			// if i == 0 {
			// 	schemaEntry = FindSchemaByType(iter.Value())
			// 	if schemaEntry == nil {
			// 		return []*dataAndPath{}, false
			// 	}
			// }
			// key, err := ydb.StrKeyGen(iter.Key(), schemaEntry.Name, schemaEntry.Key)
			// if err != nil {
			// 	continue
			// }
			rval = append(rval, &dataAndPath{Value: iter.Value(), Key: appendCopy(dpath.Key, key)})
		}
		ok := false
		if len(rval) > 0 {
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

func (dpath *dataAndPath) findChildren(key string, opt ...FindOption) ([]*dataAndPath, bool) {
	cv, ok := ydb.ValFind(dpath.Value, key, ydb.SearchByContent)
	if !ok || !cv.IsValid() {
		return []*dataAndPath{}, false
	}
	if ydb.IsValMap(cv) {
		dpath.Value = cv
		// fmt.Println("findChild MAP:", dpath.Key)
		return dpath.getAllChildren(key, opt...)
	}
	child := &dataAndPath{Value: cv, Key: appendCopy(dpath.Key, key)}
	// fmt.Println("findChild :", child.Key)
	return []*dataAndPath{child}, true
}

// findAllData - finds all nodes matched to the gNMI Path.
func findAllData(dpath *dataAndPath, elems []*gnmipb.PathElem, opt ...FindOption) []*dataAndPath {
	if len(elems) <= 0 {
		return []*dataAndPath{dpath}
	}
	if ydb.IsValScalar(dpath.Value) {
		return []*dataAndPath{}
	}

	elem := elems[0]
	// fmt.Println("** Search", elem.GetName(), "from", ydb.DebugValueStringInline(dpath.Value.Interface(), 0, nil))
	if elem.GetName() == "*" {
		rpath := []*dataAndPath{}
		childlist, ok := dpath.getAllChildren("")
		if ok {
			celems := elems[1:]
			for _, child := range childlist {
				rpath = append(rpath, findAllData(child, celems, opt...)...)
			}
		}
		return rpath
	} else if elem.GetName() == "..." {
		rpath := []*dataAndPath{}
		childlist, ok := dpath.getAllChildren("")
		if ok {
			celems := elems[1:]
			for _, child := range childlist {
				rpath = append(rpath, findAllData(child, celems, opt...)...)
			}
			for _, child := range childlist {
				rpath = append(rpath, findAllData(child, elems, opt...)...)
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
	childlist, ok := dpath.findChildren(key, opt...)
	if ok {
		celems := elems[1:]
		for _, child := range childlist {
			rpath = append(rpath, findAllData(child, celems, opt...)...)
		}
	}
	return rpath
}

// FindAllDataNodes - finds all nodes matched to the gNMI Path.
func FindAllDataNodes(gs ygot.GoStruct, path *gnmipb.Path) ([]interface{}, bool) {
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
func findAllDataNodes(v reflect.Value, elems []*gnmipb.PathElem) []reflect.Value {
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
func FindAllSchemaTypes(gs ygot.GoStruct, path *gnmipb.Path) ([]reflect.Type, bool) {
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
func findAllSchemaTypes(t reflect.Type, elems []*gnmipb.PathElem) []reflect.Type {
	// select all child nodes if the current node is a list.
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
	// fmt.Println("** Search", elem.GetName(), "from", t)
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
func FindAllSchemaPaths(gs ygot.GoStruct, path *gnmipb.Path) ([]string, bool) {
	elems := path.GetElem()
	if len(elems) <= 0 {
		return []string{"/"}, true
	}
	p := ""
	sp := pathFinder{
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

type pathFinder struct {
	t    reflect.Type
	path *string
}

// findAllSchemaPaths - finds all schema nodes' path matched to the gNMI Path.
// It is used to find all schema nodes matched to a wildcard path.
func findAllSchemaPaths(sp pathFinder, elems []*gnmipb.PathElem) []pathFinder {
	// select all child nodes if the current node is a list.
	if sp.t.Kind() == reflect.Map {
		rv := []pathFinder{}
		csplist, ok := getAllSchemaPaths(sp)
		if ok {
			for _, csp := range csplist {
				rv = append(rv, findAllSchemaPaths(csp, elems)...)
			}
		}
		return rv
	}
	if len(elems) <= 0 {
		return []pathFinder{sp}
	}
	if ydb.IsTypeScalar(sp.t) {
		return []pathFinder{}
	}
	elem := elems[0]
	// fmt.Println("** Search", elem.GetName(), "from", sp.t)
	if elem.GetName() == "*" {
		rv := []pathFinder{}
		csplist, ok := getAllSchemaPaths(sp)
		if ok {
			celems := elems[1:]
			for _, csp := range csplist {
				rv = append(rv, findAllSchemaPaths(csp, celems)...)
			}
		}
		return rv
	} else if elem.GetName() == "..." {
		rv := []pathFinder{}
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

	elemname := elem.GetName()
	csp, ok := findSchemaPath(sp, elemname)
	if !ok {
		return []pathFinder{}
	}
	sp = csp
	return findAllSchemaPaths(sp, elems[1:])
}

// TypeGetAll - Get All child values from the struct, map or slice value
func getAllSchemaPaths(sp pathFinder) ([]pathFinder, bool) {
	nt := reflect.TypeOf(nil)
	if sp.t == nt {
		return []pathFinder{}, false
	}

	switch sp.t.Kind() {
	case reflect.Interface:
		// Interface type doesn't have any dedicated type assigned!!.
		return []pathFinder{}, false
	case reflect.Ptr:
		sp.t = sp.t.Elem()
		return getAllSchemaPaths(sp)
	case reflect.Struct:
		found := false
		length := sp.t.NumField()
		rtype := make([]pathFinder, 0, length)
		for i := 0; i < length; i++ {
			sft := sp.t.Field(i)
			st := sft.Type
			if st != nt && ydb.IsStartedWithUpper(sft.Name) {
				p, ok := sft.Tag.Lookup("path")
				if ok {
					found = true
					np := *sp.path + "/" + p
					rtype = append(rtype, pathFinder{t: st, path: &np})
				}
			}
		}
		return rtype, found
	case reflect.Map, reflect.Slice:
		sp.t = sp.t.Elem()
		return []pathFinder{sp}, true
	default:
		return []pathFinder{}, false
	}
}

// findSchemaPath - finds a child type from the struct, map or slice value using the elemname.
func findSchemaPath(sp pathFinder, elemname string) (pathFinder, bool) {
	if sp.t == reflect.TypeOf(nil) {
		return sp, false
	}
	if elemname == "" {
		return sp, true
	}

	switch sp.t.Kind() {
	case reflect.Interface:
		return sp, false
	case reflect.Ptr:
		// nsp := &pathFinder{t: sp.t.Elem(), path: sp.path}
		// return findSchemaPath(nsp, elemname)
		sp.t = sp.t.Elem()
		return findSchemaPath(sp, elemname)
	case reflect.Struct:
		// ft, ok := sp.t.FieldByName(elemname)
		// if ok {
		// 	p, ok := ft.Tag.Lookup("path")
		// 	if !ok {
		// 		return sp, false
		// 	}
		// 	np := *sp.path + p
		// 	return pathFinder{t: ft.Type, path: &np}, false
		// }
		for i := 0; i < sp.t.NumField(); i++ {
			ft := sp.t.Field(i)
			if n, ok := ft.Tag.Lookup("path"); ok && n == elemname {
				np := *sp.path + "/" + n
				return pathFinder{t: ft.Type, path: &np}, true
			}
		}
	case reflect.Map, reflect.Slice, reflect.Array:
		sp.t = sp.t.Elem()
		return sp, true
	}
	return sp, false
}

// FindSchemaByGNMIPath finds the schema(*yang.Entry) using the relative path
func FindSchemaByGNMIPath(baseSchema *yang.Entry, base interface{}, path *gnmipb.Path) *yang.Entry {
	findSchema := func(entry *yang.Entry, path *gnmipb.Path) *yang.Entry {
		for _, e := range path.GetElem() {
			entry = entry.Dir[e.GetName()]
			if entry == nil {
				return nil
			}
		}
		return entry
	}
	return findSchema(baseSchema, path)
}

// writeTypedValue - Write the TypedValue to the model instance
func writeTypedValue(m *Model, path *gnmipb.Path, typedValue *gnmipb.TypedValue) error {
	// var err error
	schema := m.GetSchema()
	base := m.GetRoot()
	tValue, tSchema, err := ytypes.GetOrCreateNode(schema, base, path)
	if err != nil {
		return fmt.Errorf("%s", status.FromError(err).Message)
	}
	if tSchema.IsDir() {
		target := tValue.(ygot.GoStruct)
		// FIXME
		if err := m.Unmarshal(typedValue.GetJsonIetfVal(), target); err != nil {
			return err
		}
	} else { // (schema.IsLeaf() || schema.IsLeafList())
		err = ytypes.SetNode(schema, base, path, typedValue, &ytypes.InitMissingElements{})
		if err != nil {
			return fmt.Errorf("%s", status.FromError(err).Message)
		}
	}
	return nil
}

// writeValue - Write the value to the model instance
func writeValue(schema *yang.Entry, base ygot.GoStruct, path string, value string) error {
	gpath, err := xpath.ToGNMIPath(path)
	if err != nil {
		return err
	}
	target, tSchema, err := ytypes.GetOrCreateNode(schema, base, gpath)
	if err != nil {
		return fmt.Errorf("%s", status.FromError(err).Message)
	}
	if !tSchema.IsDir() {
		// v := reflect.ValueOf(target)
		// fmt.Println(strings.Join(keys, "/"))
		vt := reflect.TypeOf(target)
		if vt.Kind() == reflect.Ptr {
			vt = vt.Elem()
		}
		vv, err := ytypes.StringToType(vt, value)
		if err != nil {
			switch tSchema.Type.Kind {
			case yang.Yenum, yang.Yidentityref:
				return err
			}
			var yerr error
			vv, yerr = ydb.ValScalarNew(vt, value)
			if yerr != nil {
				return err
			}
		}
		typedValue, err := ygot.EncodeTypedValue(vv.Interface(), gnmipb.Encoding_JSON_IETF)
		if err != nil {
			return err
		}
		err = ytypes.SetNode(schema, base, gpath, typedValue, &ytypes.InitMissingElements{})
		if err != nil {
			return fmt.Errorf("%s", status.FromError(err).Message)
		}
	}
	return err
}

// deleteValue - Delete the value from the model instance
func deleteValue(schema *yang.Entry, base ygot.GoStruct, path string) error {
	gpath, err := xpath.ToGNMIPath(path)
	if err != nil {
		return err
	}
	tSchema := FindSchemaByGNMIPath(schema, base, gpath)
	if tSchema == nil {
		return err
	}
	// The key field deletion is not allowed.
	if pSchema := tSchema.Parent; pSchema != nil {
		elem := gpath.GetElem()
		if strings.Contains(pSchema.Key, elem[len(elem)-1].GetName()) {
			return nil
		}
	}
	if err := ytypes.DeleteNode(schema, base, gpath); err != nil {
		return err
	}
	return nil
}
