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

	ipsec "github.com/ligato/vpp-agent/api/models/vpp/ipsec"
	ipsecapi "github.com/ligato/vpp-agent/plugins/vpp/binapi/ipsec"
)

// IPSecSaDetails holds security association with VPP metadata
type IPSecSaDetails struct {
	Sa   *ipsec.SecurityAssociation
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

		sa := &ipsec.SecurityAssociation{
			Index:          strconv.Itoa(int(saData.SaID)),
			Spi:            saData.Spi,
			Protocol:       ipsec.SecurityAssociation_IPSecProtocol(saData.Protocol),
			CryptoAlg:      ipsec.SecurityAssociation_CryptoAlg(saData.CryptoAlg),
			CryptoKey:      hex.EncodeToString(saData.CryptoKey[:saData.CryptoKeyLen]),
			IntegAlg:       ipsec.SecurityAssociation_IntegAlg(saData.IntegAlg),
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

// IPSecSpdDetails represents IPSec policy databases with particular metadata
type IPSecSpdDetails struct {
	Spd         *ipsec.SecurityPolicyDatabase
	PolicyMeta  map[string]*SpdMeta // SA index name is a key
	NumPolicies uint32
}

// SpdMeta hold VPP-specific data related to SPD
type SpdMeta struct {
	SaID    uint32
	Policy  uint8
	Bytes   uint64
	Packets uint64
}

// DumpIPSecSPD implements IPSec handler.
func (h *IPSecVppHandler) DumpIPSecSPD() (spdList []*IPSecSpdDetails, err error) {
	metadata := make(map[string]*SpdMeta)

	// TODO dump IPSec SPD interfaces is not available in current VPP version

	// Get all VPP SPD indexes
	spdIndexes, err := h.dumpSpdIndexes()
	if err != nil {
		return nil, errors.Errorf("failed to dump SPD indexes: %v", err)
	}
	for spdIdx, numPolicies := range spdIndexes {
		spd := &ipsec.SecurityPolicyDatabase{
			Index: strconv.Itoa(spdIdx),
		}

		req := &ipsecapi.IpsecSpdDump{
			SpdID: uint32(spdIdx),
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
			policyEntry := &ipsec.SecurityPolicyDatabase_PolicyEntry{
				SaIndex:         strconv.Itoa(int(spdDetails.SaID)),
				Priority:        spdDetails.Priority,
				IsOutbound:      uintToBool(spdDetails.IsOutbound),
				RemoteAddrStart: remoteStartAddrStr,
				RemoteAddrStop:  remoteStopAddrStr,
				LocalAddrStart:  localStartAddrStr,
				LocalAddrStop:   localStopAddrStr,
				Protocol:        uint32(spdDetails.Protocol),
				RemotePortStart: uint32(spdDetails.RemoteStartPort),
				RemotePortStop:  resetPort(spdDetails.RemoteStopPort),
				LocalPortStart:  uint32(spdDetails.LocalStartPort),
				LocalPortStop:   resetPort(spdDetails.LocalStopPort),
				Action:          ipsec.SecurityPolicyDatabase_PolicyEntry_Action(spdDetails.Policy),
			}
			spd.PolicyEntries = append(spd.PolicyEntries, policyEntry)

			// Prepare meta and put to the metadata map
			meta := &SpdMeta{
				SaID:    spdDetails.SaID,
				Policy:  spdDetails.Policy,
				Bytes:   spdDetails.Bytes,
				Packets: spdDetails.Packets,
			}
			metadata[strconv.Itoa(int(spdDetails.SaID))] = meta
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
func (h *IPSecVppHandler) dumpSpdIndexes() (map[int]uint32, error) {
	// SPD index to number of policies
	spdIndexes := make(map[int]uint32)

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

		spdIndexes[int(spdDetails.SpdID)] = spdDetails.Npolicies
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

// ResetPort returns 0 if stop port has maximum value (default VPP value if stop port is not defined)
func resetPort(port uint16) uint32 {
	if port == ^uint16(0) {
		return 0
	}
	return uint32(port)
}

func uintToBool(input uint8) bool {
	if input == 1 {
		return true
	}
	return false
}
