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
	"net"

	"github.com/ligato/vpp-agent/plugins/linux/ifplugin/linuxcalls"

	"github.com/vishvananda/netlink"
)

// IfNetlinkHandlerMock allows to mock netlink-related methods
type IfNetlinkHandlerMock struct {
	responses []*WhenIfResp
	calls     []*called
	respCurr  int
	respMax   int
}

// NewIfNetlinkHandlerMock creates new instance of the mock and initializes response list
func NewIfNetlinkHandlerMock() *IfNetlinkHandlerMock {
	return &IfNetlinkHandlerMock{
		responses: make([]*WhenIfResp, 0),
	}
}

// WhenIfResp is helper struct with single method call and desired response items
type WhenIfResp struct {
	methodName string
	items      []interface{}
}

// called is helper struct with method name which was called including parameters
type called struct {
	methodName string
	params     []interface{}
}

// When defines name of the related method. It creates a new instance of WhenIfResp with provided method name and
// stores it to the mock.
func (mock *IfNetlinkHandlerMock) When(name string) *WhenIfResp {
	resp := &WhenIfResp{
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
func (when *WhenIfResp) ThenReturn(item ...interface{}) {
	when.items = item
}

// GetCallsFor returns number of calls for specific method including parameters used for every call
func (mock *IfNetlinkHandlerMock) GetCallsFor(name string) (numCalls int, params map[int][]interface{}) {
	params = make(map[int][]interface{})
	index := 0
	for _, call := range mock.calls {
		if call.methodName == name {
			index++
			params[index] = call.params
		}
	}
	return index, params
}

// Auxiliary method returns next return value for provided method as generic type
func (mock *IfNetlinkHandlerMock) getReturnValues(name string) (response []interface{}) {
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

// Auxiliary method adds method/params entry to the mock
func (mock *IfNetlinkHandlerMock) addCalled(name string, params ...interface{}) {
	var parameters []interface{}
	parameters = append(parameters, params)
	mock.calls = append(mock.calls, &called{methodName: name, params: params})
}

/* Mocked netlink handler methods */

// AddVethInterfacePair implements NetlinkAPI.
func (mock *IfNetlinkHandlerMock) AddVethInterfacePair(ifName, peerIfName string) error {
	items := mock.getReturnValues("AddVethInterfacePair")
	if len(items) >= 1 {
		return items[0].(error)
	}
	return nil
}

// DelVethInterfacePair implements NetlinkAPI.
func (mock *IfNetlinkHandlerMock) DelVethInterfacePair(ifName, peerIfName string) error {
	items := mock.getReturnValues("DelVethInterfacePair")
	if len(items) >= 1 {
		return items[0].(error)
	}
	return nil
}

// SetInterfaceUp implements NetlinkAPI.
func (mock *IfNetlinkHandlerMock) SetInterfaceUp(ifName string) error {
	items := mock.getReturnValues("SetInterfaceUp")
	if len(items) >= 1 {
		return items[0].(error)
	}
	return nil
}

// SetInterfaceDown implements NetlinkAPI.
func (mock *IfNetlinkHandlerMock) SetInterfaceDown(ifName string) error {
	items := mock.getReturnValues("SetInterfaceDown")
	if len(items) >= 1 {
		return items[0].(error)
	}
	return nil
}

// AddInterfaceIP implements NetlinkAPI.
func (mock *IfNetlinkHandlerMock) AddInterfaceIP(ifName string, addr *net.IPNet) error {
	mock.addCalled("AddInterfaceIP", ifName, addr)
	items := mock.getReturnValues("AddInterfaceIP")
	if len(items) >= 1 {
		return items[0].(error)
	}
	return nil
}

// DelInterfaceIP implements NetlinkAPI.
func (mock *IfNetlinkHandlerMock) DelInterfaceIP(ifName string, addr *net.IPNet) error {
	items := mock.getReturnValues("DelInterfaceIP")
	if len(items) >= 1 {
		return items[0].(error)
	}
	return nil
}

// SetInterfaceMac implements NetlinkAPI.
func (mock *IfNetlinkHandlerMock) SetInterfaceMac(ifName string, macAddress string) error {
	items := mock.getReturnValues("SetInterfaceMac")
	if len(items) >= 1 {
		return items[0].(error)
	}
	return nil
}

// SetInterfaceMTU implements NetlinkAPI.
func (mock *IfNetlinkHandlerMock) SetInterfaceMTU(ifName string, mtu int) error {
	items := mock.getReturnValues("SetInterfaceMTU")
	if len(items) >= 1 {
		return items[0].(error)
	}
	return nil
}

// RenameInterface implements NetlinkAPI.
func (mock *IfNetlinkHandlerMock) RenameInterface(ifName string, newName string) error {
	items := mock.getReturnValues("RenameInterface")
	if len(items) >= 1 {
		return items[0].(error)
	}
	return nil
}

// GetLinkByName implements NetlinkAPI.
func (mock *IfNetlinkHandlerMock) GetLinkByName(ifName string) (netlink.Link, error) {
	items := mock.getReturnValues("GetLinkByName")
	if len(items) == 1 {
		switch typed := items[0].(type) {
		case netlink.Link:
			return typed, nil
		case error:
			return nil, typed
		}
	} else if len(items) == 2 {
		return items[0].(netlink.Link), items[1].(error)
	}
	return nil, nil
}

// GetLinkList implements NetlinkAPI.
func (mock *IfNetlinkHandlerMock) GetLinkList() ([]netlink.Link, error) {
	items := mock.getReturnValues("GetLinkList")
	if len(items) == 1 {
		switch typed := items[0].(type) {
		case []netlink.Link:
			return typed, nil
		case error:
			return nil, typed
		}
	} else if len(items) == 2 {
		return items[0].([]netlink.Link), items[1].(error)
	}
	return nil, nil
}

// GetAddressList implements NetlinkAPI.
func (mock *IfNetlinkHandlerMock) GetAddressList(ifName string) ([]netlink.Addr, error) {
	items := mock.getReturnValues("GetAddressList")
	if len(items) == 1 {
		switch typed := items[0].(type) {
		case []netlink.Addr:
			return typed, nil
		case error:
			return nil, typed
		}
	} else if len(items) == 2 {
		return items[0].([]netlink.Addr), items[1].(error)
	}
	return nil, nil
}

// InterfaceExists implements NetlinkAPI.
func (mock *IfNetlinkHandlerMock) InterfaceExists(ifName string) (bool, error) {
	items := mock.getReturnValues("InterfaceExists")
	if len(items) == 1 {
		switch typed := items[0].(type) {
		case bool:
			return typed, nil
		case error:
			return false, typed
		}
	} else if len(items) == 2 {
		return items[0].(bool), items[1].(error)
	}
	return false, nil
}

// GetInterfaceType implements NetlinkAPI.
func (mock *IfNetlinkHandlerMock) GetInterfaceType(ifName string) (string, error) {
	items := mock.getReturnValues("GetInterfaceType")
	if len(items) == 1 {
		switch typed := items[0].(type) {
		case string:
			return typed, nil
		case error:
			return "", typed
		}
	} else if len(items) == 2 {
		return items[0].(string), items[1].(error)
	}
	return "", nil
}

// GetVethPeerName implements NetlinkAPI.
func (mock *IfNetlinkHandlerMock) GetVethPeerName(ifName string) (string, error) {
	items := mock.getReturnValues("GetVethPeerName")
	if len(items) == 1 {
		switch typed := items[0].(type) {
		case string:
			return typed, nil
		case error:
			return "", typed
		}
	} else if len(items) == 2 {
		return items[0].(string), items[1].(error)
	}
	return "", nil
}

// GetInterfaceByName implements NetlinkAPI.
func (mock *IfNetlinkHandlerMock) GetInterfaceByName(ifName string) (*net.Interface, error) {
	items := mock.getReturnValues("GetInterfaceByName")
	if len(items) == 1 {
		switch typed := items[0].(type) {
		case *net.Interface:
			return typed, nil
		case error:
			return nil, typed
		}
	} else if len(items) == 2 {
		return items[0].(*net.Interface), items[1].(error)
	}
	return nil, nil
}

// DumpInterfaces does not return a value
func (mock *IfNetlinkHandlerMock) DumpInterfaces() ([]*linuxcalls.LinuxInterfaceDetails, error) {
	return nil, nil
}

// DumpInterfaceStatistics  does not return a value
func (mock *IfNetlinkHandlerMock) DumpInterfaceStatistics() ([]*linuxcalls.LinuxInterfaceStatistics, error) {
	return nil, nil
}
