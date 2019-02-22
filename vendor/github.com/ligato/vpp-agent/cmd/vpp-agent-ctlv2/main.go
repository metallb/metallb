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

// package vpp-agent-ctl implements the vpp-agent-ctl test tool for testing
// VPP Agent plugins. In addition to testing, the vpp-agent-ctl tool can
// be used to demonstrate the usage of VPP Agent plugins and their APIs.

package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/ligato/vpp-agent/cmd/vpp-agent-ctlv2/data"
	"github.com/namsral/flag"

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
	// Parse service label
	if err := flag.CommandLine.ParseEnv(os.Environ()); err != nil {
		fmt.Printf("failed to parse environment variables")
		return
	}
	ctl, err := data.NewVppAgentCtl(etcdCfg, cmdSet)
	if err != nil {
		// Error is already printed in 'bytes_broker_impl.go'
		usage()
		return
	}

	do(ctl)
}

func do(ctl data.VppAgentCtl) {
	commands := ctl.GetCommands()
	switch commands[0] {
	case "-list":
		// List all keys
		ctl.ListAllAgentKeys()
	case "-dump":
		if len(commands) >= 2 {
			// Dump specific key
			ctl.Dump(commands[1])
		} else {
			// Dump all keys
			ctl.Dump("")
		}
	case "-get":
		if len(commands) >= 2 {
			// Get key
			ctl.Get(commands[1])
		}
	case "-del":
		if len(commands) >= 2 {
			// Del key
			ctl.Del(commands[1])
		}
	case "-put":
		if len(commands) >= 3 {
			ctl.Put(commands[1], commands[2])
		}
	default:
		var err error
		switch commands[0] {
		// ACL plugin
		case "-aclip":
			err = ctl.PutIPAcl()
		case "-aclipd":
			err = ctl.DeleteIPAcl()
		case "-aclmac":
			err = ctl.PutMACIPAcl()
		case "-aclmacd":
			err = ctl.DeleteMACIPAcl()
		// VPP interface plugin
		case "-eth":
			err = ctl.PutDPDKInterface()
		case "-ethd":
			err = ctl.DeleteDPDKInterface()
		case "-tap":
			err = ctl.PutTap()
		case "-tapd":
			err = ctl.DeleteTap()
		case "-loop":
			err = ctl.PutLoopback()
		case "-loopd":
			err = ctl.DeleteLoopback()
		case "-memif":
			err = ctl.PutMemoryInterface()
		case "-memifd":
			err = ctl.DeleteMemoryInterface()
		case "-vxlan":
			err = ctl.PutVxLan()
		case "-vxland":
			err = ctl.DeleteVxLan()
		case "-afpkt":
			err = ctl.PutAfPacket()
		case "-afpktd":
			err = ctl.DeleteAfPacket()
		case "-ipsectun":
			err = ctl.PutIPSecTunnelInterface()
		case "-ipsectund":
			err = ctl.DeleteIPSecTunnelInterface()
		// Linux interface plugin
		case "-veth":
			err = ctl.PutVEthPair()
		case "-vethd":
			err = ctl.DeleteVEthPair()
		case "-ltap":
			err = ctl.PutLinuxTap()
		case "-ltapd":
			err = ctl.DeleteLinuxTap()
		// IPSec plugin
		case "-spd":
			err = ctl.PutIPSecSPD()
		case "-spdd":
			err = ctl.DeleteIPSecSPD()
		case "-sa":
			err = ctl.PutIPSecSA()
		case "-sad":
			err = ctl.DeleteIPSecSA()
		// L2 plugin
		case "-bd":
			err = ctl.PutBridgeDomain()
		case "-bdd":
			err = ctl.DeleteBridgeDomain()
		case "-fib":
			err = ctl.PutFib()
		case "-fibd":
			err = ctl.DeleteFib()
		case "-xconn":
			err = ctl.PutXConn()
		case "-xconnd":
			err = ctl.DeleteXConn()
		// VPP L3 plugin
		case "-route":
			err = ctl.PutRoute()
		case "-routed":
			err = ctl.DeleteRoute()
		case "-routeint":
			err = ctl.PutInterVrfRoute()
		case "-routeintd":
			err = ctl.DeleteInterVrfRoute()
		case "-routenh":
			err = ctl.PutNextHopRoute()
		case "-routenhd":
			err = ctl.DeleteNextHopRoute()
		case "-arp":
			err = ctl.PutArp()
		case "-arpd":
			err = ctl.DeleteArp()
		case "-proxyarp":
			err = ctl.PutProxyArp()
		case "-proxyarpd":
			err = ctl.DeleteProxyArp()
		case "-ipscan":
			err = ctl.SetIPScanNeigh()
		case "-ipscand":
			err = ctl.UnsetIPScanNeigh()
		// Linux L3 plugin
		case "-lroute":
			err = ctl.PutLinuxRoute()
		case "-lrouted":
			err = ctl.DeleteLinuxRoute()
		case "-larp":
			err = ctl.PutLinuxArp()
		case "-larpd":
			err = ctl.DeleteLinuxArp()
		// NAT plugin
		case "-gnat":
			err = ctl.PutGlobalNat()
		case "-gnatd":
			err = ctl.DeleteGlobalNat()
		case "-dnat":
			err = ctl.PutDNat()
		case "-dnatd":
			err = ctl.DeleteDNat()
		// Punt plugin
		case "-punt":
			err = ctl.PutPunt()
		case "-puntd":
			err = ctl.DeletePunt()
		case "-rsocket":
			err = ctl.RegisterPuntViaSocket()
		case "-dsocket":
			err = ctl.DeregisterPuntViaSocket()
		case "-ipredir":
			err = ctl.PutIPRedirect()
		case "-ipredird":
			err = ctl.DeleteIPRedirect()
		// STN plugin
		case "-stn":
			err = ctl.PutStn()
		case "-stnd":
			err = ctl.DeleteStn()
		default:
			usage()
		}
		if err != nil {
			fmt.Printf("error calling '%s': %v", commands[0], err)
		}
	}
}

