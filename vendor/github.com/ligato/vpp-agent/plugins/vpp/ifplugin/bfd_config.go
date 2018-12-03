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

package ifplugin

import (
	"net"
	"strconv"
	"strings"

	govppapi "git.fd.io/govpp.git/api"
	"github.com/go-errors/errors"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/utils/safeclose"
	"github.com/ligato/vpp-agent/idxvpp"
	"github.com/ligato/vpp-agent/idxvpp/nametoidx"
	"github.com/ligato/vpp-agent/plugins/govppmux"
	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin/ifaceidx"
	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin/vppcalls"
	"github.com/ligato/vpp-agent/plugins/vpp/model/bfd"
)

// BFDConfigurator runs in the background in its own goroutine where it watches for any changes
// in the configuration of BFDs as modelled by the proto file "../model/bfd/bfd.proto"
// and stored in ETCD under the key "/vnf-agent/{agent-label}/vpp/config/v1/bfd/".
// Updates received from the northbound API are compared with the VPP run-time configuration and differences
// are applied through the VPP binary API.
type BFDConfigurator struct {
	log logging.Logger

	ifIndexes ifaceidx.SwIfIndex
	bfdIDSeq  uint32
	// Base mappings
	sessionsIndexes   idxvpp.NameToIdxRW
	keysIndexes       idxvpp.NameToIdxRW
	echoFunctionIndex idxvpp.NameToIdxRW

	vppChan govppapi.Channel

	// VPP API handler
	bfdHandler vppcalls.BfdVppAPI
}

// Init members and channels
func (c *BFDConfigurator) Init(logger logging.PluginLogger, goVppMux govppmux.API, swIfIndexes ifaceidx.SwIfIndex) (err error) {
	// Logger
	c.log = logger.NewLogger("bfd-conf")

	// Mappings
	c.ifIndexes = swIfIndexes
	c.sessionsIndexes = nametoidx.NewNameToIdx(c.log, "bfd_session_indexes", nil)
	c.keysIndexes = nametoidx.NewNameToIdx(c.log, "bfd_auth_keys_indexes", nil)
	c.echoFunctionIndex = nametoidx.NewNameToIdx(c.log, "bfd_echo_function_index", nil)

	// VPP channel
	if c.vppChan, err = goVppMux.NewAPIChannel(); err != nil {
		return errors.Errorf("failed to create API channel: %v", err)
	}

	// VPP API handler
	c.bfdHandler = vppcalls.NewBfdVppHandler(c.vppChan, c.ifIndexes, c.log)

	c.log.Infof(" BFD configurator initialized")

	return nil
}

// Close GOVPP channel
func (c *BFDConfigurator) Close() error {
	if err := safeclose.Close(c.vppChan); err != nil {
		return c.LogError(errors.Errorf("failed to safeclose BFD configurator: %v", err))
	}
	return nil
}

// clearMapping prepares all in-memory-mappings and other cache fields. All previous cached entries are removed.
func (c *BFDConfigurator) clearMapping() {
	c.sessionsIndexes.Clear()
	c.keysIndexes.Clear()
	c.echoFunctionIndex.Clear()

	c.log.Debugf("BFD configurator mapping cleared")
}

// GetBfdSessionIndexes gives access to BFD session indexes
func (c *BFDConfigurator) GetBfdSessionIndexes() idxvpp.NameToIdxRW {
	return c.sessionsIndexes
}

// GetBfdKeyIndexes gives access to BFD key indexes
func (c *BFDConfigurator) GetBfdKeyIndexes() idxvpp.NameToIdxRW {
	return c.keysIndexes
}

// GetBfdEchoFunctionIndexes gives access to BFD echo function indexes
func (c *BFDConfigurator) GetBfdEchoFunctionIndexes() idxvpp.NameToIdxRW {
	return c.echoFunctionIndex
}

