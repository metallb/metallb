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

package l3plugin_test

import (
	"fmt"
	"net"
	"testing"

	"github.com/ligato/vpp-agent/plugins/linux/l3plugin/l3idx"

	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/vpp-agent/idxvpp/nametoidx"
	"github.com/ligato/vpp-agent/plugins/linux/ifplugin/ifaceidx"
	"github.com/ligato/vpp-agent/plugins/linux/l3plugin"
	"github.com/ligato/vpp-agent/plugins/linux/model/l3"
	"github.com/ligato/vpp-agent/tests/linuxmock"
	. "github.com/onsi/gomega"
	"github.com/vishvananda/netlink"
)

/* Linux route configurator init and close */

// Test init function
func TestLinuxRouteConfiguratorInit(t *testing.T) {
	plugin, _, _, _ := routeTestSetup(t)
	defer routeTestTeardown(plugin)
	// Base fields
	Expect(plugin).ToNot(BeNil())
	// Mappings & cache
	Expect(plugin.GetRouteIndexes()).ToNot(BeNil())
	Expect(plugin.GetRouteIndexes().GetMapping().ListNames()).To(HaveLen(0))
	Expect(plugin.GetAutoRouteIndexes()).ToNot(BeNil())
	Expect(plugin.GetAutoRouteIndexes().GetMapping().ListNames()).To(HaveLen(0))
	Expect(plugin.GetCachedRoutes()).ToNot(BeNil())
	Expect(plugin.GetCachedRoutes().GetMapping().ListNames()).To(HaveLen(0))
	Expect(plugin.GetCachedGatewayRoutes()).ToNot(BeNil())
	Expect(plugin.GetCachedGatewayRoutes()).To(HaveLen(0))
}

/* Linux route configurator test cases */

// Configure static route entry
func TestLinuxConfiguratorAddStaticRoute(t *testing.T) {
	plugin, _, _, _ := routeTestSetup(t)
	defer routeTestTeardown(plugin)

	// Test route create
	data := getTestStaticRoute("route1", "", "10.0.0.1/24", "", "", 100, 1,
		nil, 1)
	err := plugin.ConfigureLinuxStaticRoute(data)
	Expect(err).ShouldNot(HaveOccurred())
	_, meta, found := plugin.GetRouteIndexes().LookupIdx(l3plugin.RouteIdentifier(getRouteID("10.0.0.1/24", "", 0, 1)))
	Expect(found).To(BeTrue())
	Expect(meta).ToNot(BeNil())
	_, _, found = plugin.GetAutoRouteIndexes().LookupIdx(l3plugin.RouteIdentifier(getRouteID("10.0.0.1/24", "", 0, 1)))
	Expect(found).To(BeFalse())
	_, _, found = plugin.GetCachedRoutes().LookupIdx("route1")
	Expect(found).To(BeFalse())
	_, ok := plugin.GetCachedGatewayRoutes()["route1"]
	Expect(ok).To(BeFalse())
}

// Configure static route entry with interface
func TestLinuxConfiguratorAddStaticRouteWithInterface(t *testing.T) {
	plugin, _, _, ifIndexes := routeTestSetup(t)
	defer routeTestTeardown(plugin)

	// Register interface
	ifIndexes.RegisterName("if1", 1, getInterfaceData("if1", 1))
	// Test route create
	data := getTestStaticRoute("route1", "if1", "10.0.0.1/24", "", "", 100, 1,
		&l3.LinuxStaticRoutes_Route_Scope{}, 1)
	err := plugin.ConfigureLinuxStaticRoute(data)
	Expect(err).ShouldNot(HaveOccurred())
	_, meta, found := plugin.GetRouteIndexes().LookupIdx(l3plugin.RouteIdentifier(getRouteID("10.0.0.1/24", "", 1, 1)))
	Expect(found).To(BeTrue())
	Expect(meta).ToNot(BeNil())
	_, _, found = plugin.GetAutoRouteIndexes().LookupIdx(l3plugin.RouteIdentifier(getRouteID("10.0.0.1/24", "", 1, 1)))
	Expect(found).To(BeFalse())
	_, _, found = plugin.GetCachedRoutes().LookupIdx("route1")
	Expect(found).To(BeFalse())
	_, ok := plugin.GetCachedGatewayRoutes()["route1"]
	Expect(ok).To(BeFalse())
}

