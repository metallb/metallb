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

	ipsecapi "github.com/ligato/vpp-agent/plugins/vpp/binapi/ipsec"
	"github.com/ligato/vpp-agent/plugins/vpp/model/ipsec"
)

// IPSecSaDetails holds security association with VPP metadata
type IPSecSaDetails struct {
	Sa   *ipsec.SecurityAssociations_SA
	Meta *IPSecSaMeta
}

// IPSecSaMeta contains all VPP-specific metadata
type IPSecSaMeta struct {
	SaID           uint32
	Interface      string
	IfIdx          uint32
	CryptoKeyLen   uint8
	IntegKeyLen    uint8
	Salt           uint32
	SeqOutbound    uint64
	LastSeqInbound uint64
	ReplayWindow   uint64
	TotalDataSize  uint64
}

// DumpIPSecSA implements IPSec handler.
func (h *IPSecVppHandler) DumpIPSecSA() (saList []*IPSecSaDetails, err error) {
	return h.DumpIPSecSAWithIndex(^uint32(0)) // Get everything
}

// DumpIPSecSAWithIndex implements IPSec handler.
func (h *IPSecVppHandler) DumpIPSecSAWithIndex(saID uint32) (saList []*IPSecSaDetails, err error) {
	saDetails, err := h.dumpSecurityAssociations(saID)
	if err != nil {
		return nil, err
	}

	for _, saData := range saDetails {
		// Skip tunnel interfaces
		if saData.SwIfIndex != ^uint32(0) {
			continue
		}

		// Addresses
		var tunnelSrcAddrStr, tunnelDstAddrStr string
		if uintToBool(saData.IsTunnelIP6) {
			var tunnelSrcAddr, tunnelDstAddr net.IP = saData.TunnelSrcAddr, saData.TunnelDstAddr
			tunnelSrcAddrStr, tunnelDstAddrStr = tunnelSrcAddr.String(), tunnelDstAddr.String()
		} else {
			var tunnelSrcAddr, tunnelDstAddr net.IP = saData.TunnelSrcAddr[:4], saData.TunnelDstAddr[:4]
			tunnelSrcAddrStr, tunnelDstAddrStr = tunnelSrcAddr.String(), tunnelDstAddr.String()
		}

		sa := &ipsec.SecurityAssociations_SA{
			Spi:            saData.Spi,
			Protocol:       ipsec.SecurityAssociations_SA_IPSecProtocol(saData.Protocol),
			CryptoAlg:      ipsec.CryptoAlgorithm(saData.CryptoAlg),
			CryptoKey:      hex.EncodeToString(saData.CryptoKey[:saData.CryptoKeyLen]),
			IntegAlg:       ipsec.IntegAlgorithm(saData.IntegAlg),
			IntegKey:       hex.EncodeToString(saData.IntegKey[:saData.IntegKeyLen]),
			UseEsn:         uintToBool(saData.UseEsn),
			UseAntiReplay:  uintToBool(saData.UseAntiReplay),
			TunnelSrcAddr:  tunnelSrcAddrStr,
			TunnelDstAddr:  tunnelDstAddrStr,
			EnableUdpEncap: uintToBool(saData.UDPEncap),
		}
		meta := &IPSecSaMeta{
			SaID:           saData.SaID,
			IfIdx:          saData.SwIfIndex,
			CryptoKeyLen:   saData.CryptoKeyLen,
			IntegKeyLen:    saData.IntegKeyLen,
			Salt:           saData.Salt,
			SeqOutbound:    saData.SeqOutbound,
			LastSeqInbound: saData.LastSeqInbound,
			ReplayWindow:   saData.ReplayWindow,
			TotalDataSize:  saData.TotalDataSize,
		}
		saList = append(saList, &IPSecSaDetails{
			Sa:   sa,
			Meta: meta,
		})
	}

	return saList, nil
}

// IPSecTunnelInterfaceDetails hold a list of tunnel interfaces with name/index map as metadata
type IPSecTunnelInterfaceDetails struct {
	Tunnel *ipsec.TunnelInterfaces_Tunnel
	Meta   *IPSecTunnelMeta
}

// IPSecTunnelMeta contains map of name/index pairs
type IPSecTunnelMeta struct {
	SwIfIndex uint32
}

