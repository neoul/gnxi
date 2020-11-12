package model

import (
	"flag"
	"fmt"
	"reflect"
	"strconv"
	"time"

	"github.com/golang/glog"
	"github.com/neoul/gnxi/utilities/xpath"
	"github.com/neoul/libydb/go/ydb"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmi/value"
	"github.com/openconfig/goyang/pkg/yang"
	"github.com/openconfig/ygot/ygot"
	"github.com/openconfig/ygot/ytypes"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// var (
// 	defaultSyncRequiredSchemaPath = []string{
// 		"/interfaces/interface/state/counters",
// 		"/interfaces/interface/time-sensitive-networking/state/statistics",
// 		"/interfaces/interface/radio-over-ethernet/state/statistics",
// 	}
// )

type syncRequiredPaths []string

func (srpaths *syncRequiredPaths) String() string {
	return fmt.Sprint(*srpaths)
}

func (srpaths *syncRequiredPaths) Set(value string) error {
	*srpaths = append(*srpaths, value)
	return nil
}

var srpaths syncRequiredPaths
var disableYdbChannel = flag.Bool("disable-ydb", false, "disable YAML Datablock interface")

func init() {
	flag.Var(&srpaths, "sync-required-path", "path required YDB sync operation to update data")
	// flag.Set("sync-required-path", "/interfaces/interface/state/counters")
}

// GetYDB - Get YAML DataBlock
func (m *Model) GetYDB() *ydb.YDB {
	return m.block
}

// Lock - Lock the YDB instance for use.
func (m *Model) Lock() {
	m.block.Lock()
}

// Unlock - Unlock of the YDB instance.
func (m *Model) Unlock() {
	m.block.Unlock()
}

// RLock - Lock the YDB instance for read.
func (m *Model) RLock() {
	m.block.RLock()
}

// RUnlock - Unlock of the YDB instance for read.
func (m *Model) RUnlock() {
	m.block.RUnlock()
}

// Close the connected YDB instance
func (m *Model) Close() {
	m.block.Close()
}

// // ChangeRoot - replaces the root of the Model Data
// func (m *Model) ChangeRoot(root ygot.ValidatedGoStruct) error {
// 	m.dataroot = root
// 	return m.block.ReplaceTarget(root, false)
// }

func buildSyncUpdatePath(entries []*yang.Entry, elems []*gnmipb.PathElem) string {
	entrieslen := len(entries)
	elemslen := len(elems)
	if entrieslen > elemslen {
		for i := elemslen + 1; i < entrieslen; i++ {
			elems = append(elems, &gnmipb.PathElem{Name: entries[i].Name})
		}
		return xpath.PathElemToXPATH(elems)
	}
	return xpath.PathElemToXPATH(elems[:entrieslen])
}

// GetSyncUpdatePath - synchronizes the data in the path
func (m *Model) GetSyncUpdatePath(prefix *gnmipb.Path, paths []*gnmipb.Path) []string {
	syncPaths := make([]string, 0, 8)
	for _, path := range paths {
		// glog.Info(":::SynUpdate:::", xpath.ToXPath(xpath.GNMIFullPath(prefix, path)))
		fullpath := xpath.GNMIFullPath(prefix, path)
		if len(fullpath.GetElem()) > 0 {
			schemaPaths, ok := m.FindSchemaPaths(fullpath)
			if !ok {
				continue
			}
			for _, spath := range schemaPaths {
				requiredPath := m.syncRequired.PrefixSearch(spath)
				for _, rpath := range requiredPath {
					if n, ok := m.syncRequired.Find(rpath); ok {
						entires := n.Meta().([]*yang.Entry)
						if entires != nil {
							syncPaths = append(syncPaths, buildSyncUpdatePath(entires, fullpath.GetElem()))
						}
					}
				}
				if rpath, ok := m.syncRequired.FindLongestMatch(spath); ok {
					if n, ok := m.syncRequired.Find(rpath); ok {
						entires := n.Meta().([]*yang.Entry)
						if entires != nil {
							syncPaths = append(syncPaths, buildSyncUpdatePath(entires, fullpath.GetElem()))
						}
					}
				}
			}
		} else {
			requiredPath := m.syncRequired.PrefixSearch("/")
			syncPaths = append(syncPaths, requiredPath...)
		}
	}
	return syncPaths
}

// RunSyncUpdate - synchronizes & update the data in the path. It locks model data.
func (m *Model) RunSyncUpdate(syncIgnoreTime time.Duration, syncPaths []string) {
	if syncPaths == nil || len(syncPaths) == 0 {
		return
	}
	for _, sp := range syncPaths {
		glog.Infof("sync-update %s", sp)
	}
	m.block.SyncTo(syncIgnoreTime, true, syncPaths...)
}

// WriteTypedValue - Write the TypedValue to the model instance
func (m *Model) WriteTypedValue(path *gnmipb.Path, typedValue *gnmipb.TypedValue) error {
	// var err error
	schema := m.GetSchema()
	base := m.GetRoot()
	tValue, tSchema, err := ytypes.GetOrCreateNode(schema, base, path)
	if err != nil {
		return err
	}
	if tSchema.IsDir() {
		target := tValue.(ygot.GoStruct)
		if err := m.Unmarshal(typedValue.GetJsonIetfVal(), target); err != nil {
			return status.Errorf(codes.InvalidArgument, "unmarshaling json failed: %v", err)
		}
	} else { // (schema.IsLeaf() || schema.IsLeafList())
		err = ytypes.SetNode(schema, base, path, typedValue, &ytypes.InitMissingElements{})
	}
	return err
}

// SetInit initializes the Set transaction.
func (m *Model) SetInit() error {
	if m.transaction != nil {
		return status.Errorf(codes.Unavailable, "Already running")
	}
	m.transaction = startTransaction()
	return nil
}

// SetDone resets the Set transaction.
func (m *Model) SetDone() {
	m.transaction = nil
}

// SetRollback reverts the original configuration.
func (m *Model) SetRollback() {

}

// SetCommit commit the changed configuration.
func (m *Model) SetCommit() error {
	// delete
	for _, opinfo := range m.transaction.delete {
		curlist := m.ListAll(opinfo.curval, nil, &AddFakePrefix{Prefix: opinfo.gpath}, &FindAndSort{})
		fmt.Println("Delete curlist:", curlist)
	}
	// replace (delete and then update)
	for _, opinfo := range m.transaction.replace {
		newlist := m.ListAll(m.GetRoot(), opinfo.gpath, &FindAndSort{})
		curlist := m.ListAll(opinfo.curval, nil, &AddFakePrefix{Prefix: opinfo.gpath}, &FindAndSort{})
		fmt.Println("Replace curlist:", curlist)
		fmt.Println("Replace newlist:", newlist)
	}
	// update
	for _, opinfo := range m.transaction.update {
		newlist := m.ListAll(m.GetRoot(), opinfo.gpath, &FindAndSort{})
		curlist := m.ListAll(opinfo.curval, nil, &AddFakePrefix{Prefix: opinfo.gpath}, &FindAndSort{})
		fmt.Println("Update curlist:", curlist)
		fmt.Println("Update newlist:", newlist)
	}
	m.transaction = nil
	return nil
}

// SetDelete deletes the path from root if the path exists.
func (m *Model) SetDelete(prefix, path *gnmipb.Path) error {
	fullpath := xpath.GNMIFullPath(prefix, path)
	targets, _ := m.Get(fullpath)
	for _, target := range targets {
		targetPath, err := xpath.ToGNMIPath(target.Path)
		if err != nil {
			return status.Errorf(codes.Internal, "conversion-error(%s)", target.Path)
		}
		m.transaction.add(opDelete, &target.Path, targetPath, target.Value, nil)
		if len(targetPath.GetElem()) == 0 {
			// root deletion
			if mo, err := m.NewRoot(nil); err == nil {
				m.MO = mo
			} else {
				return err
			}
		} else {
			if err = ytypes.DeleteNode(m.GetSchema(), m.GetRoot(), targetPath); err != nil {
				return err
			}
		}
	}
	return nil
}

// SetReplace deletes the path from root if the path exists.
func (m *Model) SetReplace(prefix, path *gnmipb.Path, typedValue *gnmipb.TypedValue) error {
	var err error
	fullpath := xpath.GNMIFullPath(prefix, path)
	targets, ok := m.Get(fullpath)
	if ok {
		for _, target := range targets {
			targetPath, err := xpath.ToGNMIPath(target.Path)
			if err != nil {
				return status.Errorf(codes.Internal, "conversion-error(%s)", target.Path)
			}
			m.transaction.add(opReplace, &target.Path, targetPath, target.Value, typedValue)
			err = ytypes.DeleteNode(m.GetSchema(), m.GetRoot(), targetPath)
			if err != nil {
				return err
			}
			err = m.WriteTypedValue(targetPath, typedValue)
			if err != nil {
				return err
			}
		}
		return nil
	}
	tpath := xpath.ToXPath(fullpath)
	m.transaction.add(opReplace, &tpath, fullpath, nil, typedValue)
	err = m.WriteTypedValue(fullpath, typedValue)
	if err != nil {
		return err
	}
	return nil
}

// SetUpdate deletes the path from root if the path exists.
func (m *Model) SetUpdate(prefix, path *gnmipb.Path, typedValue *gnmipb.TypedValue) error {
	var err error
	fullpath := xpath.GNMIFullPath(prefix, path)
	targets, ok := m.Get(fullpath)
	if ok {
		for _, target := range targets {
			targetPath, err := xpath.ToGNMIPath(target.Path)
			if err != nil {
				return status.Errorf(codes.Internal, "conversion-error(%s)", target.Path)
			}
			m.transaction.add(opUpdate, &target.Path, targetPath, target.Value, typedValue)
			err = m.WriteTypedValue(targetPath, typedValue)
			if err != nil {
				return err
			}
		}
		return nil
	}
	tpath := xpath.ToXPath(fullpath)
	m.transaction.add(opUpdate, &tpath, fullpath, nil, typedValue)
	err = m.WriteTypedValue(fullpath, typedValue)
	if err != nil {
		return err
	}
	return nil
}

// SetReplaceOrUpdate validates the replace or update operation to be applied to
// the device, modifies the json tree of the config struct, then calls the
// callback function to apply the operation to the device hardware.
func (m *Model) SetReplaceOrUpdate(jsonTree map[string]interface{}, op gnmipb.UpdateResult_Operation, prefix, path *gnmipb.Path, val *gnmipb.TypedValue) (*gnmipb.UpdateResult, error) {
	// Validate the operation.
	fullPath := xpath.GNMIFullPath(prefix, path)
	emptyNode := reflect.New(m.GetRootType()).Interface()
	var nodeVal interface{}
	nodeStruct, ok := emptyNode.(ygot.ValidatedGoStruct)
	if ok {
		if err := m.Unmarshal(val.GetJsonIetfVal(), nodeStruct); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "unmarshaling json data to config struct fails: %v", err)
		}
		if err := nodeStruct.Validate(); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "config data validation fails: %v", err)
		}
		var err error
		if nodeVal, err = ygot.ConstructIETFJSON(nodeStruct, &ygot.RFC7951JSONConfig{}); err != nil {
			msg := fmt.Sprintf("error in constructing IETF JSON tree from config struct: %v", err)
			// log.Error(msg)
			return nil, status.Error(codes.Internal, msg)
		}
	} else {
		var err error
		if nodeVal, err = value.ToScalar(val); err != nil {
			return nil, status.Errorf(codes.Internal, "cannot convert leaf node to scalar type: %v", err)
		}
	}

	// Update json tree of the device config.
	var curNode interface{} = jsonTree
	schema := m.RootSchema()
	for i, elem := range fullPath.Elem {
		switch node := curNode.(type) {
		case map[string]interface{}:
			// Set node value.
			if i == len(fullPath.Elem)-1 {
				if elem.GetKey() == nil {
					if grpcStatusError := setPathWithoutAttribute(op, node, elem, nodeVal); grpcStatusError != nil {
						return nil, grpcStatusError
					}
					break
				}
				if grpcStatusError := setPathWithAttribute(op, node, elem, nodeVal); grpcStatusError != nil {
					return nil, grpcStatusError
				}
				break
			}

			if curNode, schema = getChildNode(node, schema, elem, true); curNode == nil {
				return nil, status.Errorf(codes.NotFound, "path elem not found: %v", elem)
			}
		case []interface{}:
			return nil, status.Errorf(codes.NotFound, "incompatible path elem: %v", elem)
		default:
			return nil, status.Errorf(codes.Internal, "wrong node type: %T", curNode)
		}
	}
	if reflect.DeepEqual(fullPath, xpath.RootGNMIPath) { // Replace/Update root.
		if op == gnmipb.UpdateResult_UPDATE {
			return nil, status.Error(codes.Unimplemented, "update the root of config tree is unsupported")
		}
		nodeValAsTree, ok := nodeVal.(map[string]interface{})
		if !ok {
			return nil, status.Errorf(codes.InvalidArgument, "expect a tree to replace the root, got a scalar value: %T", nodeVal)
		}
		for k := range jsonTree {
			delete(jsonTree, k)
		}
		for k, v := range nodeValAsTree {
			jsonTree[k] = v
		}
	}
	// newRoot, err := m.toGoStruct(jsonTree)
	// if err != nil {
	// 	return nil, status.Error(codes.Internal, err.Error())
	// }

	// Apply the validated operation to the device.
	// if m.callback != nil {
	// 	if applyErr := execConfigCallback(m.callback, newRoot); applyErr != nil {
	// 		if rollbackErr := execConfigCallback(m.callback, m.dataroot); rollbackErr != nil {
	// 			return nil, status.Errorf(codes.Internal, "error in rollback the failed operation (%v): %v", applyErr, rollbackErr)
	// 		}
	// 		return nil, status.Errorf(codes.Aborted, "error in applying operation to device: %v", applyErr)
	// 	}
	// }
	return &gnmipb.UpdateResult{
		Path: path,
		Op:   op,
	}, nil
}

