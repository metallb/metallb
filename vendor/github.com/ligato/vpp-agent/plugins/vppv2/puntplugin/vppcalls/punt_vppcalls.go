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

package vppcalls

import (
	"net"
	"strings"

	"github.com/go-errors/errors"
	punt "github.com/ligato/vpp-agent/api/models/vpp/punt"
	api_ip "github.com/ligato/vpp-agent/plugins/vpp/binapi/ip"
	api_punt "github.com/ligato/vpp-agent/plugins/vpp/binapi/punt"
)

// AddPunt configures new punt entry
func (h *PuntVppHandler) AddPunt(puntCfg *punt.ToHost) error {
	return h.handlePuntToHost(puntCfg, true)
}

// DeletePunt removes punt entry
func (h *PuntVppHandler) DeletePunt(puntCfg *punt.ToHost) error {
	return h.handlePuntToHost(puntCfg, false)
}

// RegisterPuntSocket registers new punt to socket
func (h *PuntVppHandler) RegisterPuntSocket(puntCfg *punt.ToHost) error {
	if puntCfg.L3Protocol == punt.L3Protocol_IPv4 {
		return h.registerPuntWithSocketIPv4(puntCfg)
	} else if puntCfg.L3Protocol == punt.L3Protocol_IPv6 {
		return h.registerPuntWithSocketIPv6(puntCfg)
	}
	// if L3 is set to all, register both, IPv4 and IPv6
	err := h.registerPuntWithSocketIPv4(puntCfg)
	if err != nil {
		return err
	}
	return h.registerPuntWithSocketIPv6(puntCfg)
}

// DeregisterPuntSocket removes existing punt to socket sogistration
func (h *PuntVppHandler) DeregisterPuntSocket(puntCfg *punt.ToHost) error {
	if puntCfg.L3Protocol == punt.L3Protocol_IPv4 {
		return h.registerPuntWithSocketIPv4(puntCfg)
	} else if puntCfg.L3Protocol == punt.L3Protocol_IPv6 {
		return h.registerPuntWithSocketIPv6(puntCfg)
	}
	// if L3 is set to all, deregister both, IPv4 and IPv6
	err := h.registerPuntWithSocketIPv4(puntCfg)
	if err != nil {
		return err
	}
	return h.registerPuntWithSocketIPv6(puntCfg)
}

// AddPuntRedirect adds new redirect entry
func (h *PuntVppHandler) AddPuntRedirect(puntCfg *punt.IPRedirect) error {
	if puntCfg.L3Protocol == punt.L3Protocol_IPv4 {
		return h.handlePuntRedirectIPv4(puntCfg, true)
	} else if puntCfg.L3Protocol == punt.L3Protocol_IPv6 {
		return h.handlePuntRedirectIPv6(puntCfg, true)
	}
	// un-configure both, IPv4 and IPv6
	err := h.handlePuntRedirectIPv4(puntCfg, true)
	if err != nil {
		return err
	}
	return h.handlePuntRedirectIPv6(puntCfg, true)
}

// DeletePuntRedirect removes existing redirect entry
func (h *PuntVppHandler) DeletePuntRedirect(puntCfg *punt.IPRedirect) error {
	if puntCfg.L3Protocol == punt.L3Protocol_IPv4 {
		return h.handlePuntRedirectIPv4(puntCfg, false)
	} else if puntCfg.L3Protocol == punt.L3Protocol_IPv6 {
		return h.handlePuntRedirectIPv6(puntCfg, false)
	}
	// un-configure both, IPv4 and IPv6
	err := h.handlePuntRedirectIPv6(puntCfg, false)
	if err != nil {
		return err
	}
	return h.handlePuntRedirectIPv4(puntCfg, false)
}

func (h *PuntVppHandler) handlePuntToHost(punt *punt.ToHost, isAdd bool) error {
	req := &api_punt.Punt{
		IsAdd:      boolToUint(isAdd),
		IPv:        resolveL3Proto(punt.L3Protocol),
		L4Protocol: resolveL4Proto(punt.L4Protocol),
		L4Port:     uint16(punt.Port),
	}
	reply := &api_punt.PuntReply{}

	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return err
	}

	return nil
}

func (h *PuntVppHandler) registerPuntWithSocketIPv4(punt *punt.ToHost) error {
	return h.registerPuntWithSocket(punt, true)
}

func (h *PuntVppHandler) registerPuntWithSocketIPv6(punt *punt.ToHost) error {
	return h.registerPuntWithSocket(punt, false)
}

