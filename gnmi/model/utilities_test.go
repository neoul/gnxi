package model

import (
	"io/ioutil"
	"os"
	"reflect"
	"testing"

	"github.com/neoul/gdump"
	"github.com/neoul/gnxi/gnmi/model/gostruct"
	"github.com/neoul/gnxi/utilities/test"
	"github.com/neoul/libydb/go/ydb"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ygot/ygot"
)

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
			if !test.IsEqualList(got, tt.want) {
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

	model, err := NewModel(nil, nil, nil)
	model.Load(b, Encoding_YAML, false)
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
			if !test.IsEqualList(got, tt.want) {
				t.Errorf("FindAllDataNodes() got = %v, want %v", got, tt.want)
				gdump.Print(got)
				gdump.Print(tt.want)
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
			if !test.IsEqualList(got, tt.want) {
				t.Errorf("FindAllSchemaPaths() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("FindAllSchemaPaths() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func Test_writeValue(t *testing.T) {
	m, err := NewModel(nil, nil, nil)
	if err != nil {
		t.Errorf("%v", err)
		return
	}
	type args struct {
		path  *gnmipb.Path
		value interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "WriteValue",
			args: args{
				path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{
						&gnmipb.PathElem{
							Name: "interfaces",
						},
						&gnmipb.PathElem{
							Name: "interface",
							Key: map[string]string{
								"name": "1/1",
							},
						},
						&gnmipb.PathElem{
							Name: "config",
						},
					},
				},
				value: &gostruct.OpenconfigInterfaces_Interfaces_Interface_Config{
					Description: ygot.String("desciption field"),
					Enabled:     ygot.Bool(true),
					Name:        ygot.String("1/1"),
					Mtu:         ygot.Uint16(1500),
					Type:        gostruct.IETFInterfaces_InterfaceType_ethernetCsmacd,
				},
			},
		},
		{
			name: "WriteValue",
			args: args{
				path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{
						&gnmipb.PathElem{
							Name: "sample",
						},
						&gnmipb.PathElem{
							Name: "container-val",
						},
					},
				},
				value: &gostruct.Sample_Sample_ContainerVal{
					LeafListVal: []string{"v1", "v2"},
				},
			},
		},
		{
			name: "WriteValue",
			args: args{
				path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{
						&gnmipb.PathElem{
							Name: "sample",
						},
						&gnmipb.PathElem{
							Name: "container-val",
						},
						&gnmipb.PathElem{
							Name: "enum-val",
						},
					},
				},
				value: gostruct.Sample_Sample_ContainerVal_EnumVal_enum2,
			},
		},

		{
			name: "WriteValue",
			args: args{
				path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{
						&gnmipb.PathElem{
							Name: "sample",
						},
					},
				},
				value: &gostruct.Sample_Sample{
					ContainerVal: &gostruct.Sample_Sample_ContainerVal{
						EnumVal:     gostruct.Sample_Sample_ContainerVal_EnumVal_enum1,
						LeafListVal: []string{},
					},
					EmptyVal: gostruct.YANGEmpty(true),
					MultipleKeyList: map[gostruct.Sample_Sample_MultipleKeyList_Key]*gostruct.Sample_Sample_MultipleKeyList{
						gostruct.Sample_Sample_MultipleKeyList_Key{Str: "stringkey", Integer: 0}: &gostruct.Sample_Sample_MultipleKeyList{
							Str: ygot.String("stringkey"), Integer: ygot.Uint32(0), Ok: ygot.Bool(true),
						},
					},
					SingleKeyList: map[string]*gostruct.Sample_Sample_SingleKeyList{
						"stringkey": &gostruct.Sample_Sample_SingleKeyList{
							CountryCode: ygot.String("kr"),
							DialCode:    ygot.Uint32(82),
							ListKey:     ygot.String("stringkey"),
						},
					},
					StrVal: ygot.String("string-value"),
				},
			},
		},
		{
			name: "WriteValue",
			args: args{
				path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{
						&gnmipb.PathElem{
							Name: "sample",
						},
						&gnmipb.PathElem{
							Name: "container-val",
						},
						&gnmipb.PathElem{
							Name: "leaf-list-val",
						},
					},
				},
				value: []string{"v3"},
			},
		},
		{
			name: "DeleteValue",
			args: args{
				path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{
						&gnmipb.PathElem{
							Name: "interfaces",
						},
						&gnmipb.PathElem{
							Name: "interface",
							Key: map[string]string{
								"name": "1/1",
							},
						},
						&gnmipb.PathElem{
							Name: "config",
						},
						&gnmipb.PathElem{
							Name: "description",
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			switch tt.name {
			case "WriteValue":
				if err := m.WriteValue(tt.args.path, tt.args.value); (err != nil) != tt.wantErr {
					t.Errorf("WriteValue() error = %v, wantErr %v", err, tt.wantErr)
				}
			case "DeleteValue":
				if err := m.DeleteValue(tt.args.path); (err != nil) != tt.wantErr {
					t.Errorf("DeleteValue() error = %v, wantErr %v", err, tt.wantErr)
				}
			}

		})
	}
	j, _ := m.ExportToJSON(false)
	t.Log(string(j))
	j, _ = m.ExportToJSON(true)
	t.Log(string(j))
}

func TestMO_UpdateType(t *testing.T) {
	m, err := NewModel(nil, nil, nil)
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "UpdateType",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := m.UpdateType(); (err != nil) != tt.wantErr {
				t.Errorf("MO.UpdateType() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
