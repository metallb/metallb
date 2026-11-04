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
		name string
		ips  map[string][]IPAdvertisement
		want map[string]IPAdvertisement
	}{
		{
			name: "allInterfaces wins over interface set",
			ips: map[string][]IPAdvertisement{
				"svc1": {{ip: ipA, allInterfaces: true}},
				"svc2": {{ip: ipA, interfaces: sets.New("eth0"), allInterfaces: false}},
			},
			want: map[string]IPAdvertisement{
				ipA.String(): {ip: ipA, allInterfaces: true},
			},
		},
		{
			name: "allInterfaces wins over interface set (reversed order)",
			ips: map[string][]IPAdvertisement{
				"svc2": {{ip: ipA, interfaces: sets.New("eth0"), allInterfaces: false}},
				"svc1": {{ip: ipA, allInterfaces: true}},
			},
			want: map[string]IPAdvertisement{
				ipA.String(): {ip: ipA, allInterfaces: true},
			},
		},
		{
			name: "interface sets are unioned",
			ips: map[string][]IPAdvertisement{
				"svc1": {{ip: ipA, interfaces: sets.New("eth0"), allInterfaces: false}},
				"svc2": {{ip: ipA, interfaces: sets.New("eth1"), allInterfaces: false}},
			},
			want: map[string]IPAdvertisement{
				ipA.String(): {ip: ipA, interfaces: sets.New("eth0", "eth1"), allInterfaces: false},
			},
		},
		{
			name: "distinct IPs produce distinct entries",
			ips: map[string][]IPAdvertisement{
				"svc1": {{ip: ipA, interfaces: sets.New("eth0"), allInterfaces: false}},
				"svc2": {{ip: ipB, allInterfaces: true}},
			},
			want: map[string]IPAdvertisement{
				ipA.String(): {ip: ipA, interfaces: sets.New("eth0"), allInterfaces: false},
				ipB.String(): {ip: ipB, allInterfaces: true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			announce := &Announce{
				ips: tt.ips,
			}
			merged := announce.mergedAdvertisementsPerIP()
			if len(merged) != len(tt.want) {
				t.Fatalf("got %d merged advs, want %d", len(merged), len(tt.want))
			}
			byIP := map[string]IPAdvertisement{}
			for _, adv := range merged {
				key := adv.ip.String()
				if _, dup := byIP[key]; dup {
					t.Fatalf("duplicate merged advertisement for %s", key)
				}
				byIP[key] = adv
			}
			for ip, want := range tt.want {
				adv, ok := byIP[ip]
				if !ok {
					t.Fatalf("missing merged advertisement for %s", ip)
				}
				if adv.allInterfaces != want.allInterfaces {
					t.Errorf("ip %s: allInterfaces=%v, want %v", ip, adv.allInterfaces, want.allInterfaces)
				}
				if !want.allInterfaces && !adv.interfaces.Equal(want.interfaces) {
					t.Errorf("ip %s: interfaces=%v, want %v", ip, adv.interfaces, want.interfaces)
				}
			}
		})
	}
}

func Test_SendPeriodicGratuitous_IteratesEveryAnnouncedIP(t *testing.T) {
	// Positive control for the empty-case test: with IPs announced,
	// sendPeriodicGratuitous must call Gratuitous() once per unique announced
	// IP on the matching responder, routing by address family (IPv4 -> ARP,
	// IPv6 -> NDP). Fakes record the calls so we assert on the real gratuitous()
	// path instead of only the merge helper.
	arp := newFakeARPResponder("eth0")
	ndp := newFakeNDPResponder("eth0")
	ipV4 := net.IPv4(192, 168, 1, 20)
	ipV6a := net.ParseIP("1000::1")
	ipV6b := net.ParseIP("1000::2")
	announce := &Announce{
		arps: map[int]arpResponder{0: arp},
		ndps: map[int]ndpResponder{0: ndp},
		ips: map[string][]IPAdvertisement{
			// Same IPv6 announced by two services: still one Gratuitous call.
			"foo": {{ip: ipV6a, allInterfaces: true}},
			"bar": {{ip: ipV6a, allInterfaces: true}},
			"baz": {{ip: ipV6b, allInterfaces: true}},
			"qux": {{ip: ipV4, allInterfaces: true}},
		},
		ipRefcnt: map[string]int{
			ipV4.String():  1,
			ipV6a.String(): 2,
			ipV6b.String(): 1,
		},
		spamCh: make(chan IPAdvertisement, 1024),
	}

	announce.sendPeriodicGratuitous()

	gotNDP := ndp.gratuitousCalls()
	wantNDP := map[string]int{ipV6a.String(): 1, ipV6b.String(): 1}
	assertGratuitousCounts(t, "NDP", gotNDP, wantNDP)
	gotARP := arp.gratuitousCalls()
	wantARP := map[string]int{ipV4.String(): 1}
	assertGratuitousCounts(t, "ARP", gotARP, wantARP)
}

// assertGratuitousCounts checks that each expected IP was pinged exactly the
// wanted number of times (and no unexpected IPs were pinged), so we can catch
// both missing announcements and accidental duplicates.
func assertGratuitousCounts(t *testing.T, proto string, got, want map[string]int) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s Gratuitous called for IPs %v, want %v", proto, got, want)
	}
	for ip, wantN := range want {
		gotN, ok := got[ip]
		if !ok {
			t.Fatalf("%s Gratuitous not called for %s (got %v)", proto, ip, got)
		}
		if gotN != wantN {
			t.Fatalf("%s Gratuitous called %d times for %s, want %d", proto, gotN, ip, wantN)
		}
	}
}

// fakeARPResponder is a test double for the arpResponder interface that records the IPs
// passed to Gratuitous.
type fakeARPResponder struct {
	intf       string
	gratuitous []net.IP
}

func newFakeARPResponder(intf string) *fakeARPResponder {
	return &fakeARPResponder{intf: intf}
}

func (f *fakeARPResponder) Interface() string { return f.intf }

func (f *fakeARPResponder) Gratuitous(ip net.IP) error {
	f.gratuitous = append(f.gratuitous, ip)
	return nil
}

func (f *fakeARPResponder) Close() error { return nil }

func (f *fakeARPResponder) gratuitousCalls() map[string]int {
	m := map[string]int{}
	for _, ip := range f.gratuitous {
		m[ip.String()]++
	}
	return m
}

// fakeNDPResponder is a test double for the ndpResponder interface that records the IPs
// passed to Gratuitous.
type fakeNDPResponder struct {
	intf       string
	gratuitous []net.IP
}

func newFakeNDPResponder(intf string) *fakeNDPResponder {
	return &fakeNDPResponder{intf: intf}
}

func (f *fakeNDPResponder) Interface() string { return f.intf }

func (f *fakeNDPResponder) Gratuitous(ip net.IP) error {
	f.gratuitous = append(f.gratuitous, ip)
	return nil
}

func (f *fakeNDPResponder) Watch(net.IP) error   { return nil }
func (f *fakeNDPResponder) Unwatch(net.IP) error { return nil }
func (f *fakeNDPResponder) Close() error         { return nil }

func (f *fakeNDPResponder) gratuitousCalls() map[string]int {
	m := map[string]int{}
	for _, ip := range f.gratuitous {
		m[ip.String()]++
	}
	return m
}