// Configure static route entry with interface
func TestLinuxConfiguratorAddStaticRouteWithMissingInterface(t *testing.T) {
	plugin, _, _, _ := routeTestSetup(t)
	defer routeTestTeardown(plugin)

	// Test route create
	data := getTestStaticRoute("route1", "if1", "10.0.0.1/24", "", "", 100, 1,
		&l3.LinuxStaticRoutes_Route_Scope{}, 1)
	err := plugin.ConfigureLinuxStaticRoute(data)
	Expect(err).ShouldNot(HaveOccurred())
	_, _, found := plugin.GetRouteIndexes().LookupIdx(l3plugin.RouteIdentifier(getRouteID("10.0.0.1/24", "", 0, 1)))
	Expect(found).To(BeFalse())
	_, _, found = plugin.GetAutoRouteIndexes().LookupIdx(l3plugin.RouteIdentifier(getRouteID("10.0.0.1/24", "", 0, 1)))
	Expect(found).To(BeFalse())
	_, meta, found := plugin.GetCachedRoutes().LookupIdx("route1")
	Expect(found).To(BeTrue())
	Expect(meta).ToNot(BeNil())
	_, ok := plugin.GetCachedGatewayRoutes()["route1"]
	Expect(ok).To(BeFalse())
}

// Configure static route entry with reachable gateway and source address
func TestLinuxConfiguratorAddStaticRouteGatewaySrc(t *testing.T) {
	plugin, _, _, _ := routeTestSetup(t)
	defer routeTestTeardown(plugin)

	// Register route ensuring network reachability
	plugin.GetRouteIndexes().RegisterName("dummy-rt", 1, &l3.LinuxStaticRoutes_Route{
		Name:      "dummy-rt",
		DstIpAddr: "20.0.0.1/24",
		Namespace: &l3.LinuxStaticRoutes_Route_Namespace{
			Type: 1,
		},
	})
	// Test route create
	data := getTestStaticRoute("route1", "", "10.0.0.1/24", "192.168.1.1", "20.0.0.2", 100, 1,
		&l3.LinuxStaticRoutes_Route_Scope{}, 1)
	err := plugin.ConfigureLinuxStaticRoute(data)
	Expect(err).ShouldNot(HaveOccurred())
	_, meta, found := plugin.GetRouteIndexes().LookupIdx(l3plugin.RouteIdentifier(getRouteID("10.0.0.1/24", "20.0.0.2", 0, 1)))
	Expect(found).To(BeTrue())
	Expect(meta).ToNot(BeNil())
	_, _, found = plugin.GetAutoRouteIndexes().LookupIdx(l3plugin.RouteIdentifier(getRouteID("10.0.0.1/24", "20.0.0.2", 0, 1)))
	Expect(found).To(BeFalse())
	_, _, found = plugin.GetCachedRoutes().LookupIdx("route1")
	Expect(found).To(BeFalse())
	_, ok := plugin.GetCachedGatewayRoutes()["route1"]
	Expect(ok).To(BeFalse())
}

// Configure static route entry with reachable gateway via auto route
func TestLinuxConfiguratorAddStaticRouteGatewayAuto(t *testing.T) {
	plugin, _, _, _ := routeTestSetup(t)
	defer routeTestTeardown(plugin)

	// Register route ensuring network reachability
	plugin.GetAutoRouteIndexes().RegisterName("dummy-rt", 1, &l3.LinuxStaticRoutes_Route{
		Name:      "dummy-rt",
		DstIpAddr: "20.0.0.1/24",
		Namespace: &l3.LinuxStaticRoutes_Route_Namespace{
			Type: 1,
		},
	})
	// Test route create
	data := getTestStaticRoute("route1", "", "10.0.0.1/24", "192.168.1.1", "20.0.0.2", 100, 1,
		&l3.LinuxStaticRoutes_Route_Scope{
			Type: 1,
		}, 1)
	err := plugin.ConfigureLinuxStaticRoute(data)
	Expect(err).ShouldNot(HaveOccurred())
	_, meta, found := plugin.GetRouteIndexes().LookupIdx(l3plugin.RouteIdentifier(getRouteID("10.0.0.1/24", "20.0.0.2", 0, 1)))
	Expect(found).To(BeTrue())
	Expect(meta).ToNot(BeNil())
	_, _, found = plugin.GetAutoRouteIndexes().LookupIdx(l3plugin.RouteIdentifier(getRouteID("10.0.0.1/24", "20.0.0.2", 0, 1)))
	Expect(found).To(BeFalse())
	_, _, found = plugin.GetCachedRoutes().LookupIdx("route1")
	Expect(found).To(BeFalse())
	_, ok := plugin.GetCachedGatewayRoutes()["route1"]
	Expect(ok).To(BeFalse())
}

