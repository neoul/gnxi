package model

import (
	"testing"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
)

func TestModel_ValidatePathAndSync(t *testing.T) {
	m := NewModel()
	mdata, _ := NewModelData(m, nil, nil, nil, true)

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
			mdata.SyncUpdatePathData(tt.args.prefix, tt.args.paths)

		})
	}
}
