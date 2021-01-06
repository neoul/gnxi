package model

import (
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
)

// transUnitType [Update,Replace,Delete]
type transUnitType int

const (
	opNone transUnitType = iota
	// opUpdate - transUnitType of SetPathData (Merge)
	opUpdate
	// opReplace - transUnitType of SetPathData
	opReplace
	// opDelete - transUnitType of SetPathData
	opDelete
)

func (op transUnitType) String() string {
	switch op {
	case opUpdate: // = Merge
		return "U"
	case opReplace:
		return "R"
	case opDelete:
		return "D"
	}
	return "?"
}

type transUnit struct {
	optype gnmipb.UpdateResult_Operation
	xpath  *string
	gpath  *gnmipb.Path
	curval interface{}
}

var transID uint

type trans struct {
	id    uint
	seq   int
	units []*transUnit
}

// start the trans for modeldata
func newTrans() *trans {
	transID++
	set := &trans{
		id:    transID,
		seq:   -1,
		units: []*transUnit{},
	}
	return set
}

func (set *trans) newUnit(optype gnmipb.UpdateResult_Operation, xpath *string, gpath *gnmipb.Path, curval interface{}) *transUnit {
	unit := &transUnit{
		optype: optype,
		xpath:  xpath,
		gpath:  gpath,
		curval: curval,
	}
	set.units = append(set.units, unit)
	return unit
}
