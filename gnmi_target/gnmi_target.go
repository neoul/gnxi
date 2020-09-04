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
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/neoul/gnxi/gnmi/model"
	gnmiserver "github.com/neoul/gnxi/gnmi/server"

	"github.com/neoul/gnxi/utilities/credentials"
	"github.com/neoul/gnxi/utilities/netsession"

	pb "github.com/openconfig/gnmi/proto/gnmi"
)

var (
	configFile      = pflag.StringP("config", "c", "", "configuration file for gnmid; search gnmid.conf from $PWD, /etc and $HOME/.gnmid if not specified")
	bindAddr        = pflag.StringP("bind-address", "b", ":10161", "bind to address:port")
	startupFile     = pflag.String("startup", "", "IETF JSON or YAML file for target startup data")
	disableBundling = pflag.Bool("disable-update-bundling", false, "disable Bundling of Telemetry Updates defined in gNMI Specification 3.5.2.1")
	help            = pflag.BoolP("help", "h", false, "help for gnmi_target")
)

type server struct {
	*gnmiserver.Server
	config *configuration
}

type configuration struct {
	BindAddress     string `mapstructure:"bind-address"`
	Startup         string `mapstructure:"startup,omitempty"`
	DisableBundling bool   `mapstructure:"disable-update-bundling,omitempty"`
	DisableYDB      bool   `mapstructure:"disable-ydb,omitempty"`
	NoTLS           bool   `mapstructure:"no-tls,omitempty"`

	TLS struct {
		Insecure bool   `mapstructure:"insecure,omitempty"`
		CAFile   string `mapstructure:"ca-cert,omitempty"`
		CertFile string `mapstructure:"server-cert,omitempty"`
		KeyFile  string `mapstructure:"server-key,omitempty"`
	} `mapstructure:"tls,omitempty"`
}

func loadConfig() (*configuration, error) {
	var config configuration
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()
	if *help {
		fmt.Fprintf(os.Stderr, "gnmi_target:\n")
		fmt.Fprintf(os.Stderr, "  gRPC Network Management Interface (gNMI) server\n")
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  %s [Flag]\n", os.Args[0])
		pflag.PrintDefaults()
		os.Exit(1)
	}
	viper.BindPFlags(pflag.CommandLine)
	viper.SetConfigType("yaml")
	if *configFile != "" {
		f, err := os.Open(*configFile)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		err = viper.ReadConfig(f)
		if err != nil {
			return nil, err
		}
	} else {
		viper.SetConfigName("gnmid.conf")
		viper.AddConfigPath(".")
		viper.AddConfigPath("/etc")                  // path to look for the config file in
		viper.AddConfigPath("$HOME/.gnmid")          // call multiple times to add many search paths
		if err := viper.ReadInConfig(); err != nil { // Handle errors reading the config file
			if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
				// Config file was found but another error was produced
				return nil, err
			}
		}
	}
	err := viper.Unmarshal(&config)
	if err != nil {
		return nil, err
	}
	syncReq := viper.Get("sync-required-path")
	if syncReqList, ok := syncReq.([]interface{}); ok {
		for i := range syncReqList {
			flag.Set("sync-required-path", syncReqList[i].(string))
		}
	}
	// utilities.PrintStruct(config)
	return &config, nil
}

func newServer(model *model.Model) (*server, error) {
	var err error
	s := server{}
	s.config, err = loadConfig()
	if err != nil {
		return nil, err
	}

	var startup []byte
	var startupIsJSON bool = true
	if s.config.Startup != "" {
		startup, err = ioutil.ReadFile(s.config.Startup)
		if err != nil {
			glog.Exitf("error in reading startup file: %v", err)
		}
		if strings.HasSuffix(s.config.Startup, "yaml") ||
			strings.HasSuffix(s.config.Startup, "yml") {
			startupIsJSON = false
		} else {
			startupIsJSON = false
		}
	}

	s.Server, err = gnmiserver.NewServer(model, startup, startupIsJSON,
		s.config.DisableBundling)
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

	s, err := newServer(model)
	if err != nil {
		glog.Exitf("error in creating gnmid: %v", err)
	}
	defer s.Close()

	opts := credentials.ServerCredentials(
		&s.config.TLS.CAFile, &s.config.TLS.CertFile, &s.config.TLS.KeyFile,
		&s.config.TLS.Insecure, &s.config.NoTLS)
	// opts = append(opts, grpc.UnaryInterceptor(credentials.UnaryInterceptor))
	// opts = append(opts, grpc.StreamInterceptor(credentials.StreamInterceptor))
	g := grpc.NewServer(opts...)
	pb.RegisterGNMIServer(g, s)
	reflection.Register(g)

	glog.Infof("starting to listen on %s", s.config.BindAddress)
	listen, err := netsession.Listen("tcp", s.config.BindAddress, s)
	if err != nil {
		glog.Exitf("failed to listen: %s", err)
	}
	glog.Info("starting to serve")
	if err := g.Serve(listen); err != nil {
		glog.Exitf("failed to serve: %v", err)
	}
}
