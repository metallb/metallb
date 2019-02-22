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

package data

import (
	linuxL3 "github.com/ligato/vpp-agent/api/models/linux/l3"
	l3 "github.com/ligato/vpp-agent/api/models/vpp/l3"
)

// L3Ctl L3 plugin related methods for vpp-agent-ctl (including linux)
type L3Ctl interface {
	// PutRoute puts VPP route configuration to the ETCD
	PutRoute() error
	// DeleteRoute removes VPP route configuration from the ETCD
	DeleteRoute() error
	// PutInterVrfRoute puts inter-VRF VPP route configuration to the ETCD
	PutInterVrfRoute() error
	// DeleteRoute removes VPP route configuration from the ETCD
	DeleteInterVrfRoute() error
	// PutInterVrfRoute puts inter-VRF VPP route configuration with next hop to the ETCD
	PutNextHopRoute() error
	// DeleteNextHopRoute removes VPP route configuration from the ETCD
	DeleteNextHopRoute() error
	// PutLinuxRoute puts linux route configuration to the ETCD
	PutLinuxRoute() error
	// DeleteLinuxRoute removes linux route configuration from the ETCD
	DeleteLinuxRoute() error
	// PutLinuxDefaultRoute puts linux default route configuration to the ETCD
	PutLinuxDefaultRoute() error
	// DeleteLinuxDefaultRoute removes linux default route configuration from the ETCD
	DeleteLinuxDefaultRoute() error
	// PutArp puts VPP ARP entry configuration to the ETCD
	PutArp() error
	// PutArp puts VPP ARP entry configuration to the ETCD
	DeleteArp() error
	// PutProxyArp puts VPP proxy ARP configuration to the ETCD
	PutProxyArp() error
	// DeleteProxyArp removes VPP proxy ARP configuration from the ETCD
	DeleteProxyArp() error
	// SetIPScanNeigh puts VPP IP scan neighbor configuration to the ETCD
	SetIPScanNeigh() error
	// UnsetIPScanNeigh removes VPP IP scan neighbor configuration from the ETCD
	UnsetIPScanNeigh() error
	// CreateLinuxArp puts linux ARP entry configuration to the ETCD
	PutLinuxArp() error
	// DeleteLinuxArp removes Linux ARP entry configuration from the ETCD
	DeleteLinuxArp() error
}

// PutRoute puts VPP route configuration to the ETCD
func (ctl *VppAgentCtlImpl) PutRoute() error {
	route := &l3.Route{
		VrfId:             0,
		DstNetwork:        "10.1.1.3/32",
		NextHopAddr:       "192.168.1.13",
		Weight:            6,
		OutgoingInterface: "tap1",
	}

	ctl.Log.Infof("Route put: %v", route)
	return ctl.broker.Put(l3.RouteKey(route.VrfId, route.DstNetwork, route.NextHopAddr), route)
}

// DeleteRoute removes VPP route configuration from the ETCD
func (ctl *VppAgentCtlImpl) DeleteRoute() error {
	routeKey := l3.RouteKey(0, "10.1.1.3/32", "192.168.1.13")

	ctl.Log.Infof("Route delete: %v", routeKey)
	_, err := ctl.broker.Delete(routeKey)
	return err
}

// PutInterVrfRoute puts inter-VRF VPP route configuration to the ETCD
func (ctl *VppAgentCtlImpl) PutInterVrfRoute() error {
	route := &l3.Route{
		Type:       l3.Route_INTER_VRF,
		VrfId:      0,
		DstNetwork: "1.2.3.4/32",
		ViaVrfId:   1,
	}

	ctl.Log.Infof("Route put: %v", route)
	return ctl.broker.Put(l3.RouteKey(route.VrfId, route.DstNetwork, route.NextHopAddr), route)
}

// DeleteInterVrfRoute removes VPP route configuration from the ETCD
func (ctl *VppAgentCtlImpl) DeleteInterVrfRoute() error {
	routeKey := l3.RouteKey(0, "1.2.3.4/32", "")

	ctl.Log.Infof("Route delete: %v", routeKey)
	_, err := ctl.broker.Delete(routeKey)
	return err
}

// PutNextHopRoute puts inter-VRF VPP route configuration with next hop to the ETCD
func (ctl *VppAgentCtlImpl) PutNextHopRoute() error {
	route := &l3.Route{
		Type:        l3.Route_INTER_VRF,
		VrfId:       1,
		DstNetwork:  "10.1.1.3/32",
		NextHopAddr: "192.168.1.13",
		ViaVrfId:    0,
	}

	ctl.Log.Infof("Route put: %v", route)
	return ctl.broker.Put(l3.RouteKey(route.VrfId, route.DstNetwork, route.NextHopAddr), route)
}

// DeleteNextHopRoute removes VPP route configuration from the ETCD
func (ctl *VppAgentCtlImpl) DeleteNextHopRoute() error {
	routeKey := l3.RouteKey(1, "10.1.1.3/32", "192.168.1.13")

	ctl.Log.Infof("Route delete: %v", routeKey)
	_, err := ctl.broker.Delete(routeKey)
	return err
}

// PutLinuxRoute puts linux route configuration to the ETCD
func (ctl *VppAgentCtlImpl) PutLinuxRoute() error {
	linuxRoute := &linuxL3.Route{
		DstNetwork:        "10.0.2.0/24",
		OutgoingInterface: "veth1",
		Metric:            100,
	}

	ctl.Log.Infof("Route put: %v", linuxRoute)
	return ctl.broker.Put(linuxL3.RouteKey(linuxRoute.DstNetwork, linuxRoute.OutgoingInterface), linuxRoute)
}