// DumpIPSecTunnelInterfaces implements IPSec handler.
func (h *IPSecVppHandler) DumpIPSecTunnelInterfaces() (tun []*IPSecTunnelInterfaceDetails, err error) {
	saDetails, err := h.dumpSecurityAssociations(^uint32(0))
	if err != nil {
		return nil, err
	}

	// Every tunnel interface is returned in two API calls. To reconstruct the correct proto-modelled data,
	// first appearance is stored, and when the second part arrives, data are completed and stored.
	tunnelParts := make(map[uint32]*ipsecapi.IpsecSaDetails)

	for _, saData := range saDetails {
		// Skip non-tunnel security associations
		if saData.SwIfIndex == ^uint32(0) {
			continue
		}

		// Interface
		ifName, ifData, found := h.ifIndexes.LookupName(saData.SwIfIndex)
		if !found {
			h.log.Warnf("IPSec SA dump: interface name not found for %d", saData.SwIfIndex)
			continue
		}
		if ifData == nil {
			h.log.Warnf("IPSec SA dump: interface %s has no metadata", ifName)
			continue
		}

		// First appearance is stored in the map, the second one is used in configuration.
		firstSaData, ok := tunnelParts[saData.SwIfIndex]
		if !ok {
			tunnelParts[saData.SwIfIndex] = saData
			h.log.Debugf("first part of IPSec tunnel interface %d (name %s) stored", saData.SwIfIndex, ifName)
			continue
		}

		// Addresses
		var tunnelSrcAddrStr, tunnelDstAddrStr string
		if uintToBool(saData.IsTunnelIP6) {
			var tunnelSrcAddr, tunnelDstAddr net.IP = saData.TunnelSrcAddr, saData.TunnelDstAddr
			tunnelSrcAddrStr, tunnelDstAddrStr = tunnelSrcAddr.String(), tunnelDstAddr.String()
		} else {
			var tunnelSrcAddr, tunnelDstAddr net.IP = saData.TunnelSrcAddr[:4], saData.TunnelDstAddr[:4]
			tunnelSrcAddrStr, tunnelDstAddrStr = tunnelSrcAddr.String(), tunnelDstAddr.String()
		}

		// Prepare tunnel interface data
		tunnel := &ipsec.TunnelInterfaces_Tunnel{
			Name:        ifName,
			Esn:         uintToBool(saData.UseEsn),
			AntiReplay:  uintToBool(saData.UseAntiReplay),
			LocalIp:     tunnelSrcAddrStr,
			RemoteIp:    tunnelDstAddrStr,
			LocalSpi:    saData.Spi,
			RemoteSpi:   firstSaData.Spi, // Fill remote SPI from stored SA data
			CryptoAlg:   ipsec.CryptoAlgorithm(saData.CryptoAlg),
			IntegAlg:    ipsec.IntegAlgorithm(saData.IntegAlg),
			Enabled:     ifData.Enabled,
			IpAddresses: ifData.IpAddresses,
			Vrf:         ifData.Vrf,
		}
		tun = append(tun, &IPSecTunnelInterfaceDetails{
			Tunnel: tunnel,
			Meta: &IPSecTunnelMeta{
				SwIfIndex: saData.SwIfIndex,
			},
		})
	}

	return tun, nil
}

// IPSecSpdDetails represents IPSec policy databases with particular metadata
type IPSecSpdDetails struct {
	Spd         *ipsec.SecurityPolicyDatabases_SPD
	PolicyMeta  map[string]*SpdMeta // SA-generated name is a key
	NumPolicies uint32
}

// SpdMeta hold VPP-specific data related to SPD
type SpdMeta struct {
	SaID    uint32
	SpdID   uint32
	Policy  uint8
	Bytes   uint64
	Packets uint64
}

