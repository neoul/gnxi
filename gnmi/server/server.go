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

// Package server implements a gnmi server to mock a device with YANG models.
package server

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"sync"
	"time"

	"github.com/neoul/gnxi/utilities/status"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"

	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"
	"github.com/neoul/gnxi/gnmi/model"
	"github.com/neoul/gnxi/gnmi/model/gostruct"
	"github.com/neoul/gnxi/utilities/xpath"
	"github.com/neoul/libydb/go/ydb"
	"github.com/openconfig/ygot/ygot"
	"github.com/openconfig/ygot/ytypes"

	dpb "github.com/golang/protobuf/protoc-gen-go/descriptor"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
)

var (
	supportedEncodings = []gnmipb.Encoding{gnmipb.Encoding_JSON, gnmipb.Encoding_JSON_IETF}
)

// Server struct maintains the data structure for device state and config and implements
// the interface of gNMI server. It supports Capabilities, Get, Set and Subscribe RPCs.
type Server struct {
	*model.Model
	*teleCtrl
	disableBundling bool
	sessions        map[string]*Session
	serverAliases   map[string]string // target-defined aliases (server aliases)
	enabledAliases  bool              // whether server aliases is enabled
	iStateUpdate    *ydb.YDB          // internal StateUpdate interface
	reqSeq          uint64
}

// Option is an interface used in the gNMI Server configuration
type Option interface {
	// IsOption is a marker method for each Option.
	IsOption()
}

// DisableBundling is used to disable Bundling of Telemetry Updates defined in gNMI Specification 3.5.2.1
type DisableBundling struct{}

// IsOption - DisableBundling is a Option.
func (o DisableBundling) IsOption() {}

func hasDisableBundling(opts []Option) bool {
	for _, o := range opts {
		switch o.(type) {
		case DisableBundling:
			return true
		}
	}
	return false
}

// Startup is JSON or YAML bytes to be loaded at startup.
type Startup []byte

// IsOption - Startup is a Option.
func (o Startup) IsOption() {}

func hasStartup(opts []Option) []byte {
	for _, o := range opts {
		switch v := o.(type) {
		case Startup:
			return []byte(v)
		}
	}
	return nil
}

// SetCallback includes a callback interface to configure or execute
// gNMI Set to the system. The callback interface consists of a set of
// the following functions that must be implemented by the system.
//
// 	UpdateStart() error // Set starts.
// 	UpdateCreate(path string, value string) error // Set creates new config data.
// 	UpdateReplace(path string, value string) error // Set replaces config data.
// 	UpdateDelete(path string) error // Set deletes config data.
// 	UpdateEnd() error // Set ends.
type SetCallback struct {
	model.StateConfig
}

// IsOption - SetCallback is a Option.
func (o SetCallback) IsOption() {}

func hasSetCallback(opts []Option) model.StateConfig {
	for _, o := range opts {
		switch v := o.(type) {
		case SetCallback:
			return v.StateConfig
		}
	}
	return nil
}

// GetCallback includes a callback interface to get the config state data
// from the system immediately. The callback interface consists of
// a following function that must be implemented by the system.
//
// 	UpdateSync(path ...string) error
type GetCallback struct {
	model.StateSync
}

// IsOption - GetCallback is a Option.
func (o GetCallback) IsOption() {}

func hasGetCallback(opts []Option) model.StateSync {
	for _, o := range opts {
		switch v := o.(type) {
		case GetCallback:
			return v.StateSync
		}
	}
	return nil
}

// Aliases (Target-defined aliases configuration within a Subscription)
type Aliases map[string]string

// IsOption - Aliases is a Option.
func (o Aliases) IsOption() {}

func hasAliases(opts []Option) map[string]string {
	for _, o := range opts {
		switch a := o.(type) {
		case Aliases:
			for k, v := range a {
				glog.Infof("server-aliases(%s: %s)\n", k, v)
			}
			return map[string]string(a)
		}
	}
	return nil
}

// NewServer creates an instance of Server with given json config.
func NewServer(opts ...Option) (*Server, error) {
	return NewCustomServer(gostruct.Schema, gostruct.ΓModelData, opts...)
}

