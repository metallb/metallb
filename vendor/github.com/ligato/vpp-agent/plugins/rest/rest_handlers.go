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

//go:generate go-bindata-assetfs -pkg rest -o bindata.go ./templates/...

package rest

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	govppapi "git.fd.io/govpp.git/api"
	"github.com/ligato/vpp-agent/plugins/govppmux/vppcalls"
	"github.com/ligato/vpp-agent/plugins/rest/resturl"
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/vpe"
	"github.com/ligato/vpp-agent/plugins/vpp/model/interfaces"
	"github.com/unrolled/render"
)

// Registers access list REST handlers
func (plugin *Plugin) registerAccessListHandlers() {
	// GET IP ACLs
	plugin.registerHTTPHandler(resturl.ACLIP, GET, func() (interface{}, error) {
		return plugin.aclHandler.DumpIPACL(nil)
	})
	// GET MACIP ACLs
	plugin.registerHTTPHandler(resturl.ACLMACIP, GET, func() (interface{}, error) {
		return plugin.aclHandler.DumpMACIPACL(nil)
	})
}

// Registers interface REST handlers
func (plugin *Plugin) registerInterfaceHandlers() {
	// GET all interfaces
	plugin.registerHTTPHandler(resturl.Interface, GET, func() (interface{}, error) {
		return plugin.ifHandler.DumpInterfaces()
	})
	// GET loopback interfaces
	plugin.registerHTTPHandler(resturl.Loopback, GET, func() (interface{}, error) {
		return plugin.ifHandler.DumpInterfacesByType(interfaces.InterfaceType_SOFTWARE_LOOPBACK)
	})
	// GET ethernet interfaces
	plugin.registerHTTPHandler(resturl.Ethernet, GET, func() (interface{}, error) {
		return plugin.ifHandler.DumpInterfacesByType(interfaces.InterfaceType_ETHERNET_CSMACD)
	})
	// GET memif interfaces
	plugin.registerHTTPHandler(resturl.Memif, GET, func() (interface{}, error) {
		return plugin.ifHandler.DumpInterfacesByType(interfaces.InterfaceType_MEMORY_INTERFACE)
	})
	// GET tap interfaces
	plugin.registerHTTPHandler(resturl.Tap, GET, func() (interface{}, error) {
		return plugin.ifHandler.DumpInterfacesByType(interfaces.InterfaceType_TAP_INTERFACE)
	})
	// GET af-packet interfaces
	plugin.registerHTTPHandler(resturl.AfPacket, GET, func() (interface{}, error) {
		return plugin.ifHandler.DumpInterfacesByType(interfaces.InterfaceType_AF_PACKET_INTERFACE)
	})
	// GET VxLAN interfaces
	plugin.registerHTTPHandler(resturl.VxLan, GET, func() (interface{}, error) {
		return plugin.ifHandler.DumpInterfacesByType(interfaces.InterfaceType_VXLAN_TUNNEL)
	})
}

// Registers BFD REST handlers
func (plugin *Plugin) registerBfdHandlers() {
	// GET BFD configuration
	plugin.registerHTTPHandler(resturl.BfdURL, GET, func() (interface{}, error) {
		return plugin.bfdHandler.DumpBfdSingleHop()
	})
	// GET BFD sessions
	plugin.registerHTTPHandler(resturl.BfdSession, GET, func() (interface{}, error) {
		return plugin.bfdHandler.DumpBfdSessions()
	})
	// GET BFD authentication keys
	plugin.registerHTTPHandler(resturl.BfdAuthKey, GET, func() (interface{}, error) {
		return plugin.bfdHandler.DumpBfdAuthKeys()
	})
}

// Registers NAT REST handlers
func (plugin *Plugin) registerNatHandlers() {
	// GET NAT configuration
	plugin.registerHTTPHandler(resturl.NatURL, GET, func() (interface{}, error) {
		return plugin.natHandler.Nat44Dump()
	})
	// GET NAT global config
	plugin.registerHTTPHandler(resturl.NatGlobal, GET, func() (interface{}, error) {
		return plugin.natHandler.Nat44GlobalConfigDump()
	})
	// GET DNAT config
	plugin.registerHTTPHandler(resturl.NatDNat, GET, func() (interface{}, error) {
		return plugin.natHandler.Nat44DNatDump()
	})
}

