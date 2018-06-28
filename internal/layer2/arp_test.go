package layer2

import (
	"encoding"
	"fmt"
	"net"
	"testing"

	"github.com/go-kit/kit/log"
	"github.com/google/go-cmp/cmp"
	"github.com/mdlayher/arp"
	"github.com/mdlayher/ethernet"
)

func TestARPResponder(t *testing.T) {
	tests := []struct {
		name           string
		dstMAC         net.HardwareAddr
		arpTgt         net.IP
		arpOp          arp.Operation
		shouldAnnounce announceFunc
		reason         dropReason
	}{
		{
			name:   "ARP reply",
			arpOp:  arp.OperationReply,
			reason: dropReasonARPReply,
		},
		{
			name:   "bad Ethernet destination",
			dstMAC: net.HardwareAddr{6, 5, 4, 3, 2, 1},
			reason: dropReasonEthernetDestination,
		},
		{
			name:   "OK (unicast)",
			reason: dropReasonNone,
		},
		{
			name:   "OK (broadcast)",
			dstMAC: ethernet.Broadcast,
			reason: dropReasonNone,
		},
		{
			name: "shouldAnnounce denies request",
			shouldAnnounce: func(ip net.IP) dropReason {
				if net.IPv4(192, 168, 1, 20).Equal(ip) {
					return dropReasonNone
				}
				return dropReasonError
			},
			reason: dropReasonError,
		},
		{
			name:   "shouldAnnounce allows request",
			arpTgt: net.IPv4(192, 168, 1, 20),
			shouldAnnounce: func(ip net.IP) dropReason {
				if net.IPv4(192, 168, 1, 20).Equal(ip) {
					return dropReasonNone
				}
				return dropReasonError
			},
			reason: dropReasonNone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shouldAnnounce := tt.shouldAnnounce
			if shouldAnnounce == nil {
				shouldAnnounce = func(net.IP) dropReason {
					return dropReasonNone
				}
			}
			a, conn, done := newTestARP(t, shouldAnnounce)
			defer done()

			// Defaults for test params
			if tt.dstMAC == nil {
				tt.dstMAC = a.hardwareAddr
			}
			if tt.arpTgt == nil {
				tt.arpTgt = net.IPv4(192, 168, 1, 10)
			}
			if tt.arpOp == 0 {
				tt.arpOp = arp.OperationRequest
			}

			eth := &ethernet.Frame{
				Destination: tt.dstMAC,
				Source:      net.HardwareAddr{1, 2, 3, 4, 5, 6},
				EtherType:   ethernet.EtherTypeARP,
			}
			pkt, err := arp.NewPacket(tt.arpOp, eth.Source, net.IPv4(192, 168, 1, 1), tt.dstMAC, tt.arpTgt)
			if err != nil {
				t.Fatalf("failed to make ARP packet: %s", err)
			}

			eth.Payload = mustMarshal(pkt)
			b := mustMarshal(eth)

			dropC := make(chan dropReason)
			go func() {
				dropC <- a.processRequest()
			}()

			// Send a packet to receiver goroutine.
			if _, err := conn.Write(b); err != nil {
				t.Fatalf("failed to write: %v", err)
			}

			reason := <-dropC
			if diff := cmp.Diff(tt.reason, reason); diff != "" {
				t.Fatalf("unexpected drop reason (-want +got)\n%s", diff)
			}
		})
	}
}

func mustMarshal(m encoding.BinaryMarshaler) []byte {
	b, err := m.MarshalBinary()
	if err != nil {
		panic(fmt.Sprintf("failed to marshal: %v", err))
	}

	return b
}

func newTestARP(t *testing.T, shouldAnnounce announceFunc) (*arpResponder, *net.UDPConn, func()) {
	pc, err := net.ListenPacket("udp", "localhost:0")
	if err != nil {
		t.Fatalf("failed to listen UDP: %s", err)
	}

	intfs, err := net.Interfaces()
	if err != nil {
		t.Fatalf("failed to get interfaces: %s", err)
	}

	if len(intfs) == 0 {
		t.Fatalf("no network interfaces")
	}

	// Find any interface that has a hardware address. We don't care
	// which one, we just need something to satisfy the various
	// interfaces.
	var a *arpResponder
	for _, intf := range intfs {
		if intf.HardwareAddr == nil {
			continue
		}

		var c *arp.Client
		c, err = arp.New(&intf, pc)
		if err != nil {
			t.Fatalf("failed to create ARP client: %s", err)
		}

		a = &arpResponder{
			logger:       log.NewNopLogger(),
			hardwareAddr: intf.HardwareAddr,
			conn:         c,
			closed:       make(chan struct{}),
			announce:     shouldAnnounce,
		}
	}

	uc, err := net.DialUDP("udp", nil, pc.LocalAddr().(*net.UDPAddr))
	if err != nil {
		t.Fatalf("failed to dial UDP: %s", err)
	}

	return a, uc, func() {
		uc.Close()
		a.Close()
		pc.Close()
	}
}
