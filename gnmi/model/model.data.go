package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"time"

	"github.com/golang/glog"
	"github.com/neoul/gnxi/utilities"
	"github.com/neoul/gnxi/utilities/xpath"
	"github.com/neoul/libydb/go/ydb"
	"github.com/neoul/trie"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmi/value"
	"github.com/openconfig/goyang/pkg/yang"
	"github.com/openconfig/ygot/experimental/ygotutils"
	"github.com/openconfig/ygot/ygot"
	cpb "google.golang.org/genproto/googleapis/rpc/code"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	pbRootPath                    = &gpb.Path{}
	defaultSyncRequiredSchemaPath = []string{
		"/interfaces/interface/state/counters",
		"/interfaces/interface/time-sensitive-networking/state/statistics",
		"/interfaces/interface/radio-over-ethernet/state/statistics",
	}
)

// ModelData - the data instance for the model
type ModelData struct {
	dataroot     ygot.ValidatedGoStruct // the current data tree of the Model
	updatedroot  ygot.GoStruct          // a fake data tree to represent the changed data.
	callback     DataCallback
	block        *ydb.YDB
	syncRequired *trie.Trie
	model        *Model
}

// Lock - Lock the YDB instance for use.
func (mdata *ModelData) Lock() {
	mdata.block.Lock()
}

// Unlock - Unlock of the YDB instance.
func (mdata *ModelData) Unlock() {
	mdata.block.Unlock()
}

// RLock - Lock the YDB instance for read.
func (mdata *ModelData) RLock() {
	mdata.block.RLock()
}

// RUnlock - Unlock of the YDB instance for read.
func (mdata *ModelData) RUnlock() {
	mdata.block.RUnlock()
}

// Close the connected YDB instance
func (mdata *ModelData) Close() {
	mdata.block.Close()
}

// NewGoStruct - creates a ValidatedGoStruct of this model from jsonData. If jsonData is nil, creates an empty GoStruct.
func NewGoStruct(m *Model, jsonData []byte) (ygot.ValidatedGoStruct, error) {
	rootNode, stat := ygotutils.NewNode(m.StructRootType, &gpb.Path{})
	if stat.GetCode() != int32(cpb.Code_OK) {
		return nil, fmt.Errorf("cannot create root node: %d: %s", stat.GetCode(), stat.GetMessage())
	}

	root, ok := rootNode.(ygot.ValidatedGoStruct)
	if !ok {
		return nil, errors.New("root node is not a ygot.ValidatedGoStruct")
	}
	if jsonData != nil {
		if err := m.JSONUnmarshaler(jsonData, root); err != nil {
			return nil, err
		}
		if err := root.Validate(); err != nil {
			return nil, err
		}
	}
	return root, nil
}

// NewModelData creates a ValidatedGoStruct of this model from jsonData. If jsonData is nil, creates an empty GoStruct.
func NewModelData(m *Model, jsonData []byte, yamlData []byte, callback DataCallback, disableYdbChannel bool) (*ModelData, error) {
	root, err := NewGoStruct(m, jsonData)
	if err != nil {
		return nil, err
	}

	mdata := &ModelData{
		dataroot:     root,
		callback:     callback,
		model:        m,
		syncRequired: trie.New(),
	}
	for _, p := range defaultSyncRequiredSchemaPath {
		entry, err := m.FindSchemaByPath(p)
		if err != nil {
			continue
		}
		sentries := []*yang.Entry{}
		for entry != nil {
			sentries = append([]*yang.Entry{entry}, sentries...)
			entry = entry.Parent
		}
		mdata.syncRequired.Add(p, sentries)
	}

	if jsonData != nil {
		if err := execConfigCallback(mdata.callback, root); err != nil {
			return nil, err
		}
	}

	mdata.block, _ = ydb.OpenWithTargetStruct("gnmi_target", mdata)
	if yamlData != nil {
		if err := mdata.block.Parse(yamlData); err != nil {
			return nil, err
		}
		if err := root.Validate(); err != nil {
			return nil, err
		}
		if err := execConfigCallback(mdata.callback, root); err != nil {
			return nil, err
		}
		utilities.PrintStruct(root)
	}

	if !disableYdbChannel {
		err := mdata.block.Connect("uss://openconfig", "pub")
		if err != nil {
			mdata.block.Close()
			return nil, err
		}
		mdata.block.Serve()
	}

	return mdata, nil
}

