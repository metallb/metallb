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
	"net"
	"reflect"
	"strings"

	"github.com/gogo/protobuf/proto"
	prototypes "github.com/gogo/protobuf/types"
	"github.com/ligato/cn-infra/idxmap"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/utils/addrs"
	"github.com/pkg/errors"

	linux_intf "github.com/ligato/vpp-agent/api/models/linux/interfaces"
	linux_ns "github.com/ligato/vpp-agent/api/models/linux/namespace"
	interfaces "github.com/ligato/vpp-agent/api/models/vpp/interfaces"
	kvs "github.com/ligato/vpp-agent/plugins/kvscheduler/api"
	linux_ifdescriptor "github.com/ligato/vpp-agent/plugins/linuxv2/ifplugin/descriptor"
	linux_ifaceidx "github.com/ligato/vpp-agent/plugins/linuxv2/ifplugin/ifaceidx"
	"github.com/ligato/vpp-agent/plugins/linuxv2/nsplugin"
	"github.com/ligato/vpp-agent/plugins/vppv2/ifplugin/descriptor/adapter"
	"github.com/ligato/vpp-agent/plugins/vppv2/ifplugin/ifaceidx"
	"github.com/ligato/vpp-agent/plugins/vppv2/ifplugin/vppcalls"
)

const (
	// InterfaceDescriptorName is the name of the descriptor for VPP interfaces.
	InterfaceDescriptorName = "vpp-interface"

	// dependency labels
	afPacketHostInterfaceDep = "afpacket-host-interface-exists"
	vxlanMulticastDep        = "vxlan-multicast-interface-exists"
	microserviceDep          = "microservice-available"
	parentInterfaceDep       = "parent-interface-exists"

	// how many characters a logical interface name is allowed to have
	//  - determined by much fits into the VPP interface tag (64 null-terminated character string)
	logicalNameLengthLimit = 63

	// prefix prepended to internal names of untagged interfaces to construct unique
	// logical names
	untaggedIfPreffix = "UNTAGGED-"

	// suffix attached to logical names of dumped TAP interfaces with Linux side
	// not found by Dump of Linux-ifplugin
	tapMissingLinuxSideSuffix = "-MISSING_LINUX_SIDE"

	// suffix attached to logical names of dumped AF-PACKET interfaces connected
	// to missing Linux interfaces
	afPacketMissingAttachedIfSuffix = "-MISSING_ATTACHED_INTERFACE"

	// default memif attributes
	defaultMemifNumOfQueues uint32 = 1
	defaultMemifBufferSize  uint32 = 2048
	defaultMemifRingSize    uint32 = 1024
)

// A list of non-retriable errors:
var (
	// ErrUnsupportedVPPInterfaceType is returned for VPP interfaces of unknown type.
	ErrUnsupportedVPPInterfaceType = errors.New("unsupported VPP interface type")

	// ErrInterfaceWithoutName is returned when VPP interface configuration has undefined
	// Name attribute.
	ErrInterfaceWithoutName = errors.New("VPP interface defined without logical name")

	// ErrInterfaceNameTooLong is returned when VPP interface logical name exceeds the length limit.
	ErrInterfaceNameTooLong = errors.New("VPP interface logical name exceeds the length limit (63 characters)")

	// ErrInterfaceWithoutType is returned when VPP interface configuration has undefined
	// Type attribute.
	ErrInterfaceWithoutType = errors.New("VPP interface defined without type")

	// ErrUnnumberedWithIP is returned when configuration of a VPP unnumbered interface
	// includes an IP address.
	ErrUnnumberedWithIP = errors.New("VPP unnumbered interface was defined with IP address")

	// ErrAfPacketWithoutHostName is returned when AF-Packet configuration is missing host interface name.
	ErrAfPacketWithoutHostName = errors.New("VPP AF-Packet interface was defined without host interface name")

	// ErrInterfaceLinkMismatch is returned when interface type does not match the link configuration.
	ErrInterfaceLinkMismatch = errors.New("VPP interface type and link configuration do not match")

	// ErrUnsupportedRxMode is returned when the given interface type does not support the chosen
	// RX mode.
	ErrUnsupportedRxMode = errors.New("unsupported RX Mode")

	// ErrSubInterfaceWithoutParent is returned when interface of type sub-interface is defined without parent.
	ErrSubInterfaceWithoutParent = errors.Errorf("subinterface with no parent interface defined")
)