// Show command info TODO punt
func usage() {
	var buffer bytes.Buffer
	// Crud operation
	_, err := buffer.WriteString(` 

	Example tool for VPP configuration.

	Crud operations with .json (files can be found in /json):
		-put	<etc_key>    <json-file>
		-get	<etc_key>
		-del	<etc_key>
		-dump
		-list

	Prearranged flags (create, delete) sorted by plugin:

	Access list plugin:
		-aclip,		-aclipd		- Access List with IP rules
		-aclmac 	-aclmacd	- Access List with MAC IP rule

	Interface plugin:
		-eth,		-ethd		- Physical interface
		-tap,		-tapd		- TAP type interface
		-loop,		-loopd		- Loop type interface
		-memif,		-memifd		- Memif type interface
		-vxlan,		-vxland		- VxLAN type interface
		-afpkt,		-afpktd		- af_packet type interface
		-ipsectun, 	-ipsectund 	- IPSec tunnel interface

	Linux interface plugin:
		-veth,		-vethd		- Linux VETH interface pair
		-ltap,		-ltapd		- Linux TAP interface

	IPSec plugin:
		-spd,   	-spdd   	- IPSec security policy database
		-sa,    	-sad    	- IPSec security associations

	L2 plugin:
		-bd,		-bdd		- Bridge doamin
		-fib,		-fibd		- L2 FIB
		-xconn,		-xconnd		- L2 X-Connect

	L3 plugin:
		-route,		-routed		- L3 route
		-routeint,	-routeintd	- L3 inter-vrf route
		-routenh,	-routenhd	- L3 next-hop route
		-arp,		-arpd		- ARP entry
		-proxyarp,	-proxyarpd	- Proxy ARP configuration
		-ipscan  	-ipscand 	- VPP IP scan neighbor

	Linux L3 plugin:
		-lroute,	-lrouted	- Linux route
		-larp,		-larpd		- Linux ARP entry

	NAT plugin:
		-gnat,		-gnatd		- Global NAT configuration
		-dnat,		-dnatd		- DNAT configuration
		
	Punt plugin:
		-punt,		-puntd		- Punt to host
		-rsocket,	-dsocket	- Punt to host via socket registration
		-ipredir,	-ipredird	- IP redirect
		
	STN plugin:
		-stn,		-stnd		- STN rule
	`)

	if err != nil {
		logrus.DefaultLogger().Error(err)
	} else {
		logrus.DefaultLogger().Print(buffer.String())
	}
}
