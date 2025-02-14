// SPDX-License-Identifier:Apache-2.0

package main

import (
	"errors"
	"fmt"
	"net"
	"os"
	"reflect"
	"sort"
	"sync"
	"testing"

	"go.universe.tf/metallb/internal/bgp"
	"go.universe.tf/metallb/internal/bgp/community"
	"go.universe.tf/metallb/internal/config"
	"go.universe.tf/metallb/internal/k8s/controllers"

	"github.com/go-kit/log"
	"github.com/google/go-cmp/cmp"
	v1 "k8s.io/api/core/v1"
	discovery "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
)

func mustSelector(s string) labels.Selector {
	res, err := labels.Parse(s)
	if err != nil {
		panic(err)
	}
	return res
}

func ipnet(s string) *net.IPNet {
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		panic(err)
	}
	return n
}

func statusAssigned(ip string) v1.ServiceStatus {
	return v1.ServiceStatus{
		LoadBalancer: v1.LoadBalancerStatus{
			Ingress: []v1.LoadBalancerIngress{
				{
					IP: ip,
				},
			},
		},
	}
}

func sortAds(ads map[string][]*bgp.Advertisement) {
	if len(ads) == 0 {
		return
	}

	for _, v := range ads {
		if len(v) == 0 {
			continue
		}
		sort.Slice(v, func(i, j int) bool {
			a, b := v[i], v[j]
			if a.Prefix.String() != b.Prefix.String() {
				return a.Prefix.String() < b.Prefix.String()
			}
			if a.LocalPref != b.LocalPref {
				return a.LocalPref < b.LocalPref
			}
			if len(a.Communities) != len(b.Communities) {
				return len(a.Communities) < len(b.Communities)
			}
			sort.Slice(a.Communities, func(i, j int) bool { return a.Communities[i].LessThan(a.Communities[j]) })
			sort.Slice(b.Communities, func(i, j int) bool { return b.Communities[i].LessThan(b.Communities[j]) })
			for k := range a.Communities {
				if a.Communities[k] != b.Communities[k] {
					return a.Communities[k].LessThan(b.Communities[k])
				}
			}
			return false
		})
	}
}

type fakeBGP struct {
	t              *testing.T
	sessionManager fakeBGPSessionManager
}

func (f *fakeBGP) NewSessionManager(_ controllerConfig) bgp.SessionManager {
	f.sessionManager.t = f.t
	f.sessionManager.gotAds = make(map[string][]*bgp.Advertisement)

	return &f.sessionManager
}

type fakeBGPSessionManager struct {
	t *testing.T

	sync.Mutex
	// peer IP -> advertisements
	gotAds map[string][]*bgp.Advertisement
}

func (f *fakeBGPSessionManager) NewSession(_ log.Logger, args bgp.SessionParameters) (bgp.Session, error) {
	f.Lock()
	defer f.Unlock()

	if _, ok := f.gotAds[args.PeerAddress]; ok {
		f.t.Errorf("Tried to create already existing BGP session to %q", args.PeerAddress)
		return nil, errors.New("invariant violation")
	}
	// Nil because we haven't programmed any routes for it yet, but
	// the key now exists in the map.
	f.gotAds[args.PeerAddress] = nil
	return &fakeSession{
		f:    f,
		addr: args.PeerAddress,
	}, nil
}

func (f *fakeBGPSessionManager) SyncBFDProfiles(profiles map[string]*config.BFDProfile) error {
	return nil
}

func (f *fakeBGPSessionManager) SyncExtraInfo(extra string) error {
	return nil
}

func (f *fakeBGPSessionManager) Ads() map[string][]*bgp.Advertisement {
	ret := map[string][]*bgp.Advertisement{}

	f.Lock()
	defer f.Unlock()

	// Make a deep copy so that we can release the lock.
	for k, v := range f.gotAds {
		if v == nil {
			ret[k] = nil
			continue
		}
		s := []*bgp.Advertisement{}
		for _, ad := range v {
			adCopy := new(bgp.Advertisement)
			*adCopy = *ad
			s = append(s, adCopy)
		}
		ret[k] = s
	}

	return ret
}

func (f *fakeBGPSessionManager) SetEventCallback(_ func(interface{})) {}

type fakeSession struct {
	f    *fakeBGPSessionManager
	addr string
}

func (f *fakeSession) Close() error {
	f.f.Lock()
	defer f.f.Unlock()

	if _, ok := f.f.gotAds[f.addr]; !ok {
		f.f.t.Errorf("Tried to close non-existent session to %q", f.addr)
		return errors.New("invariant violation")
	}

	delete(f.f.gotAds, f.addr)
	return nil
}

func (f *fakeSession) Set(ads ...*bgp.Advertisement) error {
	f.f.Lock()
	defer f.f.Unlock()

	if _, ok := f.f.gotAds[f.addr]; !ok {
		f.f.t.Errorf("Tried to set ads on non-existent session to %q", f.addr)
		return errors.New("invariant violation")
	}

	f.f.gotAds[f.addr] = ads
	return nil
}

// testK8S implements service by recording what the controller wants
// to do to k8s.
type testK8S struct {
	loggedWarning bool
	t             *testing.T
}

func (s *testK8S) UpdateStatus(svc *v1.Service) error {
	panic("never called")
}

func (s *testK8S) Infof(_ *v1.Service, evtType string, msg string, args ...interface{}) {
	s.t.Logf("k8s Info event %q: %s", evtType, fmt.Sprintf(msg, args...))
}

func (s *testK8S) Errorf(_ *v1.Service, evtType string, msg string, args ...interface{}) {
	s.t.Logf("k8s Warning event %q: %s", evtType, fmt.Sprintf(msg, args...))
	s.loggedWarning = true
}
func noopCallback(_ string) {}

