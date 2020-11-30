// Package model implements a device model based on YANG modules for gNMI service.
package model

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"

	"github.com/golang/glog"
	"github.com/neoul/gnxi/gnmi/model/gostruct"
	"github.com/neoul/gtrie"
	"github.com/neoul/libydb/go/ydb"
	"github.com/openconfig/goyang/pkg/yang"
	"github.com/openconfig/ygot/ytypes"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
)

// Model contains YANG model schema information and its data instance for a device.
// The Model must be updated by StateUpdate interface that implemented in the package
// from an external process or system. The external system is a process that has source data
// for the modeled device and pushs the source data to the gNMI Model using the StateUpdate interface.
type Model struct {
	*MO
	ChangeNotification // A callback interface provided by gnmi/server for the change notification
	StateConfig        // A callback interface provided by the external system for gNMI Set RPC
	StateSync          // A callback interface provided by the external system to request the data update

	schema    func() (*ytypes.Schema, error) // Schema() function generated by ygot generator for schema info.
	modelData []*gnmipb.ModelData            // Supported models used for gNMI Capabilities RPC

	sync.RWMutex  // The lock for the access of the Model
	stateSyncPath *gtrie.Trie
	transaction   *setTransaction
	updatedroot   *MO // A fake data instance to represent the changed data.
}

// NewModel returns a new Model instance based on gnmi/model/gostruct.
// cn(ChangeNotification) is the callback interface of the gNMI server that is invoked when the data of the Model instance is changed.
// This ChangeNotification interface is used to trigger the telemetry update of the gNMI server.
// sc(StateConfig) is the callback interface produced by the external system.
// The StateConfig interface is invoked when a gNMI Set RPC is issued and applied to the external system.
// ss(StateSync) is the callback interface invoked when a gNMI Get RPC is issued to retrieve the device state.
// The StateSync interface is used to ask the external system to refresh the data in specific paths.
func NewModel(cn ChangeNotification, sc StateConfig, ss StateSync) (*Model, error) {
	return NewCustomModel(gostruct.Schema, gostruct.ΓModelData, cn, sc, ss)
}

// NewCustomModel returns new user-defined Model instance.
// schema func is an user-defined Schema() func generated by ygot.
// modelData is supported models used in gNMI Capabilities RPC.
// cn(ChangeNotification) is the callback interface of the gNMI server that is invoked when the data of the Model instance is changed.
// This ChangeNotification interface is used to trigger the telemetry update of the gNMI server.
// sc(StateConfig) is the callback interface produced by the external system.
// The StateConfig interface is invoked when a gNMI Set RPC is issued and applied to the external system.
// ss(StateSync) is the callback interface invoked when a gNMI Get RPC is issued to retrieve the device state.
// The StateSync interface is used to ask the external system to refresh the data in specific paths.
func NewCustomModel(schema func() (*ytypes.Schema, error), modelData []*gnmipb.ModelData,
	cn ChangeNotification, sc StateConfig, ss StateSync) (*Model, error) {
	s, err := schema()
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	m := &Model{
		MO:                 (*MO)(s),
		schema:             schema,
		modelData:          modelData,
		StateConfig:        sc,
		StateSync:          ss,
		ChangeNotification: cn,
		stateSyncPath:      gtrie.New(),
	}
	if m.StateConfig == nil {
		m.StateConfig = &emptyStateConfig{}
	}
	m.initStateSync(ss)
	return m, nil
}