// Registers STN REST handlers
func (plugin *Plugin) registerStnHandlers() {
	// GET STN configuration
	plugin.registerHTTPHandler(resturl.StnURL, GET, func() (interface{}, error) {
		return plugin.stnHandler.DumpStnRules()
	})
}

// Registers IPSec REST handlers
func (plugin *Plugin) registerIPSecHandlers() {
	// GET IPSec SPD configuration
	plugin.registerHTTPHandler(resturl.IPSecSpd, GET, func() (interface{}, error) {
		return plugin.ipSecHandler.DumpIPSecSPD()
	})
	// GET IPSec SA configuration
	plugin.registerHTTPHandler(resturl.IPSecSa, GET, func() (interface{}, error) {
		return plugin.ipSecHandler.DumpIPSecSA()
	})
	// GET IPSec Tunnel configuration
	plugin.registerHTTPHandler(resturl.IPSecTnIf, GET, func() (interface{}, error) {
		return plugin.ipSecHandler.DumpIPSecTunnelInterfaces()
	})
}

// Registers L2 plugin REST handlers
func (plugin *Plugin) registerL2Handlers() {
	// GET bridge domain IDs
	plugin.registerHTTPHandler(resturl.BdID, GET, func() (interface{}, error) {
		return plugin.bdHandler.DumpBridgeDomainIDs()
	})
	// GET bridge domains
	plugin.registerHTTPHandler(resturl.Bd, GET, func() (interface{}, error) {
		return plugin.bdHandler.DumpBridgeDomains()
	})
	// GET FIB entries
	plugin.registerHTTPHandler(resturl.Fib, GET, func() (interface{}, error) {
		return plugin.fibHandler.DumpFIBTableEntries()
	})
	// GET cross connects
	plugin.registerHTTPHandler(resturl.Xc, GET, func() (interface{}, error) {
		return plugin.xcHandler.DumpXConnectPairs()
	})
}

// Registers L3 plugin REST handlers
func (plugin *Plugin) registerL3Handlers() {
	// GET ARP entries
	plugin.registerHTTPHandler(resturl.Arps, GET, func() (interface{}, error) {
		return plugin.arpHandler.DumpArpEntries()
	})
	// GET proxy ARP interfaces
	plugin.registerHTTPHandler(resturl.PArpIfs, GET, func() (interface{}, error) {
		return plugin.pArpHandler.DumpProxyArpInterfaces()
	})
	// GET proxy ARP ranges
	plugin.registerHTTPHandler(resturl.PArpRngs, GET, func() (interface{}, error) {
		return plugin.pArpHandler.DumpProxyArpRanges()
	})
	// GET static routes
	plugin.registerHTTPHandler(resturl.Routes, GET, func() (interface{}, error) {
		return plugin.rtHandler.DumpStaticRoutes()
	})
}

// Registers L4 plugin REST handlers
func (plugin *Plugin) registerL4Handlers() {
	// GET static routes
	plugin.registerHTTPHandler(resturl.Sessions, GET, func() (interface{}, error) {
		return plugin.l4Handler.DumpL4Config()
	})
}

// Registers linux interface plugin REST handlers
func (plugin *Plugin) registerLinuxInterfaceHandlers() {
	// GET linux interfaces
	plugin.registerHTTPHandler(resturl.LinuxInterface, GET, func() (interface{}, error) {
		return plugin.linuxIfHandler.DumpInterfaces()
	})
	// GET linux interface stats
	plugin.registerHTTPHandler(resturl.LinuxInterfaceStats, GET, func() (interface{}, error) {
		return plugin.linuxIfHandler.DumpInterfaceStatistics()
	})
}

// Registers linux L3 plugin REST handlers
func (plugin *Plugin) registerLinuxL3Handlers() {
	// GET linux routes
	plugin.registerHTTPHandler(resturl.LinuxRoutes, GET, func() (interface{}, error) {
		return plugin.linuxL3Handler.DumpRoutes()
	})
	// GET linux ARPs
	plugin.registerHTTPHandler(resturl.LinuxArps, GET, func() (interface{}, error) {
		return plugin.linuxL3Handler.DumpArpEntries()
	})
}

// Registers Telemetry handler
func (plugin *Plugin) registerTelemetryHandlers() {
	plugin.HTTPHandlers.RegisterHTTPHandler(resturl.Telemetry, plugin.telemetryHandler, GET)
	plugin.HTTPHandlers.RegisterHTTPHandler(resturl.TMemory, plugin.telemetryMemoryHandler, GET)
	plugin.HTTPHandlers.RegisterHTTPHandler(resturl.TRuntime, plugin.telemetryRuntimeHandler, GET)
	plugin.HTTPHandlers.RegisterHTTPHandler(resturl.TNodeCount, plugin.telemetryNodeCountHandler, GET)
}

