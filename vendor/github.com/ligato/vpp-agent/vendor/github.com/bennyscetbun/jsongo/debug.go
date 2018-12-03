package jsongo

import (
	"encoding/json"
	"fmt"
	"os"
)

//DebugPrint Print a JSONNode as json withindent
func (that *JSONNode) DebugPrint(prefix string) {
	asJSON, err := json.MarshalIndent(that, "", "  ")
	if err != nil {
		fmt.Printf("%s\n", err.Error())
		os.Exit(-1)
	}
	fmt.Printf("%s%s\n", prefix, asJSON)
}

func printfindent(indentlevel int, indentchar string, format string, args ...interface{}) {
	for i := 0; i < indentlevel; i++ {
		fmt.Printf("%s", indentchar)
	}
	fmt.Printf(format, args...)
}

func (that *JSONNode) debugProspectValue(indentlevel int, indentchar string) {
	printfindent(indentlevel, indentchar, "Is of Type: TypeValue\n")
	printfindent(indentlevel, indentchar, "Value of type: %T\n", that.Get())
	printfindent(indentlevel, indentchar, "%+v\n", that.Get())
}

func (that *JSONNode) debugProspectMap(indentlevel int, indentchar string) {
	printfindent(indentlevel, indentchar, "Is of Type: TypeMap\n")
	for key := range that.m {
		printfindent(indentlevel, indentchar, "%s:\n", key)
		that.m[key].DebugProspect(indentlevel+1, indentchar)
	}
}

func (that *JSONNode) debugProspectArray(indentlevel int, indentchar string) {
	printfindent(indentlevel, indentchar, "Is of Type: TypeArray\n")
	for key := range that.a {
		printfindent(indentlevel, indentchar, "[%d]:\n", key)
		that.a[key].DebugProspect(indentlevel+1, indentchar)
	}
}

//DebugProspect Print all the data the we ve got on a node and all it s children
func (that *JSONNode) DebugProspect(indentlevel int, indentchar string) {
	switch that.t {
	case TypeValue:
		that.debugProspectValue(indentlevel, indentchar)
	case TypeMap:
		that.debugProspectMap(indentlevel, indentchar)
	case TypeArray:
		that.debugProspectArray(indentlevel, indentchar)
	case TypeUndefined:
		printfindent(indentlevel, indentchar, "Is of Type: TypeUndefined\n")
	}
}
