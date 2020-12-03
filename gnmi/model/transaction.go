package model

import (
	"fmt"

	"github.com/neoul/gtrie"
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

var setTranID uint

type setTransaction struct {
	id      uint
	seq     int
	opseqs  *gtrie.Trie
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
		opseqs:  gtrie.New(),
		delete:  []*opInfo{},
		replace: []*opInfo{},
		update:  []*opInfo{},
	}
	return set
}

func (set *setTransaction) setSequnce() {
	set.seq++
}

func (set *setTransaction) addOperation(optype opType, xpath *string, gpath *gnmipb.Path, curval interface{}) {
	opinfo := &opInfo{
		optype: optype,
		opseq:  set.seq,
		xpath:  xpath,
		gpath:  gpath,
		curval: curval,
	}
	opath := fmt.Sprintf("%s%s", optype, *xpath)
	set.opseqs.Add(opath, set.seq)
	switch optype {
	case opDelete:
		set.delete = append(set.delete, opinfo)
	case opReplace:
		set.replace = append(set.replace, opinfo)
	case opUpdate:
		set.update = append(set.update, opinfo)
	}
}

func (set *setTransaction) returnSetError(optype opType, seq int, err error) (int, error) {
	if serr, ok := err.(SetError); ok {
		opath := fmt.Sprintf("%s%s", optype, serr.ErrorPath())
		if _, fvalue, ok := set.opseqs.FindLongestMatchedPrefix(opath); ok {
			return fvalue.(int), err
		}
	}
	return seq, err
}
