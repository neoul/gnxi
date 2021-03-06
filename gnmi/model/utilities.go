package model

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/neoul/gnxi/utilities/status"
	"github.com/neoul/gnxi/utilities/xpath"
	"github.com/neoul/libydb/go/ydb"
	"github.com/openconfig/gnmi/proto/gnmi"
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
		return dpath.getAllChildren(key, opt...)
	}
	child := &dataAndPath{Value: cv, Key: appendCopy(dpath.Key, key)}
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
func FindSchemaByGNMIPath(baseSchema *yang.Entry, path *gnmipb.Path) *yang.Entry {
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

func writeLeaf(schema *yang.Entry, base ygot.GoStruct, gpath *gnmipb.Path, t reflect.Type, v string) error {
	var typedValue *gnmipb.TypedValue
	if t == reflect.TypeOf(nil) {
		return fmt.Errorf("invalid type inserted for %s", xpath.ToXPath(gpath))
	}
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	vv, err := ytypes.StringToType(t, v)
	if err != nil {
		var yerr error
		vv, yerr = ydb.ValScalarNew(t, v)
		if yerr != nil {
			return err
		}
	}

	typedValue, err = ygot.EncodeTypedValue(vv.Interface(), gnmipb.Encoding_JSON_IETF)
	if err != nil {
		return err
	}
	err = ytypes.SetNode(schema, base, gpath, typedValue, &ytypes.InitMissingElements{})
	if err != nil {
		return fmt.Errorf("%s", status.FromError(err).Message)
	}
	return err
}

func writeLeafList(schema *yang.Entry, base interface{}, gpath *gnmipb.Path, t reflect.Type, v string) error {
	sv, err := ydb.ValSliceNew(t)
	if err != nil {
		return err
	}
	vv, err := ytypes.StringToType(t.Elem(), v)
	if err != nil {
		var yerr error
		vv, yerr = ydb.ValScalarNew(t, v)
		if yerr != nil {
			return err
		}
	}
	rv, err := ydb.ValSliceAppend(sv, vv.Interface())
	if err != nil {
		return err
	}
	typedValue, err := ygot.EncodeTypedValue(rv.Interface(), gnmipb.Encoding_JSON_IETF)
	if err != nil {
		return err
	}

	err = ytypes.SetNode(schema, base, gpath, typedValue, &ytypes.InitMissingElements{})
	if err != nil {
		return fmt.Errorf("%s", status.FromError(err).Message)
	}
	return nil
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
			return fmt.Errorf("type update to schema tree failed for %s", SchemaToXPath(schema))
		}
	}
	return nil
}

func baseType(yangtype *yang.YangType) []reflect.Type {
	switch yangtype.Kind {
	case yang.Ystring:
		return []reflect.Type{reflect.TypeOf(string(""))}
	case yang.Ybool, yang.Yempty:
		return []reflect.Type{reflect.TypeOf(bool(false))}
	case yang.Yint8:
		return []reflect.Type{reflect.TypeOf(int8(0))}
	case yang.Yint16:
		return []reflect.Type{reflect.TypeOf(int16(0))}
	case yang.Yint32:
		return []reflect.Type{reflect.TypeOf(int32(0))}
	case yang.Yint64:
		return []reflect.Type{reflect.TypeOf(int64(0))}
	case yang.Yuint8:
		return []reflect.Type{reflect.TypeOf(uint8(0))}
	case yang.Yuint16:
		return []reflect.Type{reflect.TypeOf(uint16(0))}
	case yang.Yuint32:
		return []reflect.Type{reflect.TypeOf(uint32(0))}
	case yang.Yuint64:
		return []reflect.Type{reflect.TypeOf(uint64(0))}
	case yang.Ybinary:
		return []reflect.Type{reflect.TypeOf([]byte{})}
	case yang.Ybits:
		return []reflect.Type{reflect.TypeOf(int8(0))}
	case yang.Ydecimal64:
		return []reflect.Type{reflect.TypeOf(float64(0))}
	case yang.YinstanceIdentifier, yang.Yleafref:
		return []reflect.Type{reflect.TypeOf(string(""))}
	case yang.Yenum, yang.Yidentityref:
		return []reflect.Type{reflect.TypeOf(int64(0))}
	case yang.Yunion:
		types := []reflect.Type{}
		for i := range yangtype.Type {
			types = append(types, baseType(yangtype.Type[i])...)
		}
		return types
	// case yang.Ynone:
	default:
		return []reflect.Type{reflect.TypeOf(nil)}
	}
}

