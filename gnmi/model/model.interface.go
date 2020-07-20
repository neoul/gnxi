package model

import "github.com/openconfig/ygot/ygot"

// DataCallback is an interface to be invoked by the modeled data control
type DataCallback interface {
}

// ConfigCallback is an interface to be invoked by the configuration
type ConfigCallback interface {
	DataCallback
	ConfigCallback(ygot.ValidatedGoStruct) error
}

// Operation - The operation types of the model data
type Operation int

const (
	// OpCreate - Data node is created
	OpCreate Operation = iota
	// OpReplace - Data node is replaced
	OpReplace
	// OpDelete - Data node is deleted
	OpDelete
)

var operationStr = [...]string{
	"Created",
	"Replaced",
	"Deleted",
}

func (s Operation) String() string { return operationStr[s%5] }

// OnChangeCallback is an interface to be invoked upon the model data changes
type OnChangeCallback interface {
	DataCallback
	OnChangeCallback(op Operation, schemaPath, dataPath string, value interface{})
}

func execConfigCallback(callback DataCallback, vgs ygot.ValidatedGoStruct) error {
	if callback == nil || vgs == nil {
		return nil
	}
	configcallback, ok := callback.(ConfigCallback)
	if ok {
		return configcallback.ConfigCallback(vgs)
	}
	return nil
}

func execOnChangeCallback(callback DataCallback, op Operation, schemaPath string, dataPath string, value interface{}) {
	if callback == nil {
		return
	}
	configcallback, ok := callback.(OnChangeCallback)
	if ok {
		configcallback.OnChangeCallback(op, schemaPath, dataPath, value)
	}
	return
}
