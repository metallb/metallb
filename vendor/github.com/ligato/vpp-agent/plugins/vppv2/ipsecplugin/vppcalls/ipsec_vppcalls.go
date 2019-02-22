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
	"encoding/hex"
	"net"
	"strconv"

	"github.com/go-errors/errors"

	"github.com/ligato/cn-infra/utils/addrs"
	ipsec "github.com/ligato/vpp-agent/api/models/vpp/ipsec"
	api "github.com/ligato/vpp-agent/plugins/vpp/binapi/ipsec"
)

// AddSPD implements IPSec handler.
func (h *IPSecVppHandler) AddSPD(spdID uint32) error {
	return h.spdAddDel(spdID, true)
}

// DeleteSPD implements IPSec handler.
func (h *IPSecVppHandler) DeleteSPD(spdID uint32) error {
	return h.spdAddDel(spdID, false)
}

// AddSPDEntry implements IPSec handler.
func (h *IPSecVppHandler) AddSPDEntry(spdID, saID uint32, spd *ipsec.SecurityPolicyDatabase_PolicyEntry) error {
	return h.spdAddDelEntry(spdID, saID, spd, true)
}

// DeleteSPDEntry implements IPSec handler.
func (h *IPSecVppHandler) DeleteSPDEntry(spdID, saID uint32, spd *ipsec.SecurityPolicyDatabase_PolicyEntry) error {
	return h.spdAddDelEntry(spdID, saID, spd, false)
}

// AddSPDInterface implements IPSec handler.
func (h *IPSecVppHandler) AddSPDInterface(spdID uint32, ifaceCfg *ipsec.SecurityPolicyDatabase_Interface) error {
	ifaceMeta, found := h.ifIndexes.LookupByName(ifaceCfg.Name)
	if !found {
		return errors.New("failed to get interface metadata")
	}
	return h.interfaceAddDelSpd(spdID, ifaceMeta.SwIfIndex, true)
}

// DeleteSPDInterface implements IPSec handler.
func (h *IPSecVppHandler) DeleteSPDInterface(spdID uint32, ifaceCfg *ipsec.SecurityPolicyDatabase_Interface) error {
	ifaceMeta, found := h.ifIndexes.LookupByName(ifaceCfg.Name)
	if !found {
		return errors.New("failed to get interface metadata")
	}
	return h.interfaceAddDelSpd(spdID, ifaceMeta.SwIfIndex, false)
}

// AddSA implements IPSec handler.
func (h *IPSecVppHandler) AddSA(sa *ipsec.SecurityAssociation) error {
	return h.sadAddDelEntry(sa, true)
}

// DeleteSA implements IPSec handler.
func (h *IPSecVppHandler) DeleteSA(sa *ipsec.SecurityAssociation) error {
	return h.sadAddDelEntry(sa, false)
}

func (h *IPSecVppHandler) spdAddDel(spdID uint32, isAdd bool) error {
	req := &api.IpsecSpdAddDel{
		IsAdd: boolToUint(isAdd),
		SpdID: spdID,
	}
	reply := &api.IpsecSpdAddDelReply{}

	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return err
	}

	return nil
}

