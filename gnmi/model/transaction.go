package model

import (
	"github.com/neoul/gnxi/utilities/xpath"
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

type opInfo struct {
	optype opType
	opseq  int
	xpath  *string
	gpath  *gnmipb.Path
	curval interface{}
}

var setTranID int

type setTransaction struct {
	id      int
	seq     int
	opseqs  map[string]int
	delete  []*opInfo
	replace []*opInfo
	update  []*opInfo
}

// start the setTransaction for modeldata
func startTransaction() *setTransaction {
	setTranID++
	set := &setTransaction{
		id:      setTranID,
		seq:     -1,
		opseqs:  map[string]int{},
		delete:  []*opInfo{},
		replace: []*opInfo{},
		update:  []*opInfo{},
	}
	return set
}

func (set *setTransaction) setSequnce(gpath *gnmipb.Path) {
	set.seq++
	path := xpath.ToXPath(gpath)
	set.opseqs[path] = set.seq
}

func (set *setTransaction) addOperation(optype opType, xpath *string, gpath *gnmipb.Path, curval interface{}) {
	opinfo := &opInfo{
		optype: optype,
		opseq:  set.seq,
		xpath:  xpath,
		gpath:  gpath,
		curval: curval,
	}
	switch optype {
	case opDelete:
		set.delete = append(set.delete, opinfo)
	case opReplace:
		set.replace = append(set.replace, opinfo)
	case opUpdate:
		set.update = append(set.update, opinfo)
	}
}
