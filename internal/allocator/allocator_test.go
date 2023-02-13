// SPDX-License-Identifier:Apache-2.0

package allocator

import (
	"math"
	"net"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"go.universe.tf/metallb/internal/config"
	"go.universe.tf/metallb/internal/ipfamily"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"

	ptu "github.com/prometheus/client_golang/prometheus/testutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var svc = &v1.Service{
	ObjectMeta: metav1.ObjectMeta{
		Name: "test-lb-service",
	},
	Spec: v1.ServiceSpec{
		Type: v1.ServiceTypeLoadBalancer,
		Ports: []v1.ServicePort{
			{
				Protocol: v1.ProtocolTCP,
				Port:     8080,
			},
		},
	},
}

func selector(s string) labels.Selector {
	ret, err := labels.Parse(s)
	if err != nil {
		panic(err)
	}
	return ret
}

func TestAssignment(t *testing.T) {
	alloc := New()
	if err := alloc.SetPools(&config.Pools{ByName: map[string]*config.Pool{
		"test": {
			Name:       "test",
			AutoAssign: true,
			CIDR: []*net.IPNet{
				ipnet("1.2.3.4/31"),
				ipnet("1000::4/127"),
			},
		},
		"test2": {
			Name:          "test2",
			AvoidBuggyIPs: true,
			AutoAssign:    true,
			CIDR: []*net.IPNet{
				ipnet("1.2.4.0/24"),
				ipnet("1000::4:0/120"),
			},
		},
		"test3": {
			Name:               "test3",
			AvoidBuggyIPs:      true,
			AutoAssign:         true,
			ServiceAllocations: &config.ServiceAllocation{Namespaces: sets.New("test-ns1")},
			CIDR: []*net.IPNet{
				ipnet("1.2.5.0/24"),
				ipnet("1000::5:0/120"),
			},
		},
		"test4": {
			Name:               "test4",
			AvoidBuggyIPs:      true,
			AutoAssign:         true,
			ServiceAllocations: &config.ServiceAllocation{ServiceSelectors: []labels.Selector{selector("team=metallb")}},
			CIDR: []*net.IPNet{
				ipnet("1.2.6.0/24"),
				ipnet("1000::6:0/120"),
			},
		},
	}}); err != nil {
		t.Fatalf("SetPools: %s", err)
	}

	tests := []struct {
		desc       string
		svcKey     string
		svc        *v1.Service
		ips        []string
		ports      []Port
		sharingKey string
		backendKey string
		wantErr    bool
	}{
		{
			desc:   "assign s1",
			svcKey: "s1",
			svc:    svc,
			ips:    []string{"1.2.3.4"},
		},
		{
			desc:   "s1 idempotent reassign",
			svcKey: "s1",
			svc:    svc,
			ips:    []string{"1.2.3.4"},
		},
		{
			desc:    "s2 can't grab s1's IP",
			svcKey:  "s2",
			svc:     svc,
			ips:     []string{"1.2.3.4"},
			wantErr: true,
		},
		{
			desc:   "s2 can get the other IP",
			svcKey: "s2",
			svc:    svc,
			ips:    []string{"1.2.3.5"},
		},
		{
			desc:    "s1 now can't grab s2's IP",
			svcKey:  "s1",
			svc:     svc,
			ips:     []string{"1.2.3.5"},
			wantErr: true,
		},
		{
			desc:   "s1 frees its IP",
			svcKey: "s1",
			svc:    svc,
			ips:    []string{},
		},
		{
			desc:   "s2 can grab s1's former IP",
			svcKey: "s2",
			svc:    svc,
			ips:    []string{"1.2.3.4"},
		},
		{
			desc:   "s1 can now grab s2's former IP",
			svcKey: "s1",
			svc:    svc,
			ips:    []string{"1.2.3.5"},
		},
		{
			desc:    "s3 cannot grab a 0 buggy IP",
			svcKey:  "s3",
			svc:     svc,
			ips:     []string{"1.2.4.0"},
			wantErr: true,
		},
		{
			desc:    "s3 cannot grab a 255 buggy IP",
			svcKey:  "s3",
			svc:     svc,
			ips:     []string{"1.2.4.255"},
			wantErr: true,
		},
		{
			desc:   "s3 can grab another IP in that pool",
			svcKey: "s3",
			svc:    svc,
			ips:    []string{"1.2.4.254"},
		},
		{
			desc:       "s4 takes an IP, with sharing",
			svcKey:     "s4",
			svc:        svc,
			ips:        []string{"1.2.4.3"},
			ports:      ports("tcp/80"),
			sharingKey: "sharing",
			backendKey: "backend",
		},
		{
			desc:       "s4 changes its sharing key in place",
			svcKey:     "s4",
			svc:        svc,
			ips:        []string{"1.2.4.3"},
			ports:      ports("tcp/80"),
			sharingKey: "share",
			backendKey: "backend",
		},
		{
			desc:       "s3 can't share with s4 (port conflict)",
			svcKey:     "s3",
			svc:        svc,
			ips:        []string{"1.2.4.3"},
			ports:      ports("tcp/80"),
			sharingKey: "share",
			backendKey: "backend",
			wantErr:    true,
		},
		{
			desc:       "s3 can't share with s4 (wrong sharing key)",
			svcKey:     "s3",
			svc:        svc,
			ips:        []string{"1.2.4.3"},
			ports:      ports("tcp/443"),
			sharingKey: "othershare",
			backendKey: "backend",
			wantErr:    true,
		},
		{
			desc:       "s3 can't share with s4 (wrong backend key)",
			svcKey:     "s3",
			svc:        svc,
			ips:        []string{"1.2.4.3"},
			ports:      ports("tcp/443"),
			sharingKey: "share",
			backendKey: "otherbackend",
			wantErr:    true,
		},
		{
			desc:       "s3 takes the same IP as s4",
			svcKey:     "s3",
			svc:        svc,
			ips:        []string{"1.2.4.3"},
			ports:      ports("tcp/443"),
			sharingKey: "share",
			backendKey: "backend",
		},
		{
			desc:       "s3 can change its ports while keeping the same IP",
			svcKey:     "s3",
			svc:        svc,
			ips:        []string{"1.2.4.3"},
			ports:      ports("udp/53"),
			sharingKey: "share",
			backendKey: "backend",
		},
		{
			desc:       "s3 can't change its sharing key while keeping the same IP",
			svcKey:     "s3",
			svc:        svc,
			ips:        []string{"1.2.4.3"},
			ports:      ports("tcp/443"),
			sharingKey: "othershare",
			backendKey: "backend",
			wantErr:    true,
		},
		{
			desc:       "s3 can't change its backend key while keeping the same IP",
			svcKey:     "s3",
			svc:        svc,
			ips:        []string{"1.2.4.3"},
			ports:      ports("tcp/443"),
			sharingKey: "share",
			backendKey: "otherbackend",
			wantErr:    true,
		},
		{
			desc:   "s4 takes s3's former IP",
			svcKey: "s4",
			svc:    svc,
			ips:    []string{"1.2.4.254"},
		},

		// IPv6 tests (same as ipv4 but with ipv6 addresses)
		{
			desc:   "ipv6 assign s1",
			svcKey: "s1",
			svc:    svc,
			ips:    []string{"1000::4"},
		},
		{
			desc:   "s1 idempotent reassign",
			svcKey: "s1",
			svc:    svc,
			ips:    []string{"1000::4"},
		},
		{
			desc:    "s2 can't grab s1's IP",
			svcKey:  "s2",
			svc:     svc,
			ips:     []string{"1000::4"},
			wantErr: true,
		},
		{
			desc:   "s2 can get the other IP",
			svcKey: "s2",
			svc:    svc,
			ips:    []string{"1000::4:5"},
		},
		{
			desc:    "s1 now can't grab s2's IP",
			svcKey:  "s1",
			svc:     svc,
			ips:     []string{"1000::4:5"},
			wantErr: true,
		},
		{
			desc:   "s1 frees its IP",
			svcKey: "s1",
			svc:    svc,
			ips:    []string{},
		},
		{
			desc:   "s2 can grab s1's former IP",
			svcKey: "s2",
			svc:    svc,
			ips:    []string{"1000::4"},
		},
		{
			desc:   "s1 can now grab s2's former IP",
			svcKey: "s1",
			svc:    svc,
			ips:    []string{"1000::4:5"},
		},
		// (buggy-IP N/A for ipv6)
		{
			desc:   "s3 can grab another IP in that pool",
			svcKey: "s3",
			svc:    svc,
			ips:    []string{"1000::4:ff"},
		},
		{
			desc:       "s4 takes an IP, with sharing",
			svcKey:     "s4",
			svc:        svc,
			ips:        []string{"1000::4:3"},
			ports:      ports("tcp/80"),
			sharingKey: "sharing",
			backendKey: "backend",
		},
		{
			desc:       "s4 changes its sharing key in place",
			svcKey:     "s4",
			svc:        svc,
			ips:        []string{"1000::4:3"},
			ports:      ports("tcp/80"),
			sharingKey: "share",
			backendKey: "backend",
		},
		{
			desc:       "s3 can't share with s4 (port conflict)",
			svcKey:     "s3",
			svc:        svc,
			ips:        []string{"1000::4:3"},
			ports:      ports("tcp/80"),
			sharingKey: "share",
			backendKey: "backend",
			wantErr:    true,
		},
		{
			desc:       "s3 can't share with s4 (wrong sharing key)",
			svcKey:     "s3",
			svc:        svc,
			ips:        []string{"1000::4:3"},
			ports:      ports("tcp/443"),
			sharingKey: "othershare",
			backendKey: "backend",
			wantErr:    true,
		},
		{
			desc:       "s3 can't share with s4 (wrong backend key)",
			svcKey:     "s3",
			svc:        svc,
			ips:        []string{"1000::4:3"},
			ports:      ports("tcp/443"),
			sharingKey: "share",
			backendKey: "otherbackend",
			wantErr:    true,
		},
		{
			desc:       "s3 takes the same IP as s4",
			svcKey:     "s3",
			svc:        svc,
			ips:        []string{"1000::4:3"},
			ports:      ports("tcp/443"),
			sharingKey: "share",
			backendKey: "backend",
		},
		{
			desc:       "s3 can change its ports while keeping the same IP",
			svcKey:     "s3",
			svc:        svc,
			ips:        []string{"1000::4:3"},
			ports:      ports("udp/53"),
			sharingKey: "share",
			backendKey: "backend",
		},
		{
			desc:       "s3 can't change its sharing key while keeping the same IP",
			svcKey:     "s3",
			svc:        svc,
			ips:        []string{"1000::4:3"},
			ports:      ports("tcp/443"),
			sharingKey: "othershare",
			backendKey: "backend",
			wantErr:    true,
		},
		{
			desc:       "s3 can't change its backend key while keeping the same IP",
			svcKey:     "s3",
			svc:        svc,
			ips:        []string{"1000::4:3"},
			ports:      ports("tcp/443"),
			sharingKey: "share",
			backendKey: "otherbackend",
			wantErr:    true,
		},
		{
			desc:   "s4 takes s3's former IP",
			svcKey: "s4",
			svc:    svc,
			ips:    []string{"1000::4:ff"},
		},
		// IP dual-stack test
		{
			desc:   "s2 frees its IP",
			svcKey: "s2",
			svc:    svc,
			ips:    []string{},
		},
		{
			desc:   "ip dual-stack assign s1",
			svcKey: "s1",
			svc:    svc,
			ips:    []string{"1.2.3.4", "1000::4"},
		},
		{
			desc:    "ip dual-stack assign s1 from different pools",
			svcKey:  "s1",
			svc:     svc,
			ips:     []string{"1.2.4.5", "1000::4"},
			wantErr: true,
		},

		// IP Pool compatibility tests
		{
			desc:   "attempt to assign ip from pool pinned on same namespace",
			svcKey: "s1",
			svc: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test-ns1",
					Name:      "s1",
				},
				Spec: v1.ServiceSpec{
					Type: v1.ServiceTypeLoadBalancer,
					Ports: []v1.ServicePort{
						{
							Protocol: v1.ProtocolTCP,
							Port:     8080,
						},
					},
				},
			},
			ips:     []string{"1.2.5.10"},
			wantErr: false,
		},
		{
			desc:   "attempt to assign ip from pool pinned on different namespace",
			svcKey: "s1",
			svc: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "testns-not-exist",
					Name:      "s1",
				},
				Spec: v1.ServiceSpec{
					Type: v1.ServiceTypeLoadBalancer,
					Ports: []v1.ServicePort{
						{
							Protocol: v1.ProtocolTCP,
							Port:     8080,
						},
					},
				},
			},
			ips:     []string{"1.2.5.10"},
			wantErr: true,
		},
		{
			desc:   "attempt to assign ip from pool pinned with same service label",
			svcKey: "s1",
			svc: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "testns-not-exist",
					Name:      "s1",
					Labels:    map[string]string{"team": "metallb"},
				},
				Spec: v1.ServiceSpec{
					Type:     v1.ServiceTypeLoadBalancer,
					Selector: map[string]string{"team": "metallb"},
					Ports: []v1.ServicePort{
						{
							Protocol: v1.ProtocolTCP,
							Port:     8080,
						},
					},
				},
			},
			ips:     []string{"1.2.6.10"},
			wantErr: false,
		},
		{
			desc:   "attempt to assign ip from pool pinned with different service label",
			svcKey: "s1",
			svc: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "testns-not-exist",
					Name:      "s1",
					Labels:    map[string]string{"team": "red"},
				},
				Spec: v1.ServiceSpec{
					Type:     v1.ServiceTypeLoadBalancer,
					Selector: map[string]string{"team": "red"},
					Ports: []v1.ServicePort{
						{
							Protocol: v1.ProtocolTCP,
							Port:     8080,
						},
					},
				},
			},
			ips:     []string{"1.2.6.10"},
			wantErr: true,
		},
	}

	for _, test := range tests {
		if len(test.ips) == 0 {
			alloc.Unassign(test.svcKey)
			continue
		}
		ips := []net.IP{}
		for _, ip := range test.ips {
			ips = append(ips, net.ParseIP(ip))
		}
		if len(ips) == 0 {
			t.Fatalf("invalid IPs %q in test %q", ips, test.desc)
		}
		alreadyHasIPs := reflect.DeepEqual(assigned(alloc, test.svcKey), test.ips)
		err := alloc.Assign(test.svcKey, test.svc, ips, test.ports, test.sharingKey, test.backendKey)
		if test.wantErr {
			if err == nil {
				t.Errorf("%q should have caused an error, but did not", test.desc)
			} else if a := assigned(alloc, test.svcKey); !alreadyHasIPs && reflect.DeepEqual(a, test.ips) {
				t.Errorf("%q: Assign(%q, %q) failed, but allocator did record allocation", test.desc, test.svcKey, test.ips)
			}

			continue
		}

		if err != nil {
			t.Errorf("%q: Assign(%q, %q): %s", test.desc, test.svcKey, test.ips, err)
		}
		if a := assigned(alloc, test.svcKey); !reflect.DeepEqual(a, test.ips) {
			t.Errorf("%q: ran Assign(%q, %q), but allocator has recorded allocation of %q", test.desc, test.svcKey, test.ips, a)
		}
	}
}

