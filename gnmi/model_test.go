package gnmi

import (
	"flag"
	"os"
	"reflect"
	"testing"

	"github.com/neoul/gnxi/gnmi/modeldata/gostruct"
	"github.com/neoul/gnxi/utils"
	"github.com/neoul/libydb/go/ydb"
	pb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ygot/ygot"
)

func isEqualList(d1, d2 interface{}) bool {
	v1 := reflect.ValueOf(d1)
	v2 := reflect.ValueOf(d2)
	if ydb.IsTypeInterface(v1.Type()) {
		v1 = v1.Elem()
	}
	if ydb.IsTypeInterface(v2.Type()) {
		v2 = v2.Elem()
	}

	if v1.Kind() != reflect.Slice && v1.Kind() != v2.Kind() {
		return false
	}

	for v1.Len() != v2.Len() {
		return false
	}

	l := v1.Len()
	for i := 0; i < l; i++ {
		eq := false
		// fmt.Println("v1", v1.Index(i).Interface())
		for j := 0; j < l; j++ {
			// fmt.Println("v2", v2.Index(j).Interface())
			if reflect.DeepEqual(v1.Index(i).Interface(), v2.Index(j).Interface()) {
				eq = true
				break
			}
		}
		if !eq {
			return false
		}
	}
	return true
}

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
	defer s.Close()
	r, err := os.Open("modeldata/data/sample.yaml")
	defer r.Close()
	if err != nil {
		t.Fatalf("test data load failed: %v", err)
	}
	dec := s.datablock.NewDecoder(r)
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
			name: "FindAllDataNodes",
			args: args{
				vgs:  s.datastore,
				path: &pb.Path{},
			},
			want:  []interface{}{s.datastore},
			want1: true,
		},
		{
			name: "FindAllDataNodes",
			args: args{
				vgs: s.datastore,
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
				s.datastore.(*gostruct.Device).Interfaces.Interface["eth0"],
				s.datastore.(*gostruct.Device).Interfaces.Interface["eth1"],
			},
			want1: true,
		},
		{
			name: "FindAllDataNodes",
			args: args{
				vgs: s.datastore,
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
			want:  []interface{}{s.datastore.(*gostruct.Device).Interfaces.Interface["eth1"]},
			want1: true,
		},
		{
			name: "FindAllDataNodes",
			args: args{
				vgs: s.datastore,
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
			want:  []interface{}{s.datastore.(*gostruct.Device).Interfaces.Interface["eth1"].Config.Name},
			want1: true,
		},
		{
			name: "FindAllDataNodes",
			args: args{
				vgs: s.datastore,
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
				s.datastore.(*gostruct.Device).Interfaces.Interface["eth1"].Config.Description,
				s.datastore.(*gostruct.Device).Interfaces.Interface["eth1"].Config.Enabled,
				s.datastore.(*gostruct.Device).Interfaces.Interface["eth1"].Config.LoopbackMode,
				s.datastore.(*gostruct.Device).Interfaces.Interface["eth1"].Config.Mtu,
				s.datastore.(*gostruct.Device).Interfaces.Interface["eth1"].Config.Name,
				s.datastore.(*gostruct.Device).Interfaces.Interface["eth1"].Config.Type,
			},
			want1: true,
		},
		{
			name: "FindAllDataNodes",
			args: args{
				vgs: s.datastore,
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
				s.datastore.(*gostruct.Device).Interfaces.Interface["eth0"].Config.Description,
				s.datastore.(*gostruct.Device).Interfaces.Interface["eth0"].Config.Enabled,
				s.datastore.(*gostruct.Device).Interfaces.Interface["eth0"].Config.LoopbackMode,
				s.datastore.(*gostruct.Device).Interfaces.Interface["eth0"].Config.Mtu,
				s.datastore.(*gostruct.Device).Interfaces.Interface["eth0"].Config.Name,
				s.datastore.(*gostruct.Device).Interfaces.Interface["eth0"].Config.Type,
				s.datastore.(*gostruct.Device).Interfaces.Interface["eth1"].Config.Description,
				s.datastore.(*gostruct.Device).Interfaces.Interface["eth1"].Config.Enabled,
				s.datastore.(*gostruct.Device).Interfaces.Interface["eth1"].Config.LoopbackMode,
				s.datastore.(*gostruct.Device).Interfaces.Interface["eth1"].Config.Mtu,
				s.datastore.(*gostruct.Device).Interfaces.Interface["eth1"].Config.Name,
				s.datastore.(*gostruct.Device).Interfaces.Interface["eth1"].Config.Type,
			},
			want1: true,
		},
		{
			name: "FindAllDataNodes",
			args: args{
				vgs: s.datastore,
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
			want: []interface{}{
				s.datastore.(*gostruct.Device).Interfaces.Interface["eth0"].Config.Name,
				s.datastore.(*gostruct.Device).Interfaces.Interface["eth1"].Config.Name,
			},
			want1: true,
		},
		{
			name: "FindAllDataNodes",
			args: args{
				vgs: s.datastore,
				path: &pb.Path{
					Elem: []*pb.PathElem{
						&pb.PathElem{
							Name: "interfaces",
						},
						&pb.PathElem{
							Name: "...",
						},
						&pb.PathElem{
							Name: "config",
						},
					},
				},
			},
			want: []interface{}{
				s.datastore.(*gostruct.Device).Interfaces.Interface["eth0"].Config,
				s.datastore.(*gostruct.Device).Interfaces.Interface["eth1"].Config,
			},
			want1: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := FindAllDataNodes(tt.args.vgs, tt.args.path)
			for _, r := range got {
				utils.PrintStruct(r)
			}

			if !isEqualList(got, tt.want) {
				t.Errorf("FindAllDataNodes() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("FindAllDataNodes() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestFindAllSchemaTypes(t *testing.T) {
	gd := gostruct.Device{}

	type args struct {
		gs   ygot.GoStruct
		path *pb.Path
	}
	tests := []struct {
		name  string
		args  args
		want  []reflect.Type
		want1 bool
	}{
		{
			name: "TestFindAllSchemaNodes",
			args: args{
				gs: &gd,
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
							Name: "*",
						},
					},
				},
			},
			want: []reflect.Type{
				reflect.TypeOf(gostruct.OpenconfigInterfaces_Interfaces_Interface_Config{}.Description),
				reflect.TypeOf(gostruct.OpenconfigInterfaces_Interfaces_Interface_Config{}.Enabled),
				reflect.TypeOf(gostruct.OpenconfigInterfaces_Interfaces_Interface_Config{}.LoopbackMode),
				reflect.TypeOf(gostruct.OpenconfigInterfaces_Interfaces_Interface_Config{}.Mtu),
				reflect.TypeOf(gostruct.OpenconfigInterfaces_Interfaces_Interface_Config{}.Name),
				reflect.TypeOf(gostruct.OpenconfigInterfaces_Interfaces_Interface_Config{}.Type),
			},
			want1: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := FindAllSchemaTypes(tt.args.gs, tt.args.path)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FindAllSchemaTypes() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("FindAllSchemaTypes() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestFindAllSchemaPaths(t *testing.T) {
	gd := gostruct.Device{}
	type args struct {
		gs   ygot.GoStruct
		path *pb.Path
	}
	tests := []struct {
		name  string
		args  args
		want  []string
		want1 bool
	}{
		{
			name: "TestFindAllSchemaPaths",
			args: args{
				gs: &gd,
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
							Name: "*",
						},
					},
				},
			},
			want: []string{
				"/interfaces/interface/config/description",
				"/interfaces/interface/config/enabled",
				"/interfaces/interface/config/loopback-mode",
				"/interfaces/interface/config/mtu",
				"/interfaces/interface/config/name",
				"/interfaces/interface/config/type",
			},
			want1: true,
		},
		{
			name: "FindAllDataNodes",
			args: args{
				gs: &gd,
				path: &pb.Path{
					Elem: []*pb.PathElem{
						&pb.PathElem{
							Name: "interfaces",
						},
						&pb.PathElem{
							Name: "...",
						},
						&pb.PathElem{
							Name: "config",
						},
					},
				},
			},
			want: []string{
				"/interfaces/interface/config",
				"/interfaces/interface/hold-time/config",
				"/interfaces/interface/subinterfaces/subinterface/config",
			},
			want1: true,
		},
		{
			name: "FindAllDataNodes",
			args: args{
				gs: &gd,
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
			want: []string{
				"/interfaces/interface/config/description",
				"/interfaces/interface/config/enabled",
				"/interfaces/interface/config/loopback-mode",
				"/interfaces/interface/config/mtu",
				"/interfaces/interface/config/name",
				"/interfaces/interface/config/type",
			},
			want1: true,
		},
		{
			name: "FindAllDataNodes",
			args: args{
				gs: &gd,
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
			want: []string{
				"/interfaces/interface",
			},
			want1: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := FindAllSchemaPaths(tt.args.gs, tt.args.path)
			if !isEqualList(got, tt.want) {
				t.Errorf("FindAllSchemaPaths() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("FindAllSchemaPaths() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}
