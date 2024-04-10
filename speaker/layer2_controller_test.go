// SPDX-License-Identifier:Apache-2.0

package main

import (
	"fmt"
	"net"
	"os"
	"sort"
	"testing"

	"go.universe.tf/metallb/internal/config"
	"go.universe.tf/metallb/internal/k8s/controllers"
	"go.universe.tf/metallb/internal/layer2"

	"github.com/go-kit/log"
	v1 "k8s.io/api/core/v1"
	discovery "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
)

type fakeSpeakerList struct {
	speakers map[string]bool
}

func (sl *fakeSpeakerList) UsableSpeakers() map[string]bool {
	return sl.speakers
}

func (sl *fakeSpeakerList) Rejoin() {}

func compareUseableNodesReturnedValue(a, b []string) bool {
	if &a == &b {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if b[i] != v {
			return false
		}
	}
	return true
}

func TestUsableNodesEPSlices(t *testing.T) {
	b := &fakeBGP{
		t: t,
	}
	newBGP = b.NewSessionManager

	c, err := newController(controllerConfig{
		MyNode: "iris1",
		Logger: log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr)),
	})
	if err != nil {
		t.Fatalf("creating controller: %s", err)
	}
	c.client = &testK8S{t: t}

	tests := []struct {
		desc string

		eps []discovery.EndpointSlice

		usableSpeakers map[string]bool

		cExpectedResult []string
	}{
		{
			desc: "Two endpoints, different hosts, multi slice",
			eps: []discovery.EndpointSlice{
				{
					Endpoints: []discovery.Endpoint{
						{
							Addresses: []string{
								"2.3.4.5",
							},
							NodeName: ptr.To("iris1"),
							Conditions: discovery.EndpointConditions{
								Ready: ptr.To(true),
							},
						},
					},
				},
				{
					Endpoints: []discovery.Endpoint{
						{
							Addresses: []string{
								"2.3.4.15",
							},
							NodeName: ptr.To("iris2"),
							Conditions: discovery.EndpointConditions{
								Ready: ptr.To(true),
							},
						},
					},
				},
			},
			usableSpeakers:  map[string]bool{"iris1": true, "iris2": true},
			cExpectedResult: []string{"iris1", "iris2"},
		},
		{
			desc: "Two endpoints, different hosts",
			eps: []discovery.EndpointSlice{
				{
					Endpoints: []discovery.Endpoint{
						{
							Addresses: []string{
								"2.3.4.5",
							},
							NodeName: ptr.To("iris1"),
							Conditions: discovery.EndpointConditions{
								Ready: ptr.To(true),
							},
						},
						{
							Addresses: []string{
								"2.3.4.15",
							},
							NodeName: ptr.To("iris2"),
							Conditions: discovery.EndpointConditions{
								Ready: ptr.To(true),
							},
						},
					},
				},
			},
			usableSpeakers:  map[string]bool{"iris1": true, "iris2": true},
			cExpectedResult: []string{"iris1", "iris2"},
		},
		{
			desc: "Two endpoints, same host",
			eps: []discovery.EndpointSlice{
				{
					Endpoints: []discovery.Endpoint{
						{
							Addresses: []string{
								"2.3.4.5",
							},
							NodeName: ptr.To("iris1"),
							Conditions: discovery.EndpointConditions{
								Ready: ptr.To(true),
							},
						},
						{
							Addresses: []string{
								"2.3.4.15",
							},
							NodeName: ptr.To("iris1"),
							Conditions: discovery.EndpointConditions{
								Ready: ptr.To(true),
							},
						},
					},
				},
			},
			usableSpeakers:  map[string]bool{"iris1": true, "iris2": true},
			cExpectedResult: []string{"iris1"},
		},

		{
			desc: "Two endpoints, same host, one is not ready",
			eps: []discovery.EndpointSlice{
				{
					Endpoints: []discovery.Endpoint{
						{
							Addresses: []string{
								"2.3.4.5",
							},
							NodeName: ptr.To("iris1"),
							Conditions: discovery.EndpointConditions{
								Ready: ptr.To(true),
							},
						},
						{
							Addresses: []string{
								"2.3.4.15",
							},
							NodeName: ptr.To("iris1"),
							Conditions: discovery.EndpointConditions{
								Ready: ptr.To(false),
							},
						},
					},
				},
			},
			usableSpeakers:  map[string]bool{"iris1": true, "iris2": true},
			cExpectedResult: []string{"iris1"},
		},
		{
			desc: "Two endpoints, different hosts, not ready but serving",
			eps: []discovery.EndpointSlice{
				{
					Endpoints: []discovery.Endpoint{
						{
							Addresses: []string{
								"2.3.4.5",
							},
							NodeName: ptr.To("iris1"),
							Conditions: discovery.EndpointConditions{
								Ready:   ptr.To(false),
								Serving: ptr.To(true),
							},
						},
						{
							Addresses: []string{
								"2.3.4.15",
							},
							NodeName: ptr.To("iris2"),
							Conditions: discovery.EndpointConditions{
								Ready:   ptr.To(false),
								Serving: ptr.To(true),
							},
						},
					},
				},
			},
			usableSpeakers:  map[string]bool{"iris1": true, "iris2": true},
			cExpectedResult: []string{"iris1", "iris2"},
		},
		{
			desc: "Two endpoints, different hosts, ready but not serving",
			eps: []discovery.EndpointSlice{
				{
					Endpoints: []discovery.Endpoint{
						{
							Addresses: []string{
								"2.3.4.5",
							},
							NodeName: ptr.To("iris1"),
							Conditions: discovery.EndpointConditions{
								Ready:   ptr.To(true),
								Serving: ptr.To(false),
							},
						},
						{
							Addresses: []string{
								"2.3.4.15",
							},
							NodeName: ptr.To("iris2"),
							Conditions: discovery.EndpointConditions{
								Ready:   ptr.To(true),
								Serving: ptr.To(false),
							},
						},
					},
				},
			},
			usableSpeakers:  map[string]bool{"iris1": true, "iris2": true},
			cExpectedResult: []string{"iris1", "iris2"},
		},
	}

	for _, test := range tests {
		response := usableNodes(test.eps, test.usableSpeakers)
		sort.Strings(response)
		if !compareUseableNodesReturnedValue(response, test.cExpectedResult) {
			t.Errorf("%q: shouldAnnounce for controller returned incorrect result, expected '%s', but received '%s'", test.desc, test.cExpectedResult, response)
		}
	}
}

