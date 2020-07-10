// Package utils implements utilities for gnxi.
package utils

import (
	"flag"
	"fmt"

	"github.com/golang/protobuf/proto"
	"github.com/kylelemons/godebug/pretty"
	"github.com/neoul/libydb/go/ydb"
)

var (
	usePretty = flag.Bool("pretty", false, "Shows PROTOs using Pretty package instead of PROTO Text Marshal")
)

// PrintProto prints a Proto in a structured way.
func PrintProto(m proto.Message) {
	if *usePretty {
		pretty.Print(m)
		return
	}
	fmt.Println(proto.MarshalTextString(m))
}

// PrintStruct - print the type and value of the input structure with well-formed display.
func PrintStruct(value interface{}, excludedField ...string) {
	ydb.DebugValueString(value, 2, func(x ...interface{}) { fmt.Print(x...) }, excludedField...)
	fmt.Println()
}

// SprintStructInline - print the type and value of the input structure with well-formed display.
func SprintStructInline(value interface{}, excludedField ...string) string {
	return ydb.DebugValueStringInline(value, 0, nil, excludedField...)
}
