// Package mock provides a mocking framework for Go.
//
// https://github.com/maraino/go-mock
package mock

import (
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/kr/pretty"
)

// Mock should be embedded in the struct that we want to act as a Mock.
//
// Example:
// 		type MyClient struct {
//			mock.Mock
// 		}
type Mock struct {
	Functions []*MockFunction
	inorder   bool
	order     uint

	mutex sync.Mutex
}

type MockCountCheckType int

const (
	NONE MockCountCheckType = iota
	TIMES
	AT_LEAST
	AT_MOST
	BETWEEN
)

// MockFunction is the struct used to store the properties of a method stub.
type MockFunction struct {
	Name              string
	Arguments         []interface{}
	ReturnValues      []interface{}
	ReturnToArguments []MockReturnToArgument
	PanicValue        interface{}
	count             int
	countCheck        MockCountCheckType
	times             [2]int
	order             uint
	timeout           time.Duration
	call              reflect.Value
}

// MockReturnToArgument defines the function arguments used as return parameters.
type MockReturnToArgument struct {
	Argument int
	Value    interface{}
}

// MockResult struct is used to store the return arguments of a method stub.
type MockResult struct {
	Result []interface{}
}

// AnyType defines the type used as a replacement for any kind of argument in the stub configuration.
type AnyType string

// Any is the constant that should be used to represent a AnyType.
//
// Example:
//		mock.When("MyMethod", mock.Any, mock.Any).Return(0)
const (
	Any AnyType = "mock.any"
)

// AnythingOfType defines the type used as a replacement for an argument of a
// specific type in the stub configuration.
type AnythingOfType string

// AnyOfType is a helper to define AnythingOfType arguments.
//
// Example:
// 		mock.When("MyMethod", mock.AnyOfType("int"), mock.AnyOfType("string")).Return(0)
func AnyOfType(t string) AnythingOfType {
	return AnythingOfType(t)
}

// AnyIfType defines the type used as an argument that satisfies a condition.
type AnyIfType func(interface{}) bool

// AnyIf is a helper to define AnyIfType arguments.
//
// Example:
// 		f := func(i interface{}) bool {
// 			ii, ok := i.(MyType)
// 			return ok && ii.ID = "the-id"
// 		}
// 		mock.When("MyMethod", mock.AnyIf(f)).Return(0)
func AnyIf(f func(interface{}) bool) AnyIfType {
	return AnyIfType(f)
}

// RestType indicates there may optionally be one or more remaining elements.
type RestType string

// Rest indicates there may optionally be one or more remaining elements.
//
// Example:
//     mock.When("MyMethod", mock.Slice(123, mock.Rest)).Return(0)
const Rest RestType = "mock.rest"

func match(actual, expected interface{}) bool {
	switch expected := expected.(type) {
	case AnyType:
		return true

	case AnyIfType:
		return expected(actual)

	case AnythingOfType:
		return reflect.TypeOf(actual).String() == string(expected)

	default:
		if expected == nil {
			if actual == nil {
				return true
			} else {
				var v = reflect.ValueOf(actual)

				return v.CanInterface() && v.IsNil()
			}
		} else {
			if reflect.DeepEqual(actual, expected) {
				return true
			} else {
				return reflect.ValueOf(actual) == reflect.ValueOf(expected)
			}
		}
	}
}

// Slice is a helper to define AnyIfType arguments for slices and their elements.
//
// Example:
//     mock.When("MyMethod", mock.Slice(123, mock.Rest)).Return(0)
func Slice(elements ...interface{}) AnyIfType {
	return AnyIf(func(argument interface{}) bool {
		var v = reflect.ValueOf(argument)

		if v.Kind() != reflect.Slice {
			return false
		}

		var el, al = len(elements), v.Len()

		if el == 0 {
			return al == 0
		}

		if elements[el-1] == Rest {
			el--

			if al < el {
				return false
			}
		} else if al != el {
			return false
		}

		for i := 0; i < el; i++ {
			if !match(v.Index(i).Interface(), elements[i]) {
				return false
			}
		}

		return true
	})
}