// Load loads the startup state of the Model.
//  - startup: The YAML or JSON startup data to populate the creating structure (gostruct).
func (m *Model) Load(startup []byte, sync bool) error {
	mo, err := m.NewRoot(startup)
	if err != nil {
		return err
	}
	if sync && m.StateConfig != nil {
		newlist := mo.ListAll(mo.GetRoot(), nil)
		curlist := m.ListAll(m.GetRoot(), nil)
		cur := newDataAndPathMap(curlist)
		new := newDataAndPathMap(newlist)
		// Get difference between cur and new for C/R/D operations
		for p := range cur {
			if _, exists := new[p]; !exists {
				m.StateConfig.UpdateDelete(p)
			}
		}
		for p, entry := range new {
			if _, exists := cur[p]; exists {
				m.StateConfig.UpdateReplace(p, entry.GetValueString())
			} else {
				m.StateConfig.UpdateCreate(p, entry.GetValueString())
			}
		}
	}
	m.MO = mo
	return nil
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

// FindModel finds a model.
func (m *Model) FindModel(name string) (*gnmipb.ModelData, bool) {
	for _, m := range m.modelData {
		if m.GetName() == name {
			return m, true
		}
	}
	return nil, false
}

// CheckModels checks whether models are supported by the model. Return error if anything is unsupported.
func (m *Model) CheckModels(models []*gnmipb.ModelData) error {
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
func (m *Model) GetModelData() []*gnmipb.ModelData {
	return m.modelData
}

// FindAllPaths - finds all XPaths against to the gNMI Path that has wildcard
func (m *Model) FindAllPaths(path *gnmipb.Path) ([]string, bool) {
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
func (m *Model) findAllPaths(sp pathFinder, elems []*gnmipb.PathElem) []pathFinder {
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

// ValidatePathSchema - validates all schema of the gNMI Path.
func (m *Model) ValidatePathSchema(path *gnmipb.Path) bool {
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
		entry = entry.Dir[e.Name]
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
func (m *Model) FindSchemaPaths(path *gnmipb.Path) ([]string, bool) {
	t := m.GetRootType()
	entry := m.FindSchemaByType(t)
	if entry == nil {
		return nil, false
	}
	var elems []*gnmipb.PathElem
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

func (m *Model) findSchemaPath(prefix string, parent *yang.Entry, elems []*gnmipb.PathElem) []string {
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
	entry := parent.Dir[e.Name]
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

func (m *Model) findDataPath(prefix string, parent *yang.Entry, elems []*gnmipb.PathElem) []string {
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
	entry := parent.Dir[e.Name]
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

func (m *Model) findSchemaAndDataPath(path dataAndSchemaPath, parent *yang.Entry, elems []*gnmipb.PathElem) []dataAndSchemaPath {
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
	entry := parent.Dir[e.Name]
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

// UpdateCreate is a function of StateUpdate Interface to add a new value to the path of the Model.
func (m *Model) UpdateCreate(path string, value string) error {
	// fmt.Printf("m.UpdateCreate %v %v {\n", path, value)
	schema := m.RootSchema()
	err := ValWrite(schema, m.GetRoot(), path, value)
	if err == nil {
		if m.updatedroot != nil {
			fakeRoot := m.updatedroot.GetRoot()
			ValWrite(schema, fakeRoot, path, value)
			if m.ChangeNotification != nil {
				m.ChangeNotification.ChangeCreated(path, fakeRoot)
			}
		}
	} else {
		glog.Errorf("%v", err)
	}
	// fmt.Println("}")
	return nil
}

// UpdateReplace is a function of StateUpdate Interface to replace the value in the path of the Model.
func (m *Model) UpdateReplace(path string, value string) error {
	// fmt.Printf("m.UpdateCreate %v %v {\n", path, value)
	schema := m.RootSchema()
	err := ValWrite(schema, m.GetRoot(), path, value)
	if err == nil {
		if m.updatedroot != nil {
			fakeRoot := m.updatedroot.GetRoot()
			ValWrite(schema, fakeRoot, path, value)
			if m.ChangeNotification != nil {
				m.ChangeNotification.ChangeReplaced(path, fakeRoot)
			}
		}
	} else {
		glog.Errorf("%v", err)
	}
	// fmt.Println("}")
	return nil
}

// UpdateDelete is a function of StateUpdate Interface to delete the value in the path of the Model.
func (m *Model) UpdateDelete(path string) error {
	// fmt.Printf("m.UpdateDelete %v {\n", path)
	schema := m.RootSchema()
	err := ValDelete(schema, m.GetRoot(), path)
	if err == nil {
		if m.ChangeNotification != nil {
			m.ChangeNotification.ChangeDeleted(path)
		}
	} else {
		glog.Errorf("%v", err)
	}
	// fmt.Println("}")
	return nil
}

// UpdateStart indicates the start of the Model instance update
func (m *Model) UpdateStart() error {
	m.Lock()
	// updatedroot is used to save the changes of the model data.
	updatedroot, err := m.NewRoot(nil)
	if err != nil {
		glog.Errorf("%v", err)
	}
	m.updatedroot = updatedroot
	if m.ChangeNotification != nil {
		m.ChangeNotification.ChangeStarted(m.updatedroot.GetRoot())
	}
	return nil
}

// UpdateEnd indicates the end of the Model instance update
func (m *Model) UpdateEnd() error {
	if m.ChangeNotification != nil {
		m.ChangeNotification.ChangeCompleted(m.updatedroot.GetRoot())
	}
	m.updatedroot = nil
	m.Unlock()
	return nil
}
