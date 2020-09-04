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
	"encoding/json"
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

	dpb "github.com/golang/protobuf/protoc-gen-go/descriptor"
	pb "github.com/openconfig/gnmi/proto/gnmi"
)

var (
	supportedEncodings = []pb.Encoding{pb.Encoding_JSON, pb.Encoding_JSON_IETF}
)

// func init() {
// 	ydb.SetInternalLog(ydb.LogDebug)
// }

// Server struct maintains the data structure for device config and implements the interface of gnmi server. It supports Capabilities, Get, and Set APIs.
// Typical usage:
//	g := grpc.NewServer()
//	s, err := Server.NewServer(model, config, callback)
//	pb.NewServer(g, s)
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
	disableBundling bool
	Model           *model.Model
	ModelData       *model.ModelData
	*telemetryCtrl
	sessions   map[string]*Session
	alias      map[string]*pb.Alias
	useAliases bool
}

// Config - the gNMI server configuration
type Config struct {
}

// NewServer creates an instance of Server with given json config.
func NewServer(m *model.Model, startup []byte, startupIsJSON, disableBundling bool) (*Server, error) {
	var err error
	s := &Server{
		disableBundling: disableBundling,
		Model:           m,
		alias:           map[string]*pb.Alias{},
		sessions:        map[string]*Session{},
		telemetryCtrl:   newTelemetryCB(),
	}
	if startupIsJSON {
		s.ModelData, err = model.NewModelData(m, startup, nil, s)
	} else {
		s.ModelData, err = model.NewModelData(m, nil, startup, s)
	}
	return s, err
}

// Close the connected YDB instance
func (s *Server) Close() {
	s.ModelData.Close()
}

// checkEncoding checks whether encoding and models are supported by the server. Return error if anything is unsupported.
func (s *Server) checkEncoding(encoding pb.Encoding) error {
	hasSupportedEncoding := false
	for _, supportedEncoding := range supportedEncodings {
		if encoding == supportedEncoding {
			hasSupportedEncoding = true
			break
		}
	}
	if !hasSupportedEncoding {
		err := fmt.Errorf("unsupported encoding: %s", pb.Encoding_name[int32(encoding)])
		return status.Error(codes.Unimplemented, err.Error())
	}
	return nil
}

// getGNMIServiceVersion returns a pointer to the gNMI service version string.
// The method is non-trivial because of the way it is defined in the proto file.
func getGNMIServiceVersion() (*string, error) {
	gzB, _ := (&pb.Update{}).Descriptor()
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
	ver, err := proto.GetExtension(desc.Options, pb.E_GnmiService)
	if err != nil {
		return nil, fmt.Errorf("error in getting version from proto extension: %v", err)
	}
	return ver.(*string), nil
}

// Capabilities returns supported encodings and supported models.
func (s *Server) Capabilities(ctx context.Context, req *pb.CapabilityRequest) (*pb.CapabilityResponse, error) {
	ver, err := getGNMIServiceVersion()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "error in getting gnmi service version: %v", err)
	}
	return &pb.CapabilityResponse{
		SupportedModels:    s.Model.GetModelData(),
		SupportedEncodings: supportedEncodings,
		GNMIVersion:        *ver,
	}, nil
}

