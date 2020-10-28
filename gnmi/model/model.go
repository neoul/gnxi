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

package model

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/neoul/gnxi/gnmi/model/gostruct"
	"github.com/neoul/gnxi/utilities/xpath"
	"github.com/neoul/libydb/go/ydb"
	"github.com/openconfig/goyang/pkg/yang"
	"github.com/openconfig/ygot/ygot"
	"github.com/openconfig/ygot/ytypes"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
)

// Model contains the model data and GoStruct information for the device.
type Model struct {
	*ytypes.Schema
	modelData []*gpb.ModelData
	// SchemaTree      map[string]*yang.Entry
}

// GetName returns the name of the model.
func (m *Model) GetName() string {
	rootSchema := m.RootSchema()
	if rootSchema == nil {
		return "unknown"
	}
	return rootSchema.Name
}

// GetRootType returns the reflect.Type of the root
func (m *Model) GetRootType() reflect.Type {
	return reflect.TypeOf(m.Root).Elem()
}

// NewRoot returns new root (ygot.ValidatedGoStruct)
func (m *Model) NewRoot() ygot.ValidatedGoStruct {
	root := reflect.New(m.GetRootType()).Interface()
	rootGoStruct, ok := root.(ygot.ValidatedGoStruct)
	if ok {
		return rootGoStruct
	}
	return nil
}

// NewModel returns an instance of Model struct.
func NewModel() *Model {
	Schema, err := gostruct.Schema()
	if err != nil {
		panic("schema error: " + err.Error())
	}
	m := &Model{
		Schema:    Schema,
		modelData: gostruct.ΓModelData,
	}
	return m
}

// NewCustomModel returns an instance of Model struct.
func NewCustomModel(schema func() (*ytypes.Schema, error), modelData []*gpb.ModelData) *Model {
	s, err := schema()
	if err != nil {
		panic("schema error: " + err.Error())
	}
	m := &Model{
		Schema:    s,
		modelData: modelData,
	}
	return m
}

// SupportedModels returns a list of supported models.
func (m *Model) SupportedModels() []string {
	mDesc := make([]string, len(m.modelData))
	for i, m := range m.modelData {
		mDesc[i] = fmt.Sprintf("%s %s", m.Name, m.Version)
	}
	sort.Strings(mDesc)
	return mDesc
}

// CheckModels checks whether models are supported by the model. Return error if anything is unsupported.
func (m *Model) CheckModels(models []*gpb.ModelData) error {
	for _, mo := range models {
		isSupported := false
		for _, supportedModel := range m.modelData {
			if reflect.DeepEqual(mo, supportedModel) {
				isSupported = true
				break
			}
		}
		if !isSupported {
			return fmt.Errorf("unsupported model: %v", m)
		}
	}
	return nil
}

// GetModelData - returns ModelData of the model.
func (m *Model) GetModelData() []*gpb.ModelData {
	return m.modelData
}

// GetSchema - returns the root yang.Entry of the model.
func (m *Model) GetSchema() *yang.Entry {
	return m.RootSchema()
}

// FindSchemaByType - find the yang.Entry by Type for schema info.
func (m *Model) FindSchemaByType(t reflect.Type) *yang.Entry {
	if t == reflect.TypeOf(nil) {
		return nil
	}
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
		if t == reflect.TypeOf(nil) {
			return nil
		}
	}
	return m.SchemaTree[t.Name()]
}

// FindSchema - find the yang.Entry by value for schema info.
func (m *Model) FindSchema(value interface{}) *yang.Entry {
	return m.FindSchemaByType(reflect.TypeOf(value))
}

// findSchemaByName - find the yang.Entry for schema info.
func findSchemaByName(parent *yang.Entry, name string) *yang.Entry {
	return parent.Dir[name]
}

