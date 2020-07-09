package xpath

import (
	"reflect"

	"github.com/neoul/libydb/go/ydb"
	pb "github.com/openconfig/gnmi/proto/gnmi"
)

// // FindAllNodes - finds all nodes matched to the gNMI Path.
// func FindAllNodes(vgs ygot.ValidatedGoStruct, path *pb.Path) ([]interface{}, error) {
// 	elems := path.GetElem()
// 	if len(elems) <= 0 {
// 		return []interface{}{vgs}, nil
// 	}
// 	rv := []
// 	v := reflect.ValueOf(vgs)
// 	for _, elem := range elems {
// 		if elem.Name == "*" {
// 			if vs, ok := ydb.ValGetAll(v); ok {
// 				for _, cv := range vs {
// 					if cv.CanInterface() {
// 						FindAllNodes()
// 					}
// 				}
// 			}

// 		} else if elem.Name == "..." {

// 		} else {
// 			cv, ok := ydb.ValFind(v, elem.Name, ydb.SearchByContent)
// 			if !ok || !cv.IsValid() {
// 				return []interface{}{}, fmt.Errorf("not found")
// 			}
// 			v = cv
// 		}
// 	}
// }

// findAllNodes - finds all nodes matched to the gNMI Path.
func findAllNodes(v reflect.Value, elems []*pb.PathElem) []reflect.Value {
	if len(elems) <= 0 {
		return []reflect.Value{v}
	}
	rv := []reflect.Value{}
	for i, elem := range elems {
		if elem.GetName() == "*" {
			if cvlist, ok := ydb.ValGetAll(v); ok && len(cvlist) > 0 {
				if i+1 >= len(elems) {
					return append(rv, cvlist...)
				}
				celems := elems[i+1:]
				for _, cv := range cvlist {
					rv = append(rv, findAllNodes(cv, celems)...)
				}
				return rv
			}
			return rv
		} else if elem.GetName() == "..." {
			if cvlist, ok := ydb.ValGetAll(v); ok && len(cvlist) > 0 {
				if i+1 >= len(elems) {
					return append(rv, cvlist...)
				}
				for _, cv := range cvlist {
					ccvlist := findAllNodes(cv, elems[i+1:])
					if len(ccvlist) > 0 {
						rv = append(rv, ccvlist...)
					}
					rv = append(rv, findAllNodes(cv, elems[i:])...)
				}
				return rv
			}
			return rv
		} else {
			cv, ok := ydb.ValFind(v, elem.GetName(), ydb.SearchByContent)
			if !ok || !cv.IsValid() {
				return []reflect.Value{}
			}
			key := elem.GetKey()
			for k := range key {
				ccv, ok := ydb.ValFind(cv, k, ydb.SearchByContent)
				if !ok {
					return []reflect.Value{}
				}
				cv = ccv
			}
			v = cv
		}
	}
	return []reflect.Value{v}
}

type DataFinder interface {
	FindChild(*pb.Path) []interface{}
}

type PathNode struct {
	elem     *pb.PathElem
	children map[string]*PathNode
	data     interface{}
}

func (n *PathNode) find(elems []*pb.PathElem) []interface{} {
	data := []interface{}{}
	if len(elems) <= 0 {
		return []interface{}{n.data}
	}
	elem := elems[0]
	cn, ok := n.children[elem.Name]
	if ok {
		matched := true
		for k, v := range elem.Key {
			if cnv, ok := cn.elem.Key[k]; ok {
				if v != cnv {
					if cnv != "*" {
						matched = false
					}
				}
			} else {
				matched = false
			}
		}
		if matched {
			data = append(data, cn.find(elems[1:])...)
		}
	}
	cn, ok = n.children["*"]
	if ok {
		data = append(data, cn.find(elems[1:])...)
	}
	cn, ok = n.children["..."]
	if ok {
		data = append(data, cn.find(elems[1:])...)
	}
	return data
}

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