// InterfaceDescriptor teaches KVScheduler how to configure VPP interfaces.
type InterfaceDescriptor struct {
	// config
	defaultMtu uint32

	// dependencies
	log       logging.Logger
	ifHandler vppcalls.IfVppAPI

	// optional dependencies, provide if AFPacket and/or TAP+TAP_TO_VPP interfaces are used
	linuxIfPlugin  LinuxPluginAPI
	linuxIfHandler NetlinkAPI
	nsPlugin       nsplugin.API

	// runtime
	intfIndex              ifaceidx.IfaceMetadataIndex
	memifSocketToID        map[string]uint32 // memif socket filename to ID map (all known sockets)
	defaultMemifSocketPath string
	ethernetIfs            map[string]uint32 // name-to-index map of ethernet interfaces
	// (entry is not removed even if interface is un-configured)
}

// LinuxPluginAPI is defined here to avoid import cycles.
type LinuxPluginAPI interface {
	// GetInterfaceIndex gives read-only access to map with metadata of all configured
	// linux interfaces.
	GetInterfaceIndex() linux_ifaceidx.LinuxIfMetadataIndex
}

// NetlinkAPI here lists only those Netlink methods that are actually used by InterfaceDescriptor.
type NetlinkAPI interface {
	// InterfaceExists verifies interface existence
	InterfaceExists(ifName string) (bool, error)
}

// NewInterfaceDescriptor creates a new instance of the Interface descriptor.
func NewInterfaceDescriptor(ifHandler vppcalls.IfVppAPI, defaultMtu uint32,
	linuxIfHandler NetlinkAPI, linuxIfPlugin LinuxPluginAPI, nsPlugin nsplugin.API, log logging.PluginLogger) *InterfaceDescriptor {

	return &InterfaceDescriptor{
		ifHandler:       ifHandler,
		defaultMtu:      defaultMtu,
		linuxIfPlugin:   linuxIfPlugin,
		linuxIfHandler:  linuxIfHandler,
		nsPlugin:        nsPlugin,
		log:             log.NewLogger("if-descriptor"),
		memifSocketToID: make(map[string]uint32),
		ethernetIfs:     make(map[string]uint32),
	}
}

// GetDescriptor returns descriptor suitable for registration (via adapter) with
// the KVScheduler.
func (d *InterfaceDescriptor) GetDescriptor() *adapter.InterfaceDescriptor {
	return &adapter.InterfaceDescriptor{
		Name:               InterfaceDescriptorName,
		NBKeyPrefix:        interfaces.ModelInterface.KeyPrefix(),
		ValueTypeName:      interfaces.ModelInterface.ProtoName(),
		KeySelector:        interfaces.ModelInterface.IsKeyValid,
		KeyLabel:           interfaces.ModelInterface.StripKeyPrefix,
		ValueComparator:    d.EquivalentInterfaces,
		WithMetadata:       true,
		MetadataMapFactory: d.MetadataFactory,
		Validate:           d.Validate,
		Add:                d.Add,
		Delete:             d.Delete,
		Modify:             d.Modify,
		ModifyWithRecreate: d.ModifyWithRecreate,
		Dependencies:       d.Dependencies,
		DerivedValues:      d.DerivedValues,
		Dump:               d.Dump,
		// If Linux-IfPlugin is loaded, dump it first.
		DumpDependencies: []string{linux_ifdescriptor.InterfaceDescriptorName},
	}
}

// SetInterfaceIndex should be used to provide interface index immediately after
// the descriptor registration.
func (d *InterfaceDescriptor) SetInterfaceIndex(intfIndex ifaceidx.IfaceMetadataIndex) {
	d.intfIndex = intfIndex
}