func TestBGPSpeakerEPSlices(t *testing.T) {
	b := &fakeBGP{
		t: t,
	}
	newBGP = b.NewSessionManager
	c, err := newController(controllerConfig{
		MyNode:                "pandora",
		DisableLayer2:         true,
		bgpType:               bgpNative,
		BGPAdsChangedCallback: noopCallback,
	})
	if err != nil {
		t.Fatalf("creating controller: %s", err)
	}
	c.client = &testK8S{t: t}

	tests := []struct {
		desc string

		balancer string
		config   *config.Config
		svc      *v1.Service
		eps      []discovery.EndpointSlice

		wantAds map[string][]*bgp.Advertisement
	}{
		{
			desc:     "Service ignored, no config",
			balancer: "test1",
			svc: &v1.Service{
				Spec: v1.ServiceSpec{
					Type:                  "LoadBalancer",
					ExternalTrafficPolicy: "Cluster",
				},
				Status: statusAssigned("10.20.30.1"),
			},
			eps: []discovery.EndpointSlice{
				{
					Endpoints: []discovery.Endpoint{
						{
							Addresses: []string{
								"2.3.4.5",
							},
							NodeName: ptr.To("iris"),
							Conditions: discovery.EndpointConditions{
								Ready: ptr.To(true),
							},
						},
					},
				},
			},
			wantAds: map[string][]*bgp.Advertisement{},
		},
		{
			desc: "One peer, no services",
			config: &config.Config{
				Peers: map[string]*config.Peer{
					"peer1": {
						Addr:          net.ParseIP("1.2.3.4"),
						NodeSelectors: []labels.Selector{labels.Everything()},
					},
				},
				Pools: &config.Pools{ByName: map[string]*config.Pool{
					"default": {
						CIDR: []*net.IPNet{ipnet("10.20.30.0/24")},
						BGPAdvertisements: []*config.BGPAdvertisement{
							{
								AggregationLength: 32,
								Nodes:             map[string]bool{"pandora": true},
							},
						},
					},
				}},
			},
			wantAds: map[string][]*bgp.Advertisement{
				"1.2.3.4:0": nil,
			},
		},
		{
			desc:     "Add service, not an LB",
			balancer: "test1",
			svc: &v1.Service{
				Spec: v1.ServiceSpec{
					Type:                  "ClusterIP",
					ExternalTrafficPolicy: "Cluster",
				},
			},
			eps: []discovery.EndpointSlice{
				{
					Endpoints: []discovery.Endpoint{
						{
							Addresses: []string{
								"2.3.4.5",
							},
							NodeName: ptr.To("pandora"),
							Conditions: discovery.EndpointConditions{
								Ready: ptr.To(true),
							},
						},
					},
				},
			},
			wantAds: map[string][]*bgp.Advertisement{
				"1.2.3.4:0": nil,
			},
		},

		{
			desc:     "Add service, it's an LB!",
			balancer: "test1",
			svc: &v1.Service{
				Spec: v1.ServiceSpec{
					Type:                  "LoadBalancer",
					ExternalTrafficPolicy: "Cluster",
				},
				Status: statusAssigned("10.20.30.1"),
			},
			eps: []discovery.EndpointSlice{
				{
					Endpoints: []discovery.Endpoint{
						{
							Addresses: []string{
								"2.3.4.5",
							},
							NodeName: ptr.To("iris"),
							Conditions: discovery.EndpointConditions{
								Ready: ptr.To(true),
							},
						},
					},
				},
			},
			wantAds: map[string][]*bgp.Advertisement{
				"1.2.3.4:0": {
					{
						Prefix: ipnet("10.20.30.1/32"),
					},
				},
			},
		},

		{
			desc:     "LB switches to local traffic policy, endpoint isn't on our node",
			balancer: "test1",
			svc: &v1.Service{
				Spec: v1.ServiceSpec{
					Type:                  "LoadBalancer",
					ExternalTrafficPolicy: "Local",
				},
				Status: statusAssigned("10.20.30.1"),
			},
			eps: []discovery.EndpointSlice{
				{
					Endpoints: []discovery.Endpoint{
						{
							Addresses: []string{
								"2.3.4.5",
							},
							NodeName: ptr.To("iris"),
							Conditions: discovery.EndpointConditions{
								Ready: ptr.To(true),
							},
						},
					},
				},
			},
			wantAds: map[string][]*bgp.Advertisement{
				"1.2.3.4:0": nil,
			},
		},

		{
			desc:     "New endpoint, on our node",
			balancer: "test1",
			svc: &v1.Service{
				Spec: v1.ServiceSpec{
					Type:                  "LoadBalancer",
					ExternalTrafficPolicy: "Local",
				},
				Status: statusAssigned("10.20.30.1"),
			},
			eps: []discovery.EndpointSlice{
				{
					Endpoints: []discovery.Endpoint{
						{
							Addresses: []string{
								"2.3.4.5",
							},
							NodeName: ptr.To("iris"),
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
								"2.3.4.6",
							},
							NodeName: ptr.To("pandora"),
							Conditions: discovery.EndpointConditions{
								Ready: ptr.To(true),
							},
						},
					},
				},
			},
			wantAds: map[string][]*bgp.Advertisement{
				"1.2.3.4:0": {
					{
						Prefix: ipnet("10.20.30.1/32"),
					},
				},
			},
		},

		{
			desc:     "Endpoint on our node has some unready ports",
			balancer: "test1",
			svc: &v1.Service{
				Spec: v1.ServiceSpec{
					Type:                  "LoadBalancer",
					ExternalTrafficPolicy: "Local",
				},
				Status: statusAssigned("10.20.30.1"),
			},
			eps: []discovery.EndpointSlice{
				{
					Endpoints: []discovery.Endpoint{
						{
							Addresses: []string{
								"2.3.4.5",
							},
							NodeName: ptr.To("iris"),
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
								"2.3.4.6",
							},
							NodeName: ptr.To("pandora"),
							Conditions: discovery.EndpointConditions{
								Ready: ptr.To(false),
							},
						},
					},
				},
				{
					Endpoints: []discovery.Endpoint{
						{
							Addresses: []string{
								"2.3.4.7",
							},
							NodeName: ptr.To("iris"),
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
								"2.3.4.6",
							},
							NodeName: ptr.To("pandora"),
							Conditions: discovery.EndpointConditions{
								Ready: ptr.To(true),
							},
						},
					},
				},
			},
			wantAds: map[string][]*bgp.Advertisement{
				"1.2.3.4:0": nil,
			},
		},

		{
			desc:     "Endpoint list is empty",
			balancer: "test1",
			svc: &v1.Service{
				Spec: v1.ServiceSpec{
					Type:                  "LoadBalancer",
					ExternalTrafficPolicy: "Cluster",
				},
				Status: statusAssigned("10.20.30.1"),
			},
			eps: []discovery.EndpointSlice{},
			wantAds: map[string][]*bgp.Advertisement{
				"1.2.3.4:0": nil,
			},
		},

		{
			desc:     "Endpoint list contains only unhealthy endpoints",
			balancer: "test1",
			svc: &v1.Service{
				Spec: v1.ServiceSpec{
					Type:                  "LoadBalancer",
					ExternalTrafficPolicy: "Cluster",
				},
				Status: statusAssigned("10.20.30.1"),
			},
			eps: []discovery.EndpointSlice{
				{
					Endpoints: []discovery.Endpoint{
						{
							Addresses: []string{
								"2.3.4.5",
							},
							NodeName: ptr.To("iris"),
							Conditions: discovery.EndpointConditions{
								Ready: ptr.To(false),
							},
						},
					},
				},
			},
			wantAds: map[string][]*bgp.Advertisement{
				"1.2.3.4:0": nil,
			},
		},

		{
			desc:     "Endpoint list contains some unhealthy endpoints",
			balancer: "test1",
			svc: &v1.Service{
				Spec: v1.ServiceSpec{
					Type:                  "LoadBalancer",
					ExternalTrafficPolicy: "Cluster",
				},
				Status: statusAssigned("10.20.30.1"),
			},
			eps: []discovery.EndpointSlice{
				{
					Endpoints: []discovery.Endpoint{
						{
							Addresses: []string{
								"2.3.4.5",
							},
							NodeName: ptr.To("iris"),
							Conditions: discovery.EndpointConditions{
								Ready: ptr.To(true),
							},
						},
						{
							Addresses: []string{
								"2.3.4.6",
							},
							NodeName: ptr.To("pandora"),
							Conditions: discovery.EndpointConditions{
								Ready: ptr.To(false),
							},
						},
					},
				},
			},
			wantAds: map[string][]*bgp.Advertisement{
				"1.2.3.4:0": {
					{
						Prefix: ipnet("10.20.30.1/32"),
					},
				},
			},
		},

		{
			desc:     "Endpoint list contains serving but not ready endpoints",
			balancer: "test1",
			svc: &v1.Service{
				Spec: v1.ServiceSpec{
					Type:                  "LoadBalancer",
					ExternalTrafficPolicy: "Cluster",
				},
				Status: statusAssigned("10.20.30.1"),
			},
			eps: []discovery.EndpointSlice{
				{
					Endpoints: []discovery.Endpoint{
						{
							Addresses: []string{
								"2.3.4.5",
							},
							NodeName: ptr.To("iris"),
							Conditions: discovery.EndpointConditions{
								Ready:   ptr.To(false),
								Serving: ptr.To(true),
							},
						},
					},
				},
			},
			wantAds: map[string][]*bgp.Advertisement{
				"1.2.3.4:0": {
					{
						Prefix: ipnet("10.20.30.1/32"),
					},
				},
			},
		},

		{
			desc:     "Endpoint list contains ready but not serving endpoints",
			balancer: "test1",
			svc: &v1.Service{
				Spec: v1.ServiceSpec{
					Type:                  "LoadBalancer",
					ExternalTrafficPolicy: "Cluster",
				},
				Status: statusAssigned("10.20.30.1"),
			},
			eps: []discovery.EndpointSlice{
				{
					Endpoints: []discovery.Endpoint{
						{
							Addresses: []string{
								"2.3.4.5",
							},
							NodeName: ptr.To("iris"),
							Conditions: discovery.EndpointConditions{
								Ready:   ptr.To(true),
								Serving: ptr.To(false),
							},
						},
					},
				},
			},
			wantAds: map[string][]*bgp.Advertisement{
				"1.2.3.4:0": {
					{
						Prefix: ipnet("10.20.30.1/32"),
					},
				},
			},
		},

		{
			desc: "Multiple advertisement config",
			config: &config.Config{
				Peers: map[string]*config.Peer{
					"peer1": {
						Addr:          net.ParseIP("1.2.3.4"),
						NodeSelectors: []labels.Selector{labels.Everything()},
					},
				},
				Pools: &config.Pools{ByName: map[string]*config.Pool{
					"default": {
						CIDR: []*net.IPNet{ipnet("10.20.30.0/24")},
						BGPAdvertisements: []*config.BGPAdvertisement{
							{
								AggregationLength: 32,
								LocalPref:         100,
								Communities: func() map[community.BGPCommunity]bool {
									community1, _ := community.New("0:1234")
									community2, _ := community.New("0:2345")
									return map[community.BGPCommunity]bool{community1: true, community2: true}
								}(),
								Nodes: map[string]bool{"pandora": true},
							},
							{
								AggregationLength: 24,
								LocalPref:         1000,
								Nodes:             map[string]bool{"pandora": true},
							},
						},
					},
				}},
			},
			balancer: "test1",
			svc: &v1.Service{
				Spec: v1.ServiceSpec{
					Type:                  "LoadBalancer",
					ExternalTrafficPolicy: "Cluster",
				},
				Status: statusAssigned("10.20.30.1"),
			},
			eps: []discovery.EndpointSlice{
				{
					Endpoints: []discovery.Endpoint{
						{
							Addresses: []string{
								"2.3.4.5",
							},
							NodeName: ptr.To("iris"),
							Conditions: discovery.EndpointConditions{
								Ready: ptr.To(true),
							},
						},
					},
				},
			},
			wantAds: map[string][]*bgp.Advertisement{
				"1.2.3.4:0": {
					{
						Prefix:    ipnet("10.20.30.1/32"),
						LocalPref: 100,
						Communities: func() []community.BGPCommunity {
							community1, _ := community.New("0:1234")
							community2, _ := community.New("0:2345")
							return []community.BGPCommunity{community1, community2}
						}(),
					},
					{
						Prefix:    ipnet("10.20.30.0/24"),
						LocalPref: 1000,
					},
				},
			},
		},

		{
			desc: "Multiple advertisement config, one only for my node",
			config: &config.Config{
				Peers: map[string]*config.Peer{
					"peer1": {
						Addr:          net.ParseIP("1.2.3.4"),
						NodeSelectors: []labels.Selector{labels.Everything()},
					},
				},
				Pools: &config.Pools{ByName: map[string]*config.Pool{
					"default": {
						CIDR: []*net.IPNet{ipnet("10.20.30.0/24")},
						BGPAdvertisements: []*config.BGPAdvertisement{
							{
								AggregationLength: 32,
								LocalPref:         100,
								Communities: func() map[community.BGPCommunity]bool {
									community1, _ := community.New("0:1234")
									community2, _ := community.New("0:2345")
									return map[community.BGPCommunity]bool{community1: true, community2: true}
								}(),
								Nodes: map[string]bool{"pandora": true},
							},
							{
								AggregationLength: 24,
								LocalPref:         1000,
								Nodes:             map[string]bool{"iris": true},
							},
						},
					},
				}},
			},
			balancer: "test1",
			svc: &v1.Service{
				Spec: v1.ServiceSpec{
					Type:                  "LoadBalancer",
					ExternalTrafficPolicy: "Cluster",
				},
				Status: statusAssigned("10.20.30.1"),
			},
			eps: []discovery.EndpointSlice{
				{
					Endpoints: []discovery.Endpoint{
						{
							Addresses: []string{
								"2.3.4.5",
							},
							NodeName: ptr.To("iris"),
							Conditions: discovery.EndpointConditions{
								Ready: ptr.To(true),
							},
						},
					},
				},
			},
			wantAds: map[string][]*bgp.Advertisement{
				"1.2.3.4:0": {
					{
						Prefix:    ipnet("10.20.30.1/32"),
						LocalPref: 100,
						Communities: func() []community.BGPCommunity {
							community1, _ := community.New("0:1234")
							community2, _ := community.New("0:2345")
							return []community.BGPCommunity{community1, community2}
						}(),
					},
				},
			},
		},

		{
			desc: "Multiple advertisement config, one with peer selector",
			config: &config.Config{
				Peers: map[string]*config.Peer{
					"peer1": {
						Name:          "peer1",
						Addr:          net.ParseIP("1.2.3.4"),
						NodeSelectors: []labels.Selector{labels.Everything()},
					},
				},
				Pools: &config.Pools{ByName: map[string]*config.Pool{
					"default": {
						CIDR: []*net.IPNet{ipnet("10.20.30.0/24")},
						BGPAdvertisements: []*config.BGPAdvertisement{
							{
								AggregationLength: 32,
								LocalPref:         100,
								Communities: func() map[community.BGPCommunity]bool {
									community1, _ := community.New("0:1234")
									community2, _ := community.New("0:2345")
									return map[community.BGPCommunity]bool{community1: true, community2: true}
								}(),
								Peers: []string{"peer1"},
								Nodes: map[string]bool{"pandora": true},
							},
							{
								AggregationLength: 24,
								LocalPref:         1000,
								Nodes:             map[string]bool{"pandora": true},
							},
						},
					},
				}},
			},
			balancer: "test1",
			svc: &v1.Service{
				Spec: v1.ServiceSpec{
					Type:                  "LoadBalancer",
					ExternalTrafficPolicy: "Cluster",
				},
				Status: statusAssigned("10.20.30.1"),
			},
			eps: []discovery.EndpointSlice{
				{
					Endpoints: []discovery.Endpoint{
						{
							Addresses: []string{
								"2.3.4.5",
							},
							NodeName: ptr.To("iris"),
							Conditions: discovery.EndpointConditions{
								Ready: ptr.To(true),
							},
						},
					},
				},
			},

			wantAds: map[string][]*bgp.Advertisement{
				"1.2.3.4:0": {
					{
						Prefix:    ipnet("10.20.30.1/32"),
						LocalPref: 100,
						Communities: func() []community.BGPCommunity {
							community1, _ := community.New("0:1234")
							community2, _ := community.New("0:2345")
							return []community.BGPCommunity{community1, community2}
						}(),
						Peers: []string{"peer1"},
					},
					{
						Prefix:    ipnet("10.20.30.0/24"),
						LocalPref: 1000,
					},
				},
			},
		},

		{
			desc: "Multiple peers",
			config: &config.Config{
				Peers: map[string]*config.Peer{
					"peer1": {
						Addr:          net.ParseIP("1.2.3.4"),
						NodeSelectors: []labels.Selector{labels.Everything()},
					},
					"peer2": {
						Addr:          net.ParseIP("1.2.3.5"),
						NodeSelectors: []labels.Selector{labels.Everything()},
					},
				},
				Pools: &config.Pools{ByName: map[string]*config.Pool{
					"default": {
						CIDR: []*net.IPNet{ipnet("10.20.30.0/24")},
						BGPAdvertisements: []*config.BGPAdvertisement{
							{
								AggregationLength: 32,
								Nodes:             map[string]bool{"pandora": true},
							},
						},
					},
				}},
			},
			balancer: "test1",
			svc: &v1.Service{
				Spec: v1.ServiceSpec{
					Type:                  "LoadBalancer",
					ExternalTrafficPolicy: "Cluster",
				},
				Status: statusAssigned("10.20.30.1"),
			},
			eps: []discovery.EndpointSlice{
				{
					Endpoints: []discovery.Endpoint{
						{
							Addresses: []string{
								"2.3.4.5",
							},
							NodeName: ptr.To("iris"),
							Conditions: discovery.EndpointConditions{
								Ready: ptr.To(true),
							},
						},
					},
				},
			},
			wantAds: map[string][]*bgp.Advertisement{
				"1.2.3.4:0": {
					{
						Prefix: ipnet("10.20.30.1/32"),
					},
				},
				"1.2.3.5:0": {
					{
						Prefix: ipnet("10.20.30.1/32"),
					},
				},
			},
		},

		{
			desc:     "Second balancer, no ingress assigned",
			balancer: "test2",
			svc: &v1.Service{
				Spec: v1.ServiceSpec{
					Type:                  "LoadBalancer",
					ExternalTrafficPolicy: "Cluster",
				},
			},
			eps: []discovery.EndpointSlice{
				{
					Endpoints: []discovery.Endpoint{
						{
							Addresses: []string{
								"2.3.4.5",
							},
							NodeName: ptr.To("iris"),
							Conditions: discovery.EndpointConditions{
								Ready: ptr.To(true),
							},
						},
					},
				},
			},
			wantAds: map[string][]*bgp.Advertisement{
				"1.2.3.4:0": {
					{
						Prefix: ipnet("10.20.30.1/32"),
					},
				},
				"1.2.3.5:0": {
					{
						Prefix: ipnet("10.20.30.1/32"),
					},
				},
			},
		},

		{
			desc:     "Second balancer, ingress gets assigned",
			balancer: "test2",
			svc: &v1.Service{
				Spec: v1.ServiceSpec{
					Type:                  "LoadBalancer",
					ExternalTrafficPolicy: "Cluster",
				},
				Status: statusAssigned("10.20.30.5"),
			},
			eps: []discovery.EndpointSlice{
				{
					Endpoints: []discovery.Endpoint{
						{
							Addresses: []string{
								"2.3.4.5",
							},
							NodeName: ptr.To("iris"),
							Conditions: discovery.EndpointConditions{
								Ready: ptr.To(true),
							},
						},
					},
				},
			},
			wantAds: map[string][]*bgp.Advertisement{
				"1.2.3.4:0": {
					{
						Prefix: ipnet("10.20.30.1/32"),
					},
					{
						Prefix: ipnet("10.20.30.5/32"),
					},
				},
				"1.2.3.5:0": {
					{
						Prefix: ipnet("10.20.30.1/32"),
					},
					{
						Prefix: ipnet("10.20.30.5/32"),
					},
				},
			},
		},

		{
			desc:     "Second balancer, ingress shared with first",
			balancer: "test2",
			svc: &v1.Service{
				Spec: v1.ServiceSpec{
					Type:                  "LoadBalancer",
					ExternalTrafficPolicy: "Cluster",
				},
				Status: statusAssigned("10.20.30.1"),
			},
			eps: []discovery.EndpointSlice{
				{
					Endpoints: []discovery.Endpoint{
						{
							Addresses: []string{
								"2.3.4.5",
							},
							NodeName: ptr.To("iris"),
							Conditions: discovery.EndpointConditions{
								Ready: ptr.To(true),
							},
						},
					},
				},
			},
			wantAds: map[string][]*bgp.Advertisement{
				"1.2.3.4:0": {
					// Prefixes duplicated because the dedupe happens
					// inside the real BGP session.
					{
						Prefix: ipnet("10.20.30.1/32"),
					},
					{
						Prefix: ipnet("10.20.30.1/32"),
					},
				},
				"1.2.3.5:0": {
					{
						Prefix: ipnet("10.20.30.1/32"),
					},
					{
						Prefix: ipnet("10.20.30.1/32"),
					},
				},
			},
		},

		{
			desc:     "Delete svc",
			balancer: "test1",
			wantAds: map[string][]*bgp.Advertisement{
				"1.2.3.4:0": {
					{
						Prefix: ipnet("10.20.30.1/32"),
					},
				},
				"1.2.3.5:0": {
					{
						Prefix: ipnet("10.20.30.1/32"),
					},
				},
			},
		},

		{
			desc: "Delete peer",
			config: &config.Config{
				Peers: map[string]*config.Peer{
					"peer1": {
						Addr:          net.ParseIP("1.2.3.5"),
						NodeSelectors: []labels.Selector{labels.Everything()},
					},
				},
				Pools: &config.Pools{ByName: map[string]*config.Pool{
					"default": {
						CIDR: []*net.IPNet{ipnet("10.20.30.0/24")},
						BGPAdvertisements: []*config.BGPAdvertisement{
							{
								AggregationLength: 32,
								Nodes:             map[string]bool{"pandora": true},
							},
						},
					},
				}},
			},
			balancer: "test2",
			svc: &v1.Service{
				Spec: v1.ServiceSpec{
					Type:                  "LoadBalancer",
					ExternalTrafficPolicy: "Cluster",
				},
				Status: statusAssigned("10.20.30.1"),
			},
			eps: []discovery.EndpointSlice{
				{
					Endpoints: []discovery.Endpoint{
						{
							Addresses: []string{
								"2.3.4.5",
							},
							NodeName: ptr.To("iris"),
							Conditions: discovery.EndpointConditions{
								Ready: ptr.To(true),
							},
						},
					},
				},
			},
			wantAds: map[string][]*bgp.Advertisement{
				"1.2.3.5:0": {
					{
						Prefix: ipnet("10.20.30.1/32"),
					},
				},
			},
		},

		{
			desc:     "Delete second svc",
			balancer: "test2",
			wantAds: map[string][]*bgp.Advertisement{
				"1.2.3.5:0": nil,
			},
		},
	}

	l := log.NewNopLogger()
	for _, test := range tests {
		if test.config != nil {
			if c.SetConfig(l, test.config) == controllers.SyncStateError {
				t.Errorf("%q: SetConfig failed", test.desc)
			}
		}
		if test.balancer != "" {
			if c.SetBalancer(l, test.balancer, test.svc, test.eps) == controllers.SyncStateError {
				t.Errorf("%q: SetBalancer failed", test.desc)
			}
		}

		gotAds := b.sessionManager.Ads()
		sortAds(test.wantAds)
		sortAds(gotAds)
		if diff := cmp.Diff(test.wantAds, gotAds); diff != "" {
			t.Errorf("%q: unexpected advertisement state (-want +got)\n%s", test.desc, diff)
		}
	}
}

