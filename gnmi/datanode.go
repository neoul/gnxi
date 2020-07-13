package gnmi

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/neoul/gnxi/utils"
	"github.com/neoul/libydb/go/ydb"
	pb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ygot/ygot"
)

// FindAllDataNodes - finds all nodes matched to the gNMI Path.
func FindAllDataNodes(gs ygot.GoStruct, path *pb.Path) ([]interface{}, bool) {
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
func findAllDataNodes(v reflect.Value, elems []*pb.PathElem) []reflect.Value {
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
func FindAllSchemaTypes(gs ygot.GoStruct, path *pb.Path) ([]reflect.Type, bool) {
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
func findAllSchemaTypes(t reflect.Type, elems []*pb.PathElem) []reflect.Type {
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
func FindAllSchemaPaths(gs ygot.GoStruct, path *pb.Path) ([]string, bool) {
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
func findAllSchemaPaths(sp schemaPathElem, elems []*pb.PathElem) []schemaPathElem {
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
