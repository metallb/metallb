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

//go:generate go-bindata-assetfs -pkg rest -o bindata.go ./templates/...

package rest

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	govppapi "git.fd.io/govpp.git/api"
	interfaces "github.com/ligato/vpp-agent/api/models/vpp/interfaces"
	"github.com/ligato/vpp-agent/plugins/govppmux/vppcalls"
	"github.com/ligato/vpp-agent/plugins/restv2/resturl"
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/vpe"
	"github.com/unrolled/render"
)

// Registers access list REST handlers
func (p *Plugin) registerAccessListHandlers() {
	// GET IP ACLs
	p.registerHTTPHandler(resturl.ACLIP, GET, func() (interface{}, error) {
		return p.aclHandler.DumpACL()
	})
	// GET MACIP ACLs
	p.registerHTTPHandler(resturl.ACLMACIP, GET, func() (interface{}, error) {
		return p.aclHandler.DumpMACIPACL()
	})
}

// Registers interface REST handlers
func (p *Plugin) registerInterfaceHandlers() {
	// GET all interfaces
	p.registerHTTPHandler(resturl.Interface, GET, func() (interface{}, error) {
		return p.ifHandler.DumpInterfaces()
	})
	// GET loopback interfaces
	p.registerHTTPHandler(resturl.Loopback, GET, func() (interface{}, error) {
		return p.ifHandler.DumpInterfacesByType(interfaces.Interface_SOFTWARE_LOOPBACK)
	})
	// GET ethernet interfaces
	p.registerHTTPHandler(resturl.Ethernet, GET, func() (interface{}, error) {
		return p.ifHandler.DumpInterfacesByType(interfaces.Interface_DPDK)
	})
	// GET memif interfaces
	p.registerHTTPHandler(resturl.Memif, GET, func() (interface{}, error) {
		return p.ifHandler.DumpInterfacesByType(interfaces.Interface_MEMIF)
	})
	// GET tap interfaces
	p.registerHTTPHandler(resturl.Tap, GET, func() (interface{}, error) {
		return p.ifHandler.DumpInterfacesByType(interfaces.Interface_TAP)
	})
	// GET af-packet interfaces
	p.registerHTTPHandler(resturl.AfPacket, GET, func() (interface{}, error) {
		return p.ifHandler.DumpInterfacesByType(interfaces.Interface_AF_PACKET)
	})
	// GET VxLAN interfaces
	p.registerHTTPHandler(resturl.VxLan, GET, func() (interface{}, error) {
		return p.ifHandler.DumpInterfacesByType(interfaces.Interface_VXLAN_TUNNEL)
	})
}

// Registers NAT REST handlers
func (p *Plugin) registerNatHandlers() {
	// GET NAT configuration
	/*p.registerHTTPHandler(resturl.NatURL, GET, func() (interface{}, error) {
		return p.natHandler.Nat44Dump()
	})*/
	// GET NAT global config
	p.registerHTTPHandler(resturl.NatGlobal, GET, func() (interface{}, error) {
		return p.natHandler.Nat44GlobalConfigDump()
	})
	// GET DNAT config
	p.registerHTTPHandler(resturl.NatDNat, GET, func() (interface{}, error) {
		return p.natHandler.DNat44Dump()
	})
}

// Registers L2 plugin REST handlers
func (p *Plugin) registerL2Handlers() {
	// GET bridge domain IDs
	/*p.registerHTTPHandler(resturl.BdID, GET, func() (interface{}, error) {
		return p.bdHandler.DumpBridgeDomainIDs()
	})*/
	// GET bridge domains
	p.registerHTTPHandler(resturl.Bd, GET, func() (interface{}, error) {
		return p.bdHandler.DumpBridgeDomains()
	})
	// GET FIB entries
	p.registerHTTPHandler(resturl.Fib, GET, func() (interface{}, error) {
		return p.fibHandler.DumpL2FIBs()
	})
	// GET cross connects
	p.registerHTTPHandler(resturl.Xc, GET, func() (interface{}, error) {
		return p.xcHandler.DumpXConnectPairs()
	})
}

