// SPDX-License-Identifier:Apache-2.0

package main

import (
	"errors"
	"fmt"
	"net"
	"sort"
	"sync"
	"testing"
	"time"

	"go.universe.tf/metallb/internal/bgp"
	"go.universe.tf/metallb/internal/config"
	"go.universe.tf/metallb/internal/k8s"

	"github.com/go-kit/kit/log"
	"github.com/google/go-cmp/cmp"
	v1 "k8s.io/api/core/v1"
	discovery "k8s.io/api/discovery/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func strptr(s string) *string {
	return &s
}

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
			if a.NextHop.String() != b.NextHop.String() {
				return a.NextHop.String() < b.NextHop.String()
			}
			if len(a.Communities) != len(b.Communities) {
				return len(a.Communities) < len(b.Communities)
			}
			sort.Slice(a.Communities, func(i, j int) bool { return a.Communities[i] < a.Communities[j] })
			sort.Slice(b.Communities, func(i, j int) bool { return b.Communities[i] < b.Communities[j] })
			for k := range a.Communities {
				if a.Communities[k] != b.Communities[k] {
					return a.Communities[k] < b.Communities[k]
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

func (f *fakeBGP) NewSessionManager(_ bgpImplementation) bgp.SessionManager {
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

func (f *fakeBGPSessionManager) NewSession(_ log.Logger, addr string, _ net.IP, _ uint32, _ net.IP, _ uint32, _ time.Duration, _ time.Duration, _, _, _ string) (bgp.Session, error) {
	f.Lock()
	defer f.Unlock()

	if _, ok := f.gotAds[addr]; ok {
		f.t.Errorf("Tried to create already existing BGP session to %q", addr)
		return nil, errors.New("invariant violation")
	}
	// Nil because we haven't programmed any routes for it yet, but
	// the key now exists in the map.
	f.gotAds[addr] = nil
	return &fakeSession{
		f:    f,
		addr: addr,
	}, nil
}

func (f *fakeBGPSessionManager) SyncBFDProfiles(profiles map[string]*config.BFDProfile) error {
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

func TestBGPSpeaker(t *testing.T) {
	b := &fakeBGP{
		t: t,
	}
	newBGP = b.NewSessionManager
	c, err := newController(controllerConfig{
		MyNode:        "pandora",
		DisableLayer2: true,
		bgpType:       bgpNative,
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
		eps      k8s.EpsOrSlices

		wantAds        map[string][]*bgp.Advertisement
		expectedCfgRet k8s.SyncState
		expectedLBRet  k8s.SyncState
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
			eps: k8s.EpsOrSlices{
				EpVal: &v1.Endpoints{
					Subsets: []v1.EndpointSubset{
						{
							Addresses: []v1.EndpointAddress{
								{
									IP:       "2.3.4.5",
									NodeName: strptr("iris"),
								},
							},
						},
					},
				},
				Type: k8s.Eps,
			},
			wantAds:        map[string][]*bgp.Advertisement{},
			expectedCfgRet: k8s.SyncStateReprocessAll,
			expectedLBRet:  k8s.SyncStateSuccess,
		},

		{
			desc: "One peer, no services",
			config: &config.Config{
				Peers: []*config.Peer{
					{
						Addr:          net.ParseIP("1.2.3.4"),
						NodeSelectors: []labels.Selector{labels.Everything()},
					},
				},
				Pools: map[string]*config.Pool{
					"default": {
						Protocol: config.BGP,
						CIDR:     []*net.IPNet{ipnet("10.20.30.0/24")},
						BGPAdvertisements: []*config.BGPAdvertisement{
							{
								AggregationLength: 32,
							},
						},
					},
				},
			},
			wantAds: map[string][]*bgp.Advertisement{
				"1.2.3.4:0": nil,
			},
			expectedCfgRet: k8s.SyncStateReprocessAll,
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
			eps: k8s.EpsOrSlices{
				EpVal: &v1.Endpoints{
					Subsets: []v1.EndpointSubset{
						{
							Addresses: []v1.EndpointAddress{
								{
									IP:       "2.3.4.5",
									NodeName: strptr("pandora"),
								},
							},
						},
					},
				},
				Type: k8s.Eps,
			},
			wantAds: map[string][]*bgp.Advertisement{
				"1.2.3.4:0": nil,
			},
			expectedCfgRet: k8s.SyncStateReprocessAll,
			expectedLBRet:  k8s.SyncStateSuccess,
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
			eps: k8s.EpsOrSlices{
				EpVal: &v1.Endpoints{
					Subsets: []v1.EndpointSubset{
						{
							Addresses: []v1.EndpointAddress{
								{
									IP:       "2.3.4.5",
									NodeName: strptr("iris"),
								},
							},
						},
					},
				},
				Type: k8s.Eps,
			},
			wantAds: map[string][]*bgp.Advertisement{
				"1.2.3.4:0": {
					{
						Prefix: ipnet("10.20.30.1/32"),
					},
				},
			},
			expectedCfgRet: k8s.SyncStateReprocessAll,
			expectedLBRet:  k8s.SyncStateSuccess,
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
			eps: k8s.EpsOrSlices{
				EpVal: &v1.Endpoints{
					Subsets: []v1.EndpointSubset{
						{
							Addresses: []v1.EndpointAddress{
								{
									IP:       "2.3.4.5",
									NodeName: strptr("iris"),
								},
							},
						},
					},
				},
				Type: k8s.Eps,
			},
			wantAds: map[string][]*bgp.Advertisement{
				"1.2.3.4:0": nil,
			},
			expectedCfgRet: k8s.SyncStateReprocessAll,
			expectedLBRet:  k8s.SyncStateSuccess,
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
			eps: k8s.EpsOrSlices{
				EpVal: &v1.Endpoints{
					Subsets: []v1.EndpointSubset{
						{
							Addresses: []v1.EndpointAddress{
								{
									IP:       "2.3.4.5",
									NodeName: strptr("iris"),
								},
								{
									IP:       "2.3.4.6",
									NodeName: strptr("pandora"),
								},
							},
						},
					},
				},
				Type: k8s.Eps,
			},
			wantAds: map[string][]*bgp.Advertisement{
				"1.2.3.4:0": {
					{
						Prefix: ipnet("10.20.30.1/32"),
					},
				},
			},
			expectedCfgRet: k8s.SyncStateReprocessAll,
			expectedLBRet:  k8s.SyncStateSuccess,
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
			eps: k8s.EpsOrSlices{
				EpVal: &v1.Endpoints{
					Subsets: []v1.EndpointSubset{
						{
							Addresses: []v1.EndpointAddress{
								{
									IP:       "2.3.4.5",
									NodeName: strptr("iris"),
								},
								{
									IP:       "2.3.4.6",
									NodeName: strptr("pandora"),
								},
							},
						},
						{
							Addresses: []v1.EndpointAddress{
								{
									IP:       "2.3.4.5",
									NodeName: strptr("iris"),
								},
							},
							NotReadyAddresses: []v1.EndpointAddress{
								{
									IP:       "2.3.4.6",
									NodeName: strptr("pandora"),
								},
							},
						},
					},
				},
				Type: k8s.Eps,
			},
			wantAds: map[string][]*bgp.Advertisement{
				"1.2.3.4:0": nil,
			},
			expectedCfgRet: k8s.SyncStateReprocessAll,
			expectedLBRet:  k8s.SyncStateSuccess,
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
			eps: k8s.EpsOrSlices{},
			wantAds: map[string][]*bgp.Advertisement{
				"1.2.3.4:0": nil,
			},
			expectedCfgRet: k8s.SyncStateReprocessAll,
			expectedLBRet:  k8s.SyncStateSuccess,
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
			eps: k8s.EpsOrSlices{
				EpVal: &v1.Endpoints{
					Subsets: []v1.EndpointSubset{
						{
							NotReadyAddresses: []v1.EndpointAddress{
								{
									IP:       "2.3.4.5",
									NodeName: strptr("iris"),
								},
							},
						},
					},
				},
				Type: k8s.Eps,
			},
			wantAds: map[string][]*bgp.Advertisement{
				"1.2.3.4:0": nil,
			},
			expectedCfgRet: k8s.SyncStateReprocessAll,
			expectedLBRet:  k8s.SyncStateSuccess,
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
			eps: k8s.EpsOrSlices{
				EpVal: &v1.Endpoints{
					Subsets: []v1.EndpointSubset{
						{
							Addresses: []v1.EndpointAddress{
								{
									IP:       "2.3.4.5",
									NodeName: strptr("iris"),
								},
							},
							NotReadyAddresses: []v1.EndpointAddress{
								{
									IP:       "2.3.4.6",
									NodeName: strptr("pandora"),
								},
							},
						},
					},
				},
				Type: k8s.Eps,
			},
			wantAds: map[string][]*bgp.Advertisement{
				"1.2.3.4:0": {
					{
						Prefix: ipnet("10.20.30.1/32"),
					},
				},
			},
			expectedCfgRet: k8s.SyncStateReprocessAll,
			expectedLBRet:  k8s.SyncStateSuccess,
		},

		{
			desc: "Multiple advertisement config",
			config: &config.Config{
				Peers: []*config.Peer{
					{
						Addr:          net.ParseIP("1.2.3.4"),
						NodeSelectors: []labels.Selector{labels.Everything()},
					},
				},
				Pools: map[string]*config.Pool{
					"default": {
						Protocol: config.BGP,
						CIDR:     []*net.IPNet{ipnet("10.20.30.0/24")},
						BGPAdvertisements: []*config.BGPAdvertisement{
							{
								AggregationLength: 32,
								LocalPref:         100,
								Communities:       map[uint32]bool{1234: true, 2345: true},
							},
							{
								AggregationLength: 24,
								LocalPref:         1000,
							},
						},
					},
				},
			},
			balancer: "test1",
			svc: &v1.Service{
				Spec: v1.ServiceSpec{
					Type:                  "LoadBalancer",
					ExternalTrafficPolicy: "Cluster",
				},
				Status: statusAssigned("10.20.30.1"),
			},
			eps: k8s.EpsOrSlices{
				EpVal: &v1.Endpoints{
					Subsets: []v1.EndpointSubset{
						{
							Addresses: []v1.EndpointAddress{
								{
									IP:       "2.3.4.5",
									NodeName: strptr("iris"),
								},
							},
						},
					},
				},
				Type: k8s.Eps,
			},
			wantAds: map[string][]*bgp.Advertisement{
				"1.2.3.4:0": {
					{
						Prefix:      ipnet("10.20.30.1/32"),
						LocalPref:   100,
						Communities: []uint32{1234, 2345},
					},
					{
						Prefix:    ipnet("10.20.30.0/24"),
						LocalPref: 1000,
					},
				},
			},
			expectedCfgRet: k8s.SyncStateReprocessAll,
			expectedLBRet:  k8s.SyncStateSuccess,
		},

		{
			desc: "Multiple peers",
			config: &config.Config{
				Peers: []*config.Peer{
					{
						Addr:          net.ParseIP("1.2.3.4"),
						NodeSelectors: []labels.Selector{labels.Everything()},
					},
					{
						Addr:          net.ParseIP("1.2.3.5"),
						NodeSelectors: []labels.Selector{labels.Everything()},
					},
				},
				Pools: map[string]*config.Pool{
					"default": {
						Protocol: config.BGP,
						CIDR:     []*net.IPNet{ipnet("10.20.30.0/24")},
						BGPAdvertisements: []*config.BGPAdvertisement{
							{
								AggregationLength: 32,
							},
						},
					},
				},
			},
			balancer: "test1",
			svc: &v1.Service{
				Spec: v1.ServiceSpec{
					Type:                  "LoadBalancer",
					ExternalTrafficPolicy: "Cluster",
				},
				Status: statusAssigned("10.20.30.1"),
			},
			eps: k8s.EpsOrSlices{
				EpVal: &v1.Endpoints{
					Subsets: []v1.EndpointSubset{
						{
							Addresses: []v1.EndpointAddress{
								{
									IP:       "2.3.4.5",
									NodeName: strptr("iris"),
								},
							},
						},
					},
				},
				Type: k8s.Eps,
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
			expectedCfgRet: k8s.SyncStateReprocessAll,
			expectedLBRet:  k8s.SyncStateSuccess,
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
			eps: k8s.EpsOrSlices{
				EpVal: &v1.Endpoints{
					Subsets: []v1.EndpointSubset{
						{
							Addresses: []v1.EndpointAddress{
								{
									IP:       "2.3.4.5",
									NodeName: strptr("iris"),
								},
							},
						},
					},
				},
				Type: k8s.Eps,
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
			expectedCfgRet: k8s.SyncStateReprocessAll,
			expectedLBRet:  k8s.SyncStateSuccess,
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
			eps: k8s.EpsOrSlices{
				EpVal: &v1.Endpoints{
					Subsets: []v1.EndpointSubset{
						{
							Addresses: []v1.EndpointAddress{
								{
									IP:       "2.3.4.5",
									NodeName: strptr("iris"),
								},
							},
						},
					},
				},
				Type: k8s.Eps,
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
			expectedCfgRet: k8s.SyncStateReprocessAll,
			expectedLBRet:  k8s.SyncStateSuccess,
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
			eps: k8s.EpsOrSlices{
				EpVal: &v1.Endpoints{
					Subsets: []v1.EndpointSubset{
						{
							Addresses: []v1.EndpointAddress{
								{
									IP:       "2.3.4.5",
									NodeName: strptr("iris"),
								},
							},
						},
					},
				},
				Type: k8s.Eps,
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
			expectedCfgRet: k8s.SyncStateReprocessAll,
			expectedLBRet:  k8s.SyncStateSuccess,
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
			expectedCfgRet: k8s.SyncStateReprocessAll,
			expectedLBRet:  k8s.SyncStateSuccess,
		},

		{
			desc: "Delete peer",
			config: &config.Config{
				Peers: []*config.Peer{
					{
						Addr:          net.ParseIP("1.2.3.5"),
						NodeSelectors: []labels.Selector{labels.Everything()},
					},
				},
				Pools: map[string]*config.Pool{
					"default": {
						Protocol: config.BGP,
						CIDR:     []*net.IPNet{ipnet("10.20.30.0/24")},
						BGPAdvertisements: []*config.BGPAdvertisement{
							{
								AggregationLength: 32,
							},
						},
					},
				},
			},
			balancer: "test2",
			svc: &v1.Service{
				Spec: v1.ServiceSpec{
					Type:                  "LoadBalancer",
					ExternalTrafficPolicy: "Cluster",
				},
				Status: statusAssigned("10.20.30.1"),
			},
			eps: k8s.EpsOrSlices{
				EpVal: &v1.Endpoints{
					Subsets: []v1.EndpointSubset{
						{
							Addresses: []v1.EndpointAddress{
								{
									IP:       "2.3.4.5",
									NodeName: strptr("iris"),
								},
							},
						},
					},
				},
				Type: k8s.Eps,
			},
			wantAds: map[string][]*bgp.Advertisement{
				"1.2.3.5:0": {
					{
						Prefix: ipnet("10.20.30.1/32"),
					},
				},
			},
			expectedCfgRet: k8s.SyncStateReprocessAll,
			expectedLBRet:  k8s.SyncStateSuccess,
		},

		{
			desc:     "Delete second svc",
			balancer: "test2",
			wantAds: map[string][]*bgp.Advertisement{
				"1.2.3.5:0": nil,
			},
			expectedCfgRet: k8s.SyncStateReprocessAll,
		},
	}

	l := log.NewNopLogger()
	for _, test := range tests {
		if test.config != nil {
			if c.SetConfig(l, test.config) != test.expectedCfgRet {
				t.Errorf("%q: SetConfig failed", test.desc)
			}
		}
		if test.balancer != "" {
			if c.SetBalancer(l, test.balancer, test.svc, test.eps) != test.expectedLBRet {
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

func TestBGPSpeakerEPSlices(t *testing.T) {
	b := &fakeBGP{
		t: t,
	}
	newBGP = b.NewSessionManager
	c, err := newController(controllerConfig{
		MyNode:        "pandora",
		DisableLayer2: true,
		bgpType:       bgpNative,
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
		eps      k8s.EpsOrSlices

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
			eps: k8s.EpsOrSlices{
				SlicesVal: []*discovery.EndpointSlice{
					{
						Endpoints: []discovery.Endpoint{
							{
								Addresses: []string{
									"2.3.4.5",
								},
								Topology: map[string]string{
									"kubernetes.io/hostname": "iris",
								},
								Conditions: discovery.EndpointConditions{
									Ready: boolPtr(true),
								},
							},
						},
					},
				},
				Type: k8s.Slices,
			},
			wantAds: map[string][]*bgp.Advertisement{},
		},
		{
			desc: "One peer, no services",
			config: &config.Config{
				Peers: []*config.Peer{
					{
						Addr:          net.ParseIP("1.2.3.4"),
						NodeSelectors: []labels.Selector{labels.Everything()},
					},
				},
				Pools: map[string]*config.Pool{
					"default": {
						Protocol: config.BGP,
						CIDR:     []*net.IPNet{ipnet("10.20.30.0/24")},
						BGPAdvertisements: []*config.BGPAdvertisement{
							{
								AggregationLength: 32,
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
			desc:     "Add service, not an LB",
			balancer: "test1",
			svc: &v1.Service{
				Spec: v1.ServiceSpec{
					Type:                  "ClusterIP",
					ExternalTrafficPolicy: "Cluster",
				},
			},
			eps: k8s.EpsOrSlices{
				SlicesVal: []*discovery.EndpointSlice{
					{
						Endpoints: []discovery.Endpoint{
							{
								Addresses: []string{
									"2.3.4.5",
								},
								Topology: map[string]string{
									"kubernetes.io/hostname": "pandora",
								},
								Conditions: discovery.EndpointConditions{
									Ready: boolPtr(true),
								},
							},
						},
					},
				},
				Type: k8s.Slices,
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
			eps: k8s.EpsOrSlices{
				SlicesVal: []*discovery.EndpointSlice{
					{
						Endpoints: []discovery.Endpoint{
							{
								Addresses: []string{
									"2.3.4.5",
								},
								Topology: map[string]string{
									"kubernetes.io/hostname": "iris",
								},
								Conditions: discovery.EndpointConditions{
									Ready: boolPtr(true),
								},
							},
						},
					},
				},
				Type: k8s.Slices,
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
			eps: k8s.EpsOrSlices{
				SlicesVal: []*discovery.EndpointSlice{
					{
						Endpoints: []discovery.Endpoint{
							{
								Addresses: []string{
									"2.3.4.5",
								},
								Topology: map[string]string{
									"kubernetes.io/hostname": "iris",
								},
								Conditions: discovery.EndpointConditions{
									Ready: boolPtr(true),
								},
							},
						},
					},
				},
				Type: k8s.Slices,
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
			eps: k8s.EpsOrSlices{
				SlicesVal: []*discovery.EndpointSlice{
					{
						Endpoints: []discovery.Endpoint{
							{
								Addresses: []string{
									"2.3.4.5",
								},
								Topology: map[string]string{
									"kubernetes.io/hostname": "iris",
								},
								Conditions: discovery.EndpointConditions{
									Ready: boolPtr(true),
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
								Topology: map[string]string{
									"kubernetes.io/hostname": "pandora",
								},
								Conditions: discovery.EndpointConditions{
									Ready: boolPtr(true),
								},
							},
						},
					},
				},
				Type: k8s.Slices,
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
			eps: k8s.EpsOrSlices{
				SlicesVal: []*discovery.EndpointSlice{
					{
						Endpoints: []discovery.Endpoint{
							{
								Addresses: []string{
									"2.3.4.5",
								},
								Topology: map[string]string{
									"kubernetes.io/hostname": "iris",
								},
								Conditions: discovery.EndpointConditions{
									Ready: boolPtr(true),
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
								Topology: map[string]string{
									"kubernetes.io/hostname": "pandora",
								},
								Conditions: discovery.EndpointConditions{
									Ready: boolPtr(false),
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
								Topology: map[string]string{
									"kubernetes.io/hostname": "iris",
								},
								Conditions: discovery.EndpointConditions{
									Ready: boolPtr(true),
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
								Topology: map[string]string{
									"kubernetes.io/hostname": "pandora",
								},
								Conditions: discovery.EndpointConditions{
									Ready: boolPtr(true),
								},
							},
						},
					},
				},
				Type: k8s.Slices,
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
			eps: k8s.EpsOrSlices{},
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
			eps: k8s.EpsOrSlices{
				SlicesVal: []*discovery.EndpointSlice{
					{
						Endpoints: []discovery.Endpoint{
							{
								Addresses: []string{
									"2.3.4.5",
								},
								Topology: map[string]string{
									"kubernetes.io/hostname": "iris",
								},
								Conditions: discovery.EndpointConditions{
									Ready: boolPtr(false),
								},
							},
						},
					},
				},
				Type: k8s.Slices,
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
			eps: k8s.EpsOrSlices{
				SlicesVal: []*discovery.EndpointSlice{
					{
						Endpoints: []discovery.Endpoint{
							{
								Addresses: []string{
									"2.3.4.5",
								},
								Topology: map[string]string{
									"kubernetes.io/hostname": "iris",
								},
								Conditions: discovery.EndpointConditions{
									Ready: boolPtr(true),
								},
							},
							{
								Addresses: []string{
									"2.3.4.6",
								},
								Topology: map[string]string{
									"kubernetes.io/hostname": "pandora",
								},
								Conditions: discovery.EndpointConditions{
									Ready: boolPtr(false),
								},
							},
						},
					},
				},
				Type: k8s.Slices,
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
				Peers: []*config.Peer{
					{
						Addr:          net.ParseIP("1.2.3.4"),
						NodeSelectors: []labels.Selector{labels.Everything()},
					},
				},
				Pools: map[string]*config.Pool{
					"default": {
						Protocol: config.BGP,
						CIDR:     []*net.IPNet{ipnet("10.20.30.0/24")},
						BGPAdvertisements: []*config.BGPAdvertisement{
							{
								AggregationLength: 32,
								LocalPref:         100,
								Communities:       map[uint32]bool{1234: true, 2345: true},
							},
							{
								AggregationLength: 24,
								LocalPref:         1000,
							},
						},
					},
				},
			},
			balancer: "test1",
			svc: &v1.Service{
				Spec: v1.ServiceSpec{
					Type:                  "LoadBalancer",
					ExternalTrafficPolicy: "Cluster",
				},
				Status: statusAssigned("10.20.30.1"),
			},
			eps: k8s.EpsOrSlices{
				SlicesVal: []*discovery.EndpointSlice{
					{
						Endpoints: []discovery.Endpoint{
							{
								Addresses: []string{
									"2.3.4.5",
								},
								Topology: map[string]string{
									"kubernetes.io/hostname": "iris",
								},
								Conditions: discovery.EndpointConditions{
									Ready: boolPtr(true),
								},
							},
						},
					},
				},
				Type: k8s.Slices,
			},
			wantAds: map[string][]*bgp.Advertisement{
				"1.2.3.4:0": {
					{
						Prefix:      ipnet("10.20.30.1/32"),
						LocalPref:   100,
						Communities: []uint32{1234, 2345},
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
				Peers: []*config.Peer{
					{
						Addr:          net.ParseIP("1.2.3.4"),
						NodeSelectors: []labels.Selector{labels.Everything()},
					},
					{
						Addr:          net.ParseIP("1.2.3.5"),
						NodeSelectors: []labels.Selector{labels.Everything()},
					},
				},
				Pools: map[string]*config.Pool{
					"default": {
						Protocol: config.BGP,
						CIDR:     []*net.IPNet{ipnet("10.20.30.0/24")},
						BGPAdvertisements: []*config.BGPAdvertisement{
							{
								AggregationLength: 32,
							},
						},
					},
				},
			},
			balancer: "test1",
			svc: &v1.Service{
				Spec: v1.ServiceSpec{
					Type:                  "LoadBalancer",
					ExternalTrafficPolicy: "Cluster",
				},
				Status: statusAssigned("10.20.30.1"),
			},
			eps: k8s.EpsOrSlices{
				SlicesVal: []*discovery.EndpointSlice{
					{
						Endpoints: []discovery.Endpoint{
							{
								Addresses: []string{
									"2.3.4.5",
								},
								Topology: map[string]string{
									"kubernetes.io/hostname": "iris",
								},
								Conditions: discovery.EndpointConditions{
									Ready: boolPtr(true),
								},
							},
						},
					},
				},
				Type: k8s.Slices,
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
			eps: k8s.EpsOrSlices{
				SlicesVal: []*discovery.EndpointSlice{
					{
						Endpoints: []discovery.Endpoint{
							{
								Addresses: []string{
									"2.3.4.5",
								},
								Topology: map[string]string{
									"kubernetes.io/hostname": "iris",
								},
								Conditions: discovery.EndpointConditions{
									Ready: boolPtr(true),
								},
							},
						},
					},
				},
				Type: k8s.Slices,
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
			eps: k8s.EpsOrSlices{
				SlicesVal: []*discovery.EndpointSlice{
					{
						Endpoints: []discovery.Endpoint{
							{
								Addresses: []string{
									"2.3.4.5",
								},
								Topology: map[string]string{
									"kubernetes.io/hostname": "iris",
								},
								Conditions: discovery.EndpointConditions{
									Ready: boolPtr(true),
								},
							},
						},
					},
				},
				Type: k8s.Slices,
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
			eps: k8s.EpsOrSlices{
				SlicesVal: []*discovery.EndpointSlice{
					{
						Endpoints: []discovery.Endpoint{
							{
								Addresses: []string{
									"2.3.4.5",
								},
								Topology: map[string]string{
									"kubernetes.io/hostname": "iris",
								},
								Conditions: discovery.EndpointConditions{
									Ready: boolPtr(true),
								},
							},
						},
					},
				},
				Type: k8s.Slices,
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
				Peers: []*config.Peer{
					{
						Addr:          net.ParseIP("1.2.3.5"),
						NodeSelectors: []labels.Selector{labels.Everything()},
					},
				},
				Pools: map[string]*config.Pool{
					"default": {
						Protocol: config.BGP,
						CIDR:     []*net.IPNet{ipnet("10.20.30.0/24")},
						BGPAdvertisements: []*config.BGPAdvertisement{
							{
								AggregationLength: 32,
							},
						},
					},
				},
			},
			balancer: "test2",
			svc: &v1.Service{
				Spec: v1.ServiceSpec{
					Type:                  "LoadBalancer",
					ExternalTrafficPolicy: "Cluster",
				},
				Status: statusAssigned("10.20.30.1"),
			},
			eps: k8s.EpsOrSlices{
				SlicesVal: []*discovery.EndpointSlice{
					{
						Endpoints: []discovery.Endpoint{
							{
								Addresses: []string{
									"2.3.4.5",
								},
								Topology: map[string]string{
									"kubernetes.io/hostname": "iris",
								},
								Conditions: discovery.EndpointConditions{
									Ready: boolPtr(true),
								},
							},
						},
					},
				},
				Type: k8s.Slices,
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
			if c.SetConfig(l, test.config) == k8s.SyncStateError {
				t.Errorf("%q: SetConfig failed", test.desc)
			}
		}
		if test.balancer != "" {
			if c.SetBalancer(l, test.balancer, test.svc, test.eps) == k8s.SyncStateError {
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
		MyNode:        "pandora",
		DisableLayer2: true,
		bgpType:       bgpNative,
	})
	if err != nil {
		t.Fatalf("creating controller: %s", err)
	}
	c.client = &testK8S{t: t}

	pools := map[string]*config.Pool{
		"default": {
			Protocol: config.BGP,
			CIDR:     []*net.IPNet{ipnet("1.2.3.0/24")},
			BGPAdvertisements: []*config.BGPAdvertisement{
				{
					AggregationLength: 32,
				},
			},
		},
	}

	tests := []struct {
		desc    string
		config  *config.Config
		node    *v1.Node
		wantAds map[string][]*bgp.Advertisement
	}{
		{
			desc:    "No config, no advertisements",
			wantAds: map[string][]*bgp.Advertisement{},
		},

		{
			desc: "One peer, default node selector, no node labels",
			config: &config.Config{
				Peers: []*config.Peer{
					{
						Addr:          net.ParseIP("1.2.3.4"),
						NodeSelectors: []labels.Selector{labels.Everything()},
					},
				},
				Pools: pools,
			},
			wantAds: map[string][]*bgp.Advertisement{
				"1.2.3.4:0": nil,
			},
		},

		{
			desc: "Second peer, non-matching node selector",
			config: &config.Config{
				Peers: []*config.Peer{
					{
						Addr:          net.ParseIP("1.2.3.4"),
						NodeSelectors: []labels.Selector{labels.Everything()},
					},
					{
						Addr: net.ParseIP("2.3.4.5"),
						NodeSelectors: []labels.Selector{
							mustSelector("foo=bar"),
						},
					},
				},
				Pools: pools,
			},
			wantAds: map[string][]*bgp.Advertisement{
				"1.2.3.4:0": nil,
			},
		},

		{
			desc: "Add node label that matches",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
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
				Peers: []*config.Peer{
					{
						Addr:          net.ParseIP("1.2.3.4"),
						NodeSelectors: []labels.Selector{labels.Everything()},
					},
					{
						Addr: net.ParseIP("2.3.4.5"),
						NodeSelectors: []labels.Selector{
							mustSelector("foo in (bar, baz)"),
						},
					},
				},
				Pools: pools,
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
				Peers: []*config.Peer{
					{
						Addr:          net.ParseIP("1.2.3.4"),
						NodeSelectors: []labels.Selector{labels.Everything()},
					},
					{
						Addr: net.ParseIP("2.3.4.5"),
						NodeSelectors: []labels.Selector{
							mustSelector("host=frontend"),
							mustSelector("foo in (bar, baz)"),
						},
					},
				},
				Pools: pools,
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
	}

	l := log.NewNopLogger()
	for _, test := range tests {
		if test.config != nil {
			if c.SetConfig(l, test.config) == k8s.SyncStateError {
				t.Errorf("%q: SetConfig failed", test.desc)
			}
		}

		if test.node != nil {
			if c.SetNode(l, test.node) == k8s.SyncStateError {
				t.Errorf("%q: SetNode failed", test.desc)
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