func TestPoolAllocation(t *testing.T) {
	alloc := New()
	// This test only allocates from the "test" pool, so it will run
	// out of IPs quickly even though there are tons available in
	// other pools.
	if err := alloc.SetPools(&config.Pools{ByName: map[string]*config.Pool{
		"not_this_one": {
			Name:       "not_this_one",
			AutoAssign: true,
			CIDR:       []*net.IPNet{ipnet("192.168.0.0/16"), ipnet("fc00::1:0/112")},
		},
		"test": {
			Name:       "test",
			AutoAssign: true,
			CIDR: []*net.IPNet{
				ipnet("1.2.3.4/31"),
				ipnet("1.2.3.10/31"),
				ipnet("1000::/127"),
				ipnet("2000::/127"),
			},
		},
		"test2": {
			Name:       "test2",
			AutoAssign: true,
			CIDR:       []*net.IPNet{ipnet("10.20.30.0/24"), ipnet("fc00::2:0/120")},
		},
	}}); err != nil {
		t.Fatalf("SetPools: %s", err)
	}

	validIP4s := map[string]bool{
		"1.2.3.4":  true,
		"1.2.3.5":  true,
		"1.2.3.10": true,
		"1.2.3.11": true,
	}
	validIP6s := map[string]bool{
		"1000::":  true,
		"1000::1": true,
		"2000::":  true,
		"2000::1": true,
	}
	validIPDualStacks := map[string]bool{
		"1.2.3.4":  true,
		"1.2.3.5":  true,
		"1.2.3.10": true,
		"1.2.3.11": true,
		"1000::":   true,
		"1000::1":  true,
		"2000::":   true,
		"2000::1":  true,
	}

	tests := []struct {
		desc       string
		svcKey     string
		svc        *v1.Service
		ports      []Port
		sharingKey string
		unassign   bool
		wantErr    bool
		ipFamily   ipfamily.Family
	}{
		{
			desc:     "s1 gets an IPv4",
			svcKey:   "s1",
			svc:      svc,
			ipFamily: ipfamily.IPv4,
		},
		{
			desc:     "s2 gets an IPv4",
			svcKey:   "s2",
			svc:      svc,
			ipFamily: ipfamily.IPv4,
		},
		{
			desc:     "s3 gets an IPv4",
			svcKey:   "s3",
			svc:      svc,
			ipFamily: ipfamily.IPv4,
		},
		{
			desc:     "s4 gets an IPv4",
			svcKey:   "s4",
			svc:      svc,
			ipFamily: ipfamily.IPv4,
		},
		{
			desc:     "s5 can't get an IPv4",
			svcKey:   "s5",
			svc:      svc,
			wantErr:  true,
			ipFamily: ipfamily.IPv4,
		},
		{
			desc:     "s6 can't get an IPv4",
			svcKey:   "s6",
			svc:      svc,
			wantErr:  true,
			ipFamily: ipfamily.IPv4,
		},
		{
			desc:     "s1 releases its IPv4",
			svcKey:   "s1",
			svc:      svc,
			unassign: true,
			ipFamily: ipfamily.IPv4,
		},
		{
			desc:     "s5 can now grab s1's former IPv4",
			svcKey:   "s5",
			svc:      svc,
			ipFamily: ipfamily.IPv4,
		},
		{
			desc:     "s6 still can't get an IPv4",
			svcKey:   "s6",
			svc:      svc,
			wantErr:  true,
			ipFamily: ipfamily.IPv4,
		},
		{
			desc:     "s5 unassigns in prep for enabling IPv4 sharing",
			svcKey:   "s5",
			svc:      svc,
			unassign: true,
			ipFamily: ipfamily.IPv4,
		},
		{
			desc:       "s5 enables IPv4 sharing",
			svcKey:     "s5",
			svc:        svc,
			ports:      ports("tcp/80"),
			sharingKey: "share",
			ipFamily:   ipfamily.IPv4,
		},
		{
			desc:       "s6 can get an IPv4 now, with sharing",
			svcKey:     "s6",
			svc:        svc,
			ports:      ports("tcp/443"),
			sharingKey: "share",
			ipFamily:   ipfamily.IPv4,
		},

		// Clear old ipv4 addresses
		{
			desc:     "s1 clear old ipv4 address",
			svcKey:   "s1",
			svc:      svc,
			unassign: true,
			ipFamily: ipfamily.IPv4,
		},
		{
			desc:     "s2 clear old ipv4 address",
			svcKey:   "s2",
			svc:      svc,
			unassign: true,
			ipFamily: ipfamily.IPv4,
		},
		{
			desc:     "s3 clear old ipv4 address",
			svcKey:   "s3",
			svc:      svc,
			unassign: true,
			ipFamily: ipfamily.IPv4,
		},
		{
			desc:     "s4 clear old ipv4 address",
			svcKey:   "s4",
			svc:      svc,
			unassign: true,
			ipFamily: ipfamily.IPv4,
		},
		{
			desc:     "s5 clear old ipv4 address",
			svcKey:   "s5",
			svc:      svc,
			unassign: true,
			ipFamily: ipfamily.IPv4,
		},
		{
			desc:     "s6 clear old ipv4 address",
			svcKey:   "s6",
			svc:      svc,
			unassign: true,
			ipFamily: ipfamily.IPv4,
		},

		// IPv6 tests.
		{
			desc:     "s1 gets an IP6",
			svcKey:   "s1",
			svc:      svc,
			ipFamily: ipfamily.IPv6,
		},
		{
			desc:     "s2 gets an IP6",
			svcKey:   "s2",
			svc:      svc,
			ipFamily: ipfamily.IPv6,
		},
		{
			desc:     "s3 gets an IP6",
			svcKey:   "s3",
			svc:      svc,
			ipFamily: ipfamily.IPv6,
		},
		{
			desc:     "s4 gets an IP6",
			svcKey:   "s4",
			svc:      svc,
			ipFamily: ipfamily.IPv6,
		},
		{
			desc:     "s5 can't get an IP6",
			svcKey:   "s5",
			svc:      svc,
			ipFamily: ipfamily.IPv6,
			wantErr:  true,
		},
		{
			desc:     "s6 can't get an IP6",
			svcKey:   "s6",
			svc:      svc,
			ipFamily: ipfamily.IPv6,
			wantErr:  true,
		},
		{
			desc:     "s1 releases its IP6",
			svcKey:   "s1",
			svc:      svc,
			unassign: true,
			ipFamily: ipfamily.IPv4,
		},
		{
			desc:     "s5 can now grab s1's former IP6",
			svcKey:   "s5",
			svc:      svc,
			ipFamily: ipfamily.IPv6,
		},
		{
			desc:     "s6 still can't get an IP6",
			svcKey:   "s6",
			svc:      svc,
			ipFamily: ipfamily.IPv6,
			wantErr:  true,
		},
		{
			desc:     "s5 unassigns in prep for enabling IP6 sharing",
			svcKey:   "s5",
			svc:      svc,
			unassign: true,
			ipFamily: ipfamily.IPv4,
		},
		{
			desc:       "s5 enables IP6 sharing",
			svcKey:     "s5",
			svc:        svc,
			ports:      ports("tcp/80"),
			sharingKey: "share",
			ipFamily:   ipfamily.IPv6,
		},
		{
			desc:       "s6 can get an IP6 now, with sharing",
			svcKey:     "s6",
			svc:        svc,
			ports:      ports("tcp/443"),
			sharingKey: "share",
			ipFamily:   ipfamily.IPv6,
		},

		// Test the "should-not-happen" case where an svc already has a IP from the wrong family
		{
			desc:     "s1 clear",
			svcKey:   "s1",
			svc:      svc,
			unassign: true,
			ipFamily: ipfamily.IPv4,
		},
		{
			desc:     "s1 get an IPv4",
			svcKey:   "s1",
			svc:      svc,
			ipFamily: ipfamily.IPv4,
		},
		{
			desc:     "s1 get an IPv6",
			svcKey:   "s1",
			svc:      svc,
			ipFamily: ipfamily.IPv6,
			wantErr:  true,
		},
		// Clear old ipv6 addresses
		{
			desc:     "s1 clear old ipv6 address",
			svcKey:   "s1",
			svc:      svc,
			unassign: true,
			ipFamily: ipfamily.IPv6,
		},
		{
			desc:     "s2 clear old ipv6 address",
			svcKey:   "s2",
			svc:      svc,
			unassign: true,
			ipFamily: ipfamily.IPv6,
		},
		{
			desc:     "s3 clear old ipv6 address",
			svcKey:   "s3",
			svc:      svc,
			unassign: true,
			ipFamily: ipfamily.IPv6,
		},
		{
			desc:     "s4 clear old ipv6 address",
			svcKey:   "s4",
			svc:      svc,
			unassign: true,
			ipFamily: ipfamily.IPv6,
		},
		{
			desc:     "s5 clear old ipv6 address",
			svcKey:   "s5",
			svc:      svc,
			unassign: true,
			ipFamily: ipfamily.IPv6,
		},
		{
			desc:     "s6 clear old ipv6 address",
			svcKey:   "s6",
			svc:      svc,
			unassign: true,
			ipFamily: ipfamily.IPv6,
		},

		// Dual-stack tests.
		{
			desc:     "s1 gets dual-stack IPs",
			svcKey:   "s1",
			svc:      svc,
			ipFamily: ipfamily.DualStack,
		},
		{
			desc:     "s2 gets dual-stack IPs",
			svcKey:   "s2",
			svc:      svc,
			ipFamily: ipfamily.DualStack,
		},
		{
			desc:     "s3 gets dual-stack IPs",
			svcKey:   "s3",
			svc:      svc,
			ipFamily: ipfamily.DualStack,
		},
		{
			desc:     "s4 gets dual-stack IPs",
			svcKey:   "s4",
			svc:      svc,
			ipFamily: ipfamily.DualStack,
		},
		{
			desc:     "s5 can't get dual-stack IPs",
			svcKey:   "s5",
			svc:      svc,
			ipFamily: ipfamily.DualStack,
			wantErr:  true,
		},
		{
			desc:     "s6 can't get dual-stack IPs",
			svcKey:   "s6",
			svc:      svc,
			ipFamily: ipfamily.DualStack,
			wantErr:  true,
		},
		{
			desc:     "s1 releases its dual-stack IPs",
			svcKey:   "s1",
			svc:      svc,
			unassign: true,
			ipFamily: ipfamily.DualStack,
		},
		{
			desc:     "s5 can now grab s1's former dual-stack IPs",
			svcKey:   "s5",
			svc:      svc,
			ipFamily: ipfamily.DualStack,
		},
		{
			desc:     "s6 still can't get dual-stack IPs",
			svcKey:   "s6",
			svc:      svc,
			ipFamily: ipfamily.DualStack,
			wantErr:  true,
		},
		{
			desc:     "s5 unassigns in prep for enabling dual-stack IPs sharing",
			svcKey:   "s5",
			svc:      svc,
			unassign: true,
			ipFamily: ipfamily.DualStack,
		},
		{
			desc:       "s5 enables dual-stack IP sharing",
			svcKey:     "s5",
			svc:        svc,
			ports:      ports("tcp/80"),
			sharingKey: "share",
			ipFamily:   ipfamily.DualStack,
		},
		{
			desc:       "s6 can get an dual-stack IPs now, with sharing",
			svcKey:     "s6",
			svc:        svc,
			ports:      ports("tcp/443"),
			sharingKey: "share",
			ipFamily:   ipfamily.DualStack,
		},
	}

	for _, test := range tests {
		if test.unassign {
			alloc.Unassign(test.svcKey)
			continue
		}
		ips, err := alloc.AllocateFromPool(test.svcKey, test.svc, test.ipFamily, "test", test.ports, test.sharingKey, "")
		if test.wantErr {
			if err == nil {
				t.Errorf("%s: should have caused an error, but did not", test.desc)

			}
			continue
		}
		if err != nil {
			t.Errorf("%s: AllocateFromPool(%q, \"test\"): %s", test.desc, test.svcKey, err)
		}
		validIPs := validIP4s
		if test.ipFamily == ipfamily.IPv6 {
			validIPs = validIP6s
		} else if test.ipFamily == ipfamily.DualStack {
			validIPs = validIPDualStacks
		}
		for _, ip := range ips {
			if !validIPs[ip.String()] {
				t.Errorf("%s: allocated unexpected IP %q", test.desc, ip)
			}
		}
	}

	alloc.Unassign("s5")
	if _, err := alloc.AllocateFromPool("s5", svc, ipfamily.IPv4, "nonexistentpool", nil, "", ""); err == nil {
		t.Error("Allocating from non-existent pool succeeded")
	}
}

