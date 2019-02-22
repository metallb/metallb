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
	"net"
	"reflect"
	"strconv"
	"strings"

	"github.com/gogo/protobuf/proto"
	prototypes "github.com/gogo/protobuf/types"
	"github.com/ligato/vpp-agent/pkg/models"
	"github.com/pkg/errors"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ligato/cn-infra/idxmap"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/logging/logrus"
	"github.com/ligato/cn-infra/servicelabel"
	"github.com/ligato/cn-infra/utils/addrs"
	kvs "github.com/ligato/vpp-agent/plugins/kvscheduler/api"

	interfaces "github.com/ligato/vpp-agent/api/models/linux/interfaces"
	namespace "github.com/ligato/vpp-agent/api/models/linux/namespace"
	vpp_intf "github.com/ligato/vpp-agent/api/models/vpp/interfaces"
	"github.com/ligato/vpp-agent/plugins/linuxv2/ifplugin/descriptor/adapter"
	"github.com/ligato/vpp-agent/plugins/linuxv2/ifplugin/ifaceidx"
	iflinuxcalls "github.com/ligato/vpp-agent/plugins/linuxv2/ifplugin/linuxcalls"
	"github.com/ligato/vpp-agent/plugins/linuxv2/nsplugin"
	nsdescriptor "github.com/ligato/vpp-agent/plugins/linuxv2/nsplugin/descriptor"
	nslinuxcalls "github.com/ligato/vpp-agent/plugins/linuxv2/nsplugin/linuxcalls"
	vpp_ifaceidx "github.com/ligato/vpp-agent/plugins/vppv2/ifplugin/ifaceidx"
)

const (
	// InterfaceDescriptorName is the name of the descriptor for Linux interfaces.
	InterfaceDescriptorName = "linux-interface"

	// defaultEthernetMTU - expected when MTU is not specified in the config.
	defaultEthernetMTU = 1500

	// dependency labels
	tapInterfaceDep = "vpp-tap-interface-exists"
	vethPeerDep     = "veth-peer-exists"
	microserviceDep = "microservice-available"

	// suffix attached to logical names of duplicate VETH interfaces
	vethDuplicateSuffix = "-DUPLICATE"

	// suffix attached to logical names of VETH interfaces with peers not found by Dump
	vethMissingPeerSuffix = "-MISSING_PEER"

	// minimum number of namespaces to be given to a single Go routine for processing
	// in the Dump operation
	minWorkForGoRoutine = 3
)

// A list of non-retriable errors:
var (
	// ErrUnsupportedLinuxInterfaceType is returned for Linux interfaces of unknown type.
	ErrUnsupportedLinuxInterfaceType = errors.New("unsupported Linux interface type")

	// ErrInterfaceWithoutName is returned when Linux interface configuration has undefined
	// Name attribute.
	ErrInterfaceWithoutName = errors.New("Linux interface defined without logical name")

	// ErrInterfaceWithoutType is returned when Linux interface configuration has undefined
	// Type attribute.
	ErrInterfaceWithoutType = errors.New("Linux interface defined without type")

	// ErrNamespaceWithoutReference is returned when namespace is missing reference.
	ErrInterfaceReferenceMismatch = errors.New("Linux interface reference does not match the interface type")

	// ErrVETHWithoutPeer is returned when VETH interface is missing peer interface
	// reference.
	ErrVETHWithoutPeer = errors.New("VETH interface defined without peer reference")

	// ErrTAPWithoutVPPReference is returned when TAP_TO_VPP interface is missing reference to VPP TAP.
	ErrTAPWithoutVPPReference = errors.New("TAP_TO_VPP interface defined without reference to VPP TAP")

	// ErrTAPRequiresVPPIfPlugin is returned when TAP_TO_VPP is supposed to be configured but VPP ifplugin
	// is not loaded.
	ErrTAPRequiresVPPIfPlugin = errors.New("TAP_TO_VPP interface requires VPP interface plugin to be loaded")

	// ErrNamespaceWithoutReference is returned when namespace is missing reference.
	ErrNamespaceWithoutReference = errors.New("namespace defined without name")
)

// InterfaceDescriptor teaches KVScheduler how to configure Linux interfaces.
type InterfaceDescriptor struct {
	log          logging.Logger
	serviceLabel servicelabel.ReaderAPI
	ifHandler    iflinuxcalls.NetlinkAPI
	nsPlugin     nsplugin.API
	vppIfPlugin  VPPIfPluginAPI
	scheduler    kvs.KVScheduler

	// parallelization of the Dump operation
	dumpGoRoutinesCnt int
}

// VPPIfPluginAPI is defined here to avoid import cycles.
type VPPIfPluginAPI interface {
	// GetInterfaceIndex gives read-only access to map with metadata of all configured
	// VPP interfaces.
	GetInterfaceIndex() vpp_ifaceidx.IfaceMetadataIndex
}

