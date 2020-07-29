package config

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
				BindAddress: ":10161",
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
				BindAddress: ":10161",
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
			if got, err := NewConfig(tt.args.configfile); err != nil || !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewConfig() = %v, %v", got, err)
				t.Errorf("NewConfig() want %v", tt.want)
			}
		})
	}
}