// EquivalentInterfaces is case-insensitive comparison function for
// interfaces.Interface, also ignoring the order of assigned IP addresses.
func (d *InterfaceDescriptor) EquivalentInterfaces(key string, oldIntf, newIntf *interfaces.Interface) bool {
	// attributes compared as usually:
	if oldIntf.Name != newIntf.Name ||
		oldIntf.Type != newIntf.Type ||
		oldIntf.Enabled != newIntf.Enabled ||
		oldIntf.Vrf != newIntf.Vrf ||
		oldIntf.SetDhcpClient != newIntf.SetDhcpClient {
		return false
	}
	if !proto.Equal(oldIntf.Unnumbered, newIntf.Unnumbered) ||
		!proto.Equal(getRxPlacement(oldIntf), getRxPlacement(newIntf)) {
		return false
	}

	// type-specific (defaults considered)
	if !d.equivalentTypeSpecificConfig(oldIntf, newIntf) {
		return false
	}

	// TODO: for TAPv2 the RxMode dump is unstable
	//       (it goes between POLLING and INTERRUPT, maybe it should actually return ADAPTIVE?)
	if oldIntf.Type != interfaces.Interface_TAP || oldIntf.GetTap().GetVersion() != 2 {
		if !proto.Equal(getRxMode(oldIntf), getRxMode(newIntf)) {
			return false
		}
	}

	// handle default/unspecified MTU (except VxLAN and IPSec tunnel)
	if newIntf.Type != interfaces.Interface_VXLAN_TUNNEL && newIntf.Type != interfaces.Interface_IPSEC_TUNNEL {
		if d.getInterfaceMTU(newIntf) != 0 && d.getInterfaceMTU(oldIntf) != d.getInterfaceMTU(newIntf) {
			return false
		}
	}

	// compare MAC addresses case-insensitively (also handle unspecified MAC address)
	if newIntf.PhysAddress != "" &&
		strings.ToLower(oldIntf.PhysAddress) != strings.ToLower(newIntf.PhysAddress) {
		return false
	}

	// order-irrelevant comparison of IP addresses
	oldIntfAddrs, err1 := addrs.StrAddrsToStruct(oldIntf.IpAddresses)
	newIntfAddrs, err2 := addrs.StrAddrsToStruct(newIntf.IpAddresses)
	if err1 != nil || err2 != nil {
		// one or both of the configurations are invalid, compare lazily
		return reflect.DeepEqual(oldIntf.IpAddresses, newIntf.IpAddresses)
	}
	obsoleteAddr, newAddr := addrs.DiffAddr(oldIntfAddrs, newIntfAddrs)
	if len(obsoleteAddr) != 0 || len(newAddr) != 0 {
		return false
	}

	return true
}

// equivalentTypeSpecificConfig compares type-specific sections of two interface configurations.
func (d *InterfaceDescriptor) equivalentTypeSpecificConfig(oldIntf, newIntf *interfaces.Interface) bool {
	switch oldIntf.Type {
	case interfaces.Interface_TAP:
		if !proto.Equal(getTapConfig(oldIntf), getTapConfig(newIntf)) {
			return false
		}
	case interfaces.Interface_VXLAN_TUNNEL:
		if !proto.Equal(oldIntf.GetVxlan(), newIntf.GetVxlan()) {
			return false
		}
	case interfaces.Interface_AF_PACKET:
		if oldIntf.GetAfpacket().GetHostIfName() != newIntf.GetAfpacket().GetHostIfName() {
			return false
		}
	case interfaces.Interface_MEMIF:
		if !d.equivalentMemifs(oldIntf.GetMemif(), newIntf.GetMemif()) {
			return false
		}
	case interfaces.Interface_IPSEC_TUNNEL:
		if !d.equivalentIPSecTunnels(oldIntf.GetIpsec(), newIntf.GetIpsec()) {
			return false
		}
	case interfaces.Interface_SUB_INTERFACE:
		if !proto.Equal(oldIntf.GetSub(), newIntf.GetSub()) {
			return false
		}
	case interfaces.Interface_VMXNET3_INTERFACE:
		if !d.equivalentVmxNet3(oldIntf.GetVmxNet3(), newIntf.GetVmxNet3()) {
			return false
		}
	}
	return true
}

