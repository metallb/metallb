package main

import (
	"errors"
	"net"
	"sort"
	"sync"
	"testing"
	"time"

	"go.universe.tf/metallb/internal/bgp"
	"go.universe.tf/metallb/internal/config"

	"github.com/google/go-cmp/cmp"
	"k8s.io/api/core/v1"
)

func strptr(s string) *string {
	return &s
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
	t *testing.T

	sync.Mutex
	// peer IP -> advertisements
	gotAds map[string][]*bgp.Advertisement
}

func (f *fakeBGP) New(addr string, _ uint32, _ net.IP, _ uint32, _ time.Duration) (session, error) {
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

func (f *fakeBGP) Ads() map[string][]*bgp.Advertisement {
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
	f    *fakeBGP
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

func TestBGPSppeaker(t *testing.T) {
	b := &fakeBGP{
		t:      t,
		gotAds: map[string][]*bgp.Advertisement{},
	}
	newBGP = b.New
	c, _ := newBGPController(net.ParseIP("1.2.3.4"), "pandora")

	tests := []struct {
		desc string

		balancer string
		config   *config.Config
		svc      *v1.Service
		eps      *v1.Endpoints

		wantAds map[string][]*bgp.Advertisement
	}{
		{
			desc:     "Service ignored, no config",
			balancer: "test1",
			svc: &v1.Service{
				Spec: v1.ServiceSpec{
					Type: "LoadBalancer",
					ExternalTrafficPolicy: "Cluster",
				},
				Status: statusAssigned("10.20.30.1"),
			},
			eps: &v1.Endpoints{
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
			wantAds: map[string][]*bgp.Advertisement{},
		},

		{
			desc: "One peer, no services",
			config: &config.Config{
				Peers: []*config.Peer{
					{
						Addr: net.ParseIP("1.2.3.4"),
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
					Type: "ClusterIP",
					ExternalTrafficPolicy: "Cluster",
				},
			},
			eps: &v1.Endpoints{
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
			wantAds: map[string][]*bgp.Advertisement{
				"1.2.3.4:0": nil,
			},
		},

		{
			desc:     "Add service, it's an LB!",
			balancer: "test1",
			svc: &v1.Service{
				Spec: v1.ServiceSpec{
					Type: "LoadBalancer",
					ExternalTrafficPolicy: "Cluster",
				},
				Status: statusAssigned("10.20.30.1"),
			},
			eps: &v1.Endpoints{
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
			wantAds: map[string][]*bgp.Advertisement{
				"1.2.3.4:0": {
					{
						Prefix:  ipnet("10.20.30.1/32"),
						NextHop: net.ParseIP("1.2.3.4"),
					},
				},
			},
		},

		{
			desc:     "LB switches to local traffic policy, endpoint isn't on our node",
			balancer: "test1",
			svc: &v1.Service{
				Spec: v1.ServiceSpec{
					Type: "LoadBalancer",
					ExternalTrafficPolicy: "Local",
				},
				Status: statusAssigned("10.20.30.1"),
			},
			eps: &v1.Endpoints{
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
			wantAds: map[string][]*bgp.Advertisement{
				"1.2.3.4:0": nil,
			},
		},

		{
			desc:     "New endpoint, on our node",
			balancer: "test1",
			svc: &v1.Service{
				Spec: v1.ServiceSpec{
					Type: "LoadBalancer",
					ExternalTrafficPolicy: "Local",
				},
				Status: statusAssigned("10.20.30.1"),
			},
			eps: &v1.Endpoints{
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
			wantAds: map[string][]*bgp.Advertisement{
				"1.2.3.4:0": {
					{
						Prefix:  ipnet("10.20.30.1/32"),
						NextHop: net.ParseIP("1.2.3.4"),
					},
				},
			},
		},

		{
			desc:     "Endpoint on our node has some unready ports",
			balancer: "test1",
			svc: &v1.Service{
				Spec: v1.ServiceSpec{
					Type: "LoadBalancer",
					ExternalTrafficPolicy: "Local",
				},
				Status: statusAssigned("10.20.30.1"),
			},
			eps: &v1.Endpoints{
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
			wantAds: map[string][]*bgp.Advertisement{
				"1.2.3.4:0": nil,
			},
		},

		{
			desc: "Multiple advertisement config",
			config: &config.Config{
				Peers: []*config.Peer{
					{
						Addr: net.ParseIP("1.2.3.4"),
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
					Type: "LoadBalancer",
					ExternalTrafficPolicy: "Cluster",
				},
				Status: statusAssigned("10.20.30.1"),
			},
			eps: &v1.Endpoints{
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
			wantAds: map[string][]*bgp.Advertisement{
				"1.2.3.4:0": {
					{
						Prefix:      ipnet("10.20.30.1/32"),
						NextHop:     net.ParseIP("1.2.3.4"),
						LocalPref:   100,
						Communities: []uint32{1234, 2345},
					},
					{
						Prefix:    ipnet("10.20.30.0/24"),
						NextHop:   net.ParseIP("1.2.3.4"),
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
						Addr: net.ParseIP("1.2.3.4"),
					},
					{
						Addr: net.ParseIP("1.2.3.5"),
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
					Type: "LoadBalancer",
					ExternalTrafficPolicy: "Cluster",
				},
				Status: statusAssigned("10.20.30.1"),
			},
			eps: &v1.Endpoints{
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
			wantAds: map[string][]*bgp.Advertisement{
				"1.2.3.4:0": {
					{
						Prefix:  ipnet("10.20.30.1/32"),
						NextHop: net.ParseIP("1.2.3.4"),
					},
				},
				"1.2.3.5:0": {
					{
						Prefix:  ipnet("10.20.30.1/32"),
						NextHop: net.ParseIP("1.2.3.4"),
					},
				},
			},
		},

		{
			desc:     "Second balancer, no ingress assigned",
			balancer: "test2",
			svc: &v1.Service{
				Spec: v1.ServiceSpec{
					Type: "LoadBalancer",
					ExternalTrafficPolicy: "Cluster",
				},
			},
			eps: &v1.Endpoints{
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
			wantAds: map[string][]*bgp.Advertisement{
				"1.2.3.4:0": {
					{
						Prefix:  ipnet("10.20.30.1/32"),
						NextHop: net.ParseIP("1.2.3.4"),
					},
				},
				"1.2.3.5:0": {
					{
						Prefix:  ipnet("10.20.30.1/32"),
						NextHop: net.ParseIP("1.2.3.4"),
					},
				},
			},
		},

		{
			desc:     "Second balancer, ingress gets assigned",
			balancer: "test2",
			svc: &v1.Service{
				Spec: v1.ServiceSpec{
					Type: "LoadBalancer",
					ExternalTrafficPolicy: "Cluster",
				},
				Status: statusAssigned("10.20.30.5"),
			},
			eps: &v1.Endpoints{
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
			wantAds: map[string][]*bgp.Advertisement{
				"1.2.3.4:0": {
					{
						Prefix:  ipnet("10.20.30.1/32"),
						NextHop: net.ParseIP("1.2.3.4"),
					},
					{
						Prefix:  ipnet("10.20.30.5/32"),
						NextHop: net.ParseIP("1.2.3.4"),
					},
				},
				"1.2.3.5:0": {
					{
						Prefix:  ipnet("10.20.30.1/32"),
						NextHop: net.ParseIP("1.2.3.4"),
					},
					{
						Prefix:  ipnet("10.20.30.5/32"),
						NextHop: net.ParseIP("1.2.3.4"),
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
						Prefix:  ipnet("10.20.30.5/32"),
						NextHop: net.ParseIP("1.2.3.4"),
					},
				},
				"1.2.3.5:0": {
					{
						Prefix:  ipnet("10.20.30.5/32"),
						NextHop: net.ParseIP("1.2.3.4"),
					},
				},
			},
		},

		{
			desc: "Delete peer",
			config: &config.Config{
				Peers: []*config.Peer{
					{
						Addr: net.ParseIP("1.2.3.5"),
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
					Type: "LoadBalancer",
					ExternalTrafficPolicy: "Cluster",
				},
				Status: statusAssigned("10.20.30.5"),
			},
			eps: &v1.Endpoints{
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
			wantAds: map[string][]*bgp.Advertisement{
				"1.2.3.5:0": {
					{
						Prefix:  ipnet("10.20.30.5/32"),
						NextHop: net.ParseIP("1.2.3.4"),
					},
				},
			},
		},
	}

	for _, test := range tests {
		if test.config != nil {
			if err := c.SetConfig(test.config); err != nil {
				t.Errorf("%q: SetConfig failed: %s", test.desc, err)
			}
		}
		if test.balancer != "" {
			if err := c.SetBalancer(test.balancer, test.svc, test.eps); err != nil {
				t.Errorf("%q: SetBalancer failed: %s", test.desc, err)
			}
		}

		gotAds := b.Ads()
		sortAds(test.wantAds)
		sortAds(gotAds)
		if diff := cmp.Diff(test.wantAds, gotAds); diff != "" {
			t.Errorf("%q: unexpected advertisement state (-want +got)\n%s", test.desc, diff)
		}
	}
}