// ConfigureBfdSession configures bfd session (including authentication if exists). Provided interface has to contain
// ip address defined in BFD as source
func (c *BFDConfigurator) ConfigureBfdSession(bfdInput *bfd.SingleHopBFD_Session) error {
	// Verify interface presence
	ifIdx, ifMeta, found := c.ifIndexes.LookupIdx(bfdInput.Interface)
	if !found {
		return errors.Errorf("interface %s does not exist", bfdInput.Interface)
	}

	// Check whether BFD contains source IP address
	if ifMeta == nil {
		return errors.Errorf("unable to get IP address data from interface %s", bfdInput.Interface)
	}
	var ipFound bool
	for _, ipAddr := range ifMeta.IpAddresses {
		// Remove suffix (BFD is not using it)
		ipWithMask := strings.Split(ipAddr, "/")
		if len(ipWithMask) == 0 {
			return errors.Errorf("incorrect interface %s IP address %s format", bfdInput.Interface, ipAddr)
		}
		ipAddrWithoutMask := ipWithMask[0] // the first index is IP address
		if ipAddrWithoutMask == bfdInput.SourceAddress {
			ipFound = true
			break
		}
	}
	if !ipFound {
		return errors.Errorf("interface %s does not contain IP address %s required for BFD session",
			bfdInput.Interface, bfdInput.SourceAddress)
	}

	// Call vpp api
	err := c.bfdHandler.AddBfdUDPSession(bfdInput, ifIdx, c.keysIndexes)
	if err != nil {
		return errors.Errorf("failed to configure BFD UDP session for interface %s: %v", bfdInput.Interface, err)
	}

	c.sessionsIndexes.RegisterName(bfdInput.Interface, c.bfdIDSeq, nil)
	c.log.Debugf("BFD session for interface %s registered", bfdInput.Interface)
	c.bfdIDSeq++

	c.log.Infof("BFD session for interface %s configured ", bfdInput.Interface)

	return nil
}

// ModifyBfdSession modifies BFD session fields. Source and destination IP address for old and new config has to be the
// same. Authentication is NOT changed here, BFD modify bin api call does not support that
func (c *BFDConfigurator) ModifyBfdSession(oldBfdInput *bfd.SingleHopBFD_Session, newBfdInput *bfd.SingleHopBFD_Session) error {
	// Verify interface presence
	ifIdx, ifMeta, found := c.ifIndexes.LookupIdx(newBfdInput.Interface)
	if !found {
		return errors.Errorf("interface %s does not exist", newBfdInput.Interface)
	}

	// Check whether BFD contains source IP address
	if ifMeta == nil {
		return errors.Errorf("unable to get IP address data from interface %v", newBfdInput.Interface)
	}
	var ipFound bool
	for _, ipAddr := range ifMeta.IpAddresses {
		// Remove suffix
		ipWithMask := strings.Split(ipAddr, "/")
		if len(ipWithMask) == 0 {
			return errors.Errorf("incorrect interface %s IP address %s format", newBfdInput.Interface, ipAddr)
		}
		ipAddrWithoutMask := ipWithMask[0] // the first index is IP address
		if ipAddrWithoutMask == newBfdInput.SourceAddress {
			ipFound = true
			break
		}
	}
	if !ipFound {
		return errors.Errorf("interface %s does not contain IP address %s required for modified BFD session",
			newBfdInput.Interface, newBfdInput.SourceAddress)
	}

	// Find old BFD session
	_, _, found = c.sessionsIndexes.LookupIdx(oldBfdInput.Interface)
	if !found {
		c.log.Warnf("Previous BFD session does not exist, creating a new one for interface %s", newBfdInput.Interface)
		err := c.bfdHandler.AddBfdUDPSession(newBfdInput, ifIdx, c.keysIndexes)
		if err != nil {
			return errors.Errorf("failed to re-add BFD UDP session for interface %s: %v", newBfdInput.Interface, err)
		}
		c.sessionsIndexes.RegisterName(newBfdInput.Interface, c.bfdIDSeq, nil)
		c.log.Debugf("BFD session for interface %s registered", newBfdInput.Interface)
		c.bfdIDSeq++
	} else {
		// Compare source and destination addresses which cannot change if BFD session is modified
		// todo new BFD input should be compared to BFD data on the vpp, not the last change (old BFD data)
		if oldBfdInput.SourceAddress != newBfdInput.SourceAddress || oldBfdInput.DestinationAddress != newBfdInput.DestinationAddress {
			return errors.Errorf("unable to modify BFD session, addresses do not match. Old session source: %s, dest: %s, new session source: %s, dest: %s",
				oldBfdInput.SourceAddress, oldBfdInput.DestinationAddress, newBfdInput.SourceAddress, newBfdInput.DestinationAddress)
		}
		err := c.bfdHandler.ModifyBfdUDPSession(newBfdInput, c.ifIndexes)
		if err != nil {
			return errors.Errorf("failed to modify BFD session for interface %s: %v", newBfdInput.Interface, err)
		}
	}

	c.log.Infof("Modified BFD session for interface %s", newBfdInput.Interface)

	return nil
}