func TestShouldAnnounceEPSlices(t *testing.T) {
	fakeSL := &fakeSpeakerList{
		speakers: map[string]bool{
			"iris1": true,
			"iris2": true,
		},
	}
	c1, err := newController(controllerConfig{
		MyNode: "iris1",
		Logger: log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr)),
		SList:  fakeSL,
	})
	if err != nil {
		t.Fatalf("creating controller: %s", err)
	}
	c1.client = &testK8S{t: t}

	c2, err := newController(controllerConfig{
		MyNode: "iris2",
		Logger: log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr)),
		SList:  fakeSL,
	})
	if err != nil {
		t.Fatalf("creating controller: %s", err)
	}
	c2.client = &testK8S{t: t}
	advertisementsForNode := []*config.L2Advertisement{
		{
			Nodes: map[string]bool{
				"iris1": true,
				"iris2": true,
			},
		},
	}

	tests := []struct {
		desc string

		balancer string
		config   *config.Config
		svcs     []*v1.Service
		eps      map[string][]discovery.EndpointSlice

		c1ExpectedResult map[string]string
		c2ExpectedResult map[string]string
	}{
		{
			desc:     "One service, two endpoints, one host, controller 1 should announce",
			balancer: "test1",
			config: &config.Config{
				Pools: &config.Pools{ByName: map[string]*config.Pool{
					"default": {
						CIDR:             []*net.IPNet{ipnet("10.20.30.0/24")},
						L2Advertisements: advertisementsForNode,
					},
				}},
			},
			svcs: []*v1.Service{
				{
					Spec: v1.ServiceSpec{
						Type:                  "LoadBalancer",
						ExternalTrafficPolicy: v1.ServiceExternalTrafficPolicyTypeLocal,
					},
					Status: statusAssigned("10.20.30.1"),
				},
			},
			eps: map[string][]discovery.EndpointSlice{
				"10.20.30.1": {
					{
						Endpoints: []discovery.Endpoint{
							{
								Addresses: []string{
									"2.3.4.5",
								},
								NodeName: ptr.To("iris1"),
								Conditions: discovery.EndpointConditions{
									Ready: ptr.To(true),
								},
							},
							{
								Addresses: []string{
									"2.3.4.15",
								},
								NodeName: ptr.To("iris1"),
								Conditions: discovery.EndpointConditions{
									Ready: ptr.To(true),
								},
							},
						},
					},
				},
			},
			c1ExpectedResult: map[string]string{
				"10.20.30.1": "",
			},
			c2ExpectedResult: map[string]string{
				"10.20.30.1": "notOwner",
			},
		}, {
			desc: "One service, two endpoints, one host, neither endpoint is ready, no controller should announce",
			config: &config.Config{
				Pools: &config.Pools{ByName: map[string]*config.Pool{
					"default": {
						CIDR:             []*net.IPNet{ipnet("10.20.30.0/24")},
						L2Advertisements: advertisementsForNode,
					},
				}},
			},
			svcs: []*v1.Service{
				{
					Spec: v1.ServiceSpec{
						Type:                  "LoadBalancer",
						ExternalTrafficPolicy: v1.ServiceExternalTrafficPolicyTypeLocal,
					},
					Status: statusAssigned("10.20.30.1"),
				},
			},
			eps: map[string][]discovery.EndpointSlice{
				"10.20.30.1": {
					{
						Endpoints: []discovery.Endpoint{
							{
								Addresses: []string{
									"2.3.4.5",
								},
								NodeName: ptr.To("iris1"),
								Conditions: discovery.EndpointConditions{
									Ready: ptr.To(false),
								},
							},
							{
								Addresses: []string{
									"2.3.4.15",
								},
								NodeName: ptr.To("iris1"),
								Conditions: discovery.EndpointConditions{
									Ready: ptr.To(false),
								},
							},
						},
					},
				},
			},
			c1ExpectedResult: map[string]string{
				"10.20.30.1": "notOwner",
			},
			c2ExpectedResult: map[string]string{
				"10.20.30.1": "notOwner",
			},
		}, {
			desc: "One service, two endpoints across two hosts, controller2 should announce",
			config: &config.Config{
				Pools: &config.Pools{ByName: map[string]*config.Pool{
					"default": {
						CIDR:             []*net.IPNet{ipnet("10.20.30.0/24")},
						L2Advertisements: advertisementsForNode,
					},
				}},
			},
			svcs: []*v1.Service{
				{
					Spec: v1.ServiceSpec{
						Type:                  "LoadBalancer",
						ExternalTrafficPolicy: v1.ServiceExternalTrafficPolicyTypeLocal,
					},
					Status: statusAssigned("10.20.30.1"),
				},
			},
			eps: map[string][]discovery.EndpointSlice{
				"10.20.30.1": {
					{
						Endpoints: []discovery.Endpoint{
							{
								Addresses: []string{
									"2.3.4.5",
								},
								NodeName: ptr.To("iris1"),
								Conditions: discovery.EndpointConditions{
									Ready: ptr.To(true),
								},
							},
							{
								Addresses: []string{
									"2.3.4.15",
								},
								NodeName: ptr.To("iris2"),
								Conditions: discovery.EndpointConditions{
									Ready:   ptr.To(false),
									Serving: ptr.To(true),
								},
							},
						},
					},
				},
			},
			c1ExpectedResult: map[string]string{
				"10.20.30.1": "notOwner",
			},
			c2ExpectedResult: map[string]string{
				"10.20.30.1": "",
			},
		}, {
			desc: "One service, two endpoints across two hosts, neither endpoint is ready, no controllers should announce",
			config: &config.Config{
				Pools: &config.Pools{ByName: map[string]*config.Pool{
					"default": {
						CIDR:             []*net.IPNet{ipnet("10.20.30.0/24")},
						L2Advertisements: advertisementsForNode,
					},
				}},
			},
			svcs: []*v1.Service{
				{
					Spec: v1.ServiceSpec{
						Type:                  "LoadBalancer",
						ExternalTrafficPolicy: v1.ServiceExternalTrafficPolicyTypeLocal,
					},
					Status: statusAssigned("10.20.30.1"),
				},
			},
			eps: map[string][]discovery.EndpointSlice{
				"10.20.30.1": {
					{
						Endpoints: []discovery.Endpoint{
							{
								Addresses: []string{
									"2.3.4.5",
								},
								NodeName: ptr.To("iris1"),
								Conditions: discovery.EndpointConditions{
									Ready: ptr.To(false),
								},
							},
							{
								Addresses: []string{
									"2.3.4.15",
								},
								NodeName: ptr.To("iris2"),
								Conditions: discovery.EndpointConditions{
									Ready: ptr.To(false),
								},
							},
						},
					},
				},
			},
			c1ExpectedResult: map[string]string{
				"10.20.30.1": "notOwner",
			},
			c2ExpectedResult: map[string]string{
				"10.20.30.1": "notOwner",
			},
		}, {
			desc: "One service, two endpoints across two hosts, controller 2 is not ready, controller 1 should announce",
			config: &config.Config{
				Pools: &config.Pools{ByName: map[string]*config.Pool{
					"default": {
						CIDR:             []*net.IPNet{ipnet("10.20.30.0/24")},
						L2Advertisements: advertisementsForNode,
					},
				}},
			},
			svcs: []*v1.Service{
				{
					Spec: v1.ServiceSpec{
						Type:                  "LoadBalancer",
						ExternalTrafficPolicy: v1.ServiceExternalTrafficPolicyTypeLocal,
					},
					Status: statusAssigned("10.20.30.1"),
				},
			},
			eps: map[string][]discovery.EndpointSlice{
				"10.20.30.1": {
					{
						Endpoints: []discovery.Endpoint{
							{
								Addresses: []string{
									"2.3.4.5",
								},
								NodeName: ptr.To("iris1"),
								Conditions: discovery.EndpointConditions{
									Ready: ptr.To(true),
								},
							},
							{
								Addresses: []string{
									"2.3.4.15",
								},
								NodeName: ptr.To("iris2"),
								Conditions: discovery.EndpointConditions{
									Ready: ptr.To(false),
								},
							},
						},
					},
				},
			},
			c1ExpectedResult: map[string]string{
				"10.20.30.1": "",
			},
			c2ExpectedResult: map[string]string{
				"10.20.30.1": "notOwner",
			},
		}, {
			desc: "Two services each with two endpoints across across two hosts, each controller should announce one service",
			config: &config.Config{
				Pools: &config.Pools{ByName: map[string]*config.Pool{
					"default": {
						CIDR:             []*net.IPNet{ipnet("10.20.30.0/24")},
						L2Advertisements: advertisementsForNode,
					},
				}},
			},
			svcs: []*v1.Service{
				{
					Spec: v1.ServiceSpec{
						Type:                  "LoadBalancer",
						ExternalTrafficPolicy: v1.ServiceExternalTrafficPolicyTypeLocal,
					},
					Status: statusAssigned("10.20.30.1"),
				},
				{
					Spec: v1.ServiceSpec{
						Type:                  "LoadBalancer",
						ExternalTrafficPolicy: v1.ServiceExternalTrafficPolicyTypeLocal,
					},
					Status: statusAssigned("10.20.30.2"),
				},
			},
			eps: map[string][]discovery.EndpointSlice{
				"10.20.30.1": {
					{
						Endpoints: []discovery.Endpoint{
							{
								Addresses: []string{
									"2.3.4.5",
								},
								NodeName: ptr.To("iris1"),
								Conditions: discovery.EndpointConditions{
									Ready: ptr.To(true),
								},
							},
							{
								Addresses: []string{
									"2.3.4.15",
								},
								NodeName: ptr.To("iris2"),
								Conditions: discovery.EndpointConditions{
									Ready: ptr.To(true),
								},
							},
						},
					},
				},
				"10.20.30.2": {
					{
						Endpoints: []discovery.Endpoint{
							{
								Addresses: []string{
									"2.3.4.25",
								},
								NodeName: ptr.To("iris1"),
								Conditions: discovery.EndpointConditions{
									Ready: ptr.To(true),
								},
							},
							{
								Addresses: []string{
									"2.3.4.35",
								},
								NodeName: ptr.To("iris2"),
								Conditions: discovery.EndpointConditions{
									Ready: ptr.To(true),
								},
							},
						},
					},
				},
			},
			c1ExpectedResult: map[string]string{
				"10.20.30.1": "notOwner",
				"10.20.30.2": "",
			},
			c2ExpectedResult: map[string]string{
				"10.20.30.1": "",
				"10.20.30.2": "notOwner",
			},
		}, {
			desc: "Two services each with two endpoints across across two hosts, one service has an endpoint not ready on controller 2, each controller should announce one service",
			config: &config.Config{
				Pools: &config.Pools{ByName: map[string]*config.Pool{
					"default": {
						CIDR:             []*net.IPNet{ipnet("10.20.30.0/24")},
						L2Advertisements: advertisementsForNode,
					},
				}},
			},
			svcs: []*v1.Service{
				{
					Spec: v1.ServiceSpec{
						Type:                  "LoadBalancer",
						ExternalTrafficPolicy: v1.ServiceExternalTrafficPolicyTypeLocal,
					},
					Status: statusAssigned("10.20.30.1"),
				},
				{
					Spec: v1.ServiceSpec{
						Type:                  "LoadBalancer",
						ExternalTrafficPolicy: v1.ServiceExternalTrafficPolicyTypeLocal,
					},
					Status: statusAssigned("10.20.30.2"),
				},
			},
			eps: map[string][]discovery.EndpointSlice{
				"10.20.30.1": {
					{
						Endpoints: []discovery.Endpoint{
							{
								Addresses: []string{
									"2.3.4.5",
								},
								NodeName: ptr.To("iris1"),
								Conditions: discovery.EndpointConditions{
									Ready: ptr.To(true),
								},
							},
							{
								Addresses: []string{
									"2.3.4.15",
								},
								NodeName: ptr.To("iris2"),
								Conditions: discovery.EndpointConditions{
									Ready: ptr.To(true),
								},
							},
						},
					},
				},
				"10.20.30.2": {
					{
						Endpoints: []discovery.Endpoint{
							{
								Addresses: []string{
									"2.3.4.25",
								},
								NodeName: ptr.To("iris1"),
								Conditions: discovery.EndpointConditions{
									Ready: ptr.To(true),
								},
							},
							{
								Addresses: []string{
									"2.3.4.35",
								},
								NodeName: ptr.To("iris2"),
								Conditions: discovery.EndpointConditions{
									Ready: ptr.To(false),
								},
							},
						},
					},
				},
			},
			c1ExpectedResult: map[string]string{
				"10.20.30.1": "notOwner",
				"10.20.30.2": "",
			},
			c2ExpectedResult: map[string]string{
				"10.20.30.1": "",
				"10.20.30.2": "notOwner",
			},
		}, {
			desc: "Two services each with two endpoints across across two hosts, one service has an endpoint not ready on controller 1, the other service has an endpoint not ready on controller 2. Each controller should announce for the service with the ready endpoint on that controller",
			config: &config.Config{
				Pools: &config.Pools{ByName: map[string]*config.Pool{
					"default": {
						CIDR:             []*net.IPNet{ipnet("10.20.30.0/24")},
						L2Advertisements: advertisementsForNode,
					},
				}},
			},
			svcs: []*v1.Service{
				{
					Spec: v1.ServiceSpec{
						Type:                  "LoadBalancer",
						ExternalTrafficPolicy: v1.ServiceExternalTrafficPolicyTypeLocal,
					},
					Status: statusAssigned("10.20.30.1"),
				},
				{
					Spec: v1.ServiceSpec{
						Type:                  "LoadBalancer",
						ExternalTrafficPolicy: v1.ServiceExternalTrafficPolicyTypeLocal,
					},
					Status: statusAssigned("10.20.30.2"),
				},
			},
			eps: map[string][]discovery.EndpointSlice{
				"10.20.30.1": {
					{
						Endpoints: []discovery.Endpoint{
							{
								Addresses: []string{
									"2.3.4.5",
								},
								NodeName: ptr.To("iris2"),
								Conditions: discovery.EndpointConditions{
									Ready: ptr.To(true),
								},
							},
							{
								Addresses: []string{
									"2.3.4.15",
								},
								NodeName: ptr.To("iris1"),
								Conditions: discovery.EndpointConditions{
									Ready: ptr.To(false),
								},
							},
						},
					},
				},
				"10.20.30.2": {
					{
						Endpoints: []discovery.Endpoint{
							{
								Addresses: []string{
									"2.3.4.25",
								},
								NodeName: ptr.To("iris1"),
								Conditions: discovery.EndpointConditions{
									Ready: ptr.To(true),
								},
							},
							{
								Addresses: []string{
									"2.3.4.35",
								},
								NodeName: ptr.To("iris2"),
								Conditions: discovery.EndpointConditions{
									Ready: ptr.To(false),
								},
							},
						},
					},
				},
			},
			c1ExpectedResult: map[string]string{
				"10.20.30.1": "notOwner",
				"10.20.30.2": "",
			},
			c2ExpectedResult: map[string]string{
				"10.20.30.1": "",
				"10.20.30.2": "notOwner",
			},
		}, {
			desc: "One service with three endpoints across across two hosts, controller 2 hosts two endpoints controller 2 should announce for the service",
			config: &config.Config{
				Pools: &config.Pools{ByName: map[string]*config.Pool{
					"default": {
						CIDR:             []*net.IPNet{ipnet("10.20.30.0/24")},
						L2Advertisements: advertisementsForNode,
					},
				}},
			},
			svcs: []*v1.Service{
				{
					Spec: v1.ServiceSpec{
						Type:                  "LoadBalancer",
						ExternalTrafficPolicy: v1.ServiceExternalTrafficPolicyTypeLocal,
					},
					Status: statusAssigned("10.20.30.1"),
				},
			},
			eps: map[string][]discovery.EndpointSlice{
				"10.20.30.1": {
					{
						Endpoints: []discovery.Endpoint{
							{
								Addresses: []string{
									"2.3.4.5",
								},
								NodeName: ptr.To("iris1"),
								Conditions: discovery.EndpointConditions{
									Ready: ptr.To(true),
								},
							},
							{
								Addresses: []string{
									"2.3.4.15",
								},
								NodeName: ptr.To("iris2"),
								Conditions: discovery.EndpointConditions{
									Ready: ptr.To(true),
								},
							},
						},
					},
					{
						Endpoints: []discovery.Endpoint{
							{
								Addresses: []string{
									"2.3.4.25",
								},
								NodeName: ptr.To("iris2"),
								Conditions: discovery.EndpointConditions{
									Ready: ptr.To(true),
								},
							},
						},
					},
				},
			},
			c1ExpectedResult: map[string]string{
				"10.20.30.1": "notOwner",
			},
			c2ExpectedResult: map[string]string{
				"10.20.30.1": "",
			},
		}, {
			desc: "One service with three endpoints across across two hosts, controller 1 hosts two endpoints controller 2 should announce for the service",
			config: &config.Config{
				Pools: &config.Pools{ByName: map[string]*config.Pool{
					"default": {
						CIDR:             []*net.IPNet{ipnet("10.20.30.0/24")},
						L2Advertisements: advertisementsForNode,
					},
				}},
			},
			svcs: []*v1.Service{
				{
					Spec: v1.ServiceSpec{
						Type:                  "LoadBalancer",
						ExternalTrafficPolicy: v1.ServiceExternalTrafficPolicyTypeLocal,
					},
					Status: statusAssigned("10.20.30.1"),
				},
			},
			eps: map[string][]discovery.EndpointSlice{
				"10.20.30.1": {
					{
						Endpoints: []discovery.Endpoint{
							{
								Addresses: []string{
									"2.3.4.5",
								},
								NodeName: ptr.To("iris1"),
								Conditions: discovery.EndpointConditions{
									Ready: ptr.To(true),
								},
							},
							{
								Addresses: []string{
									"2.3.4.15",
								},
								NodeName: ptr.To("iris2"),
								Conditions: discovery.EndpointConditions{
									Ready: ptr.To(true),
								},
							},
							{
								Addresses: []string{
									"2.3.4.25",
								},
								NodeName: ptr.To("iris1"),
								Conditions: discovery.EndpointConditions{
									Ready: ptr.To(true),
								},
							},
						},
					},
				},
			},
			c1ExpectedResult: map[string]string{
				"10.20.30.1": "notOwner",
			},
			c2ExpectedResult: map[string]string{
				"10.20.30.1": "",
			},
		}, {
			desc: "One service with three endpoints across across two hosts, controller 2 hosts two endpoints, one of which is not ready, controller 2 should announce for the service",
			config: &config.Config{
				Pools: &config.Pools{ByName: map[string]*config.Pool{
					"default": {
						CIDR:             []*net.IPNet{ipnet("10.20.30.0/24")},
						L2Advertisements: advertisementsForNode,
					},
				}},
			},
			svcs: []*v1.Service{
				{
					Spec: v1.ServiceSpec{
						Type:                  "LoadBalancer",
						ExternalTrafficPolicy: v1.ServiceExternalTrafficPolicyTypeLocal,
					},
					Status: statusAssigned("10.20.30.1"),
				},
			},
			eps: map[string][]discovery.EndpointSlice{
				"10.20.30.1": {
					{
						Endpoints: []discovery.Endpoint{
							{
								Addresses: []string{
									"2.3.4.5",
								},
								NodeName: ptr.To("iris1"),
								Conditions: discovery.EndpointConditions{
									Ready: ptr.To(true),
								},
							},
							{
								Addresses: []string{
									"2.3.4.15",
								},
								NodeName: ptr.To("iris2"),
								Conditions: discovery.EndpointConditions{
									Ready: ptr.To(true),
								},
							},
							{
								Addresses: []string{
									"2.3.4.25",
								},
								NodeName: ptr.To("iris2"),
								Conditions: discovery.EndpointConditions{
									Ready: ptr.To(true),
								},
							},
						},
					},
				},
			},
			c1ExpectedResult: map[string]string{
				"10.20.30.1": "notOwner",
			},
			c2ExpectedResult: map[string]string{
				"10.20.30.1": "",
			},
		}, {
			desc: "One service with three endpoints across across two hosts, controller 1 hosts two endpoints, one of which is not ready, controller 1 should announce for the service",
			config: &config.Config{
				Pools: &config.Pools{ByName: map[string]*config.Pool{
					"default": {
						CIDR:             []*net.IPNet{ipnet("10.20.30.0/24")},
						L2Advertisements: advertisementsForNode,
					},
				}},
			},
			svcs: []*v1.Service{
				{
					Spec: v1.ServiceSpec{
						Type:                  "LoadBalancer",
						ExternalTrafficPolicy: v1.ServiceExternalTrafficPolicyTypeLocal,
					},
					Status: statusAssigned("10.20.30.1"),
				},
			},
			eps: map[string][]discovery.EndpointSlice{
				"10.20.30.1": {
					{
						Endpoints: []discovery.Endpoint{
							{
								Addresses: []string{
									"2.3.4.5",
								},
								NodeName: ptr.To("iris1"),
								Conditions: discovery.EndpointConditions{
									Ready: ptr.To(true),
								},
							},
							{
								Addresses: []string{
									"2.3.4.15",
								},
								NodeName: ptr.To("iris2"),
								Conditions: discovery.EndpointConditions{
									Ready: ptr.To(true),
								},
							},
							{
								Addresses: []string{
									"2.3.4.25",
								},
								NodeName: ptr.To("iris1"),
								Conditions: discovery.EndpointConditions{
									Ready: ptr.To(false),
								},
							},
						},
					},
				},
			},
			c1ExpectedResult: map[string]string{
				"10.20.30.1": "notOwner",
			},
			c2ExpectedResult: map[string]string{
				"10.20.30.1": "",
			},
		}, {
			desc: "One service with three endpoints across across two hosts, controller 2 hosts two endpoints, both of which are not ready, controller 1 should announce for the service",
			config: &config.Config{
				Pools: &config.Pools{ByName: map[string]*config.Pool{
					"default": {
						CIDR:             []*net.IPNet{ipnet("10.20.30.0/24")},
						L2Advertisements: advertisementsForNode,
					},
				}},
			},
			svcs: []*v1.Service{
				{
					Spec: v1.ServiceSpec{
						Type:                  "LoadBalancer",
						ExternalTrafficPolicy: v1.ServiceExternalTrafficPolicyTypeLocal,
					},
					Status: statusAssigned("10.20.30.1"),
				},
			},
			eps: map[string][]discovery.EndpointSlice{
				"10.20.30.1": {
					{
						Endpoints: []discovery.Endpoint{
							{
								Addresses: []string{
									"2.3.4.5",
								},
								NodeName: ptr.To("iris1"),
								Conditions: discovery.EndpointConditions{
									Ready: ptr.To(true),
								},
							},
							{
								Addresses: []string{
									"2.3.4.15",
								},
								NodeName: ptr.To("iris2"),
								Conditions: discovery.EndpointConditions{
									Ready: ptr.To(false),
								},
							},
							{
								Addresses: []string{
									"2.3.4.25",
								},
								NodeName: ptr.To("iris2"),
								Conditions: discovery.EndpointConditions{
									Ready: ptr.To(false),
								},
							},
						},
					},
				},
			},
			c1ExpectedResult: map[string]string{
				"10.20.30.1": "",
			},
			c2ExpectedResult: map[string]string{
				"10.20.30.1": "notOwner",
			},
		},
	}

	l := log.NewNopLogger()
	for _, test := range tests {
		if test.config != nil {
			if c1.SetConfig(l, test.config) == controllers.SyncStateError {
				t.Errorf("%q: SetConfig failed", test.desc)
			}
			if c2.SetConfig(l, test.config) == controllers.SyncStateError {
				t.Errorf("%q: SetConfig failed", test.desc)
			}
		}

		for _, svc := range test.svcs {
			lbIP := net.ParseIP(svc.Status.LoadBalancer.Ingress[0].IP)
			lbIPStr := lbIP.String()
			response1 := c1.protocolHandlers[config.Layer2].ShouldAnnounce(l, "test1", []net.IP{lbIP}, test.config.Pools.ByName["default"], svc, test.eps[lbIPStr], nil)
			response2 := c2.protocolHandlers[config.Layer2].ShouldAnnounce(l, "test1", []net.IP{lbIP}, test.config.Pools.ByName["default"], svc, test.eps[lbIPStr], nil)
			if response1 != test.c1ExpectedResult[lbIPStr] {
				t.Errorf("%q: shouldAnnounce for controller 1 for service %s returned incorrect result, expected '%s', but received '%s'", test.desc, lbIPStr, test.c1ExpectedResult[lbIPStr], response1)
			}
			if response2 != test.c2ExpectedResult[lbIPStr] {
				t.Errorf("%q: shouldAnnounce for controller 2 for service %s returned incorrect result, expected '%s', but received '%s'", test.desc, lbIPStr, test.c2ExpectedResult[lbIPStr], response2)
			}
		}
	}
}

