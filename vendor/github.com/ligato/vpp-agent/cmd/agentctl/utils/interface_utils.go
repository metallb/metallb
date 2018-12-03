// Copyright (c) 2017 Cisco and/or its affiliates.
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

package utils

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/ligato/cn-infra/db/keyval"
	"github.com/ligato/vpp-agent/plugins/vpp/model/interfaces"
)

// Interface flag names
const (
	HostIfName       = "host-if-name"
	MemifMaster      = "master"
	MemifMode        = "mode"
	MemifID          = "id"
	MemifSktFileName = "sock-filename"
	MemifSecret      = "secret"
	MemifRingSize    = "ring-size"
	MemifBufferSize  = "buffer-size"
	MemifRxQueues    = "rx-queues"
	MemifTxQueues    = "tx-queues"
	VxLanSrcAddr     = "src-addr"
	VxLanDstAddr     = "dst-addr"
	VxLanVni         = "vni"
)

// WriteInterfaceToDb validates and writes interface to the etcd.
func WriteInterfaceToDb(db keyval.ProtoBroker, key string, ifc *interfaces.Interfaces_Interface) {
	validateInterface(ifc)
	db.Put(key, ifc)
}

// GetInterfaceKeyAndValue returns true if an interface with the specified name
// was found together with the interface key, and data, and data broker.
func GetInterfaceKeyAndValue(endpoints []string, label string, ifName string) (bool, string, *interfaces.Interfaces_Interface, keyval.ProtoBroker) {
	validateIfIdentifiers(label, ifName)
	db, err := GetDbForOneAgent(endpoints, label)
	if err != nil {
		ExitWithError(ExitBadConnection, err)
	}

	key := interfaces.InterfaceKey(ifName)
	ifc := &interfaces.Interfaces_Interface{}

	found, _, err := db.GetValue(key, ifc)
	if err != nil {
		ExitWithError(ExitError, errors.New("Error getting existing config - "+err.Error()))
	}

	return found, key, ifc, db
}

// IsFlagPresent verifies flag presence in the OS args.
func IsFlagPresent(flag string) bool {
	arg := "--" + flag
	for _, b := range os.Args {
		if strings.Split(b, "=")[0] == arg {
			return true
		}
	}
	return false
}

// UpdateIpv4Address updates interface's IPv4 address.
func UpdateIpv4Address(old []string, updates []string) []string {
Loop:
	for i := range updates {
		validateIpv4AddrCIDR(updates[i])
		addr := updates[i]
		for j := range old {
			if old[j] == addr {
				old[j] = addr
				break Loop
			}
		}
		old = append(old, addr)
	}
	return old
}

// UpdateIpv6Address updates interface's IPv6 address.
func UpdateIpv6Address(old []string, updates []string) []string {
Loop:
	for i := range updates {
		validateIpv6AddrCIDR(updates[i])

		addr := updates[i]
		for j := range old {
			if old[j] == addr {
				old[j] = addr
				break Loop
			}
		}
		old = append(old, addr)
	}
	return old
}

// ValidatePhyAddr validates string representation of MAC address.
func ValidatePhyAddr(pAddr string) {
	match, err := regexp.MatchString("^([0-9a-fA-F][0-9a-fA-F]:){5}([0-9a-fA-F][0-9a-fA-F])$", pAddr)
	if err != nil {
		ExitWithError(ExitIO, errors.New("Failed to parse Physical Address: "+err.Error()))
	}
	if !match {
		ExitWithError(ExitIO, errors.New("Invalid Phy Address: "+pAddr))
	}
}

// ValidateIpv4Addr validates string representation of IPv4 address.
func ValidateIpv4Addr(ipv4Addr string) bool {
	match, err := regexp.MatchString("^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$", ipv4Addr)
	if err != nil {
		ExitWithError(ExitIO, errors.New("Failed to parse IPv4 Address: "+err.Error()))
	}
	return match
}