// DeleteLinuxRoute removes linux route configuration from the ETCD
func (ctl *VppAgentCtlImpl) DeleteLinuxRoute() error {
	linuxRouteKey := linuxL3.RouteKey("10.0.2.0/24", "veth1")

	ctl.Log.Println("Linux route delete: %v", linuxRouteKey)
	_, err := ctl.broker.Delete(linuxRouteKey)
	return err
}

// PutLinuxDefaultRoute puts linux default route configuration to the ETCD
func (ctl *VppAgentCtlImpl) PutLinuxDefaultRoute() error {
	linuxRoute := &linuxL3.Route{
		GwAddr:            "10.0.2.2",
		OutgoingInterface: "veth1",
		Metric:            100,
	}

	ctl.Log.Infof("Linux default route put: %v", linuxRoute)
	return ctl.broker.Put(linuxL3.RouteKey(linuxRoute.DstNetwork, linuxRoute.OutgoingInterface), linuxRoute)
}

// DeleteLinuxDefaultRoute removes linux default route configuration from the ETCD
func (ctl *VppAgentCtlImpl) DeleteLinuxDefaultRoute() error {
	linuxRouteKey := linuxL3.RouteKey("0.0.0.0/32", "veth1")

	ctl.Log.Info("Linux route delete: %v", linuxRouteKey)
	_, err := ctl.broker.Delete(linuxRouteKey)
	return err
}

// PutArp puts VPP ARP entry configuration to the ETCD
func (ctl *VppAgentCtlImpl) PutArp() error {
	arp := &l3.ARPEntry{
		Interface:   "tap1",
		IpAddress:   "192.168.10.21",
		PhysAddress: "59:6C:45:59:8E:BD",
		Static:      true,
	}

	ctl.Log.Infof("ARP put: %v", arp)
	return ctl.broker.Put(l3.ArpEntryKey(arp.Interface, arp.IpAddress), arp)
}

// DeleteArp removes VPP ARP entry configuration from the ETCD
func (ctl *VppAgentCtlImpl) DeleteArp() error {
	arpKey := l3.ArpEntryKey("tap1", "192.168.10.21")

	ctl.Log.Info("Linux route delete: %v", arpKey)
	_, err := ctl.broker.Delete(arpKey)
	return err
}

// PutProxyArp puts VPP proxy ARP configuration to the ETCD
func (ctl *VppAgentCtlImpl) PutProxyArp() error {
	proxyArp := &l3.ProxyARP{
		Interfaces: []*l3.ProxyARP_Interface{
			{
				Name: "tap1",
			},
			{
				Name: "tap2",
			},
		},
		Ranges: []*l3.ProxyARP_Range{
			{
				FirstIpAddr: "10.0.0.1",
				LastIpAddr:  "10.0.0.3",
			},
		},
	}

	ctl.Log.Infof("Proxy ARP put: %v", proxyArp)
	return ctl.broker.Put(l3.ProxyARPKey(), proxyArp)
}

// DeleteProxyArp removes VPP proxy ARPconfiguration from the ETCD
func (ctl *VppAgentCtlImpl) DeleteProxyArp() error {
	ctl.Log.Info("Proxy ARP deleted")
	_, err := ctl.broker.Delete(l3.ProxyARPKey())
	return err
}

// SetIPScanNeigh puts VPP IP scan neighbor configuration to the ETCD
func (ctl *VppAgentCtlImpl) SetIPScanNeigh() error {
	ipScanNeigh := &l3.IPScanNeighbor{
		Mode:           l3.IPScanNeighbor_BOTH,
		ScanInterval:   11,
		MaxProcTime:    36,
		MaxUpdate:      5,
		ScanIntDelay:   16,
		StaleThreshold: 26,
	}

	ctl.Log.Info("IP scan neighbor set")
	return ctl.broker.Put(l3.IPScanNeighborKey(), ipScanNeigh)
}

// UnsetIPScanNeigh removes VPP IP scan neighbor configuration from the ETCD
func (ctl *VppAgentCtlImpl) UnsetIPScanNeigh() error {
	ctl.Log.Info("IP scan neighbor unset")
	_, err := ctl.broker.Delete(l3.IPScanNeighborKey())
	return err
}

// PutLinuxArp puts linux ARP entry configuration to the ETCD
func (ctl *VppAgentCtlImpl) PutLinuxArp() error {
	linuxArp := &linuxL3.ARPEntry{
		Interface: "veth1",
		IpAddress: "130.0.0.1",
		HwAddress: "46:06:18:DB:05:3A",
	}

	ctl.Log.Info("Linux ARP put: %v", linuxArp)
	return ctl.broker.Put(linuxL3.ArpKey(linuxArp.Interface, linuxArp.IpAddress), linuxArp)
}

// DeleteLinuxArp removes Linux ARP entry configuration from the ETCD
func (ctl *VppAgentCtlImpl) DeleteLinuxArp() error {
	linuxArpKey := linuxL3.ArpKey("veth1", "130.0.0.1")

	ctl.Log.Info("Linux ARP delete: %v", linuxArpKey)
	_, err := ctl.broker.Delete(linuxArpKey)
	return err
}