// func (m *Model) toGoStruct(jsonTree map[string]interface{}) (ygot.ValidatedGoStruct, error) {
// 	jsonDump, err := json.Marshal(jsonTree)
// 	if err != nil {
// 		return nil, fmt.Errorf("error in marshaling IETF JSON tree to bytes: %v", err)
// 	}
// 	root, err := m.NewRoot(jsonDump)
// 	if err != nil {
// 		return nil, fmt.Errorf("error in creating config struct from IETF JSON data: %v", err)
// 	}
// 	return root, nil
// }

// isNIl checks if an interface is nil or its value is nil.
func isNil(i interface{}) bool {
	if i == nil {
		return true
	}
	switch kind := reflect.ValueOf(i).Kind(); kind {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return reflect.ValueOf(i).IsNil()
	default:
		return false
	}
}

// setPathWithAttribute replaces or updates a child node of curNode in the IETF
// JSON config tree, where the child node is indexed by pathElem with attribute.
// The function returns grpc status error if unsuccessful.
func setPathWithAttribute(op gnmipb.UpdateResult_Operation, curNode map[string]interface{}, pathElem *gnmipb.PathElem, nodeVal interface{}) error {
	nodeValAsTree, ok := nodeVal.(map[string]interface{})
	if !ok {
		return status.Errorf(codes.InvalidArgument, "expect nodeVal is a json node of map[string]interface{}, received %T", nodeVal)
	}
	m := getKeyedListEntry(curNode, pathElem, true)
	if m == nil {
		return status.Errorf(codes.NotFound, "path elem not found: %v", pathElem)
	}
	if op == gnmipb.UpdateResult_REPLACE {
		for k := range m {
			delete(m, k)
		}
	}
	for attrKey, attrVal := range pathElem.GetKey() {
		m[attrKey] = attrVal
		if asNum, err := strconv.ParseFloat(attrVal, 64); err == nil {
			m[attrKey] = asNum
		}
		for k, v := range nodeValAsTree {
			if k == attrKey && fmt.Sprintf("%v", v) != attrVal {
				return status.Errorf(codes.InvalidArgument, "invalid config data: %v is a path attribute", k)
			}
		}
	}
	for k, v := range nodeValAsTree {
		m[k] = v
	}
	return nil
}