// DumpIPSecSPD implements IPSec handler.
func (h *IPSecVppHandler) DumpIPSecSPD() (spdList []*IPSecSpdDetails, err error) {
	metadata := make(map[string]*SpdMeta)

	// Get all VPP SPD indexes
	spdIndexes, err := h.dumpSpdIndexes()
	if err != nil {
		return nil, errors.Errorf("failed to dump SPD indexes: %v", err)
	}
	for spdIdx, numPolicies := range spdIndexes {
		spd := &ipsec.SecurityPolicyDatabases_SPD{}

		req := &ipsecapi.IpsecSpdDump{
			SpdID: spdIdx,
			SaID:  ^uint32(0),
		}
		requestCtx := h.callsChannel.SendMultiRequest(req)

		for {
			spdDetails := &ipsecapi.IpsecSpdDetails{}
			stop, err := requestCtx.ReceiveReply(spdDetails)
			if stop {
				break
			}
			if err != nil {
				return nil, err
			}

			// Security association name, to distinguish metadata. Generated name points to SA, so the name can be
			// the same as for other policies.
			saGenName := "sa-id-" + strconv.Itoa(int(spdDetails.SaID))

			// Addresses
			var remoteStartAddrStr, remoteStopAddrStr, localStartAddrStr, localStopAddrStr string
			if uintToBool(spdDetails.IsIPv6) {
				var remoteStartAddr, remoteStopAddr net.IP = spdDetails.RemoteStartAddr, spdDetails.RemoteStopAddr
				remoteStartAddrStr, remoteStopAddrStr = remoteStartAddr.String(), remoteStopAddr.String()
				var localStartAddr, localStopAddr net.IP = spdDetails.LocalStartAddr, spdDetails.LocalStopAddr
				localStartAddrStr, localStopAddrStr = localStartAddr.String(), localStopAddr.String()
			} else {
				var remoteStartAddr, remoteStopAddr net.IP = spdDetails.RemoteStartAddr[:4], spdDetails.RemoteStopAddr[:4]
				remoteStartAddrStr, remoteStopAddrStr = remoteStartAddr.String(), remoteStopAddr.String()
				var localStartAddr, localStopAddr net.IP = spdDetails.LocalStartAddr[:4], spdDetails.LocalStopAddr[:4]
				localStartAddrStr, localStopAddrStr = localStartAddr.String(), localStopAddr.String()
			}

			// Prepare policy entry and put to the SPD
			policyEntry := &ipsec.SecurityPolicyDatabases_SPD_PolicyEntry{
				Sa:              saGenName,
				Priority:        spdDetails.Priority,
				IsOutbound:      uintToBool(spdDetails.IsOutbound),
				RemoteAddrStart: remoteStartAddrStr,
				RemoteAddrStop:  remoteStopAddrStr,
				LocalAddrStart:  localStartAddrStr,
				LocalAddrStop:   localStopAddrStr,
				Protocol:        uint32(spdDetails.Protocol),
				RemotePortStart: uint32(spdDetails.RemoteStartPort),
				RemotePortStop:  uint32(spdDetails.RemoteStopPort),
				LocalPortStart:  uint32(spdDetails.LocalStartPort),
				LocalPortStop:   uint32(spdDetails.LocalStopPort),
			}
			spd.PolicyEntries = append(spd.PolicyEntries, policyEntry)

			// Prepare meta and put to the metadata map
			meta := &SpdMeta{
				SpdID:   spdDetails.SpdID,
				SaID:    spdDetails.SaID,
				Policy:  spdDetails.Policy,
				Bytes:   spdDetails.Bytes,
				Packets: spdDetails.Packets,
			}
			metadata[saGenName] = meta
		}
		// Store SPD in list
		spdList = append(spdList, &IPSecSpdDetails{
			Spd:         spd,
			PolicyMeta:  metadata,
			NumPolicies: numPolicies,
		})
	}

	return spdList, nil
}

// Get all indexes of SPD configured on the VPP
func (h *IPSecVppHandler) dumpSpdIndexes() (map[uint32]uint32, error) {
	// SPD index to number of policies
	spdIndexes := make(map[uint32]uint32)

	req := &ipsecapi.IpsecSpdsDump{}
	reqCtx := h.callsChannel.SendMultiRequest(req)

	for {
		spdDetails := &ipsecapi.IpsecSpdsDetails{}
		stop, err := reqCtx.ReceiveReply(spdDetails)
		if stop {
			break
		}
		if err != nil {
			return nil, err
		}

		spdIndexes[spdDetails.SpdID] = spdDetails.Npolicies
	}

	return spdIndexes, nil
}

// Get all security association (used also for tunnel interfaces) in binary api format
func (h *IPSecVppHandler) dumpSecurityAssociations(saID uint32) (saList []*ipsecapi.IpsecSaDetails, err error) {
	req := &ipsecapi.IpsecSaDump{
		SaID: saID,
	}
	requestCtx := h.callsChannel.SendMultiRequest(req)

	for {
		saDetails := &ipsecapi.IpsecSaDetails{}
		stop, err := requestCtx.ReceiveReply(saDetails)
		if stop {
			break
		}
		if err != nil {
			return nil, err
		}

		saList = append(saList, saDetails)
	}

	return saList, nil
}

func uintToBool(input uint8) bool {
	if input == 1 {
		return true
	}
	return false
}
