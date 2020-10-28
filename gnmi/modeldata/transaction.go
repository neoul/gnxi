package modeldata

import (
	"fmt"

	"github.com/neoul/gnxi/gnmi/model"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
)

// opType [Update,Replace,Delete]
type opType int

const (
	// opUpdate - opType of SetPathData (Merge)
	opUpdate opType = iota
	// opReplace - opType of SetPathData
	opReplace
	// opDelete - opType of SetPathData
	opDelete
)

func (op opType) String() string {
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

type rollback struct {
	op     opType
	base   *model.DataAndPath
	target *model.DataAndPath
}

type opInfo struct {
	optype  opType
	path    *string
	cvalue  interface{}
	nvalue  *gnmipb.TypedValue
	created bool
}

func (opinfo *opInfo) getKey() string {
	return fmt.Sprintf("%s%s", opinfo.optype, *opinfo.path)
}

var setTranID int

type setTransaction struct {
	id      int
	delete  map[string]*opInfo
	replace map[string]*opInfo
	update  map[string]*opInfo
}

// start the setTransaction for modeldata
func startTransaction() *setTransaction {
	setTranID++
	t := &setTransaction{id: setTranID,
		delete:  map[string]*opInfo{},
		replace: map[string]*opInfo{},
		update:  map[string]*opInfo{},
	}
	return t
}

func (t *setTransaction) add(optype opType, path *string, cvalue interface{}, nvalue *gnmipb.TypedValue) {
	opinfo := &opInfo{
		optype: optype,
		path:   path,
		cvalue: cvalue,
		nvalue: nvalue,
	}
	switch optype {
	case opDelete:
		t.delete[opinfo.getKey()] = opinfo
	case opReplace:
		t.replace[opinfo.getKey()] = opinfo
	case opUpdate:
		t.update[opinfo.getKey()] = opinfo
	}
}