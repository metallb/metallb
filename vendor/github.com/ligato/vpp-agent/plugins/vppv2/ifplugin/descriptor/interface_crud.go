package descriptor

import (
	"net"

	"github.com/gogo/protobuf/proto"
	"github.com/pkg/errors"

	"github.com/ligato/cn-infra/utils/addrs"

	interfaces "github.com/ligato/vpp-agent/api/models/vpp/interfaces"
	"github.com/ligato/vpp-agent/pkg/models"
	kvs "github.com/ligato/vpp-agent/plugins/kvscheduler/api"
	nslinuxcalls "github.com/ligato/vpp-agent/plugins/linuxv2/nsplugin/linuxcalls"
	"github.com/ligato/vpp-agent/plugins/vppv2/ifplugin/descriptor/adapter"
	"github.com/ligato/vpp-agent/plugins/vppv2/ifplugin/ifaceidx"
)

// Add creates a VPP interface.
func (d *InterfaceDescriptor) Add(key string, intf *interfaces.Interface) (metadata *ifaceidx.IfaceMetadata, err error) {
	var ifIdx uint32
	var tapHostIfName string

	// create the interface of the given type
	switch intf.Type {
	case interfaces.Interface_TAP:
		tapCfg := getTapConfig(intf)
		tapHostIfName = tapCfg.HostIfName
		ifIdx, err = d.ifHandler.AddTapInterface(intf.Name, tapCfg)
		if err != nil {
			d.log.Error(err)
			return nil, err
		}

		// TAP hardening: verify that the Linux side was created
		if d.linuxIfHandler != nil && d.nsPlugin != nil {
			// first, move to the default namespace and lock the thread
			nsCtx := nslinuxcalls.NewNamespaceMgmtCtx()
			revert, err := d.nsPlugin.SwitchToNamespace(nsCtx, nil)
			if err != nil {
				d.log.Error(err)
				return nil, err
			}
			exists, err := d.linuxIfHandler.InterfaceExists(tapHostIfName)
			revert()
			if err != nil {
				d.log.Error(err)
				return nil, err
			}
			if !exists {
				err = errors.Errorf("failed to create the Linux side (%s) of the TAP interface %s", tapHostIfName, intf.Name)
				d.log.Error(err)
				return nil, err
			}
		}

	case interfaces.Interface_MEMIF:
		var socketID uint32
		if socketID, err = d.resolveMemifSocketFilename(intf.GetMemif()); err != nil {
			d.log.Error(err)
			return nil, err
		}
		ifIdx, err = d.ifHandler.AddMemifInterface(intf.Name, intf.GetMemif(), socketID)
		if err != nil {
			d.log.Error(err)
			return nil, err
		}

	case interfaces.Interface_VXLAN_TUNNEL:
		var multicastIfIdx uint32
		multicastIf := intf.GetVxlan().GetMulticast()
		if multicastIf != "" {
			multicastMeta, found := d.intfIndex.LookupByName(multicastIf)
			if !found {
				err = errors.Errorf("failed to find multicast interface %s referenced by VXLAN %s",
					multicastIf, intf.Name)
				d.log.Error(err)
				return nil, err
			}
			multicastIfIdx = multicastMeta.SwIfIndex
		}
		ifIdx, err = d.ifHandler.AddVxLanTunnel(intf.Name, intf.GetVrf(), multicastIfIdx, intf.GetVxlan())
		if err != nil {
			d.log.Error(err)
			return nil, err
		}

	case interfaces.Interface_SOFTWARE_LOOPBACK:
		ifIdx, err = d.ifHandler.AddLoopbackInterface(intf.Name)
		if err != nil {
			d.log.Error(err)
			return nil, err
		}

	case interfaces.Interface_DPDK:
		var found bool
		ifIdx, found = d.ethernetIfs[intf.Name]
		if !found {
			err = errors.Errorf("failed to find physical interface %s", intf.Name)
			d.log.Error(err)
			return nil, err
		}

	case interfaces.Interface_AF_PACKET:
		ifIdx, err = d.ifHandler.AddAfPacketInterface(intf.Name, intf.GetPhysAddress(), intf.GetAfpacket())
		if err != nil {
			d.log.Error(err)
			return nil, err
		}
	case interfaces.Interface_IPSEC_TUNNEL:
		ifIdx, err = d.ifHandler.AddIPSecTunnelInterface(intf.Name, intf.GetIpsec())
		if err != nil {
			d.log.Error(err)
			return nil, err
		}
	case interfaces.Interface_SUB_INTERFACE:
		sub := intf.GetSub()
		parentMeta, found := d.intfIndex.LookupByName(sub.GetParentName())
		if !found {
			err = errors.Errorf("unable to find parent interface %s referenced by sub interface %s",
				sub.GetParentName(), intf.Name)
			d.log.Error(err)
			return nil, err
		}
		ifIdx, err = d.ifHandler.CreateSubif(parentMeta.SwIfIndex, sub.GetSubId())
		if err != nil {
			d.log.Error(err)
			return nil, err
		}
		err = d.ifHandler.SetInterfaceTag(intf.Name, ifIdx)
		if err != nil {
			d.log.Error(err)
			return nil, err
		}
	case interfaces.Interface_VMXNET3_INTERFACE:
		ifIdx, err = d.ifHandler.AddVmxNet3(intf.Name, intf.GetVmxNet3())
		if err != nil {
			d.log.Error(err)
			return nil, err
		}
	}

	/*
		Rx-mode

		Legend:
		P - polling
		I - interrupt
		A - adaptive

		Interfaces - supported modes:
			* tap interface - PIA
			* memory interface - PIA
			* vxlan tunnel - PIA
			* software loopback - PIA
			* ethernet csmad - P
			* af packet - PIA
	*/
	if intf.RxModeSettings != nil {
		rxMode := getRxMode(intf)
		err = d.ifHandler.SetRxMode(ifIdx, rxMode)
		if err != nil {
			err = errors.Errorf("failed to set Rx-mode for interface %s: %v", intf.Name, err)
			d.log.Error(err)
			return nil, err
		}
	}

	// rx-placement
	if intf.GetRxPlacementSettings() != nil {
		if err = d.ifHandler.SetRxPlacement(ifIdx, intf.GetRxPlacementSettings()); err != nil {
			err = errors.Errorf("failed to set rx-placement for interface %s: %v", intf.Name, err)
			d.log.Error(err)
			return nil, err
		}
	}

	// MAC address. Note: af-packet uses HwAddr from the host and physical interfaces cannot have the MAC address changed
	if intf.GetPhysAddress() != "" &&
		intf.GetType() != interfaces.Interface_AF_PACKET &&
		intf.GetType() != interfaces.Interface_DPDK {
		if err = d.ifHandler.SetInterfaceMac(ifIdx, intf.GetPhysAddress()); err != nil {
			err = errors.Errorf("failed to set MAC address %s to interface %s: %v",
				intf.GetPhysAddress(), intf.Name, err)
			d.log.Error(err)
			return nil, err
		}
	}

	// convert IP addresses to net.IPNet
	ipAddresses, err := addrs.StrAddrsToStruct(intf.IpAddresses)
	if err != nil {
		err = errors.Errorf("failed to convert %s IP address list to IPNet structures: %v", intf.Name, err)
		d.log.Error(err)
		return nil, err
	}

	// VRF (optional), should be done before IP addresses
	err = setInterfaceVrf(d.ifHandler, intf.Name, ifIdx, intf.Vrf, ipAddresses)
	if err != nil {
		d.log.Error(err)
		return nil, err
	}

	// configure IP addresses
	for _, address := range ipAddresses {
		if err := d.ifHandler.AddInterfaceIP(ifIdx, address); err != nil {
			err = errors.Errorf("adding IP address %v to interface %v failed: %v", address, intf.Name, err)
			d.log.Error(err)
			return nil, err
		}
	}

	// configure MTU. Prefer value in the interface config, otherwise set the plugin-wide
	// default value if provided.
	if ifaceSupportsSetMTU(intf) {
		mtuToConfigure := intf.Mtu
		if mtuToConfigure == 0 && d.defaultMtu != 0 {
			mtuToConfigure = d.defaultMtu
		}
		if mtuToConfigure != 0 {
			if err = d.ifHandler.SetInterfaceMtu(ifIdx, mtuToConfigure); err != nil {
				err = errors.Errorf("failed to set MTU %d to interface %s: %v", mtuToConfigure, intf.Name, err)
				d.log.Error(err)
				return nil, err
			}
		}
	}

	// set interface up if enabled
	if intf.Enabled {
		if err = d.ifHandler.InterfaceAdminUp(ifIdx); err != nil {
			err = errors.Errorf("failed to set interface %s up: %v", intf.Name, err)
			d.log.Error(err)
			return nil, err
		}
	}

	// fill the metadata
	metadata = &ifaceidx.IfaceMetadata{
		SwIfIndex:     ifIdx,
		Vrf:           intf.Vrf,
		IPAddresses:   intf.GetIpAddresses(),
		TAPHostIfName: tapHostIfName,
	}
	return metadata, nil
}

