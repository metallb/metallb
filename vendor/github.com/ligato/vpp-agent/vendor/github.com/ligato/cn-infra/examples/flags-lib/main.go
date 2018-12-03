package main

import (
	"fmt"
	"time"

	"github.com/namsral/flag"
)

// *************************************************************************
// This file contains example of how to register CLI flags and how to show
// their runtime values.
// ************************************************************************/

func main() {
	RegisterFlags()
	ParseFlags()
	PrintFlags()
}

// Flag variables
var (
	testFlagString string
	testFlagInt    int
	testFlagInt64  int64
	testFlagUint   uint
	testFlagUint64 uint64
	testFlagBool   bool
	testFlagDur    time.Duration
)

// RegisterFlags contains examples of how to register flags of various types.
func RegisterFlags() {
	fmt.Println("Registering flags...")
	flag.StringVar(&testFlagString, "ep-string", "my-value",
		"Example of a string flag.")
	flag.IntVar(&testFlagInt, "ep-int", 1122,
		"Example of an int flag.")
	flag.Int64Var(&testFlagInt64, "ep-int64", -3344,
		"Example of an int64 flag.")
	flag.UintVar(&testFlagUint, "ep-uint", 5566,
		"Example of a uint flag.")
	flag.Uint64Var(&testFlagUint64, "ep-uint64", 7788,
		"Example of a uint64 flag.")
	flag.BoolVar(&testFlagBool, "ep-bool", true,
		"Example of a bool flag.")
	flag.DurationVar(&testFlagDur, "ep-duration", time.Second*5,
		"Example of a duration flag.")
}

// ParseFlags parses the command-line flags.
func ParseFlags() {
	flag.Parse()
}

// PrintFlags shows the runtime values of CLI flags.
func PrintFlags() {
	fmt.Println("Printing flags...")
	fmt.Printf("testFlagString:'%s'\n", testFlagString)
	fmt.Printf("testFlagInt:'%d'\n", testFlagInt)
	fmt.Printf("testFlagInt64:'%d'\n", testFlagInt64)
	fmt.Printf("testFlagUint:'%d'\n", testFlagUint)
	fmt.Printf("testFlagUint64:'%d'\n", testFlagUint64)
	fmt.Printf("testFlagBool:'%v'\n", testFlagBool)
	fmt.Printf("testFlagDur:'%v'\n", testFlagDur)
}
