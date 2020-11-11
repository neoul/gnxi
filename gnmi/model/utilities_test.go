package model

import (
	"io/ioutil"
	"os"
	"reflect"
	"testing"

	"github.com/neoul/gnxi/gnmi/model/gostruct"
	"github.com/neoul/gostruct-dump/dump"
	"github.com/neoul/libydb/go/ydb"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ygot/ygot"
)

func testIsEqualList(d1, d2 interface{}) bool {
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

func testSetDataAndPath(s string, value interface{}) *DataAndPath {
	return &DataAndPath{
		Value: value, Path: s,
	}
}

func TestFindAllData(t *testing.T) {
	root := gostruct.Device{}
	datablock, _ := ydb.OpenWithSync("gnmi_target", &root)
	defer datablock.Close()

	r, err := os.Open("data/sample.yaml")
	defer r.Close()
	if err != nil {
		t.Fatalf("test data load failed: %v", err)
	}
	dec := datablock.NewDecoder(r)
	dec.Decode()

	type args struct {
		gs   ygot.GoStruct
		path *gnmipb.Path
	}
	tests := []struct {
		name  string
		args  args
		want  []*DataAndPath
		want1 bool
	}{
		{
			name: "FindAllDataNodes",
			args: args{
				gs:   &root,
				path: &gnmipb.Path{},
			},
			want: []*DataAndPath{
				testSetDataAndPath("/", &root),
			},
			want1: true,
		},
		{
			name: "FindAllDataNodes",
			args: args{
				gs: &root,
				path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{
						&gnmipb.PathElem{
							Name: "interfaces",
						},
						&gnmipb.PathElem{
							Name: "interface",
						},
					},
				},
			},
			want: []*DataAndPath{
				testSetDataAndPath("/interfaces/interface[name=eth0]", root.Interfaces.Interface["eth0"]),
				testSetDataAndPath("/interfaces/interface[name=eth1]", root.Interfaces.Interface["eth1"]),
			},
			want1: true,
		},
		{
			name: "FindAllDataNodes",
			args: args{
				gs: &root,
				path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{
						&gnmipb.PathElem{
							Name: "interfaces",
						},
						&gnmipb.PathElem{
							Name: "interface",
							Key: map[string]string{
								"name": "eth1",
							},
						},
					},
				},
			},
			want:  []*DataAndPath{testSetDataAndPath("/interfaces/interface[name=eth1]", root.Interfaces.Interface["eth1"])},
			want1: true,
		},
		{
			name: "FindAllDataNodes",
			args: args{
				gs: &root,
				path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{
						&gnmipb.PathElem{
							Name: "interfaces",
						},
						&gnmipb.PathElem{
							Name: "interface",
							Key: map[string]string{
								"name": "eth1",
							},
						},
						&gnmipb.PathElem{
							Name: "config",
						},
						&gnmipb.PathElem{
							Name: "name",
						},
					},
				},
			},
			want: []*DataAndPath{
				testSetDataAndPath("/interfaces/interface[name=eth1]/config/name", root.Interfaces.Interface["eth1"].Config.Name)},
			want1: true,
		},
		{
			name: "FindAllDataNodes",
			args: args{
				gs: &root,
				path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{
						&gnmipb.PathElem{
							Name: "interfaces",
						},
						&gnmipb.PathElem{
							Name: "interface",
							Key: map[string]string{
								"name": "eth1",
							},
						},
						&gnmipb.PathElem{
							Name: "config",
						},
						&gnmipb.PathElem{
							Name: "*",
						},
					},
				},
			},
			want: []*DataAndPath{
				testSetDataAndPath("/interfaces/interface[name=eth1]/config/description", root.Interfaces.Interface["eth1"].Config.Description),
				testSetDataAndPath("/interfaces/interface[name=eth1]/config/enabled", root.Interfaces.Interface["eth1"].Config.Enabled),
				testSetDataAndPath("/interfaces/interface[name=eth1]/config/loopback-mode", root.Interfaces.Interface["eth1"].Config.LoopbackMode),
				testSetDataAndPath("/interfaces/interface[name=eth1]/config/mtu", root.Interfaces.Interface["eth1"].Config.Mtu),
				testSetDataAndPath("/interfaces/interface[name=eth1]/config/name", root.Interfaces.Interface["eth1"].Config.Name),
				testSetDataAndPath("/interfaces/interface[name=eth1]/config/type", root.Interfaces.Interface["eth1"].Config.Type),
			},
			want1: true,
		},
		{
			name: "FindAllDataNodes",
			args: args{
				gs: &root,
				path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{
						&gnmipb.PathElem{
							Name: "interfaces",
						},
						&gnmipb.PathElem{
							Name: "interface",
						},
						&gnmipb.PathElem{
							Name: "config",
						},
						&gnmipb.PathElem{
							Name: "*",
						},
					},
				},
			},
			want: []*DataAndPath{
				testSetDataAndPath("/interfaces/interface[name=eth0]/config/description", root.Interfaces.Interface["eth0"].Config.Description),
				testSetDataAndPath("/interfaces/interface[name=eth0]/config/enabled", root.Interfaces.Interface["eth0"].Config.Enabled),
				testSetDataAndPath("/interfaces/interface[name=eth0]/config/loopback-mode", root.Interfaces.Interface["eth0"].Config.LoopbackMode),
				testSetDataAndPath("/interfaces/interface[name=eth0]/config/mtu", root.Interfaces.Interface["eth0"].Config.Mtu),
				testSetDataAndPath("/interfaces/interface[name=eth0]/config/name", root.Interfaces.Interface["eth0"].Config.Name),
				testSetDataAndPath("/interfaces/interface[name=eth0]/config/type", root.Interfaces.Interface["eth0"].Config.Type),
				testSetDataAndPath("/interfaces/interface[name=eth1]/config/description", root.Interfaces.Interface["eth1"].Config.Description),
				testSetDataAndPath("/interfaces/interface[name=eth1]/config/enabled", root.Interfaces.Interface["eth1"].Config.Enabled),
				testSetDataAndPath("/interfaces/interface[name=eth1]/config/loopback-mode", root.Interfaces.Interface["eth1"].Config.LoopbackMode),
				testSetDataAndPath("/interfaces/interface[name=eth1]/config/mtu", root.Interfaces.Interface["eth1"].Config.Mtu),
				testSetDataAndPath("/interfaces/interface[name=eth1]/config/name", root.Interfaces.Interface["eth1"].Config.Name),
				testSetDataAndPath("/interfaces/interface[name=eth1]/config/type", root.Interfaces.Interface["eth1"].Config.Type),
			},
			want1: true,
		},
		{
			name: "FindAllDataNodes",
			args: args{
				gs: &root,
				path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{
						&gnmipb.PathElem{
							Name: "interfaces",
						},
						&gnmipb.PathElem{
							Name: "*",
						},
						&gnmipb.PathElem{
							Name: "config",
						},
						&gnmipb.PathElem{
							Name: "name",
						},
					},
				},
			},
			want: []*DataAndPath{
				testSetDataAndPath("/interfaces/interface[name=eth0]/config/name", root.Interfaces.Interface["eth0"].Config.Name),
				testSetDataAndPath("/interfaces/interface[name=eth1]/config/name", root.Interfaces.Interface["eth1"].Config.Name),
			},
			want1: true,
		},
		{
			name: "FindAllDataNodes",
			args: args{
				gs: &root,
				path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{
						&gnmipb.PathElem{
							Name: "interfaces",
						},
						&gnmipb.PathElem{
							Name: "...",
						},
						&gnmipb.PathElem{
							Name: "config",
						},
					},
				},
			},
			want: []*DataAndPath{
				testSetDataAndPath("/interfaces/interface[name=eth0]/config", root.Interfaces.Interface["eth0"].Config),
				testSetDataAndPath("/interfaces/interface[name=eth1]/config", root.Interfaces.Interface["eth1"].Config),
			},
			want1: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := FindAllData(tt.args.gs, tt.args.path)
			for _, g := range got {
				t.Log("got::", g)
			}
			if !testIsEqualList(got, tt.want) {
				t.Errorf("FindAllDataNodes() got = %v, want %v", got, tt.want)
				for _, g := range tt.want {
					t.Log("tt.want::", g)
				}
			}
			if got1 != tt.want1 {
				t.Errorf("FindAllDataNodes() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestFindAllNodes(t *testing.T) {
	r, err := os.Open("data/sample.yaml")
	defer r.Close()
	if err != nil {
		t.Fatalf("test data load failed: %v", err)
	}
	b, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatalf("test data load failed: %v", err)
	}

	model, err := NewModel(b, nil)
	root := model.GetRoot().(*gostruct.Device)

	type args struct {
		vgs  ygot.GoStruct
		path *gnmipb.Path
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
				vgs:  root,
				path: &gnmipb.Path{},
			},
			want:  []interface{}{root},
			want1: true,
		},
		{
			name: "FindAllDataNodes",
			args: args{
				vgs: root,
				path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{
						&gnmipb.PathElem{
							Name: "interfaces",
						},
						&gnmipb.PathElem{
							Name: "interface",
						},
					},
				},
			},
			want: []interface{}{
				root.Interfaces.Interface["eth0"],
				root.Interfaces.Interface["eth1"],
			},
			want1: true,
		},
		{
			name: "FindAllDataNodes",
			args: args{
				vgs: root,
				path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{
						&gnmipb.PathElem{
							Name: "interfaces",
						},
						&gnmipb.PathElem{
							Name: "interface",
							Key: map[string]string{
								"name": "eth1",
							},
						},
					},
				},
			},
			want:  []interface{}{root.Interfaces.Interface["eth1"]},
			want1: true,
		},
		{
			name: "FindAllDataNodes",
			args: args{
				vgs: root,
				path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{
						&gnmipb.PathElem{
							Name: "interfaces",
						},
						&gnmipb.PathElem{
							Name: "interface",
							Key: map[string]string{
								"name": "eth1",
							},
						},
						&gnmipb.PathElem{
							Name: "config",
						},
						&gnmipb.PathElem{
							Name: "name",
						},
					},
				},
			},
			want:  []interface{}{root.Interfaces.Interface["eth1"].Config.Name},
			want1: true,
		},
		{
			name: "FindAllDataNodes",
			args: args{
				vgs: root,
				path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{
						&gnmipb.PathElem{
							Name: "interfaces",
						},
						&gnmipb.PathElem{
							Name: "interface",
							Key: map[string]string{
								"name": "eth1",
							},
						},
						&gnmipb.PathElem{
							Name: "config",
						},
						&gnmipb.PathElem{
							Name: "*",
						},
					},
				},
			},
			want: []interface{}{
				root.Interfaces.Interface["eth1"].Config.Description,
				root.Interfaces.Interface["eth1"].Config.Enabled,
				root.Interfaces.Interface["eth1"].Config.LoopbackMode,
				root.Interfaces.Interface["eth1"].Config.Mtu,
				root.Interfaces.Interface["eth1"].Config.Name,
				root.Interfaces.Interface["eth1"].Config.Type,
			},
			want1: true,
		},
		{
			name: "FindAllDataNodes",
			args: args{
				vgs: root,
				path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{
						&gnmipb.PathElem{
							Name: "interfaces",
						},
						&gnmipb.PathElem{
							Name: "interface",
						},
						&gnmipb.PathElem{
							Name: "config",
						},
						&gnmipb.PathElem{
							Name: "*",
						},
					},
				},
			},
			want: []interface{}{
				root.Interfaces.Interface["eth0"].Config.Description,
				root.Interfaces.Interface["eth0"].Config.Enabled,
				root.Interfaces.Interface["eth0"].Config.LoopbackMode,
				root.Interfaces.Interface["eth0"].Config.Mtu,
				root.Interfaces.Interface["eth0"].Config.Name,
				root.Interfaces.Interface["eth0"].Config.Type,
				root.Interfaces.Interface["eth1"].Config.Description,
				root.Interfaces.Interface["eth1"].Config.Enabled,
				root.Interfaces.Interface["eth1"].Config.LoopbackMode,
				root.Interfaces.Interface["eth1"].Config.Mtu,
				root.Interfaces.Interface["eth1"].Config.Name,
				root.Interfaces.Interface["eth1"].Config.Type,
			},
			want1: true,
		},
		{
			name: "FindAllDataNodes",
			args: args{
				vgs: root,
				path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{
						&gnmipb.PathElem{
							Name: "interfaces",
						},
						&gnmipb.PathElem{
							Name: "*",
						},
						&gnmipb.PathElem{
							Name: "config",
						},
						&gnmipb.PathElem{
							Name: "name",
						},
					},
				},
			},
			want: []interface{}{
				root.Interfaces.Interface["eth0"].Config.Name,
				root.Interfaces.Interface["eth1"].Config.Name,
			},
			want1: true,
		},
		{
			name: "FindAllDataNodes",
			args: args{
				vgs: root,
				path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{
						&gnmipb.PathElem{
							Name: "interfaces",
						},
						&gnmipb.PathElem{
							Name: "...",
						},
						&gnmipb.PathElem{
							Name: "config",
						},
					},
				},
			},
			want: []interface{}{
				root.Interfaces.Interface["eth0"].Config,
				root.Interfaces.Interface["eth1"].Config,
			},
			want1: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := FindAllDataNodes(tt.args.vgs, tt.args.path)
			if !testIsEqualList(got, tt.want) {
				t.Errorf("FindAllDataNodes() got = %v, want %v", got, tt.want)
				dump.Print(got)
				dump.Print(tt.want)
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
		path *gnmipb.Path
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
				path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{
						&gnmipb.PathElem{
							Name: "interfaces",
						},
						&gnmipb.PathElem{
							Name: "*",
						},
						&gnmipb.PathElem{
							Name: "config",
						},
						&gnmipb.PathElem{
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
		path *gnmipb.Path
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
				path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{
						&gnmipb.PathElem{
							Name: "interfaces",
						},
						&gnmipb.PathElem{
							Name: "*",
						},
						&gnmipb.PathElem{
							Name: "config",
						},
						&gnmipb.PathElem{
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
				path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{
						&gnmipb.PathElem{
							Name: "interfaces",
						},
						&gnmipb.PathElem{
							Name: "...",
						},
						&gnmipb.PathElem{
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
				path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{
						&gnmipb.PathElem{
							Name: "interfaces",
						},
						&gnmipb.PathElem{
							Name: "interface",
							Key: map[string]string{
								"name": "eth1",
							},
						},
						&gnmipb.PathElem{
							Name: "config",
						},
						&gnmipb.PathElem{
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
				path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{
						&gnmipb.PathElem{
							Name: "interfaces",
						},
						&gnmipb.PathElem{
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
			if !testIsEqualList(got, tt.want) {
				t.Errorf("FindAllSchemaPaths() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("FindAllSchemaPaths() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}
