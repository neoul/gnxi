package model

import (
	"fmt"
)

// Op [Merge,Replace,Delete]
type Op int

const (
	// OpMerge - Operation of SetPathData
	OpMerge Op = iota
	// OpReplace - Operation of SetPathData
	OpReplace
	// OpDelete - Operation of SetPathData
	OpDelete
)

func (op Op) String() string {
	switch op {
	case OpMerge:
		return "merge"
	case OpReplace:
		return "replace"
	case OpDelete:
		return "delete"
	}
	return "unknown"
}

// SetPathData - is used to store Operation [Create,Replace,Delete], XPATH, raw data from gNMI Set request.
type SetPathData struct {
	Op    Op
	XPath string
	data  interface{}
}

func (pathdata *SetPathData) String() string {
	return fmt.Sprintf("%s%s%v", pathdata.Op, pathdata.XPath, pathdata.data)
}

// gostruct functions
// ygot.DeepCopy()
