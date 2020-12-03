package model

import (
	"fmt"

	"github.com/neoul/gnxi/utilities/xpath"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
)

// opType [Update,Replace,Delete]
type opType int

const (
	opNone opType = iota
	// opUpdate - opType of SetPathData (Merge)
	opUpdate
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

func (set *setTransaction) setSequnce(optype opType, gpath *gnmipb.Path) {
	set.seq++
	path := xpath.ToXPath(gpath)
	opXPath := fmt.Sprintf("%s%s", optype, path)
	set.opseqs[opXPath] = set.seq
}

func (set *setTransaction) addOperation(optype opType, xpath *string, gpath *gnmipb.Path, curval interface{}) {
	opinfo := &opInfo{
		optype: optype,
		opseq:  set.seq,
		xpath:  xpath,
		gpath:  gpath,
		curval: curval,
	}
	opXPath := fmt.Sprintf("%s%s", optype, *xpath)
	set.opseqs[opXPath] = set.seq
	switch optype {
	case opDelete:
		set.delete = append(set.delete, opinfo)
	case opReplace:
		set.replace = append(set.replace, opinfo)
	case opUpdate:
		set.update = append(set.update, opinfo)
	}
}

func (set *setTransaction) returnSetErr(optype opType, seq int, err error) (int, error) {
	if serr, ok := err.(SetError); ok {
		path := fmt.Sprintf("%s%s", optype, serr.ErrorPath())
		foundseq := set.opseqs[path]
		return foundseq, err
	}
	return seq, err
}