func TestShouldAnnounceFromNodes(t *testing.T) {
	fakeSL := &fakeSpeakerList{
		speakers: map[string]bool{
			"iris1": true,
			"iris2": true,
		},
	}
	advertisementsForBoth := []*config.L2Advertisement{
		{
			Nodes: map[string]bool{
				"iris1": true,
				"iris2": true,
			},
		},
	}

	advertisementOnIris1 := []*config.L2Advertisement{
		{
			Nodes: map[string]bool{
				"iris1": true,
			},
		},
	}

	advertisementSplit := []*config.L2Advertisement{
		{
			Nodes: map[string]bool{
				"iris1": true,
			},
		},
		{
			Nodes: map[string]bool{
				"iris2": true,
			},
		},
	}

	advertisementOnIris2 := []*config.L2Advertisement{
		{
			Nodes: map[string]bool{
				"iris2": true,
			},
		},
	}

	epsOnBothNodes := map[string][]discovery.EndpointSlice{
		"10.20.30.1": {
			{
				Endpoints: []discovery.Endpoint{
					{
						Addresses: []string{
							"2.3.4.5",
						},
						NodeName: ptr.To("iris1"),
						Conditions: discovery.EndpointConditions{
							Ready: ptr.To(true),
						},
					},
					{
						Addresses: []string{
							"2.3.4.15",
						},
						NodeName: ptr.To("iris2"),
						Conditions: discovery.EndpointConditions{
							Ready: ptr.To(true),
						},
					},
				},
			},
		},
	}

	epsOn := func(node string) map[string][]discovery.EndpointSlice {
		return map[string][]discovery.EndpointSlice{
			"10.20.30.1": {
				{
					Endpoints: []discovery.Endpoint{
						{
							Addresses: []string{
								"2.3.4.5",
							},
							NodeName: ptr.To(node),
							Conditions: discovery.EndpointConditions{
								Ready: ptr.To(true),
							},
						},
					},
				},
			},
		}
	}

	tests := []struct {
		desc string

		balancer            string
		L2Advertisements    []*config.L2Advertisement
		eps                 map[string][]discovery.EndpointSlice
		trafficPolicy       v1.ServiceExternalTrafficPolicyType
		excludeFromLB       []string
		ignoreExcludeFromLB bool
		c1ExpectedResult    map[string]string
		c2ExpectedResult    map[string]string
	}{
		{
			desc:             "One service, endpoint on iris1, selector on iris1, c1 should announce",
			balancer:         "test1",
			eps:              epsOn("iris1"),
			L2Advertisements: advertisementOnIris1,
			trafficPolicy:    v1.ServiceExternalTrafficPolicyTypeCluster,
			c1ExpectedResult: map[string]string{
				"10.20.30.1": "",
			},
			c2ExpectedResult: map[string]string{
				"10.20.30.1": "notOwner",
			},
		},
		{
			desc:             "One service, etp local, endpoint on iris1, selector on both(split), c2 should announce",
			balancer:         "test1",
			eps:              epsOn("iris1"),
			L2Advertisements: advertisementSplit,
			trafficPolicy:    v1.ServiceExternalTrafficPolicyTypeLocal,
			c1ExpectedResult: map[string]string{
				"10.20.30.1": "",
			},
			c2ExpectedResult: map[string]string{
				"10.20.30.1": "notOwner",
			},
		},
		{
			desc:             "One service, endpoint on iris1, selector on both, c2 should announce",
			balancer:         "test1",
			eps:              epsOn("iris1"),
			L2Advertisements: advertisementsForBoth,
			trafficPolicy:    v1.ServiceExternalTrafficPolicyTypeCluster,
			c1ExpectedResult: map[string]string{
				"10.20.30.1": "notOwner",
			},
			c2ExpectedResult: map[string]string{
				"10.20.30.1": "",
			},
		},
		{
			desc:             "One service, endpoint on both nodes, advertisement on iris1, c1 should announce",
			balancer:         "test1",
			eps:              epsOnBothNodes,
			L2Advertisements: advertisementOnIris1,
			trafficPolicy:    v1.ServiceExternalTrafficPolicyTypeCluster,
			c1ExpectedResult: map[string]string{
				"10.20.30.1": "",
			},
			c2ExpectedResult: map[string]string{
				"10.20.30.1": "notOwner",
			},
		},
		{
			desc:             "One service, ltp endpoint on iris1, advertisement on iris2, no one should announce",
			balancer:         "test1",
			eps:              epsOn("iris1"),
			L2Advertisements: advertisementOnIris2,
			trafficPolicy:    v1.ServiceExternalTrafficPolicyTypeLocal,
			c1ExpectedResult: map[string]string{
				"10.20.30.1": "notOwner",
			},
			c2ExpectedResult: map[string]string{
				"10.20.30.1": "notOwner",
			},
		},
		{
			desc:             "One service, endpoint on iris1, no selector, iris2 excluded, c1 should announce",
			balancer:         "test1",
			eps:              epsOn("iris1"),
			L2Advertisements: advertisementsForBoth,
			trafficPolicy:    v1.ServiceExternalTrafficPolicyTypeCluster,
			excludeFromLB:    []string{"iris2"},
			c1ExpectedResult: map[string]string{
				"10.20.30.1": "",
			},
			c2ExpectedResult: map[string]string{
				"10.20.30.1": "notOwner",
			},
		},
		{
			desc:             "One service, endpoint on iris1, no selector, iris1 excluded, c2 should announce",
			balancer:         "test1",
			eps:              epsOn("iris1"),
			L2Advertisements: advertisementsForBoth,
			trafficPolicy:    v1.ServiceExternalTrafficPolicyTypeCluster,
			excludeFromLB:    []string{"iris1"},
			c1ExpectedResult: map[string]string{
				"10.20.30.1": "notOwner",
			},
			c2ExpectedResult: map[string]string{
				"10.20.30.1": "",
			},
		},
		{
			desc:             "One service, endpoint on iris1, no selector, both excluded, both should not announce",
			balancer:         "test1",
			eps:              epsOn("iris1"),
			L2Advertisements: advertisementsForBoth,
			trafficPolicy:    v1.ServiceExternalTrafficPolicyTypeCluster,
			excludeFromLB:    []string{"iris1", "iris2"},
			c1ExpectedResult: map[string]string{
				"10.20.30.1": "notOwner",
			},
			c2ExpectedResult: map[string]string{
				"10.20.30.1": "notOwner",
			},
		},
		{
			desc:             "One service, endpoint on iris1, no selector, etplocal, both excluded, ignore excludelb, c1 should announce",
			balancer:         "test1",
			eps:              epsOn("iris1"),
			L2Advertisements: advertisementsForBoth,
			trafficPolicy:    v1.ServiceExternalTrafficPolicyTypeLocal,
			excludeFromLB:    []string{"iris1", "iris2"},
			c1ExpectedResult: map[string]string{
				"10.20.30.1": "",
			},
			c2ExpectedResult: map[string]string{
				"10.20.30.1": "notOwner",
			},
			ignoreExcludeFromLB: true,
		},
	}
	l := log.NewNopLogger()
	for _, test := range tests {
		cfg := config.Config{
			Pools: &config.Pools{ByName: map[string]*config.Pool{
				"default": {
					CIDR:             []*net.IPNet{ipnet("10.20.30.0/24")},
					L2Advertisements: test.L2Advertisements,
				},
			}},
		}
		c1, err := newController(controllerConfig{
			MyNode:          "iris1",
			Logger:          log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr)),
			SList:           fakeSL,
			bgpType:         bgpNative,
			IgnoreExcludeLB: test.ignoreExcludeFromLB,
		})
		if err != nil {
			t.Fatalf("creating controller: %s", err)
		}
		c1.client = &testK8S{t: t}

		c2, err := newController(controllerConfig{
			MyNode:          "iris2",
			Logger:          log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr)),
			SList:           fakeSL,
			bgpType:         bgpNative,
			IgnoreExcludeLB: test.ignoreExcludeFromLB,
		})
		if err != nil {
			t.Fatalf("creating controller: %s", err)
		}
		c2.client = &testK8S{t: t}

		if c1.SetConfig(l, &cfg) == controllers.SyncStateError {
			t.Errorf("%q: SetConfig failed", test.desc)
		}
		if c2.SetConfig(l, &cfg) == controllers.SyncStateError {
			t.Errorf("%q: SetConfig failed", test.desc)
		}
		svc := v1.Service{
			Spec: v1.ServiceSpec{
				Type:                  "LoadBalancer",
				ExternalTrafficPolicy: test.trafficPolicy,
			},
			Status: statusAssigned("10.20.30.1"),
		}

		lbIP := net.ParseIP(svc.Status.LoadBalancer.Ingress[0].IP)
		lbIPStr := lbIP.String()

		nodes := map[string]*v1.Node{
			"iris1": {
				ObjectMeta: metav1.ObjectMeta{
					Name: "iris1",
				},
			},
			"iris2": {
				ObjectMeta: metav1.ObjectMeta{
					Name: "iris2",
				},
			},
		}
		for _, n := range test.excludeFromLB {
			nodes[n].Labels = map[string]string{
				v1.LabelNodeExcludeBalancers: "",
			}
		}

		response1 := c1.protocolHandlers[config.Layer2].ShouldAnnounce(l, "test1", []net.IP{lbIP}, cfg.Pools.ByName["default"], &svc, test.eps[lbIPStr], nodes)
		response2 := c2.protocolHandlers[config.Layer2].ShouldAnnounce(l, "test1", []net.IP{lbIP}, cfg.Pools.ByName["default"], &svc, test.eps[lbIPStr], nodes)
		if response1 != test.c1ExpectedResult[lbIPStr] {
			t.Errorf("%q: shouldAnnounce for controller 1 for service %s returned incorrect result, expected '%s', but received '%s'", test.desc, lbIPStr, test.c1ExpectedResult[lbIPStr], response1)
		}
		if response2 != test.c2ExpectedResult[lbIPStr] {
			t.Errorf("%q: shouldAnnounce for controller 2 for service %s returned incorrect result, expected '%s', but received '%s'", test.desc, lbIPStr, test.c2ExpectedResult[lbIPStr], response2)
		}
	}
}