func TestNodeSelectors(t *testing.T) {
	b := &fakeBGP{
		t: t,
	}
	newBGP = b.NewSessionManager
	c, err := newController(controllerConfig{
		MyNode:                "pandora",
		DisableLayer2:         true,
		bgpType:               bgpNative,
		BGPAdsChangedCallback: noopCallback,
	})
	if err != nil {
		t.Fatalf("creating controller: %s", err)
	}
	c.client = &testK8S{t: t}

	pools := map[string]*config.Pool{
		"default": {
			CIDR: []*net.IPNet{ipnet("1.2.3.0/24")},
			BGPAdvertisements: []*config.BGPAdvertisement{
				{
					AggregationLength: 32,
					Nodes:             map[string]bool{"pandora": true},
				},
			},
		},
	}

	tests := []struct {
		desc            string
		config          *config.Config
		node            *v1.Node
		wantAds         map[string][]*bgp.Advertisement
		wantReturnState controllers.SyncState
	}{
		{
			desc:    "No config, no advertisements",
			wantAds: map[string][]*bgp.Advertisement{},
		},

		{
			desc: "One peer, default node selector, no node labels",
			config: &config.Config{
				Peers: map[string]*config.Peer{
					"peer1": {
						Addr:          net.ParseIP("1.2.3.4"),
						NodeSelectors: []labels.Selector{labels.Everything()},
					},
				},
				Pools: &config.Pools{ByName: pools},
			},
			wantAds: map[string][]*bgp.Advertisement{
				"1.2.3.4:0": nil,
			},
		},

		{
			desc: "Second peer, non-matching node selector",
			config: &config.Config{
				Peers: map[string]*config.Peer{
					"peer1": {
						Addr:          net.ParseIP("1.2.3.4"),
						NodeSelectors: []labels.Selector{labels.Everything()},
					},
					"peer2": {
						Addr: net.ParseIP("2.3.4.5"),
						NodeSelectors: []labels.Selector{
							mustSelector("foo=bar"),
						},
					},
				},
				Pools: &config.Pools{ByName: pools},
			},
			wantAds: map[string][]*bgp.Advertisement{
				"1.2.3.4:0": nil,
			},
		},

		{
			desc: "Add node label that matches",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pandora",
					Labels: map[string]string{
						"foo": "bar",
					},
				},
			},
			wantAds: map[string][]*bgp.Advertisement{
				"1.2.3.4:0": nil,
				"2.3.4.5:0": nil,
			},
		},

		{
			desc: "Change node label so it no longer matches",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pandora",
					Labels: map[string]string{
						"foo": "baz",
					},
				},
			},
			wantAds: map[string][]*bgp.Advertisement{
				"1.2.3.4:0": nil,
			},
		},

		{
			desc: "Change node selector so it matches again",
			config: &config.Config{
				Peers: map[string]*config.Peer{
					"peer1": {
						Addr:          net.ParseIP("1.2.3.4"),
						NodeSelectors: []labels.Selector{labels.Everything()},
					},
					"peer2": {
						Addr: net.ParseIP("2.3.4.5"),
						NodeSelectors: []labels.Selector{
							mustSelector("foo in (bar, baz)"),
						},
					},
				},
				Pools: &config.Pools{ByName: pools},
			},
			wantAds: map[string][]*bgp.Advertisement{
				"1.2.3.4:0": nil,
				"2.3.4.5:0": nil,
			},
		},

		{
			desc: "Change node label back, still matches",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pandora",
					Labels: map[string]string{
						"foo": "bar",
					},
				},
			},
			wantAds: map[string][]*bgp.Advertisement{
				"1.2.3.4:0": nil,
				"2.3.4.5:0": nil,
			},
		},

		{
			desc: "Multiple node selectors, only one matches",
			config: &config.Config{
				Peers: map[string]*config.Peer{
					"peer1": {
						Addr:          net.ParseIP("1.2.3.4"),
						NodeSelectors: []labels.Selector{labels.Everything()},
					},
					"peer2": {
						Addr: net.ParseIP("2.3.4.5"),
						NodeSelectors: []labels.Selector{
							mustSelector("host=frontend"),
							mustSelector("foo in (bar, baz)"),
						},
					},
				},
				Pools: &config.Pools{ByName: pools},
			},
			wantAds: map[string][]*bgp.Advertisement{
				"1.2.3.4:0": nil,
				"2.3.4.5:0": nil,
			},
		},

		{
			desc: "Change node labels to match the other selector",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pandora",
					Labels: map[string]string{
						"host": "frontend",
					},
				},
			},
			wantAds: map[string][]*bgp.Advertisement{
				"1.2.3.4:0": nil,
				"2.3.4.5:0": nil,
			},
		},

		{
			desc: "Change node availability - Unschedulable becomes true",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pandora",
					Labels: map[string]string{
						"host": "frontend",
					},
				},
				Spec: v1.NodeSpec{Unschedulable: true},
			},
			wantAds: map[string][]*bgp.Advertisement{
				"1.2.3.4:0": nil,
				"2.3.4.5:0": nil,
			},
			wantReturnState: controllers.SyncStateReprocessAll,
		},

		{
			desc: "Change node availability - Unschedulable remains true",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pandora",
					Labels: map[string]string{
						"host": "frontend",
					},
				},
				Spec: v1.NodeSpec{Unschedulable: true},
			},
			wantAds: map[string][]*bgp.Advertisement{
				"1.2.3.4:0": nil,
				"2.3.4.5:0": nil,
			},
		},

		{
			desc: "Change node availability - Unschedulable becomes false",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pandora",
					Labels: map[string]string{
						"host": "frontend",
					},
				},
				Spec: v1.NodeSpec{Unschedulable: false},
			},
			wantAds: map[string][]*bgp.Advertisement{
				"1.2.3.4:0": nil,
				"2.3.4.5:0": nil,
			},
			wantReturnState: controllers.SyncStateReprocessAll,
		},
	}

	l := log.NewNopLogger()
	for _, test := range tests {
		if test.config != nil {
			if c.SetConfig(l, test.config) == controllers.SyncStateError {
				t.Errorf("%q: SetConfig failed", test.desc)
			}
		}

		if test.node != nil {
			if r := c.SetNode(l, test.node); r != test.wantReturnState {
				t.Fatalf("%q: SetNode returns wrong value, got: %+v, want: %+v", test.desc, test.wantReturnState, r)
			}
		}

		gotAds := b.sessionManager.Ads()
		sortAds(test.wantAds)
		sortAds(gotAds)
		if diff := cmp.Diff(test.wantAds, gotAds); diff != "" {
			t.Errorf("%q: unexpected advertisement state (-want +got)\n%s", test.desc, diff)
		}
	}
}