// Registers Tracer handler
func (plugin *Plugin) registerTracerHandler() {
	plugin.HTTPHandlers.RegisterHTTPHandler(resturl.Tracer, plugin.tracerHandler, GET)
}

// Registers command handler
func (plugin *Plugin) registerCommandHandler() {
	plugin.HTTPHandlers.RegisterHTTPHandler(resturl.Command, plugin.commandHandler, POST)
}

// Registers index page
func (plugin *Plugin) registerIndexHandlers() {
	r := render.New(render.Options{
		Directory:  "templates",
		Asset:      Asset,
		AssetNames: AssetNames,
	})

	handlerFunc := func(formatter *render.Render) http.HandlerFunc {
		return func(w http.ResponseWriter, req *http.Request) {

			plugin.Log.Debugf("%v - %s %q", req.RemoteAddr, req.Method, req.URL)
			r.HTML(w, http.StatusOK, "index", plugin.index)
		}
	}
	plugin.HTTPHandlers.RegisterHTTPHandler(resturl.Index, handlerFunc, GET)
}

// registerHTTPHandler is common register method for all handlers
func (plugin *Plugin) registerHTTPHandler(key, method string, f func() (interface{}, error)) {
	handlerFunc := func(formatter *render.Render) http.HandlerFunc {
		return func(w http.ResponseWriter, req *http.Request) {
			plugin.govppmux.Lock()
			defer plugin.govppmux.Unlock()

			res, err := f()
			if err != nil {
				plugin.Deps.Log.Errorf("Error: %v", err)
				errStr := fmt.Sprintf("500 Internal server error: %s\n", err.Error())
				formatter.Text(w, http.StatusInternalServerError, errStr)
				return
			}
			plugin.Deps.Log.Debugf("Rest uri: %s, data: %v", key, res)
			formatter.JSON(w, http.StatusOK, res)
		}
	}
	plugin.HTTPHandlers.RegisterHTTPHandler(key, handlerFunc, method)
}

// commandHandler - used to execute VPP CLI commands
func (plugin *Plugin) commandHandler(formatter *render.Render) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {

		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			plugin.Log.Error("Failed to parse request body.")
			formatter.JSON(w, http.StatusBadRequest, err)
			return
		}

		var reqParam map[string]string
		err = json.Unmarshal(body, &reqParam)
		if err != nil {
			plugin.Log.Error("Failed to unmarshal request body.")
			formatter.JSON(w, http.StatusBadRequest, err)
			return
		}

		command, ok := reqParam["vppclicommand"]
		if !ok || command == "" {
			plugin.Log.Error("vppclicommand parameter missing or empty")
			formatter.JSON(w, http.StatusBadRequest, "vppclicommand parameter missing or empty")
			return
		}

		plugin.Log.Debugf("VPPCLI command: %v", command)

		ch, err := plugin.GoVppmux.NewAPIChannel()
		if err != nil {
			plugin.Log.Errorf("Error creating channel: %v", err)
			formatter.JSON(w, http.StatusInternalServerError, err)
			return
		}
		defer ch.Close()

		r := &vpe.CliInband{
			Length: uint32(len(command)),
			Cmd:    []byte(command),
		}
		reply := &vpe.CliInbandReply{}
		err = ch.SendRequest(r).ReceiveReply(reply)
		if err != nil {
			err = fmt.Errorf("sending request failed: %v", err)
			plugin.Log.Error(err)
			formatter.JSON(w, http.StatusInternalServerError, err)
			return
		} else if reply.Retval > 0 {
			err = fmt.Errorf("request returned error code: %v", reply.Retval)
			plugin.Log.Error(err)
			formatter.JSON(w, http.StatusInternalServerError, err)
			return
		}

		plugin.Log.Debugf("VPPCLI response: %s", reply.Reply)
		formatter.Text(w, http.StatusOK, string(reply.Reply))
	}
}