func (h *IPSecVppHandler) spdAddDelEntry(spdID, saID uint32, spd *ipsec.SecurityPolicyDatabase_PolicyEntry, isAdd bool) error {
	req := &api.IpsecSpdAddDelEntry{
		IsAdd:           boolToUint(isAdd),
		SpdID:           spdID,
		Priority:        spd.Priority,
		IsOutbound:      boolToUint(spd.IsOutbound),
		Protocol:        uint8(spd.Protocol),
		RemotePortStart: uint16(spd.RemotePortStart),
		RemotePortStop:  uint16(spd.RemotePortStop),
		LocalPortStart:  uint16(spd.LocalPortStart),
		LocalPortStop:   uint16(spd.LocalPortStop),
		Policy:          uint8(spd.Action),
		SaID:            saID,
	}
	if req.RemotePortStop == 0 {
		req.RemotePortStop = ^req.RemotePortStop
	}
	if req.LocalPortStop == 0 {
		req.LocalPortStop = ^req.LocalPortStop
	}
	if spd.RemoteAddrStart != "" {
		isIPv6, err := addrs.IsIPv6(spd.RemoteAddrStart)
		if err != nil {
			return err
		}
		if isIPv6 {
			req.IsIPv6 = 1
			req.RemoteAddressStart = net.ParseIP(spd.RemoteAddrStart).To16()
			req.RemoteAddressStop = net.ParseIP(spd.RemoteAddrStop).To16()
			req.LocalAddressStart = net.ParseIP(spd.LocalAddrStart).To16()
			req.LocalAddressStop = net.ParseIP(spd.LocalAddrStop).To16()
		} else {
			req.IsIPv6 = 0
			req.RemoteAddressStart = net.ParseIP(spd.RemoteAddrStart).To4()
			req.RemoteAddressStop = net.ParseIP(spd.RemoteAddrStop).To4()
			req.LocalAddressStart = net.ParseIP(spd.LocalAddrStart).To4()
			req.LocalAddressStop = net.ParseIP(spd.LocalAddrStop).To4()
		}
	} else {
		req.RemoteAddressStart = net.ParseIP("0.0.0.0").To4()
		req.RemoteAddressStop = net.ParseIP("255.255.255.255").To4()
		req.LocalAddressStart = net.ParseIP("0.0.0.0").To4()
		req.LocalAddressStop = net.ParseIP("255.255.255.255").To4()
	}
	reply := &api.IpsecSpdAddDelEntryReply{}

	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return err
	}

	return nil
}

func (h *IPSecVppHandler) interfaceAddDelSpd(spdID, swIfIdx uint32, isAdd bool) error {
	req := &api.IpsecInterfaceAddDelSpd{
		IsAdd:     boolToUint(isAdd),
		SwIfIndex: swIfIdx,
		SpdID:     spdID,
	}
	reply := &api.IpsecInterfaceAddDelSpdReply{}

	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return err
	}

	return nil
}

func (h *IPSecVppHandler) sadAddDelEntry(sa *ipsec.SecurityAssociation, isAdd bool) error {
	cryptoKey, err := hex.DecodeString(sa.CryptoKey)
	if err != nil {
		return err
	}
	integKey, err := hex.DecodeString(sa.IntegKey)
	if err != nil {
		return err
	}

	saID, err := strconv.Atoi(sa.Index)
	if err != nil {
		return err
	}

	req := &api.IpsecSadAddDelEntry{
		IsAdd:                     boolToUint(isAdd),
		SadID:                     uint32(saID),
		Spi:                       sa.Spi,
		Protocol:                  uint8(sa.Protocol),
		CryptoAlgorithm:           uint8(sa.CryptoAlg),
		CryptoKey:                 cryptoKey,
		CryptoKeyLength:           uint8(len(cryptoKey)),
		IntegrityAlgorithm:        uint8(sa.IntegAlg),
		IntegrityKey:              integKey,
		IntegrityKeyLength:        uint8(len(integKey)),
		UseExtendedSequenceNumber: boolToUint(sa.UseEsn),
		UseAntiReplay:             boolToUint(sa.UseAntiReplay),
		UDPEncap:                  boolToUint(sa.EnableUdpEncap),
	}
	if sa.TunnelSrcAddr != "" {
		req.IsTunnel = 1
		isIPv6, err := addrs.IsIPv6(sa.TunnelSrcAddr)
		if err != nil {
			return err
		}
		if isIPv6 {
			req.IsTunnelIPv6 = 1
			req.TunnelSrcAddress = net.ParseIP(sa.TunnelSrcAddr).To16()
			req.TunnelDstAddress = net.ParseIP(sa.TunnelDstAddr).To16()
		} else {
			req.IsTunnelIPv6 = 0
			req.TunnelSrcAddress = net.ParseIP(sa.TunnelSrcAddr).To4()
			req.TunnelDstAddress = net.ParseIP(sa.TunnelDstAddr).To4()
		}
	}
	reply := &api.IpsecSadAddDelEntryReply{}

	if err = h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return err
	}

	return nil
}

func boolToUint(value bool) uint8 {
	if value {
		return 1
	}
	return 0
}
