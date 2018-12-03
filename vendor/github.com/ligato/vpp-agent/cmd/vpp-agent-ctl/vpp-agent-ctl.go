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

// package vpp-agent-ctl implements the vpp-agent-ctl test tool for testing
// VPP Agent plugins. In addition to testing, the vpp-agent-ctl tool can
// be used to demonstrate the usage of VPP Agent plugins and their APIs.
package main

import (
	"bytes"
	"os"
	"strings"

	"github.com/ligato/cn-infra/logging/logrus"
)

func main() {
	// Read args
	args := os.Args
	argsLen := len(args)

	// First argument is not a command
	if argsLen == 1 {
		usage()
		return
	}
	// Check if second argument is a command or path to the ETCD config file
	var etcdCfg string
	var cmdSet []string
	if argsLen >= 2 && !strings.HasPrefix(args[1], "-") {
		etcdCfg = args[1]
		// Remove first two arguments
		cmdSet = args[2:]
	} else {
		// Remove first argument
		cmdSet = args[1:]
	}
	ctl, err := initCtl(etcdCfg, cmdSet)
	if err != nil {
		// Error is already printed in 'bytes_broker_impl.go'
		usage()
		return
	}

	do(ctl)
}

func do(ctl *VppAgentCtl) {
	switch ctl.Commands[0] {
	case "-list":
		// List all keys
		ctl.listAllAgentKeys()
	case "-dump":
		if len(ctl.Commands) >= 2 {
			// Dump specific key
			ctl.etcdDump(ctl.Commands[1])
		} else {
			// Dump all keys
			ctl.etcdDump("")
		}
	case "-get":
		if len(ctl.Commands) >= 2 {
			// Get key
			ctl.etcdGet(ctl.Commands[1])
		}
	case "-del":
		if len(ctl.Commands) >= 2 {
			// Del key
			ctl.etcdDel(ctl.Commands[1])
		}
	case "-put":
		if len(ctl.Commands) >= 3 {
			ctl.etcdPut(ctl.Commands[1], ctl.Commands[2])
		}
	default:
		switch ctl.Commands[0] {
		// ACL
		case "-acl":
			ctl.createACL()
		case "-acld":
			ctl.deleteACL()
			// BFD
		case "-bfds":
			ctl.createBfdSession()
		case "-bfdsd":
			ctl.deleteBfdSession()
		case "-bfdk":
			ctl.createBfdKey()
		case "-bfdkd":
			ctl.deleteBfdKey()
		case "-bfde":
			ctl.createBfdEcho()
		case "-bfded":
			ctl.deleteBfdEcho()
			// VPP interfaces
		case "-eth":
			ctl.createEthernet()
		case "-ethd":
			ctl.deleteEthernet()
		case "-tap":
			ctl.createTap()
		case "-tapd":
			ctl.deleteTap()
		case "-loop":
			ctl.createLoopback()
		case "-loopd":
			ctl.deleteLoopback()
		case "-memif":
			ctl.createMemif()
		case "-memifd":
			ctl.deleteMemif()
		case "-vxlan":
			ctl.createVxlan()
		case "-vxland":
			ctl.deleteVxlan()
		case "-afpkt":
			ctl.createAfPacket()
		case "-afpktd":
			ctl.deleteAfPacket()
			// Linux interfaces
		case "-veth":
			ctl.createVethPair()
		case "-vethd":
			ctl.deleteVethPair()
		case "-ltap":
			ctl.createLinuxTap()
		case "-ltapd":
			ctl.deleteLinuxTap()
			// IPsec
		case "-spd":
			ctl.createIPsecSPD()
		case "-spdd":
			ctl.deleteIPsecSPD()
		case "-sa":
			ctl.createIPsecSA()
		case "-sad":
			ctl.deleteIPsecSA()
		case "-tun":
			ctl.createIPSecTunnelInterface()
		case "-tund":
			ctl.deleteIPSecTunnelInterface()
			// STN
		case "-stn":
			ctl.createStn()
		case "-stnd":
			ctl.deleteStn()
			// NAT
		case "-gnat":
			ctl.createGlobalNat()
		case "-gnatd":
			ctl.deleteGlobalNat()
		case "-snat":
			ctl.createSNat()
		case "-snatd":
			ctl.deleteSNat()
		case "-dnat":
			ctl.createDNat()
		case "-dnatd":
			ctl.deleteDNat()
			// Bridge domains
		case "-bd":
			ctl.createBridgeDomain()
		case "-bdd":
			ctl.deleteBridgeDomain()
			// FIB
		case "-fib":
			ctl.createFib()
		case "-fibd":
			ctl.deleteFib()
			// L2 xConnect
		case "-xconn":
			ctl.createXConn()
		case "-xconnd":
			ctl.deleteXConn()
			// VPP routes
		case "-route":
			ctl.createRoute()
		case "-routed":
			ctl.deleteRoute()
			// Linux routes
		case "-lrte":
			ctl.createLinuxRoute()
		case "-lrted":
			ctl.deleteLinuxRoute()
			// VPP ARP
		case "-arp":
			ctl.createArp()
		case "-arpd":
			ctl.deleteArp()
		case "-prxi":
			ctl.addProxyArpInterfaces()
		case "-prxid":
			ctl.deleteProxyArpInterfaces()
		case "-prxr":
			ctl.addProxyArpRanges()
		case "-prxrd":
			ctl.deleteProxyArpRanges()
		case "-ipscn":
			ctl.setIPScanNeigh()
		case "-ipscnd":
			ctl.unsetIPScanNeigh()
			// Linux ARP
		case "-larp":
			ctl.createLinuxArp()
		case "-larpd":
			ctl.deleteLinuxArp()
			// L4 plugin
		case "-el4":
			ctl.enableL4Features()
		case "-dl4":
			ctl.disableL4Features()
		case "-appns":
			ctl.createAppNamespace()
		case "-appnsd":
			ctl.deleteAppNamespace()
			// TXN (transaction)
		case "-txn":
			ctl.createTxn()
		case "-txnd":
			ctl.deleteTxn()
			// Error reporting
		case "-errIf":
			ctl.reportIfaceErrorState()
		case "-errBd":
			ctl.reportBdErrorState()
		default:
			usage()
		}
	}
}

