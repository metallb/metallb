// SPDX-License-Identifier:Apache-2.0

package main

import (
	"net"
	"strings"
	"testing"

	"github.com/go-kit/log"
	"go.universe.tf/metallb/internal/config"
	"go.universe.tf/metallb/internal/k8s/controllers"
	v1 "k8s.io/api/core/v1"
	discovery "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var logger = log.NewNopLogger()

func mockNewController(l2Handler *MockProtocol, bgpHandler *MockProtocol, t *testing.T) *controller {
	ret := &controller{
		myNode:  "nodeName",
		bgpType: "frr",
		protocolHandlers: map[config.Proto]Protocol{
			config.Layer2: l2Handler,
			config.BGP:    bgpHandler,
		},
		announced: map[config.Proto]map[string]bool{},
		svcIPs:    map[string][]net.IP{},
		protocols: config.Protocols,
		client:    &testK8S{t: t},
	}
	ret.announced[config.BGP] = map[string]bool{}
	ret.announced[config.Layer2] = map[string]bool{}
	return ret
}

func TestLoadBalancerCreation(t *testing.T) {
	var l2MockHandler = &MockProtocol{
		protocol:       config.Layer2,
		shouldAnnounce: true,
	}

	var bgpMockHandler = &MockProtocol{
		protocol:       config.BGP,
		shouldAnnounce: true,
	}
	c := mockNewController(l2MockHandler, bgpMockHandler, t)

	svc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "testsvc",
		},
		Spec: v1.ServiceSpec{
			Type:                  "LoadBalancer",
			ExternalTrafficPolicy: "Cluster",
		},
		Status: statusAssigned("10.20.30.1"),
	}

	cfg := &config.Config{
		Pools: &config.Pools{ByName: map[string]*config.Pool{
			"default": {
				CIDR: []*net.IPNet{ipnet("10.20.30.0/24")},
			},
		}},
	}

	state := c.SetConfig(logger, cfg)
	if state != controllers.SyncStateReprocessAll {
		t.Fatalf("Set config failed")
	}

	// both handlers want to announce the lb ip
	state = c.SetBalancer(logger,
		"testsvc",
		svc,
		[]discovery.EndpointSlice{})
	if state != controllers.SyncStateSuccess {
		t.Fatalf("Set balancer failed")
	}
	if !l2MockHandler.setBalancerCalled {
		t.Fatal("two handlers, l2 handler was not called")
	}
	if !bgpMockHandler.setBalancerCalled {
		t.Fatal("two handlers, bgp handler was not called")
	}
	if !c.svcIPs["testsvc"][0].Equal(net.ParseIP("10.20.30.1")) {
		t.Fatal("two handlers, svc ip is not valid", c.svcIPs["testsvc"][0])
	}
	if !c.announced[config.BGP]["testsvc"] {
		t.Fatal("two handlers, ip is not announced in bgp")
	}
	if !c.announced[config.Layer2]["testsvc"] {
		t.Fatal("two handlers, ip is not announced in bgp")
	}

	l2MockHandler.reset()
	bgpMockHandler.reset()
	l2MockHandler.shouldAnnounce = false

	// the config changed, the l2 handler is not advertising the ip anymore, we check l2 cancels the lb
	state = c.SetBalancer(logger,
		"testsvc",
		svc,
		[]discovery.EndpointSlice{})

	if state != controllers.SyncStateSuccess {
		t.Fatalf("Set balancer failed")
	}
	if l2MockHandler.setBalancerCalled {
		t.Fatal("one handler, l2 handler was called")
	}
	if !l2MockHandler.deleteBalancerCalled {
		t.Fatal("one handlers, l2 delete handler was not called")
	}
	if !bgpMockHandler.setBalancerCalled {
		t.Fatal("one handler, bgp handler was not called")
	}
	if !c.svcIPs["testsvc"][0].Equal(net.ParseIP("10.20.30.1")) {
		t.Fatal("one handler, svc ip is not valid", c.svcIPs["testsvc"][0])
	}
	if !c.announced[config.BGP]["testsvc"] {
		t.Fatal("one handler, ip is not announced in bgp")
	}
	if c.announced[config.Layer2]["testsvc"] {
		t.Fatal("one handler, ip is not announced in l2")
	}

	l2MockHandler.reset()
	bgpMockHandler.reset()
	l2MockHandler.shouldAnnounce = false
	bgpMockHandler.shouldAnnounce = false

	// the config changed, no handler is advertising the ip, we check bgp cancels
	state = c.SetBalancer(logger,
		"testsvc",
		svc,
		[]discovery.EndpointSlice{})
	if state != controllers.SyncStateSuccess {
		t.Fatalf("Set balancer failed")
	}
	if l2MockHandler.setBalancerCalled {
		t.Fatal("no handlers, l2 handler was called")
	}
	if bgpMockHandler.setBalancerCalled {
		t.Fatal("no handlers, bgp handler was called")
	}
	if !bgpMockHandler.deleteBalancerCalled {
		t.Fatal("no handlers, bgp delete handler was not called")
	}
	if _, ok := c.svcIPs["testsvc"]; ok {
		t.Fatal("no handlers, svc ip is not removed")
	}
	if c.announced[config.BGP]["testsvc"] {
		t.Fatal("no handlers, ip is announced in bgp")
	}
	if c.announced[config.Layer2]["testsvc"] {
		t.Fatal("no handlers, ip is announced in l2")
	}
}

