package model

import (
	"os"
	"testing"

	"github.com/neoul/gnxi/utilities/test"
	"github.com/neoul/libydb/go/ydb"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
)

type testStateSync struct {
	path        []string
	updatedPath []string
}

func newTestStateSync(path ...string) *testStateSync {
	tss := &testStateSync{path: make([]string, 0, len(path))}
	for i := range path {
		tss.path = append(tss.path, path[i])
	}
	return tss
}

func (tss *testStateSync) UpdateSync(path ...string) error {
	tss.updatedPath = append(tss.updatedPath, path...)
	return nil
}

func (tss *testStateSync) UpdateSyncPath() []string {
	return tss.path
}

func TestModel_RequestStateSync(t *testing.T) {
	syncRequestedPath := []string{
		"/interfaces/interface/state/counters",
		"/interfaces/interface/state/enabled",
		"/interfaces/interface/config/enabled",
	}
	tss := newTestStateSync(syncRequestedPath...)
	m, err := NewModel(nil, nil, tss)
	if err != nil {
		t.Error("failed to create a model")
	}
	datablock, _ := ydb.OpenWithSync("gnmi_target", m)
	defer datablock.Close()

	r, err := os.Open("data/sample.yaml")
	defer r.Close()
	if err != nil {
		t.Fatalf("test data load failed: %v", err)
	}
	dec := datablock.NewDecoder(r)
	dec.Decode()
	// gdump.Print(m.Root)

	// flag.Set("alsologtostderr", "true")
	// flag.Set("v", "100")
	// flag.Parse()

	type args struct {
		prefix *gnmipb.Path
		paths  []*gnmipb.Path
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "RequestStateSync 1",
			args: args{
				paths: []*gnmipb.Path{
					&gnmipb.Path{
						Elem: []*gnmipb.PathElem{
							&gnmipb.PathElem{
								Name: "interfaces",
							},
						},
					},
				},
			},
			want: []string{
				"/interfaces/interface[name=eth0]/config/enabled",
				"/interfaces/interface[name=eth1]/config/enabled",
				"/interfaces/interface[name=eth0]/state/counters",
			},
		},
		{
			name: "RequestStateSync 2",
			args: args{
				prefix: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{
						&gnmipb.PathElem{
							Name: "interfaces",
						},
					},
				},
				paths: []*gnmipb.Path{
					&gnmipb.Path{
						Elem: []*gnmipb.PathElem{
							&gnmipb.PathElem{
								Name: "interface",
							},
							&gnmipb.PathElem{
								Name: "config",
							},
						},
					},
				},
			},
			want: []string{
				"/interfaces/interface[name=eth0]/config/enabled",
				"/interfaces/interface[name=eth1]/config/enabled",
			},
		},
		// {
		// 	name: "RequestStateSync 3",
		// 	args: args{
		// 		prefix: &gnmipb.Path{
		// 			Elem: []*gnmipb.PathElem{
		// 				&gnmipb.PathElem{
		// 					Name: "interfaces",
		// 				},
		// 			},
		// 		},
		// 		paths: []*gnmipb.Path{
		// 			&gnmipb.Path{
		// 				Elem: []*gnmipb.PathElem{
		// 					&gnmipb.PathElem{
		// 						Name: "interface",
		// 						Key: map[string]string{
		// 							"name": "1/1",
		// 						},
		// 					},
		// 					&gnmipb.PathElem{
		// 						Name: "config",
		// 					},
		// 				},
		// 			},
		// 		},
		// 	},
		// 	want: []string{
		// 		"/interfaces/interface[name=1/1]/config/enabled",
		// 	},
		// },
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m.RequestStateSync(tt.args.prefix, tt.args.paths)
			if !test.IsEqualList(tss.updatedPath, tt.want) {
				t.Errorf("FindAllDataNodes() got = %v, want %v", tss.updatedPath, tt.want)
				for _, g := range tss.updatedPath {
					t.Log("tss.updatedPath::", g)
				}
				for _, g := range tt.want {
					t.Log("tt.want::", g)
				}
			}
			tss.updatedPath = []string{}
		})
	}
}