// setPathWithoutAttribute replaces or updates a child node of curNode in the
// IETF config tree, where the child node is indexed by pathElem without
// attribute. The function returns grpc status error if unsuccessful.
func setPathWithoutAttribute(op gnmipb.UpdateResult_Operation, curNode map[string]interface{}, pathElem *gnmipb.PathElem, nodeVal interface{}) error {
	target, hasElem := curNode[pathElem.Name]
	nodeValAsTree, nodeValIsTree := nodeVal.(map[string]interface{})
	if op == gnmipb.UpdateResult_REPLACE || !hasElem || !nodeValIsTree {
		curNode[pathElem.Name] = nodeVal
		return nil
	}
	targetAsTree, ok := target.(map[string]interface{})
	if !ok {
		return status.Errorf(codes.Internal, "error in setting path: expect map[string]interface{} to update, got %T", target)
	}
	for k, v := range nodeValAsTree {
		targetAsTree[k] = v
	}
	return nil
}

// deleteKeyedListEntry deletes the keyed list entry from node that matches the
// path elem. If the entry is the only one in keyed list, deletes the entire
// list. If the entry is found and deleted, the function returns true. If it is
// not found, the function returns false.
func deleteKeyedListEntry(node map[string]interface{}, elem *gnmipb.PathElem) bool {
	curNode, ok := node[elem.Name]
	if !ok {
		return false
	}

	keyedList, ok := curNode.([]interface{})
	if !ok {
		return false
	}
	for i, n := range keyedList {
		m, ok := n.(map[string]interface{})
		if !ok {
			// log.Errorf("expect map[string]interface{} for a keyed list entry, got %T", n)
			return false
		}
		keyMatching := true
		for k, v := range elem.Key {
			attrVal, ok := m[k]
			if !ok {
				return false
			}
			if v != fmt.Sprintf("%v", attrVal) {
				keyMatching = false
				break
			}
		}
		if keyMatching {
			listLen := len(keyedList)
			if listLen == 1 {
				delete(node, elem.Name)
				return true
			}
			keyedList[i] = keyedList[listLen-1]
			node[elem.Name] = keyedList[0 : listLen-1]
			return true
		}
	}
	return false
}

