package modeldata

import (
	"github.com/openconfig/ygot/ygot"
)

// DataCallback is an interface to be invoked by the modeled data control
type DataCallback interface {
}

// ConfigCallback is an interface to be invoked by the configuration
type ConfigCallback interface {
	DataCallback
	ConfigCallback(ygot.GoStruct) error
}

// OnChangeCallback is an interface to be invoked upon the model data changes
type OnChangeCallback interface {
	DataCallback
	OnChangeStarted(changes ygot.GoStruct)
	OnChangeCreated(path []string, changes ygot.GoStruct)
	OnChangeReplaced(path []string, changes ygot.GoStruct)
	OnChangeDeleted(path []string)
	OnChangeFinished(changes ygot.GoStruct)
}

func execConfigCallback(callback DataCallback, vgs ygot.GoStruct) error {
	if callback == nil || vgs == nil {
		return nil
	}
	configcb, ok := callback.(ConfigCallback)
	if ok {
		return configcb.ConfigCallback(vgs)
	}
	return nil
}