// DeleteBfdSession removes BFD session
func (c *BFDConfigurator) DeleteBfdSession(bfdInput *bfd.SingleHopBFD_Session) error {
	ifIndex, _, found := c.ifIndexes.LookupIdx(bfdInput.Interface)
	if !found {
		return errors.Errorf("cannot remove BFD session, interface %s not found", bfdInput.Interface)
	}

	err := c.bfdHandler.DeleteBfdUDPSession(ifIndex, bfdInput.SourceAddress, bfdInput.DestinationAddress)
	if err != nil {
		return errors.Errorf("failed to remove BFD UDP session %s: %v", bfdInput.Interface, err)
	}

	c.sessionsIndexes.UnregisterName(bfdInput.Interface)
	c.log.Debugf("BFD session for interface %v unregistered", bfdInput.Interface)

	c.log.Info("BFD session for interface %s removed", bfdInput.Interface)

	return nil
}

// ConfigureBfdAuthKey crates new authentication key which can be used for BFD session
func (c *BFDConfigurator) ConfigureBfdAuthKey(bfdAuthKey *bfd.SingleHopBFD_Key) error {
	err := c.bfdHandler.SetBfdUDPAuthenticationKey(bfdAuthKey)
	if err != nil {
		return errors.Errorf("failed to set BFD authentication key with name %s (ID %d): %v",
			bfdAuthKey.Name, bfdAuthKey.Id, err)
	}

	authKeyIDAsString := AuthKeyIdentifier(bfdAuthKey.Id)
	c.keysIndexes.RegisterName(authKeyIDAsString, c.bfdIDSeq, nil)
	c.log.Debugf("BFD authentication key with name %s (ID %d) registered", bfdAuthKey.Name, bfdAuthKey.Id)
	c.bfdIDSeq++

	c.log.Infof("BFD authentication key with name %s (ID %d) configured", bfdAuthKey.Name, bfdAuthKey.Id)

	return nil
}

