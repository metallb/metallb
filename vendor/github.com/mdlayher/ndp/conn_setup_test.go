package ndp

import (
	"fmt"
	"net"
	"os"
	"testing"

	"golang.org/x/net/ipv6"
)

func testICMPConn(t *testing.T) (*Conn, *Conn, net.IP, func()) {
	ifi := testInterface(t)

	// Create two ICMPv6 connections that will communicate with each other.
	c1, addr := icmpConn(t, ifi)
	c2, _ := icmpConn(t, ifi)

	return c1, c2, addr, func() {
		_ = c1.Close()
		_ = c2.Close()
	}
}

func testUDPConn(t *testing.T) (*Conn, *Conn, net.IP, func()) {
	ifi := testInterface(t)

	addrs, err := ifi.Addrs()
	if err != nil {
		t.Fatalf("failed to get addresses: %v", err)
	}

	addr, err := chooseAddr(addrs, ifi.Name, LinkLocal)
	if err != nil {
		// TODO(mdlayher): remove when travis can do IPv6.
		t.Skipf("failed to choose address, skipping test: %v", err)
	}

	// Create two UDPv6 connections and instruct them to communicate
	// with each other for Conn tests.
	c1, p1 := udpConn(t, addr, ifi)
	c2, p2 := udpConn(t, addr, ifi)

	c1.udpTestPort = p2
	c2.udpTestPort = p1

	return c1, c2, addr.IP, func() {
		_ = c1.Close()
		_ = c2.Close()
	}
}

func testInterface(t *testing.T) *net.Interface {
	// TODO(mdlayher): expand this to be more flexible.
	ifi, err := net.InterfaceByName("eth0")
	if err != nil {
		t.Fatalf("failed to get eth0: %v", err)
	}

	return ifi
}

func icmpConn(t *testing.T, ifi *net.Interface) (*Conn, net.IP) {
	// Wire up a standard ICMPv6 NDP connection.
	c, addr, err := Dial(ifi, LinkLocal)
	if err != nil {
		oerr, ok := err.(*net.OpError)
		if !ok && (ok && !os.IsPermission(err)) {
			t.Fatalf("failed to dial NDP: %v", err)
		}

		t.Skipf("permission denied, cannot test ICMPv6 NDP: %v", oerr)
	}
	c.icmpTest = true

	return c, addr
}

func udpConn(t *testing.T, addr *net.IPAddr, ifi *net.Interface) (*Conn, int) {
	laddr := &net.UDPAddr{
		IP:   addr.IP,
		Zone: addr.Zone,
	}

	uc, err := net.ListenUDP("udp6", laddr)
	if err != nil {
		t.Fatalf("failed to listen UDPv6: %v", err)
	}

	pc := ipv6.NewPacketConn(uc)

	c, _, err := newConn(pc, addr, ifi)
	if err != nil {
		t.Fatalf("failed to create NDP conn: %v", err)
	}

	return c, uc.LocalAddr().(*net.UDPAddr).Port
}

func panicf(format string, a ...interface{}) {
	panic(fmt.Sprintf(format, a...))
}
