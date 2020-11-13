package model

import (
	"github.com/openconfig/ygot/ygot"
)

// ChangeNotification is an interface to be invoked upon the model data changes
type ChangeNotification interface {
	ChangeStarted(changes ygot.GoStruct)
	ChangeCreated(path string, changes ygot.GoStruct)
	ChangeReplaced(path string, changes ygot.GoStruct)
	ChangeDeleted(path string)
	ChangeFinished(changes ygot.GoStruct)
}

// StateConfig is an interface that must be implemented to the data source (e.g. system)
// The system must configure the configuration changes and then update the modeled data via StateUpdate interface.
type StateConfig interface {
	UpdateStart()
	UpdateCreate(path string, value string) error
	UpdateReplace(path string, value string) error
	UpdateDelete(path string) error
	UpdateEnd()
}

// StateUpdate is an interface implemented to the MO & Model struct in gnmi/model.
// The StateUpdate must be invoked to update the state of a Model instance.
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
