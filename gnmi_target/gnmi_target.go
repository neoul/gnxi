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
	"os"
	"strings"

	"github.com/golang/glog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/neoul/gnxi/gnmi/model"
	gnmiserver "github.com/neoul/gnxi/gnmi/server"
	"github.com/neoul/gnxi/gnmi_target/config"

	"github.com/neoul/gnxi/utils/credentials"
	"github.com/neoul/gnxi/utils/netsession"

	pb "github.com/openconfig/gnmi/proto/gnmi"
)

var (
	bindAddr         = flag.String("bind_address", ":10161", "Bind to address:port or just :port")
	startupFile      = flag.String("startup", "", "IETF JSON file or YAML file for target startup data")
	serverConfigFile = flag.String("config", "", "YAML configuration file")
)

type server struct {
	*gnmiserver.Server
	*config.Config
}

func newServer(model *model.Model) (*server, error) {
	c, err := config.NewConfig(*serverConfigFile)
	if err != nil {
		return nil, err
	}
	var startupJSON []byte
	var startupYAML []byte
	if c.Startup.File != "" {
		var err error
		var startup []byte
		startup, err = ioutil.ReadFile(c.Startup.File)
		if err != nil {
			glog.Exitf("error in reading config file: %v", err)
		}
		if strings.HasSuffix(c.Startup.File, "yaml") ||
			strings.HasSuffix(c.Startup.File, "yml") {
			startupYAML = startup
		} else {
			startupJSON = startup
		}
	}
	s, err := gnmiserver.NewServer(model, startupJSON, startupYAML)
	if err != nil {
		return nil, err
	}
	return &server{Server: s, Config: c}, nil
}

func main() {
	model := model.NewModel()
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
	s, err := newServer(model)
	if err != nil {
		glog.Exitf("error in creating gnmid: %v", err)
	}
	defer s.Close()

	opts := credentials.ServerCredentials(
		&s.TLS.CAFile, &s.TLS.CertFile, &s.TLS.KeyFile,
		&s.TLS.Insecure, &s.NoTLS)
	// opts = append(opts, grpc.UnaryInterceptor(credentials.UnaryInterceptor))
	// opts = append(opts, grpc.StreamInterceptor(credentials.StreamInterceptor))
	g := grpc.NewServer(opts...)
	pb.RegisterGNMIServer(g, s)
	reflection.Register(g)

	glog.Infof("starting to listen on %s", s.Config.BindAddress)
	listen, err := netsession.Listen("tcp", s.Config.BindAddress, s)
	glog.Info("starting to serve")
	if err := g.Serve(listen); err != nil {
		glog.Exitf("failed to serve: %v", err)
	}
}