// equivalentMemifs compares two memifs for equivalence.
func (d *InterfaceDescriptor) equivalentMemifs(oldMemif, newMemif *interfaces.MemifLink) bool {
	if oldMemif.GetMode() != newMemif.GetMode() ||
		oldMemif.GetMaster() != newMemif.GetMaster() ||
		oldMemif.GetId() != newMemif.GetId() ||
		oldMemif.GetSecret() != newMemif.GetSecret() {
		return false
	}
	// default values considered:
	if d.getMemifSocketFilename(oldMemif) != d.getMemifSocketFilename(newMemif) ||
		d.getMemifBufferSize(oldMemif) != d.getMemifBufferSize(newMemif) ||
		d.getMemifRingSize(oldMemif) != d.getMemifRingSize(newMemif) ||
		d.getMemifNumOfRxQueues(oldMemif) != d.getMemifNumOfRxQueues(newMemif) ||
		d.getMemifNumOfTxQueues(oldMemif) != d.getMemifNumOfTxQueues(newMemif) {
		return false
	}
	return true
}

// equivalentIPSecTunnels compares two IPSec tunnels for equivalence.
func (d *InterfaceDescriptor) equivalentIPSecTunnels(oldTun, newTun *interfaces.IPSecLink) bool {
	return oldTun.Esn == newTun.Esn &&
		oldTun.AntiReplay == newTun.AntiReplay &&
		oldTun.LocalSpi == newTun.LocalSpi &&
		oldTun.RemoteSpi == newTun.RemoteSpi &&
		oldTun.CryptoAlg == newTun.CryptoAlg &&
		oldTun.LocalCryptoKey == newTun.LocalCryptoKey &&
		oldTun.RemoteCryptoKey == newTun.RemoteCryptoKey &&
		oldTun.IntegAlg == newTun.IntegAlg &&
		oldTun.LocalIntegKey == newTun.LocalIntegKey &&
		oldTun.RemoteIntegKey == newTun.RemoteIntegKey &&
		oldTun.EnableUdpEncap == newTun.EnableUdpEncap
}

// equivalentVmxNets compares two vmxnet3 interfaces for equivalence.
func (d *InterfaceDescriptor) equivalentVmxNet3(oldVmxNet3, newVmxNet3 *interfaces.VmxNet3Link) bool {
	return oldVmxNet3.RxqSize == newVmxNet3.RxqSize &&
		oldVmxNet3.TxqSize == newVmxNet3.TxqSize
}

// MetadataFactory is a factory for index-map customized for VPP interfaces.
func (d *InterfaceDescriptor) MetadataFactory() idxmap.NamedMappingRW {
	return ifaceidx.NewIfaceIndex(d.log, "vpp-interface-index")
}

// Validate validates VPP interface configuration.
func (d *InterfaceDescriptor) Validate(key string, intf *interfaces.Interface) error {
	// validate name
	if name := intf.GetName(); name == "" {
		return kvs.NewInvalidValueError(ErrInterfaceWithoutName, "name")
	} else if len(name) > logicalNameLengthLimit {
		return kvs.NewInvalidValueError(ErrInterfaceNameTooLong, "name")
	}

	// validate link with type
	linkMismatchErr := kvs.NewInvalidValueError(ErrInterfaceLinkMismatch, "link")
	switch intf.Link.(type) {
	case *interfaces.Interface_Sub:
		if intf.Type != interfaces.Interface_SUB_INTERFACE {
			return linkMismatchErr
		}
	case *interfaces.Interface_Memif:
		if intf.Type != interfaces.Interface_MEMIF {
			return linkMismatchErr
		}
	case *interfaces.Interface_Afpacket:
		if intf.Type != interfaces.Interface_AF_PACKET {
			return linkMismatchErr
		}
	case *interfaces.Interface_Vxlan:
		if intf.Type != interfaces.Interface_VXLAN_TUNNEL {
			return linkMismatchErr
		}
	case *interfaces.Interface_Tap:
		if intf.Type != interfaces.Interface_TAP {
			return linkMismatchErr
		}
	}

	// validate type specific
	switch intf.GetType() {
	case interfaces.Interface_SUB_INTERFACE:
		if parentName := intf.GetSub().GetParentName(); parentName == "" {
			return kvs.NewInvalidValueError(ErrSubInterfaceWithoutParent, "link.sub.parent_name")
		}
	case interfaces.Interface_DPDK:
		if getRxMode(intf).GetRxMode() != interfaces.Interface_RxModeSettings_POLLING {
			return kvs.NewInvalidValueError(ErrUnsupportedRxMode, "rx_mode_settings.rx_mode")
		}
	case interfaces.Interface_AF_PACKET:
		if intf.GetAfpacket().GetHostIfName() == "" {
			return kvs.NewInvalidValueError(ErrAfPacketWithoutHostName, "link.afpacket.host_if_name")
		}
	case interfaces.Interface_UNDEFINED_TYPE:
		return kvs.NewInvalidValueError(ErrInterfaceWithoutType, "type")
	}

	// validate unnumbered
	if intf.GetUnnumbered() != nil {
		if len(intf.GetIpAddresses()) > 0 {
			return kvs.NewInvalidValueError(ErrUnnumberedWithIP, "unnumbered", "ip_addresses")
		}
	}

	return nil
}