// Configure static route entry with unreachable gateway
func TestLinuxConfiguratorAddStaticRouteGatewayUnreachable(t *testing.T) {
	plugin, _, _, _ := routeTestSetup(t)
	defer routeTestTeardown(plugin)

	// Test route create
	data := getTestStaticRoute("route1", "", "10.0.0.1/24", "192.168.1.1", "20.0.0.2", 100, 1,
		&l3.LinuxStaticRoutes_Route_Scope{
			Type: 1,
		}, 1)
	err := plugin.ConfigureLinuxStaticRoute(data)
	Expect(err).ShouldNot(HaveOccurred())
	_, _, found := plugin.GetRouteIndexes().LookupIdx(l3plugin.RouteIdentifier(getRouteID("10.0.0.1/24", "20.0.0.2", 0, 1)))
	Expect(found).To(BeFalse())
	_, _, found = plugin.GetAutoRouteIndexes().LookupIdx(l3plugin.RouteIdentifier(getRouteID("10.0.0.1/24", "20.0.0.2", 0, 1)))
	Expect(found).To(BeFalse())
	_, _, found = plugin.GetCachedRoutes().LookupIdx("route1")
	Expect(found).To(BeFalse())
	val, ok := plugin.GetCachedGatewayRoutes()["route1"]
	Expect(ok).To(BeTrue())
	Expect(val).ToNot(BeNil())
}

// Configure static route entry and delete obsolete cached gateway entry
func TestLinuxConfiguratorAddStaticRouteDeleteCachedGateway(t *testing.T) {
	plugin, _, _, _ := routeTestSetup(t)
	defer routeTestTeardown(plugin)

	// Test route create
	data := getTestStaticRoute("route1", "", "10.0.0.1/24", "", "", 100, 1,
		&l3.LinuxStaticRoutes_Route_Scope{
			Type: 2,
		}, 1)
	// Register gateway route
	plugin.GetCachedGatewayRoutes()["route1"] = data
	// Configure
	err := plugin.ConfigureLinuxStaticRoute(data)
	Expect(err).ShouldNot(HaveOccurred())
	_, ok := plugin.GetCachedGatewayRoutes()["route1"]
	Expect(ok).To(BeFalse())
}

// Configure static route with invalid destination address
func TestLinuxConfiguratorAddStaticRouteInvalidDstIP(t *testing.T) {
	plugin, _, _, _ := routeTestSetup(t)
	defer routeTestTeardown(plugin)

	// Test route create
	data := getTestStaticRoute("route1", "", "invalid-dst-ip/24", "", "", 100, 1,
		nil, 1)
	// Configure
	err := plugin.ConfigureLinuxStaticRoute(data)
	Expect(err).Should(HaveOccurred())
}

// Configure static route with missing destination address mask
func TestLinuxConfiguratorAddStaticRouteMissingDstIPMask(t *testing.T) {
	plugin, _, _, _ := routeTestSetup(t)
	defer routeTestTeardown(plugin)

	// Test route create
	data := getTestStaticRoute("route1", "", "10.0.0.1", "", "", 100, 1,
		nil, 1)
	// Configure
	err := plugin.ConfigureLinuxStaticRoute(data)
	Expect(err).Should(HaveOccurred())
}

// Configure static route with missing destination address
func TestLinuxConfiguratorAddStaticRouteMissingDstIP(t *testing.T) {
	plugin, _, _, _ := routeTestSetup(t)
	defer routeTestTeardown(plugin)

	// Test route create
	data := getTestStaticRoute("route1", "", "", "", "", 100, 1,
		nil, 1)
	// Configure
	err := plugin.ConfigureLinuxStaticRoute(data)
	Expect(err).Should(HaveOccurred())
}

// Configure default route entry
func TestLinuxConfiguratorAddDefaultRoute(t *testing.T) {
	plugin, _, _, _ := routeTestSetup(t)
	defer routeTestTeardown(plugin)

	// Register route ensuring network reachability
	plugin.GetAutoRouteIndexes().RegisterName("dummy-rt", 1, &l3.LinuxStaticRoutes_Route{
		Name:      "dummy-rt",
		DstIpAddr: "20.0.0.1/24",
		Namespace: &l3.LinuxStaticRoutes_Route_Namespace{
			Type: 1,
		},
	})
	// Test route create
	data := getTestDefaultRoute("route1", "", "", "", "20.0.0.2", 100, 0,
		nil, 1)
	err := plugin.ConfigureLinuxStaticRoute(data)
	Expect(err).ShouldNot(HaveOccurred())
	_, meta, found := plugin.GetRouteIndexes().LookupIdx(l3plugin.RouteIdentifier(getRouteID("", "20.0.0.2", 0, 0)))
	Expect(found).To(BeTrue())
	Expect(meta).ToNot(BeNil())
	_, _, found = plugin.GetAutoRouteIndexes().LookupIdx(l3plugin.RouteIdentifier(getRouteID("", "20.0.0.2", 0, 0)))
	Expect(found).To(BeFalse())
	_, _, found = plugin.GetCachedRoutes().LookupIdx("route1")
	Expect(found).To(BeFalse())
	_, ok := plugin.GetCachedGatewayRoutes()["route1"]
	Expect(ok).To(BeFalse())
}

