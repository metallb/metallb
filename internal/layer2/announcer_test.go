// SPDX-License-Identifier:Apache-2.0

package layer2

import (
	"errors"
	"net/netip"
	"testing"

	"github.com/go-kit/log"
	"github.com/mdlayher/ndp"
	"k8s.io/apimachinery/pkg/util/sets"
)

func Test_SetBalancer_AddsToAnnouncedServices(t *testing.T) {
	announce := &Announce{
		ips:      map[string][]IPAdvertisement{},
		ipRefcnt: map[netip.Addr]int{},
		spamCh:   make(chan IPAdvertisement, 1),
	}

	services := []struct {
		name string
		adv  IPAdvertisement
	}{
		{
			name: "foo",
			adv: IPAdvertisement{
				ip:            netip.MustParseAddr("192.168.1.20"),
				interfaces:    sets.Set[string]{},
				allInterfaces: true,
			},
		},
		{
			name: "foo",
			adv: IPAdvertisement{
				ip:            netip.MustParseAddr("1000::1"),
				interfaces:    sets.New("eth0"),
				allInterfaces: true,
			},
		},
		{
			name: "bar",
			adv: IPAdvertisement{
				ip:            netip.MustParseAddr("192.168.1.20"),
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
	if announce.ipRefcnt[netip.MustParseAddr("192.168.1.20")] != 2 {
		t.Fatalf("ip 192.168.1.20 has not 2 refcnt: %d", announce.ipRefcnt[netip.MustParseAddr("192.168.1.20")])
	}
}

func Test_DeleteBalancer_RemovesUnusedIPRefcounts(t *testing.T) {
	for _, address := range []string{"192.0.2.1", "2001:db8::1"} {
		t.Run(address, func(t *testing.T) {
			announce := newTestAnnounce()
			ip := netip.MustParseAddr(address)
			setBalancer := func(name string) {
				announce.SetBalancer(name, IPAdvertisement{ip: ip, allInterfaces: true})
				<-announce.spamCh
			}
			setBalancer("first")
			setBalancer("second")

			announce.DeleteBalancer("first")
			if count := announce.ipRefcnt[ip]; count != 1 {
				t.Fatalf("shared address reference count = %d, want 1", count)
			}
			announce.DeleteBalancer("second")
			if _, exists := announce.ipRefcnt[ip]; exists {
				t.Fatal("address reference count remains after the final service is deleted")
			}
			announce.DeleteBalancer("second")
			if _, exists := announce.ipRefcnt[ip]; exists {
				t.Fatal("repeated deletion recreated the address reference count")
			}

			setBalancer("first")
			if count := announce.ipRefcnt[ip]; count != 1 {
				t.Fatalf("re-added address reference count = %d, want 1", count)
			}
			announce.DeleteBalancer("first")
			if len(announce.ipRefcnt) != 0 {
				t.Fatalf("reference counts remain after deleting all services: %v", announce.ipRefcnt)
			}
		})
	}
}

func Test_AddNDPResponder_WatchesActiveAddresses(t *testing.T) {
	announce := &Announce{
		logger:   log.NewNopLogger(),
		ndps:     map[int]ndpResponder{},
		ips:      map[string][]IPAdvertisement{},
		ipRefcnt: map[netip.Addr]int{},
		spamCh:   make(chan IPAdvertisement, 1),
	}
	sharedIP := netip.MustParseAddr("2001:db8::1")
	otherIP := netip.MustParseAddr("2001:db8:1::1") // Shares a solicited-node group with sharedIP.
	deletedIP := netip.MustParseAddr("2001:db8::2")
	ipv4 := netip.MustParseAddr("192.0.2.1")
	setBalancer := func(name string, ip netip.Addr) {
		announce.SetBalancer(name, IPAdvertisement{ip: ip, allInterfaces: true})
		<-announce.spamCh
	}
	setBalancer("shared1", sharedIP)
	setBalancer("shared2", sharedIP)
	setBalancer("other", otherIP)
	setBalancer("deleted", deletedIP)
	announce.DeleteBalancer("deleted")
	setBalancer("ipv4", ipv4)

	resp := newFakeNDPResponder("eth0")
	announce.Lock()
	announce.addNDPResponder(1, resp)
	announce.Unlock()
	if announce.ndps[1] != resp {
		t.Fatal("new NDP responder was not registered")
	}
	if len(resp.watches) != 2 || resp.watches[sharedIP] != 1 || resp.watches[otherIP] != 1 {
		t.Fatalf("got watches %v, want one per active IPv6 address", resp.watches)
	}

	announce.DeleteBalancer("shared1")
	if resp.watches[sharedIP] != 1 {
		t.Fatalf("shared IP watch count = %d, want 1 while another service uses it", resp.watches[sharedIP])
	}
	announce.DeleteBalancer("shared2")
	if resp.watches[sharedIP] != 0 {
		t.Fatalf("shared IP watch count = %d, want 0 after the last service is deleted", resp.watches[sharedIP])
	}
	setBalancer("shared1", sharedIP)
	if resp.watches[sharedIP] != 1 {
		t.Fatalf("shared IP watch count = %d, want 1 after re-adding it", resp.watches[sharedIP])
	}
	if resp.watches[otherIP] != 1 {
		t.Fatalf("other IP watch count = %d, want 1", resp.watches[otherIP])
	}
}

func Test_NDPFailedWatchDoesNotReleaseSharedGroup(t *testing.T) {
	for _, replay := range []bool{true, false} {
		name := "SetBalancer"
		if replay {
			name = "replay"
		}
		t.Run(name, func(t *testing.T) {
			announce := newTestAnnounce()
			failedIP := netip.MustParseAddr("2001:db8::1")
			otherIP := netip.MustParseAddr("2001:db8:1::1")
			group, err := ndp.SolicitedNodeMulticast(otherIP)
			if err != nil {
				t.Fatal(err)
			}
			resp := newFakeNDPResponder("eth0")
			resp.watchErrors[failedIP] = errors.New("injected join failure")
			setBalancer := func(name string, ip netip.Addr) {
				announce.SetBalancer(name, IPAdvertisement{ip: ip, allInterfaces: true})
				<-announce.spamCh
			}
			if replay {
				setBalancer("failed", failedIP)
			}
			announce.addNDPResponder(1, resp)
			if !replay {
				setBalancer("failed", failedIP)
			}
			if resp.watchCalls[failedIP] != 1 || announce.ndpWatches[1][failedIP] {
				t.Fatal("failed watch should be attempted but not recorded as successful")
			}

			// The next address joins the group whose first watch failed.
			setBalancer("other", otherIP)
			announce.DeleteBalancer("failed")
			announce.watchNDPResponder(1)
			if resp.groups[group] != 1 || resp.leaves[group] != 0 {
				t.Fatalf("deleting failed watch released a shared group: counts=%v, leaves=%v", resp.groups, resp.leaves)
			}
			if resp.watchCalls[failedIP] != 1 {
				t.Fatal("deleted address's failed watch was retried")
			}
			announce.DeleteBalancer("other")
			if resp.groups[group] != 0 || resp.leaves[group] != 1 {
				t.Fatalf("last successful watch did not leave the group once: counts=%v, leaves=%v", resp.groups, resp.leaves)
			}
		})
	}
}

func Test_NDPWatchRetriesPreserveSuccessfulWatches(t *testing.T) {
	announce := newTestAnnounce()
	failedIP := netip.MustParseAddr("2001:db8::1")
	healthyIP := netip.MustParseAddr("2001:db8::2")
	for name, ip := range map[string]netip.Addr{"failed": failedIP, "healthy1": healthyIP, "healthy2": healthyIP} {
		announce.SetBalancer(name, IPAdvertisement{ip: ip, allInterfaces: true})
		<-announce.spamCh
	}
	resp := newFakeNDPResponder("eth0")
	resp.watchErrors[failedIP] = errors.New("injected persistent join failure")
	announce.addNDPResponder(1, resp)
	other := newFakeNDPResponder("eth1")
	announce.addNDPResponder(2, other)

	// Each scan retries only the missing watch, without closing either
	// responder or changing the successful watches on either interface.
	for range 3 {
		announce.watchNDPResponder(1)
		announce.watchNDPResponder(2)
	}
	if resp.watchCalls[failedIP] != 4 || resp.watchCalls[healthyIP] != 1 {
		t.Fatalf("unexpected retry attempts: %v", resp.watchCalls)
	}
	if resp.watches[healthyIP] != 1 || resp.closed || len(resp.leaves) != 0 {
		t.Fatal("persistent failure disturbed the healthy address")
	}
	if other.watchCalls[failedIP] != 1 || other.watchCalls[healthyIP] != 1 || other.closed || len(other.leaves) != 0 {
		t.Fatal("retry changed successful watches on another interface")
	}

	delete(resp.watchErrors, failedIP)
	announce.watchNDPResponder(1)
	announce.watchNDPResponder(1)
	if resp.watchCalls[failedIP] != 5 || resp.watches[failedIP] != 1 || !announce.ndpWatches[1][failedIP] {
		t.Fatalf("recovered watch was not acquired exactly once: attempts=%v, watches=%v", resp.watchCalls, resp.watches)
	}
	announce.DeleteBalancer("failed")
	if resp.watches[failedIP] != 0 || other.watches[failedIP] != 0 {
		t.Fatal("recovered address's watches were not released on both interfaces")
	}
	if resp.watches[healthyIP] != 1 || other.watches[healthyIP] != 1 {
		t.Fatal("deleting recovered address disturbed the healthy address")
	}
}

func newTestAnnounce() *Announce {
	return &Announce{
		logger:   log.NewNopLogger(),
		ndps:     map[int]ndpResponder{},
		ips:      map[string][]IPAdvertisement{},
		ipRefcnt: map[netip.Addr]int{},
		spamCh:   make(chan IPAdvertisement, 1),
	}
}

func Test_MergedAdvertisementsPerIP(t *testing.T) {
	ipA := netip.MustParseAddr("192.168.1.100")
	ipB := netip.MustParseAddr("192.168.1.101")

	tests := []struct {
		name string
		ips  map[string][]IPAdvertisement
		want map[netip.Addr]IPAdvertisement
	}{
		{
			name: "allInterfaces wins over interface set",
			ips: map[string][]IPAdvertisement{
				"svc1": {{ip: ipA, allInterfaces: true}},
				"svc2": {{ip: ipA, interfaces: sets.New("eth0"), allInterfaces: false}},
			},
			want: map[netip.Addr]IPAdvertisement{
				ipA: {ip: ipA, allInterfaces: true},
			},
		},
		{
			name: "allInterfaces wins over interface set (reversed order)",
			ips: map[string][]IPAdvertisement{
				"svc2": {{ip: ipA, interfaces: sets.New("eth0"), allInterfaces: false}},
				"svc1": {{ip: ipA, allInterfaces: true}},
			},
			want: map[netip.Addr]IPAdvertisement{
				ipA: {ip: ipA, allInterfaces: true},
			},
		},
		{
			name: "interface sets are unioned",
			ips: map[string][]IPAdvertisement{
				"svc1": {{ip: ipA, interfaces: sets.New("eth0"), allInterfaces: false}},
				"svc2": {{ip: ipA, interfaces: sets.New("eth1"), allInterfaces: false}},
			},
			want: map[netip.Addr]IPAdvertisement{
				ipA: {ip: ipA, interfaces: sets.New("eth0", "eth1"), allInterfaces: false},
			},
		},
		{
			name: "distinct IPs produce distinct entries",
			ips: map[string][]IPAdvertisement{
				"svc1": {{ip: ipA, interfaces: sets.New("eth0"), allInterfaces: false}},
				"svc2": {{ip: ipB, allInterfaces: true}},
			},
			want: map[netip.Addr]IPAdvertisement{
				ipA: {ip: ipA, interfaces: sets.New("eth0"), allInterfaces: false},
				ipB: {ip: ipB, allInterfaces: true},
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
			byIP := map[netip.Addr]IPAdvertisement{}
			for _, adv := range merged {
				key := adv.ip
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
	ipV4 := netip.MustParseAddr("192.168.1.20")
	ipV6a := netip.MustParseAddr("1000::1")
	ipV6b := netip.MustParseAddr("1000::2")
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
		ipRefcnt: map[netip.Addr]int{
			ipV4:  1,
			ipV6a: 2,
			ipV6b: 1,
		},
		spamCh: make(chan IPAdvertisement, 1024),
	}

	announce.sendPeriodicGratuitous()

	gotNDP := ndp.gratuitousCalls()
	wantNDP := map[netip.Addr]int{ipV6a: 1, ipV6b: 1}
	assertGratuitousCounts(t, "NDP", gotNDP, wantNDP)
	gotARP := arp.gratuitousCalls()
	wantARP := map[netip.Addr]int{ipV4: 1}
	assertGratuitousCounts(t, "ARP", gotARP, wantARP)
}

// assertGratuitousCounts checks that each expected IP was pinged exactly the
// wanted number of times (and no unexpected IPs were pinged), so we can catch
// both missing announcements and accidental duplicates.
func assertGratuitousCounts(t *testing.T, proto string, got, want map[netip.Addr]int) {
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
	gratuitous []netip.Addr
}

func newFakeARPResponder(intf string) *fakeARPResponder {
	return &fakeARPResponder{intf: intf}
}

func (f *fakeARPResponder) Interface() string { return f.intf }

func (f *fakeARPResponder) Gratuitous(ip netip.Addr) error {
	f.gratuitous = append(f.gratuitous, ip)
	return nil
}

func (f *fakeARPResponder) Close() error { return nil }

func (f *fakeARPResponder) gratuitousCalls() map[netip.Addr]int {
	m := map[netip.Addr]int{}
	for _, ip := range f.gratuitous {
		m[ip]++
	}
	return m
}

// fakeNDPResponder is a test double for the ndpResponder interface that records the IPs
// passed to Gratuitous and tracks active watches.
type fakeNDPResponder struct {
	intf        string
	gratuitous  []netip.Addr
	watches     map[netip.Addr]int
	watchCalls  map[netip.Addr]int
	watchErrors map[netip.Addr]error
	groups      map[netip.Addr]int
	leaves      map[netip.Addr]int
	closed      bool
}

func newFakeNDPResponder(intf string) *fakeNDPResponder {
	return &fakeNDPResponder{
		intf:        intf,
		watches:     map[netip.Addr]int{},
		watchCalls:  map[netip.Addr]int{},
		watchErrors: map[netip.Addr]error{},
		groups:      map[netip.Addr]int{},
		leaves:      map[netip.Addr]int{},
	}
}

func (f *fakeNDPResponder) Interface() string { return f.intf }

func (f *fakeNDPResponder) Gratuitous(ip netip.Addr) error {
	f.gratuitous = append(f.gratuitous, ip)
	return nil
}

func (f *fakeNDPResponder) Watch(ip netip.Addr) error {
	if ip.Is4() {
		return nil
	}
	f.watchCalls[ip]++
	if err := f.watchErrors[ip]; err != nil {
		return err
	}
	group, err := ndp.SolicitedNodeMulticast(ip)
	if err != nil {
		return err
	}
	f.watches[ip]++
	f.groups[group]++
	return nil
}

func (f *fakeNDPResponder) Unwatch(ip netip.Addr) error {
	if ip.Is4() {
		return nil
	}
	group, err := ndp.SolicitedNodeMulticast(ip)
	if err != nil {
		return err
	}
	// Match ndpClient: an unmatched Unwatch would release another address's
	// group reference, or make the group count negative.
	f.watches[ip]--
	f.groups[group]--
	if f.groups[group] == 0 {
		f.leaves[group]++
	}
	return nil
}

func (f *fakeNDPResponder) Close() error {
	f.closed = true
	return nil
}

func (f *fakeNDPResponder) gratuitousCalls() map[netip.Addr]int {
	m := map[netip.Addr]int{}
	for _, ip := range f.gratuitous {
		m[ip]++
	}
	return m
}
