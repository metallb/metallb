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

package descriptor

import (
	"fmt"
	"hash/fnv"
	"strings"

	interfaces "github.com/ligato/vpp-agent/api/models/linux/interfaces"
	"github.com/ligato/vpp-agent/plugins/linuxv2/ifplugin/ifaceidx"
	nslinuxcalls "github.com/ligato/vpp-agent/plugins/linuxv2/nsplugin/linuxcalls"
)

// addVETH creates a new VETH pair if neither of VETH-ends are configured, or just
// applies configuration to the unfinished VETH-end with a temporary host name.
func (d *InterfaceDescriptor) addVETH(nsCtx nslinuxcalls.NamespaceMgmtCtx, key string,
	linuxIf *interfaces.Interface) (metadata *ifaceidx.LinuxIfMetadata, err error) {

	// determine host/logical/temporary interface names
	hostName := getHostIfName(linuxIf)
	peerName := linuxIf.GetVeth().GetPeerIfName()
	tempHostName := getVethTemporaryHostName(linuxIf.GetName())
	tempPeerHostName := getVethTemporaryHostName(peerName)

	// context
	ifIndex := d.scheduler.GetMetadataMap(InterfaceDescriptorName)
	agentPrefix := d.serviceLabel.GetAgentPrefix()

	// check if this VETH-end was already created by the other end
	_, peerExists := ifIndex.GetValue(peerName)
	if !peerExists {
		// delete obsolete/invalid unfinished VETH (ignore errors)
		d.ifHandler.DeleteInterface(tempHostName)
		d.ifHandler.DeleteInterface(tempPeerHostName)

		// create a new VETH pair
		err = d.ifHandler.AddVethInterfacePair(tempHostName, tempPeerHostName)
		if err != nil {
			d.log.Error(err)
			return nil, err
		}

		// add alias to both VETH ends
		err = d.ifHandler.SetInterfaceAlias(tempHostName, agentPrefix+getVethAlias(linuxIf.Name, peerName))
		if err != nil {
			d.log.Error(err)
			return nil, err
		}
		err = d.ifHandler.SetInterfaceAlias(tempPeerHostName, agentPrefix+getVethAlias(peerName, linuxIf.Name))
		if err != nil {
			d.log.Error(err)
			return nil, err
		}
	}

	// move the VETH-end to the right namespace
	err = d.setInterfaceNamespace(nsCtx, tempHostName, linuxIf.Namespace)
	if err != nil {
		d.log.Error(err)
		return nil, err
	}

	// move to the namespace with the interface
	revert, err := d.nsPlugin.SwitchToNamespace(nsCtx, linuxIf.Namespace)
	if err != nil {
		d.log.Error(err)
		return nil, err
	}
	defer revert()

	// rename from the temporary host name to the requested host name
	if err = d.ifHandler.RenameInterface(tempHostName, hostName); err != nil {
		d.log.Error(err)
		return nil, err
	}

	// build metadata
	link, err := d.ifHandler.GetLinkByName(hostName)
	if err != nil {
		d.log.Error(err)
		return nil, err
	}
	metadata = &ifaceidx.LinuxIfMetadata{
		Namespace:    linuxIf.Namespace,
		LinuxIfIndex: link.Attrs().Index,
	}

	return metadata, nil
}

// deleteVETH either un-configures one VETH-end if the other end is still configured, or
// removes the entire VETH pair.
func (d *InterfaceDescriptor) deleteVETH(nsCtx nslinuxcalls.NamespaceMgmtCtx, key string, linuxIf *interfaces.Interface, metadata *ifaceidx.LinuxIfMetadata) error {
	// determine host/logical/temporary interface names
	hostName := getHostIfName(linuxIf)
	peerName := linuxIf.GetVeth().GetPeerIfName()
	tempHostName := getVethTemporaryHostName(linuxIf.Name)
	tempPeerHostName := getVethTemporaryHostName(peerName)

	// check if the other end is still configured
	ifIndex := d.scheduler.GetMetadataMap(InterfaceDescriptorName)
	_, peerExists := ifIndex.GetValue(peerName)
	if peerExists {
		// just un-configure this VETH-end, but do not delete the pair

		// rename to the temporary host name
		err := d.ifHandler.RenameInterface(hostName, tempHostName)
		if err != nil {
			d.log.Error(err)
			return err
		}

		// move this VETH-end to the default namespace
		err = d.setInterfaceNamespace(nsCtx, tempHostName, nil)
		if err != nil {
			d.log.Error(err)
			return err
		}
	} else {
		// remove the VETH pair completely now
		err := d.ifHandler.DeleteInterface(hostName)
		if err != nil {
			d.log.Error(err)
			return err
		}
		if tempPeerHostName != "" {
			// peer should be automatically removed as well, but just in case...
			d.ifHandler.DeleteInterface(tempPeerHostName) // ignore errors
		}
	}

	return nil
}

// getVethAlias returns alias for Linux VETH interface managed by the agent.
// The alias stores the VETH logical name together with the peer (logical) name.
func getVethAlias(vethName, peerName string) string {
	return vethName + "/" + peerName
}

// parseVethAlias parses out VETH logical name together with the peer name from the alias.
func parseVethAlias(alias string) (vethName, peerName string) {
	aliasParts := strings.Split(alias, "/")
	vethName = aliasParts[0]
	if len(aliasParts) > 0 {
		peerName = aliasParts[1]
	}
	return
}

// getVethTemporaryHostName (deterministically) generates a temporary host name
// for a VETH interface.
func getVethTemporaryHostName(vethName string) string {
	if vethName == "" {
		return ""
	}
	return fmt.Sprintf("veth-%d", fnvHash(vethName))
}

// fnvHash hashes string using fnv32a algorithm.
func fnvHash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}
