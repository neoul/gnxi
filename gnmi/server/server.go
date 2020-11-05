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

	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/golang/protobuf/proto"
	"github.com/neoul/gnxi/gnmi/model"
	"github.com/neoul/gnxi/utilities"
	"github.com/neoul/gnxi/utilities/xpath"
	"github.com/openconfig/ygot/ygot"
	"github.com/openconfig/ygot/ytypes"

	dpb "github.com/golang/protobuf/protoc-gen-go/descriptor"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
)

var (
	supportedEncodings = []gnmipb.Encoding{gnmipb.Encoding_JSON, gnmipb.Encoding_JSON_IETF}
)

// func init() {
// 	ydb.SetInternalLog(ydb.LogDebug)
// }

// Server struct maintains the data structure for device config and implements the interface of gnmi server. It supports Capabilities, Get, and Set APIs.
// Typical usage:
//	g := grpc.NewServer()
//	s, err := Server.NewServer(model, config, callback)
//	gnmipb.NewServer(g, s)
//	reflection.Register(g)
//	listen, err := net.Listen("tcp", ":8080")
//	g.Serve(listen)
//
// For a real device, apply the config changes to the hardware in the callback function.
// Arguments:
//		newRoot: new root config to be applied on the device.
// func callback(newRoot ygot.ValidatedGoStruct) error {
//		// Apply the config to your device and return nil if success. return error if fails.
//		//
//		// Do something ...
// }
type Server struct {
	Model *model.Model
	*telemetryCtrl
	disableBundling bool
	sessions        map[string]*Session
	alias           map[string]*gnmipb.Alias
	useAliases      bool
}

// NewServer creates an instance of Server with given json config.
func NewServer(startup []byte, disableBundling bool) (*Server, error) {
	var err error
	var m *model.Model
	s := &Server{
		disableBundling: disableBundling,
		alias:           map[string]*gnmipb.Alias{},
		sessions:        map[string]*Session{},
		telemetryCtrl:   newTelemetryCB(),
	}
	m, err = model.NewModel(startup, s)
	if err != nil {
		return nil, err
	}
	s.Model = m
	return s, nil
}

// NewCustomServer creates an instance of Server with given json config.
func NewCustomServer(schema func() (*ytypes.Schema, error), supportedModels []*gnmipb.ModelData, startup []byte, startupIsJSON, disableBundling bool) (*Server, error) {
	var err error
	var m *model.Model
	s := &Server{
		disableBundling: disableBundling,
		alias:           map[string]*gnmipb.Alias{},
		sessions:        map[string]*Session{},
		telemetryCtrl:   newTelemetryCB(),
	}
	m, err = model.NewCustomModel(schema, supportedModels, startup, s)
	if err != nil {
		return nil, err
	}
	s.Model = m
	return s, nil
}

// Close the connected YDB instance
func (s *Server) Close() {
	s.Model.Close()
}

// checkEncoding checks whether encoding and models are supported by the server. Return error if anything is unsupported.
func (s *Server) checkEncoding(encoding gnmipb.Encoding) error {
	hasSupportedEncoding := false
	for _, supportedEncoding := range supportedEncodings {
		if encoding == supportedEncoding {
			hasSupportedEncoding = true
			break
		}
	}
	if !hasSupportedEncoding {
		err := fmt.Errorf("unsupported encoding: %s", gnmipb.Encoding_name[int32(encoding)])
		return status.Error(codes.Unimplemented, err.Error())
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

// Capabilities returns supported encodings and supported models.
func (s *Server) Capabilities(ctx context.Context, req *gnmipb.CapabilityRequest) (*gnmipb.CapabilityResponse, error) {
	ver, err := getGNMIServiceVersion()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "error in getting gnmi service version: %v", err)
	}
	return &gnmipb.CapabilityResponse{
		SupportedModels:    s.Model.GetModelData(),
		SupportedEncodings: supportedEncodings,
		GNMIVersion:        *ver,
	}, nil
}