// UpdateType updates the schema for the fast type searching..
func (mo *MO) UpdateType() error {
	curSchema := mo.GetSchema()
	curType := mo.GetRootType()
	return updateType(mo, curSchema, curType)
}

// GetOrCreateNode gets or creates all parent nodes and the target node in gpath and then returns
// the parent and the target nodes via ytypes.TreeNode with the error information.
func (mo *MO) GetOrCreateNode(gpath *gnmipb.Path) (*ytypes.TreeNode, *ytypes.TreeNode, error) {
	var err error
	var index int
	var tSchema, pSchema *yang.Entry
	var leaf, parent interface{}
	if len(gpath.GetElem()) == 0 {
		return &ytypes.TreeNode{Schema: mo.GetSchema(), Data: mo.GetRoot(), Path: &gnmipb.Path{}},
			&ytypes.TreeNode{Schema: mo.GetSchema(), Data: mo.GetRoot(), Path: &gnmipb.Path{}},
			nil
	}
	pSchema = mo.GetSchema()
	parent = mo.GetRoot()
	tSchema = mo.GetSchema()
	leaf = mo.GetRoot()
	index = 0
	for i := range gpath.GetElem() {
		index = i
		pSchema = tSchema
		parent = leaf
		p := &gnmipb.Path{
			Elem: []*gnmipb.PathElem{
				gpath.Elem[i],
			},
		}
		leaf, tSchema, err = ytypes.GetOrCreateNode(pSchema, parent, p)
		if err != nil {
			return nil, nil, fmt.Errorf("%s", status.FromError(err).Message)
		}
	}
	return &ytypes.TreeNode{Schema: pSchema, Data: parent, Path: &gnmipb.Path{Elem: gpath.Elem[:index]}},
		&ytypes.TreeNode{Schema: tSchema, Data: leaf, Path: &gnmipb.Path{Elem: gpath.Elem[index:]}},
		nil
}

// WriteValue writes Go Struct or raw data directly
func (mo *MO) WriteValue(gpath *gnmipb.Path, value interface{}) error {
	parent, target, err := mo.GetOrCreateNode(gpath)
	if err != nil {
		return err
	}
	if target.Schema.IsDir() {
		tv := ydb.GetNonIfOrPtrValueDeep(reflect.ValueOf(target.Data))
		v := ydb.GetNonIfOrPtrValueDeep(reflect.ValueOf(value))
		if !tv.CanSet() {
			return fmt.Errorf("unable to set %s", target.Schema.Name)
		}
		tv.Set(v)
	} else { // (schema.IsLeaf() || schema.IsLeafList())
		if _, err = ydb.ValChildSet(reflect.ValueOf(parent.Data),
			target.Schema.Name, value, ydb.SearchByContent); err != nil {
			return err
		}
	}
	return nil
}

// WriteTypedValue writes the TypedValue to the MO (Modeled Object)
func (mo *MO) WriteTypedValue(gpath *gnmipb.Path, typedValue *gnmipb.TypedValue) error {
	parent, target, err := mo.GetOrCreateNode(gpath)
	if err != nil {
		return err
	}
	switch {
	case target.Schema.IsDir():
		// json & json-ietf values configuration for container and list nodes
		if typedValue.GetJsonVal() != nil {
			return mo.UnmarshalInternalJSON(gpath, typedValue.GetJsonVal())
		}
		return mo.Unmarshal(typedValue.GetJsonIetfVal(), target.Data.(ygot.GoStruct))
	}
	switch target.Schema.Type.Kind {
	case yang.Yempty:
		_, err = ydb.ValChildSet(reflect.ValueOf(parent.Data),
			target.Schema.Name, typedValue.GetBoolVal(), ydb.SearchByContent)
		return err
	}
	// json & json-ietf values configuration for leaf and leaf-list
	switch {
	case typedValue.GetJsonVal() != nil:
		return mo.UnmarshalInternalJSON(gpath, typedValue.GetJsonVal())
	case typedValue.GetJsonIetfVal() != nil:
		var rawdata interface{}
		json.Unmarshal(typedValue.GetJsonIetfVal(), &rawdata)
		vstring := fmt.Sprint(rawdata)
		return WriteStringValue(parent.Schema, parent.Data, target.Path, vstring)
	}
	err = ytypes.SetNode(mo.GetSchema(), mo.GetRoot(), gpath, typedValue, &ytypes.InitMissingElements{})
	if err != nil {
		return fmt.Errorf("%s", status.FromError(err).Message)
	}
	return nil
}