// findSchemaByXPath - find *yang.Entry by xpath
func findSchemaByXPath(entry *yang.Entry, p string) (*yang.Entry, error) {
	if entry == nil {
		return nil, fmt.Errorf("no base schema entry found")
	}
	slicedPath, err := xpath.ParseStringPath(p)
	if err != nil {
		return nil, err
	}
	for _, elem := range slicedPath {
		switch v := elem.(type) {
		case string:
			entry = entry.Dir[v]
			if entry == nil {
				return nil, fmt.Errorf("no schema entry found for '%s'", v)
			}
		case map[string]string:
			// skip keys
		default:
			return nil, fmt.Errorf("wrong data type: %v(%T)", v, v)
		}
	}
	if entry == nil {
		return nil, fmt.Errorf("no schema entry found")
	}
	return entry, nil
}

// FindSchemaByXPath - find *yang.Entry by XPath from root model
func (m *Model) FindSchemaByXPath(p string) (*yang.Entry, error) {
	return findSchemaByXPath(m.RootSchema(), p)
}

// FindSchemaByRelativeXPath - find *yang.Entry by XPath from baseValue
func (m *Model) FindSchemaByRelativeXPath(baseValue interface{}, p string) (*yang.Entry, error) {
	return findSchemaByXPath(m.FindSchema(baseValue), p)
}

// findSchemaByGNMIPath - find *yang.Entry by gNMI Path
func findSchemaByGNMIPath(entry *yang.Entry, path *gpb.Path) (*yang.Entry, error) {
	if path == nil {
		return nil, fmt.Errorf("no path")
	}
	if entry == nil {
		return nil, fmt.Errorf("no base value")
	}
	for _, e := range path.GetElem() {
		entry = findSchemaByName(entry, e.GetName())
		if entry == nil {
			return nil, fmt.Errorf("schema not found for %s", e.GetName())
		}
	}
	return entry, nil
}

// FindSchemaByGNMIPath - find *yang.Entry by gNMI Path from root model
func (m *Model) FindSchemaByGNMIPath(path *gpb.Path) (*yang.Entry, error) {
	return findSchemaByGNMIPath(m.RootSchema(), path)
}

// FindSchemaByRelativeGNMIPath - find *yang.Entry by gNMI Path from baseValue
func (m *Model) FindSchemaByRelativeGNMIPath(baseValue interface{}, path *gpb.Path) (*yang.Entry, error) {
	return findSchemaByGNMIPath(m.FindSchema(baseValue), path)
}