// Verify verifies the restrictions set in the stubbing.
func (m *Mock) Verify() (bool, error) {
	for i, f := range m.Functions {
		switch f.countCheck {
		case TIMES:
			if f.count != f.times[1] {
				return false, fmt.Errorf("Function #%d %s executed %d times, expected: %d", i+1, f.Name, f.count, f.times[1])
			}
		case AT_LEAST:
			if f.count < f.times[1] {
				return false, fmt.Errorf("Function #%d %s executed %d times, expected at least: %d", i+1, f.Name, f.count, f.times[1])
			}
		case AT_MOST:
			if f.count > f.times[1] {
				return false, fmt.Errorf("Function #%d %s executed %d times, expected at most: %d", i+1, f.Name, f.count, f.times[1])
			}
		case BETWEEN:
			if f.count < f.times[0] || f.count > f.times[1] {
				return false, fmt.Errorf("Function #%d %s executed %d times, expected between: [%d, %d]", i+1, f.Name, f.count, f.times[0], f.times[1])
			}
		}
	}
	return true, nil
}

// HasVerify is used as the input of VerifyMocks (Mock satisfies it, obviously)
type HasVerify interface {
	Verify() (bool, error)
}

// VerifyMocks verifies a list of mocks, and returns the first error, if any.
func VerifyMocks(mocks ...HasVerify) (bool, error) {
	for _, m := range mocks {
		if ok, err := m.Verify(); !ok {
			return ok, err
		}
	}
	return true, nil
}

// Used to represent a test we can fail, without importing the testing package
// Importing "testing" in a file not named *_test.go results in tons of test.* flags being added to any compiled binary including this package
type HasError interface {
	Error(...interface{})
}

// Fail the test if any of the mocks fail verification
func AssertVerifyMocks(t HasError, mocks ...HasVerify) {
	if ok, err := VerifyMocks(mocks...); !ok {
		t.Error(err)
	}
}

// Reset removes all stubs defined.
func (m *Mock) Reset() *Mock {
	defer m.mutex.Unlock()
	m.mutex.Lock()

	m.Functions = nil
	m.order = 0
	return m
}

// When defines an stub of one method with some specific arguments. It returns a *MockFunction
// that can be configured with Return, ReturnToArgument, Panic, ...
func (m *Mock) When(name string, arguments ...interface{}) *MockFunction {
	defer m.mutex.Unlock()
	m.mutex.Lock()

	f := &MockFunction{
		Name:      name,
		Arguments: arguments,
	}

	m.Functions = append(m.Functions, f)
	return f
}

// Called is the function used in the mocks to replace the actual task.
//
// Example:
// 		func (m *MyClient) Request(url string) (int, string, error) {
// 			r := m.Called(url)
//			return r.Int(0), r.String(1), r.Error(2)
// 		}
func (m *Mock) Called(arguments ...interface{}) *MockResult {
	var timeout time.Duration
	defer func() {
		m.mutex.Unlock()
		if timeout > 0 {
			time.Sleep(timeout)
		}
	}()
	m.mutex.Lock()

	pc, _, _, ok := runtime.Caller(1)
	if !ok {
		panic("Could not get the caller information")
	}

	functionPath := runtime.FuncForPC(pc).Name()
	parts := strings.Split(functionPath, ".")
	functionName := parts[len(parts)-1]

	if f := m.find(functionName, arguments...); f != nil {
		// Increase the counter
		f.count++
		f.order = m.order
		m.order++

		if f.call.IsValid() {
			typ := f.call.Type()
			numIn := typ.NumIn()
			numArgs := len(arguments)

			// Assign arguments in order.
			// Not all of them are strictly required.
			values := make([]reflect.Value, numIn)
			for i := 0; i < numIn; i++ {
				if i < numArgs {
					values[i] = reflect.ValueOf(arguments[i])
				} else {
					values[i] = reflect.Zero(typ.In(i))
				}
			}

			if typ.IsVariadic() {
				values = f.call.CallSlice(values)
			} else {
				values = f.call.Call(values)
			}

			f.ReturnValues = []interface{}{}
			for i := range values {
				f.ReturnValues = append(f.ReturnValues, values[i].Interface())
			}
		}

		timeout = f.timeout

		if f.PanicValue != nil {
			panic(f.PanicValue)
		}

		// Return to arguments
		for _, r := range f.ReturnToArguments {
			arg := arguments[r.Argument]
			argTyp := reflect.TypeOf(arg)
			argElem := reflect.ValueOf(arg).Elem()
			typ := reflect.TypeOf(r.Value)
			if typ.Kind() == reflect.Ptr {
				if typ == argTyp {
					// *type vs *type
					argElem.Set(reflect.ValueOf(r.Value).Elem())
				} else {
					// *type vs **type
					argElem.Set(reflect.ValueOf(r.Value))
				}
			} else {
				if typ == argTyp.Elem() {
					// type vs *type
					argElem.Set(reflect.ValueOf(r.Value))
				} else {
					// type vs **type
					value := reflect.New(typ).Elem()
					value.Set(reflect.ValueOf(r.Value))
					argElem.Set(value.Addr())
				}
			}
		}

		return &MockResult{f.ReturnValues}
	}

	var msg string
	if len(arguments) == 0 {
		msg = fmt.Sprintf("Mock call missing for %s()", functionName)
	} else {
		argsStr := pretty.Sprintf("%# v", arguments)
		argsStr = argsStr[15 : len(argsStr)-1]
		msg = fmt.Sprintf("Mock call missing for %s(%s)", functionName, argsStr)
	}
	panic(msg)
}

