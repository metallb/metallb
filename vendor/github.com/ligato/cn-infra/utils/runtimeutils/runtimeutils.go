package runtimeutils

import (
	"bytes"
	"fmt"
	"reflect"
	"runtime"
	"strconv"
	"strings"
)

var goroutineSpace = []byte("goroutine ")

// GoroutineID returns current GO Routine ID (parsed from runtime.Stack)
func GoroutineID() uint64 {
	b := make([]byte, 64)
	b = b[:runtime.Stack(b, false)]
	// Parse the 4707 out of "goroutine 4707 ["
	b = bytes.TrimPrefix(b, goroutineSpace)
	i := bytes.IndexByte(b, ' ')
	if i < 0 {
		panic(fmt.Sprintf("No space found in %q", b))
	}
	b = b[:i]
	n, err := strconv.ParseUint(string(b), 10, 64)
	if err != nil {
		panic(fmt.Sprintf("Failed to parse goroutine ID out of %q: %v", b, err))
	}
	return n
}

// GetFunction returns metadata about function based on pointer to a function
//
// Example usage:
//
// 	func foo() {}
// 	GetFunction(foo)
func GetFunction(function interface{}) *runtime.Func {
	return runtime.FuncForPC(reflect.ValueOf(function).Pointer())
}

// GetFunctionName returns name of the function
//
// Example usage:
//
// 	func foo() {}
// 	GetFunctionName(foo) // returns string containing "foo" substring
func GetFunctionName(function interface{}) string {
	fullName := GetFunction(function).Name()
	name := strings.TrimSuffix(fullName, "-fm")
	dot := strings.LastIndex(name, ".")
	if dot > 0 && dot+1 < len(name) {
		return name[dot+1:]
	}
	return fullName
}
