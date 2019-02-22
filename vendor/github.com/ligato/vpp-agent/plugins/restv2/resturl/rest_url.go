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

package resturl

// Linux Dumps
const (
	// Interfaces

	// LinuxInterface is a linux interface rest path
	LinuxInterface = "/dump/linux/v2/interfaces"

	// L3

	// LinuxRoutes is the rest linux route path
	LinuxRoutes = "/dump/linux/v2/routes"
	// LinuxArps is the rest linux ARPs path
	LinuxArps = "/dump/linux/v2/arps"
)

// VPP ACL
const (
	// REST ACL IP prefix
	ACLIP = "/dump/vpp/v2/acl/ip"
	// REST ACL MACIP prefix
	ACLMACIP = "/dump/vpp/v2/acl/macip"
)

// VPP Interfaces
const (
	// Interface is rest interface path
	Interface = "/dump/vpp/v2/interfaces"

	// Loopback is path for loopback interface
	Loopback = "/dump/vpp/v2/interfaces/loopback"
	// Ethernet is path for physical interface
	Ethernet = "/dump/vpp/v2/interfaces/ethernet"
	// Memif is path for memif interface
	Memif = "/dump/vpp/v2/interfaces/memif"
	// Tap is path for tap interface
	Tap = "/dump/vpp/v2/interfaces/tap"
	// AfPacket is path for af-packet interface
	AfPacket = "/dump/vpp/v2/interfaces/afpacket"
	// VxLan is path for vxlan interface
	VxLan = "/dump/vpp/v2/interfaces/vxlan"
)

// VPP NAT
const (
	// NatURL is a REST path of a NAT
	NatURL = "/dump/vpp/v2/nat"
	// NatGlobal is a REST path of a global NAT config
	NatGlobal = "/dump/vpp/v2/nat/global"
	// NatDNat is a REST path of a DNAT configurations
	NatDNat = "/dump/vpp/v2/nat/dnat"
)

// L2 plugin
const (
	// restBd is rest bridge domain path
	Bd = "/dump/vpp/v2/bd"
	// restBdId is rest bridge domain ID path
	BdID = "/dump/vpp/v2/bdid"
	// restFib is rest FIB path
	Fib = "/dump/vpp/v2/fib"
	// restXc is rest cross-connect path
	Xc = "/dump/vpp/v2/xc"
)

// VPP L3 plugin
const (
	// Routes is rest static route path
	Routes = "/dump/vpp/v2/routes"
	// Arps is rest ARPs path
	Arps = "/dump/vpp/v2/arps"
	// PArpIfs is rest proxy ARP interfaces path
	PArpIfs = "/dump/vpp/v2/proxyarp/interfaces"
	// PArpRngs is rest proxy ARP ranges path
	PArpRngs = "/dump/vpp/v2/proxyarp/ranges"
)

// Command
const (
	// Command allows to put CLI command to the rest
	Command = "/vpp/command"
)

// Telemetry
const (
	// Telemetry reads various types of metrics data from the VPP
	Telemetry  = "/vpp/telemetry"
	TMemory    = "/vpp/telemetry/memory"
	TRuntime   = "/vpp/telemetry/runtime"
	TNodeCount = "/vpp/telemetry/nodecount"
)

// Tracer
const (
	// Traced binary API calls
	Tracer = "/vpp/binapitrace"
)

// Index
const (
	// Index can be used to get the full index page
	Index = "/"
)
