package gnmi

import (
	"flag"
	"os"
	"reflect"
	"testing"

	"github.com/neoul/gnxi/gnmi/modeldata/gostruct"
	"github.com/neoul/gnxi/utils"
	pb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ygot/ygot"
)

func TestFindAllNodes(t *testing.T) {
	flag.Set("disable-ydb", "true")
	// model is the model for test config server.
	model = &Model{
		modelData:       gostruct.ΓModelData,
		structRootType:  reflect.TypeOf((*gostruct.Device)(nil)),
		schemaTreeRoot:  gostruct.SchemaTree["Device"],
		jsonUnmarshaler: gostruct.Unmarshal,
		enumData:        gostruct.ΛEnum,
	}
	s, err := NewServer(model, nil, nil)
	if err != nil {
		t.Fatalf("error in creating server: %v", err)
	}
	r, err := os.Open("modeldata/data/sample.yaml")
	defer r.Close()
	if err != nil {
		t.Fatalf("test data load failed: %v", err)
	}
	dec := s.dataBlock.NewDecoder(r)
	dec.Decode()

	type args struct {
		vgs  ygot.ValidatedGoStruct
		path *pb.Path
	}
	tests := []struct {
		name  string
		args  args
		want  []interface{}
		want1 bool
	}{
		{
			name: "FindAllNodes",
			args: args{
				vgs:  s.config,
				path: &pb.Path{},
			},
			want:  []interface{}{s.config},
			want1: true,
		},
		{
			name: "FindAllNodes",
			args: args{
				vgs: s.config,
				path: &pb.Path{
					Elem: []*pb.PathElem{
						&pb.PathElem{
							Name: "interfaces",
						},
						&pb.PathElem{
							Name: "interface",
						},
					},
				},
			},
			want: []interface{}{
				s.config.(*gostruct.Device).Interfaces.Interface["eth0"],
				s.config.(*gostruct.Device).Interfaces.Interface["eth1"],
			},
			want1: true,
		},
		{
			name: "FindAllNodes",
			args: args{
				vgs: s.config,
				path: &pb.Path{
					Elem: []*pb.PathElem{
						&pb.PathElem{
							Name: "interfaces",
						},
						&pb.PathElem{
							Name: "interface",
							Key: map[string]string{
								"name": "eth1",
							},
						},
					},
				},
			},
			want:  []interface{}{s.config.(*gostruct.Device).Interfaces.Interface["eth1"]},
			want1: true,
		},
		{
			name: "FindAllNodes",
			args: args{
				vgs: s.config,
				path: &pb.Path{
					Elem: []*pb.PathElem{
						&pb.PathElem{
							Name: "interfaces",
						},
						&pb.PathElem{
							Name: "interface",
							Key: map[string]string{
								"name": "eth1",
							},
						},
						&pb.PathElem{
							Name: "config",
						},
						&pb.PathElem{
							Name: "name",
						},
					},
				},
			},
			want:  []interface{}{s.config.(*gostruct.Device).Interfaces.Interface["eth1"].Config.Name},
			want1: true,
		},
		{
			name: "FindAllNodes",
			args: args{
				vgs: s.config,
				path: &pb.Path{
					Elem: []*pb.PathElem{
						&pb.PathElem{
							Name: "interfaces",
						},
						&pb.PathElem{
							Name: "interface",
							Key: map[string]string{
								"name": "eth1",
							},
						},
						&pb.PathElem{
							Name: "config",
						},
						&pb.PathElem{
							Name: "*",
						},
					},
				},
			},
			want: []interface{}{
				s.config.(*gostruct.Device).Interfaces.Interface["eth1"].Config.Description,
				s.config.(*gostruct.Device).Interfaces.Interface["eth1"].Config.Enabled,
				s.config.(*gostruct.Device).Interfaces.Interface["eth1"].Config.LoopbackMode,
				s.config.(*gostruct.Device).Interfaces.Interface["eth1"].Config.Mtu,
				s.config.(*gostruct.Device).Interfaces.Interface["eth1"].Config.Name,
				s.config.(*gostruct.Device).Interfaces.Interface["eth1"].Config.Type,
			},
			want1: true,
		},
		{
			name: "FindAllNodes",
			args: args{
				vgs: s.config,
				path: &pb.Path{
					Elem: []*pb.PathElem{
						&pb.PathElem{
							Name: "interfaces",
						},
						&pb.PathElem{
							Name: "interface",
						},
						&pb.PathElem{
							Name: "config",
						},
						&pb.PathElem{
							Name: "*",
						},
					},
				},
			},
			want: []interface{}{
				s.config.(*gostruct.Device).Interfaces.Interface["eth0"].Config.Description,
				s.config.(*gostruct.Device).Interfaces.Interface["eth0"].Config.Enabled,
				s.config.(*gostruct.Device).Interfaces.Interface["eth0"].Config.LoopbackMode,
				s.config.(*gostruct.Device).Interfaces.Interface["eth0"].Config.Mtu,
				s.config.(*gostruct.Device).Interfaces.Interface["eth0"].Config.Name,
				s.config.(*gostruct.Device).Interfaces.Interface["eth0"].Config.Type,
				s.config.(*gostruct.Device).Interfaces.Interface["eth1"].Config.Description,
				s.config.(*gostruct.Device).Interfaces.Interface["eth1"].Config.Enabled,
				s.config.(*gostruct.Device).Interfaces.Interface["eth1"].Config.LoopbackMode,
				s.config.(*gostruct.Device).Interfaces.Interface["eth1"].Config.Mtu,
				s.config.(*gostruct.Device).Interfaces.Interface["eth1"].Config.Name,
				s.config.(*gostruct.Device).Interfaces.Interface["eth1"].Config.Type,
			},
			want1: true,
		},
		{
			name: "FindAllNodes",
			args: args{
				vgs: s.config,
				path: &pb.Path{
					Elem: []*pb.PathElem{
						&pb.PathElem{
							Name: "interfaces",
						},
						&pb.PathElem{
							Name: "*",
						},
						&pb.PathElem{
							Name: "config",
						},
						&pb.PathElem{
							Name: "name",
						},
					},
				},
			},
			want:  []interface{}{},
			want1: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := FindAllNodes(tt.args.vgs, tt.args.path)
			for _, r := range got {
				utils.PrintStruct(r)
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FindAllNodes() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("FindAllNodes() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}
