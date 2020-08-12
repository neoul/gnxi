package utilities

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
	if path == nil {
		return fmt.Errorf("no path input")
	}
	if path.GetElem() == nil && path.GetElement() != nil {
		return fmt.Errorf("deprecated path element")
	}
	if prefix == nil {
		return nil
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

// GNMIFullPath builds the full path from the prefix and path.
func GNMIFullPath(prefix, path *gpb.Path) *gpb.Path {
	if prefix == nil {
		if path == nil {
			return &gpb.Path{}
		}
		return path
	}
	fullPath := &gpb.Path{Origin: path.Origin}
	if path.GetElement() != nil {
		fullPath.Element = append(prefix.GetElement(), path.GetElement()...)
	}
	if path.GetElem() != nil {
		fullPath.Elem = append(prefix.GetElem(), path.GetElem()...)
	}
	return fullPath
}

// IsSchemaPath - returns the path is schema path.
func IsSchemaPath(prefix, path *gpb.Path) bool {
	isSchemaPath := true
	if prefix != nil {
		for _, e := range prefix.Elem {
			if e.Key != nil && len(e.Key) > 0 {
				isSchemaPath = false
			}
		}
	}
	if path != nil {
		for _, e := range path.Elem {
			if e.Key != nil && len(e.Key) > 0 {
				isSchemaPath = false
			}
		}
	}
	return isSchemaPath
}
