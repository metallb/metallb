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
	"strings"

	"github.com/pkg/errors"

	interfaces "github.com/ligato/vpp-agent/api/models/linux/interfaces"
	"github.com/ligato/vpp-agent/plugins/linuxv2/ifplugin/ifaceidx"

	nslinuxcalls "github.com/ligato/vpp-agent/plugins/linuxv2/nsplugin/linuxcalls"
)

// addTAPToVPP moves Linux-side of the VPP-TAP interface to the destination namespace
// and sets the requested host name, IP addresses, etc.
func (d *InterfaceDescriptor) addTAPToVPP(nsCtx nslinuxcalls.NamespaceMgmtCtx, key string,
	linuxIf *interfaces.Interface) (metadata *ifaceidx.LinuxIfMetadata, err error) {

	// determine TAP interface name as set by VPP ifplugin
	vppTapName := linuxIf.GetTap().GetVppTapIfName()
	vppTapMeta, found := d.vppIfPlugin.GetInterfaceIndex().LookupByName(vppTapName)
	if !found {
		err = errors.Errorf("failed to find VPP-side for the TAP-To-VPP interface %s", linuxIf.Name)
		d.log.Error(err)
		return nil, err
	}
	vppTapHostName := vppTapMeta.TAPHostIfName
	hostName := getHostIfName(linuxIf)

	// context
	agentPrefix := d.serviceLabel.GetAgentPrefix()

	// add alias to associate TAP with the logical name and VPP-TAP reference
	err = d.ifHandler.SetInterfaceAlias(vppTapHostName, agentPrefix+getTapAlias(linuxIf, vppTapHostName))
	if err != nil {
		d.log.Error(err)
		return nil, err
	}

	// move the TAP to the right namespace
	err = d.setInterfaceNamespace(nsCtx, vppTapHostName, linuxIf.Namespace)
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

	// rename from temporary host name to the request host name
	d.ifHandler.RenameInterface(vppTapHostName, hostName)
	if err != nil {
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
		VPPTapName:   vppTapName,
		Namespace:    linuxIf.Namespace,
		LinuxIfIndex: link.Attrs().Index,
	}

	return metadata, nil
}

// deleteAutoTAP returns TAP interface back to the default namespace and renames
// the interface back to original name.
func (d *InterfaceDescriptor) deleteAutoTAP(nsCtx nslinuxcalls.NamespaceMgmtCtx, key string, linuxIf *interfaces.Interface, metadata *ifaceidx.LinuxIfMetadata) error {
	hostName := getHostIfName(linuxIf)
	agentPrefix := d.serviceLabel.GetAgentPrefix()

	// get original TAP name
	link, err := d.ifHandler.GetLinkByName(hostName)
	if err != nil {
		d.log.Error(err)
		return err
	}
	alias := strings.TrimPrefix(link.Attrs().Alias, agentPrefix)
	_, _, origVppTapHostName := parseTapAlias(alias)
	if origVppTapHostName == "" {
		err = errors.New("failed to obtain the original TAP host name")
		d.log.Error(err)
		return err
	}

	// rename back to the temporary name
	d.ifHandler.RenameInterface(hostName, origVppTapHostName)
	if err != nil {
		d.log.Error(err)
		return err
	}

	// move TAP back to the default namespace
	err = d.setInterfaceNamespace(nsCtx, origVppTapHostName, nil)
	if err != nil {
		d.log.Error(err)
		return err
	}

	// move to the default namespace
	revert, err := d.nsPlugin.SwitchToNamespace(nsCtx, nil)
	if err != nil {
		d.log.Error(err)
		return err
	}
	defer revert()

	// remove interface alias at last(!)
	// - actually vishvananda/netlink does not support alias removal, so we just change
	//   it to a string which is not prefixed with agent label
	err = d.ifHandler.SetInterfaceAlias(origVppTapHostName, "unconfigured")
	if err != nil {
		d.log.Error(err)
		return err
	}

	return nil
}

// getTapAlias returns alias for Linux TAP interface managed by the agent.
// The alias stores the TAP_TO_VPP logical name together with VPP-TAP logical name
// and the host interface name as originally set by VPP side.
func getTapAlias(linuxIf *interfaces.Interface, origHostIfName string) string {
	return linuxIf.Name + "/" + linuxIf.GetTap().GetVppTapIfName() + "/" + origHostIfName
}

// parseTapAlias parses out TAP_TO_VPP logical name together with the name of the
// linked VPP-TAP and the original TAP host interface name.
func parseTapAlias(alias string) (linuxTapName, vppTapName, origHostIfName string) {
	aliasParts := strings.Split(alias, "/")
	linuxTapName = aliasParts[0]
	if len(aliasParts) > 1 {
		vppTapName = aliasParts[1]
	}
	if len(aliasParts) > 2 {
		origHostIfName = aliasParts[2]
	}
	return
}