// ModifyBfdAuthKey modifies auth key fields. Key which is assigned to one or more BFD session cannot be modified
func (c *BFDConfigurator) ModifyBfdAuthKey(oldInput *bfd.SingleHopBFD_Key, newInput *bfd.SingleHopBFD_Key) error {
	// Check that this auth key is not used in any session
	sessionList, err := c.bfdHandler.DumpBfdUDPSessionsWithID(newInput.Id)
	if err != nil {
		return errors.Errorf("error while verifying authentication key %s (ID: %d) usage: %v",
			oldInput.Name, oldInput.Id, err)
	}
	if sessionList != nil && len(sessionList.Session) != 0 {
		// Authentication Key is used and cannot be removed directly
		for _, bfds := range sessionList.Session {
			sourceAddr := net.HardwareAddr(bfds.SourceAddress).String()
			destAddr := net.HardwareAddr(bfds.DestinationAddress).String()
			ifIdx, _, found := c.ifIndexes.LookupIdx(bfds.Interface)
			if !found {
				return errors.Errorf("Modify BFD auth key: interface index for %s not found in the mapping",
					bfds.Interface)
			}
			err := c.bfdHandler.DeleteBfdUDPSession(ifIdx, sourceAddr, destAddr)
			if err != nil {
				return errors.Errorf("failed to remove BFD UDP session %s (temporary removal): %v",
					bfds.Interface, err)
			}
		}
		c.log.Debugf("%d session(s) temporary removed while updating authentication keys", len(sessionList.Session))
	}

	err = c.bfdHandler.DeleteBfdUDPAuthenticationKey(oldInput)
	if err != nil {
		return errors.Errorf("error while removing BFD auth key with name %s (ID %d): %v",
			oldInput.Name, oldInput.Id, err)
	}
	err = c.bfdHandler.SetBfdUDPAuthenticationKey(newInput)
	if err != nil {
		return errors.Errorf("error while setting up BFD auth key with name %s (ID %d): %v",
			oldInput.Name, oldInput.Id, err)
	}

	// Recreate BFD sessions if necessary
	if sessionList != nil && len(sessionList.Session) != 0 {
		for _, bfdSession := range sessionList.Session {
			ifIdx, _, found := c.ifIndexes.LookupIdx(bfdSession.Interface)
			if !found {
				return errors.Errorf("Modify BFD auth key: interface index for %s not found in the mapping",
					bfdSession.Interface)
			}
			err := c.bfdHandler.AddBfdUDPSession(bfdSession, ifIdx, c.keysIndexes)
			if err != nil {
				return errors.Errorf("failed to re-add BFD UDP session for interface %s: %v",
					bfdSession.Interface, err)
			}
		}
		c.log.Debugf("%d session(s) recreated after authentication key update", len(sessionList.Session))
	}

	c.log.Infof("BFD authentication key with name %s (ID %d) modified", newInput.Name, newInput.Id)

	return nil
}

// DeleteBfdAuthKey removes BFD authentication key but only if it is not used in any BFD session
func (c *BFDConfigurator) DeleteBfdAuthKey(bfdInput *bfd.SingleHopBFD_Key) error {
	// Check that this auth key is not used in any session
	// TODO perhaps bfd session mapping can be used instead of dump
	sessionList, err := c.bfdHandler.DumpBfdUDPSessionsWithID(bfdInput.Id)
	if err != nil {
		return errors.Errorf("Delete BFD auth key %s (ID %d): failed to dump BFD sessions: %v",
			bfdInput.Name, bfdInput.Id, err)
	}

	if sessionList != nil && len(sessionList.Session) != 0 {
		// Authentication Key is used and cannot be removed directly
		for _, bfds := range sessionList.Session {
			ifIdx, _, found := c.ifIndexes.LookupIdx(bfds.Interface)
			if !found {
				return errors.Errorf("Delete BFD auth key: interface index %s not found in the mapping",
					bfds.Interface)
			}
			err := c.bfdHandler.DeleteBfdUDPSession(ifIdx, bfds.SourceAddress, bfds.DestinationAddress)
			if err != nil {
				return errors.Errorf("failed to remove BFD UDP session %s: %v", bfds.Interface, err)
			}
		}
		c.log.Debugf("%d session(s) temporary removed", len(sessionList.Session))
	}
	err = c.bfdHandler.DeleteBfdUDPAuthenticationKey(bfdInput)
	if err != nil {
		return errors.Errorf("error while removing BFD auth key %s (ID %d): %v", bfdInput.Name, bfdInput.Id, err)
	}
	authKeyIDAsString := AuthKeyIdentifier(bfdInput.Id)
	c.keysIndexes.UnregisterName(authKeyIDAsString)
	c.log.Debugf("BFD authentication key %s (ID %d) unregistered", bfdInput.Name, bfdInput.Id)
	// Recreate BFD sessions if necessary
	if sessionList != nil && len(sessionList.Session) != 0 {
		for _, bfdSession := range sessionList.Session {
			ifIdx, _, found := c.ifIndexes.LookupIdx(bfdSession.Interface)
			if !found {
				return errors.Errorf("Delete BFD auth key: interface index for %s not found", bfdSession.Interface)
			}
			err := c.bfdHandler.AddBfdUDPSession(bfdSession, ifIdx, c.keysIndexes)
			if err != nil {
				return errors.Errorf("failed to add BFD UDP session for interface %s: %v",
					bfdSession.Interface, err)
			}
		}
		c.log.Debugf("%d session(s) recreated", len(sessionList.Session))
	}
	return nil
}

