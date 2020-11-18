// gnmid (gNMI target implementation)
package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/golang/glog"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	gnmiserver "github.com/neoul/gnxi/gnmi/server"
	"github.com/neoul/libydb/go/ydb"

	"github.com/neoul/gnxi/utilities/netsession"
	"github.com/neoul/gnxi/utilities/server/credentials"
	"github.com/neoul/gnxi/utilities/server/login"

	// "model/m6424"

	pb "github.com/openconfig/gnmi/proto/gnmi"
)

var (
	configFile      = pflag.StringP("config", "c", "", "Configuration file for gnmid; search gnmid.conf from $PWD, /etc and $HOME/.gnmid if not specified")
	bindAddr        = pflag.StringP("bind-address", "b", ":57400", "Bind to address:port")
	startup         = pflag.String("startup", "", "IETF JSON or YAML file for target startup data")
	disableBundling = pflag.Bool("disable-update-bundling", false, "Disable Bundling of Telemetry Updates defined in gNMI Specification 3.5.2.1")
	help            = pflag.BoolP("help", "h", false, "Help for gnmid")
	enableSyslog    = pflag.Bool("enable-syslog", false, "Enable syslog message over gNMI")
)

// Server is gNMI server instance.
type Server struct {
	*gnmiserver.Server
	Config    *Config
	DataBlock *ydb.YDB
}

// Config is used to gnmid configuration.
type Config struct {
	BindAddress     string   `mapstructure:"bind-address"`
	Startup         string   `mapstructure:"startup,omitempty"`
	DisableBundling bool     `mapstructure:"disable-update-bundling,omitempty"`
	NoTLS           bool     `mapstructure:"no-tls,omitempty"`
	CheatCode       string   `mapstructure:"cheat-code,omitempty"`
	SkipVerify      bool     `mapstructure:"skip-verify,omitempty"`
	CAFile          string   `mapstructure:"ca-cert,omitempty"`
	CertFile        string   `mapstructure:"server-cert,omitempty"`
	KeyFile         string   `mapstructure:"server-key,omitempty"`
	EnableSyslog    bool     `mapstructure:"enable-syslog,omitempty"`
	SyncPath        []string `mapstructure:"sync-path,omitempty"`
	AlsoLogToStdErr bool     `mapstructure:"alsologtostderr,omitempty"`
	StdErrThreshold int      `mapstructure:"stderrthreshold,omitempty"`
}

// LoadConfig loads gnmid configuration from flags or a configuration file.
func LoadConfig() (*Config, error) {
	var config Config
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()
	if *help {
		fmt.Fprintf(os.Stderr, "\n gnmid (gRPC Network Management Interface server)\n")
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, " Usage: %s [Flag]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\n")
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
	if f := flag.Lookup("sync-path"); f != nil && f.Value.String() == f.DefValue {
		syncReq := viper.Get("sync-path")
		if syncReqList, ok := syncReq.([]interface{}); ok {
			for i := range syncReqList {
				flag.Set("sync-path", syncReqList[i].(string))
			}
		}
	}
	if f := flag.Lookup("alsologtostderr"); f != nil && f.Value.String() == f.DefValue {
		f.Value.Set(viper.GetString("alsologtostderr"))
	}
	if f := flag.Lookup("stderrthreshold"); f != nil && f.Value.String() == f.DefValue {
		f.Value.Set(viper.GetString("stderrthreshold"))
	}
	return &config, nil
}

// NewServer creates gNMI server instance.
func NewServer() (*Server, error) {
	config, err := LoadConfig()
	if err != nil {
		return nil, err
	}

	// ydb.SetLogLevel(logrus.DebugLevel)
	// ydb.SetInternalLog(ydb.LogDebug)
	db, _ := ydb.Open("gnmid")

	server := Server{
		Config: config,
	}
	var opts []gnmiserver.Option
	if config.Startup != "" {
		var startup []byte
		startup, err = ioutil.ReadFile(config.Startup)
		if err != nil {
			db.Close()
			return nil, err
		}
		opts = append(opts, gnmiserver.Startup(startup))
	}

	if config.DisableBundling {
		opts = append(opts, gnmiserver.DisableBundling{})
	}
	opts = append(opts, gnmiserver.GetCallback{StateSync: db},
		gnmiserver.SetCallback{StateConfig: db})
	if server.Server, err = gnmiserver.NewServer(opts...); err != nil {
		db.Close()
		return nil, err
	}

	db.SetTarget(server.Model, true)
	if err = db.Connect("uss://gnmi", "pub"); err != nil {
		db.Close()
		return nil, err
	}
	db.Serve()
	server.DataBlock = db
	return &server, nil
}

func main() {

	s, err := NewServer()
	if err != nil {
		glog.Exitf("%v", err)
	}
	defer s.DataBlock.Close()

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Supported models:\n")
		for _, m := range s.Model.SupportedModels() {
			fmt.Fprintf(os.Stderr, "  %s\n", m)
		}
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
	}

	opts := credentials.ServerCredentials(s.Config.CAFile,
		s.Config.CertFile, s.Config.KeyFile,
		s.Config.SkipVerify, s.Config.NoTLS)
	opts = append(opts, grpc.UnaryInterceptor(login.UnaryInterceptor))
	opts = append(opts, grpc.StreamInterceptor(login.StreamInterceptor))
	g := grpc.NewServer(opts...)
	pb.RegisterGNMIServer(g, s)
	reflection.Register(g)

	glog.Infof("starting to listen on %s", s.Config.BindAddress)
	listen, err := netsession.Listen("tcp", s.Config.BindAddress, s)
	if err != nil {
		glog.Exitf("failed to listen: %s", err)
	}
	glog.Info("starting to serve")
	if err := g.Serve(listen); err != nil {
		glog.Exitf("failed to serve: %v", err)
	}
}