// NewCustomServer creates an instance of Server with given json config.
func NewCustomServer(schema func() (*ytypes.Schema, error), supportedModels []*gnmipb.ModelData, opts ...Option) (*Server, error) {
	var err error
	var m *model.Model
	s := &Server{
		disableBundling: hasDisableBundling(opts),
		serverAliases:   hasAliases(opts),
		sessions:        map[string]*Session{},
		teleCtrl:        newTeleCtrl(),
		iStateUpdate:    ydb.New("gnmi.iStateUpdate"),
	}

	m, err = model.NewCustomModel(schema, supportedModels, s, hasSetCallback(opts), hasGetCallback(opts))
	if err != nil {
		return nil, err
	}
	s.Model = m
	if startup := hasStartup(opts); startup != nil {
		m.Load(startup, true)
	}
	s.iStateUpdate.EnableAtomicUpdate(true)
	s.iStateUpdate.SetTarget(m, false)
	return s, nil
}

// Load loads the startup state of the Server Model.
// startup is YAML or JSON startup data to populate the creating structure (gostruct).
func (s *Server) Load(startup []byte) error {
	return s.Model.Load(startup, true)
}

// CheckEncoding checks whether encoding and models are supported by the server. Return error if anything is unsupported.
func (s *Server) CheckEncoding(encoding gnmipb.Encoding) error {
	hasSupportedEncoding := false
	for _, supportedEncoding := range supportedEncodings {
		if encoding == supportedEncoding {
			hasSupportedEncoding = true
			break
		}
	}
	if !hasSupportedEncoding {
		return status.TaggedErrorf(codes.Unimplemented, status.TagNotSupport,
			"unsupported encoding: %s", gnmipb.Encoding_name[int32(encoding)])
	}
	return nil
}

// getGNMIServiceVersion returns a pointer to the gNMI service version string.
// The method is non-trivial because of the way it is defined in the proto file.
func getGNMIServiceVersion() (*string, error) {
	gzB, _ := (&gnmipb.Update{}).Descriptor()
	r, err := gzip.NewReader(bytes.NewReader(gzB))
	if err != nil {
		return nil, fmt.Errorf("error in initializing gzip reader: %v", err)
	}
	defer r.Close()
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("error in reading gzip data: %v", err)
	}
	desc := &dpb.FileDescriptorProto{}
	if err := proto.Unmarshal(b, desc); err != nil {
		return nil, fmt.Errorf("error in unmarshaling proto: %v", err)
	}
	ver, err := proto.GetExtension(desc.Options, gnmipb.E_GnmiService)
	if err != nil {
		return nil, fmt.Errorf("error in getting version from proto extension: %v", err)
	}
	return ver.(*string), nil
}

func (s *Server) getReqSequence() uint64 {
	s.reqSeq++
	return s.reqSeq
}

// Capabilities returns supported encodings and supported models.
func (s *Server) Capabilities(ctx context.Context, req *gnmipb.CapabilityRequest) (*gnmipb.CapabilityResponse, error) {
	seq := s.getReqSequence()
	glog.V(1).Infof("Capabilities.request[%d]::\n%s", seq, proto.MarshalTextString(req))
	resp, err := s.capabilities(ctx, req)
	if err != nil {
		glog.Errorf("Capabilities.response[%d]:: %v", seq, err)
	} else {
		glog.V(1).Infof("Capabilities.response[%d]::\n%s", seq, proto.MarshalTextString(resp))
	}
	return resp, err
}

// Get implements the Get RPC in gNMI spec.
func (s *Server) Get(ctx context.Context, req *gnmipb.GetRequest) (*gnmipb.GetResponse, error) {
	seq := s.getReqSequence()
	glog.V(1).Infof("Get.request[%d]::\n%s", seq, proto.MarshalTextString(req))
	resp, err := s.get(ctx, req)
	if err != nil {
		glog.Errorf("Get.response[%d]:: %v", seq, err)
	} else {
		glog.V(1).Infof("Get.response[%d]::\n%s", seq, proto.MarshalTextString(resp))
	}
	return resp, err
}

// Set implements the Set RPC in gNMI spec.
func (s *Server) Set(ctx context.Context, req *gnmipb.SetRequest) (*gnmipb.SetResponse, error) {
	seq := s.getReqSequence()
	glog.V(1).Infof("Set.request[%d]::\n%s", seq, proto.MarshalTextString(req))
	resp, err := s.set(ctx, req)
	if err != nil {
		glog.Errorf("Set.response[%d]:: %v", seq, err)
	} else {
		glog.V(1).Infof("Set.response[%d]::\n%s", seq, proto.MarshalTextString(resp))
	}
	return resp, err
}