// ModifyWithRecreate returns true if Type, VRF (or VRF IP version) or Type-specific
// attributes are different.
func (d *InterfaceDescriptor) ModifyWithRecreate(key string, oldIntf, newIntf *interfaces.Interface, metadata *ifaceidx.IfaceMetadata) bool {
	if oldIntf.Type != newIntf.Type ||
		oldIntf.Vrf != newIntf.Vrf {
		return true
	}

	// if type-specific attributes have changed, then re-create the interface
	if !d.equivalentTypeSpecificConfig(oldIntf, newIntf) {
		return true
	}

	// check if VRF IP version has changed
	newAddrs, err1 := addrs.StrAddrsToStruct(newIntf.IpAddresses)
	oldAddrs, err2 := addrs.StrAddrsToStruct(oldIntf.IpAddresses)
	if err1 != nil || err2 != nil {
		return false
	}
	wasIPv4, wasIPv6 := getIPAddressVersions(oldAddrs)
	isIPv4, isIPv6 := getIPAddressVersions(newAddrs)
	if wasIPv4 != isIPv4 || wasIPv6 != isIPv6 {
		return true
	}

	return false
}

// Dependencies lists dependencies for a VPP interface.
func (d *InterfaceDescriptor) Dependencies(key string, intf *interfaces.Interface) (dependencies []kvs.Dependency) {
	switch intf.Type {
	case interfaces.Interface_AF_PACKET:
		// AF-PACKET depends on a referenced Linux interface in the default namespace
		dependencies = append(dependencies, kvs.Dependency{
			Label: afPacketHostInterfaceDep,
			Key:   linux_intf.InterfaceHostNameKey(intf.GetAfpacket().GetHostIfName()),
		})
	case interfaces.Interface_TAP:
		// TAP connects VPP with microservice
		if toMicroservice := intf.GetTap().GetToMicroservice(); toMicroservice != "" {
			dependencies = append(dependencies, kvs.Dependency{
				Label: microserviceDep,
				Key:   linux_ns.MicroserviceKey(toMicroservice),
			})
		}
	case interfaces.Interface_VXLAN_TUNNEL:
		// VXLAN referencing an interface with Multicast IP address
		if vxlanMulticast := intf.GetVxlan().GetMulticast(); vxlanMulticast != "" {
			dependencies = append(dependencies, kvs.Dependency{
				Label: vxlanMulticastDep,
				AnyOf: func(key string) bool {
					ifName, ifaceAddr, _, isIfaceAddrKey := interfaces.ParseInterfaceAddressKey(key)
					return isIfaceAddrKey && ifName == vxlanMulticast && ifaceAddr.IsMulticast()
				},
			})
		}
	case interfaces.Interface_SUB_INTERFACE:
		// SUB_INTERFACE requires parent interface
		if parentName := intf.GetSub().GetParentName(); parentName != "" {
			dependencies = append(dependencies, kvs.Dependency{
				Label: parentInterfaceDep,
				Key:   interfaces.InterfaceKey(parentName),
			})
		}
	}

	return dependencies
}