// Delete removes VPP interface.
func (d *InterfaceDescriptor) Delete(key string, intf *interfaces.Interface, metadata *ifaceidx.IfaceMetadata) error {
	ifIdx := metadata.SwIfIndex

	// set interface to ADMIN_DOWN unless the type is AF_PACKET_INTERFACE
	if intf.Type != interfaces.Interface_AF_PACKET {
		if err := d.ifHandler.InterfaceAdminDown(ifIdx); err != nil {
			err = errors.Errorf("failed to set interface %s down: %v", intf.Name, err)
			d.log.Error(err)
			return err
		}
	}

	// remove IP addresses from the interface
	var nonLocalIPs []string
	for _, ipAddr := range intf.IpAddresses {
		ip := net.ParseIP(ipAddr)
		if !ip.IsLinkLocalUnicast() {
			nonLocalIPs = append(nonLocalIPs, ipAddr)
		}
	}
	ipAddrs, err := addrs.StrAddrsToStruct(nonLocalIPs)
	if err != nil {
		err = errors.Errorf("failed to convert %s IP address list to IPNet structures: %v", intf.Name, err)
		d.log.Error(err)
		return err
	}
	for _, ipAddr := range ipAddrs {
		if err = d.ifHandler.DelInterfaceIP(ifIdx, ipAddr); err != nil {
			err = errors.Errorf("failed to remove IP address %s from interface %s: %v",
				ipAddr, intf.Name, err)
			d.log.Error(err)
			return err
		}
	}

	// remove the interface
	switch intf.Type {
	case interfaces.Interface_TAP:
		err = d.ifHandler.DeleteTapInterface(intf.Name, ifIdx, intf.GetTap().GetVersion())
	case interfaces.Interface_MEMIF:
		err = d.ifHandler.DeleteMemifInterface(intf.Name, ifIdx)
	case interfaces.Interface_VXLAN_TUNNEL:
		err = d.ifHandler.DeleteVxLanTunnel(intf.Name, ifIdx, intf.Vrf, intf.GetVxlan())
	case interfaces.Interface_SOFTWARE_LOOPBACK:
		err = d.ifHandler.DeleteLoopbackInterface(intf.Name, ifIdx)
	case interfaces.Interface_DPDK:
		d.log.Debugf("Interface %s removal skipped: cannot remove (blacklist) physical interface", intf.Name) // Not an error
		return nil
	case interfaces.Interface_AF_PACKET:
		err = d.ifHandler.DeleteAfPacketInterface(intf.Name, ifIdx, intf.GetAfpacket())
	case interfaces.Interface_IPSEC_TUNNEL:
		err = d.ifHandler.DeleteIPSecTunnelInterface(intf.Name, intf.GetIpsec())
	case interfaces.Interface_SUB_INTERFACE:
		err = d.ifHandler.DeleteSubif(ifIdx)
	case interfaces.Interface_VMXNET3_INTERFACE:
		err = d.ifHandler.DeleteVmxNet3(intf.Name, ifIdx)
	}
	if err != nil {
		err = errors.Errorf("failed to remove interface %s, index %d: %v", intf.Name, ifIdx, err)
		d.log.Error(err)
		return err
	}

	return nil
}

