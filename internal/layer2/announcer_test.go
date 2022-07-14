// SPDX-License-Identifier:Apache-2.0

package layer2

import (
	"net"
	"testing"

	"k8s.io/apimachinery/pkg/util/sets"
)

func Test_SetBalancer_AddsToAnnouncedServices(t *testing.T) {
	announce := &Announce{
		ips:      map[string][]IPAdvertisement{},
		ipRefcnt: map[string]int{},
		spamCh:   make(chan IPAdvertisement, 1),
	}

	services := []struct {
		name string
		adv  IPAdvertisement
	}{
		{
			name: "foo",
			adv: IPAdvertisement{
				ip:            net.IPv4(192, 168, 1, 20),
				interfaces:    sets.NewString(),
				allInterfaces: true,
			},
		},
		{
			name: "foo",
			adv: IPAdvertisement{
				ip:            net.ParseIP("1000::1"),
				interfaces:    sets.NewString("eth0"),
				allInterfaces: true,
			},
		},
		{
			name: "bar",
			adv: IPAdvertisement{
				ip:            net.IPv4(192, 168, 1, 20),
				interfaces:    sets.NewString("eth1"),
				allInterfaces: false,
			},
		},
	}

	for _, service := range services {
		announce.SetBalancer(service.name, service.adv)
		// We need to empty spamCh as spamLoop() is not started.
		<-announce.spamCh

		if !announce.AnnounceName(service.name) {
			t.Fatalf("service %v is not anounced", service.name)
		}
	}
}

func TestUpdateBalancerIPs(t *testing.T) {
	tests := []struct {
		desc          string
		a             *Announce
		name          string
		adv           IPAdvertisement
		expectUpdated bool
		expectNewIPs  map[string][]IPAdvertisement
	}{
		{
			desc: "Add a new IP of an exist service",
			a: &Announce{
				ips: map[string][]IPAdvertisement{
					"svc1": {
						{
							ip:         net.IPv4(192, 168, 1, 10),
							interfaces: sets.NewString("eth0"),
						},
					},
				},
			},
			name: "svc1",
			adv: IPAdvertisement{
				ip:         net.IPv4(192, 168, 1, 12),
				interfaces: sets.NewString("eth1"),
			},
			expectUpdated: true,
			expectNewIPs: map[string][]IPAdvertisement{
				"svc1": {
					{
						ip:         net.IPv4(192, 168, 1, 10),
						interfaces: sets.NewString("eth0"),
					}, {
						ip:         net.IPv4(192, 168, 1, 12),
						interfaces: sets.NewString("eth1"),
					},
				},
			},
		}, {
			desc: "Add a new IP of an un-exist service",
			a: &Announce{
				ips: map[string][]IPAdvertisement{
					"svc0": {
						{
							ip:         net.IPv4(192, 168, 1, 10),
							interfaces: sets.NewString("eth0"),
						},
					},
				},
			},
			name: "svc1",
			adv: IPAdvertisement{
				ip:         net.IPv4(192, 168, 1, 12),
				interfaces: sets.NewString("eth1"),
			},
			expectUpdated: true,
			expectNewIPs: map[string][]IPAdvertisement{
				"svc0": {
					{
						ip:         net.IPv4(192, 168, 1, 10),
						interfaces: sets.NewString("eth0"),
					},
				},
				"svc1": {
					{
						ip:         net.IPv4(192, 168, 1, 12),
						interfaces: sets.NewString("eth1"),
					},
				},
			},
		}, {
			desc: "Update interfaces",
			a: &Announce{
				ips: map[string][]IPAdvertisement{
					"svc1": {
						{
							ip:         net.IPv4(192, 168, 1, 10),
							interfaces: sets.NewString("eth0"),
						}, {
							ip:         net.IPv4(192, 168, 1, 12),
							interfaces: sets.NewString("eth1"),
						},
					},
				},
			},
			name: "svc1",
			adv: IPAdvertisement{
				ip:         net.IPv4(192, 168, 1, 12),
				interfaces: sets.NewString("eth1", "eth3"),
			},

			expectUpdated: true,
			expectNewIPs: map[string][]IPAdvertisement{
				"svc1": {
					{
						ip:         net.IPv4(192, 168, 1, 10),
						interfaces: sets.NewString("eth0"),
					}, {
						ip:         net.IPv4(192, 168, 1, 12),
						interfaces: sets.NewString("eth1", "eth3"),
					},
				},
			},
		}, {
			desc: "Update allInterface",
			a: &Announce{
				ips: map[string][]IPAdvertisement{
					"svc1": {
						{
							ip:         net.IPv4(192, 168, 1, 10),
							interfaces: sets.NewString("eth0"),
						}, {
							ip:         net.IPv4(192, 168, 1, 12),
							interfaces: sets.NewString("eth1"),
						},
					},
				},
			},
			name: "svc1",
			adv: IPAdvertisement{
				ip:            net.IPv4(192, 168, 1, 12),
				allInterfaces: true,
			},
			expectUpdated: true,
			expectNewIPs: map[string][]IPAdvertisement{
				"svc1": {
					{
						ip:         net.IPv4(192, 168, 1, 10),
						interfaces: sets.NewString("eth0"),
					}, {
						ip:            net.IPv4(192, 168, 1, 12),
						allInterfaces: true,
					},
				},
			},
		}, {
			desc: "Doesn't update ips",
			a: &Announce{
				ips: map[string][]IPAdvertisement{
					"svc1": {
						{
							ip:         net.IPv4(192, 168, 1, 10),
							interfaces: sets.NewString("eth0"),
						}, {
							ip:         net.IPv4(192, 168, 1, 12),
							interfaces: sets.NewString("eth1"),
						},
					},
				},
			},
			name: "svc1",
			adv: IPAdvertisement{
				ip:         net.IPv4(192, 168, 1, 12),
				interfaces: sets.NewString("eth1"),
			},

			expectUpdated: false,
			expectNewIPs: map[string][]IPAdvertisement{
				"svc1": {
					{
						ip:         net.IPv4(192, 168, 1, 10),
						interfaces: sets.NewString("eth0"),
					}, {
						ip:         net.IPv4(192, 168, 1, 12),
						interfaces: sets.NewString("eth1"),
					},
				},
			},
		},
	}
	for _, test := range tests {
		updated := test.a.updateBalancerIPs(test.name, test.adv)
		if updated != test.expectUpdated {
			t.Errorf("%s: expectUpdated is %v, but result is %v", test.desc, test.expectUpdated, updated)
		}
		for svc, expectIPs := range test.expectNewIPs {
			resultIPs := test.a.ips[svc]
			for _, expectIPAdv := range expectIPs {
				ipAdvEqual := false
				for _, ipAdv := range resultIPs {
					if expectIPAdv.ip.Equal(ipAdv.ip) {
						if expectIPAdv.Equal(ipAdv) {
							ipAdvEqual = true
						}
						break
					}
				}
				if !ipAdvEqual {
					t.Errorf("%s: expectIPadvertisement of service %s is %v, but result is %v", test.desc, svc, expectIPs, resultIPs)
				}
			}
		}
	}
}