func (h *PuntVppHandler) registerPuntWithSocket(punt *punt.ToHost, isIPv4 bool) error {
	pathName := []byte(punt.SocketPath)
	pathByte := make([]byte, 108) // linux sun_path defined to 108 bytes as by unix(7)
	for i, c := range pathName {
		pathByte[i] = c
	}

	req := &api_punt.PuntSocketRegister{
		HeaderVersion: 1,
		IsIP4:         boolToUint(isIPv4),
		L4Protocol:    resolveL4Proto(punt.L4Protocol),
		L4Port:        uint16(punt.Port),
		Pathname:      pathByte,
	}
	reply := &api_punt.PuntSocketRegisterReply{}

	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return err
	}

	p := *punt
	p.SocketPath = strings.SplitN(string(reply.Pathname), "\x00", 2)[0]
	socketPathMap[punt.Port] = &p

	return nil
}

func (h *PuntVppHandler) unregisterPuntWithSocketIPv4(punt *punt.ToHost) error {
	return h.unregisterPuntWithSocket(punt, true)
}

func (h *PuntVppHandler) unregisterPuntWithSocketIPv6(punt *punt.ToHost) error {
	return h.unregisterPuntWithSocket(punt, false)
}

func (h *PuntVppHandler) unregisterPuntWithSocket(punt *punt.ToHost, isIPv4 bool) error {
	req := &api_punt.PuntSocketDeregister{
		IsIP4:      boolToUint(isIPv4),
		L4Protocol: resolveL4Proto(punt.L4Protocol),
		L4Port:     uint16(punt.Port),
	}
	reply := &api_punt.PuntSocketDeregisterReply{}

	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return err
	}

	delete(socketPathMap, punt.Port)

	return nil
}

func (h *PuntVppHandler) handlePuntRedirectIPv4(punt *punt.IPRedirect, isAdd bool) error {
	return h.handlePuntRedirect(punt, true, isAdd)
}

func (h *PuntVppHandler) handlePuntRedirectIPv6(punt *punt.IPRedirect, isAdd bool) error {
	return h.handlePuntRedirect(punt, false, isAdd)
}

func (h *PuntVppHandler) handlePuntRedirect(punt *punt.IPRedirect, isIPv4, isAdd bool) error {
	// rx interface
	var rxIfIdx uint32
	if punt.RxInterface == "" {
		rxIfIdx = ^uint32(0)
	} else {
		rxMetadata, exists := h.ifIndexes.LookupByName(punt.RxInterface)
		if !exists {
			return errors.Errorf("index not found for interface %s", punt.RxInterface)
		}
		rxIfIdx = rxMetadata.SwIfIndex
	}

	// tx interface
	txMetadata, exists := h.ifIndexes.LookupByName(punt.TxInterface)
	if !exists {
		return errors.Errorf("index not found for interface %s", punt.TxInterface)
	}

	// next hop address
	//  - remove mask from IP address if necessary
	nextHopStr := punt.NextHop
	ipParts := strings.Split(punt.NextHop, "/")
	if len(ipParts) > 1 {
		h.log.Debugf("IP punt redirect next hop IP address %s is defined with mask, removing it")
		nextHopStr = ipParts[0]
	}
	var nextHop []byte
	if isIPv4 {
		nextHop = net.ParseIP(nextHopStr).To4()
	} else {
		nextHop = net.ParseIP(nextHopStr).To16()
	}

	req := &api_ip.IPPuntRedirect{
		IsAdd:       boolToUint(isAdd),
		IsIP6:       boolToUint(!isIPv4),
		RxSwIfIndex: rxIfIdx,
		TxSwIfIndex: txMetadata.SwIfIndex,
		Nh:          nextHop,
	}
	reply := &api_ip.IPPuntRedirectReply{}
	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return err
	}

	return nil
}

func resolveL3Proto(protocol punt.L3Protocol) uint8 {
	switch protocol {
	case punt.L3Protocol_IPv4:
		return uint8(punt.L3Protocol_IPv4)
	case punt.L3Protocol_IPv6:
		return uint8(punt.L3Protocol_IPv6)
	case punt.L3Protocol_ALL:
		return ^uint8(0) // binary API representation for both protocols
	}
	return uint8(punt.L3Protocol_UNDEFINED_L3)
}

func resolveL4Proto(protocol punt.L4Protocol) uint8 {
	switch protocol {
	case punt.L4Protocol_TCP:
		return uint8(punt.L4Protocol_TCP)
	case punt.L4Protocol_UDP:
		return uint8(punt.L4Protocol_UDP)
	}
	return uint8(punt.L4Protocol_UNDEFINED_L4)
}

func boolToUint(input bool) uint8 {
	if input {
		return 1
	}
	return 0
}
