package model

import (
	"flag"
	"testing"
	"time"

	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
)

func TestModel_ValidatePathAndSync(t *testing.T) {
	flag.Set("disable-ydb", "true")
	m, err := NewModel(nil, nil, nil)
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	type args struct {
		prefix *gnmipb.Path
		paths  []*gnmipb.Path
	}
	tests := []struct {
		name    string
		args    args
		wanterr bool
	}{
		{
			name: "FindSchemaPaths",
			args: args{
				prefix: nil,
				paths:  []*gnmipb.Path{&gnmipb.Path{}},
			},
			wanterr: true,
		},
		{
			name: "FindSchemaPaths",
			args: args{
				prefix: nil,
				paths: []*gnmipb.Path{
					&gnmipb.Path{
						Elem: []*gnmipb.PathElem{
							&gnmipb.PathElem{
								Name: "interfaces",
							},
							&gnmipb.PathElem{
								Name: "interface",
							},
							&gnmipb.PathElem{
								Name: "state",
							},
							&gnmipb.PathElem{
								Name: "counters",
							},
							&gnmipb.PathElem{
								Name: "in-discards",
							},
						},
					},
				},
			},
			wanterr: true,
		},
		{
			name: "FindSchemaPaths",
			args: args{
				prefix: nil,
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
			wanterr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			syncPaths := m.GetSyncUpdatePath(tt.args.prefix, tt.args.paths)
			m.RunSyncUpdate(time.Second*10, syncPaths)
		})
	}
}