// WriteStringValue writes the string value to the model instance
func WriteStringValue(pschema *yang.Entry, pvalue interface{}, gpath *gnmipb.Path, value string) error {
	var err error
	var curSchema *yang.Entry
	var curValue interface{}
	var p *gnmipb.Path
	curSchema = pschema
	curValue = pvalue
	for i := range gpath.GetElem() {
		switch {
		case curSchema.IsLeafList():
			return writeLeafList(pschema, pvalue, p, reflect.TypeOf(curValue), gpath.Elem[i].Name)
		case !curSchema.IsDir():
			return fmt.Errorf("invalid path: %s", xpath.ToXPath(gpath))
		}
		p = &gnmipb.Path{
			Elem: []*gnmipb.PathElem{
				gpath.Elem[i],
			},
		}
		curValue, curSchema, err = ytypes.GetOrCreateNode(curSchema, curValue, p)
		if err != nil {
			return fmt.Errorf("%s", status.FromError(err).Message)
		}
		switch {
		case curSchema.IsDir():
			pschema = curSchema
			pvalue = curValue
		case curSchema.IsLeafList():
			if value != "" {
				return writeLeafList(pschema, pvalue, p, reflect.TypeOf(curValue), value)
			}
		default:
			if curSchema.Type != nil {
				switch curSchema.Type.Kind {
				case yang.Yempty:
					_, err = ydb.ValChildSet(reflect.ValueOf(pvalue),
						curSchema.Name, value, ydb.SearchByContent)
					return err
				case yang.Yunion:
					types := baseType(curSchema.Type)
					for i := range types {
						if types[i] == reflect.TypeOf(nil) {
							return fmt.Errorf("invalid union type for %s", xpath.ToXPath(gpath))
						}
						if err = writeLeaf(pschema, pvalue.(ygot.GoStruct), p, types[i], value); err == nil {
							break
						}
					}
					return err
				}
			}
			ct := reflect.TypeOf(curValue)
			return writeLeaf(pschema, pvalue.(ygot.GoStruct), p, ct, value)
		}
	}
	return nil
}

// WriteStringValue writes the string value to the model instance
func (mo *MO) WriteStringValue(gpath *gnmipb.Path, value string) error {
	return WriteStringValue(mo.RootSchema(), mo.GetRoot(), gpath, value)
}

// DeleteValue deletes the value from the modeled object
func (mo *MO) DeleteValue(gpath *gnmipb.Path) error {
	tSchema := FindSchemaByGNMIPath(mo.GetSchema(), gpath)
	if tSchema == nil {
		return fmt.Errorf("schema for %s not found", xpath.ToXPath(gpath))
	}
	// The key field deletion is not allowed.
	if pSchema := tSchema.Parent; pSchema != nil {
		elem := gpath.GetElem()
		if strings.Contains(pSchema.Key, elem[len(elem)-1].GetName()) {
			return nil
		}
	}
	if err := ytypes.DeleteNode(mo.GetSchema(), mo.GetRoot(), gpath); err != nil {
		return err
	}
	return nil
}

// FindValue returns the value in the gpath.
func (mo *MO) FindValue(gpath *gnmipb.Path) interface{} {
	tNode, err := ytypes.GetNode(mo.GetSchema(), mo.GetRoot(), gpath)
	if err != nil {
		return nil
	}
	if len(tNode) <= 0 {
		return nil
	}
	return tNode[0].Data
}