// Configure static route entry switch namespace error
func TestLinuxConfiguratorAddStaticRouteNamespaceError(t *testing.T) {
	plugin, _, nsHandler, _ := routeTestSetup(t)
	defer routeTestTeardown(plugin)

	nsHandler.When("SwitchNamespace").ThenReturn(fmt.Errorf("switch-namespace-err"))

	// Test route create
	data := getTestStaticRoute("route1", "", "10.0.0.1/24", "", "", 100, 1,
		nil, 1)
	err := plugin.ConfigureLinuxStaticRoute(data)
	Expect(err).Should(HaveOccurred())
}

// Configure static route entry error
func TestLinuxConfiguratorAddStaticRouteError(t *testing.T) {
	plugin, l3Handler, _, _ := routeTestSetup(t)
	defer routeTestTeardown(plugin)

	l3Handler.When("AddStaticRoute").ThenReturn(fmt.Errorf("add-static-route-err"))

	// Test route create
	data := getTestStaticRoute("route1", "", "10.0.0.1/24", "", "", 100, 1,
		nil, 1)
	err := plugin.ConfigureLinuxStaticRoute(data)
	Expect(err).Should(HaveOccurred())
}

/* Route Test Setup */

func routeTestSetup(t *testing.T) (*l3plugin.LinuxRouteConfigurator, *linuxmock.L3NetlinkHandlerMock, *linuxmock.NamespacePluginMock, ifaceidx.LinuxIfIndexRW) {
	RegisterTestingT(t)

	// Loggers
	pluginLog := logging.ForPlugin("linux-route-log")
	pluginLog.SetLevel(logging.DebugLevel)
	nsHandleLog := logging.ForPlugin("ns-handle-log")
	nsHandleLog.SetLevel(logging.DebugLevel)
	// Linux interface indexes
	ifIndexes := ifaceidx.NewLinuxIfIndex(nametoidx.NewNameToIdx(pluginLog, "if", nil))
	rtIndexes := l3idx.NewLinuxRouteIndex(nametoidx.NewNameToIdx(pluginLog, "rt", nil))
	// Configurator
	plugin := &l3plugin.LinuxRouteConfigurator{}
	linuxMock := linuxmock.NewL3NetlinkHandlerMock()
	nsMock := linuxmock.NewNamespacePluginMock()
	err := plugin.Init(pluginLog, linuxMock, nsMock, rtIndexes, ifIndexes)
	Expect(err).To(BeNil())

	return plugin, linuxMock, nsMock, ifIndexes
}

func routeTestTeardown(plugin *l3plugin.LinuxRouteConfigurator) {
	err := plugin.Close()
	Expect(err).To(BeNil())
	logging.DefaultRegistry.ClearRegistry()
}

func getRouteID(dst, gw string, ifIdx, table uint32) *netlink.Route {
	if dst == "" {
		dst = "0.0.0.0/0"
	}
	_, dstIP, _ := net.ParseCIDR(dst) // error is not important
	return &netlink.Route{
		LinkIndex: int(ifIdx),
		Dst:       dstIP,
		Gw:        net.ParseIP(gw),
		Table:     int(table),
	}
}

/* Linux route Test Data */

func getTestStaticRoute(rtName, ifName, dstIp, srcIp, gwIp string, metric, table uint32, scope *l3.LinuxStaticRoutes_Route_Scope,
	namespaceType l3.LinuxStaticRoutes_Route_Namespace_NamespaceType) *l3.LinuxStaticRoutes_Route {
	return &l3.LinuxStaticRoutes_Route{
		Name:      rtName,
		Namespace: getNamespace(ifName, namespaceType),
		Interface: ifName,
		DstIpAddr: dstIp,
		SrcIpAddr: srcIp,
		GwAddr:    gwIp,
		Scope:     scope,
		Metric:    metric,
		Table:     table,
	}
}

func getTestDefaultRoute(rtName, ifName, dstIp, srcIp, gwIp string, metric, table uint32, scope *l3.LinuxStaticRoutes_Route_Scope,
	namespaceType l3.LinuxStaticRoutes_Route_Namespace_NamespaceType) *l3.LinuxStaticRoutes_Route {
	return &l3.LinuxStaticRoutes_Route{
		Name:      rtName,
		Default:   true,
		Namespace: getNamespace(ifName, namespaceType),
		Interface: ifName,
		DstIpAddr: dstIp,
		SrcIpAddr: srcIp,
		GwAddr:    gwIp,
		Scope:     scope,
		Metric:    metric,
		Table:     table,
	}
}

func getNamespace(rtName string, namespaceType l3.LinuxStaticRoutes_Route_Namespace_NamespaceType) *l3.LinuxStaticRoutes_Route_Namespace {
	if namespaceType < 4 {
		return &l3.LinuxStaticRoutes_Route_Namespace{
			Type:         namespaceType,
			Microservice: rtName + "-ms",
		}
	}
	return nil
}
