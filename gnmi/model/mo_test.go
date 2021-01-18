package model

import (
	"reflect"
	"testing"

	"github.com/neoul/gnxi/gnmi/model/gostruct"
	"github.com/openconfig/ygot/ygot"
)

func TestMO_NewRoot(t *testing.T) {
	type args struct {
	}
	sch, err := gostruct.Schema()
	if err != nil {
		t.Fatalf("schema loading failed: %v", err)
	}
	mo := (*MO)(sch)
	tests := []struct {
		name    string
		startup []byte
		want    *MO
		wantErr bool
	}{
		{
			name: "MO_NewRoot",
			startup: []byte(
				`{
"interfaces": {
	 "interface": {
	  "1/1": {
	   "config": {
		"enabled": true,
		"mtu": 1500,
		"name": "1/1",
		"type": "ethernetCsmacd"
	   },
	   "name": "1/1"
	  }
	 }
	},
	"sample": {
	 "container-val": {
	  "enum-val": "enum1",
	  "leaf-list-val": [
	   "v3"
	  ]
	 },
	 "empty-val": true,
	 "multiple-key-list": {
	  "stringkey 1": {
	   "integer": 1,
	   "ok": true,
	   "str": "stringkey"
	  }
	 },
	 "single-key-list": {
	  "stringkey": {
	   "country-code": "kr",
	   "dial-code": 82,
	   "list-key": "stringkey"
	  }
	 },
	 "str-val": "string-value"
	}
   }`),
			want: &MO{
				Unmarshal:  mo.Unmarshal,
				SchemaTree: mo.SchemaTree,
				Root: &gostruct.Device{
					Interfaces: &gostruct.OpenconfigInterfaces_Interfaces{
						Interface: map[string]*gostruct.OpenconfigInterfaces_Interfaces_Interface{
							"1/1": &gostruct.OpenconfigInterfaces_Interfaces_Interface{
								Config: &gostruct.OpenconfigInterfaces_Interfaces_Interface_Config{
									Name:    ygot.String("1/1"),
									Enabled: ygot.Bool(true),
									Mtu:     ygot.Uint16(1500),
									Type:    gostruct.IETFInterfaces_InterfaceType_ethernetCsmacd,
								},
								Name: ygot.String("1/1"),
							},
						},
					},
					Sample: &gostruct.Sample_Sample{
						ContainerVal: &gostruct.Sample_Sample_ContainerVal{
							EnumVal: gostruct.Sample_Sample_ContainerVal_EnumVal_enum1,
							LeafListVal: []string{
								"v3",
							},
						},
						EmptyVal: true,
						MultipleKeyList: map[gostruct.Sample_Sample_MultipleKeyList_Key]*gostruct.Sample_Sample_MultipleKeyList{
							gostruct.Sample_Sample_MultipleKeyList_Key{
								Str:     "stringkey",
								Integer: 1,
							}: &gostruct.Sample_Sample_MultipleKeyList{
								Str:     ygot.String("stringkey"),
								Integer: ygot.Uint32(1),
								Ok:      ygot.Bool(true),
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
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := mo.NewRoot(tt.startup, Encoding_JSON)
			if (err != nil) != tt.wantErr {
				t.Errorf("MO.NewRoot() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			g, err := got.ExportToJSON(true)
			if err != nil {
				t.Errorf("MO.ExportToJSON() error = %v", err)
			}
			w, err := tt.want.ExportToJSON(true)
			if err != nil {
				t.Errorf("MO.ExportToJSON() error = %v", err)
			}
			if !reflect.DeepEqual(g, w) {
				t.Fatalf("got server config %v\nwant: %v", string(g), string(w))
			}
		})
	}
}
