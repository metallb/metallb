//  Copyright (c) 2018 Cisco and/or its affiliates.
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at:
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

// +build !windows,!darwin

package linuxcalls

import (
	"fmt"
	"syscall"
	"unsafe"
)

const (
	siocEthtool = 0x8946 // linux/sockios.h

	ethtoolGRxCsum = 0x00000014 // linux/ethtool.h
	ethtoolSRxCsum = 0x00000015 // linux/ethtool.h

	ethtoolGTxCsum = 0x00000016 // linux/ethtool.h
	ethtoolSTxCsum = 0x00000017 // linux/ethtool.h

	maxIfNameSize = 16 // linux/if.h
)

// linux/if.h 'struct ifreq'
type ifreq struct {
	Name [maxIfNameSize]byte
	Data uintptr
}

// linux/ethtool.h 'struct ethtool_value'
type ethtoolValue struct {
	Cmd  uint32
	Data uint32
}

// GetChecksumOffloading returns the state of Rx/Tx checksum offloading
// for the given interface.
func (h *NetLinkHandler) GetChecksumOffloading(ifName string) (rxOn, txOn bool, err error) {
	rxVal, err := ethtool(ifName, ethtoolGRxCsum, 0)
	if err != nil {
		return
	}
	txVal, err := ethtool(ifName, ethtoolGTxCsum, 0)
	if err != nil {
		return
	}
	return rxVal != 0, txVal != 0, nil
}

// SetChecksumOffloading enables/disables Rx/Tx checksum offloading
// for the given interface.
func (h *NetLinkHandler) SetChecksumOffloading(ifName string, rxOn, txOn bool) error {
	var rxVal, txVal uint32
	if rxOn {
		rxVal = 1
	}
	if txOn {
		txVal = 1
	}
	_, err := ethtool(ifName, ethtoolSRxCsum, rxVal)
	if err != nil {
		return err
	}
	_, err = ethtool(ifName, ethtoolSTxCsum, txVal)
	return err
}

// ethtool executes Linux ethtool syscall.
func ethtool(iface string, cmd, val uint32) (retval uint32, err error) {
	if len(iface)+1 > maxIfNameSize {
		return 0, fmt.Errorf("interface name is too long")
	}
	socket, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM, 0)
	if err != nil {
		return 0, err
	}
	defer syscall.Close(socket)

	// prepare ethtool request
	value := ethtoolValue{cmd, val}
	request := ifreq{Data: uintptr(unsafe.Pointer(&value))}
	copy(request.Name[:], iface)

	// ioctl system call
	_, _, errno := syscall.RawSyscall(syscall.SYS_IOCTL, uintptr(socket), uintptr(siocEthtool),
		uintptr(unsafe.Pointer(&request)))
	if errno != 0 {
		return 0, errno
	}
	return value.Data, nil
}
