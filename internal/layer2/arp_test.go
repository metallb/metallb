// SPDX-License-Identifier:Apache-2.0

package layer2

import (
	"encoding"
	"fmt"
	"net"
	"net/netip"
	"testing"

	"github.com/go-kit/log"
	"github.com/google/go-cmp/cmp"
	"github.com/mdlayher/arp"
	"github.com/mdlayher/ethernet"
	"github.com/mdlayher/packet"
)

func TestARPResponder(t *testing.T) {
	tests := []struct {
		name           string
		dstMAC         net.HardwareAddr
		arpTgt         netip.Addr
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
			shouldAnnounce: func(ip netip.Addr, intf string) dropReason {
				if netip.MustParseAddr("192.168.1.20") == ip {
					return dropReasonNone
				}
				return dropReasonError
			},
			reason: dropReasonError,
		},
		{
			name:   "shouldAnnounce allows request",
			arpTgt: netip.MustParseAddr("192.168.1.20"),
			shouldAnnounce: func(ip netip.Addr, intf string) dropReason {
				if netip.MustParseAddr("192.168.1.20") == ip {
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
				shouldAnnounce = func(netip.Addr, string) dropReason {
					return dropReasonNone
				}
			}
			a, conn, done := newTestARP(t, shouldAnnounce)
			defer done()

			// Defaults for test params
			if tt.dstMAC == nil {
				tt.dstMAC = a.hardwareAddr
			}
			if !tt.arpTgt.IsValid() {
				tt.arpTgt = netip.MustParseAddr("192.168.1.10")
			}
			if tt.arpOp == 0 {
				tt.arpOp = arp.OperationRequest
			}

			eth := &ethernet.Frame{
				Destination: tt.dstMAC,
				Source:      net.HardwareAddr{1, 2, 3, 4, 5, 6},
				EtherType:   ethernet.EtherTypeARP,
			}
			pkt, err := arp.NewPacket(tt.arpOp, eth.Source, netip.MustParseAddr("192.168.1.1"), tt.dstMAC, tt.arpTgt)
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

func newTestARP(t *testing.T, shouldAnnounce announceFunc) (*arpClient, *net.UDPConn, func()) {
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
	var a *arpClient
	for _, intf := range intfs {
		if intf.HardwareAddr == nil {
			continue
		}

		a = &arpClient{
			logger:       log.NewNopLogger(),
			hardwareAddr: intf.HardwareAddr,
			conn:         pc,
			closed:       make(chan struct{}),
			announce:     shouldAnnounce,
		}
	}

	if a == nil {
		pc.Close()
		t.Skip("requires a network interface with a hardware address")
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

// The interface's own IP is not needed: gratuitous packets use the service VIP.
func TestARPGratuitousWithoutLocalIPv4(t *testing.T) {
	conn := &testARPWriter{}
	a := &arpClient{
		hardwareAddr: net.HardwareAddr{2, 3, 4, 5, 6, 7},
		conn:         conn,
	}
	vip := netip.MustParseAddr("192.0.2.1")
	if err := a.Gratuitous(vip); err != nil {
		t.Fatal(err)
	}
	if len(conn.frames) != 2 {
		t.Fatalf("sent %d frames, want 2", len(conn.frames))
	}
	for i, operation := range []arp.Operation{arp.OperationRequest, arp.OperationReply} {
		var frame ethernet.Frame
		if err := frame.UnmarshalBinary(conn.frames[i]); err != nil {
			t.Fatal(err)
		}
		var pkt arp.Packet
		if err := pkt.UnmarshalBinary(frame.Payload); err != nil {
			t.Fatal(err)
		}
		if pkt.Operation != operation || pkt.SenderIP != vip || pkt.TargetIP != vip {
			t.Fatalf("unexpected gratuitous packet: %+v", pkt)
		}
		if diff := cmp.Diff(a.hardwareAddr, frame.Source); diff != "" {
			t.Fatalf("incorrect Ethernet source (-want +got): %s", diff)
		}
		if diff := cmp.Diff(ethernet.Broadcast, frame.Destination); diff != "" {
			t.Fatalf("incorrect Ethernet destination (-want +got): %s", diff)
		}
		addr, ok := conn.destinations[i].(*packet.Addr)
		if !ok {
			t.Fatalf("socket destination type = %T, want *packet.Addr", conn.destinations[i])
		}
		if diff := cmp.Diff(ethernet.Broadcast, addr.HardwareAddr); diff != "" {
			t.Fatalf("incorrect socket destination (-want +got): %s", diff)
		}
	}
}

type testARPWriter struct {
	net.PacketConn
	frames       [][]byte
	destinations []net.Addr
}

func (c *testARPWriter) WriteTo(b []byte, addr net.Addr) (int, error) {
	c.frames = append(c.frames, append([]byte(nil), b...))
	c.destinations = append(c.destinations, addr)
	return len(b), nil
}