// Modify is able to change Type-unspecific attributes.
func (d *InterfaceDescriptor) Modify(key string, oldIntf, newIntf *interfaces.Interface, oldMetadata *ifaceidx.IfaceMetadata) (newMetadata *ifaceidx.IfaceMetadata, err error) {
	ifIdx := oldMetadata.SwIfIndex

	// rx-mode
	oldRx := getRxMode(oldIntf)
	newRx := getRxMode(newIntf)
	if !proto.Equal(oldRx, newRx) {
		err = d.ifHandler.SetRxMode(ifIdx, newRx)
		if err != nil {
			err = errors.Errorf("failed to modify rx-mode for interface %s: %v", newIntf.Name, err)
			d.log.Error(err)
			return oldMetadata, err
		}
	}

	// rx-placement
	if !proto.Equal(getRxPlacement(oldIntf), getRxPlacement(newIntf)) {
		if err = d.ifHandler.SetRxPlacement(ifIdx, newIntf.GetRxPlacementSettings()); err != nil {
			err = errors.Errorf("failed to modify rx-placement for interface %s: %v", newIntf.Name, err)
			d.log.Error(err)
			return oldMetadata, err
		}
	}

	// admin status
	if newIntf.Enabled != oldIntf.Enabled {
		if newIntf.Enabled {
			if err = d.ifHandler.InterfaceAdminUp(ifIdx); err != nil {
				err = errors.Errorf("failed to set interface %s up: %v", newIntf.Name, err)
				d.log.Error(err)
				return oldMetadata, err
			}
		} else {
			if err = d.ifHandler.InterfaceAdminDown(ifIdx); err != nil {
				err = errors.Errorf("failed to set interface %s down: %v", newIntf.Name, err)
				d.log.Error(err)
				return oldMetadata, err
			}
		}
	}

	// configure new MAC address if set (and only if it was changed and only for supported interface type)
	if newIntf.PhysAddress != "" &&
		newIntf.PhysAddress != oldIntf.PhysAddress &&
		oldIntf.Type != interfaces.Interface_AF_PACKET &&
		oldIntf.Type != interfaces.Interface_DPDK {
		if err := d.ifHandler.SetInterfaceMac(ifIdx, newIntf.PhysAddress); err != nil {
			err = errors.Errorf("setting interface %s MAC address %s failed: %v",
				newIntf.Name, newIntf.PhysAddress, err)
			d.log.Error(err)
			return oldMetadata, err
		}
	}

	// calculate diff of IP addresses
	newIPAddresses, err := addrs.StrAddrsToStruct(newIntf.IpAddresses)
	if err != nil {
		err = errors.Errorf("failed to convert %s IP address list to IPNet structures: %v", newIntf.Name, err)
		d.log.Error(err)
		return oldMetadata, err
	}
	oldIPAddresses, err := addrs.StrAddrsToStruct(oldIntf.IpAddresses)
	if err != nil {
		err = errors.Errorf("failed to convert %s IP address list to IPNet structures: %v", oldIntf.Name, err)
		d.log.Error(err)
		return oldMetadata, err
	}
	del, add := addrs.DiffAddr(newIPAddresses, oldIPAddresses)

	// delete obsolete IP addresses
	for _, address := range del {
		err := d.ifHandler.DelInterfaceIP(ifIdx, address)
		if nil != err {
			err = errors.Errorf("failed to remove obsolete IP address %v from interface %s: %v",
				address, newIntf.Name, err)
			d.log.Error(err)
			return oldMetadata, err
		}
	}

	// add new IP addresses
	for _, address := range add {
		err := d.ifHandler.AddInterfaceIP(ifIdx, address)
		if nil != err {
			err = errors.Errorf("failed to add new IP addresses %v to interface %s: %v",
				address, newIntf.Name, err)
			d.log.Error(err)
			return oldMetadata, err
		}
	}

	// update IP addresses in the metadata
	oldMetadata.IPAddresses = newIntf.IpAddresses

	// update MTU (except VxLan, IPSec)
	if ifaceSupportsSetMTU(newIntf) {
		if newIntf.Mtu != 0 && newIntf.Mtu != oldIntf.Mtu {
			if err := d.ifHandler.SetInterfaceMtu(ifIdx, newIntf.Mtu); err != nil {
				err = errors.Errorf("failed to set MTU to interface %s: %v", newIntf.Name, err)
				d.log.Error(err)
				return oldMetadata, err
			}
		} else if newIntf.Mtu == 0 && d.defaultMtu != 0 {
			if err := d.ifHandler.SetInterfaceMtu(ifIdx, d.defaultMtu); err != nil {
				err = errors.Errorf("failed to set MTU to interface %s: %v", newIntf.Name, err)
				d.log.Error(err)
				return oldMetadata, err
			}
		}
	}

	return oldMetadata, nil
}

