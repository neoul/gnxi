// gnmid (gNMI target implementation)
package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/golang/glog"
	"github.com/spf13/pflag"
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
	configFile   = pflag.StringP("config", "c", "", "Configuration file for gnmid; search gnmid.conf from $PWD, /etc and $HOME/.gnmid if not specified")
	bindAddr     = pflag.StringP("bind-address", "b", ":57400", "Bind to address:port")
	startup      = pflag.String("startup", "", "IETF JSON or YAML file for target startup data")
	help         = pflag.BoolP("help", "h", false, "Help for gnmid")
	enableSyslog = pflag.Bool("enable-syslog", false, "Enable syslog message over gNMI")
	syncPaths    = pflag.StringSliceP("sync-path", "s", []string{}, "The paths that needs to be updated before read")
)

// Server is gNMI server instance.
type Server struct {
	*gnmiserver.Server
	datablock *datablock
}

type datablock struct {
	*ydb.YDB
}

// newDatablock creates a new datablock.
func newDatablock() *datablock {
	// ydb.SetLogLevel(logrus.DebugLevel)
	// ydb.SetInternalLog(ydb.LogDebug)
	db := ydb.New("gnmid")
	if db == nil {
		glog.Exitf("datablock creation failed")
		return nil
	}
	if err := db.Connect("uss://gnmi", "pub"); err != nil {
		db.Close()
		glog.Exitf("failed to listen: %s", err)
		return nil
	}
	db.Serve()
	return &datablock{YDB: db}
}

// UpdateCreate function of the DataUpdate interface for *datablock
func (db *datablock) UpdateCreate(path string, value string) error {
	return db.WriteTo(path, value)
}

// UpdateReplace function of the DataUpdate interface for *datablock
func (db *datablock) UpdateReplace(path string, value string) error {
	return db.WriteTo(path, value)
}

// UpdateDelete function of the DataUpdate interface for *datablock
func (db *datablock) UpdateDelete(path string) error {
	return db.DeleteFrom(path)
}

// UpdateStart function of the DataUpdateStartEnd interface for *datablock
func (db *datablock) UpdateStart() error {
	return nil
}

// UpdateEnd function of the DataUpdateStartEnd interface for *datablock
func (db *datablock) UpdateEnd() error {
	return nil
}

// UpdateSync requests the update to remote datablock instances in order to refresh the data nodes.
func (db *datablock) UpdateSync(path ...string) error {
	return db.EnhansedSyncTo(time.Second*3, true, path...)
}

// UpdateSyncPath requests the update to remote datablock instances in order to refresh the data nodes.
func (db *datablock) UpdateSyncPath() []string {
	return *syncPaths
}

// NewServer creates gNMI server instance.
func NewServer() (*Server, error) {
	server := Server{
		datablock: newDatablock(),
	}
	var opts []gnmiserver.Option
	if *startup != "" {
		var startbyte []byte
		startbyte, err := ioutil.ReadFile(*startup)
		if err != nil {
			server.datablock.Close()
			return nil, err
		}
		opts = append(opts, gnmiserver.Startup(startbyte))
	}

	opts = append(opts, gnmiserver.GetCallback{StateSync: server.datablock},
		gnmiserver.SetCallback{StateConfig: server.datablock})
	s, err := gnmiserver.NewServer(opts...)
	if err != nil {
		server.datablock.Close()
		return nil, err
	}
	server.Server = s
	server.datablock.SetTarget(server.Model, true)
	return &server, nil
}

func main() {
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

	s, err := NewServer()
	if err != nil {
		glog.Exitf("%v", err)
	}
	defer s.datablock.Close()

	opts := credentials.ServerCredentials("", "", "", false, false)
	opts = append(opts, grpc.UnaryInterceptor(login.UnaryInterceptor))
	opts = append(opts, grpc.StreamInterceptor(login.StreamInterceptor))
	g := grpc.NewServer(opts...)
	pb.RegisterGNMIServer(g, s)
	reflection.Register(g)

	glog.Infof("starting to listen on %s", *bindAddr)
	listen, err := netsession.Listen("tcp", *bindAddr, s)
	if err != nil {
		glog.Exitf("failed to listen: %s", err)
	}
	glog.Info("starting to serve")
	if err := g.Serve(listen); err != nil {
		glog.Exitf("failed to serve: %v", err)
	}
}