func TestAllocation(t *testing.T) {
	alloc := New()
	if err := alloc.SetPools(&config.Pools{ByName: map[string]*config.Pool{
		"test1": {
			Name:       "test1",
			AutoAssign: true,
			CIDR:       []*net.IPNet{ipnet("1.2.3.4/31"), ipnet("1000::4/127")},
		},
		"test2": {
			Name:       "test2",
			AutoAssign: true,
			CIDR:       []*net.IPNet{ipnet("1.2.3.10/31"), ipnet("1000::10/127")},
		},
	}}); err != nil {
		t.Fatalf("SetPools: %s", err)
	}

	validIP4s := map[string]bool{
		"1.2.3.4":  true,
		"1.2.3.5":  true,
		"1.2.3.10": true,
		"1.2.3.11": true,
	}
	validIP6s := map[string]bool{
		"1000::4":  true,
		"1000::5":  true,
		"1000::10": true,
		"1000::11": true,
	}
	validIPDualStacks := map[string]bool{
		"1.2.3.4":  true,
		"1.2.3.5":  true,
		"1.2.3.10": true,
		"1.2.3.11": true,
		"1000::4":  true,
		"1000::5":  true,
		"1000::10": true,
		"1000::11": true,
	}
	tests := []struct {
		desc       string
		svcKey     string
		svc        *v1.Service
		ports      []Port
		sharingKey string
		unassign   bool
		wantErr    bool
		ipFamily   ipfamily.Family
	}{
		{
			desc:     "s1 gets an IPv4",
			svcKey:   "s1",
			svc:      svc,
			ipFamily: ipfamily.IPv4,
		},
		{
			desc:     "s2 gets an IPv4",
			svcKey:   "s2",
			svc:      svc,
			ipFamily: ipfamily.IPv4,
		},
		{
			desc:     "s3 gets an IPv4",
			svcKey:   "s3",
			svc:      svc,
			ipFamily: ipfamily.IPv4,
		},
		{
			desc:     "s4 gets an IPv4",
			svcKey:   "s4",
			svc:      svc,
			ipFamily: ipfamily.IPv4,
		},
		{
			desc:     "s5 can't get an IPv4",
			svcKey:   "s5",
			svc:      svc,
			wantErr:  true,
			ipFamily: ipfamily.IPv4,
		},
		{
			desc:     "s6 can't get an IPv4",
			svcKey:   "s6",
			svc:      svc,
			wantErr:  true,
			ipFamily: ipfamily.IPv4,
		},
		{
			desc:     "s1 gives up its IPv4",
			svcKey:   "s1",
			svc:      svc,
			unassign: true,
			ipFamily: ipfamily.IPv4,
		},
		{
			desc:       "s5 can now get an IPv4",
			svcKey:     "s5",
			svc:        svc,
			ports:      ports("tcp/80"),
			sharingKey: "share",
			ipFamily:   ipfamily.IPv4,
		},
		{
			desc:     "s6 still can't get an IPv4",
			svcKey:   "s6",
			svc:      svc,
			wantErr:  true,
			ipFamily: ipfamily.IPv4,
		},
		{
			desc:       "s6 can get an IPv4 with sharing",
			svcKey:     "s6",
			svc:        svc,
			ports:      ports("tcp/443"),
			sharingKey: "share",
			ipFamily:   ipfamily.IPv4,
		},

		// Clear old ipv4 addresses
		{
			desc:     "s1 clear old ipv4 address",
			svcKey:   "s1",
			svc:      svc,
			unassign: true,
			ipFamily: ipfamily.IPv4,
		},
		{
			desc:     "s2 clear old ipv4 address",
			svcKey:   "s2",
			svc:      svc,
			unassign: true,
			ipFamily: ipfamily.IPv4,
		},
		{
			desc:     "s3 clear old ipv4 address",
			svcKey:   "s3",
			svc:      svc,
			unassign: true,
			ipFamily: ipfamily.IPv4,
		},
		{
			desc:     "s4 clear old ipv4 address",
			svcKey:   "s4",
			svc:      svc,
			unassign: true,
			ipFamily: ipfamily.IPv4,
		},
		{
			desc:     "s5 clear old ipv4 address",
			svcKey:   "s5",
			svc:      svc,
			unassign: true,
			ipFamily: ipfamily.IPv4,
		},
		{
			desc:     "s6 clear old ipv4 address",
			svcKey:   "s6",
			svc:      svc,
			unassign: true,
			ipFamily: ipfamily.IPv4,
		},

		// IPv6 tests
		{
			desc:     "s1 gets an IPv6",
			svcKey:   "s1",
			svc:      svc,
			ipFamily: ipfamily.IPv6,
		},
		{
			desc:     "s2 gets an IPv6",
			svcKey:   "s2",
			svc:      svc,
			ipFamily: ipfamily.IPv6,
		},
		{
			desc:     "s3 gets an IPv6",
			svcKey:   "s3",
			svc:      svc,
			ipFamily: ipfamily.IPv6,
		},
		{
			desc:     "s4 gets an IPv6",
			svcKey:   "s4",
			svc:      svc,
			ipFamily: ipfamily.IPv6,
		},
		{
			desc:     "s5 can't get an IPv6",
			svcKey:   "s5",
			svc:      svc,
			ipFamily: ipfamily.IPv6,
			wantErr:  true,
		},
		{
			desc:     "s6 can't get an IPv6",
			svcKey:   "s6",
			svc:      svc,
			ipFamily: ipfamily.IPv6,
			wantErr:  true,
		},
		{
			desc:     "s1 gives up its IPv6",
			svcKey:   "s1",
			svc:      svc,
			unassign: true,
		},
		{
			desc:       "s5 can now get an IPv6",
			svcKey:     "s5",
			svc:        svc,
			ports:      ports("tcp/80"),
			sharingKey: "share",
			ipFamily:   ipfamily.IPv6,
		},
		{
			desc:     "s6 still can't get an IPv6",
			svcKey:   "s6",
			svc:      svc,
			ipFamily: ipfamily.IPv6,
			wantErr:  true,
		},
		{
			desc:       "s6 can get an IPv6 with sharing",
			svcKey:     "s6",
			svc:        svc,
			ports:      ports("tcp/443"),
			sharingKey: "share",
			ipFamily:   ipfamily.IPv6,
		},
		// Clear old ipv6 addresses
		{
			desc:     "s1 clear old ipv6 address",
			svcKey:   "s1",
			svc:      svc,
			unassign: true,
			ipFamily: ipfamily.IPv6,
		},
		{
			desc:     "s2 clear old ipv6 address",
			svcKey:   "s2",
			svc:      svc,
			unassign: true,
			ipFamily: ipfamily.IPv6,
		},
		{
			desc:     "s3 clear old ipv6 address",
			svcKey:   "s3",
			svc:      svc,
			unassign: true,
			ipFamily: ipfamily.IPv6,
		},
		{
			desc:     "s4 clear old ipv6 address",
			svcKey:   "s4",
			svc:      svc,
			unassign: true,
			ipFamily: ipfamily.IPv6,
		},
		{
			desc:     "s5 clear old ipv6 address",
			svcKey:   "s5",
			svc:      svc,
			unassign: true,
			ipFamily: ipfamily.IPv6,
		},
		{
			desc:     "s6 clear old ipv6 address",
			svcKey:   "s6",
			svc:      svc,
			unassign: true,
			ipFamily: ipfamily.IPv6,
		},
		//Dual-stack test cases
		{
			desc:     "s1 gets dual-stack IPs",
			svcKey:   "s1",
			svc:      svc,
			ipFamily: ipfamily.DualStack,
		},
		{
			desc:     "s2 gets dual-stack IPs",
			svcKey:   "s2",
			svc:      svc,
			ipFamily: ipfamily.DualStack,
		},
		{
			desc:     "s3 gets dual-stack IPs",
			svcKey:   "s3",
			svc:      svc,
			ipFamily: ipfamily.DualStack,
		},
		{
			desc:     "s4 gets dual-stack IPs",
			svcKey:   "s4",
			svc:      svc,
			ipFamily: ipfamily.DualStack,
		},
		{
			desc:     "s5 can't get dual-stack IPs",
			svcKey:   "s5",
			svc:      svc,
			ipFamily: ipfamily.DualStack,
			wantErr:  true,
		},
		{
			desc:     "s6 can't get dual-stack IPs",
			svcKey:   "s6",
			svc:      svc,
			ipFamily: ipfamily.DualStack,
			wantErr:  true,
		},
		{
			desc:     "s1 gives up its IPs",
			svcKey:   "s1",
			svc:      svc,
			unassign: true,
		},
		{
			desc:       "s5 can now get dual-stack IPs",
			svcKey:     "s5",
			svc:        svc,
			ports:      ports("tcp/80"),
			sharingKey: "share",
			ipFamily:   ipfamily.DualStack,
		},
		{
			desc:     "s6 still can't get dual-stack IPs",
			svcKey:   "s6",
			svc:      svc,
			ipFamily: ipfamily.DualStack,
			wantErr:  true,
		},
		{
			desc:       "s6 can get dual-stack IPs with sharing",
			svcKey:     "s6",
			svc:        svc,
			ports:      ports("tcp/443"),
			sharingKey: "share",
			ipFamily:   ipfamily.DualStack,
		},
	}

	for _, test := range tests {
		if test.unassign {
			alloc.Unassign(test.svcKey)
			continue
		}
		ips, err := alloc.Allocate(test.svcKey, test.svc, test.ipFamily, test.ports, test.sharingKey, "")
		if test.wantErr {
			if err == nil {
				t.Errorf("%s: should have caused an error, but did not", test.desc)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: Allocate(%q, \"test\"): %s", test.desc, test.svcKey, err)
		}

		validIPs := validIP4s
		if test.ipFamily == ipfamily.IPv6 {
			validIPs = validIP6s
		} else if test.ipFamily == ipfamily.DualStack {
			validIPs = validIPDualStacks
		}
		for _, ip := range ips {
			if !validIPs[ip.String()] {
				t.Errorf("%s allocated unexpected IP %q", test.desc, ip)
			}
		}
	}
}

func TestBuggyIPs(t *testing.T) {
	alloc := New()
	if err := alloc.SetPools(&config.Pools{ByName: map[string]*config.Pool{
		"test": {
			Name:       "test",
			AutoAssign: true,
			CIDR:       []*net.IPNet{ipnet("1.2.3.0/31")},
		},
		"test2": {
			Name:       "test2",
			AutoAssign: true,
			CIDR:       []*net.IPNet{ipnet("1.2.3.254/31")},
		},
		"test3": {
			Name:          "test3",
			AvoidBuggyIPs: true,
			AutoAssign:    true,
			CIDR:          []*net.IPNet{ipnet("1.2.4.0/31")},
		},
		"test4": {
			Name:          "test4",
			AvoidBuggyIPs: true,
			AutoAssign:    true,
			CIDR:          []*net.IPNet{ipnet("1.2.4.254/31")},
		},
	}}); err != nil {
		t.Fatalf("SetPools: %s", err)
	}

	validIPs := map[string]bool{
		"1.2.3.0":   true,
		"1.2.3.1":   true,
		"1.2.3.254": true,
		"1.2.3.255": true,
		"1.2.4.1":   true,
		"1.2.4.254": true,
	}

	tests := []struct {
		svcKey  string
		svc     *v1.Service
		wantErr bool
	}{
		{svcKey: "s1", svc: svc},
		{svcKey: "s2", svc: svc},
		{svcKey: "s3", svc: svc},
		{svcKey: "s4", svc: svc},
		{svcKey: "s5", svc: svc},
		{svcKey: "s6", svc: svc},
		{
			svcKey:  "s7",
			svc:     svc,
			wantErr: true,
		},
	}

	for i, test := range tests {
		ips, err := alloc.Allocate(test.svcKey, test.svc, ipfamily.IPv4, nil, "", "")
		if test.wantErr {
			if err == nil {
				t.Errorf("#%d should have caused an error, but did not", i+1)

			}
			continue
		}
		if err != nil {
			t.Errorf("#%d Allocate(%q, \"test\"): %s", i+1, test.svcKey, err)
		}
		for _, ip := range ips {
			if !validIPs[ip.String()] {
				t.Errorf("#%d allocated unexpected IP %q", i+1, ip)
			}
		}
	}

}

func TestConfigReload(t *testing.T) {
	alloc := New()
	if err := alloc.SetPools(&config.Pools{ByName: map[string]*config.Pool{
		"test": {
			Name:       "test",
			AutoAssign: true,
			CIDR:       []*net.IPNet{ipnet("1.2.3.0/30"), ipnet("1000::/126")},
		},
	}}); err != nil {
		t.Fatalf("SetPools: %s", err)
	}
	if err := alloc.Assign("s1", svc, []net.IP{net.ParseIP("1.2.3.0")}, nil, "", ""); err != nil {
		t.Fatalf("Assign(s1, 1.2.3.0): %s", err)
	}
	if err := alloc.Assign("s2", svc, []net.IP{net.ParseIP("1000::")}, nil, "", ""); err != nil {
		t.Fatalf("Assign(s2, 1000::): %s", err)
	}
	tests := []struct {
		desc    string
		pools   map[string]*config.Pool
		wantErr bool
		pool    string // Pool that 1.2.3.0 and 1000:: should be in
	}{
		{
			desc: "set same config is no-op",
			pools: map[string]*config.Pool{
				"test": {
					Name:       "test",
					AutoAssign: true,
					CIDR:       []*net.IPNet{ipnet("1.2.3.0/30"), ipnet("1000::/126")},
				},
			},
			pool: "test",
		},
		{
			desc: "expand pool",
			pools: map[string]*config.Pool{
				"test": {
					Name:       "test",
					AutoAssign: true,
					CIDR:       []*net.IPNet{ipnet("1.2.3.0/24"), ipnet("1000::/120")},
				},
			},
			pool: "test",
		},
		{
			desc: "shrink pool",
			pools: map[string]*config.Pool{
				"test": {
					Name:       "test",
					AutoAssign: true,
					CIDR:       []*net.IPNet{ipnet("1.2.3.0/30"), ipnet("1000::/126")},
				},
			},
			pool: "test",
		},
		{
			desc: "can't shrink further",
			pools: map[string]*config.Pool{
				"test": {
					Name:       "test",
					AutoAssign: true,
					CIDR:       []*net.IPNet{ipnet("1.2.3.2/31"), ipnet("1000::0/126")},
				},
			},
			pool:    "test",
			wantErr: true,
		},
		{
			desc: "can't shrink further ipv6",
			pools: map[string]*config.Pool{
				"test": {
					Name:       "test",
					AutoAssign: true,
					CIDR:       []*net.IPNet{ipnet("1.2.3.0/30"), ipnet("1000::2/127")},
				},
			},
			pool:    "test",
			wantErr: true,
		},
		{
			desc: "rename the pool",
			pools: map[string]*config.Pool{
				"test2": {
					Name:       "test2",
					AutoAssign: true,
					CIDR:       []*net.IPNet{ipnet("1.2.3.0/30"), ipnet("1000::0/126")},
				},
			},
			pool: "test2",
		},
		{
			desc: "split pool",
			pools: map[string]*config.Pool{
				"test": {
					Name:       "test",
					AutoAssign: true,
					CIDR:       []*net.IPNet{ipnet("1.2.3.0/31"), ipnet("1000::/127")},
				},
				"test2": {
					Name:       "test2",
					AutoAssign: true,
					CIDR:       []*net.IPNet{ipnet("1.2.3.2/31"), ipnet("1000::2/127")},
				},
			},
			pool: "test",
		},
		{
			desc: "swap pool names",
			pools: map[string]*config.Pool{
				"test2": {
					Name:       "test2",
					AutoAssign: true,
					CIDR:       []*net.IPNet{ipnet("1.2.3.0/31"), ipnet("1000::/127")},
				},
				"test": {
					Name:       "test",
					AutoAssign: true,
					CIDR:       []*net.IPNet{ipnet("1.2.3.2/31"), ipnet("1000::2/127")},
				},
			},
			pool: "test2",
		},
		{
			desc: "delete used pool",
			pools: map[string]*config.Pool{
				"test": {
					Name:       "test",
					AutoAssign: true,
					CIDR:       []*net.IPNet{ipnet("1.2.3.2/31"), ipnet("1000::/126")},
				},
			},
			pool:    "test2",
			wantErr: true,
		},
		{
			desc: "delete used pool ipv6",
			pools: map[string]*config.Pool{
				"test": {
					Name:       "test",
					AutoAssign: true,
					CIDR:       []*net.IPNet{ipnet("1.2.3.0/30"), ipnet("1000::2/127")},
				},
			},
			pool:    "test2",
			wantErr: true,
		},
		{
			desc: "delete unused pool",
			pools: map[string]*config.Pool{
				"test2": {
					Name:       "test2",
					AutoAssign: true,
					CIDR:       []*net.IPNet{ipnet("1.2.3.0/31"), ipnet("1000::/127")},
				},
			},
			pool: "test2",
		},
		{
			desc: "enable buggy IPs not allowed",
			pools: map[string]*config.Pool{
				"test2": {
					Name:          "test2",
					AutoAssign:    true,
					AvoidBuggyIPs: true,
					CIDR:          []*net.IPNet{ipnet("1.2.3.0/31"), ipnet("1000::/127")},
				},
			},
			pool:    "test2",
			wantErr: true,
		},
	}

	for _, test := range tests {
		err := alloc.SetPools(&config.Pools{ByName: test.pools})
		if test.wantErr {
			if err == nil {
				t.Errorf("%q should have failed to SetPools, but succeeded", test.desc)
			}
		} else if err != nil {
			t.Errorf("%q failed to SetPools: %s", test.desc, err)
		}
		gotPool := alloc.Pool("s1")
		if gotPool != test.pool {
			t.Errorf("%q: s1 is in wrong pool, want %q, got %q", test.desc, test.pool, gotPool)
		}
	}
}

func TestAutoAssign(t *testing.T) {
	alloc := New()
	if err := alloc.SetPools(&config.Pools{ByName: map[string]*config.Pool{
		"test1": {
			Name:       "test1",
			AutoAssign: false,
			CIDR:       []*net.IPNet{ipnet("1.2.3.4/31"), ipnet("1000::4/127")},
		},
		"test2": {
			Name:       "test2",
			AutoAssign: true,
			CIDR:       []*net.IPNet{ipnet("1.2.3.10/31"), ipnet("1000::10/127")},
		},
	}}); err != nil {
		t.Fatalf("SetPools: %s", err)
	}

	validIP4s := map[string]bool{
		"1.2.3.4":  false,
		"1.2.3.5":  false,
		"1.2.3.10": true,
		"1.2.3.11": true,
	}
	validIP6s := map[string]bool{
		"1000::4":  false,
		"1000::5":  false,
		"1000::10": true,
		"1000::11": true,
	}
	validIPDualStacks := map[string]bool{
		"1.2.3.4":  false,
		"1.2.3.5":  false,
		"1.2.3.10": true,
		"1.2.3.11": true,
		"1000::4":  false,
		"1000::5":  false,
		"1000::10": true,
		"1000::11": true,
	}
	tests := []struct {
		svcKey   string
		svc      *v1.Service
		wantErr  bool
		ipFamily ipfamily.Family
		unassign bool
	}{
		{
			svcKey:   "s1",
			svc:      svc,
			ipFamily: ipfamily.IPv4,
		},
		{
			svcKey:   "s2",
			svc:      svc,
			ipFamily: ipfamily.IPv4,
		},
		{
			svcKey:   "s3",
			svc:      svc,
			wantErr:  true,
			ipFamily: ipfamily.IPv4,
		},
		{
			svcKey:   "s4",
			svc:      svc,
			wantErr:  true,
			ipFamily: ipfamily.IPv4,
		},
		{
			svcKey:   "s5",
			svc:      svc,
			wantErr:  true,
			ipFamily: ipfamily.IPv4,
		},

		// Clear old ipv4 addresses
		{
			svcKey:   "s1",
			svc:      svc,
			unassign: true,
			ipFamily: ipfamily.IPv4,
		},
		{
			svcKey:   "s2",
			svc:      svc,
			unassign: true,
			ipFamily: ipfamily.IPv4,
		},
		{
			svcKey:   "s3",
			svc:      svc,
			unassign: true,
			ipFamily: ipfamily.IPv4,
		},
		{
			svcKey:   "s4",
			svc:      svc,
			unassign: true,
			ipFamily: ipfamily.IPv4,
		},
		{
			svcKey:   "s5",
			svc:      svc,
			unassign: true,
			ipFamily: ipfamily.IPv4,
		},
		{
			svcKey:   "s6",
			svc:      svc,
			unassign: true,
			ipFamily: ipfamily.IPv4,
		},

		// IPv6 tests;
		{
			svcKey:   "s1",
			svc:      svc,
			ipFamily: ipfamily.IPv6,
		},
		{
			svcKey:   "s2",
			svc:      svc,
			ipFamily: ipfamily.IPv6,
		},
		{
			svcKey:   "s3",
			svc:      svc,
			ipFamily: ipfamily.IPv6,
			wantErr:  true,
		},
		{
			svcKey:   "s4",
			svc:      svc,
			ipFamily: ipfamily.IPv6,
			wantErr:  true,
		},
		{
			svcKey:   "s5",
			svc:      svc,
			ipFamily: ipfamily.IPv6,
			wantErr:  true,
		},
		// Dual-stack tests;
		{
			svcKey:   "s1",
			svc:      svc,
			ipFamily: ipfamily.DualStack,
		},
		{
			svcKey:   "s2",
			svc:      svc,
			ipFamily: ipfamily.DualStack,
		},
		{
			svcKey:   "s3",
			svc:      svc,
			wantErr:  true,
			ipFamily: ipfamily.DualStack,
		},
		{
			svcKey:   "s4",
			svc:      svc,
			wantErr:  true,
			ipFamily: ipfamily.DualStack,
		},
		{
			svcKey:   "s5",
			svc:      svc,
			wantErr:  true,
			ipFamily: ipfamily.DualStack,
		},
	}

	for i, test := range tests {
		if test.unassign {
			alloc.Unassign(test.svcKey)
			continue
		}
		ips, err := alloc.Allocate(test.svcKey, test.svc, test.ipFamily, nil, "", "")
		if test.wantErr {
			if err == nil {
				t.Errorf("#%d should have caused an error, but did not", i+1)
			}
			continue
		}
		if err != nil {
			t.Errorf("#%d Allocate(%q, \"test\"): %s", i+1, test.svcKey, err)
		}
		validIPs := validIP4s
		if test.ipFamily == ipfamily.IPv6 {
			validIPs = validIP6s
		} else if test.ipFamily == ipfamily.DualStack {
			validIPs = validIPDualStacks
		}
		for _, ip := range ips {
			if !validIPs[ip.String()] {
				t.Errorf("#%d allocated unexpected IP %q", i+1, ip)
			}
		}
	}
}

func TestPoolCount(t *testing.T) {
	tests := []struct {
		desc string
		pool *config.Pool
		want int64
	}{
		{
			desc: "BGP /24",
			pool: &config.Pool{
				CIDR: []*net.IPNet{ipnet("1.2.3.0/24")},
			},
			want: 256,
		},
		{
			desc: "BGP /24 and /25",
			pool: &config.Pool{
				CIDR: []*net.IPNet{ipnet("1.2.3.0/24"), ipnet("2.3.4.128/25")},
			},
			want: 384,
		},
		{
			desc: "BGP /24 and /25, no buggy IPs",
			pool: &config.Pool{
				CIDR:          []*net.IPNet{ipnet("1.2.3.0/24"), ipnet("2.3.4.128/25")},
				AvoidBuggyIPs: true,
			},
			want: 381,
		},
		{
			desc: "BGP a BIG ipv6 range",
			pool: &config.Pool{
				CIDR:          []*net.IPNet{ipnet("1.2.3.0/24"), ipnet("2.3.4.128/25"), ipnet("1000::/64")},
				AvoidBuggyIPs: true,
			},
			want: math.MaxInt64,
		},
	}

	for _, test := range tests {
		got := poolCount(test.pool)
		if test.want != got {
			t.Errorf("%q: wrong pool count, want %d, got %d", test.desc, test.want, got)
		}
	}
}

func TestPoolMetrics(t *testing.T) {
	alloc := New()
	if err := alloc.SetPools(&config.Pools{ByName: map[string]*config.Pool{
		"test": {
			Name:       "test",
			AutoAssign: true,
			CIDR: []*net.IPNet{
				ipnet("1.2.3.4/30"),
				ipnet("1000::4/126"),
			},
		},
	}}); err != nil {
		t.Fatalf("SetPools: %s", err)
	}

	tests := []struct {
		desc       string
		svcKey     string
		svc        *v1.Service
		ips        []string
		ports      []Port
		sharingKey string
		backendKey string
		ipsInUse   float64
	}{
		{
			desc:     "assign s1",
			svcKey:   "s1",
			svc:      svc,
			ips:      []string{"1.2.3.4"},
			ipsInUse: 1,
		},
		{
			desc:     "assign s2",
			svcKey:   "s2",
			svc:      svc,
			ips:      []string{"1.2.3.5"},
			ipsInUse: 2,
		},
		{
			desc:     "unassign s1",
			svcKey:   "s1",
			svc:      svc,
			ipsInUse: 1,
		},
		{
			desc:     "unassign s2",
			svcKey:   "s2",
			svc:      svc,
			ipsInUse: 0,
		},
		{
			desc:       "assign s1 shared",
			svcKey:     "s1",
			svc:        svc,
			ips:        []string{"1.2.3.4"},
			sharingKey: "key",
			ports:      ports("tcp/80"),
			ipsInUse:   1,
		},
		{
			desc:       "assign s2 shared",
			svcKey:     "s2",
			svc:        svc,
			ips:        []string{"1.2.3.4"},
			sharingKey: "key",
			ports:      ports("tcp/443"),
			ipsInUse:   1,
		},
		{
			desc:       "assign s3 shared",
			svcKey:     "s3",
			svc:        svc,
			ips:        []string{"1.2.3.4"},
			sharingKey: "key",
			ports:      ports("tcp/23"),
			ipsInUse:   1,
		},
		{
			desc:     "unassign s1 shared",
			svcKey:   "s1",
			svc:      svc,
			ports:    ports("tcp/80"),
			ipsInUse: 1,
		},
		{
			desc:     "unassign s2 shared",
			svcKey:   "s2",
			svc:      svc,
			ports:    ports("tcp/443"),
			ipsInUse: 1,
		},
		{
			desc:     "unassign s3 shared",
			svcKey:   "s3",
			svc:      svc,
			ports:    ports("tcp/23"),
			ipsInUse: 0,
		},
	}

	// The "test1" pool contains two ranges; 1.2.3.4/30, 1000::4/126
	// All bits can be used for lb-addresses which gives a total capacity of; 4+4=8
	value := ptu.ToFloat64(stats.poolCapacity.WithLabelValues("test"))
	if int(value) != 8 {
		t.Errorf("stats.poolCapacity invalid %f. Expected 8", value)
	}

	for _, test := range tests {
		if len(test.ips) == 0 {
			alloc.Unassign(test.svcKey)
			value := ptu.ToFloat64(stats.poolActive.WithLabelValues("test"))
			if value != test.ipsInUse {
				t.Errorf("%v; in-use %v. Expected %v", test.desc, value, test.ipsInUse)
			}
			continue
		}

		ips := make([]net.IP, 0)
		for _, ip := range test.ips {
			ips = append(ips, net.ParseIP(ip))
		}
		if len(ips) == 0 {
			t.Fatalf("invalid IP %q in test %q", test.ips, test.desc)
		}
		err := alloc.Assign(test.svcKey, test.svc, ips, test.ports, test.sharingKey, test.backendKey)
		if err != nil {
			t.Errorf("%q: Assign(%q, %q): %v", test.desc, test.svcKey, test.ips, err)
		}
		if a := assigned(alloc, test.svcKey); !compareIPs(a, test.ips) {
			t.Errorf("%q: ran Assign(%q, %q), but allocator has recorded allocation of %q", test.desc, test.svcKey, test.ips, a)
		}
		value := ptu.ToFloat64(stats.poolActive.WithLabelValues("test"))
		if value != test.ipsInUse {
			t.Errorf("%v; in-use %v. Expected %v", test.desc, value, test.ipsInUse)
		}
	}
}

// Some helpers.

func assigned(a *Allocator, svc string) []string {
	res := []string{}
	if alloc := a.allocated[svc]; alloc != nil {
		for _, ip := range alloc.ips {
			if ip == nil {
				return nil
			}
			res = append(res, ip.String())
		}
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

func ports(ports ...string) []Port {
	var ret []Port
	for _, s := range ports {
		fs := strings.Split(s, "/")
		p, err := strconv.Atoi(fs[1])
		if err != nil {
			panic("bad port in test")
		}
		ret = append(ret, Port{fs[0], p})
	}
	return ret
}

func compareIPs(ips1, ips2 []string) bool {
	if len(ips1) != len(ips2) {
		return false
	}

	for _, ip1 := range ips1 {
		found := false
		for _, ip2 := range ips2 {
			if net.ParseIP(ip1).Equal(net.ParseIP(ip2)) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