func jsonDataToString(jvalue interface{}) (string, error) {
	var jstring string
	switch jdata := jvalue.(type) {
	case float64:
		jstring = fmt.Sprint(jdata)
	case string:
		jstring = jdata
	case nil:
		jstring = ""
	case bool:
		jstring = fmt.Sprint(jdata)
	case []interface{}:
		return "", fmt.Errorf("unexpected json array")
	case map[string]interface{}:
		return "", fmt.Errorf("unexpected json object")
	default:
		return "", fmt.Errorf("unexpected json type %T", jvalue)
	}
	return jstring, nil
}

func unmarshalInternalJSON(keys []string, j interface{}, mo *MO, gpath *gnmipb.Path) error {
	switch jdata := j.(type) {
	case map[string]interface{}:
		for key, subtree := range jdata {
			keys := append(keys, key)
			if err := unmarshalInternalJSON(keys, subtree, mo, gpath); err != nil {
				return err
			}
		}
	case []interface{}:
		for i := range jdata {
			if err := unmarshalInternalJSON(keys, jdata[i], mo, gpath); err != nil {
				return err
			}
		}
	default:
		jstr, err := jsonDataToString(jdata)
		if err != nil {
			return err
		}
		p := &gnmipb.Path{
			Elem: []*gnmipb.PathElem{},
		}
		length := len(keys)
		schema := mo.FindSchemaByGNMIPath(gpath)
		for i := 0; i < length; i++ {
			schema = schema.Find(keys[i])
			if schema == nil {
				return fmt.Errorf("schema not found for %s", xpath.ToXPath(gpath)+"/"+strings.Join(keys[:i], "/"))
			}
			switch {
			case schema.IsList():
				pe := &gnmipb.PathElem{Name: keys[i], Key: map[string]string{}}
				knames := strings.Split(schema.Key, " ")
				if i+1 >= length {
					return fmt.Errorf("unable to extract key from json bytes from %v", keys)
				}
				keys := strings.Split(keys[i+1], " ")
				if len(keys) != len(knames) {
					return fmt.Errorf("invalid key value (%v) for %s", keys, knames)
				}
				for i := range knames {
					pe.Key[knames[i]] = keys[i]
				}
				p.Elem = append(p.Elem, pe)
				i++
			default:
				p.Elem = append(p.Elem, &gnmipb.PathElem{Name: keys[i]})
			}
		}
		fpath := xpath.GNMIFullPath(gpath, p)
		if err := mo.WriteStringValue(fpath, jstr); err != nil {
			return err
		}
	}
	return nil
}

// UnmarshalInternalJSON unmarshs internal json bytes to the target in the gpath.
func (mo *MO) UnmarshalInternalJSON(gpath *gnmipb.Path, j []byte) error {
	var jsonTree interface{}
	if err := json.Unmarshal(j, &jsonTree); err != nil {
		return err
	}
	if s := mo.FindSchemaByGNMIPath(gpath); s == nil {
		return fmt.Errorf("schema not found for %s", xpath.ToXPath(gpath))
	}
	return unmarshalInternalJSON(nil, jsonTree, mo, gpath)
}

// SchemaToGNMIPath returns gNMI Path (*gnmipb.Path) of the schema (*yang.Entry)
func SchemaToGNMIPath(entry *yang.Entry) *gnmipb.Path {
	if entry == nil {
		return nil
	}
	gpath := &gnmipb.Path{
		Elem: []*gnmipb.PathElem{},
	}
	length := 0
	for e := entry; e != nil && e.Parent != nil; e = e.Parent {
		length++
	}
	gpath.Elem = make([]*gnmipb.PathElem, length)
	for e := entry; e != nil && e.Parent != nil; e = e.Parent {
		length--
		gpath.Elem[length] = &gnmipb.PathElem{Name: e.Name}
	}
	return gpath
}

// SchemaToXPath returns XPath of the schema (*yang.Entry)
func SchemaToXPath(entry *yang.Entry) string {
	if entry == nil {
		return ""
	}
	length := 0
	for e := entry; e != nil && e.Parent != nil; e = e.Parent {
		length++
	}
	p := make([]string, length+1)
	for e := entry; e != nil && e.Parent != nil; e = e.Parent {
		p[length] = e.Name
		length--
	}
	p[0] = ""
	return strings.Join(p, "/")
}