func (plugin *Plugin) sendCommand(ch govppapi.Channel, command string) ([]byte, error) {
	r := &vpe.CliInband{
		Length: uint32(len(command)),
		Cmd:    []byte(command),
	}

	reply := &vpe.CliInbandReply{}
	if err := ch.SendRequest(r).ReceiveReply(reply); err != nil {
		return nil, fmt.Errorf("sending request failed: %v", err)
	} else if reply.Retval > 0 {
		return nil, fmt.Errorf("request returned error code: %v", reply.Retval)
	}

	return reply.Reply[:reply.Length], nil
}

// telemetryHandler - returns various telemetry data
func (plugin *Plugin) telemetryHandler(formatter *render.Render) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {

		ch, err := plugin.GoVppmux.NewAPIChannel()
		if err != nil {
			plugin.Log.Errorf("Error creating channel: %v", err)
			formatter.JSON(w, http.StatusInternalServerError, err)
			return
		}
		defer ch.Close()

		type cmdOut struct {
			Command string
			Output  interface{}
		}
		var cmdOuts []cmdOut

		var runCmd = func(command string) {
			out, err := plugin.sendCommand(ch, command)
			if err != nil {
				plugin.Log.Errorf("Sending command failed: %v", err)
				formatter.JSON(w, http.StatusInternalServerError, err)
				return
			}
			cmdOuts = append(cmdOuts, cmdOut{
				Command: command,
				Output:  string(out),
			})
		}

		runCmd("show node counters")
		runCmd("show runtime")
		runCmd("show buffers")
		runCmd("show memory")
		runCmd("show ip fib")
		runCmd("show ip6 fib")

		formatter.JSON(w, http.StatusOK, cmdOuts)
	}
}

// telemetryMemoryHandler - returns various telemetry data
func (plugin *Plugin) telemetryMemoryHandler(formatter *render.Render) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {

		ch, err := plugin.GoVppmux.NewAPIChannel()
		if err != nil {
			plugin.Log.Errorf("Error creating channel: %v", err)
			formatter.JSON(w, http.StatusInternalServerError, err)
			return
		}
		defer ch.Close()

		info, err := vppcalls.GetMemory(ch)
		if err != nil {
			plugin.Log.Errorf("Sending command failed: %v", err)
			formatter.JSON(w, http.StatusInternalServerError, err)
			return
		}

		formatter.JSON(w, http.StatusOK, info)
	}
}

// telemetryHandler - returns various telemetry data
func (plugin *Plugin) telemetryRuntimeHandler(formatter *render.Render) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {

		ch, err := plugin.GoVppmux.NewAPIChannel()
		if err != nil {
			plugin.Log.Errorf("Error creating channel: %v", err)
			formatter.JSON(w, http.StatusInternalServerError, err)
			return
		}
		defer ch.Close()

		runtimeInfo, err := vppcalls.GetRuntimeInfo(ch)
		if err != nil {
			plugin.Log.Errorf("Sending command failed: %v", err)
			formatter.JSON(w, http.StatusInternalServerError, err)
			return
		}

		formatter.JSON(w, http.StatusOK, runtimeInfo)
	}
}

// telemetryHandler - returns various telemetry data
func (plugin *Plugin) telemetryNodeCountHandler(formatter *render.Render) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {

		ch, err := plugin.GoVppmux.NewAPIChannel()
		if err != nil {
			plugin.Log.Errorf("Error creating channel: %v", err)
			formatter.JSON(w, http.StatusInternalServerError, err)
			return
		}
		defer ch.Close()

		nodeCounters, err := vppcalls.GetNodeCounters(ch)
		if err != nil {
			plugin.Log.Errorf("Sending command failed: %v", err)
			formatter.JSON(w, http.StatusInternalServerError, err)
			return
		}

		formatter.JSON(w, http.StatusOK, nodeCounters)
	}
}

// tracerHandler - returns binary API call trace
func (plugin *Plugin) tracerHandler(formatter *render.Render) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		ch, err := plugin.GoVppmux.NewAPIChannel()
		if err != nil {
			plugin.Log.Errorf("Error creating channel: %v", err)
			formatter.JSON(w, http.StatusInternalServerError, err)
			return
		}
		defer ch.Close()

		entries := plugin.GoVppmux.GetTrace()
		if err != nil {
			plugin.Log.Errorf("Sending command failed: %v", err)
			formatter.JSON(w, http.StatusInternalServerError, err)
			return
		}
		if entries == nil {
			formatter.JSON(w, http.StatusOK, "VPP api trace is disabled")
			return
		}

		formatter.JSON(w, http.StatusOK, entries)
	}
}