func (m *Mock) find(name string, arguments ...interface{}) *MockFunction {
	var ff *MockFunction

	for _, f := range m.Functions {
		if f.Name != name {
			continue
		}

		if len(f.Arguments) != len(arguments) {
			continue
		}

		found := true
		for i, arg := range f.Arguments {
			switch arg.(type) {
			case AnyType:
				continue
			case AnythingOfType:
				if string(arg.(AnythingOfType)) == reflect.TypeOf(arguments[i]).String() {
					continue
				} else {
					found = false
				}
			case AnyIfType:
				cond, ok := arg.(AnyIfType)
				if ok && cond(arguments[i]) {
					continue
				} else {
					found = false
				}
			default:
				v := reflect.ValueOf(arguments[i])
				if arg == nil && (arguments[i] == nil || (v.CanInterface() && v.IsNil())) {
					continue
				}

				if reflect.DeepEqual(arg, arguments[i]) || reflect.ValueOf(arg) == reflect.ValueOf(arguments[i]) {
					continue
				} else {
					found = false
				}
			}
		}

		if !found {
			continue
		}

		// Check if the count check is valid.
		// If it's not try to match another function.
		if f.isMaxCountCheck() {
			if ff == nil {
				ff = f
			}
			continue
		}

		return f
	}

	return ff
}

// Return defines the return values of a *MockFunction.
func (f *MockFunction) Return(v ...interface{}) *MockFunction {
	f.ReturnValues = append(f.ReturnValues, v...)
	return f
}

// ReturnToArgument defines the values returned to a specific argument of a *MockFunction.
func (f *MockFunction) ReturnToArgument(n int, v interface{}) *MockFunction {
	f.ReturnToArguments = append(f.ReturnToArguments, MockReturnToArgument{n, v})
	return f
}

// Panic defines a panic for a specific *MockFunction.
func (f *MockFunction) Panic(v interface{}) *MockFunction {
	f.PanicValue = v
	return f
}

// Times defines how many times a *MockFunction must be called.
// This is verified if mock.Verify is called.
func (f *MockFunction) Times(a int) *MockFunction {
	f.countCheck = TIMES
	f.times = [2]int{-1, a}
	return f
}

// AtLeast defines the number of times that a *MockFunction must be at least called.
// This is verified if mock.Verify is called.
func (f *MockFunction) AtLeast(a int) *MockFunction {
	f.countCheck = AT_LEAST
	f.times = [2]int{-1, a}
	return f
}

// AtMost defines the number of times that a *MockFunction must be at most called.
// This is verified if mock.Verify is called.
func (f *MockFunction) AtMost(a int) *MockFunction {
	f.countCheck = AT_MOST
	f.times = [2]int{-1, a}
	return f
}

// Between defines a range of times that a *MockFunction must be called.
// This is verified if mock.Verify is called.
func (f *MockFunction) Between(a, b int) *MockFunction {
	f.countCheck = BETWEEN
	f.times = [2]int{a, b}
	return f
}

// Timeout defines a timeout to sleep before returning the value of a function.
func (f *MockFunction) Timeout(d time.Duration) *MockFunction {
	f.timeout = d
	return f
}

// Call executes a function passed as an argument using the arguments pased to the stub.
// If the function returns any output parameters they will be used as a return arguments
// when the stub is called. If the call argument is not a function it will panic when
// the stub is executed.
//
// Example:
// 		mock.When("MyMethod", mock.Any, mock.Any).Call(func(a int, b int) int {
// 			return a+b
// 		})
func (f *MockFunction) Call(call interface{}) *MockFunction {
	f.call = reflect.ValueOf(call)
	return f
}

