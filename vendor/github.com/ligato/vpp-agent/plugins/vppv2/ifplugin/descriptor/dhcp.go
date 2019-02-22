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
	"context"
	"fmt"
	"net"
	"strings"
	"sync"

	prototypes "github.com/gogo/protobuf/types"

	govppapi "git.fd.io/govpp.git/api"
	"github.com/ligato/cn-infra/logging"
	kvs "github.com/ligato/vpp-agent/plugins/kvscheduler/api"

	"bytes"

	"github.com/gogo/protobuf/proto"
	interfaces "github.com/ligato/vpp-agent/api/models/vpp/interfaces"
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/dhcp"
	"github.com/ligato/vpp-agent/plugins/vppv2/ifplugin/ifaceidx"
	"github.com/ligato/vpp-agent/plugins/vppv2/ifplugin/vppcalls"
	"github.com/pkg/errors"
)

const (
	// DHCPDescriptorName is the name of the descriptor configuring DHCP for VPP
	// interfaces.
	DHCPDescriptorName = "vpp-dhcp"
)

// DHCPDescriptor enables/disables DHCP for VPP interfaces and notifies about
// new DHCP leases.
type DHCPDescriptor struct {
	// provided by the plugin
	log         logging.Logger
	ifHandler   vppcalls.IfVppAPI
	kvscheduler kvs.KVScheduler
	ifIndex     ifaceidx.IfaceMetadataIndex

	// DHCP notification watching
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewDHCPDescriptor creates a new instance of DHCPDescriptor.
func NewDHCPDescriptor(kvscheduler kvs.KVScheduler, ifHandler vppcalls.IfVppAPI, log logging.PluginLogger) *DHCPDescriptor {
	descriptor := &DHCPDescriptor{
		kvscheduler: kvscheduler,
		ifHandler:   ifHandler,
		log:         log.NewLogger("dhcp-descriptor"),
	}
	return descriptor
}

// GetDescriptor returns descriptor suitable for registration with the KVScheduler.
func (d *DHCPDescriptor) GetDescriptor() *kvs.KVDescriptor {
	return &kvs.KVDescriptor{
		Name:             DHCPDescriptorName,
		KeySelector:      d.IsDHCPRelatedKey,
		KeyLabel:         d.InterfaceNameFromKey,
		WithMetadata:     true,            // DHCP leases
		Add:              d.Add,           // DHCP client
		Delete:           d.Delete,        // DHCP client
		DerivedValues:    d.DerivedValues, // IP address from DHCP lease
		Dump:             d.Dump,          // DHCP leases
		DumpDependencies: []string{InterfaceDescriptorName},
	}
}

// SetInterfaceIndex should be used to provide interface index immediately after
// the descriptor registration.
func (d *DHCPDescriptor) SetInterfaceIndex(ifIndex ifaceidx.IfaceMetadataIndex) {
	d.ifIndex = ifIndex
}

// WatchDHCPNotifications starts watching for DHCP notifications.
func (d *DHCPDescriptor) WatchDHCPNotifications(ctx context.Context, dhcpChan chan govppapi.Message) {
	// Create child context
	var childCtx context.Context
	childCtx, d.cancel = context.WithCancel(ctx)

	d.wg.Add(1)
	go d.watchDHCPNotifications(childCtx, dhcpChan)
}

// Close stops watching of DHCP notifications.
func (d *DHCPDescriptor) Close() error {
	d.cancel()
	d.wg.Wait()
	return nil
}

// IsDHCPRelatedKey returns true if the key is identifying DHCP client (derived value)
// or DHCP lease (notification).
func (d *DHCPDescriptor) IsDHCPRelatedKey(key string) bool {
	if _, isValid := interfaces.ParseNameFromDHCPClientKey(key); isValid {
		return true
	}
	if _, isValid := interfaces.ParseNameFromDHCPLeaseKey(key); isValid {
		return true
	}
	return false
}

// InterfaceNameFromKey returns interface name from DHCP-related key.
func (d *DHCPDescriptor) InterfaceNameFromKey(key string) string {
	if iface, isValid := interfaces.ParseNameFromDHCPClientKey(key); isValid {
		return iface
	}
	if iface, isValid := interfaces.ParseNameFromDHCPLeaseKey(key); isValid {
		return iface
	}
	return key
}

// Add enables DHCP client.
func (d *DHCPDescriptor) Add(key string, emptyVal proto.Message) (metadata kvs.Metadata, err error) {
	ifName, _ := interfaces.ParseNameFromDHCPClientKey(key)
	ifMeta, found := d.ifIndex.LookupByName(ifName)
	if !found {
		err = errors.Errorf("failed to find DHCP-enabled interface %s", ifName)
		d.log.Error(err)
		return nil, err
	}

	if err := d.ifHandler.SetInterfaceAsDHCPClient(ifMeta.SwIfIndex, ifName); err != nil {
		err = errors.Errorf("failed to enable DHCP client for interface %s", ifName)
		d.log.Error(err)
		return nil, err
	}

	return nil, err
}

// Delete disables DHCP client.
func (d *DHCPDescriptor) Delete(key string, emptyVal proto.Message, metadata kvs.Metadata) error {
	ifName, _ := interfaces.ParseNameFromDHCPClientKey(key)
	ifMeta, found := d.ifIndex.LookupByName(ifName)
	if !found {
		err := errors.Errorf("failed to find DHCP-enabled interface %s", ifName)
		d.log.Error(err)
		return err
	}

	if err := d.ifHandler.UnsetInterfaceAsDHCPClient(ifMeta.SwIfIndex, ifName); err != nil {
		err = errors.Errorf("failed to disable DHCP client for interface %s", ifName)
		d.log.Error(err)
		return err
	}

	// notify about the unconfigured client by removing the lease notification
	return d.kvscheduler.PushSBNotification(
		interfaces.DHCPLeaseKey(ifName), nil, nil)
}

// DerivedValues derives empty value for leased IP address.
func (d *DHCPDescriptor) DerivedValues(key string, dhcpData proto.Message) (derValues []kvs.KeyValuePair) {
	if strings.HasPrefix(key, interfaces.DHCPLeaseKeyPrefix) {
		dhcpLease, ok := dhcpData.(*interfaces.DHCPLease)
		if ok && dhcpLease.HostIpAddress != "" {
			return []kvs.KeyValuePair{
				{
					Key:   interfaces.InterfaceAddressKey(dhcpLease.InterfaceName, dhcpLease.HostIpAddress),
					Value: &prototypes.Empty{},
				},
			}
		}
	}
	return derValues
}

// Dump returns all existing DHCP leases.
func (d *DHCPDescriptor) Dump(correlate []kvs.KVWithMetadata) (
	dump []kvs.KVWithMetadata, err error,
) {
	// Retrieve VPP configuration.
	dhcpDump, err := d.ifHandler.DumpDhcpClients()
	if err != nil {
		d.log.Error(err)
		return dump, err
	}

	for ifIdx, dhcpData := range dhcpDump {
		ifName, _, found := d.ifIndex.LookupBySwIfIndex(ifIdx)
		if !found {
			d.log.Warnf("failed to find interface sw_if_index=%d with DHCP lease", ifIdx)
			return dump, err
		}
		// Store lease under both value (for visibility & to derive interface IP address)
		// and metadata (for watching).
		lease := &interfaces.DHCPLease{
			InterfaceName:   ifName,
			HostName:        dhcpData.Lease.Hostname,
			HostPhysAddress: dhcpData.Lease.HostMac,
			IsIpv6:          dhcpData.Lease.IsIPv6,
			HostIpAddress:   dhcpData.Lease.HostAddress,
			RouterIpAddress: dhcpData.Lease.RouterAddress,
		}
		dump = append(dump, kvs.KVWithMetadata{
			Key:      interfaces.DHCPLeaseKey(ifName),
			Value:    lease,
			Metadata: lease,
			Origin:   kvs.FromSB,
		})
	}

	return dump, nil
}

// watchDHCPNotifications watches and processes DHCP notifications.
func (d *DHCPDescriptor) watchDHCPNotifications(ctx context.Context, dhcpChan chan govppapi.Message) {
	defer d.wg.Done()
	d.log.Debug("Started watcher on DHCP notifications")

	for {
		select {
		case notification := <-dhcpChan:
			switch dhcpNotif := notification.(type) {
			case *dhcp.DHCPComplEvent:
				lease := dhcpNotif.Lease

				// L2 address (defined for L2 rewrite)
				var hwAddr net.HardwareAddr = lease.HostMac

				// interface hostname
				hostname := string(bytes.SplitN(dhcpNotif.Lease.Hostname, []byte{0x00}, 2)[0])

				// interface and router IP addresses
				var hostIPAddr, routerIPAddr string
				if lease.IsIPv6 == 1 {
					hostIPAddr = fmt.Sprintf("%s/%d", net.IP(lease.HostAddress).To16().String(), uint32(lease.MaskWidth))
					routerIPAddr = fmt.Sprintf("%s/%d", net.IP(lease.RouterAddress).To16().String(), uint32(lease.MaskWidth))
				} else {
					hostIPAddr = fmt.Sprintf("%s/%d", net.IP(lease.HostAddress[:4]).To4().String(), uint32(lease.MaskWidth))
					routerIPAddr = fmt.Sprintf("%s/%d", net.IP(lease.RouterAddress[:4]).To4().String(), uint32(lease.MaskWidth))
				}

				// interface logical name
				ifName, _, found := d.ifIndex.LookupBySwIfIndex(lease.SwIfIndex)
				if !found {
					d.log.Warnf("Interface sw_if_index=%d with DHCP lease was not found in the mapping", lease.SwIfIndex)
					continue
				}

				d.log.Debugf("DHCP assigned %v to interface %q (router address %v)", hostIPAddr, ifName, routerIPAddr)

				// notify about the new lease
				dhcpLease := &interfaces.DHCPLease{
					InterfaceName:   ifName,
					HostName:        hostname,
					HostPhysAddress: hwAddr.String(),
					IsIpv6:          lease.IsIPv6 == 1,
					HostIpAddress:   hostIPAddr,
					RouterIpAddress: routerIPAddr,
				}
				if err := d.kvscheduler.PushSBNotification(
					interfaces.DHCPLeaseKey(ifName),
					dhcpLease,
					dhcpLease); err != nil {
					d.log.Error(err)
				}
			}
		case <-ctx.Done():
			return
		}
	}
}