func TestShouldAnnounceExcludeLB(t *testing.T) {
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
		eps                 map[string][]discovery.EndpointSlice
		trafficPolicy       v1.ServiceExternalTrafficPolicyType
		excludeFromLB       []string
		ignoreExcludeFromLB bool
		c1ExpectedResult    map[string]string
		c2ExpectedResult    map[string]string
	}{
		{
			desc:          "One service, endpoint on iris1, no selector, both excluded, both should not announce",
			balancer:      "test1",
			eps:           epsOn("iris1"),
			trafficPolicy: v1.ServiceExternalTrafficPolicyTypeCluster,
			excludeFromLB: []string{"iris1", "iris2"},
			c1ExpectedResult: map[string]string{
				"10.20.30.1": "nodeLabeledExcludeBalancers",
			},
			c2ExpectedResult: map[string]string{
				"10.20.30.1": "nodeLabeledExcludeBalancers",
			},
		},
		{
			desc:          "One service, endpoint on iris1, no selector, etplocal, ignore excludelb, both should announce",
			balancer:      "test1",
			eps:           epsOn("iris1"),
			trafficPolicy: v1.ServiceExternalTrafficPolicyTypeCluster,
			excludeFromLB: []string{"iris1", "iris2"},
			c1ExpectedResult: map[string]string{
				"10.20.30.1": "",
			},
			c2ExpectedResult: map[string]string{
				"10.20.30.1": "",
			},
			ignoreExcludeFromLB: true,
		},
	}
	l := log.NewNopLogger()
	for _, test := range tests {
		cfg := config.Config{
			Pools: &config.Pools{ByName: map[string]*config.Pool{
				"default": {
					CIDR: []*net.IPNet{ipnet("10.20.30.0/24")},
					BGPAdvertisements: []*config.BGPAdvertisement{{
						Nodes: map[string]bool{
							"iris1": true,
							"iris2": true,
						},
					}},
				},
			}},
		}
		c1, err := newController(controllerConfig{
			MyNode:                "iris1",
			Logger:                log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr)),
			bgpType:               bgpNative,
			IgnoreExcludeLB:       test.ignoreExcludeFromLB,
			BGPAdsChangedCallback: noopCallback,
		})
		if err != nil {
			t.Fatalf("creating controller: %s", err)
		}
		c1.client = &testK8S{t: t}

		c2, err := newController(controllerConfig{
			MyNode:                "iris2",
			Logger:                log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr)),
			bgpType:               bgpNative,
			IgnoreExcludeLB:       test.ignoreExcludeFromLB,
			BGPAdsChangedCallback: noopCallback,
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

		response1 := c1.protocolHandlers[config.BGP].ShouldAnnounce(l, "test1", []net.IP{lbIP}, cfg.Pools.ByName["default"], &svc, test.eps[lbIPStr], nodes)
		response2 := c2.protocolHandlers[config.BGP].ShouldAnnounce(l, "test1", []net.IP{lbIP}, cfg.Pools.ByName["default"], &svc, test.eps[lbIPStr], nodes)
		if response1 != test.c1ExpectedResult[lbIPStr] {
			t.Errorf("%q: shouldAnnounce for controller 1 for service %s returned incorrect result, expected '%s', but received '%s'", test.desc, lbIPStr, test.c1ExpectedResult[lbIPStr], response1)
		}
		if response2 != test.c2ExpectedResult[lbIPStr] {
			t.Errorf("%q: shouldAnnounce for controller 2 for service %s returned incorrect result, expected '%s', but received '%s'", test.desc, lbIPStr, test.c2ExpectedResult[lbIPStr], response2)
		}
	}
}
func TestPasswordForSession(t *testing.T) {
	tests := []struct {
		name           string
		cfg            *config.Peer
		bgpType        bgpImplementation
		secretHandling SecretHandling
		expectedPass   string
		expectedRef    v1.SecretReference
	}{
		{
			name: "Native BGP with plain text password",
			cfg: &config.Peer{
				Password: "password123",
			},
			bgpType:        bgpNative,
			secretHandling: SecretPassThrough,
			expectedPass:   "password123",
			expectedRef:    v1.SecretReference{},
		},
		{
			name: "FRR BGP with plain text password",
			cfg: &config.Peer{
				Password: "password123",
			},
			bgpType:        bgpFrr,
			secretHandling: SecretPassThrough,
			expectedPass:   "password123",
			expectedRef:    v1.SecretReference{},
		},
		{
			name: "FRR-K8s BGP with plain text password",
			cfg: &config.Peer{
				Password: "password123",
			},
			bgpType:        bgpFrrK8s,
			secretHandling: SecretPassThrough,
			expectedPass:   "password123",
			expectedRef:    v1.SecretReference{},
		},
		{
			name: "native BGP with secret password",
			cfg: &config.Peer{
				SecretPassword: "my-secret-password",
				PasswordRef: v1.SecretReference{
					Name:      "my-secret",
					Namespace: "my-namespace",
				},
			},
			bgpType:        bgpNative,
			secretHandling: SecretConvert,
			expectedPass:   "my-secret-password",
			expectedRef:    v1.SecretReference{},
		},
		{
			name: "frr BGP with secret password",
			cfg: &config.Peer{
				SecretPassword: "my-secret-password",
				PasswordRef: v1.SecretReference{
					Name:      "my-secret",
					Namespace: "my-namespace",
				},
			},
			bgpType:        bgpFrr,
			secretHandling: SecretConvert,
			expectedPass:   "my-secret-password",
			expectedRef:    v1.SecretReference{},
		},
		{
			name: "FRR-K8s BGP with secret password, convert",
			cfg: &config.Peer{
				SecretPassword: "my-secret-password",
				PasswordRef: v1.SecretReference{
					Name:      "my-secret",
					Namespace: "my-namespace",
				},
			},
			bgpType:        bgpFrrK8s,
			secretHandling: SecretConvert,
			expectedPass:   "my-secret-password",
			expectedRef:    v1.SecretReference{},
		},
		{
			name: "FRR-K8s BGP with secret password, passthrough",
			cfg: &config.Peer{
				SecretPassword: "my-secret-password",
				PasswordRef: v1.SecretReference{
					Name:      "my-secret",
					Namespace: "my-namespace",
				},
			},
			bgpType:        bgpFrrK8s,
			secretHandling: SecretPassThrough,
			expectedPass:   "",
			expectedRef: v1.SecretReference{
				Name:      "my-secret",
				Namespace: "my-namespace",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pass, ref := passwordForSession(tt.cfg, tt.bgpType, tt.secretHandling)
			if pass != tt.expectedPass {
				t.Errorf("unexpected password, got: %s, want: %s", pass, tt.expectedPass)
			}
			if !reflect.DeepEqual(ref, tt.expectedRef) {
				t.Errorf("unexpected secret reference, got: %+v, want: %+v", ref, tt.expectedRef)
			}
		})
	}
}

