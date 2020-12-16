/* Copyright 2017 Google Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package model

import (
	"testing"

	"github.com/neoul/gnxi/utilities/test"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
)

func TestModel_FindSchemaPaths(t *testing.T) {
	m, err := NewModel(nil, nil, nil)
	if err != nil {
		t.Errorf("%v", err)
		return
	}
	type args struct {
		path *gnmipb.Path
	}
	tests := []struct {
		name  string
		args  args
		want  []string
		want1 bool
	}{
		{
			name: "FindSchemaPaths",
			args: args{
				path: &gnmipb.Path{},
			},
			want:  []string{"/"},
			want1: true,
		},
		{
			name: "FindSchemaPaths",
			args: args{
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
			name: "FindSchemaPaths",
			args: args{
				path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{
						&gnmipb.PathElem{
							Name: "interfaces",
						},
					},
				},
			},
			want:  []string{"/interfaces"},
			want1: true,
		},
		{
			name: "FindSchemaPaths",
			args: args{
				path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{
						&gnmipb.PathElem{
							Name: "interfaces",
						},
						&gnmipb.PathElem{
							Name: "interface",
							Key: map[string]string{
								"name": "eth0",
							},
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
			want:  []string{"/interfaces/interface/state/counters/in-discards"},
			want1: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := m.FindSchemaPaths(tt.args.path)
			if !test.IsEqualList(got, tt.want) {
				t.Errorf("Model.FindSchemaPaths() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("Model.FindSchemaPaths() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}