// Subscribe implements the Subscribe RPC in gNMI spec.
func (s *Server) Subscribe(stream gnmipb.GNMI_SubscribeServer) error {
	return s.subscribe(stream)
}

// Capabilities returns supported encodings and supported models.
func (s *Server) capabilities(ctx context.Context, req *gnmipb.CapabilityRequest) (*gnmipb.CapabilityResponse, error) {
	ver, err := getGNMIServiceVersion()
	if err != nil {
		return nil, status.TaggedErrorf(codes.Internal, status.TagOperationFail,
			"gnmi service version error: %v", err)
	}
	return &gnmipb.CapabilityResponse{
		SupportedModels:    s.Model.GetModelData(),
		SupportedEncodings: supportedEncodings,
		GNMIVersion:        *ver,
	}, nil
}

// Get implements the Get RPC in gNMI spec.
func (s *Server) get(ctx context.Context, req *gnmipb.GetRequest) (*gnmipb.GetResponse, error) {
	if req.GetType() != gnmipb.GetRequest_ALL {
		return nil, status.TaggedErrorf(codes.Unimplemented, status.TagNotSupport,
			"unsupported request type: %s", gnmipb.GetRequest_DataType_name[int32(gnmipb.GetRequest_ALL)])
	}
	if err := s.Model.CheckModels(req.GetUseModels()); err != nil {
		return nil, err
	}
	if err := s.CheckEncoding(req.GetEncoding()); err != nil {
		return nil, err
	}

	// fmt.Println(proto.MarshalTextString(req))

	prefix := req.GetPrefix()
	paths := req.GetPath()
	s.Model.RequestStateSync(prefix, paths)

	s.Model.RLock()
	defer s.Model.RUnlock()

	// each prefix + path ==> one notification message
	if err := xpath.ValidateGNMIPath(prefix); err != nil {
		return nil, status.TaggedErrorf(codes.InvalidArgument, status.TagInvalidPath,
			"prefix validation failed: %s", err)
	}
	toplist, ok := s.Model.Find(s.Model.GetRoot(), prefix)
	if !ok || len(toplist) <= 0 {
		if ok = s.Model.ValidatePathSchema(prefix); ok {
			return nil, status.TaggedErrorf(codes.NotFound, status.TagDataMissing,
				"no data found in %v", xpath.ToXPath(prefix))
		}
		return nil, status.TaggedErrorf(codes.NotFound, status.TagUnknownPath,
			"unable to find %s from the schema tree", xpath.ToXPath(prefix))
	}
	notifications := []*gnmipb.Notification{}
	for _, top := range toplist {
		bpath := top.Path
		branch := top.Value.(ygot.GoStruct)
		bprefix, err := xpath.ToGNMIPath(bpath)
		if err != nil {
			return nil, status.TaggedErrorf(codes.Internal, status.TagInvalidPath,
				"xpath-to-gpath converting error for %s", bpath)
		}
		for _, path := range paths {
			if err := xpath.ValidateGNMIFullPath(prefix, path); err != nil {
				return nil, status.TaggedErrorf(codes.Unimplemented, status.TagInvalidPath,
					"full path (%s + %s) validation failed: %v",
					xpath.ToXPath(prefix), xpath.ToXPath(path), err)
			}
			datalist, ok := s.Model.Find(branch, path)
			if !ok || len(datalist) <= 0 {
				// [FIXME] Should it be failed?
				fullpath := xpath.GNMIFullPath(prefix, path)
				if ok = s.Model.ValidatePathSchema(fullpath); !ok {
					return nil, status.TaggedErrorf(codes.NotFound, status.TagUnknownPath,
						"unable to find %s from the schema tree", xpath.ToXPath(fullpath))
				}
			}
			update := make([]*gnmipb.Update, len(datalist))
			for j, data := range datalist {
				typedValue, err := ygot.EncodeTypedValue(data.Value, req.GetEncoding())
				if err != nil {
					return nil, status.TaggedErrorf(codes.Internal, status.TagBadData,
						"typed-value encoding error in %s: %v", data.Path, err)
				}
				datapath, err := xpath.ToGNMIPath(data.Path)
				if err != nil {
					return nil, status.TaggedErrorf(codes.Internal, status.TagInvalidPath,
						"xpath-to-gpath converting error for %s", data.Path)
				}
				update[j] = &gnmipb.Update{Path: datapath, Val: typedValue}
			}
			notification := &gnmipb.Notification{
				Timestamp: time.Now().UnixNano(),
				Prefix:    bprefix,
				Update:    update,
			}
			notifications = append(notifications, notification)
		}
	}
	if len(notifications) <= 0 {
		return nil, status.TaggedErrorf(codes.NotFound, status.TagDataMissing, "no data found")
	}
	return &gnmipb.GetResponse{Notification: notifications}, nil
}