// DerivedValues derives:
//  - key-value for unnumbered configuration sub-section
//  - empty value for enabled DHCP client
//  - one empty value for every IP address assigned to the interface.
func (d *InterfaceDescriptor) DerivedValues(key string, intf *interfaces.Interface) (derValues []kvs.KeyValuePair) {
	// unnumbered interface
	if intf.GetUnnumbered() != nil {
		derValues = append(derValues, kvs.KeyValuePair{
			Key:   interfaces.UnnumberedKey(intf.Name),
			Value: intf.GetUnnumbered(),
		})
	}

	// DHCP client
	if intf.SetDhcpClient {
		derValues = append(derValues, kvs.KeyValuePair{
			Key:   interfaces.DHCPClientKey(intf.Name),
			Value: &prototypes.Empty{},
		})
	}

	// IP addresses
	for _, ipAddr := range intf.IpAddresses {
		derValues = append(derValues, kvs.KeyValuePair{
			Key:   interfaces.InterfaceAddressKey(intf.Name, ipAddr),
			Value: &prototypes.Empty{},
		})
	}

	// TODO: define derived value for UP/DOWN state (needed for subinterfaces)

	return derValues
}

// getInterfaceMTU returns the interface MTU.
func (d *InterfaceDescriptor) getInterfaceMTU(intf *interfaces.Interface) uint32 {
	if mtu := intf.GetMtu(); mtu != 0 {
		return mtu
	}
	return d.defaultMtu /* still can be 0, i.e. undefined */
}

// resolveMemifSocketFilename returns memif socket filename ID.
// Registers it if does not exists yet.
func (d *InterfaceDescriptor) resolveMemifSocketFilename(memifIf *interfaces.MemifLink) (uint32, error) {
	socketFileName := d.getMemifSocketFilename(memifIf)
	registeredID, registered := d.memifSocketToID[socketFileName]
	if !registered {
		// Register new socket. ID is generated (default filename ID is 0, first is ID 1, second ID 2, etc)
		registeredID = uint32(len(d.memifSocketToID))
		err := d.ifHandler.RegisterMemifSocketFilename([]byte(socketFileName), registeredID)
		if err != nil {
			return 0, errors.Errorf("error registering socket file name %s (ID %d): %v", socketFileName, registeredID, err)
		}
		d.memifSocketToID[socketFileName] = registeredID
		d.log.Debugf("Memif socket filename %s registered under ID %d", socketFileName, registeredID)
	}
	return registeredID, nil
}

// getRxMode returns the RX mode of the given interface.
// If the mode is not defined, it returns the default settings for the given
// interface type.
func getRxMode(intf *interfaces.Interface) *interfaces.Interface_RxModeSettings {
	if rxModeSettings := intf.RxModeSettings; rxModeSettings != nil {
		return rxModeSettings
	}

	rxModeSettings := &interfaces.Interface_RxModeSettings{
		RxMode: interfaces.Interface_RxModeSettings_DEFAULT,
	}
	// return default mode for the given interface type
	switch intf.GetType() {
	case interfaces.Interface_DPDK:
		rxModeSettings.RxMode = interfaces.Interface_RxModeSettings_POLLING
	case interfaces.Interface_AF_PACKET:
		rxModeSettings.RxMode = interfaces.Interface_RxModeSettings_INTERRUPT
	case interfaces.Interface_TAP:
		if intf.GetTap().GetVersion() == 2 {
			// TAP v2
			rxModeSettings.RxMode = interfaces.Interface_RxModeSettings_INTERRUPT
		}
	}
	return rxModeSettings
}

// getRxPlacement returns the RX placement of the given interface.
func getRxPlacement(intf *interfaces.Interface) *interfaces.Interface_RxPlacementSettings {
	if rxPlacementSettings := intf.GetRxPlacementSettings(); rxPlacementSettings != nil {
		return rxPlacementSettings
	}
	return &interfaces.Interface_RxPlacementSettings{}
}

