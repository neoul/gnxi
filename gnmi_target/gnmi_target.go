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

// Binary gnmi_target implements a gNMI Target with in-memory configuration and telemetry.
package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"reflect"

	"github.com/golang/glog"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/neoul/gnxi/gnmi"
	"github.com/neoul/gnxi/gnmi/modeldata/gostruct"

	"github.com/neoul/gnxi/utils/credentials"

	pb "github.com/openconfig/gnmi/proto/gnmi"
)

var (
	bindAddr   = flag.String("bind_address", ":10161", "Bind to address:port or just :port")
	configFile = flag.String("config", "", "IETF JSON file for target startup config")
)

type server struct {
	*gnmi.Server
}

func newServer(model *gnmi.Model, config []byte) (*server, error) {
	s, err := gnmi.NewServer(model, config, nil)
	if err != nil {
		return nil, err
	}
	return &server{Server: s}, nil
}

// validateUser validates the user.
func authorizeUser(session *gnmi.Session) error {
	return nil
	// if session.Username == "" {
	// 	return status.Errorf(codes.InvalidArgument, "missing username")
	// }
	// if session.Password == "" {
	// 	return status.Errorf(codes.InvalidArgument, "missing password")
	// }
	// // [FIXME] authorize user here
	// return status.Errorf(codes.Unauthenticated, "authentication failed")
}

// Capabilities - Wrapping function for creating session info.
func (s *server) Capabilities(ctx context.Context, req *pb.CapabilityRequest) (*pb.CapabilityResponse, error) {
	var resp *pb.CapabilityResponse
	session, err := s.NewSession(ctx)
	if err != nil {
		return nil, err
	}
	err = authorizeUser(session)
	if err == nil {
		resp, err = s.Server.Capabilities(ctx, req)
	}
	glog.Infof("[%s] result = '%v'", session.SID, err)
	s.CloseSession(session)
	return resp, err
}

// Get - Wrapping function for creating session info.
func (s *server) Get(ctx context.Context, req *pb.GetRequest) (*pb.GetResponse, error) {
	var resp *pb.GetResponse
	session, err := s.NewSession(ctx)
	if err != nil {
		return nil, err
	}
	err = authorizeUser(session)
	if err == nil {
		resp, err = s.Server.Get(ctx, req)
	}
	glog.Infof("[%s] result = '%v'", session.SID, err)
	s.CloseSession(session)
	return resp, err
}

// Set - Wrapping function for creating session info.
func (s *server) Set(ctx context.Context, req *pb.SetRequest) (*pb.SetResponse, error) {
	var resp *pb.SetResponse
	session, err := s.NewSession(ctx)
	if err != nil {
		return nil, err
	}
	err = authorizeUser(session)
	if err == nil {
		resp, err = s.Server.Set(ctx, req)
	}
	glog.Infof("[%s] result = '%v'", session.SID, err)
	s.CloseSession(session)
	return resp, err
}

// Subscribe - Wrapping function for creating session info.
func (s *server) Subscribe(stream pb.GNMI_SubscribeServer) error {
	session, err := s.NewSession(stream.Context())
	if err != nil {
		return err
	}
	err = authorizeUser(session)
	if err == nil {
		err = s.Server.Subscribe(stream)
	}
	glog.Infof("[%s] result = '%v'", session.SID, err)
	s.CloseSession(session)
	return err
}

func main() {
	model := gnmi.NewModel(
		reflect.TypeOf((*gostruct.Device)(nil)),
		gostruct.SchemaTree["Device"],
		gostruct.Unmarshal,
		gostruct.ΛEnum)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Supported models:\n")
		for _, m := range model.SupportedModels() {
			fmt.Fprintf(os.Stderr, "  %s\n", m)
		}
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
	}

	flag.Parse()

	opts := credentials.ServerCredentials()
	// opts = append(opts, grpc.UnaryInterceptor(credentials.UnaryInterceptor))
	// opts = append(opts, grpc.StreamInterceptor(credentials.StreamInterceptor))
	g := grpc.NewServer(opts...)

	var configData []byte
	if *configFile != "" {
		var err error
		configData, err = ioutil.ReadFile(*configFile)
		if err != nil {
			glog.Exitf("error in reading config file: %v", err)
		}
	}
	s, err := newServer(model, configData)
	if err != nil {
		glog.Exitf("error in creating gnmid: %v", err)
	}
	defer s.Close()

	pb.RegisterGNMIServer(g, s)
	reflection.Register(g)

	glog.Infof("starting to listen on %s", *bindAddr)
	listen, err := net.Listen("tcp", *bindAddr)
	if err != nil {
		glog.Exitf("failed to listen: %v", err)
	}

	glog.Info("starting to serve")
	if err := g.Serve(listen); err != nil {
		glog.Exitf("failed to serve: %v", err)
	}
}
