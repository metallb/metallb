go-mock
=======

A mocking framework for [Go](http://golang.org/).

Read online reference at http://godoc.org/github.com/maraino/go-mock

Status
------

[![Build Status](https://travis-ci.org/maraino/go-mock.svg)](https://travis-ci.org/maraino/go-mock)
[![Coverage Status](https://coveralls.io/repos/maraino/go-mock/badge.svg?branch=master&service=github)](https://coveralls.io/github/maraino/go-mock?branch=master)
[![GoDoc](https://godoc.org/github.com/maraino/go-mock?status.svg)](http://godoc.org/github.com/maraino/go-mock)

Usage
-----

Let's say that we have an interface like this that we want to Mock.

	type Client interface {
		Request(url *url.URL) (int, string, error)
	}


We need to create a new struct that implements the interface. But we will use
github.com/maraino/go-mock to replace the actual calls with some specific results.

	import (
		"github.com/maraino/go-mock"
		"net/url"
	)

	type MyClient struct {
		mock.Mock
	}

	func (c *MyClient) Request(url *url.URL) (int, string, error) {
		ret := c.Called(url)
		return ret.Int(0), ret.String(1), ret.Error(2)
	}

Then we need to configure the responses for the defined functions:

	c := &MyClient{}
	url, _ := url.Parse("http://www.example.org")
	c.When("Request", url).Return(200, "{result:1}", nil).Times(1)

We will execute the function that we have Mocked:

	code, json, err := c.Request(url)
	fmt.Printf("Code: %d, JSON: %s, Error: %v\n", code, json, err)

This will produce the output:

	Code: 200, JSON: {result:1}, Error: <nil>

And finally if we want to verify the number of calls we can use:

	if ok, err := c.Verify(); !ok {
		fmt.Println(err)
	}

API Reference
-------------

### func (m *Mock) When(name string, arguments ...interface{}) *MockFunction

Creates an stub for a specific function and a list of arguments.

	c.When("FunctionName", argument1, argument2, ...)

It returns a mock.MockFunction that can be used to configure the behavior and
validations when the method is called

### func (m *Mock) Reset() *Mock

Removes all the defined stubs and returns a clean mock.

### func (m *Mock) Verify() (bool, error)

Checks all validations and return if true if they are ok, or false and an error
if at least one validation have failed.

### func (m *Mock) Called(arguments ...interface{}) *MockResult

Called must be used in the struct that implements the interface that we want to mock.
It's the code that glues that struct with the go-mock package.

We will need to implement the interface and then use Called with the function arguments
and use the return value to return the values to our mocked struct.

	type Map interface {
		Set(key string, value interface{})
		Get(key string) (interface{}, error)
		GetString(key string) (string, error)
		Load(key string, value interface{}) error
	}

	type MyMap struct {
		mock.Mock
	}

	func (m *MyMap) Set(key string, value interface{}) {
		m.Called(key, value)
	}

	func (m *MyMap) Get(key string) (interface{}, error) {
		ret := m.Called(key)
		return ret.Get(0), ret.Error(1)
	}

	func (m *MyMap) GetString(key string) (string, error) {
		ret := m.Called(key)
		return ret.String(0), ret.Error(1)
	}

	func (m *MyMap) Load(key string, value interface{}) error {
		ret := m.Called(key, value)
		return ret.Error(0)
	}

### func (f *MockFunction) Return(v ...interface{}) *MockFunction

Defines the return parameters of our stub. The use of it is pretty simple, we
can simply chain mock.When with Return to set the return values.

	m.When("Get", "a-test-key").Return("a-test-value", nil)
	m.When("GetString", "a-test-key").Return("a-test-value", nil)
	m.When("Get", "another-test-key").Return(123, nil)
	m.When("Get", mock.Any).Return(nil, errors.New("not-found"))

If no return values are set, the method will return 0 for numeric types,
false for bools, "" for strings and nil for errors or any other type.

### func (f *MockFunction) ReturnToArgument(n int, v interface{}) *MockFunction

Defines a special return parameter to an argument of the function. We can also chain
this method to a When or a Return.

	m.When("Load", "a-test-key").ReturnToArgument(1, "a-test-value")
	m.When("Load", "another-test-key").Return(nil).ReturnToArgument(1, 123)

### func (f *MockFunction) Panic(v interface{}) *MockFunction

Panic will cause a panic when the stub method is called with the specified parameters.

	m.When("Get", "foobar").Panic("internal error")

### func (f *MockFunction) Times(a int) *MockFunction

Defines the exact number of times a method should be called. This is validated if mock.Verify
is executed.

	m.When("Get", "a-test-key").Return("a-test-value", nil).Times(1)

### func (f *MockFunction) AtLeast(a int) *MockFunction

Defines the minimum number of times a method should be called. This is validated if mock.Verify
is executed.

	m.When("Get", "a-test-key").Return("a-test-value", nil).AtLeast(2)

### func (f *MockFunction) AtMost(a int) *MockFunction

Defines the maximum number of times a method should be called. This is validated if mock.Verify
is executed.

	m.When("Get", "a-test-key").Return("a-test-value", nil).AtMost(1)

### func (f *MockFunction) Between(a, b int) *MockFunction

Defines a range of times a method should be called. This is validated if mock.Verify
is executed.

	m.When("Get", "a-test-key").Return("a-test-value", nil).Between(2, 5)

### func (f *MockFunction) Timeout(d time.Duration) *MockFunction

Defines a timeout to sleep before returning the value of a function.

	m.When("Get", "a-test-key").Return("a-test-value", nil).Timeout(100 * time.Millisecond)

### func (f *MockFunction) Call(call interface{}) *MockFunction

Defines a custom function that will be executed instead of the function in the stub.
The return values of the function will be used as the return values for the stub.

	datastore := make(map[string]interface{})

	m.When("Get", mock.Any).Call(func(key string) (interface{}, error) {
		if i, ok := datastore[key]; ok {
			return i, nil
		} else {
			return nil, ErrNotFound
		}
	})

	m.When("Set", mock.Any, mock.Any).Call(func(key string, value interface{}) error {
		datastore[key] = value
		return nil
	})
