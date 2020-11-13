package model

import (
	"github.com/openconfig/ygot/ygot"
)

// Callback is the interface invoked by the model to notify the model data change.
type Callback interface {
}

// ChangeNotification is an interface to be invoked upon the model data changes
type ChangeNotification interface {
	Callback
	ChangeStarted(changes ygot.GoStruct)
	ChangeCreated(path string, changes ygot.GoStruct)
	ChangeReplaced(path string, changes ygot.GoStruct)
	ChangeDeleted(path string)
	ChangeFinished(changes ygot.GoStruct)
}

// StateConfig interface is invoked by the Model to inform configuration changes to the system.
// The system must configure the configuration changes and then update the modeled data via StateUpdate interface.
type StateConfig interface {
	UpdateStart()
	UpdateCreate(path string, value string) error
	UpdateReplace(path string, value string) error
	UpdateDelete(path string) error
	UpdateEnd()
}

// StateUpdate interface is used for the update of modeled data.
// The system that has source data must invoke the StateUpdate interface to update the modeled data.
type StateUpdate interface {
	UpdateStart()
	UpdateCreate(path string, value string) error
	UpdateReplace(path string, value string) error
	UpdateDelete(path string) error
	UpdateEnd()
}

// StateUpdateSync interface is used to synchonize the modeled data.
// The system must update the data requested by the path if it is invoked.
type StateUpdateSync interface {
	UpdateSync(path string) error
}
