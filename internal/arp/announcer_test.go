package arp

import (
	"encoding"
	"fmt"
	"net"
	"sync"
	"testing"

	"go.universe.tf/metallb/internal/iface"

	"github.com/google/go-cmp/cmp"
	"github.com/mdlayher/arp"
	"github.com/mdlayher/ethernet"
)

func TestAnnounceRun(t *testing.T) {
	announceIP := net.IPv4(10, 0, 0, 1).To4()

	tests := []struct {
		name     string
		modify   func(f *ethernet.Frame, p *arp.Packet)
		announce net.IP
		leader   bool
		reason   iface.DropReason
	}{
		{
			name: "ARP reply",
			modify: func(_ *ethernet.Frame, p *arp.Packet) {
				p.Operation = arp.OperationReply
			},
			reason: iface.DropReasonARPReply,
		},
		{
			name: "bad Ethernet destination",
			modify: func(f *ethernet.Frame, _ *arp.Packet) {
				f.Destination = net.HardwareAddr{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}
			},
			reason: iface.DropReasonEthernetDestination,
		},
		{
			name:   "not announcing IP",
			modify: func(_ *ethernet.Frame, _ *arp.Packet) {},
			reason: iface.DropReasonAnnounceIP,
		},
		{
			name: "not leader",
			modify: func(_ *ethernet.Frame, p *arp.Packet) {
				p.TargetIP = announceIP
			},
			announce: announceIP,
			reason:   iface.DropReasonNotLeader,
		},
		{
			name: "OK",
			modify: func(_ *ethernet.Frame, p *arp.Packet) {
				p.TargetIP = announceIP
			},
			announce: announceIP,
			leader:   true,
			reason:   iface.DropReasonNone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, conn, done := newAnnounce(t)
			defer done()

			// Set leader status and announce IP if necessary.
			a.leader = tt.leader

			if tt.announce != nil {
				a.SetBalancer("test", announceIP)
			}

			// Stock data which can be overridden by modify function.
			var (
				srcHW = net.HardwareAddr{0xde, 0xad, 0xbe, 0xef, 0xde, 0xad}
				srcIP = net.IPv4(192, 168, 1, 1).To4()

				dstHW = a.responder.hardwareAddr
				dstIP = net.IPv4(192, 168, 1, 10).To4()
			)

			eth := &ethernet.Frame{
				Destination: a.responder.hardwareAddr,
				Source:      srcHW,
				EtherType:   ethernet.EtherTypeARP,
			}

			pkt, err := arp.NewPacket(arp.OperationRequest, srcHW, srcIP, dstHW, dstIP)
			if err != nil {
				t.Fatalf("failed to make ARP packet: %v", err)
			}

			// Modify the frame and packet before marshaling.
			tt.modify(eth, pkt)

			eth.Payload = mustMarshal(pkt)
			b := mustMarshal(eth)

			// Wait for a single packet to be received.
			var wg sync.WaitGroup
			wg.Add(1)
			defer wg.Wait()

			dropC := make(chan iface.DropReason)
			go func() {
				defer wg.Done()

				dropC <- a.responder.processRequest()
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

func newAnnounce(t *testing.T) (*Announce, *net.UDPConn, func()) {
	pc, err := net.ListenPacket("udp", "localhost:0")
	if err != nil {
		t.Fatalf("failed to listen UDP: %v", err)
	}

	// TODO(mdlayher): don't depend on an interface when we really just care
	// about its addresses anyway.
	ifi, err := net.InterfaceByName("eth0")
	if err != nil {
		t.Skipf("skipping, couldn't get eth0: %v", err)
	}

	c, err := arp.New(ifi, pc)
	if err != nil {
		t.Fatalf("failed to create ARP client: %v", err)
	}

	a := &Announce{
		ips: make(map[string]net.IP),
	}
	a.responder = &arpResponder{
		hardwareAddr: ifi.HardwareAddr,
		conn:         c,
		announce:     a.shouldAnnounce,
	}

	uc, err := net.DialUDP("udp", nil, pc.LocalAddr().(*net.UDPAddr))
	if err != nil {
		t.Fatalf("failed to dial UDP: %v", err)
	}

	return a, uc, func() {
		uc.Close()
		pc.Close()
		a.responder.Close()
	}
}