// Registers L3 plugin REST handlers
func (p *Plugin) registerL3Handlers() {
	// GET ARP entries
	p.registerHTTPHandler(resturl.Arps, GET, func() (interface{}, error) {
		return p.arpHandler.DumpArpEntries()
	})
	// GET proxy ARP interfaces
	p.registerHTTPHandler(resturl.PArpIfs, GET, func() (interface{}, error) {
		return p.pArpHandler.DumpProxyArpInterfaces()
	})
	// GET proxy ARP ranges
	p.registerHTTPHandler(resturl.PArpRngs, GET, func() (interface{}, error) {
		return p.pArpHandler.DumpProxyArpRanges()
	})
	// GET static routes
	p.registerHTTPHandler(resturl.Routes, GET, func() (interface{}, error) {
		return p.rtHandler.DumpRoutes()
	})
}

// Registers linux interface plugin REST handlers
func (p *Plugin) registerLinuxInterfaceHandlers() {
	// GET linux interfaces
	p.registerHTTPHandler(resturl.LinuxInterface, GET, func() (interface{}, error) {
		return p.linuxIfHandler.GetLinkList()
	})
	// GET linux interface stats
	/*p.registerHTTPHandler(resturl.LinuxInterfaceStats, GET, func() (interface{}, error) {
		return p.linuxIfHandler.DumpInterfaceStatistics()
	})*/
}

// Registers linux L3 plugin REST handlers
func (p *Plugin) registerLinuxL3Handlers() {
	// GET linux routes
	p.registerHTTPHandler(resturl.LinuxRoutes, GET, func() (interface{}, error) {
		routes4, routes6, err := p.linuxL3Handler.GetRoutes(0)
		if err != nil {
			return nil, err
		}
		return append(routes4, routes6...), nil
	})
	// GET linux ARPs
	p.registerHTTPHandler(resturl.LinuxArps, GET, func() (interface{}, error) {
		return p.linuxL3Handler.GetARPEntries(0)
	})
}

// Registers Telemetry handler
func (p *Plugin) registerTelemetryHandlers() {
	p.HTTPHandlers.RegisterHTTPHandler(resturl.Telemetry, p.telemetryHandler, GET)
	p.HTTPHandlers.RegisterHTTPHandler(resturl.TMemory, p.telemetryMemoryHandler, GET)
	p.HTTPHandlers.RegisterHTTPHandler(resturl.TRuntime, p.telemetryRuntimeHandler, GET)
	p.HTTPHandlers.RegisterHTTPHandler(resturl.TNodeCount, p.telemetryNodeCountHandler, GET)
}

// Registers Tracer handler
func (p *Plugin) registerTracerHandler() {
	p.HTTPHandlers.RegisterHTTPHandler(resturl.Tracer, p.tracerHandler, GET)
}

// Registers command handler
func (p *Plugin) registerCommandHandler() {
	p.HTTPHandlers.RegisterHTTPHandler(resturl.Command, p.commandHandler, POST)
}

// Registers index page
func (p *Plugin) registerIndexHandlers() {
	r := render.New(render.Options{
		Directory:  "templates",
		Asset:      Asset,
		AssetNames: AssetNames,
	})

	handlerFunc := func(formatter *render.Render) http.HandlerFunc {
		return func(w http.ResponseWriter, req *http.Request) {

			p.Log.Debugf("%v - %s %q", req.RemoteAddr, req.Method, req.URL)
			p.logError(r.HTML(w, http.StatusOK, "index", p.index))
		}
	}
	p.HTTPHandlers.RegisterHTTPHandler(resturl.Index, handlerFunc, GET)
}

