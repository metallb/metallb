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

func Test_MergedAdvertisementsPerIP(t *testing.T) {
	ipA := net.IPv4(192, 168, 1, 100)
	ipB := net.IPv4(192, 168, 1, 101)

	tests := []struct {
		name              string
		ips               map[string][]IPAdvertisement
		wantAllInterfaces map[string]bool
		wantInterfaces    map[string]sets.Set[string]
	}{
		{
			name: "allInterfaces wins over interface set",
			ips: map[string][]IPAdvertisement{
				"svc1": {{ip: ipA, allInterfaces: true}},
				"svc2": {{ip: ipA, interfaces: sets.New("eth0"), allInterfaces: false}},
			},
			wantAllInterfaces: map[string]bool{ipA.String(): true},
		},
		{
			name: "interface sets are unioned",
			ips: map[string][]IPAdvertisement{
				"svc1": {{ip: ipA, interfaces: sets.New("eth0"), allInterfaces: false}},
				"svc2": {{ip: ipA, interfaces: sets.New("eth1"), allInterfaces: false}},
			},
			wantAllInterfaces: map[string]bool{ipA.String(): false},
			wantInterfaces:    map[string]sets.Set[string]{ipA.String(): sets.New("eth0", "eth1")},
		},
		{
			name: "distinct IPs produce distinct entries",
			ips: map[string][]IPAdvertisement{
				"svc1": {{ip: ipA, interfaces: sets.New("eth0"), allInterfaces: false}},
				"svc2": {{ip: ipB, allInterfaces: true}},
			},
			wantAllInterfaces: map[string]bool{ipA.String(): false, ipB.String(): true},
			wantInterfaces:    map[string]sets.Set[string]{ipA.String(): sets.New("eth0")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			announce := &Announce{
				ips: tt.ips,
			}
			merged := announce.mergedAdvertisementsPerIP()
			if len(merged) != len(tt.wantAllInterfaces) {
				t.Fatalf("got %d merged advs, want %d", len(merged), len(tt.wantAllInterfaces))
			}
			byIP := map[string]IPAdvertisement{}
			for _, adv := range merged {
				byIP[adv.ip.String()] = adv
			}
			for ip, wantAll := range tt.wantAllInterfaces {
				adv, ok := byIP[ip]
				if !ok {
					t.Fatalf("missing merged advertisement for %s", ip)
				}
				if adv.allInterfaces != wantAll {
					t.Errorf("ip %s: allInterfaces=%v, want %v", ip, adv.allInterfaces, wantAll)
				}
				if !wantAll {
					if !adv.interfaces.Equal(tt.wantInterfaces[ip]) {
						t.Errorf("ip %s: interfaces=%v, want %v", ip, adv.interfaces, tt.wantInterfaces[ip])
					}
				}
			}
		})
	}
}

func Test_MergedAdvertisementsPerIP_DoesNotMutateInputs(t *testing.T) {
	ip := net.IPv4(192, 168, 1, 100)
	original := sets.New("eth0")
	announce := &Announce{
		ips: map[string][]IPAdvertisement{
			"svc1": {{ip: ip, interfaces: original, allInterfaces: false}},
			"svc2": {{ip: ip, interfaces: sets.New("eth1"), allInterfaces: false}},
		},
	}
	_ = announce.mergedAdvertisementsPerIP()
	if !original.Equal(sets.New("eth0")) {
		t.Fatalf("source interface set was mutated: %v", original)
	}
}

func Test_SendPeriodicGratuitous_SkipsWhenNoIPs(t *testing.T) {
	announce := &Announce{
		ips:      map[string][]IPAdvertisement{},
		ipRefcnt: map[string]int{},
		spamCh:   make(chan IPAdvertisement, 1024),
	}

	// No IPs registered — should be a no-op.
	announce.sendPeriodicGratuitous()
}