// getChildNode gets a node's child with corresponding schema specified by path
// element. If not found and createIfNotExist is set as true, an empty node is
// created and returned.
func getChildNode(node map[string]interface{}, schema *yang.Entry, elem *gnmipb.PathElem, createIfNotExist bool) (interface{}, *yang.Entry) {
	var nextSchema *yang.Entry
	var ok bool

	if nextSchema, ok = schema.Dir[elem.Name]; !ok {
		return nil, nil
	}

	var nextNode interface{}
	if elem.GetKey() == nil {
		if nextNode, ok = node[elem.Name]; !ok {
			if createIfNotExist {
				node[elem.Name] = make(map[string]interface{})
				nextNode = node[elem.Name]
			}
		}
		return nextNode, nextSchema
	}

	nextNode = getKeyedListEntry(node, elem, createIfNotExist)
	return nextNode, nextSchema
}

// getKeyedListEntry finds the keyed list entry in node by the name and key of
// path elem. If entry is not found and createIfNotExist is true, an empty entry
// will be created (the list will be created if necessary).
func getKeyedListEntry(node map[string]interface{}, elem *gnmipb.PathElem, createIfNotExist bool) map[string]interface{} {
	curNode, ok := node[elem.Name]
	if !ok {
		if !createIfNotExist {
			return nil
		}

		// Create a keyed list as node child and initialize an entry.
		m := make(map[string]interface{})
		for k, v := range elem.Key {
			m[k] = v
			if vAsNum, err := strconv.ParseFloat(v, 64); err == nil {
				m[k] = vAsNum
			}
		}
		node[elem.Name] = []interface{}{m}
		return m
	}

	// Search entry in keyed list.
	keyedList, ok := curNode.([]interface{})
	if !ok {
		return nil
	}
	for _, n := range keyedList {
		m, ok := n.(map[string]interface{})
		if !ok {
			// log.Errorf("wrong keyed list entry type: %T", n)
			return nil
		}
		keyMatching := true
		// must be exactly match
		for k, v := range elem.Key {
			attrVal, ok := m[k]
			if !ok {
				return nil
			}
			if v != fmt.Sprintf("%v", attrVal) {
				keyMatching = false
				break
			}
		}
		if keyMatching {
			return m
		}
	}
	if !createIfNotExist {
		return nil
	}

	// Create an entry in keyed list.
	m := make(map[string]interface{})
	for k, v := range elem.Key {
		m[k] = v
		if vAsNum, err := strconv.ParseFloat(v, 64); err == nil {
			m[k] = vAsNum
		}
	}
	node[elem.Name] = append(keyedList, m)
	return m
}
