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
	"gopkg.in/yaml.v2"

	"github.com/neoul/gnxi/gnmi/model"
	gnmiserver "github.com/neoul/gnxi/gnmi/server"
	"github.com/neoul/gnxi/utilities"

	"github.com/neoul/gnxi/utilities/credentials"
	"github.com/neoul/gnxi/utilities/netsession"

	pb "github.com/openconfig/gnmi/proto/gnmi"
)

var (
	bindAddr          = flag.String("bind_address", ":10161", "Bind to address:port")
	startupFile       = flag.String("startup", "", "IETF JSON file or YAML file for target startup data")
	configFile        = flag.String("config", "", "YAML configuration file")
	disableBundling   = flag.Bool("disable-update-bundling", false, "Disable Bundling of Telemetry Updates defined in gNMI Specification 3.5.2.1")
	disableYdbChannel = flag.Bool("disable-ydb", false, "Disable YAML Datablock interface")
)

type server struct {
	*gnmiserver.Server `yaml:"server,omitempty"`
	ConfigFile         string `yaml:"config,omitempty"`
	BindAddress        string `yaml:"bind_address"`

	NoTLS bool `yaml:"no-tls,omitempty"`
	TLS   struct {
		Insecure bool   `yaml:"insecure,omitempty"`
		CAFile   string `yaml:"ca-cert,omitempty"`
		CertFile string `yaml:"server-cert,omitempty"`
		KeyFile  string `yaml:"server-key,omitempty"`
	} `yaml:"tls,omitempty"`
	GNMIServer struct {
		DisableBundling bool     `yaml:"disable-update-bundling,omitempty"`
		DisableYDB      bool     `yaml:"disable-ydb,omitempty"`
		SyncRequired    []string `yaml:"sync-required,omitempty"`
	}

	Startup struct {
		File string `yaml:"file,omitempty"`
	} `yaml:"startup,omitempty"`
	Debug struct {
		LogLevel int `yaml:"cert-file,omitempty"`
	} `yaml:"debug,omitempty"`
}

func newServer(model *model.Model) (*server, error) {
	s := server{}
	s.BindAddress = utilities.GetFlag("bind_address", ":10161").(string)
	s.Startup.File = utilities.GetFlag("startup", s.Startup.File).(string)
	s.NoTLS = utilities.GetFlag("no-tls", s.NoTLS).(bool)
	if !s.NoTLS {
		s.TLS.Insecure = utilities.GetFlag("insecure", s.TLS.Insecure).(bool)
		s.TLS.CAFile = utilities.GetFlag("ca", s.TLS.CAFile).(string)
		s.TLS.CertFile = utilities.GetFlag("cert", s.TLS.CertFile).(string)
		s.TLS.KeyFile = utilities.GetFlag("key", s.TLS.KeyFile).(string)
	}
	s.GNMIServer.DisableBundling = utilities.GetFlag("disable-update-bundling", false).(bool)
	s.GNMIServer.DisableYDB = utilities.GetFlag("disable-ydb", false).(bool)
	s.GNMIServer.SyncRequired = []string{}
	s.Startup.File = utilities.GetFlag("startup", s.Startup.File).(string)
	s.ConfigFile = utilities.GetFlag("config", "").(string)
	if s.ConfigFile != "" {
		f, err := os.Open(s.ConfigFile)
		if err != nil {
			return nil, err
		}
		decoder := yaml.NewDecoder(f)
		err = decoder.Decode(&s)
		if err != nil {
			return nil, err
		}
		f.Close()
	}
	var err error
	var startup []byte
	var startupIsJSON bool = true
	if s.Startup.File != "" {
		startup, err = ioutil.ReadFile(s.Startup.File)
		if err != nil {
			glog.Exitf("error in reading config file: %v", err)
		}
		if strings.HasSuffix(s.Startup.File, "yaml") ||
			strings.HasSuffix(s.Startup.File, "yml") {
			startupIsJSON = false
		} else {
			startupIsJSON = false
		}
	}

	b, err := yaml.Marshal(&s)
	if err == nil {
		fmt.Println("[configuration]")
		fmt.Println(string(b))
	}

	s.Server, err = gnmiserver.NewServer(model, startup, startupIsJSON,
		s.GNMIServer.DisableBundling, s.GNMIServer.DisableYDB)
	if err != nil {
		return nil, err
	}
	return &s, nil
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

	glog.Infof("starting to listen on %s", s.BindAddress)
	listen, err := netsession.Listen("tcp", s.BindAddress, s)
	glog.Info("starting to serve")
	if err := g.Serve(listen); err != nil {
		glog.Exitf("failed to serve: %v", err)
	}
}
