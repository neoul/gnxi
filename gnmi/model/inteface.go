package model

import (
	"github.com/openconfig/ygot/ygot"
)

// StateCallback is the default interface called back or called to, in order to update all modeled data.
type StateCallback interface {
	UpdateStart()
	UpdateCreate(path string, value string) error
	UpdateReplace(path string, value string) error
	UpdateDelete(path string) error
	UpdateEnd()
}

// ChangeNotification is an interface to be invoked upon the model data changes
type ChangeNotification interface {
	ChangeStarted(changes ygot.GoStruct)
	ChangeCreated(path string, changes ygot.GoStruct)
	ChangeReplaced(path string, changes ygot.GoStruct)
	ChangeDeleted(path string)
	ChangeFinished(changes ygot.GoStruct)
}

// StateConfig is an interface that must be implemented to the external system.
// The external system must configure the configuration changes and then update the modeled data via StateUpdate interface.
type StateConfig interface {
	StateCallback
}

// StateUpdate is an interface implemented to the MO & Model struct in gnmi/model.
// The StateUpdate must be invoked to update the state of a Model instance.
type StateUpdate interface {
	StateCallback
}

// StateSync interface is used to request the sync of the modeled data immediately.
// The external system must update the data requested by the path if it is invoked.
type StateSync interface {
	UpdateSync(path ...string) error
}