// Set implements the Set RPC in gNMI spec.
func (s *Server) set(ctx context.Context, req *gnmipb.SetRequest) (*gnmipb.SetResponse, error) {
	// utilities.PrintProto(req)
	prefix := req.GetPrefix()

	var index int
	var err error
	result := make([]*gnmipb.UpdateResult, 0, 6)
	s.Model.SetInit()
	for _, path := range req.GetDelete() {
		if err != nil {
			result = append(result, buildUpdateResultAborted(gnmipb.UpdateResult_DELETE, path))
			continue
		}
		err = s.Model.SetDelete(prefix, path)
		result = append(result, buildUpdateResult(gnmipb.UpdateResult_DELETE, path, err))
	}
	for _, r := range req.GetReplace() {
		path := r.GetPath()
		if err != nil {
			result = append(result, buildUpdateResultAborted(gnmipb.UpdateResult_REPLACE, path))
			continue
		}
		err = s.Model.SetReplace(prefix, path, r.GetVal())
		result = append(result, buildUpdateResult(gnmipb.UpdateResult_REPLACE, path, err))
	}
	for _, u := range req.GetUpdate() {
		path := u.GetPath()
		if err != nil {
			result = append(result, buildUpdateResultAborted(gnmipb.UpdateResult_UPDATE, path))
			continue
		}
		err = s.Model.SetUpdate(prefix, path, u.GetVal())
		result = append(result, buildUpdateResult(gnmipb.UpdateResult_UPDATE, path, err))
	}
	if err == nil {
		index, err = s.Model.SetCommit()
	}
	if err != nil {
		// update error
		if index < -1 {
			resultlen := len(result)
			for i := index; i < resultlen; i++ {
				result[i] = buildUpdateResultAborted(result[i].Op, result[i].Path)
			}
			result[index] = buildUpdateResult(result[index].Op, result[index].Path, err)
		}
		s.Model.SetRollback()
	}
	s.Model.SetDone()
	resp := &gnmipb.SetResponse{
		Prefix:   req.GetPrefix(),
		Response: result,
	}
	// utilities.PrintProto(resp)
	return resp, err
}

// Subscribe implements the Subscribe RPC in gNMI spec.
func (s *Server) subscribe(stream gnmipb.GNMI_SubscribeServer) error {
	subses := newSubSession(stream.Context(), s)
	// run stream responsor
	subses.waitgroup.Add(1)
	go func(
		stream gnmipb.GNMI_SubscribeServer,
		telemetrychannel chan *gnmipb.SubscribeResponse,
		shutdown chan struct{},
		waitgroup *sync.WaitGroup,
	) {
		defer waitgroup.Done()
		for {
			select {
			case resp, ok := <-telemetrychannel:
				if ok {
					stream.Send(resp)
					glog.V(1).Infof("Subscribe[%s:%d:%d].response::\n%s",
						subses.Address, subses.Port, 0, proto.MarshalTextString(resp))
				} else {
					return
				}
			case <-shutdown:
				return
			}
		}
	}(stream, subses.respchan, subses.shutdown, subses.waitgroup)

	defer func() {
		subses.stopSubSession()
	}()
	for {
		req, err := stream.Recv()
		seq := s.getReqSequence()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			glog.Errorf("Subscribe[%s:%d:%d].response:: %v",
				subses.Address, subses.Port, 0, err)
			return err
		}
		glog.V(1).Infof("Subscribe[%s:%d:%d].request::\n%s",
			subses.Address, subses.Port, seq, proto.MarshalTextString(req))
		if err = subses.processSubscribeRequest(req); err != nil {
			glog.Errorf("Subscribe[%s:%d:%d].response:: %v",
				subses.Address, subses.Port, seq, err)
			return err
		}
	}
}

// InternalUpdate is an experimental feature to let the server update its
// internal states. Use it with your own risk.
func (s *Server) InternalUpdate(funcPtr func(Model ygot.ValidatedGoStruct) error) error {
	s.Model.Lock()
	defer s.Model.Unlock()
	return funcPtr(s.Model.GetRoot())
}