// GetRoot - replaces the root of the Model Data
func (mdata *ModelData) GetRoot() ygot.ValidatedGoStruct {
	return mdata.dataroot
}

// ChangeRoot - replaces the root of the Model Data
func (mdata *ModelData) ChangeRoot(root ygot.ValidatedGoStruct) error {
	mdata.dataroot = root
	return mdata.block.RelaceTargetStruct(root, false)
}

func buildSyncUpdatePath(entries []*yang.Entry, elems []*gpb.PathElem) string {
	entrieslen := len(entries)
	elemslen := len(elems)
	if entrieslen > elemslen {
		for i := elemslen + 1; i < entrieslen; i++ {
			elems = append(elems, &gpb.PathElem{Name: entries[i].Name})
		}
		return xpath.PathElemToXPATH(elems)
	}
	return xpath.PathElemToXPATH(elems[:entrieslen])
}

// GetSyncUpdatePath - synchronizes the data in the path
func (mdata *ModelData) GetSyncUpdatePath(prefix *gpb.Path, paths []*gpb.Path) []string {
	m := mdata.model
	entry := m.SchemaTreeRoot
	syncPaths := make([]string, 0, 8)
	for _, path := range paths {
		// glog.Info(":::SynUpdate:::", xpath.ToXPATH(utilities.GNMIFullPath(prefix, path)))
		var elems []*gpb.PathElem
		if prefix != nil {
			elems = append(prefix.GetElem(), path.GetElem()...)
		} else {
			elems = path.GetElem()
		}
		if len(elems) > 0 {
			schemaPaths := m.findSchemaPath("", entry, elems)
			for _, spath := range schemaPaths {
				requiredPath := mdata.syncRequired.PrefixSearch(spath)
				for _, rpath := range requiredPath {
					if n, ok := mdata.syncRequired.Find(rpath); ok {
						entires := n.Meta().([]*yang.Entry)
						if entires != nil {
							syncPaths = append(syncPaths, buildSyncUpdatePath(entires, elems))
						}
					}
				}
				if rpath, ok := mdata.syncRequired.FindLongestMatch(spath); ok {
					if n, ok := mdata.syncRequired.Find(rpath); ok {
						entires := n.Meta().([]*yang.Entry)
						if entires != nil {
							syncPaths = append(syncPaths, buildSyncUpdatePath(entires, elems))
						}
					}
				}
			}
		} else {
			requiredPath := mdata.syncRequired.PrefixSearch("/")
			syncPaths = append(syncPaths, requiredPath...)
		}
	}
	return syncPaths
}

// RunSyncUpdate - synchronizes & update the data in the path. It locks model data.
func (mdata *ModelData) RunSyncUpdate(syncIgnoreTime time.Duration, syncPaths []string) {
	if syncPaths == nil || len(syncPaths) == 0 {
		return
	}
	for _, sp := range syncPaths {
		glog.Infof("sync-update %s", sp)
	}
	mdata.block.SyncTo(syncIgnoreTime, true, syncPaths...)
}

// SetDelete deletes the path from the json tree if the path exists. If success,
// it calls the callback function to apply the change to the device hardware.
func (mdata *ModelData) SetDelete(jsonTree map[string]interface{}, prefix, path *gpb.Path) (*gpb.UpdateResult, error) {
	// Update json tree of the device config
	var curNode interface{} = jsonTree
	pathDeleted := false
	fullPath := utilities.GNMIFullPath(prefix, path)
	schema := mdata.model.SchemaTreeRoot
	for i, elem := range fullPath.Elem { // Delete sub-tree or leaf node.
		node, ok := curNode.(map[string]interface{})
		if !ok {
			break
		}

		// Delete node
		if i == len(fullPath.Elem)-1 {
			if elem.GetKey() == nil {
				delete(node, elem.Name)
				pathDeleted = true
				break
			}
			pathDeleted = deleteKeyedListEntry(node, elem)
			break
		}

		if curNode, schema = getChildNode(node, schema, elem, false); curNode == nil {
			break
		}
	}

	if reflect.DeepEqual(fullPath, pbRootPath) { // Delete root
		for k := range jsonTree {
			delete(jsonTree, k)
		}
	}

	// Apply the validated operation to the config tree and device.
	if pathDeleted {
		newRoot, err := mdata.toGoStruct(jsonTree)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		if mdata.callback != nil {
			if applyErr := execConfigCallback(mdata.callback, newRoot); applyErr != nil {
				if rollbackErr := execConfigCallback(mdata.callback, mdata.dataroot); rollbackErr != nil {
					return nil, status.Errorf(codes.Internal, "error in rollback the failed operation (%v): %v", applyErr, rollbackErr)
				}
				return nil, status.Errorf(codes.Aborted, "error in applying operation to device: %v", applyErr)
			}
		}
	}
	return &gpb.UpdateResult{
		Path: path,
		Op:   gpb.UpdateResult_DELETE,
	}, nil
}

