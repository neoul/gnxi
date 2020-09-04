package model

import (
	"flag"
	"testing"
	"time"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
)

func TestModel_ValidatePathAndSync(t *testing.T) {
	m := NewModel()
	flag.Set("disable-ydb", "true")
	mdata, _ := NewModelData(m, nil, nil, nil)

	type args struct {
		prefix *gpb.Path
		paths  []*gpb.Path
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
				paths:  []*gpb.Path{&gpb.Path{}},
			},
			wanterr: true,
		},
		{
			name: "FindSchemaPaths",
			args: args{
				prefix: nil,
				paths: []*gpb.Path{
					&gpb.Path{
						Elem: []*gpb.PathElem{
							&gpb.PathElem{
								Name: "interfaces",
							},
							&gpb.PathElem{
								Name: "interface",
							},
							&gpb.PathElem{
								Name: "state",
							},
							&gpb.PathElem{
								Name: "counters",
							},
							&gpb.PathElem{
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
				paths: []*gpb.Path{
					&gpb.Path{
						Elem: []*gpb.PathElem{
							&gpb.PathElem{
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
			syncPaths := mdata.GetSyncUpdatePath(tt.args.prefix, tt.args.paths)
			mdata.RunSyncUpdate(time.Second*10, syncPaths)
		})
	}
}
