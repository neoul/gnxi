package model

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"

	"github.com/golang/glog"
	"github.com/neoul/gnxi/utilities/status"
	"github.com/neoul/gnxi/utilities/xpath"
	"github.com/neoul/libydb/go/ydb"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/goyang/pkg/yang"
	"github.com/openconfig/ygot/ygot"
	"github.com/openconfig/ygot/ytypes"
	"google.golang.org/grpc/codes"
)

// MO (ModeledObject) for wrapping ygot.GoStruct and schema
type MO ytypes.Schema

// NewMO creates and returns new Modeled Object
func NewMO(schema func() (*ytypes.Schema, error)) (*MO, error) {
	s, err := schema()
	if err != nil {
		return nil, err
	}
	mo := (*MO)(s)
	if err := mo.UpdateType(); err != nil {
		return nil, err
	}
	return (*MO)(s), nil
}

// RootSchema returns the YANG entry schema corresponding to the type
// of the root within the schema.
func (mo *MO) RootSchema() *yang.Entry {
	return mo.SchemaTree[reflect.TypeOf(mo.Root).Elem().Name()]
}

// GetSchema returns the root yang.Entry of the model.
func (mo *MO) GetSchema() *yang.Entry {
	return mo.RootSchema()
}

// GetName returns the name of the MO.
func (mo *MO) GetName() string {
	// schema := (*ytypes.Schema)(mo)
	// rootSchema := schema.RootSchema()
	rootSchema := mo.GetSchema()
	if rootSchema == nil {
		return "unknown"
	}
	return rootSchema.Name
}

// GetRootType returns the reflect.Type of the root
func (mo *MO) GetRootType() reflect.Type {
	return reflect.TypeOf(mo.Root).Elem()
}

// GetRoot returns the Root
func (mo *MO) GetRoot() ygot.ValidatedGoStruct {
	return mo.Root
}

// FindSchemaByType - find the yang.Entry by Type for schema info.
func (mo *MO) FindSchemaByType(t reflect.Type) *yang.Entry {
	if t == reflect.TypeOf(nil) {
		return nil
	}
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
		if t == reflect.TypeOf(nil) {
			return nil
		}
	}
	return mo.SchemaTree[t.Name()]
}

// FindSchema finds the *yang.Entry by value(ygot.GoStruct) for schema info.
func (mo *MO) FindSchema(value interface{}) *yang.Entry {
	return mo.FindSchemaByType(reflect.TypeOf(value))
}

// FindSchemaByXPath finds *yang.Entry by XPath string from root model
func (mo *MO) FindSchemaByXPath(path string) *yang.Entry {
	return mo.FindSchemaByRelativeXPath(mo.Root, path)
}

// FindSchemaByRelativeXPath finds *yang.Entry by XPath string from base(ygot.GoStruct)
func (mo *MO) FindSchemaByRelativeXPath(base interface{}, path string) *yang.Entry {
	bSchema := mo.FindSchema(base)
	if bSchema == nil {
		return nil
	}
	findSchemaByXPath := func(entry *yang.Entry, path string) *yang.Entry {
		slicedPath, err := xpath.ParseStringPath(path)
		if err != nil {
			return nil
		}
		for _, elem := range slicedPath {
			switch v := elem.(type) {
			case string:
				entry = entry.Dir[v]
				if entry == nil {
					return nil
				}
			case map[string]string:
				// skip keys
			default:
				return nil
			}
		}
		return entry
	}
	return findSchemaByXPath(bSchema, path)
}

// FindSchemaByGNMIPath finds the schema(*yang.Entry) using the gNMI Path
func (mo *MO) FindSchemaByGNMIPath(path *gnmipb.Path) *yang.Entry {
	return mo.FindSchemaByRelativeGNMIPath(mo.Root, path)
}

// FindSchemaByRelativeGNMIPath finds the schema(*yang.Entry) using the relative path
func (mo *MO) FindSchemaByRelativeGNMIPath(base interface{}, path *gnmipb.Path) *yang.Entry {
	bSchema := mo.FindSchema(base)
	if bSchema == nil {
		return nil
	}
	if path == nil {
		return bSchema
	}
	findSchema := func(entry *yang.Entry, path *gnmipb.Path) *yang.Entry {
		for _, e := range path.GetElem() {
			entry = entry.Dir[e.GetName()]
			if entry == nil {
				return nil
			}
		}
		return entry
	}
	return findSchema(bSchema, path)
}

// FindOption is an interface that is implemented for the option of MO.Find().
type FindOption interface {
	// IsFindOpt is a marker method for each FindOption.
	IsFindOpt()
}

// AddFakePrefix is a FindOption to add a prefix to DataAndPath.Path.
type AddFakePrefix struct {
	Prefix *gnmipb.Path
}

// IsFindOpt - AddFakePrefix is a FindOption.
func (f *AddFakePrefix) IsFindOpt() {}

func hasAddFakePrefix(opts []FindOption) *gnmipb.Path {
	for _, o := range opts {
		switch v := o.(type) {
		case *AddFakePrefix:
			return v.Prefix
		}
	}
	return nil
}

