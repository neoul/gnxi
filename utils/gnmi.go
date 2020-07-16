package utils

import (
	"errors"
	"fmt"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
)

// ValidateGNMIPath - checks the validation of the gNMI path.
func ValidateGNMIPath(path *gpb.Path) error {
	if path.GetElem() == nil && path.GetElement() != nil {
		return fmt.Errorf("deprecated path element")
	}
	return nil
}

// ValidateGNMIFullPath - check the validation of the gNMI full path
func ValidateGNMIFullPath(prefix, path *gpb.Path) error {
	if path.GetElem() == nil && path.GetElement() != nil {
		return fmt.Errorf("deprecated path element")
	}
	oPre, oPath := prefix.GetOrigin(), path.GetOrigin()
	switch {
	case oPre != "" && oPath != "":
		return errors.New("origin is set both in prefix and path")
	case oPath != "":
		if len(prefix.GetElem()) > 0 {
			return errors.New("path elements in prefix are set even though origin is set in path")
		}
	default:
	}
	return nil
}
