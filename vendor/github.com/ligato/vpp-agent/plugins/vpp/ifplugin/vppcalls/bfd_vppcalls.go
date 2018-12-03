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

package vppcalls

import (
	"fmt"
	"net"
	"strconv"

	"github.com/ligato/cn-infra/utils/addrs"
	"github.com/ligato/vpp-agent/idxvpp"
	bfd_api "github.com/ligato/vpp-agent/plugins/vpp/binapi/bfd"
	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin/ifaceidx"
	"github.com/ligato/vpp-agent/plugins/vpp/model/bfd"
)

// AddBfdUDPSession implements BFD handler.
func (h *BfdVppHandler) AddBfdUDPSession(bfdSess *bfd.SingleHopBFD_Session, ifIdx uint32, bfdKeyIndexes idxvpp.NameToIdx) error {
	req := &bfd_api.BfdUDPAdd{
		SwIfIndex:     ifIdx,
		DesiredMinTx:  bfdSess.DesiredMinTxInterval,
		RequiredMinRx: bfdSess.RequiredMinRxInterval,
		DetectMult:    uint8(bfdSess.DetectMultiplier),
	}

	isLocalIpv6, err := addrs.IsIPv6(bfdSess.SourceAddress)
	if err != nil {
		return err
	}
	isPeerIpv6, err := addrs.IsIPv6(bfdSess.DestinationAddress)
	if err != nil {
		return err
	}
	if isLocalIpv6 && isPeerIpv6 {
		req.IsIPv6 = 1
		req.LocalAddr = net.ParseIP(bfdSess.SourceAddress).To16()
		req.PeerAddr = net.ParseIP(bfdSess.DestinationAddress).To16()
	} else if !isLocalIpv6 && !isPeerIpv6 {
		req.IsIPv6 = 0
		req.LocalAddr = net.ParseIP(bfdSess.SourceAddress).To4()
		req.PeerAddr = net.ParseIP(bfdSess.DestinationAddress).To4()
	} else {
		return fmt.Errorf("different IP versions or missing IP address. Local: %v, Peer: %v",
			bfdSess.SourceAddress, bfdSess.DestinationAddress)
	}

	// Authentication
	if bfdSess.Authentication != nil {
		keyID := strconv.Itoa(int(bfdSess.Authentication.KeyId))
		h.log.Infof("Setting up authentication with index %v", keyID)
		_, _, found := bfdKeyIndexes.LookupIdx(keyID)
		if found {
			req.IsAuthenticated = 1
			req.BfdKeyID = uint8(bfdSess.Authentication.KeyId)
			req.ConfKeyID = bfdSess.Authentication.AdvertisedKeyId
		} else {
			h.log.Infof("Authentication key %v not found", bfdSess.Authentication.KeyId)
			req.IsAuthenticated = 0
		}
	} else {
		req.IsAuthenticated = 0
	}
	reply := &bfd_api.BfdUDPAddReply{}

	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return err
	} else if reply.Retval != 0 {
		return fmt.Errorf("%s returned %d", reply.GetMessageName(), reply.Retval)
	}

	return nil
}

// AddBfdUDPSessionFromDetails implements BFD handler.
func (h *BfdVppHandler) AddBfdUDPSessionFromDetails(bfdSess *bfd_api.BfdUDPSessionDetails, bfdKeyIndexes idxvpp.NameToIdx) error {
	req := &bfd_api.BfdUDPAdd{
		SwIfIndex:     bfdSess.SwIfIndex,
		DesiredMinTx:  bfdSess.DesiredMinTx,
		RequiredMinRx: bfdSess.RequiredMinRx,
		LocalAddr:     bfdSess.LocalAddr,
		PeerAddr:      bfdSess.PeerAddr,
		DetectMult:    bfdSess.DetectMult,
		IsIPv6:        bfdSess.IsIPv6,
	}

	// Authentication
	if bfdSess.IsAuthenticated != 0 {
		keyID := string(bfdSess.BfdKeyID)
		h.log.Infof("Setting up authentication with index %v", keyID)
		_, _, found := bfdKeyIndexes.LookupIdx(keyID)
		if found {
			req.IsAuthenticated = 1
			req.BfdKeyID = bfdSess.BfdKeyID
			req.ConfKeyID = bfdSess.ConfKeyID
		} else {
			h.log.Infof("Authentication key %v not found", bfdSess.BfdKeyID)
			req.IsAuthenticated = 0
		}
	} else {
		req.IsAuthenticated = 0
	}
	reply := &bfd_api.BfdUDPAddReply{}

	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return err
	} else if reply.Retval != 0 {
		return fmt.Errorf("%s returned %d", reply.GetMessageName(), reply.Retval)
	}

	return nil
}

