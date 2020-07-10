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

// FindAllNodes - finds all nodes matched to the gNMI Path.
func FindAllNodes(vgs ygot.ValidatedGoStruct, path *pb.Path) ([]interface{}, bool) {
	elems := path.GetElem()
	if len(elems) <= 0 {
		return []interface{}{vgs}, true
	}
	v := reflect.ValueOf(vgs)
	rvlist := findAllNodes(v, elems)
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

// findAllNodes - finds all nodes matched to the gNMI Path.
func findAllNodes(v reflect.Value, elems []*pb.PathElem) []reflect.Value {
	// select all child nodes if the current node is a list.
	if v.Kind() == reflect.Map {
		rv := []reflect.Value{}
		cvlist, ok := ydb.ValGetAll(v)
		if ok {
			for _, cv := range cvlist {
				rv = append(rv, findAllNodes(cv, elems)...)
			}
		}
		return rv
	}
	if len(elems) <= 0 {
		return []reflect.Value{v}
	}
	elem := elems[0]
	fmt.Println("** Search", elem.GetName(), "from", utils.SprintStructInline(v.Interface()))
	if elem.GetName() == "*" {
		rv := []reflect.Value{}
		cvlist, ok := ydb.ValGetAll(v)
		if ok {
			celems := elems[1:]
			for _, cv := range cvlist {
				rv = append(rv, findAllNodes(cv, celems)...)
			}
		}
		return rv
	} else if elem.GetName() == "..." {
		rv := []reflect.Value{}
		cvlist, ok := ydb.ValGetAll(v)
		if ok {
			celems := elems[1:]
			for _, cv := range cvlist {
				rv = append(rv, findAllNodes(cv, celems)...)
			}
			for _, cv := range cvlist {
				ccvlist := findAllNodes(cv, celems)
				if len(ccvlist) > 0 {
					rv = append(rv, ccvlist...)
				}
				rv = append(rv, findAllNodes(cv, elems)...)
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
	return findAllNodes(v, elems[1:])
}

// type DataFinder interface {
// 	FindChild(*pb.Path) []interface{}
// }

// type PathNode struct {
// 	elem     *pb.PathElem
// 	children map[string]*PathNode
// 	data     interface{}
// }

// func (n *PathNode) find(elems []*pb.PathElem) []interface{} {
// 	data := []interface{}{}
// 	if len(elems) <= 0 {
// 		return []interface{}{n.data}
// 	}
// 	elem := elems[0]
// 	cn, ok := n.children[elem.Name]
// 	if ok {
// 		matched := true
// 		for k, v := range elem.Key {
// 			if cnv, ok := cn.elem.Key[k]; ok {
// 				if v != cnv {
// 					if cnv != "*" {
// 						matched = false
// 					}
// 				}
// 			} else {
// 				matched = false
// 			}
// 		}
// 		if matched {
// 			data = append(data, cn.find(elems[1:])...)
// 		}
// 	}
// 	cn, ok = n.children["*"]
// 	if ok {
// 		data = append(data, cn.find(elems[1:])...)
// 	}
// 	cn, ok = n.children["..."]
// 	if ok {
// 		data = append(data, cn.find(elems[1:])...)
// 	}
// 	return data
// }

// // FindSchema - finds the child schema node from top
// func FindSchema(top *yang.Entry, prefix, path *pb.Path) (*yang.Entry, error) {
// 	fullpath := GNMIFullPath(prefix, path)
// 	elems := fullpath.GetElem()
// 	if len(elems) <= 0 {
// 		return top, nil
// 	}
// 	schema := top
// 	for _, elem := range elems {
// 		if name == "*" {

// 		}
// 		name := []string{elem.Name}
// 		schema = yutil.FirstChild(schema, name)

// 	}
// }