func TestClusterPolicy(t *testing.T) {
	fakeSL := &fakeSpeakerList{
		speakers: map[string]bool{
			"iris1": true,
			"iris2": true,
		},
	}
	c1, err := newController(controllerConfig{
		MyNode: "iris1",
		Logger: log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr)),
		SList:  fakeSL,
	})
	if err != nil {
		t.Fatalf("creating controller: %s", err)
	}
	c1.client = &testK8S{t: t}

	c2, err := newController(controllerConfig{
		MyNode: "iris2",
		Logger: log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr)),
		SList:  fakeSL,
	})
	if err != nil {
		t.Fatalf("creating controller: %s", err)
	}
	c2.client = &testK8S{t: t}

	l := log.NewNopLogger()

	cfg := &config.Config{
		Pools: &config.Pools{ByName: map[string]*config.Pool{
			"default": {
				CIDR: []*net.IPNet{ipnet("10.20.30.0/24")},
				L2Advertisements: []*config.L2Advertisement{
					{
						Nodes: map[string]bool{
							"iris1": true,
							"iris2": true,
						},
					},
				},
			},
		}},
	}

	if c1.SetConfig(l, cfg) == controllers.SyncStateError {
		t.Errorf("SetConfig failed")
	}
	if c2.SetConfig(l, cfg) == controllers.SyncStateError {
		t.Errorf("SetConfig failed")
	}

	eps1 := []discovery.EndpointSlice{
		{
			Endpoints: []discovery.Endpoint{
				{
					Addresses: []string{
						"2.3.4.5",
					},
					NodeName: ptr.To("iris1"),
					Conditions: discovery.EndpointConditions{
						Ready: ptr.To(true),
					},
				},
			},
		},
	}

	eps2 := []discovery.EndpointSlice{
		{
			Endpoints: []discovery.Endpoint{
				{
					Addresses: []string{
						"2.3.4.5",
					},
					NodeName: ptr.To("iris2"),
					Conditions: discovery.EndpointConditions{
						Ready: ptr.To(true),
					},
				},
			},
		},
	}
	c1Found := false
	c2Found := false

	// Cluster policy doesn't care about the locality of the endpoint, as long as there is
	// at least one endpoint active. Here we check that the distribution of the service happens
	// in a way that only a speaker notifies it, and that the assignement is consistent with the
	// service ip.
	for i := 1; i < 256; i++ {
		ip := fmt.Sprintf("10.20.30.%d", i)
		svc1 := &v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name: "svc1",
			},
			Spec: v1.ServiceSpec{
				Type:                  "LoadBalancer",
				ExternalTrafficPolicy: v1.ServiceExternalTrafficPolicyTypeCluster,
			},
			Status: statusAssigned(ip),
		}
		svc2 := &v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name: "svc2",
			},
			Spec: v1.ServiceSpec{
				Type:                  "LoadBalancer",
				ExternalTrafficPolicy: v1.ServiceExternalTrafficPolicyTypeCluster,
			},
			Status: statusAssigned(ip),
		}

		lbIP := net.ParseIP(ip)
		response1svc1 := c1.protocolHandlers[config.Layer2].ShouldAnnounce(l, "test1", []net.IP{lbIP}, cfg.Pools.ByName["default"], svc1, eps1, nil)
		response2svc1 := c2.protocolHandlers[config.Layer2].ShouldAnnounce(l, "test1", []net.IP{lbIP}, cfg.Pools.ByName["default"], svc1, eps1, nil)

		response1svc2 := c1.protocolHandlers[config.Layer2].ShouldAnnounce(l, "test1", []net.IP{lbIP}, cfg.Pools.ByName["default"], svc2, eps2, nil)
		response2svc2 := c2.protocolHandlers[config.Layer2].ShouldAnnounce(l, "test1", []net.IP{lbIP}, cfg.Pools.ByName["default"], svc2, eps2, nil)

		// We check that only one speaker announces the service, so their response must be different
		if response1svc1 == response2svc1 {
			t.Fatalf("Expected only one speaker to announce ip %s , got %s from speaker1, %s from speaker2", ip, response1svc1, response2svc1)
		}
		// Speakers must announce different services with the same ip as the same way
		if response1svc1 != response1svc2 {
			t.Fatalf("Expected both speakers announce svc1 and svc2 with ip %s consistently, got %s from speaker1 for svc1, %s from speaker1 for svc2", ip, response1svc1, response1svc2)
		}
		if response2svc1 != response2svc2 {
			t.Fatalf("Expected both speakers announce svc1 and svc2 with ip %s consistently, got %s from speaker2 for svc1, %s from speaker2 for svc2", ip, response1svc1, response1svc2)
		}

		// we check that both speaker announce at least one service
		if response1svc1 == "" {
			c1Found = true
		}
		if response2svc1 == "" {
			c2Found = true
		}
	}
	if !c1Found {
		t.Fatalf("All services assigned to speaker2")
	}
	if !c2Found {
		t.Fatalf("All services assigned to speaker1")
	}
}

