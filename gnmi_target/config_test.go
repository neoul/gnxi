package main

import (
	"reflect"
	"testing"
)

func TestNewConfig(t *testing.T) {
	type args struct {
		configfile string
	}

	tests := []struct {
		name string
		args args
		want *Config
	}{
		{
			name: "config creating",
			args: args{},
			want: &Config{
				Address: struct {
					IPAddress string "yaml:\"ip-address,omitempty\""
					Port      uint16 "yaml:\"port,omitempty\""
				}{Port: defaultServerPort},
				// GNMI: struct {
				// 	SupportedEncoding []string "yaml:\"supported-encoding,omitempty\""
				// }{
				// 	SupportedEncoding: []string{"JSON", "JSON_IETF"},
				// },
			},
		},
		{
			name: "config loading from a file",
			args: args{
				configfile: "data/config.yaml",
			},
			want: &Config{
				Address: struct {
					IPAddress string "yaml:\"ip-address,omitempty\""
					Port      uint16 "yaml:\"port,omitempty\""
				}{Port: defaultServerPort},
				// GNMI: struct {
				// 	SupportedEncoding []string "yaml:\"supported-encoding,omitempty\""
				// }{
				// 	SupportedEncoding: []string{"JSON_IETF"},
				// },
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, err := newConfig(tt.args.configfile); err != nil || !reflect.DeepEqual(got, tt.want) {
				t.Errorf("newConfig() = %v, %v", got, err)
				t.Errorf("newConfig() want %v", tt.want)
			}
		})
	}
}