// getMemifSocketFilename returns the memif socket filename.
func (d *InterfaceDescriptor) getMemifSocketFilename(memif *interfaces.MemifLink) string {
	if socketFilename := memif.GetSocketFilename(); socketFilename != "" {
		return socketFilename
	}
	return d.defaultMemifSocketPath
}

// getMemifNumOfRxQueues returns the number of memif RX queues.
func (d *InterfaceDescriptor) getMemifNumOfRxQueues(memif *interfaces.MemifLink) uint32 {
	if memif.GetRxQueues() == 0 {
		return defaultMemifNumOfQueues
	}
	return memif.GetRxQueues()
}

// getMemifNumOfTxQueues returns the number of memif TX queues.
func (d *InterfaceDescriptor) getMemifNumOfTxQueues(memif *interfaces.MemifLink) uint32 {
	if memif.GetTxQueues() == 0 {
		return defaultMemifNumOfQueues
	}
	return memif.GetTxQueues()
}

// getMemifBufferSize returns the memif buffer size.
func (d *InterfaceDescriptor) getMemifBufferSize(memif *interfaces.MemifLink) uint32 {
	if memif.GetBufferSize() == 0 {
		return defaultMemifBufferSize
	}
	return memif.GetBufferSize()
}

// getMemifRingSize returns the memif ring size.
func (d *InterfaceDescriptor) getMemifRingSize(memif *interfaces.MemifLink) uint32 {
	if memif.GetRingSize() == 0 {
		return defaultMemifRingSize
	}
	return memif.GetRingSize()
}

// getTapConfig returns the TAP-specific configuration section (handling undefined attributes).
func getTapConfig(intf *interfaces.Interface) *interfaces.TapLink {
	tapCfg := &interfaces.TapLink{
		Version:        intf.GetTap().GetVersion(),
		HostIfName:     intf.GetTap().GetHostIfName(),
		ToMicroservice: intf.GetTap().GetToMicroservice(),
		RxRingSize:     intf.GetTap().GetRxRingSize(),
		TxRingSize:     intf.GetTap().GetTxRingSize(),
	}
	if tapCfg.Version == 0 {
		tapCfg.Version = 1
	}
	if tapCfg.HostIfName == "" {
		tapCfg.HostIfName = generateTAPHostName(intf.Name)
	}
	return tapCfg
}

// generateTAPHostName (deterministically) generates the host name for a TAP interface.
func generateTAPHostName(tapName string) string {
	if tapName == "" {
		return ""
	}
	return fmt.Sprintf("tap-%d", fnvHash(tapName))
}

// fnvHash hashes string using fnv32a algorithm.
func fnvHash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

// getIPAddressVersions returns two flags to tell whether the provided list of addresses
// contains IPv4 and/or IPv6 type addresses
func getIPAddressVersions(ipAddrs []*net.IPNet) (isIPv4, isIPv6 bool) {
	for _, ip := range ipAddrs {
		if ip.IP.To4() != nil {
			isIPv4 = true
		} else {
			isIPv6 = true
		}
	}
	return
}

// setInterfaceVrf sets the interface to VRF according to versions of IP addresses configured on the interface.
func setInterfaceVrf(ifHandler vppcalls.IfVppAPI, ifName string, ifIdx uint32, vrf uint32, ipAddresses []*net.IPNet) error {
	if vrf == 0 {
		// explicit set to VRF 0 seems to be causing issues on VPP
		// NOTE: because of this return, this function cannot be used to switch VRF from non-zero to zero
		// (can be only used to put a newly created interface to a VRF)
		return nil
	}
	isIPv4, isIPv6 := getIPAddressVersions(ipAddresses)
	if isIPv4 {
		if err := ifHandler.SetInterfaceVrf(ifIdx, vrf); err != nil {
			err = errors.Errorf("failed to set interface %s as IPv4 VRF %d: %v", ifName, vrf, err)
			return err
		}
	}
	if isIPv6 {
		if err := ifHandler.SetInterfaceVrfIPv6(ifIdx, vrf); err != nil {
			err = errors.Errorf("failed to set interface %s as IPv6 VRF %d: %v", ifName, vrf, err)
			return err
		}
	}
	return nil
}