func TestPeersForService(t *testing.T) {
	callbackCounters := map[string]int{}
	callback := func(key string) {
		callbackCounters[key]++
	}

	b := &fakeBGP{
		t: t,
	}
	newBGP = b.NewSessionManager
	c, err := newController(controllerConfig{
		MyNode:                "pandora",
		DisableLayer2:         true,
		bgpType:               bgpNative,
		BGPAdsChangedCallback: callback,
	})
	if err != nil {
		t.Fatalf("creating controller: %s", err)
	}
	c.client = &testK8S{t: t}

	svc1Name, svc2Name := "test1", "test2"
	peer1Name, peer2Name := "peer1", "peer2"
	tests := []struct {
		desc string

		balancer              string
		config                *config.Config
		svc                   *v1.Service
		eps                   []discovery.EndpointSlice
		expectedPeers         map[string][]string // svc -> expected peers
		expectedCallbacked    []string            // the services that should get callbacked
		expectedNotCallbacked []string            // the services that should not get callbacked
	}{
		{
			desc: "Set the config with 2 peers and 2 pools",
			config: &config.Config{
				Peers: map[string]*config.Peer{
					peer1Name: {
						Name:          peer1Name,
						Addr:          net.ParseIP("1.2.3.4"),
						NodeSelectors: []labels.Selector{labels.Everything()},
					},
					peer2Name: {
						Name:          peer2Name,
						Addr:          net.ParseIP("1.2.3.5"),
						NodeSelectors: []labels.Selector{labels.Everything()},
					},
				},
				Pools: &config.Pools{ByName: map[string]*config.Pool{
					"pool1": {
						CIDR: []*net.IPNet{ipnet("10.20.30.0/24")},
						BGPAdvertisements: []*config.BGPAdvertisement{
							{
								AggregationLength: 32,
								Nodes:             map[string]bool{"pandora": true},
							},
						},
					},
					"pool2": {
						CIDR: []*net.IPNet{ipnet("10.20.40.0/24")},
						BGPAdvertisements: []*config.BGPAdvertisement{
							{
								AggregationLength: 32,
								Nodes:             map[string]bool{"pandora": true},
							},
						},
					},
				}},
			},
			expectedPeers:         map[string][]string{svc1Name: {}, svc2Name: {}},
			expectedCallbacked:    []string{},
			expectedNotCallbacked: []string{svc1Name, svc2Name},
		},
		{
			desc:     "Add first service",
			balancer: svc1Name,
			svc: &v1.Service{
				Spec: v1.ServiceSpec{
					Type:                  "LoadBalancer",
					ExternalTrafficPolicy: "Cluster",
				},
				Status: statusAssigned("10.20.30.1"),
			},
			eps: []discovery.EndpointSlice{
				{
					Endpoints: []discovery.Endpoint{
						{
							Addresses: []string{
								"2.3.4.5",
							},
							NodeName: ptr.To("iris"),
							Conditions: discovery.EndpointConditions{
								Ready: ptr.To(true),
							},
						},
					},
				},
			},
			expectedPeers:         map[string][]string{svc1Name: {peer1Name, peer2Name}, svc2Name: {}},
			expectedCallbacked:    []string{svc1Name},
			expectedNotCallbacked: []string{svc2Name},
		},
		{
			desc:     "Add second service",
			balancer: svc2Name,
			svc: &v1.Service{
				Spec: v1.ServiceSpec{
					Type:                  "LoadBalancer",
					ExternalTrafficPolicy: "Cluster",
				},
				Status: statusAssigned("10.20.40.1"),
			},
			eps: []discovery.EndpointSlice{
				{
					Endpoints: []discovery.Endpoint{
						{
							Addresses: []string{
								"2.3.4.5",
							},
							NodeName: ptr.To("iris"),
							Conditions: discovery.EndpointConditions{
								Ready: ptr.To(true),
							},
						},
					},
				},
			},
			expectedPeers:         map[string][]string{svc1Name: {peer1Name, peer2Name}, svc2Name: {peer1Name, peer2Name}},
			expectedCallbacked:    []string{svc2Name},
			expectedNotCallbacked: []string{svc1Name},
		},
		{
			desc: "Advertise service 1 to peer1 only",
			config: &config.Config{
				Peers: map[string]*config.Peer{
					peer1Name: {
						Name:          peer1Name,
						Addr:          net.ParseIP("1.2.3.4"),
						NodeSelectors: []labels.Selector{labels.Everything()},
					},
					peer2Name: {
						Name:          peer2Name,
						Addr:          net.ParseIP("1.2.3.5"),
						NodeSelectors: []labels.Selector{labels.Everything()},
					},
				},
				Pools: &config.Pools{ByName: map[string]*config.Pool{
					"pool1": {
						CIDR: []*net.IPNet{ipnet("10.20.30.0/24")},
						BGPAdvertisements: []*config.BGPAdvertisement{
							{
								AggregationLength: 32,
								Nodes:             map[string]bool{"pandora": true},
								Peers:             []string{peer1Name},
							},
						},
					},
					"pool2": {
						CIDR: []*net.IPNet{ipnet("10.20.40.0/24")},
						BGPAdvertisements: []*config.BGPAdvertisement{
							{
								AggregationLength: 32,
								Nodes:             map[string]bool{"pandora": true},
							},
						},
					},
				}},
			},
			balancer: svc1Name,
			svc: &v1.Service{
				Spec: v1.ServiceSpec{
					Type:                  "LoadBalancer",
					ExternalTrafficPolicy: "Cluster",
				},
				Status: statusAssigned("10.20.30.1"),
			},
			eps: []discovery.EndpointSlice{
				{
					Endpoints: []discovery.Endpoint{
						{
							Addresses: []string{
								"2.3.4.5",
							},
							NodeName: ptr.To("iris"),
							Conditions: discovery.EndpointConditions{
								Ready: ptr.To(true),
							},
						},
					},
				},
			},
			expectedPeers:         map[string][]string{svc1Name: {peer1Name}, svc2Name: {peer1Name, peer2Name}},
			expectedCallbacked:    []string{svc1Name},
			expectedNotCallbacked: []string{svc2Name},
		},
		{
			desc:                  "Delete first service",
			balancer:              svc1Name,
			expectedPeers:         map[string][]string{svc1Name: {}, svc2Name: {peer1Name, peer2Name}},
			expectedCallbacked:    []string{svc1Name},
			expectedNotCallbacked: []string{svc2Name},
		},
		{
			desc: "Delete first peer",
			config: &config.Config{
				Peers: map[string]*config.Peer{
					peer2Name: {
						Name:          peer2Name,
						Addr:          net.ParseIP("1.2.3.5"),
						NodeSelectors: []labels.Selector{labels.Everything()},
					},
				},
				Pools: &config.Pools{ByName: map[string]*config.Pool{
					"pool1": {
						CIDR: []*net.IPNet{ipnet("10.20.30.0/24")},
						BGPAdvertisements: []*config.BGPAdvertisement{
							{
								AggregationLength: 32,
								Nodes:             map[string]bool{"pandora": true},
								Peers:             []string{peer1Name},
							},
						},
					},
					"pool2": {
						CIDR: []*net.IPNet{ipnet("10.20.40.0/24")},
						BGPAdvertisements: []*config.BGPAdvertisement{
							{
								AggregationLength: 32,
								Nodes:             map[string]bool{"pandora": true},
							},
						},
					},
				}},
			},
			balancer: svc2Name,
			svc: &v1.Service{
				Spec: v1.ServiceSpec{
					Type:                  "LoadBalancer",
					ExternalTrafficPolicy: "Cluster",
				},
				Status: statusAssigned("10.20.40.1"),
			},
			eps: []discovery.EndpointSlice{
				{
					Endpoints: []discovery.Endpoint{
						{
							Addresses: []string{
								"2.3.4.5",
							},
							NodeName: ptr.To("iris"),
							Conditions: discovery.EndpointConditions{
								Ready: ptr.To(true),
							},
						},
					},
				},
			},
			expectedPeers:         map[string][]string{svc1Name: {}, svc2Name: {peer2Name}},
			expectedCallbacked:    []string{svc2Name},
			expectedNotCallbacked: []string{svc1Name},
		},
	}

	l := log.NewNopLogger()
	for _, test := range tests {
		oldCallbackCounters := map[string]int{}
		for k, v := range callbackCounters {
			oldCallbackCounters[k] = v
		}
		if test.config != nil {
			if c.SetConfig(l, test.config) == controllers.SyncStateError {
				t.Errorf("%q: SetConfig failed", test.desc)
			}
		}
		if test.balancer != "" {
			if c.SetBalancer(l, test.balancer, test.svc, test.eps) == controllers.SyncStateError {
				t.Errorf("%q: SetBalancer failed", test.desc)
			}
		}

		for svc, wantPeers := range test.expectedPeers {
			if diff := cmp.Diff(wantPeers, sets.List(c.bgpPeersFetcher(svc))); diff != "" {
				t.Errorf("%q: unexpected peers for service %s (-want +got)\n%s", test.desc, svc, diff)
			}
		}

		for _, svc := range test.expectedNotCallbacked {
			if oldCallbackCounters[svc] != callbackCounters[svc] {
				t.Errorf("%q: unexpected callback counters for service %s on NotCallbacked, want %v got %v", test.desc, svc, oldCallbackCounters[svc], callbackCounters[svc])
			}
		}

		for _, svc := range test.expectedCallbacked {
			if oldCallbackCounters[svc]+1 != callbackCounters[svc] {
				t.Errorf("%q: unexpected callback counters for service %s on Callbacked, want %v got %v", test.desc, svc, oldCallbackCounters[svc]+1, callbackCounters[svc])
			}
		}
	}
}