// registerHTTPHandler is common register method for all handlers
func (p *Plugin) registerHTTPHandler(key, method string, f func() (interface{}, error)) {
	handlerFunc := func(formatter *render.Render) http.HandlerFunc {
		return func(w http.ResponseWriter, req *http.Request) {
			p.govppmux.Lock()
			defer p.govppmux.Unlock()

			res, err := f()
			if err != nil {
				errMsg := fmt.Sprintf("500 Internal server error: request failed: %v\n", err)
				p.Log.Error(errMsg)
				p.logError(formatter.JSON(w, http.StatusInternalServerError, errMsg))
				return
			}
			p.Deps.Log.Debugf("Rest uri: %s, data: %v", key, res)
			p.logError(formatter.JSON(w, http.StatusOK, res))
		}
	}
	p.HTTPHandlers.RegisterHTTPHandler(key, handlerFunc, method)
}

// commandHandler - used to execute VPP CLI commands
func (p *Plugin) commandHandler(formatter *render.Render) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {

		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			errMsg := fmt.Sprintf("400 Bad request: failed to parse request body: %v\n", err)
			p.Log.Error(errMsg)
			p.logError(formatter.JSON(w, http.StatusBadRequest, errMsg))
			return
		}

		var reqParam map[string]string
		err = json.Unmarshal(body, &reqParam)
		if err != nil {
			errMsg := fmt.Sprintf("400 Bad request: failed to unmarshall request body: %v\n", err)
			p.Log.Error(errMsg)
			p.logError(formatter.JSON(w, http.StatusBadRequest, errMsg))
			return
		}

		command, ok := reqParam["vppclicommand"]
		if !ok || command == "" {
			errMsg := fmt.Sprintf("400 Bad request: vppclicommand parameter missing or empty\n")
			p.Log.Error(errMsg)
			p.logError(formatter.JSON(w, http.StatusBadRequest, errMsg))
			return
		}

		p.Log.Debugf("VPPCLI command: %v", command)

		ch, err := p.GoVppmux.NewAPIChannel()
		if err != nil {
			errMsg := fmt.Sprintf("500 Internal server error: error creating channel: %v\n", err)
			p.Log.Error(errMsg)
			p.logError(formatter.JSON(w, http.StatusInternalServerError, errMsg))
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
			errMsg := fmt.Sprintf("500 Internal server error: sending request failed: %v\n", err)
			p.Log.Error(errMsg)
			p.logError(formatter.JSON(w, http.StatusInternalServerError, errMsg))
			return
		} else if reply.Retval > 0 {
			errMsg := fmt.Sprintf("500 Internal server error: request returned error code: %v\n", reply.Retval)
			p.Log.Error(err)
			p.logError(formatter.JSON(w, http.StatusInternalServerError, errMsg))
			return
		}

		p.Log.Debugf("VPPCLI response: %s", reply.Reply)
		p.logError(formatter.JSON(w, http.StatusOK, string(reply.Reply)))
	}
}