// Show command info
func usage() {
	var buffer bytes.Buffer
	// Crud operation
	buffer.WriteString(` 

	Crud operations with .json:
		-put	<etc_key>    <json-file>
		-get	<etc_key>
		-del	<etc_key>
		-dump
		-list

	Prearranged flags (create, delete):
		-acl,	-acld	- Access List
		-bfds,	-bfdsd	- BFD session
		-bfdk,	-bfdkd	- BFD authentication key
		-bfde,	-bfded	- BFD echo function
		-eth,	-ethd	- Physical interface
		-tap,	-tapd	- TAP type interface
		-loop,	-loopd	- Loop type interface
		-memif,	-memifd	- Memif type interface
		-vxlan,	-vxland	- VxLAN type interface
		-afpkt,	-afpktd	- af_packet type interface
		-veth,	-vethd	- Linux VETH interface pair
		-ltap,	-ltapd	- Linux TAP interface
		-spd,   -spdd   - IPSec security policy database
		-sa,    -sad    - IPSec security associations
		-tun    -tund   - IPSec tunnel interface
		-stn,	-stnd	- STN rule
		-gnat,	-gnatd	- Global NAT configuration
		-snat,	-snatd	- SNAT configuration
		-dnat,	-dnatd	- DNAT configuration
		-bd,	-bdd	- Bridge doamin
		-fib,	-fibd	- L2 FIB
		-xconn,	-xconnd	- L2 X-Connect
		-route,	-routed	- L3 route
		-arp,	-arpd	- ARP entry
		-prxi,	-prxid	- Proxy ARP interfaces
		-prxr,	-prxrd	- Proxy ARP ranges
		-lrte,	-lrted	- Linux route
		-larp,	-larpd	- Linux ARP entry
		-ipscn  -ipscnd - VPP IP scan neighbor
		-el4,	-dl4	- L4 features
		-appns,	-appnsd	- Application namespace

	Other:
		-txn,	-txnd	- Transaction
		-errIf		- Interface error state report
		-errBd		- Bridge domain error state report
	`)

	logrus.DefaultLogger().Print(buffer.String())
}