// ValidateIpv6Addr validates string representation of IPv6 address.
func ValidateIpv6Addr(ipv6Addr string) bool {
	match, err := regexp.MatchString("^s*((([0-9A-Fa-f]{1,4}:){7}([0-9A-Fa-f]{1,4}|:))|(([0-9A-Fa-f]{1,4}:){6}(:[0-9A-Fa-f]{1,4}|((25[0-5]|2[0-4]d|1dd|[1-9]?d)(.(25[0-5]|2[0-4]d|1dd|[1-9]?d)){3})|:))|(([0-9A-Fa-f]{1,4}:){5}(((:[0-9A-Fa-f]{1,4}){1,2})|:((25[0-5]|2[0-4]d|1dd|[1-9]?d)(.(25[0-5]|2[0-4]d|1dd|[1-9]?d)){3})|:))|(([0-9A-Fa-f]{1,4}:){4}(((:[0-9A-Fa-f]{1,4}){1,3})|((:[0-9A-Fa-f]{1,4})?:((25[0-5]|2[0-4]d|1dd|[1-9]?d)(.(25[0-5]|2[0-4]d|1dd|[1-9]?d)){3}))|:))|(([0-9A-Fa-f]{1,4}:){3}(((:[0-9A-Fa-f]{1,4}){1,4})|((:[0-9A-Fa-f]{1,4}){0,2}:((25[0-5]|2[0-4]d|1dd|[1-9]?d)(.(25[0-5]|2[0-4]d|1dd|[1-9]?d)){3}))|:))|(([0-9A-Fa-f]{1,4}:){2}(((:[0-9A-Fa-f]{1,4}){1,5})|((:[0-9A-Fa-f]{1,4}){0,3}:((25[0-5]|2[0-4]d|1dd|[1-9]?d)(.(25[0-5]|2[0-4]d|1dd|[1-9]?d)){3}))|:))|(([0-9A-Fa-f]{1,4}:){1}(((:[0-9A-Fa-f]{1,4}){1,6})|((:[0-9A-Fa-f]{1,4}){0,4}:((25[0-5]|2[0-4]d|1dd|[1-9]?d)(.(25[0-5]|2[0-4]d|1dd|[1-9]?d)){3}))|:))|(:(((:[0-9A-Fa-f]{1,4}){1,7})|((:[0-9A-Fa-f]{1,4}){0,5}:((25[0-5]|2[0-4]d|1dd|[1-9]?d)(.(25[0-5]|2[0-4]d|1dd|[1-9]?d)){3}))|:)))(%.+)?s*$", ipv6Addr)
	if err != nil {
		ExitWithError(ExitIO, errors.New("Failed to parse IPv6 Address: "+err.Error()))
	}
	return match
}

func validateInterface(ifc *interfaces.Interfaces_Interface) {
	fmt.Printf("Validating interface\n ifc: %+v\n", ifc)
}

func validateIpv4AddrCIDR(ipv4Addr string) {
	match, err := regexp.MatchString("^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])(\\/([0-9]|[1-2][0-9]|3[0-2]))$", ipv4Addr)
	if err != nil {
		ExitWithError(ExitIO, errors.New("Failed to parse IPv4 Address: "+err.Error()))
	}
	if !match {
		ExitWithError(ExitIO, errors.New("Invalid Ipv4 Address: "+ipv4Addr))
	}
}

func validateIpv6AddrCIDR(ipv6Addr string) {
	match, err := regexp.MatchString("^s*((([0-9A-Fa-f]{1,4}:){7}([0-9A-Fa-f]{1,4}|:))|(([0-9A-Fa-f]{1,4}:){6}(:[0-9A-Fa-f]{1,4}|((25[0-5]|2[0-4]d|1dd|[1-9]?d)(.(25[0-5]|2[0-4]d|1dd|[1-9]?d)){3})|:))|(([0-9A-Fa-f]{1,4}:){5}(((:[0-9A-Fa-f]{1,4}){1,2})|:((25[0-5]|2[0-4]d|1dd|[1-9]?d)(.(25[0-5]|2[0-4]d|1dd|[1-9]?d)){3})|:))|(([0-9A-Fa-f]{1,4}:){4}(((:[0-9A-Fa-f]{1,4}){1,3})|((:[0-9A-Fa-f]{1,4})?:((25[0-5]|2[0-4]d|1dd|[1-9]?d)(.(25[0-5]|2[0-4]d|1dd|[1-9]?d)){3}))|:))|(([0-9A-Fa-f]{1,4}:){3}(((:[0-9A-Fa-f]{1,4}){1,4})|((:[0-9A-Fa-f]{1,4}){0,2}:((25[0-5]|2[0-4]d|1dd|[1-9]?d)(.(25[0-5]|2[0-4]d|1dd|[1-9]?d)){3}))|:))|(([0-9A-Fa-f]{1,4}:){2}(((:[0-9A-Fa-f]{1,4}){1,5})|((:[0-9A-Fa-f]{1,4}){0,3}:((25[0-5]|2[0-4]d|1dd|[1-9]?d)(.(25[0-5]|2[0-4]d|1dd|[1-9]?d)){3}))|:))|(([0-9A-Fa-f]{1,4}:){1}(((:[0-9A-Fa-f]{1,4}){1,6})|((:[0-9A-Fa-f]{1,4}){0,4}:((25[0-5]|2[0-4]d|1dd|[1-9]?d)(.(25[0-5]|2[0-4]d|1dd|[1-9]?d)){3}))|:))|(:(((:[0-9A-Fa-f]{1,4}){1,7})|((:[0-9A-Fa-f]{1,4}){0,5}:((25[0-5]|2[0-4]d|1dd|[1-9]?d)(.(25[0-5]|2[0-4]d|1dd|[1-9]?d)){3}))|:)))(%.+)?s*(\\/(d|dd|1[0-1]d|12[0-8]))$", ipv6Addr)
	if err != nil {
		ExitWithError(ExitIO, errors.New("Failed to parse IPv6 Address: "+err.Error()))
	}
	if !match {
		ExitWithError(ExitIO, errors.New("Invalid Ipv6 Address: "+ipv6Addr))
	}
}

func validateIfIdentifiers(label string, name string) {
	if label == "" {
		ExitWithError(ExitInvalidInput, errors.New("Missing microservice label"))
	}
	if name == "" {
		ExitWithError(ExitInvalidInput, errors.New("Missing interface name"))
	}
}