// FindAllPaths - finds all XPaths against to the gNMI Path that has wildcard
func (m *Model) FindAllPaths(path *gpb.Path) ([]string, bool) {
	elems := path.GetElem()
	if len(elems) <= 0 {
		return []string{"/"}, true
	}
	p := ""
	sp := pathFinder{
		t:    m.GetRootType(),
		path: &p,
	}

	rsplist := m.findAllPaths(sp, elems)
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

// findAllPaths - finds all XPaths matched to the gNMI Path.
// It is used to find all schema nodes matched to a wildcard path.
func (m *Model) findAllPaths(sp pathFinder, elems []*gpb.PathElem) []pathFinder {
	// select all child nodes if the current node is a list.
	if sp.t.Kind() == reflect.Map {
		rv := []pathFinder{}
		csplist, ok := getAllSchemaPaths(sp)
		if ok {
			for _, csp := range csplist {
				rv = append(rv, m.findAllPaths(csp, elems)...)
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
				rv = append(rv, m.findAllPaths(csp, celems)...)
			}
		}
		return rv
	} else if elem.GetName() == "..." {
		rv := []pathFinder{}
		csplist, ok := getAllSchemaPaths(sp)
		if ok {
			celems := elems[1:]
			for _, csp := range csplist {
				ccsplist := m.findAllPaths(csp, celems)
				if len(ccsplist) > 0 {
					rv = append(rv, ccsplist...)
				}
				rv = append(rv, m.findAllPaths(csp, elems)...)
			}
		}
		return rv
	}

	elemName := elem.GetName()
	csp, ok := findSchemaPath(sp, elemName)
	if !ok {
		return []pathFinder{}
	}
	keys := elem.GetKey()
	if keys != nil && len(keys) > 0 {
		if ydb.IsTypeMap(csp.t) {
			schema := m.FindSchemaByType(csp.t.Elem())
			if schema != nil {
				npath := ""
				knamelist := strings.Split(schema.Key, " ")
				for _, kname := range knamelist {
					kvalue, ok := keys[kname]
					if ok {
						npath = npath + fmt.Sprintf("[%s=%s]", kname, kvalue)
					} else {
						npath = ""
						break
					}
				}
				if npath != "" {
					npath = *csp.path + npath
					csp.path = &npath
				}
			}
		}
	}

	return m.findAllPaths(csp, elems[1:])
}

// FindAllData - finds all data nodes matched to the gNMI Path.
func (m *Model) FindAllData(gs ygot.GoStruct, path *gpb.Path) ([]*DataAndPath, bool) {
	t := reflect.TypeOf(gs)
	entry := m.FindSchemaByType(t)
	if entry == nil {
		return []*DataAndPath{}, false
	}

	elems := path.GetElem()
	if len(elems) <= 0 {
		dataAndGNMIPath := &DataAndPath{
			Value: gs, Path: "/",
		}
		return []*DataAndPath{dataAndGNMIPath}, true
	}
	for _, e := range elems {
		if e.Name == "*" || e.Name == "..." {
			break
		}
		entry = findSchemaByName(entry, e.Name)
		if entry == nil {
			return []*DataAndPath{}, false
		}
		if e.Key != nil {
			for kname := range e.Key {
				if !strings.Contains(entry.Key, kname) {
					return []*DataAndPath{}, false
				}
			}
		}
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

// ValidatePathSchema - validates all schema of the gNMI Path.
func (m *Model) ValidatePathSchema(path *gpb.Path) bool {
	t := m.GetRootType()
	entry := m.FindSchemaByType(t)
	if entry == nil {
		return false
	}

	elems := path.GetElem()
	if len(elems) <= 0 {
		return true
	}
	for _, e := range elems {
		entry = findSchemaByName(entry, e.Name)
		if entry == nil {
			return false
		}
		if e.Key != nil {
			for kname := range e.Key {
				if !strings.Contains(entry.Key, kname) {
					return false
				}
			}
		}
	}
	return true
}

// FindSchemaPaths - validates all schema of the gNMI Path.
func (m *Model) FindSchemaPaths(path *gpb.Path) ([]string, bool) {
	t := m.GetRootType()
	entry := m.FindSchemaByType(t)
	if entry == nil {
		return nil, false
	}
	var elems []*gpb.PathElem
	elems = path.GetElem()
	if len(elems) == 0 {
		return []string{"/"}, true
	}
	paths := m.findSchemaPath("", entry, elems)
	// for i, p := range paths {
	// 	fmt.Println(i, p)
	// }
	return paths, true
}

func (m *Model) findSchemaPath(prefix string, parent *yang.Entry, elems []*gpb.PathElem) []string {
	if len(elems) == 0 {
		return []string{prefix}
	}
	if parent.Dir == nil || len(parent.Dir) == 0 {
		return nil
	}
	e := elems[0]
	if e.Name == "*" {
		founds := make([]string, 0, 8)
		for cname, centry := range parent.Dir {
			founds = append(founds,
				m.findSchemaPath(prefix+"/"+cname, centry, elems[1:])...)
		}
		return founds
	} else if e.Name == "..." {
		founds := make([]string, 0, 16)
		for cname, centry := range parent.Dir {
			founds = append(founds,
				m.findSchemaPath(prefix+"/"+cname, centry, elems[1:])...)
			founds = append(founds,
				m.findSchemaPath(prefix+"/"+cname, centry, elems[0:])...)
		}
		return founds
	}
	entry := findSchemaByName(parent, e.Name)
	if entry == nil {
		return nil
	}
	if e.Key != nil {
		for kname := range e.Key {
			if !strings.Contains(entry.Key, kname) {
				return nil
			}
		}
	}
	return m.findSchemaPath(prefix+"/"+e.Name, entry, elems[1:])
}

func (m *Model) findDataPath(prefix string, parent *yang.Entry, elems []*gpb.PathElem) []string {
	if len(elems) == 0 {
		return []string{prefix}
	}
	if parent.Dir == nil || len(parent.Dir) == 0 {
		return nil
	}
	e := elems[0]
	if e.Name == "*" {
		founds := make([]string, 0, 8)
		for cname, centry := range parent.Dir {
			founds = append(founds,
				m.findDataPath(prefix+"/"+cname, centry, elems[1:])...)
		}
		return founds
	} else if e.Name == "..." {
		founds := make([]string, 0, 16)
		for cname, centry := range parent.Dir {
			founds = append(founds,
				m.findDataPath(prefix+"/"+cname, centry, elems[1:])...)
			founds = append(founds,
				m.findDataPath(prefix+"/"+cname, centry, elems[0:])...)
		}
		return founds
	}
	name := e.Name
	entry := findSchemaByName(parent, e.Name)
	if entry == nil {
		return nil
	}
	if e.Key != nil {
		for kname := range e.Key {
			if !strings.Contains(entry.Key, kname) {
				return nil
			}
		}
		knames := strings.Split(entry.Key, " ")
		for _, kname := range knames {
			if kval, ok := e.Key[kname]; ok {
				if kval == "*" {
					break
				}
				name = fmt.Sprintf("%s[%s=%s]", name, kname, kval)
			} else {
				break
			}
		}
	}
	return m.findDataPath(prefix+"/"+name, entry, elems[1:])
}

type dataAndSchemaPath struct {
	schemaPath *string
	dataPath   *string
}

func (m *Model) findSchemaAndDataPath(path dataAndSchemaPath, parent *yang.Entry, elems []*gpb.PathElem) []dataAndSchemaPath {
	if len(elems) == 0 {
		return []dataAndSchemaPath{path}
	}
	if parent.Dir == nil || len(parent.Dir) == 0 {
		return nil
	}
	e := elems[0]
	if e.Name == "*" {
		founds := make([]dataAndSchemaPath, 0, 8)
		for cname, centry := range parent.Dir {
			datapath := *path.dataPath + "/" + cname
			schemapath := *path.schemaPath + "/" + cname
			path.dataPath = &datapath
			path.schemaPath = &schemapath
			founds = append(founds,
				m.findSchemaAndDataPath(path, centry, elems[1:])...)
		}
		return founds
	} else if e.Name == "..." {
		founds := make([]dataAndSchemaPath, 0, 16)
		for cname, centry := range parent.Dir {
			datapath := *path.dataPath + "/" + cname
			schemapath := *path.schemaPath + "/" + cname
			path.dataPath = &datapath
			path.schemaPath = &schemapath
			founds = append(founds,
				m.findSchemaAndDataPath(path, centry, elems[1:])...)
			founds = append(founds,
				m.findSchemaAndDataPath(path, centry, elems[0:])...)
		}
		return founds
	}
	name := e.Name
	entry := findSchemaByName(parent, e.Name)
	if entry == nil {
		return nil
	}
	if e.Key != nil {
		for kname := range e.Key {
			if !strings.Contains(entry.Key, kname) {
				return nil
			}
		}
		knames := strings.Split(entry.Key, " ")
		for _, kname := range knames {
			if kval, ok := e.Key[kname]; ok {
				if kval == "*" {
					break
				}
				name = fmt.Sprintf("%s[%s=%s]", name, kname, kval)
			} else {
				break
			}
		}
	}
	datapath := *path.dataPath + "/" + name
	schemapath := *path.schemaPath + "/" + e.Name
	path.dataPath = &datapath
	path.schemaPath = &schemapath
	return m.findSchemaAndDataPath(path, entry, elems[1:])
}
