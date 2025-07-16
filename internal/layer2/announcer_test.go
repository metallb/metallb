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
				interfaces:    sets.Set[string]{},
				allInterfaces: true,
			},
		},
		{
			name: "foo",
			adv: IPAdvertisement{
				ip:            net.ParseIP("1000::1"),
				interfaces:    sets.New("eth0"),
				allInterfaces: true,
			},
		},
		{
			name: "bar",
			adv: IPAdvertisement{
				ip:            net.IPv4(192, 168, 1, 20),
				interfaces:    sets.New("eth1"),
				allInterfaces: false,
			},
		},
	}

	for _, service := range services {
		announce.SetBalancer(service.name, service.adv)
		// We need to empty spamCh as spamLoop() is not started.
		<-announce.spamCh

		if !announce.AnnounceName(service.name) {
			t.Fatalf("service %v is not announced", service.name)
		}
	}
	if len(announce.ips["foo"]) != 2 {
		t.Fatalf("service foo has more than 2 ips: %d", len(announce.ips["foo"]))
	}
	if announce.ipRefcnt["192.168.1.20"] != 2 {
		t.Fatalf("ip 192.168.1.20 has not 2 refcnt: %d", announce.ipRefcnt["192.168.1.20"])
	}
}

func TestInterfaceDetection(t *testing.T) {
	tests := []struct {
		name     string
		addrs    []net.Addr
		flags    net.Flags
		expected struct {
			arp bool
			ndp bool
		}
	}{
		{
			name: "IPv4 only interface with broadcast",
			addrs: []net.Addr{
				&net.IPNet{IP: net.ParseIP("192.168.1.10"), Mask: net.CIDRMask(24, 32)},
			},
			flags: net.FlagUp | net.FlagBroadcast,
			expected: struct {
				arp bool
				ndp bool
			}{
				arp: true,
				ndp: false,
			},
		},
		{
			name: "IPv6 only interface",
			addrs: []net.Addr{
				&net.IPNet{IP: net.ParseIP("2001:db8::1"), Mask: net.CIDRMask(64, 128)},
			},
			flags: net.FlagUp | net.FlagBroadcast,
			expected: struct {
				arp bool
				ndp bool
			}{
				arp: false,
				ndp: true,
			},
		},
		{
			name: "Dual-stack interface",
			addrs: []net.Addr{
				&net.IPNet{IP: net.ParseIP("192.168.1.10"), Mask: net.CIDRMask(24, 32)},
				&net.IPNet{IP: net.ParseIP("2001:db8::1"), Mask: net.CIDRMask(64, 128)},
			},
			flags: net.FlagUp | net.FlagBroadcast,
			expected: struct {
				arp bool
				ndp bool
			}{
				arp: true,
				ndp: true,
			},
		},
		{
			name: "Interface with only link-local IPv6",
			addrs: []net.Addr{
				&net.IPNet{IP: net.ParseIP("fe80::1"), Mask: net.CIDRMask(64, 128)},
			},
			flags: net.FlagUp | net.FlagBroadcast,
			expected: struct {
				arp bool
				ndp bool
			}{
				arp: false,
				ndp: false,
			},
		},
		{
			name: "Interface with only private IPv4",
			addrs: []net.Addr{
				&net.IPNet{IP: net.ParseIP("10.0.0.1"), Mask: net.CIDRMask(8, 32)},
			},
			flags: net.FlagUp | net.FlagBroadcast,
			expected: struct {
				arp bool
				ndp bool
			}{
				arp: true,
				ndp: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock interface
			ifi := &net.Interface{
				Index:        1,
				MTU:          1500,
				Name:         "test0",
				HardwareAddr: net.HardwareAddr{0x00, 0x11, 0x22, 0x33, 0x44, 0x55},
				Flags:        tt.flags,
			}

			// Test the interface detection logic
			hasIPv4 := false
			hasIPv6 := false

			for _, addr := range tt.addrs {
				ipaddr, ok := addr.(*net.IPNet)
				if !ok {
					continue
				}

				if ipaddr.IP.To4() != nil && ipaddr.IP.IsGlobalUnicast() {
					hasIPv4 = true
				}

				if ipaddr.IP.To4() == nil && ipaddr.IP.To16() != nil && ipaddr.IP.IsGlobalUnicast() {
					hasIPv6 = true
				}
			}

			keepARP := hasIPv4 && ifi.Flags&net.FlagBroadcast != 0
			keepNDP := hasIPv6

			if keepARP != tt.expected.arp {
				t.Errorf("ARP detection mismatch: got %v, want %v", keepARP, tt.expected.arp)
			}
			if keepNDP != tt.expected.ndp {
				t.Errorf("NDP detection mismatch: got %v, want %v", keepNDP, tt.expected.ndp)
			}
		})
	}
}