// Get implements the Get RPC in gNMI spec.
func (s *Server) Get(ctx context.Context, req *gnmipb.GetRequest) (*gnmipb.GetResponse, error) {

	if req.GetType() != gnmipb.GetRequest_ALL {
		return nil, status.Errorf(codes.Unimplemented, "unsupported request type: %s",
			gnmipb.GetRequest_DataType_name[int32(req.GetType())])
	}
	if err := s.Model.CheckModels(req.GetUseModels()); err != nil {
		return nil, status.Errorf(codes.Unimplemented, err.Error())
	}
	if err := s.checkEncoding(req.GetEncoding()); err != nil {
		return nil, err
	}

	// fmt.Println(proto.MarshalTextString(req))

	prefix := req.GetPrefix()
	paths := req.GetPath()
	syncPaths := s.Model.GetSyncUpdatePath(prefix, paths)
	s.Model.RunSyncUpdate(time.Second*3, syncPaths)

	s.Model.RLock()
	defer s.Model.RUnlock()

	// each prefix + path ==> one notification message
	if err := xpath.ValidateGNMIPath(prefix); err != nil {
		return nil, status.Errorf(codes.Unimplemented, "invalid-path(%s)", err.Error())
	}
	toplist, ok := s.Model.Find(s.Model.GetRoot(), prefix)
	if !ok || len(toplist) <= 0 {
		if ok = s.Model.ValidatePathSchema(prefix); ok {
			return nil, status.Errorf(codes.NotFound, "data-missing(%v)", xpath.ToXPath(prefix))
		}
		return nil, status.Errorf(codes.NotFound, "unknown-schema(%s)", xpath.ToXPath(prefix))
	}
	notifications := []*gnmipb.Notification{}
	for _, top := range toplist {
		bpath := top.Path
		branch := top.Value.(ygot.GoStruct)
		bprefix, err := xpath.ToGNMIPath(bpath)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "path-conversion-error(%s)", bprefix)
		}
		for _, path := range paths {
			if err := xpath.ValidateGNMIFullPath(prefix, path); err != nil {
				return nil, status.Errorf(codes.Unimplemented, "invalid-path(%s)", err.Error())
			}
			datalist, ok := s.Model.Find(branch, path)
			if !ok || len(datalist) <= 0 {
				continue
			}
			update := make([]*gnmipb.Update, len(datalist))
			for j, data := range datalist {
				typedValue, err := ygot.EncodeTypedValue(data.Value, req.GetEncoding())
				if err != nil {
					return nil, status.Errorf(codes.Internal, "encoding-error(%s)", err.Error())
				}
				datapath, err := xpath.ToGNMIPath(data.Path)
				if err != nil {
					return nil, status.Errorf(codes.Internal, "path-conversion-error(%s)", data.Path)
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
		return nil, status.Errorf(codes.NotFound, "data-missing")
	}
	return &gnmipb.GetResponse{Notification: notifications}, nil
}

// Set implements the Set RPC in gNMI spec.
func (s *Server) Set(ctx context.Context, req *gnmipb.SetRequest) (*gnmipb.SetResponse, error) {
	utilities.PrintProto(req)
	prefix := req.GetPrefix()

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
		typedvalue := r.GetVal()
		if err != nil {
			result = append(result, buildUpdateResultAborted(gnmipb.UpdateResult_REPLACE, path))
			continue
		}
		err = s.Model.SetReplace(prefix, path, typedvalue)
		result = append(result, buildUpdateResult(gnmipb.UpdateResult_REPLACE, path, err))
	}
	for _, u := range req.GetUpdate() {
		path := u.GetPath()
		typedvalue := u.GetVal()
		if err != nil {
			result = append(result, buildUpdateResultAborted(gnmipb.UpdateResult_UPDATE, path))
			continue
		}
		err = s.Model.SetUpdate(prefix, path, typedvalue)
		result = append(result, buildUpdateResult(gnmipb.UpdateResult_UPDATE, path, err))
	}
	if err == nil {
		err = s.Model.SetCommit()
	}
	if err != nil {
		s.Model.SetRollback()
	}
	resp := &gnmipb.SetResponse{
		Prefix:   req.GetPrefix(),
		Response: result,
	}
	utilities.PrintProto(resp)
	return resp, err
}

// Subscribe implements the Subscribe RPC in gNMI spec.
func (s *Server) Subscribe(stream gnmipb.GNMI_SubscribeServer) error {
	teleses := newTelemetrySession(stream.Context(), s)
	// utilities.PrintStruct(teleses)
	// run stream responsor
	teleses.waitgroup.Add(1)
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
					// fmt.Println(proto.MarshalTextString(resp))
					stream.Send(resp)
				} else {
					return
				}
			case <-shutdown:
				return
			}
		}
	}(stream, teleses.respchan, teleses.shutdown, teleses.waitgroup)

	defer func() {
		teleses.stopTelemetrySession()
	}()
	for {
		req, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		// fmt.Println(proto.MarshalTextString(req))
		err = processSR(teleses, req)
		if err != nil {
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