// Dump returns all configured VPP interfaces.
func (d *InterfaceDescriptor) Dump(correlate []adapter.InterfaceKVWithMetadata) (dump []adapter.InterfaceKVWithMetadata, err error) {
	// make sure that any checks on the Linux side are done in the default namespace with locked thread
	if d.nsPlugin != nil {
		nsCtx := nslinuxcalls.NewNamespaceMgmtCtx()
		revert, err := d.nsPlugin.SwitchToNamespace(nsCtx, nil)
		if err == nil {
			defer revert()
		}
	}

	// convert interfaces for correlation into a map
	ifCfg := make(map[string]*interfaces.Interface) // interface logical name -> interface config (as expected by correlate)
	for _, kv := range correlate {
		ifCfg[kv.Value.Name] = kv.Value
	}

	// refresh the map of memif socket IDs
	d.memifSocketToID, err = d.ifHandler.DumpMemifSocketDetails()
	if err != nil {
		err = errors.Errorf("failed to dump memif socket details: %v", err)
		d.log.Error(err)
		return dump, err
	}
	for socketPath, socketID := range d.memifSocketToID {
		if socketID == 0 {
			d.defaultMemifSocketPath = socketPath
		}
	}

	// clear the map of ethernet interfaces
	d.ethernetIfs = make(map[string]uint32)

	// dump current state of VPP interfaces
	vppIfs, err := d.ifHandler.DumpInterfaces()
	if err != nil {
		err = errors.Errorf("failed to dump interfaces: %v", err)
		d.log.Error(err)
		return dump, err
	}

	for ifIdx, intf := range vppIfs {
		origin := kvs.FromNB
		if ifIdx == 0 {
			// local0 is created automatically
			origin = kvs.FromSB
		}
		if intf.Interface.Type == interfaces.Interface_DPDK {
			d.ethernetIfs[intf.Interface.Name] = ifIdx
			if !intf.Interface.Enabled && len(intf.Interface.IpAddresses) == 0 {
				// unconfigured physical interface => skip (but add entry to d.ethernetIfs)
				continue
			}
		}
		if intf.Interface.Name == "" {
			// untagged interface - generate a logical name for it
			// (apart from local0 it will get removed by resync)
			intf.Interface.Name = untaggedIfPreffix + intf.Meta.InternalName
		}

		// get TAP host interface name
		var tapHostIfName string
		if intf.Interface.Type == interfaces.Interface_TAP {
			tapHostIfName = intf.Interface.GetTap().GetHostIfName()
			if generateTAPHostName(intf.Interface.Name) == tapHostIfName {
				// interface host name was unset
				intf.Interface.GetTap().HostIfName = ""
			}
		}

		// correlate attributes that cannot be dumped
		if expCfg, hasExpCfg := ifCfg[intf.Interface.Name]; hasExpCfg {
			if expCfg.Type == interfaces.Interface_TAP && intf.Interface.GetTap() != nil {
				intf.Interface.GetTap().ToMicroservice = expCfg.GetTap().GetToMicroservice()
				intf.Interface.GetTap().RxRingSize = expCfg.GetTap().GetRxRingSize()
				intf.Interface.GetTap().TxRingSize = expCfg.GetTap().GetTxRingSize()
				// FIXME: VPP BUG - TAPv2 host name is sometimes not properly dumped
				// (seemingly uninitialized section of memory is returned)
				if intf.Interface.GetTap().GetVersion() == 2 {
					intf.Interface.GetTap().HostIfName = expCfg.GetTap().GetHostIfName()
				}

			}
			if expCfg.Type == interfaces.Interface_MEMIF && intf.Interface.GetMemif() != nil {
				intf.Interface.GetMemif().Secret = expCfg.GetMemif().GetSecret()
				intf.Interface.GetMemif().RxQueues = expCfg.GetMemif().GetRxQueues()
				intf.Interface.GetMemif().TxQueues = expCfg.GetMemif().GetTxQueues()
				// if memif is not connected yet, ring-size and buffer-size are
				// 1 and 0, respectively
				if intf.Interface.GetMemif().GetRingSize() == 1 {
					intf.Interface.GetMemif().RingSize = expCfg.GetMemif().GetRingSize()
				}
				if intf.Interface.GetMemif().GetBufferSize() == 0 {
					intf.Interface.GetMemif().BufferSize = expCfg.GetMemif().GetBufferSize()
				}
			}
		}

		// verify links between VPP and Linux side
		if d.linuxIfPlugin != nil && d.linuxIfHandler != nil && d.nsPlugin != nil {
			if intf.Interface.Type == interfaces.Interface_AF_PACKET {
				hostIfName := intf.Interface.GetAfpacket().HostIfName
				exists, _ := d.linuxIfHandler.InterfaceExists(hostIfName)
				if !exists {
					// the Linux interface that the AF-Packet is attached to does not exist
					// - append special suffix that will make this interface unwanted
					intf.Interface.Name += afPacketMissingAttachedIfSuffix
				}
			}
			if intf.Interface.Type == interfaces.Interface_TAP {
				exists, _ := d.linuxIfHandler.InterfaceExists(tapHostIfName)
				if !exists {
					// check if it was "stolen" by the Linux plugin
					_, _, exists = d.linuxIfPlugin.GetInterfaceIndex().LookupByVPPTap(
						intf.Interface.Name)
				}
				if !exists {
					// the Linux side of the TAP interface side was not found
					// - append special suffix that will make this interface unwanted
					intf.Interface.Name += tapMissingLinuxSideSuffix
				}
			}
		}

		// add interface record into the dump
		metadata := &ifaceidx.IfaceMetadata{
			SwIfIndex:     ifIdx,
			IPAddresses:   intf.Interface.IpAddresses,
			TAPHostIfName: tapHostIfName,
		}
		dump = append(dump, adapter.InterfaceKVWithMetadata{
			Key:      models.Key(intf.Interface),
			Value:    intf.Interface,
			Metadata: metadata,
			Origin:   origin,
		})

	}

	return dump, nil
}

func ifaceSupportsSetMTU(intf *interfaces.Interface) bool {
	switch intf.Type {
	case interfaces.Interface_VXLAN_TUNNEL,
		interfaces.Interface_IPSEC_TUNNEL,
		interfaces.Interface_SUB_INTERFACE:
		// MTU not supported
		return false
	}
	return true
}
