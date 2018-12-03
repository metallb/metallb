// Copyright (c) 2018 Cisco and/or its affiliates.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package linuxmock

import (
	"os"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

// SystemMock allows to mock netlink-related methods
type SystemMock struct {
	responses []*WhenStResp
	respCurr  int
	respMax   int
}

// NewSystemMock creates new instance of the mock and initializes response list
func NewSystemMock() *SystemMock {
	return &SystemMock{
		responses: make([]*WhenStResp, 0),
	}
}

// WhenStResp is helper struct with single method call and desired response items
type WhenStResp struct {
	methodName string
	items      []interface{}
}

// When defines name of the related method. It creates a new instance of WhenStResp with provided method name and
// stores it to the mock.
func (mock *SystemMock) When(name string) *WhenStResp {
	resp := &WhenStResp{
		methodName: name,
	}
	mock.responses = append(mock.responses, resp)
	return resp
}

// ThenReturn receives array of items, which are desired to be returned in mocked method defined in "When". The full
// logic is:
// - When('someMethod').ThenReturn('values')
//
// Provided values should match return types of method. If method returns multiple values and only one is provided,
// mock tries to parse the value and returns it, while others will be nil or empty.
//
// If method is called several times, all cases must be defined separately, even if the return value is the same:
// - When('method1').ThenReturn('val1')
// - When('method1').ThenReturn('val1')
//
// All mocked methods are evaluated in same order they were assigned.
func (when *WhenStResp) ThenReturn(item ...interface{}) {
	when.items = item
}

// Auxiliary method returns next return value for provided method as generic type
func (mock *SystemMock) getReturnValues(name string) (response []interface{}) {
	for i, resp := range mock.responses {
		if resp.methodName == name {
			// Remove used response but retain order
			mock.responses = append(mock.responses[:i], mock.responses[i+1:]...)
			return resp.items
		}
	}
	// Return empty response
	return
}

// OpenFile implements OperatingSystem.
func (mock *SystemMock) OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	items := mock.getReturnValues("OpenFile")
	if len(items) == 1 {
		switch typed := items[0].(type) {
		case *os.File:
			return typed, nil
		case error:
			return nil, typed
		}
	} else if len(items) == 2 {
		return items[0].(*os.File), items[1].(error)
	}
	return nil, nil
}

// MkDirAll implements OperatingSystem.
func (mock *SystemMock) MkDirAll(path string, perm os.FileMode) error {
	items := mock.getReturnValues("MkDirAll")
	if len(items) >= 1 {
		return items[0].(error)
	}
	return nil
}

// Remove implements OperatingSystem.
func (mock *SystemMock) Remove(name string) error {
	items := mock.getReturnValues("Remove")
	if len(items) >= 1 {
		return items[0].(error)
	}
	return nil
}

// Mount implements Syscall.
func (mock *SystemMock) Mount(source string, target string, fsType string, flags uintptr, data string) error {
	items := mock.getReturnValues("Mount")
	if len(items) >= 1 {
		return items[0].(error)
	}
	return nil
}

// Unmount implements Syscall.
func (mock *SystemMock) Unmount(target string, flags int) error {
	items := mock.getReturnValues("Unmount")
	if len(items) >= 1 {
		return items[0].(error)
	}
	return nil
}

// NewNetworkNamespace implements NetlinkNamespace.
func (mock *SystemMock) NewNetworkNamespace() (netns.NsHandle, error) {
	items := mock.getReturnValues("NewNetworkNamespace")
	if len(items) == 1 {
		switch typed := items[0].(type) {
		case netns.NsHandle:
			return typed, nil
		case error:
			return 0, typed
		}
	} else if len(items) == 2 {
		return items[0].(netns.NsHandle), items[1].(error)
	}
	return 0, nil
}

// GetNamespaceFromName implements NetNsNamespace.
func (mock *SystemMock) GetNamespaceFromName(name string) (netns.NsHandle, error) {
	items := mock.getReturnValues("GetNamespaceFromName")
	if len(items) == 1 {
		switch typed := items[0].(type) {
		case netns.NsHandle:
			return typed, nil
		case error:
			return 0, typed
		}
	} else if len(items) == 2 {
		return items[0].(netns.NsHandle), items[1].(error)
	}
	return 0, nil
}

// SetNamespace implements NetNsNamespace.
func (mock *SystemMock) SetNamespace(ns netns.NsHandle) error {
	items := mock.getReturnValues("SetNamespace")
	if len(items) >= 1 {
		return items[0].(error)
	}
	return nil
}

// LinkSetNsFd implements NetlinkNamespace.
func (mock *SystemMock) LinkSetNsFd(link netlink.Link, fd int) error {
	items := mock.getReturnValues("LinkSetNsFd")
	if len(items) >= 1 {
		return items[0].(error)
	}
	return nil
}
