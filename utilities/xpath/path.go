/* Copyright 2017 Google Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package xpath parses xpath string into gnmi Path struct.
package xpath

import (
	"fmt"
	"strings"

	"github.com/golang/protobuf/proto"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
)

var (
	// RootGNMIPath - '/'
	RootGNMIPath = &gnmipb.Path{}
	// EmptyGNMIPath - ./
	EmptyGNMIPath = RootGNMIPath

	// WildcardGNMIPathDot3 - Wildcard Path '...'
	WildcardGNMIPathDot3 = &gnmipb.Path{
		Elem: []*gnmipb.PathElem{
			&gnmipb.PathElem{
				Name: "...",
			},
		},
	}
	// WildcardGNMIPathAsterisk - Wildcard Path '*'
	WildcardGNMIPathAsterisk = &gnmipb.Path{
		Elem: []*gnmipb.PathElem{
			&gnmipb.PathElem{
				Name: "*",
			},
		},
	}
)

// GNMIAliasPath returns Alias gNMI Path.
func GNMIAliasPath(name, target, origin string) *gnmipb.Path {
	return &gnmipb.Path{
		Target: target,
		Origin: origin,
		Elem: []*gnmipb.PathElem{
			&gnmipb.PathElem{
				Name: name,
			},
		},
	}
}

// CloneGNMIPath returns cloned gNMI Path.
func CloneGNMIPath(src *gnmipb.Path) *gnmipb.Path {
	return proto.Clone(src).(*gnmipb.Path)
}

// UpdateGNMIPathElem returns the updated gNMI Path.
func UpdateGNMIPathElem(target, src *gnmipb.Path) *gnmipb.Path {
	if target == nil {
		return CloneGNMIPath(src)
	}
	if src == nil {
		return target
	}
	l := len(src.GetElem())
	pathElems := []*gnmipb.PathElem{}
	if l > 0 {
		pathElems = make([]*gnmipb.PathElem, l)
		copy(pathElems, src.GetElem())
	}
	target.Elem = pathElems
	return target
}

// ValidateGNMIPath - checks the validation of the gNMI path.
func ValidateGNMIPath(path ...*gnmipb.Path) (*gnmipb.Path, error) {
	if len(path) == 0 {
		return nil, fmt.Errorf("no input path")
	}
	p := proto.Clone(path[0]).(*gnmipb.Path)
	for i := range path {
		if path[i].GetElem() == nil && path[i].GetElement() != nil {
			return nil, fmt.Errorf("deprecated path.element used")
		}
		if i > 0 {
			if path[i].Target != "" { // 2.2.2.1 Path Target
				return nil, fmt.Errorf("path.target MUST only ever be present on the prefix path")
			}
			p = GNMIFullPath(p, path[i])
		}
	}
	return p, nil
}

// mergePaths returns a single merged path
func mergePaths(absPathElem, relativePathElem []*gnmipb.PathElem) []*gnmipb.PathElem {
	abslen := len(absPathElem)
	mergedPath := make([]*gnmipb.PathElem, abslen, abslen+len(relativePathElem))
	copy(mergedPath, absPathElem)
	for _, elem := range relativePathElem {
		switch {
		case elem.Name == ".":
			// do nothing
		case elem.Name == "..":
			max := len(mergedPath)
			if max < 0 {
				return nil
			}
			mergedPath = mergedPath[:max-1]
		default:
			mergedPath = append(mergedPath, elem)
		}
	}
	return mergedPath
}

// GNMIFullPath builds the full path from the prefix and path.
func GNMIFullPath(prefix, path *gnmipb.Path) *gnmipb.Path {
	if prefix == nil {
		if path == nil {
			return &gnmipb.Path{}
		}
		return path
	}
	fullPath := proto.Clone(prefix).(*gnmipb.Path)
	// fullPath := &gnmipb.Path{Target: prefix.Target, Origin: prefix.Origin}
	if path.GetElement() != nil {
		fullPath.Element = append(prefix.GetElement(), path.GetElement()...)
	}
	if path.GetElem() != nil {
		fullPath.Elem = mergePaths(prefix.GetElem(), path.GetElem())
		if fullPath.Elem == nil {
			return nil
		}
	}
	return fullPath
}

// IsSchemaPath - returns the path is schema path.
func IsSchemaPath(path *gnmipb.Path) bool {
	isSchemaPath := true
	if path != nil {
		for _, e := range path.Elem {
			if e.Key != nil && len(e.Key) > 0 {
				isSchemaPath = false
			}
		}
	}
	return isSchemaPath
}

// ToGNMIPath parses an xpath string into a gnmi Path struct defined in gnmi
// proto. Path convention can be found in
// https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-path-conventions.md
//
// For example, xpath /interfaces/interface[name=Ethernet1/2/3]/state/counters
// will be parsed to:
//
//    elem: <name: "interfaces" >
//    elem: <
//        name: "interface"
//        key: <
//            key: "name"
//            value: "Ethernet1/2/3"
//        >
//    >
//    elem: <name: "state" >
//    elem: <name: "counters" >
func ToGNMIPath(xpath string) (*gnmipb.Path, error) {
	xpathElements, err := ParseStringPath(xpath)
	if err != nil {
		return nil, err
	}
	var pbPathElements []*gnmipb.PathElem
	for _, elem := range xpathElements {
		switch v := elem.(type) {
		case string:
			pbPathElements = append(pbPathElements, &gnmipb.PathElem{Name: v})
		case map[string]string:
			n := len(pbPathElements)
			if n == 0 {
				return nil, fmt.Errorf("missing name before key-value list")
			}
			if pbPathElements[n-1].Key != nil {
				return nil, fmt.Errorf("two subsequent key-value lists")
			}
			pbPathElements[n-1].Key = v
		default:
			return nil, fmt.Errorf("wrong data type: %T", v)
		}
	}
	return &gnmipb.Path{Elem: pbPathElements}, nil
}

// ToSchemaPath - returns the schema path of the data xpath
func ToSchemaPath(xpath string) (string, error) {
	xpathElements, err := ParseStringPath(xpath)
	if err != nil {
		return "", err
	}
	schemaElememts := []string{""}
	for _, elem := range xpathElements {
		switch v := elem.(type) {
		case string:
			schemaElememts = append(schemaElememts, v)
		case map[string]string:
			// skip keys
		default:
			return "", fmt.Errorf("wrong data type: %T", v)
		}
	}
	return strings.Join(schemaElememts, "/"), nil
}

// ToSchemaSlicePath - returns the sliced schema path and whether equal or not of the input sliced xpath. And
func ToSchemaSlicePath(xpath []string) ([]string, bool) {
	isEqual := true
	schemaElememts := make([]string, 0, len(xpath))
	for _, x := range xpath {
		xpathElements, err := ParseStringPath(x)
		if err != nil {
			return nil, false
		}
		for _, elem := range xpathElements {
			switch v := elem.(type) {
			case string:
				schemaElememts = append(schemaElememts, v)
			case map[string]string:
				// skip keys
				isEqual = false
			default:
				return nil, false
			}
		}
	}

	return schemaElememts, isEqual
}

// ToXPath - returns XPath string converted from gNMI Path
func ToXPath(p *gnmipb.Path) string {
	if p == nil {
		return ""
	}
	// elems := p.GetElem()
	// if len(elems) == 0 {
	// 	return "/"
	// }

	pe := []string{""}
	for _, e := range p.GetElem() {
		if e.GetKey() != nil {
			ke := []string{e.GetName()}
			for k, kv := range e.GetKey() {
				ke = append(ke, fmt.Sprintf("[%s=%s]", k, kv))
			}
			pe = append(pe, strings.Join(ke, ""))
		} else {
			pe = append(pe, e.GetName())
		}
	}
	// gnmi.Path.Element is deprecated, but being gracefully handled
	// when gnmi.PathElem doesn't exist
	if len(pe) == 0 {
		return strings.Join(p.GetElement(), "/")
	}
	return strings.Join(pe, "/")
}

// PathElemToXPATH - returns XPath string converted from gNMI Path
func PathElemToXPATH(elem []*gnmipb.PathElem, schemaPath bool) string {
	if elem == nil {
		return ""
	}
	if schemaPath {
		pe := []string{""}
		for _, e := range elem {
			pe = append(pe, e.GetName())
		}
		return strings.Join(pe, "/")
	}

	pe := []string{""}
	for _, e := range elem {
		if e.GetKey() != nil {
			ke := []string{e.GetName()}
			for k, kv := range e.GetKey() {
				ke = append(ke, fmt.Sprintf("[%s=%s]", k, kv))
			}
			pe = append(pe, strings.Join(ke, ""))
		} else {
			pe = append(pe, e.GetName())
		}
	}
	return strings.Join(pe, "/")
}

// SlicePathToGNMIPath returns the gNMI Path from a string slice.
func SlicePathToGNMIPath(path []string) (*gnmipb.Path, error) {
	var pathElem []*gnmipb.PathElem
	for _, pelem := range path {
		xpathElements, err := ParseStringPath(pelem)
		if err != nil {
			return nil, err
		}
		for _, elem := range xpathElements {
			switch v := elem.(type) {
			case string:
				pathElem = append(pathElem, &gnmipb.PathElem{Name: v})
			case map[string]string:
				n := len(pathElem)
				if n == 0 {
					return nil, fmt.Errorf("missing name before key-value list")
				}
				if pathElem[n-1].Key != nil {
					return nil, fmt.Errorf("two subsequent key-value lists")
				}
				pathElem[n-1].Key = v
			default:
				return nil, fmt.Errorf("wrong data type: %T", v)
			}
		}
	}
	return &gnmipb.Path{Elem: pathElem}, nil
}