func replaceAddFakePrefix(opts []FindOption, opt FindOption) []FindOption {
	var ok bool
	for i, o := range opts {
		_, ok := o.(*AddFakePrefix)
		if ok {
			opts[i] = opt
			ok = true
			break
		}
	}
	if !ok {
		opts = append(opts, opt)
	}
	return opts
}

// FindAndSort is used to sort the found result.
type FindAndSort struct{}

// IsFindOpt - FindAndSort is a FindOption.
func (f *FindAndSort) IsFindOpt() {}

func hasFindAndSort(opts []FindOption) bool {
	for _, o := range opts {
		switch o.(type) {
		case *FindAndSort:
			return true
		}
	}
	return false
}

// Get - Get all values and paths (XPath, Value) from the root
func (mo *MO) Get(path *gnmipb.Path) ([]*DataAndPath, bool) {
	return mo.Find(mo.GetRoot(), path)
}

// Find - Find all values and paths (XPath, Value) from the base ygot.GoStruct
func (mo *MO) Find(base interface{}, path *gnmipb.Path, opts ...FindOption) ([]*DataAndPath, bool) {
	t := reflect.TypeOf(base)
	entry := mo.FindSchemaByType(t)
	if entry == nil {
		return []*DataAndPath{}, false
	}
	fprefix := hasAddFakePrefix(opts)
	if fprefix == nil {
		fprefix = xpath.EmptyGNMIPath
	}
	prefix := xpath.ToXPath(fprefix)

	elems := path.GetElem()
	if len(elems) <= 0 {
		dataAndGNMIPath := &DataAndPath{
			Value: base, Path: prefix,
		}
		return []*DataAndPath{dataAndGNMIPath}, true
	}
	for _, e := range elems {
		if e.Name == "*" || e.Name == "..." {
			break
		}
		entry = entry.Dir[e.Name]
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
	v := reflect.ValueOf(base)
	datapath := &dataAndPath{Value: v, Key: []string{""}}
	founds := findAllData(datapath, elems, opts...)
	num := len(founds)
	if num <= 0 {
		return []*DataAndPath{}, false
	}

	rvalues := make([]*DataAndPath, 0, num)
	for _, each := range founds {
		if each.Value.CanInterface() {
			var p string
			if prefix == "/" {
				p = strings.Join(each.Key, "/")
			} else {
				p = prefix + strings.Join(each.Key, "/")
			}
			dataAndGNMIPath := &DataAndPath{
				Value: each.Value.Interface(),
				Path:  p,
			}
			rvalues = append(rvalues, dataAndGNMIPath)
		}
	}
	if hasFindAndSort(opts) {
		sort.Slice(rvalues, func(i, j int) bool {
			return rvalues[i].Path < rvalues[j].Path
		})
	}
	return rvalues, true
}

// ListAll find and list all child values.
func (mo *MO) ListAll(base interface{}, path *gnmipb.Path, opts ...FindOption) []*DataAndPath {
	var targetNodes []*DataAndPath
	var children []*DataAndPath
	if path == nil {
		targetNodes, _ = mo.Find(base, xpath.WildcardGNMIPathDot3, opts...)
		return targetNodes
	}

	targetNodes, _ = mo.Find(base, path, opts...)
	for _, targetNode := range targetNodes {
		switch v := targetNode.Value.(type) {
		case ygot.GoStruct:
			tpath, _ := xpath.ToGNMIPath(targetNode.Path)
			opts = replaceAddFakePrefix(opts, &AddFakePrefix{Prefix: tpath})
			allNodes, _ := mo.Find(v, xpath.WildcardGNMIPathDot3, opts...)
			for _, node := range allNodes {
				children = append(children, node)
			}
		default:
			children = append(children, targetNode)
		}
	}
	return children
}

// Encoding defines the value encoding formats that are supported by the model
type Encoding int32

const (
	Encoding_JSON Encoding = 0 // JSON encoded text.
	// Encoding_BYTES     Encoding = 1 // Arbitrarily encoded bytes.
	// Encoding_PROTO     Encoding = 2 // Encoded according to out-of-band agreed Protobuf.
	// Encoding_ASCII     Encoding = 3 // ASCII text of an out-of-band agreed format.
	Encoding_JSON_IETF Encoding = 4 // JSON encoded text as per RFC7951.
	Encoding_YAML      Encoding = 5
)

// Enum value maps for Encoding.
var (
	encoding_name = map[int32]string{
		0: "JSON",
		// 1: "BYTES",
		// 2: "PROTO",
		// 3: "ASCII",
		4: "JSON_IETF",
		5: "YAML",
	}
	encoding_value = map[string]int32{
		"JSON": 0,
		// "BYTES":     1,
		// "PROTO":     2,
		// "ASCII":     3,
		"JSON_IETF": 4,
		"YAML":      5,
	}
)

func (x Encoding) String() string {
	return encoding_name[int32(x)]
}

// NewRoot returns new MO (ModeledObject).
// Use NewEmptyRoot() instead of the function if there is not startup configuration.
func (mo *MO) NewRoot(startup []byte, encoding Encoding) (*MO, error) {
	vgs := reflect.New(mo.GetRootType()).Interface()
	root := vgs.(ygot.ValidatedGoStruct)
	newMO := &MO{
		Root:       root,
		SchemaTree: mo.SchemaTree,
		Unmarshal:  mo.Unmarshal,
	}
	if startup == nil {
		return newMO, nil
	}
	switch encoding {
	case Encoding_YAML:
		db, close := ydb.Open("_startup")
		defer close()
		if err := db.Parse(startup); err != nil {
			return nil, status.TaggedErrorf(codes.InvalidArgument, status.TagBadData,
				"invalid startup data:: yaml: %v", err)
		}
		if err := db.Convert(newMO); err != nil {
			return nil, status.TaggedErrorf(codes.Internal, status.TagOperationFail,
				"startup converting error:: %v", err)
		}
	case Encoding_JSON:
		if err := newMO.UnmarshalInternalJSON(nil, startup); err != nil {
			return nil, status.TaggedErrorf(codes.InvalidArgument, status.TagBadData,
				"invalid startup data:: internal-json: %v", err)
		}
	case Encoding_JSON_IETF:
		fallthrough
	default:
		if err := newMO.Unmarshal(startup, newMO.GetRoot()); err != nil {
			return nil, status.TaggedErrorf(codes.InvalidArgument, status.TagBadData,
				"invalid startup data:: json: %v", err)
		}
	}
	// [FIXME] - error in creating gnmid: /device/interfaces: /device/interfaces/interface: list interface contains more than max allowed elements: 2 > 0
	// if err := root.Validate(); err != nil {
	// 	return nil, err
	// }
	return newMO, nil
}

// NewEmptyRoot returns new MO (ModeledObject)
func (mo *MO) NewEmptyRoot() *MO {
	vgs := reflect.New(mo.GetRootType()).Interface()
	root := vgs.(ygot.ValidatedGoStruct)
	return &MO{
		Root:       root,
		SchemaTree: mo.SchemaTree,
		Unmarshal:  mo.Unmarshal,
	}
}

// ExportToJSON returns json bytes of the MO
func (mo *MO) ExportToJSON(rfc7951json bool) ([]byte, error) {
	var err error
	var jm map[string]interface{}
	if rfc7951json {
		jm, err = ygot.ConstructIETFJSON(mo.GetRoot(), &ygot.RFC7951JSONConfig{AppendModuleName: true})
	} else {
		jm, err = ygot.ConstructInternalJSON(mo.GetRoot())
	}
	if err != nil {
		return nil, status.TaggedErrorf(codes.Internal, status.TagBadData, "json-constructing error:: %v", err)
	}
	b, err := json.MarshalIndent(jm, "", " ")
	if err != nil {
		return nil, status.TaggedErrorf(codes.Internal, status.TagBadData, "json-marshaling error:: %v", err)

	}
	return b, nil
}

// Export returns json map[string]interface{} of the MO
func (mo *MO) Export(rfc7951json bool) (map[string]interface{}, error) {
	var err error
	var jm map[string]interface{}
	if rfc7951json {
		jm, err = ygot.ConstructIETFJSON(mo.GetRoot(), &ygot.RFC7951JSONConfig{AppendModuleName: true})
	} else {
		jm, err = ygot.ConstructInternalJSON(mo.GetRoot())
	}
	if err != nil {
		return nil, status.TaggedErrorf(codes.Internal, status.TagBadData, "json-constructing error:: %v", err)
	}
	return jm, nil
}

// UpdateCreate is a function of StateUpdate Interface to add a new value to the path of the MO.
func (mo *MO) UpdateCreate(path string, value string) error {
	gpath, err := xpath.ToGNMIPath(path)
	if err != nil {
		if glog.V(10) {
			glog.Errorf("model.create:: %v in %s", err, path)
		}
		return nil
	}
	err = mo.WriteStringValue(gpath, value)
	if err != nil {
		if glog.V(10) {
			glog.Errorf("mo.create:: %v in %s", err, path)
		}
	}
	return nil
}

// UpdateReplace is a function of StateUpdate Interface to replace the value in the path of the MO.
func (mo *MO) UpdateReplace(path string, value string) error {
	gpath, err := xpath.ToGNMIPath(path)
	if err != nil {
		if glog.V(10) {
			glog.Errorf("model.replace:: %v in %s", err, path)
		}
		return nil
	}
	err = mo.WriteStringValue(gpath, value)
	if err != nil {
		if glog.V(10) {
			glog.Errorf("mo.replace:: %v in %s", err, path)
		}
	}
	return nil
}

// UpdateDelete is a function of StateUpdate Interface to delete the value in the path of the MO.
func (mo *MO) UpdateDelete(path string) error {
	gpath, err := xpath.ToGNMIPath(path)
	if err != nil {
		if glog.V(10) {
			glog.Errorf("model.replace:: %v in %s", err, path)
		}
		return nil
	}
	err = mo.DeleteValue(gpath)
	if err != nil {
		if glog.V(10) {
			glog.Errorf("mo.delete:: %v in %s", err, path)
		}
	}
	return nil
}