// SetReplaceOrUpdate validates the replace or update operation to be applied to
// the device, modifies the json tree of the config struct, then calls the
// callback function to apply the operation to the device hardware.
func (mdata *ModelData) SetReplaceOrUpdate(jsonTree map[string]interface{}, op gpb.UpdateResult_Operation, prefix, path *gpb.Path, val *gpb.TypedValue) (*gpb.UpdateResult, error) {
	// Validate the operation.
	fullPath := utilities.GNMIFullPath(prefix, path)
	emptyNode, stat := ygotutils.NewNode(mdata.model.StructRootType, fullPath)
	if stat.GetCode() != int32(cpb.Code_OK) {
		return nil, status.Errorf(codes.NotFound, "path %v is not found in the config structure: %d %s", fullPath, stat.GetCode(), stat.GetMessage())
	}
	var nodeVal interface{}
	nodeStruct, ok := emptyNode.(ygot.ValidatedGoStruct)
	if ok {
		if err := mdata.model.JSONUnmarshaler(val.GetJsonIetfVal(), nodeStruct); err != nil {
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
	schema := mdata.model.SchemaTreeRoot
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
	if reflect.DeepEqual(fullPath, pbRootPath) { // Replace/Update root.
		if op == gpb.UpdateResult_UPDATE {
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
	newRoot, err := mdata.toGoStruct(jsonTree)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Apply the validated operation to the device.
	if mdata.callback != nil {
		if applyErr := execConfigCallback(mdata.callback, newRoot); applyErr != nil {
			if rollbackErr := execConfigCallback(mdata.callback, mdata.dataroot); rollbackErr != nil {
				return nil, status.Errorf(codes.Internal, "error in rollback the failed operation (%v): %v", applyErr, rollbackErr)
			}
			return nil, status.Errorf(codes.Aborted, "error in applying operation to device: %v", applyErr)
		}
	}
	return &gpb.UpdateResult{
		Path: path,
		Op:   op,
	}, nil
}

func (mdata *ModelData) toGoStruct(jsonTree map[string]interface{}) (ygot.ValidatedGoStruct, error) {
	jsonDump, err := json.Marshal(jsonTree)
	if err != nil {
		return nil, fmt.Errorf("error in marshaling IETF JSON tree to bytes: %v", err)
	}
	root, err := NewGoStruct(mdata.model, jsonDump)
	if err != nil {
		return nil, fmt.Errorf("error in creating config struct from IETF JSON data: %v", err)
	}
	return root, nil
}

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
func setPathWithAttribute(op gpb.UpdateResult_Operation, curNode map[string]interface{}, pathElem *gpb.PathElem, nodeVal interface{}) error {
	nodeValAsTree, ok := nodeVal.(map[string]interface{})
	if !ok {
		return status.Errorf(codes.InvalidArgument, "expect nodeVal is a json node of map[string]interface{}, received %T", nodeVal)
	}
	m := getKeyedListEntry(curNode, pathElem, true)
	if m == nil {
		return status.Errorf(codes.NotFound, "path elem not found: %v", pathElem)
	}
	if op == gpb.UpdateResult_REPLACE {
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
func setPathWithoutAttribute(op gpb.UpdateResult_Operation, curNode map[string]interface{}, pathElem *gpb.PathElem, nodeVal interface{}) error {
	target, hasElem := curNode[pathElem.Name]
	nodeValAsTree, nodeValIsTree := nodeVal.(map[string]interface{})
	if op == gpb.UpdateResult_REPLACE || !hasElem || !nodeValIsTree {
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
func deleteKeyedListEntry(node map[string]interface{}, elem *gpb.PathElem) bool {
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
func getChildNode(node map[string]interface{}, schema *yang.Entry, elem *gpb.PathElem, createIfNotExist bool) (interface{}, *yang.Entry) {
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
func getKeyedListEntry(node map[string]interface{}, elem *gpb.PathElem, createIfNotExist bool) map[string]interface{} {
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