// Get implements the Get RPC in gNMI spec.
func (s *Server) Get(ctx context.Context, req *pb.GetRequest) (*pb.GetResponse, error) {

	if req.GetType() != pb.GetRequest_ALL {
		return nil, status.Errorf(codes.Unimplemented, "unsupported request type: %s",
			pb.GetRequest_DataType_name[int32(req.GetType())])
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
	syncPaths := s.ModelData.GetSyncUpdatePath(prefix, paths)
	s.ModelData.RunSyncUpdate(time.Second*3, syncPaths)

	s.ModelData.RLock()
	defer s.ModelData.RUnlock()

	// each prefix + path ==> one notification message
	if err := utilities.ValidateGNMIPath(prefix); err != nil {
		return nil, status.Errorf(codes.Unimplemented, "invalid-path(%s)", err.Error())
	}
	toplist, ok := s.Model.FindAllData(s.ModelData.GetRoot(), prefix)
	if !ok || len(toplist) <= 0 {
		if ok = s.Model.ValidatePathSchema(prefix); ok {
			return nil, status.Errorf(codes.NotFound, "data-missing(%v)", xpath.ToXPATH(prefix))
		}
		return nil, status.Errorf(codes.NotFound, "unknown-schema(%s)", xpath.ToXPATH(prefix))
	}
	notifications := []*pb.Notification{}
	for _, top := range toplist {
		bpath := top.Path
		branch := top.Value.(ygot.GoStruct)
		bprefix, err := xpath.ToGNMIPath(bpath)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "path-conversion-error(%s)", bprefix)
		}
		for _, path := range paths {
			if err := utilities.ValidateGNMIFullPath(prefix, path); err != nil {
				return nil, status.Errorf(codes.Unimplemented, "invalid-path(%s)", err.Error())
			}
			datalist, ok := s.Model.FindAllData(branch, path)
			if !ok || len(datalist) <= 0 {
				continue
			}
			update := make([]*pb.Update, len(datalist))
			for j, data := range datalist {
				typedValue, err := ygot.EncodeTypedValue(data.Value, req.GetEncoding())
				if err != nil {
					return nil, status.Errorf(codes.Internal, "encoding-error(%s)", err.Error())
				}
				datapath, err := xpath.ToGNMIPath(data.Path)
				if err != nil {
					return nil, status.Errorf(codes.Internal, "path-conversion-error(%s)", data.Path)
				}
				update[j] = &pb.Update{Path: datapath, Val: typedValue}
			}
			notification := &pb.Notification{
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
	return &pb.GetResponse{Notification: notifications}, nil
}

// Set implements the Set RPC in gNMI spec.
func (s *Server) Set(ctx context.Context, req *pb.SetRequest) (*pb.SetResponse, error) {
	s.ModelData.Lock()
	defer s.ModelData.Unlock()

	utilities.PrintProto(req)

	jsonTree, err := ygot.ConstructIETFJSON(s.ModelData.GetRoot(), &ygot.RFC7951JSONConfig{})
	if err != nil {
		msg := fmt.Sprintf("error in constructing IETF JSON tree from config struct: %v", err)
		return nil, status.Error(codes.Internal, msg)
	}

	prefix := req.GetPrefix()
	var results []*pb.UpdateResult

	for _, path := range req.GetDelete() {
		res, grpcStatusError := s.ModelData.SetDelete(jsonTree, prefix, path)
		if grpcStatusError != nil {
			return nil, grpcStatusError
		}
		results = append(results, res)
	}
	for _, upd := range req.GetReplace() {
		res, grpcStatusError := s.ModelData.SetReplaceOrUpdate(jsonTree, pb.UpdateResult_REPLACE, prefix, upd.GetPath(), upd.GetVal())
		if grpcStatusError != nil {
			return nil, grpcStatusError
		}
		results = append(results, res)
	}
	for _, upd := range req.GetUpdate() {
		res, grpcStatusError := s.ModelData.SetReplaceOrUpdate(jsonTree, pb.UpdateResult_UPDATE, prefix, upd.GetPath(), upd.GetVal())
		if grpcStatusError != nil {
			return nil, grpcStatusError
		}
		results = append(results, res)
	}

	jsonDump, err := json.Marshal(jsonTree)
	if err != nil {
		msg := fmt.Sprintf("error in marshaling IETF JSON tree to bytes: %v", err)
		return nil, status.Error(codes.Internal, msg)
	}
	newRoot, err := model.NewGoStruct(s.Model, jsonDump)
	if err != nil {
		msg := fmt.Sprintf("error in creating config struct from IETF JSON data: %v", err)
		return nil, status.Error(codes.Internal, msg)
	}
	s.ModelData.ChangeRoot(newRoot)
	return &pb.SetResponse{
		Prefix:   req.GetPrefix(),
		Response: results,
	}, nil
}

// Subscribe implements the Subscribe RPC in gNMI spec.
func (s *Server) Subscribe(stream pb.GNMI_SubscribeServer) error {
	teleses := newTelemetrySession(s)
	// run stream responsor
	teleses.waitgroup.Add(1)
	go func(
		stream pb.GNMI_SubscribeServer,
		telemetrychannel chan *pb.SubscribeResponse,
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
func (s *Server) InternalUpdate(funcPtr func(ModelData ygot.ValidatedGoStruct) error) error {
	s.ModelData.Lock()
	defer s.ModelData.Unlock()
	return funcPtr(s.ModelData.GetRoot())
}