func TestNodeAssignedEventOnlyOnChange(t *testing.T) {
	var l2MockHandler = &MockProtocol{
		protocol:       config.Layer2,
		shouldAnnounce: true,
	}

	var bgpMockHandler = &MockProtocol{
		protocol:       config.BGP,
		shouldAnnounce: false,
	}
	c := mockNewController(l2MockHandler, bgpMockHandler, t)
	client := c.client.(*testK8S)

	svc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "eventsvc",
		},
		Spec: v1.ServiceSpec{
			Type:                  "LoadBalancer",
			ExternalTrafficPolicy: "Cluster",
		},
		Status: statusAssigned("10.20.30.5"),
	}

	cfg := &config.Config{
		Pools: &config.Pools{ByName: map[string]*config.Pool{
			"default": {
				CIDR: []*net.IPNet{ipnet("10.20.30.0/24")},
			},
		}},
	}

	if state := c.SetConfig(logger, cfg); state != controllers.SyncStateReprocessAll {
		t.Fatalf("Set config failed")
	}

	setBalancer := func(desc string) {
		t.Helper()
		if state := c.SetBalancer(logger, "eventsvc", svc, []discovery.EndpointSlice{}); state != controllers.SyncStateSuccess {
			t.Fatalf("%s: set balancer failed", desc)
		}
	}
	expectEvents := func(desc string, want int) {
		t.Helper()
		if got := client.events["nodeAssigned"]; got != want {
			t.Fatalf("%s: expected %d nodeAssigned events, got %d", desc, want, got)
		}
	}

	// The first announcement from this node must emit the event.
	setBalancer("first announcement")
	expectEvents("first announcement", 1)

	// Reconciles that do not change the announcement must not emit it again. This
	// is the steady state of a service whose endpoints are rewritten on a timer by
	// an external controller.
	for i := 0; i < 5; i++ {
		setBalancer("repeated reconcile")
	}
	expectEvents("repeated reconcile", 1)

	// A change of the announced IPs is a real transition and must emit again.
	svc.Status = statusAssigned("10.20.30.6")
	setBalancer("changed load balancer ip")
	expectEvents("changed load balancer ip", 2)

	// Losing and then regaining the announcement is a real transition too.
	l2MockHandler.shouldAnnounce = false
	setBalancer("withdrawn")
	expectEvents("withdrawn", 2)

	l2MockHandler.shouldAnnounce = true
	setBalancer("announced again")
	expectEvents("announced again", 3)

	// Ownership moves to a different speaker: the node losing the announcement
	// withdraws it and stays quiet, and the node taking over emits its own event
	// naming itself.
	var otherL2MockHandler = &MockProtocol{
		protocol:       config.Layer2,
		shouldAnnounce: true,
	}

	var otherBGPMockHandler = &MockProtocol{
		protocol:       config.BGP,
		shouldAnnounce: false,
	}
	other := mockNewController(otherL2MockHandler, otherBGPMockHandler, t)
	other.myNode = "othernode"
	otherClient := other.client.(*testK8S)

	if state := other.SetConfig(logger, cfg); state != controllers.SyncStateReprocessAll {
		t.Fatalf("other node: set config failed")
	}

	l2MockHandler.shouldAnnounce = false
	setBalancer("ownership moved away")
	expectEvents("ownership moved away", 3)

	for i := 0; i < 3; i++ {
		if state := other.SetBalancer(logger, "eventsvc", svc, []discovery.EndpointSlice{}); state != controllers.SyncStateSuccess {
			t.Fatalf("other node: set balancer failed")
		}
	}
	if got := otherClient.events["nodeAssigned"]; got != 1 {
		t.Fatalf("ownership moved to other node: expected 1 nodeAssigned event, got %d", got)
	}
	if msg := otherClient.eventMsgs["nodeAssigned"][0]; !strings.Contains(msg, `"othernode"`) {
		t.Fatalf("ownership moved to other node: event does not name the new node: %s", msg)
	}
}

type MockProtocol struct {
	config               *config.Config
	protocol             config.Proto
	shouldAnnounce       bool
	setBalancerCalled    bool
	deleteBalancerCalled bool
}

func (m *MockProtocol) SetConfig(l log.Logger, c *config.Config) error {
	m.config = c
	return nil
}

func (m *MockProtocol) ShouldAnnounce(_ log.Logger, _ string, _ []net.IP, _ *config.Pool, _ *v1.Service, _ []discovery.EndpointSlice, _ map[string]*v1.Node) string {
	if m.shouldAnnounce {
		return ""
	}
	return "no announce"
}

func (m *MockProtocol) SetBalancer(_ log.Logger, _ string, _ []net.IP, _ *config.Pool, _ service, _ *v1.Service) error {
	m.setBalancerCalled = true
	return nil
}

func (m *MockProtocol) DeleteBalancer(_ log.Logger, _ string, _ string) error {
	m.deleteBalancerCalled = true
	return nil
}

func (m *MockProtocol) SetNode(_ log.Logger, _ *v1.Node) error {
	panic("not implemented") // TODO: Implement
}

func (m *MockProtocol) SetEventCallback(_ func(interface{})) {}

func (m *MockProtocol) reset() {
	m.deleteBalancerCalled = false
	m.setBalancerCalled = false
}