func (p *Plugin) sendCommand(ch govppapi.Channel, command string) ([]byte, error) {
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
func (p *Plugin) telemetryHandler(formatter *render.Render) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {

		ch, err := p.GoVppmux.NewAPIChannel()
		if err != nil {
			errMsg := fmt.Sprintf("500 Internal server error: error creating channel: %v\n", err)
			p.Log.Error(errMsg)
			p.logError(formatter.JSON(w, http.StatusInternalServerError, errMsg))
			return
		}
		defer ch.Close()

		type cmdOut struct {
			Command string
			Output  interface{}
		}
		var cmdOuts []cmdOut

		var runCmd = func(command string) {
			out, err := p.sendCommand(ch, command)
			if err != nil {
				errMsg := fmt.Sprintf("500 Internal server error: sending command failed: %v\n", err)
				p.Log.Error(errMsg)
				p.logError(formatter.JSON(w, http.StatusInternalServerError, errMsg))
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

		p.logError(formatter.JSON(w, http.StatusOK, cmdOuts))
	}
}

// telemetryMemoryHandler - returns various telemetry data
func (p *Plugin) telemetryMemoryHandler(formatter *render.Render) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {

		ch, err := p.GoVppmux.NewAPIChannel()
		if err != nil {
			errMsg := fmt.Sprintf("500 Internal server error: error creating channel: %v\n", err)
			p.Log.Error(errMsg)
			p.logError(formatter.JSON(w, http.StatusInternalServerError, errMsg))
			return
		}
		defer ch.Close()

		info, err := vppcalls.GetMemory(ch)
		if err != nil {
			errMsg := fmt.Sprintf("500 Internal server error: sending command failed: %v\n", err)
			p.Log.Error(errMsg)
			p.logError(formatter.JSON(w, http.StatusInternalServerError, errMsg))
			return
		}

		p.logError(formatter.JSON(w, http.StatusOK, info))
	}
}

// telemetryHandler - returns various telemetry data
func (p *Plugin) telemetryRuntimeHandler(formatter *render.Render) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {

		ch, err := p.GoVppmux.NewAPIChannel()
		if err != nil {
			errMsg := fmt.Sprintf("500 Internal server error: error creating channel: %v\n", err)
			p.Log.Error(errMsg)
			p.logError(formatter.JSON(w, http.StatusInternalServerError, errMsg))
			return
		}
		defer ch.Close()

		runtimeInfo, err := vppcalls.GetRuntimeInfo(ch)
		if err != nil {
			errMsg := fmt.Sprintf("500 Internal server error: sending command failed: %v\n", err)
			p.Log.Error(errMsg)
			p.logError(formatter.JSON(w, http.StatusInternalServerError, errMsg))
			return
		}

		p.logError(formatter.JSON(w, http.StatusOK, runtimeInfo))
	}
}

// telemetryHandler - returns various telemetry data
func (p *Plugin) telemetryNodeCountHandler(formatter *render.Render) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {

		ch, err := p.GoVppmux.NewAPIChannel()
		if err != nil {
			errMsg := fmt.Sprintf("500 Internal server error: error creating channel: %v\n", err)
			p.Log.Error(errMsg)
			p.logError(formatter.JSON(w, http.StatusInternalServerError, errMsg))
			return
		}
		defer ch.Close()

		nodeCounters, err := vppcalls.GetNodeCounters(ch)
		if err != nil {
			errMsg := fmt.Sprintf("500 Internal server error: sending command failed: %v\n", err)
			p.Log.Error(errMsg)
			p.logError(formatter.JSON(w, http.StatusInternalServerError, errMsg))
			return
		}

		p.logError(formatter.JSON(w, http.StatusOK, nodeCounters))
	}
}

// tracerHandler - returns binary API call trace
func (p *Plugin) tracerHandler(formatter *render.Render) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		ch, err := p.GoVppmux.NewAPIChannel()
		if err != nil {
			errMsg := fmt.Sprintf("500 Internal server error: error creating channel: %v\n", err)
			p.Log.Error(errMsg)
			p.logError(formatter.JSON(w, http.StatusInternalServerError, errMsg))
			return
		}
		defer ch.Close()

		entries := p.GoVppmux.GetTrace()
		if err != nil {
			errMsg := fmt.Sprintf("500 Internal server error: sending command failed: %v\n", err)
			p.Log.Error(errMsg)
			p.logError(formatter.JSON(w, http.StatusInternalServerError, errMsg))
			return
		}
		if entries == nil {
			p.logError(formatter.JSON(w, http.StatusOK, "VPP api trace is disabled"))
			return
		}

		p.logError(formatter.JSON(w, http.StatusOK, entries))
	}
}

// logError logs non-nil errors from JSON formatter
func (p *Plugin) logError(err error) {
	if err != nil {
		p.Log.Error(err)
	}
}
