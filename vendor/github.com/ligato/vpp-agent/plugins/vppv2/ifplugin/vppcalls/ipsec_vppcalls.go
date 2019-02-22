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
	"fmt"
	"net"

	interfaces "github.com/ligato/vpp-agent/api/models/vpp/interfaces"
	api "github.com/ligato/vpp-agent/plugins/vpp/binapi/ipsec"
)

// AddIPSecTunnelInterface adds a new IPSec tunnel interface.
func (h *IfVppHandler) AddIPSecTunnelInterface(ifName string, ipSecLink *interfaces.IPSecLink) (uint32, error) {
	return h.tunnelIfAddDel(ifName, ipSecLink, true)
}

// DeleteIPSecTunnelInterface removes existing IPSec tunnel interface.
func (h *IfVppHandler) DeleteIPSecTunnelInterface(ifName string, ipSecLink *interfaces.IPSecLink) error {
	// Note: ifIdx is not used now, tunnel should be matched based on parameters
	_, err := h.tunnelIfAddDel(ifName, ipSecLink, false)
	return err
}

func (h *IfVppHandler) tunnelIfAddDel(ifName string, ipSecLink *interfaces.IPSecLink, isAdd bool) (uint32, error) {
	localCryptoKey, err := hex.DecodeString(ipSecLink.LocalCryptoKey)
	if err != nil {
		return 0, err
	}
	remoteCryptoKey, err := hex.DecodeString(ipSecLink.RemoteCryptoKey)
	if err != nil {
		return 0, err
	}
	localIntegKey, err := hex.DecodeString(ipSecLink.LocalIntegKey)
	if err != nil {
		return 0, err
	}
	remoteIntegKey, err := hex.DecodeString(ipSecLink.RemoteIntegKey)
	if err != nil {
		return 0, err
	}

	req := &api.IpsecTunnelIfAddDel{
		IsAdd:              boolToUint(isAdd),
		Esn:                boolToUint(ipSecLink.Esn),
		AntiReplay:         boolToUint(ipSecLink.AntiReplay),
		LocalIP:            net.ParseIP(ipSecLink.LocalIp).To4(),
		RemoteIP:           net.ParseIP(ipSecLink.RemoteIp).To4(),
		LocalSpi:           ipSecLink.LocalSpi,
		RemoteSpi:          ipSecLink.RemoteSpi,
		CryptoAlg:          uint8(ipSecLink.CryptoAlg),
		LocalCryptoKey:     localCryptoKey,
		LocalCryptoKeyLen:  uint8(len(localCryptoKey)),
		RemoteCryptoKey:    remoteCryptoKey,
		RemoteCryptoKeyLen: uint8(len(remoteCryptoKey)),
		IntegAlg:           uint8(ipSecLink.IntegAlg),
		LocalIntegKey:      localIntegKey,
		LocalIntegKeyLen:   uint8(len(localIntegKey)),
		RemoteIntegKey:     remoteIntegKey,
		RemoteIntegKeyLen:  uint8(len(remoteIntegKey)),
		UDPEncap:           boolToUint(ipSecLink.EnableUdpEncap),
	}
	reply := &api.IpsecTunnelIfAddDelReply{}

	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return 0, err
	} else if reply.Retval != 0 {
		return 0, fmt.Errorf("%s returned %d", reply.GetMessageName(), reply.Retval)
	}

	return reply.SwIfIndex, nil
}
