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
	"errors"
	"fmt"
	"strings"

	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
)

var (
	// RootGNMIPath - '/'
	RootGNMIPath = &gnmipb.Path{}

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

// ValidateGNMIPath - checks the validation of the gNMI path.
func ValidateGNMIPath(path *gnmipb.Path) error {
	if path.GetElem() == nil && path.GetElement() != nil {
		return fmt.Errorf("deprecated path element")
	}
	return nil
}

// ValidateGNMIFullPath - check the validation of the gNMI full path
func ValidateGNMIFullPath(prefix, path *gnmipb.Path) error {
	if path == nil {
		return fmt.Errorf("no path input")
	}
	if path.GetElem() == nil && path.GetElement() != nil {
		return fmt.Errorf("deprecated path element")
	}
	if prefix == nil {
		return nil
	}
	oPre, oPath := prefix.GetOrigin(), path.GetOrigin()
	switch {
	case oPre != "" && oPath != "":
		return errors.New("origin is set both in prefix and path")
	case oPath != "":
		if len(prefix.GetElem()) > 0 {
			return errors.New("path elements in prefix are set even though origin is set in path")
		}
	default:
	}
	return nil
}

// GNMIFullPath builds the full path from the prefix and path.
func GNMIFullPath(prefix, path *gnmipb.Path) *gnmipb.Path {
	if prefix == nil {
		if path == nil {
			return &gnmipb.Path{}
		}
		return path
	}
	fullPath := &gnmipb.Path{Origin: prefix.Origin}
	if path.GetElement() != nil {
		fullPath.Element = append(prefix.GetElement(), path.GetElement()...)
	}
	if path.GetElem() != nil {
		fullPath.Elem = append(prefix.GetElem(), path.GetElem()...)
	}
	return fullPath
}

// IsSchemaPath - returns the path is schema path.
func IsSchemaPath(prefix, path *gnmipb.Path) bool {
	isSchemaPath := true
	if prefix != nil {
		for _, e := range prefix.Elem {
			if e.Key != nil && len(e.Key) > 0 {
				isSchemaPath = false
			}
		}
	}
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
func PathElemToXPATH(elem []*gnmipb.PathElem) string {
	if elem == nil {
		return ""
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
