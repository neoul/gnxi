package model

import (
	"github.com/openconfig/ygot/ygot"
)

// Callback is the interface invoked by the model to notify the model data change.
type Callback interface {
}

// // ConfigCallback is an interface to be invoked by the configuration
// type ConfigCallback interface {
// 	Callback
// 	ConfigCallback(ygot.GoStruct) error
// }

// ChangeNotification is an interface to be invoked upon the model data changes
type ChangeNotification interface {
	Callback
	ChangeStarted(changes ygot.GoStruct)
	ChangeCreated(path []string, changes ygot.GoStruct)
	ChangeReplaced(path []string, changes ygot.GoStruct)
	ChangeDeleted(path []string)
	ChangeFinished(changes ygot.GoStruct)
}

// func execConfigCallback(callback Callback, vgs ygot.GoStruct) error {
// 	if callback == nil || vgs == nil {
// 		return nil
// 	}
// 	configcb, ok := callback.(ConfigCallback)
// 	if ok {
// 		return configcb.ConfigCallback(vgs)
// 	}
// 	return nil
// }