// NewInterfaceDescriptor creates a new instance of the Interface descriptor.
func NewInterfaceDescriptor(
	scheduler kvs.KVScheduler, serviceLabel servicelabel.ReaderAPI, nsPlugin nsplugin.API,
	vppIfPlugin VPPIfPluginAPI, ifHandler iflinuxcalls.NetlinkAPI, log logging.PluginLogger, dumpGoRoutinesCnt int) *InterfaceDescriptor {

	return &InterfaceDescriptor{
		scheduler:         scheduler,
		ifHandler:         ifHandler,
		nsPlugin:          nsPlugin,
		vppIfPlugin:       vppIfPlugin,
		serviceLabel:      serviceLabel,
		dumpGoRoutinesCnt: dumpGoRoutinesCnt,
		log:               log.NewLogger("if-descriptor"),
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
		DumpDependencies:   []string{nsdescriptor.MicroserviceDescriptorName},
	}
}

// EquivalentInterfaces is case-insensitive comparison function for
// interfaces.LinuxInterface, also ignoring the order of assigned IP addresses.
func (d *InterfaceDescriptor) EquivalentInterfaces(key string, oldIntf, newIntf *interfaces.Interface) bool {
	// attributes compared as usually:
	if oldIntf.Name != newIntf.Name ||
		oldIntf.Type != newIntf.Type ||
		oldIntf.Enabled != newIntf.Enabled ||
		getHostIfName(oldIntf) != getHostIfName(newIntf) {
		return false
	}
	if oldIntf.Type == interfaces.Interface_VETH {
		if oldIntf.GetVeth().GetPeerIfName() != newIntf.GetVeth().GetPeerIfName() {
			return false
		}
		// handle default config for checksum offloading
		if getRxChksmOffloading(oldIntf) != getRxChksmOffloading(newIntf) ||
			getTxChksmOffloading(oldIntf) != getTxChksmOffloading(newIntf) {
			return false
		}
	}
	if oldIntf.Type == interfaces.Interface_TAP_TO_VPP &&
		oldIntf.GetTap().GetVppTapIfName() != newIntf.GetTap().GetVppTapIfName() {
		return false
	}
	if !proto.Equal(oldIntf.Namespace, newIntf.Namespace) {
		return false
	}

	// handle default MTU
	if getInterfaceMTU(oldIntf) != getInterfaceMTU(newIntf) {
		return false
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
	obsolete, new := addrs.DiffAddr(oldIntfAddrs, newIntfAddrs)
	if len(obsolete) != 0 || len(new) != 0 {
		return false
	}

	return true
}

// MetadataFactory is a factory for index-map customized for Linux interfaces.
func (d *InterfaceDescriptor) MetadataFactory() idxmap.NamedMappingRW {
	return ifaceidx.NewLinuxIfIndex(logrus.DefaultLogger(), "linux-interface-index")
}

// Validate validates Linux interface configuration.
func (d *InterfaceDescriptor) Validate(key string, linuxIf *interfaces.Interface) error {
	if linuxIf.GetName() == "" {
		return kvs.NewInvalidValueError(ErrInterfaceWithoutName, "name")
	}
	if linuxIf.GetType() == interfaces.Interface_UNDEFINED {
		return kvs.NewInvalidValueError(ErrInterfaceWithoutType, "type")
	}
	if linuxIf.GetType() == interfaces.Interface_TAP_TO_VPP && d.vppIfPlugin == nil {
		return ErrTAPRequiresVPPIfPlugin
	}
	if linuxIf.GetNamespace() != nil &&
		(linuxIf.GetNamespace().GetType() == namespace.NetNamespace_UNDEFINED ||
			linuxIf.GetNamespace().GetReference() == "") {
		return kvs.NewInvalidValueError(ErrNamespaceWithoutReference, "namespace")
	}
	switch linuxIf.Link.(type) {
	case *interfaces.Interface_Tap:
		if linuxIf.GetType() != interfaces.Interface_TAP_TO_VPP {
			return kvs.NewInvalidValueError(ErrInterfaceReferenceMismatch, "link")
		}
		if linuxIf.GetTap().GetVppTapIfName() == "" {
			return kvs.NewInvalidValueError(ErrTAPWithoutVPPReference, "vpp_tap_if_name")
		}
	case *interfaces.Interface_Veth:
		if linuxIf.GetType() != interfaces.Interface_VETH {
			return kvs.NewInvalidValueError(ErrInterfaceReferenceMismatch, "link")
		}
		if linuxIf.GetVeth().GetPeerIfName() == "" {
			return kvs.NewInvalidValueError(ErrVETHWithoutPeer, "peer_if_name")
		}
	}
	return nil
}

// Add creates VETH or configures TAP interface.
func (d *InterfaceDescriptor) Add(key string, linuxIf *interfaces.Interface) (metadata *ifaceidx.LinuxIfMetadata, err error) {
	// move to the default namespace
	nsCtx := nslinuxcalls.NewNamespaceMgmtCtx()
	revert1, err := d.nsPlugin.SwitchToNamespace(nsCtx, nil)
	if err != nil {
		d.log.Error(err)
		return nil, err
	}
	defer revert1()

	// create interface based on its type
	switch linuxIf.Type {
	case interfaces.Interface_VETH:
		metadata, err = d.addVETH(nsCtx, key, linuxIf)
	case interfaces.Interface_TAP_TO_VPP:
		metadata, err = d.addTAPToVPP(nsCtx, key, linuxIf)
	default:
		return nil, ErrUnsupportedLinuxInterfaceType
	}

	if err != nil {
		return nil, err
	}

	// move to the namespace with the interface
	revert2, err := d.nsPlugin.SwitchToNamespace(nsCtx, linuxIf.Namespace)
	if err != nil {
		d.log.Error(err)
		return nil, err
	}
	defer revert2()

	// set interface up
	hostName := getHostIfName(linuxIf)
	if linuxIf.Enabled {
		err = d.ifHandler.SetInterfaceUp(hostName)
		if nil != err {
			err = errors.Errorf("failed to set linux interface %s up: %v", linuxIf.Name, err)
			d.log.Error(err)
			return nil, err
		}
	}

	// set interface MAC address
	if linuxIf.PhysAddress != "" {
		err = d.ifHandler.SetInterfaceMac(hostName, linuxIf.PhysAddress)
		if err != nil {
			err = errors.Errorf("failed to set MAC address %s to linux interface %s: %v",
				linuxIf.PhysAddress, linuxIf.Name, err)
			d.log.Error(err)
			return nil, err
		}
	}

	// set interface IP addresses
	ipAddresses, err := addrs.StrAddrsToStruct(linuxIf.IpAddresses)
	if err != nil {
		err = errors.Errorf("failed to convert IP addresses %v for interface %s: %v",
			linuxIf.IpAddresses, linuxIf.Name, err)
		d.log.Error(err)
		return nil, err
	}
	for _, ipAddress := range ipAddresses {
		err = d.ifHandler.AddInterfaceIP(hostName, ipAddress)
		if err != nil {
			err = errors.Errorf("failed to add IP address %v to linux interface %s: %v",
				ipAddress, linuxIf.Name, err)
			d.log.Error(err)
			return nil, err
		}
	}

	// set interface MTU
	if linuxIf.Mtu != 0 {
		mtu := int(linuxIf.Mtu)
		err = d.ifHandler.SetInterfaceMTU(hostName, mtu)
		if err != nil {
			err = errors.Errorf("failed to set MTU %d to linux interface %s: %v",
				mtu, linuxIf.Name, err)
			d.log.Error(err)
			return nil, err
		}
	}

	// set checksum offloading
	if linuxIf.Type == interfaces.Interface_VETH {
		rxOn := getRxChksmOffloading(linuxIf)
		txOn := getTxChksmOffloading(linuxIf)
		err = d.ifHandler.SetChecksumOffloading(hostName, rxOn, txOn)
		if err != nil {
			err = errors.Errorf("failed to configure checksum offloading (rx=%t,tx=%t) for linux interface %s: %v",
				rxOn, txOn, linuxIf.Name, err)
			d.log.Error(err)
			return nil, err
		}
	}

	return metadata, nil
}

// Delete removes VETH or unconfigures TAP interface.
func (d *InterfaceDescriptor) Delete(key string, linuxIf *interfaces.Interface, metadata *ifaceidx.LinuxIfMetadata) error {
	// move to the namespace with the interface
	nsCtx := nslinuxcalls.NewNamespaceMgmtCtx()
	revert, err := d.nsPlugin.SwitchToNamespace(nsCtx, linuxIf.Namespace)
	if err != nil {
		d.log.Error(err)
		return err
	}
	defer revert()

	// unassign IP addresses
	ipAddresses, err := addrs.StrAddrsToStruct(linuxIf.IpAddresses)
	if err != nil {
		err = errors.Errorf("failed to convert IP addresses %v for interface %s: %v",
			linuxIf.IpAddresses, linuxIf.Name, err)
		d.log.Error(err)
		return err
	}
	for _, ipAddress := range ipAddresses {
		err = d.ifHandler.DelInterfaceIP(getHostIfName(linuxIf), ipAddress)
		if err != nil {
			err = errors.Errorf("failed to remove IP address %v from linux interface %s: %v",
				ipAddress, linuxIf.Name, err)
			d.log.Error(err)
			return err
		}
	}

	switch linuxIf.Type {
	case interfaces.Interface_VETH:
		return d.deleteVETH(nsCtx, key, linuxIf, metadata)
	case interfaces.Interface_TAP_TO_VPP:
		return d.deleteAutoTAP(nsCtx, key, linuxIf, metadata)
	}

	err = ErrUnsupportedLinuxInterfaceType
	d.log.Error(err)
	return err
}

// Modify is able to change Type-unspecific attributes.
func (d *InterfaceDescriptor) Modify(key string, oldLinuxIf, newLinuxIf *interfaces.Interface, oldMetadata *ifaceidx.LinuxIfMetadata) (newMetadata *ifaceidx.LinuxIfMetadata, err error) {
	oldHostName := getHostIfName(oldLinuxIf)
	newHostName := getHostIfName(newLinuxIf)

	// move to the namespace with the interface
	nsCtx := nslinuxcalls.NewNamespaceMgmtCtx()
	revert, err := d.nsPlugin.SwitchToNamespace(nsCtx, oldLinuxIf.Namespace)
	if err != nil {
		d.log.Error(err)
		return nil, err
	}
	defer revert()

	// update host name
	if oldHostName != newHostName {
		d.ifHandler.RenameInterface(oldHostName, newHostName)
		if err != nil {
			d.log.Error(err)
			return nil, err
		}
	}

	// update admin status
	if oldLinuxIf.Enabled != newLinuxIf.Enabled {
		if newLinuxIf.Enabled {
			err = d.ifHandler.SetInterfaceUp(newHostName)
			if nil != err {
				err = errors.Errorf("failed to set linux interface %s UP: %v", newHostName, err)
				d.log.Error(err)
				return nil, err
			}
		} else {
			err = d.ifHandler.SetInterfaceDown(newHostName)
			if nil != err {
				err = errors.Errorf("failed to set linux interface %s DOWN: %v", newHostName, err)
				d.log.Error(err)
				return nil, err
			}
		}
	}

	// update MAC address
	if newLinuxIf.PhysAddress != "" && newLinuxIf.PhysAddress != oldLinuxIf.PhysAddress {
		err := d.ifHandler.SetInterfaceMac(newHostName, newLinuxIf.PhysAddress)
		if err != nil {
			err = errors.Errorf("failed to reconfigure MAC address for linux interface %s: %v",
				newLinuxIf.Name, err)
			d.log.Error(err)
			return nil, err
		}
	}

	// IP addresses
	newAddrs, err := addrs.StrAddrsToStruct(newLinuxIf.IpAddresses)
	if err != nil {
		err = errors.Errorf("linux interface modify: failed to convert IP addresses for %s: %v",
			newLinuxIf.Name, err)
		d.log.Error(err)
		return nil, err
	}
	oldAddrs, err := addrs.StrAddrsToStruct(oldLinuxIf.IpAddresses)
	if err != nil {
		err = errors.Errorf("linux interface modify: failed to convert IP addresses for %s: %v",
			newLinuxIf.Name, err)
		d.log.Error(err)
		return nil, err
	}
	del, add := addrs.DiffAddr(newAddrs, oldAddrs)

	for i := range del {
		err := d.ifHandler.DelInterfaceIP(newHostName, del[i])
		if nil != err {
			err = errors.Errorf("failed to remove IPv4 address from a Linux interface %s: %v",
				newLinuxIf.Name, err)
			d.log.Error(err)
			return nil, err
		}
	}

	for i := range add {
		err := d.ifHandler.AddInterfaceIP(newHostName, add[i])
		if nil != err {
			err = errors.Errorf("linux interface modify: failed to add IP addresses %s to %s: %v",
				add[i], newLinuxIf.Name, err)
			d.log.Error(err)
			return nil, err
		}
	}

	// MTU
	if getInterfaceMTU(newLinuxIf) != getInterfaceMTU(oldLinuxIf) {
		mtu := getInterfaceMTU(newLinuxIf)
		err := d.ifHandler.SetInterfaceMTU(newHostName, mtu)
		if nil != err {
			err = errors.Errorf("failed to reconfigure MTU for the linux interface %s: %v",
				newLinuxIf.Name, err)
			d.log.Error(err)
			return nil, err
		}
	}

	// update checksum offloading
	if newLinuxIf.Type == interfaces.Interface_VETH {
		rxOn := getRxChksmOffloading(newLinuxIf)
		txOn := getTxChksmOffloading(newLinuxIf)
		if rxOn != getRxChksmOffloading(oldLinuxIf) || txOn != getTxChksmOffloading(oldLinuxIf) {
			err = d.ifHandler.SetChecksumOffloading(newHostName, rxOn, txOn)
			if err != nil {
				err = errors.Errorf("failed to reconfigure checksum offloading (rx=%t,tx=%t) for linux interface %s: %v",
					rxOn, txOn, newLinuxIf.Name, err)
				d.log.Error(err)
				return nil, err
			}
		}
	}

	// update metadata
	link, err := d.ifHandler.GetLinkByName(newHostName)
	if err != nil {
		d.log.Error(err)
		return nil, err
	}
	oldMetadata.LinuxIfIndex = link.Attrs().Index
	return oldMetadata, nil
}

// ModifyWithRecreate returns true if Type or Type-specific attributes are different.
func (d *InterfaceDescriptor) ModifyWithRecreate(key string, oldLinuxIf, newLinuxIf *interfaces.Interface, metadata *ifaceidx.LinuxIfMetadata) bool {
	if oldLinuxIf.Type != newLinuxIf.Type {
		return true
	}
	if !proto.Equal(oldLinuxIf.Namespace, newLinuxIf.Namespace) {
		// anything attached to the interface (ARPs, routes, ...) will be re-created as well
		return true
	}
	switch oldLinuxIf.Type {
	case interfaces.Interface_VETH:
		return oldLinuxIf.GetVeth().GetPeerIfName() != newLinuxIf.GetVeth().GetPeerIfName()
	case interfaces.Interface_TAP_TO_VPP:
		return oldLinuxIf.GetTap().GetVppTapIfName() != newLinuxIf.GetTap().GetVppTapIfName()
	}
	return false
}

// Dependencies lists dependencies for a Linux interface.
func (d *InterfaceDescriptor) Dependencies(key string, linuxIf *interfaces.Interface) []kvs.Dependency {
	var dependencies []kvs.Dependency

	if linuxIf.Type == interfaces.Interface_TAP_TO_VPP {
		// dependency on VPP TAP
		dependencies = append(dependencies, kvs.Dependency{
			Label: tapInterfaceDep,
			Key:   vpp_intf.InterfaceKey(linuxIf.GetTap().GetVppTapIfName()),
		})
	}

	// circular dependency between VETH ends
	if linuxIf.Type == interfaces.Interface_VETH {
		peerName := linuxIf.GetVeth().GetPeerIfName()
		if peerName != "" {
			dependencies = append(dependencies, kvs.Dependency{
				Label: vethPeerDep,
				Key:   interfaces.InterfaceKey(peerName),
			})
		}
	}

	if linuxIf.GetNamespace().GetType() == namespace.NetNamespace_MICROSERVICE {
		dependencies = append(dependencies, kvs.Dependency{
			Label: microserviceDep,
			Key:   namespace.MicroserviceKey(linuxIf.Namespace.Reference),
		})
	}

	return dependencies
}

// DerivedValues derives one empty value to represent interface state and also
// one empty value for every IP address assigned to the interface.
func (d *InterfaceDescriptor) DerivedValues(key string, linuxIf *interfaces.Interface) (derValues []kvs.KeyValuePair) {
	// interface state
	derValues = append(derValues, kvs.KeyValuePair{
		Key:   interfaces.InterfaceStateKey(linuxIf.Name, linuxIf.Enabled),
		Value: &prototypes.Empty{},
	})
	// IP addresses
	for _, ipAddr := range linuxIf.IpAddresses {
		derValues = append(derValues, kvs.KeyValuePair{
			Key:   interfaces.InterfaceAddressKey(linuxIf.Name, ipAddr),
			Value: &prototypes.Empty{},
		})
	}
	return derValues
}

// ifaceDump is used as the return value sent via channel by dumpInterfaces().
type ifaceDump struct {
	interfaces []adapter.InterfaceKVWithMetadata
	err        error
}

// Dump returns all Linux interfaces managed by this agent, attached to the default namespace
// or to one of the configured non-default namespaces.
func (d *InterfaceDescriptor) Dump(correlate []adapter.InterfaceKVWithMetadata) ([]adapter.InterfaceKVWithMetadata, error) {
	nsList := []*namespace.NetNamespace{nil}        // nil = default namespace, which always should be dumped
	ifCfg := make(map[string]*interfaces.Interface) // interface logical name -> interface config (as expected by correlate)

	// process interfaces for correlation to get:
	//  - the set of namespaces to dump
	//  - mapping between interface name and the configuration for correlation
	// beware: the same namespace can have multiple different references (e.g. integration of Contiv with SFC)
	for _, kv := range correlate {
		nsListed := false
		for _, ns := range nsList {
			if proto.Equal(ns, kv.Value.Namespace) {
				nsListed = true
				break
			}
		}
		if !nsListed {
			nsList = append(nsList, kv.Value.Namespace)
		}
		ifCfg[kv.Value.Name] = kv.Value
	}

	// determine the number of go routines to invoke
	goRoutinesCnt := len(nsList) / minWorkForGoRoutine
	if goRoutinesCnt == 0 {
		goRoutinesCnt = 1
	}
	if goRoutinesCnt > d.dumpGoRoutinesCnt {
		goRoutinesCnt = d.dumpGoRoutinesCnt
	}
	dumpCh := make(chan ifaceDump, goRoutinesCnt)

	// invoke multiple go routines for more efficient parallel dumping
	for idx := 0; idx < goRoutinesCnt; idx++ {
		if goRoutinesCnt > 1 {
			go d.dumpInterfaces(nsList, idx, goRoutinesCnt, dumpCh)
		} else {
			d.dumpInterfaces(nsList, idx, goRoutinesCnt, dumpCh)
		}
	}

	// receive results from the go routines
	ifDump := make(map[string]adapter.InterfaceKVWithMetadata) // interface logical name -> interface dump
	indexes := make(map[int]struct{})                          // already dumped interfaces by their Linux indexes
	for idx := 0; idx < goRoutinesCnt; idx++ {
		dump := <-dumpCh
		if dump.err != nil {
			return nil, dump.err
		}
		for _, kv := range dump.interfaces {
			// skip if this interface was already dumped and this is not the expected
			// namespace from correlation - remember, the same namespace may have
			// multiple different references
			rewrite := false
			if _, dumped := indexes[kv.Metadata.LinuxIfIndex]; dumped {
				if expCfg, hasExpCfg := ifCfg[kv.Value.Name]; hasExpCfg {
					if proto.Equal(expCfg.Namespace, kv.Value.Namespace) {
						rewrite = true
					}
				}
				if !rewrite {
					continue
				}
			}
			indexes[kv.Metadata.LinuxIfIndex] = struct{}{}

			// test for duplicity of VETH logical names
			if kv.Value.Type == interfaces.Interface_VETH {
				if _, duplicate := ifDump[kv.Value.Name]; duplicate && !rewrite {
					// add suffix to the duplicate to make its logical name unique
					// (and not configured by NB so that it will get removed)
					dupIndex := 1
					for intf2 := range ifDump {
						if strings.HasPrefix(intf2, kv.Value.Name+vethDuplicateSuffix) {
							dupIndex++
						}
					}
					kv.Value.Name = kv.Value.Name + vethDuplicateSuffix + strconv.Itoa(dupIndex)
					kv.Key = interfaces.InterfaceKey(kv.Value.Name)
				}
			}
			ifDump[kv.Value.Name] = kv
		}
	}

	// first collect VETHs with duplicate logical names
	var dump []adapter.InterfaceKVWithMetadata
	for ifName, kv := range ifDump {
		if kv.Value.Type == interfaces.Interface_VETH {
			isDuplicate := strings.Contains(ifName, vethDuplicateSuffix)
			// first interface dumped from the set of duplicate VETHs still
			// does not have the vethDuplicateSuffix appended to the name
			_, hasDuplicate := ifDump[ifName+vethDuplicateSuffix+"1"]
			if hasDuplicate {
				kv.Value.Name = ifName + vethDuplicateSuffix + "0"
				kv.Key = interfaces.InterfaceKey(kv.Value.Name)
			}
			if isDuplicate || hasDuplicate {
				// clear peer reference so that Delete removes the VETH-end
				// as standalone
				kv.Value.Link = &interfaces.Interface_Veth{}
				delete(ifDump, ifName)
				dump = append(dump, kv)
			}
		}
	}

	// next collect VETHs with missing peer
	for ifName, kv := range ifDump {
		if kv.Value.Type == interfaces.Interface_VETH {
			peer, dumped := ifDump[kv.Value.GetVeth().GetPeerIfName()]
			if !dumped || peer.Value.GetVeth().GetPeerIfName() != kv.Value.Name {
				// append vethMissingPeerSuffix to the logical name so that VETH
				// will get removed during resync
				kv.Value.Name = ifName + vethMissingPeerSuffix
				kv.Key = interfaces.InterfaceKey(kv.Value.Name)
				// clear peer reference so that Delete removes the VETH-end
				// as standalone
				kv.Value.Link = &interfaces.Interface_Veth{}
				delete(ifDump, ifName)
				dump = append(dump, kv)
			}
		}
	}

	// finally collect AUTO-TAPs and valid VETHs
	for _, kv := range ifDump {
		dump = append(dump, kv)
	}

	return dump, nil
}

// dumpInterfaces is run by a separate go routine to dump all interfaces present
// in every <goRoutineIdx>-th network namespace from the list.
func (d *InterfaceDescriptor) dumpInterfaces(nsList []*namespace.NetNamespace, goRoutineIdx, goRoutinesCnt int, dumpCh chan<- ifaceDump) {
	var dump ifaceDump
	agentPrefix := d.serviceLabel.GetAgentPrefix()
	nsCtx := nslinuxcalls.NewNamespaceMgmtCtx()

	for i := goRoutineIdx; i < len(nsList); i += goRoutinesCnt {
		nsRef := nsList[i]
		// switch to the namespace
		revert, err := d.nsPlugin.SwitchToNamespace(nsCtx, nsRef)
		if err != nil {
			d.log.WithFields(logging.Fields{
				"err":       err,
				"namespace": nsRef,
			}).Warn("Failed to dump namespace")
			continue // continue with the next namespace
		}

		// get all links in the namespace
		links, err := d.ifHandler.GetLinkList()
		if err != nil {
			// switch back to the default namespace before returning error
			revert()
			dump.err = err
			d.log.Error(dump.err)
			break
		}

		// dump every interface managed by this agent
		for _, link := range links {
			intf := &interfaces.Interface{
				Namespace:   nsRef,
				HostIfName:  link.Attrs().Name,
				PhysAddress: link.Attrs().HardwareAddr.String(),
				Mtu:         uint32(link.Attrs().MTU),
			}

			alias := link.Attrs().Alias
			if !strings.HasPrefix(alias, agentPrefix) {
				// skip interface not configured by this agent
				continue
			}
			alias = strings.TrimPrefix(alias, agentPrefix)

			// parse alias to obtain logical references
			var vppTapIfName string
			if link.Type() == (&netlink.Veth{}).Type() {
				var vethPeerIfName string
				intf.Type = interfaces.Interface_VETH
				intf.Name, vethPeerIfName = parseVethAlias(alias)
				intf.Link = &interfaces.Interface_Veth{
					Veth: &interfaces.VethLink{PeerIfName: vethPeerIfName}}
			} else if link.Type() == (&netlink.Tuntap{}).Type() || link.Type() == "tun" /* not defined in vishvananda */ {
				intf.Type = interfaces.Interface_TAP_TO_VPP
				intf.Name, vppTapIfName, _ = parseTapAlias(alias)
				intf.Link = &interfaces.Interface_Tap{
					Tap: &interfaces.TapLink{VppTapIfName: vppTapIfName}}
			} else {
				// unsupported interface type supposedly configured by agent => print warning
				d.log.WithFields(logging.Fields{
					"if-host-name": link.Attrs().Name,
					"if-type":      link.Type(),
					"namespace":    nsRef,
				}).Warn("Managed interface of unsupported type")
				continue
			}

			// skip interfaces with invalid aliases
			if intf.Name == "" {
				continue
			}

			// dump interface status
			intf.Enabled, err = d.ifHandler.IsInterfaceUp(link.Attrs().Name)
			if err != nil {
				d.log.WithFields(logging.Fields{
					"if-host-name": link.Attrs().Name,
					"namespace":    nsRef,
					"err":          err,
				}).Warn("Failed to read interface status")
			}

			// dump assigned IP addresses
			addressList, err := d.ifHandler.GetAddressList(link.Attrs().Name)
			if err != nil {
				d.log.WithFields(logging.Fields{
					"if-host-name": link.Attrs().Name,
					"namespace":    nsRef,
					"err":          err,
				}).Warn("Failed to read IP addresses")
			}
			for _, address := range addressList {
				if address.Scope == unix.RT_SCOPE_LINK {
					// ignore link-local IPv6 addresses
					continue
				}
				mask, _ := address.Mask.Size()
				addrStr := address.IP.String() + "/" + strconv.Itoa(mask)
				intf.IpAddresses = append(intf.IpAddresses, addrStr)
			}

			// dump checksum offloading
			if intf.Type == interfaces.Interface_VETH {
				rxOn, txOn, err := d.ifHandler.GetChecksumOffloading(link.Attrs().Name)
				if err != nil {
					d.log.WithFields(logging.Fields{
						"if-host-name": link.Attrs().Name,
						"namespace":    nsRef,
						"err":          err,
					}).Warn("Failed to read checksum offloading")
				} else {
					if !rxOn {
						intf.GetVeth().RxChecksumOffloading = interfaces.VethLink_CHKSM_OFFLOAD_DISABLED
					}
					if !txOn {
						intf.GetVeth().TxChecksumOffloading = interfaces.VethLink_CHKSM_OFFLOAD_DISABLED
					}
				}
			}

			// build key-value pair for the dumped interface
			dump.interfaces = append(dump.interfaces, adapter.InterfaceKVWithMetadata{
				//Key:    interfaces.InterfaceKey(intf.Name),
				Key:    models.Key(intf),
				Value:  intf,
				Origin: kvs.FromNB,
				Metadata: &ifaceidx.LinuxIfMetadata{
					LinuxIfIndex: link.Attrs().Index,
					VPPTapName:   vppTapIfName,
					Namespace:    nsRef,
				},
			})
		}

		// switch back to the default namespace
		revert()
	}

	dumpCh <- dump
}

// setInterfaceNamespace moves linux interface from the current to the desired
// namespace.
func (d *InterfaceDescriptor) setInterfaceNamespace(ctx nslinuxcalls.NamespaceMgmtCtx, ifName string, namespace *namespace.NetNamespace) error {
	// Get namespace handle.
	ns, err := d.nsPlugin.GetNamespaceHandle(ctx, namespace)
	if err != nil {
		return err
	}
	defer ns.Close()

	// Get the interface link handle.
	link, err := d.ifHandler.GetLinkByName(ifName)
	if err != nil {
		return errors.Errorf("failed to get link for interface %s: %v", ifName, err)
	}

	// When interface moves from one namespace to another, it loses all its IP addresses, admin status
	// and MTU configuration -- we need to remember the interface configuration before the move
	// and re-configure the interface in the new namespace.
	addresses, isIPv6, err := d.getInterfaceAddresses(link.Attrs().Name)
	if err != nil {
		return errors.Errorf("failed to get IP address list from interface %s: %v", link.Attrs().Name, err)
	}
	enabled, err := d.ifHandler.IsInterfaceUp(ifName)
	if err != nil {
		return errors.Errorf("failed to get admin status of the interface %s: %v", link.Attrs().Name, err)
	}

	// Move the interface into the namespace.
	err = d.ifHandler.SetLinkNamespace(link, ns)
	if err != nil {
		return errors.Errorf("failed to set interface %s file descriptor: %v", link.Attrs().Name, err)
	}

	// Re-configure interface in its new namespace
	revertNs, err := d.nsPlugin.SwitchToNamespace(ctx, namespace)
	if err != nil {
		return errors.Errorf("failed to switch namespace: %v", err)
	}
	defer revertNs()

	if enabled {
		// Re-enable interface
		err = d.ifHandler.SetInterfaceUp(ifName)
		if nil != err {
			return errors.Errorf("failed to re-enable Linux interface `%s`: %v", ifName, err)
		}
	}

	// Re-add IP addresses
	for _, address := range addresses {
		// Skip IPv6 link local address if there is no other IPv6 address
		if !isIPv6 && address.IP.IsLinkLocalUnicast() {
			continue
		}
		err = d.ifHandler.AddInterfaceIP(ifName, address)
		if err != nil {
			if err.Error() == "file exists" {
				continue
			}
			return errors.Errorf("failed to re-assign IP address to a Linux interface `%s`: %v", ifName, err)
		}
	}

	// Revert back the MTU config
	err = d.ifHandler.SetInterfaceMTU(ifName, link.Attrs().MTU)
	if nil != err {
		return errors.Errorf("failed to re-assign MTU of a Linux interface `%s`: %v", ifName, err)
	}

	return nil
}

// getInterfaceAddresses returns a list of IP addresses assigned to the given linux interface.
// <hasIPv6> is returned as true if a non link-local IPv6 address is among them.
func (d *InterfaceDescriptor) getInterfaceAddresses(ifName string) (addresses []*net.IPNet, hasIPv6 bool, err error) {
	// get all assigned IP addresses
	ipAddrs, err := d.ifHandler.GetAddressList(ifName)
	if err != nil {
		return nil, false, err
	}

	// iterate over IP addresses to see if there is IPv6 among them
	for _, ipAddr := range ipAddrs {
		if ipAddr.IP.To4() == nil && !ipAddr.IP.IsLinkLocalUnicast() {
			// IP address is version 6 and not a link local address
			hasIPv6 = true
		}
		addresses = append(addresses, ipAddr.IPNet)
	}
	return addresses, hasIPv6, nil
}

// getHostIfName returns the interface host name.
func getHostIfName(linuxIf *interfaces.Interface) string {
	hostIfName := linuxIf.HostIfName
	if hostIfName == "" {
		hostIfName = linuxIf.Name
	}
	return hostIfName
}

// getInterfaceMTU returns the interface MTU.
func getInterfaceMTU(linuxIntf *interfaces.Interface) int {
	mtu := int(linuxIntf.Mtu)
	if mtu == 0 {
		return defaultEthernetMTU
	}
	return mtu
}

func getRxChksmOffloading(linuxIntf *interfaces.Interface) (rxOn bool) {
	return isChksmOffloadingOn(linuxIntf.GetVeth().GetRxChecksumOffloading())
}

func getTxChksmOffloading(linuxIntf *interfaces.Interface) (txOn bool) {
	return isChksmOffloadingOn(linuxIntf.GetVeth().GetTxChecksumOffloading())
}

func isChksmOffloadingOn(offloading interfaces.VethLink_ChecksumOffloading) bool {
	switch offloading {
	case interfaces.VethLink_CHKSM_OFFLOAD_DEFAULT:
		return true // enabled by default
	case interfaces.VethLink_CHKSM_OFFLOAD_ENABLED:
		return true
	case interfaces.VethLink_CHKSM_OFFLOAD_DISABLED:
		return false
	}
	return true
}
