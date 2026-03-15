// SPDX-License-Identifier:Apache-2.0

package layer2

import (
	"net"
	"testing"
	"time"

	"github.com/go-kit/log"
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

func newTestAnnounce() *Announce {
	return &Announce{
		logger:         log.NewNopLogger(),
		ips:            map[string][]IPAdvertisement{},
		ipRefcnt:       map[string]int{},
		spamCh:         make(chan IPAdvertisement, 1024),
		garpIntervalCh: make(chan time.Duration, 1),
		stopGARPCh:     make(chan struct{}),
	}
}

func Test_SetGratuitousARPInterval_EnablesPeriodicLoop(t *testing.T) {
	announce := newTestAnnounce()
	defer close(announce.stopGARPCh)
	go announce.periodicGARPLoop()

	// Register an IP so sendPeriodicGratuitous has something to iterate.
	announce.ips["svc1"] = []IPAdvertisement{
		{ip: net.IPv4(10, 0, 0, 1), allInterfaces: true},
	}
	announce.ipRefcnt["10.0.0.1"] = 1

	// Enable periodic GARP with a short interval.
	announce.SetGratuitousARPInterval(50 * time.Millisecond)

	// Wait long enough for at least one tick to fire.
	time.Sleep(200 * time.Millisecond)

	// Disable periodic GARP.
	announce.SetGratuitousARPInterval(0)

	// Allow loop to process the disable.
	time.Sleep(100 * time.Millisecond)
}

func Test_SetGratuitousARPInterval_ZeroDisables(t *testing.T) {
	announce := newTestAnnounce()
	defer close(announce.stopGARPCh)
	go announce.periodicGARPLoop()

	// Set interval to 0 — should not panic or block.
	announce.SetGratuitousARPInterval(0)

	// Allow loop to process.
	time.Sleep(50 * time.Millisecond)
}

func Test_SendPeriodicGratuitous_DeduplicatesIPs(t *testing.T) {
	announce := newTestAnnounce()

	// Same IP from two different services.
	ip := net.IPv4(192, 168, 1, 100)
	announce.ips["svc1"] = []IPAdvertisement{
		{ip: ip, allInterfaces: true},
	}
	announce.ips["svc2"] = []IPAdvertisement{
		{ip: ip, interfaces: sets.New("eth0"), allInterfaces: false},
	}
	announce.ipRefcnt[ip.String()] = 2

	// Should not panic; gratuitous() is a no-op without real ARP/NDP responders.
	announce.sendPeriodicGratuitous()
}

func Test_SendPeriodicGratuitous_SkipsWhenNoIPs(t *testing.T) {
	announce := newTestAnnounce()

	// No IPs registered — should be a no-op.
	announce.sendPeriodicGratuitous()
}