// Check if the number of times that a function has been called
// has reach the top range.
func (f *MockFunction) isMaxCountCheck() bool {
	switch f.countCheck {
	case TIMES:
		if f.count >= f.times[1] {
			return true
		}
	case AT_LEAST:
		// At least does not have a maximum
		return false
	case AT_MOST:
		if f.count >= f.times[1] {
			return true
		}
	case BETWEEN:
		if f.count >= f.times[1] {
			return true
		}
	}

	return false
}

// Contains returns true if the results have the index i, false otherwise.
func (r *MockResult) Contains(i int) bool {
	if len(r.Result) > i {
		return true
	} else {
		return false
	}
}

// Get returns a specific return parameter.
// If a result has not been set, it returns nil,
func (r *MockResult) Get(i int) interface{} {
	if r.Contains(i) {
		return r.Result[i]
	} else {
		return nil
	}
}

// GetType returns a specific return parameter with the same type of
// the second argument. A nil version of the type can be casted
// without causing a panic.
func (r *MockResult) GetType(i int, ii interface{}) interface{} {
	t := reflect.TypeOf(ii)
	if t == nil {
		panic(fmt.Sprintf("Could not get type information for %#v", ii))
	}
	v := reflect.New(t).Elem()
	if r.Contains(i) {
		if r.Result[i] != nil {
			v.Set(reflect.ValueOf(r.Result[i]))
		}
	}
	return v.Interface()
}

// Bool returns a specific return parameter as a bool.
// If a result has not been set, it returns false.
func (r *MockResult) Bool(i int) bool {
	if r.Contains(i) {
		return r.Result[i].(bool)
	} else {
		return false
	}
}

// Byte returns a specific return parameter as a byte.
// If a result has not been set, it returns 0.
func (r *MockResult) Byte(i int) byte {
	if r.Contains(i) {
		return r.Result[i].(byte)
	} else {
		return 0
	}
}

// Bytes returns a specific return parameter as a []byte.
// If a result has not been set, it returns nil.
func (r *MockResult) Bytes(i int) []byte {
	if r.Contains(i) {
		if rr := r.Result[i]; rr == nil {
			return nil
		} else {
			return rr.([]byte)
		}
	} else {
		return nil
	}
}

// Error returns a specific return parameter as an error.
// If a result has not been set, it returns nil.
func (r *MockResult) Error(i int) error {
	if r.Contains(i) && r.Result[i] != nil {
		return r.Result[i].(error)
	} else {
		return nil
	}
}

// Float32 returns a specific return parameter as a float32.
// If a result has not been set, it returns 0.
func (r *MockResult) Float32(i int) float32 {
	if r.Contains(i) {
		return r.Result[i].(float32)
	} else {
		return 0
	}
}

// Float64 returns a specific return parameter as a float64.
// If a result has not been set, it returns 0.
func (r *MockResult) Float64(i int) float64 {
	if r.Contains(i) {
		return r.Result[i].(float64)
	} else {
		return 0
	}
}

// Int returns a specific return parameter as an int.
// If a result has not been set, it returns 0.
func (r *MockResult) Int(i int) int {
	if r.Contains(i) {
		return r.Result[i].(int)
	} else {
		return 0
	}
}

// Int8 returns a specific return parameter as an int8.
// If a result has not been set, it returns 0.
func (r *MockResult) Int8(i int) int8 {
	if r.Contains(i) {
		return r.Result[i].(int8)
	} else {
		return 0
	}
}

// Int16 returns a specific return parameter as an int16.
// If a result has not been set, it returns 0.
func (r *MockResult) Int16(i int) int16 {
	if r.Contains(i) {
		return r.Result[i].(int16)
	} else {
		return 0
	}
}

// Int32 returns a specific return parameter as an int32.
// If a result has not been set, it returns 0.
func (r *MockResult) Int32(i int) int32 {
	if r.Contains(i) {
		return r.Result[i].(int32)
	} else {
		return 0
	}
}

// Int64 returns a specific return parameter as an int64.
// If a result has not been set, it returns 0.
func (r *MockResult) Int64(i int) int64 {
	if r.Contains(i) {
		return r.Result[i].(int64)
	} else {
		return 0
	}
}

// String returns a specific return parameter as a string.
// If a result has not been set, it returns "".
func (r *MockResult) String(i int) string {
	if r.Contains(i) {
		return r.Result[i].(string)
	} else {
		return ""
	}
}