// ModifyBfdUDPSession implements BFD handler.
func (h *BfdVppHandler) ModifyBfdUDPSession(bfdSess *bfd.SingleHopBFD_Session, swIfIndexes ifaceidx.SwIfIndex) error {
	// Find the interface
	ifIdx, _, found := swIfIndexes.LookupIdx(bfdSess.Interface)
	if !found {
		return fmt.Errorf("interface %v does not exist", bfdSess.Interface)
	}

	req := &bfd_api.BfdUDPMod{
		SwIfIndex:     ifIdx,
		DesiredMinTx:  bfdSess.DesiredMinTxInterval,
		RequiredMinRx: bfdSess.RequiredMinRxInterval,
		DetectMult:    uint8(bfdSess.DetectMultiplier),
	}

	isLocalIpv6, err := addrs.IsIPv6(bfdSess.SourceAddress)
	if err != nil {
		return err
	}
	isPeerIpv6, err := addrs.IsIPv6(bfdSess.DestinationAddress)
	if err != nil {
		return err
	}
	if isLocalIpv6 && isPeerIpv6 {
		req.IsIPv6 = 1
		req.LocalAddr = net.ParseIP(bfdSess.SourceAddress).To16()
		req.PeerAddr = net.ParseIP(bfdSess.DestinationAddress).To16()
	} else if !isLocalIpv6 && !isPeerIpv6 {
		req.IsIPv6 = 0
		req.LocalAddr = net.ParseIP(bfdSess.SourceAddress).To4()
		req.PeerAddr = net.ParseIP(bfdSess.DestinationAddress).To4()
	} else {
		return fmt.Errorf("different IP versions or missing IP address. Local: %v, Peer: %v",
			bfdSess.SourceAddress, bfdSess.DestinationAddress)
	}
	reply := &bfd_api.BfdUDPModReply{}

	if err = h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return err
	} else if reply.Retval != 0 {
		return fmt.Errorf("%s returned %d", reply.GetMessageName(), reply.Retval)
	}

	return nil
}

// DeleteBfdUDPSession implements BFD handler.
func (h *BfdVppHandler) DeleteBfdUDPSession(ifIndex uint32, sourceAddress string, destAddress string) error {
	req := &bfd_api.BfdUDPDel{
		SwIfIndex: ifIndex,
		LocalAddr: net.ParseIP(sourceAddress).To4(),
		PeerAddr:  net.ParseIP(destAddress).To4(),
		IsIPv6:    0,
	}
	reply := &bfd_api.BfdUDPDelReply{}

	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return err
	} else if reply.Retval != 0 {
		return fmt.Errorf("%s returned %d", reply.GetMessageName(), reply.Retval)
	}

	return nil
}

// SetBfdUDPAuthenticationKey implements BFD handler.
func (h *BfdVppHandler) SetBfdUDPAuthenticationKey(bfdKey *bfd.SingleHopBFD_Key) error {
	// Convert authentication according to RFC5880.
	var authentication uint8
	if bfdKey.AuthenticationType == 0 {
		authentication = 4 // Keyed SHA1
	} else if bfdKey.AuthenticationType == 1 {
		authentication = 5 // Meticulous keyed SHA1
	} else {
		h.log.Warnf("Provided authentication type not supported, setting up SHA1")
		authentication = 4
	}

	req := &bfd_api.BfdAuthSetKey{
		ConfKeyID: bfdKey.Id,
		AuthType:  authentication,
		Key:       []byte(bfdKey.Secret),
		KeyLen:    uint8(len(bfdKey.Secret)),
	}
	reply := &bfd_api.BfdAuthSetKeyReply{}

	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return err
	} else if reply.Retval != 0 {
		return fmt.Errorf("%s returned %d", reply.GetMessageName(), reply.Retval)
	}

	return nil
}

// DeleteBfdUDPAuthenticationKey implements BFD handler.
func (h *BfdVppHandler) DeleteBfdUDPAuthenticationKey(bfdKey *bfd.SingleHopBFD_Key) error {
	req := &bfd_api.BfdAuthDelKey{
		ConfKeyID: bfdKey.Id,
	}
	reply := &bfd_api.BfdAuthDelKeyReply{}

	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return err
	} else if reply.Retval != 0 {
		return fmt.Errorf("%s returned %d", reply.GetMessageName(), reply.Retval)
	}

	return nil
}

// AddBfdEchoFunction implements BFD handler.
func (h *BfdVppHandler) AddBfdEchoFunction(bfdInput *bfd.SingleHopBFD_EchoFunction, swIfIndexes ifaceidx.SwIfIndex) error {
	// Verify the interface presence.
	ifIdx, _, found := swIfIndexes.LookupIdx(bfdInput.EchoSourceInterface)
	if !found {
		return fmt.Errorf("interface %v does not exist", bfdInput.EchoSourceInterface)
	}

	req := &bfd_api.BfdUDPSetEchoSource{
		SwIfIndex: ifIdx,
	}
	reply := &bfd_api.BfdUDPSetEchoSourceReply{}

	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return err
	} else if reply.Retval != 0 {
		return fmt.Errorf("%s returned %d", reply.GetMessageName(), reply.Retval)
	}

	return nil
}

// DeleteBfdEchoFunction implements BFD handler.
func (h *BfdVppHandler) DeleteBfdEchoFunction() error {
	// Prepare the message.
	req := &bfd_api.BfdUDPDelEchoSource{}
	reply := &bfd_api.BfdUDPDelEchoSourceReply{}

	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return err
	} else if reply.Retval != 0 {
		return fmt.Errorf("%s returned %d", reply.GetMessageName(), reply.Retval)
	}

	return nil
}