func TestIPAdvertisementFor(t *testing.T) {
	tests := []struct {
		desc             string
		ip               net.IP
		localNode        string
		l2Advertisements []*config.L2Advertisement
		expect           layer2.IPAdvertisement
	}{
		{
			desc:      "Not specify the IPPool related interfaces",
			ip:        net.IP{192, 168, 10, 3},
			localNode: "nodeA",
			l2Advertisements: []*config.L2Advertisement{
				{
					Nodes: map[string]bool{
						"nodeB": true,
						"nodeA": true,
					},
					Interfaces:    []string{},
					AllInterfaces: true,
				},
			},
			expect: layer2.NewIPAdvertisement(net.IP{192, 168, 10, 3}, true, sets.Set[string]{}),
		}, {
			desc:      "LocalNode doesn't match L2Advertisement",
			ip:        net.IP{192, 168, 10, 3},
			localNode: "nodeA",
			l2Advertisements: []*config.L2Advertisement{
				{
					Nodes: map[string]bool{
						"nodeB": true,
					},
					Interfaces: []string{"eth0", "eth1"},
				}, {
					Nodes: map[string]bool{
						"nodeA": false,
						"nodeC": true,
					},
					Interfaces: []string{"eth3", "eth4"},
				},
			},
			expect: layer2.NewIPAdvertisement(net.IP{192, 168, 10, 3}, false, sets.Set[string]{}),
		}, {
			desc:      "LocalNode only match one L2Advertisement",
			ip:        net.ParseIP("2000:12"),
			localNode: "nodeA",
			l2Advertisements: []*config.L2Advertisement{
				{
					Nodes: map[string]bool{
						"nodeB": true,
						"nodeA": false,
					},
					Interfaces: []string{"eth0"},
				}, {
					Nodes: map[string]bool{
						"nodeC": true,
						"nodeA": true,
					},
					Interfaces: []string{"eth1", "eth2"},
				},
			},
			expect: layer2.NewIPAdvertisement(net.ParseIP("2000:12"), false, sets.New("eth1", "eth2")),
		}, {
			desc:      "LocalNode match multi-L2Advertisement",
			ip:        net.IP{192, 168, 10, 3},
			localNode: "nodeA",
			l2Advertisements: []*config.L2Advertisement{
				{
					Nodes: map[string]bool{
						"nodeB": true,
						"nodeA": true,
					},
					Interfaces: []string{"eth0", "eth1"},
				}, {
					Nodes: map[string]bool{
						"nodeC": true,
						"nodeA": true,
					},
					Interfaces: []string{"eth0", "eth2"},
				}, {
					Nodes: map[string]bool{
						"nodeC": true,
						"nodeD": true,
					},
					Interfaces: []string{"eth4", "eth3"},
				},
			},
			expect: layer2.NewIPAdvertisement(net.IP{192, 168, 10, 3}, false, sets.New("eth0", "eth1", "eth2")),
		}, {
			desc:      "LocalNode match multi-L2Advertisement, and one of them not specify interfaces",
			ip:        net.IP{192, 168, 10, 3},
			localNode: "nodeA",
			l2Advertisements: []*config.L2Advertisement{
				{
					Nodes: map[string]bool{
						"nodeB": true,
						"nodeA": true,
					},
					Interfaces: []string{"eth0", "eth1"},
				}, {
					Nodes: map[string]bool{
						"nodeC": true,
						"nodeA": true,
					},
					AllInterfaces: true,
				}, {
					Nodes: map[string]bool{
						"nodeC": true,
						"nodeD": true,
					},
					Interfaces: []string{"eth4", "eth3"},
				},
			},
			expect: layer2.NewIPAdvertisement(net.IP{192, 168, 10, 3}, true, sets.Set[string]{}),
		},
	}
	for _, test := range tests {
		r := ipAdvertisementFor(test.ip, test.localNode, test.l2Advertisements)
		if !r.Equal(&test.expect) {
			t.Errorf("%s: expect interfaces is %+v, but the result is %+v", test.desc, test.expect, r)
		}
	}
}
