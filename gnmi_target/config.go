package main

import (
	"flag"
	"os"

	"github.com/neoul/gnxi/gnmi"
	"github.com/neoul/gnxi/gnmi/model"
	"gopkg.in/yaml.v2"
)

// Config - gNMI Server configuration data
type Config struct {
	Address struct {
		IPAddress string `yaml:"ip-address,omitempty"`
		Port      uint16 `yaml:"port,omitempty"`
	} `yaml:"address"`

	TLS struct {
		Insecure bool   `yaml:"insecure,omitempty"`
		CAFile   string `yaml:"ca-cert-file,omitempty"`
		CertFile string `yaml:"server-cert-file,omitempty"`
		KeyFile  string `yaml:"server-key-file,omitempty"`
	} `yaml:"tls,omitempty"`

	// GNMI struct {
	// 	SupportedEncoding []string `yaml:"supported-encoding,omitempty"`
	// } `yaml:"gnmi,omitempty"`

	Control struct {
		DisableBundling bool `yaml:"disable-update-bundling,omitempty"`
		DisableYDB      bool `yaml:"disable-ydb,omitempty"`
		YDB             struct {
			SyncRequired []string `yaml:"sync-required,omitempty"`
		} `yaml:"ydb,omitempty"`
	} `yaml:"control,omitempty"`

	Startup struct {
		StartupYAML string `yaml:"file,omitempty"`
	} `yaml:"startup,omitempty"`

	Debug struct {
		LogLevel int `yaml:"cert-file,omitempty"`
	} `yaml:"debug,omitempty"`
}

var (
	config = flag.String("config", "", "YAML configuration file")
)

const (
	defaultServerPort = 10161
)

// NewConfig - load the gnmi server configuration from a file
func newConfig(configfile string) (*Config, error) {
	c := Config{}
	c.Address.IPAddress = ""
	c.Address.Port = defaultServerPort
	c.Control.DisableBundling = *gnmi.DisableBundling
	c.Control.DisableYDB = *model.DisableYdbChannel

	if configfile != "" {
		f, err := os.Open(configfile)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		decoder := yaml.NewDecoder(f)
		err = decoder.Decode(&c)
		if err != nil {
			return nil, err
		}
		// for _, e := range c.GNMI.SupportedEncoding {
		// 	if _, ok := pb.Encoding_value[e]; !ok {
		// 		return nil, fmt.Errorf("invalid-encoding")
		// 	}
		// }
	} else {
		// for _, e := range supportedEncodings {
		// 	c.GNMI.SupportedEncoding = append(c.GNMI.SupportedEncoding, e.String())
		// }
	}
	// b, err := yaml.Marshal(&c)
	// if err == nil {
	// 	fmt.Println(string(b))
	// }
	return &c, nil
}