// ConfigureBfdEchoFunction is used to setup BFD Echo function on existing interface
func (c *BFDConfigurator) ConfigureBfdEchoFunction(bfdInput *bfd.SingleHopBFD_EchoFunction) error {
	// Verify interface presence
	_, _, found := c.ifIndexes.LookupIdx(bfdInput.EchoSourceInterface)
	if !found {
		return errors.Errorf("BFD echo function add: interface %s does not exist", bfdInput.EchoSourceInterface)
	}

	err := c.bfdHandler.AddBfdEchoFunction(bfdInput, c.ifIndexes)
	if err != nil {
		return errors.Errorf("failed to set BFD echo source for interface %s: %v",
			bfdInput.EchoSourceInterface, err)
	}

	c.echoFunctionIndex.RegisterName(bfdInput.EchoSourceInterface, c.bfdIDSeq, nil)
	c.log.Debugf("BFD echo function for interface %s registered", bfdInput.EchoSourceInterface)
	c.bfdIDSeq++

	c.log.Infof("BFD echo source set for interface %s ", bfdInput.EchoSourceInterface)

	return nil
}

// ModifyBfdEchoFunction handles echo function changes
func (c *BFDConfigurator) ModifyBfdEchoFunction(oldInput *bfd.SingleHopBFD_EchoFunction, newInput *bfd.SingleHopBFD_EchoFunction) error {
	c.log.Warnf("There is nothing to modify for BFD echo function")
	// NO-OP

	/* todo: the reason is echo function uses interface name in key, so if interface is changed, the key changes (despite
	   there is 'name' field in the model which is currently unused). Maybe it would be better to use name in the key,
	   and change interface in modify as usually */

	return nil
}

// DeleteBfdEchoFunction removes BFD echo function
func (c *BFDConfigurator) DeleteBfdEchoFunction(bfdInput *bfd.SingleHopBFD_EchoFunction) error {
	err := c.bfdHandler.DeleteBfdEchoFunction()
	if err != nil {
		return errors.Errorf("error while removing BFD echo source for interface %s: %v",
			bfdInput.EchoSourceInterface, err)
	}

	c.echoFunctionIndex.UnregisterName(bfdInput.EchoSourceInterface)
	c.log.Debugf("BFD echo function for interface %s unregistered", bfdInput.EchoSourceInterface)

	c.log.Infof("Echo source unset (was set to %s)", bfdInput.EchoSourceInterface)

	return nil
}

// AuthKeyIdentifier generates common identifier for authentication key
func AuthKeyIdentifier(id uint32) string {
	return strconv.Itoa(int(id))
}

// LogError prints error if not nil, including stack trace. The same value is also returned, so it can be easily propagated further
func (c *BFDConfigurator) LogError(err error) error {
	if err == nil {
		return nil
	}
	switch err.(type) {
	case *errors.Error:
		c.log.WithField("logger", c.log).Errorf(string(err.Error() + "\n" + string(err.(*errors.Error).Stack())))
	default:
		c.log.Error(err)
	}
	return err
}
