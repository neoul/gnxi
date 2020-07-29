package config

import (
	"flag"
	"fmt"
	"os"
	"reflect"

	"gopkg.in/yaml.v2"
)

// Config - gNMI Server configuration data
type Config struct {
	ConfigFile  string `yaml:"config,omitempty"`
	BindAddress string `yaml:"bind_address"`

	NoTLS bool `yaml:"no-tls,omitempty"`
	TLS   struct {
		Insecure bool   `yaml:"insecure,omitempty"`
		CAFile   string `yaml:"ca-cert,omitempty"`
		CertFile string `yaml:"server-cert,omitempty"`
		KeyFile  string `yaml:"server-key,omitempty"`
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
		File string `yaml:"file,omitempty"`
	} `yaml:"startup,omitempty"`

	Debug struct {
		LogLevel int `yaml:"cert-file,omitempty"`
	} `yaml:"debug,omitempty"`
}

func getflag(name string, defValue interface{}) interface{} {
	f := flag.Lookup(name)
	if f == nil {
		return defValue
	}
	v := reflect.ValueOf(f.Value)
	if v.Kind() == reflect.Interface {
		v = v.Elem()
	}
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Bool:
		return v.Convert(reflect.TypeOf(false)).Interface()
	case reflect.String:
		return v.Convert(reflect.TypeOf("")).Interface()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		var i int
		return v.Convert(reflect.TypeOf(i)).Interface()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		var i uint
		return v.Convert(reflect.TypeOf(i)).Interface()
	case reflect.Float32, reflect.Float64:
		var f float64
		return v.Convert(reflect.TypeOf(f)).Interface()
	}
	return defValue
}

// NewConfig - load the gnmi server configuration from a file
func NewConfig(configfile string) (*Config, error) {
	c := Config{}
	c.BindAddress = getflag("bind_address", ":10161").(string)
	c.Startup.File = getflag("startup", c.Startup.File).(string)
	c.NoTLS = getflag("no-tls", c.NoTLS).(bool)
	if !c.NoTLS {
		c.TLS.Insecure = getflag("insecure", c.TLS.Insecure).(bool)
		c.TLS.CAFile = getflag("ca", c.TLS.CAFile).(string)
		c.TLS.CertFile = getflag("cert", c.TLS.CertFile).(string)
		c.TLS.KeyFile = getflag("key", c.TLS.KeyFile).(string)
	}
	c.Control.DisableBundling = getflag("disable-update-bundling", false).(bool)
	c.Startup.File = getflag("startup", c.Startup.File).(string)
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
		b, err := yaml.Marshal(&c)
		if err == nil {
			fmt.Println(string(b))
		}
		c.ConfigFile = getflag("config", "").(string)
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
