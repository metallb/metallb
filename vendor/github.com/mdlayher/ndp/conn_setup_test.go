package ndp

import (
	"fmt"
	"net"
	"os"
	"testing"
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

	c1, c2, ip, err := TestConns(ifi)
	if err != nil {
		// TODO(mdlayher): remove when travis can do IPv6.
		t.Skipf("failed to create test connections, skipping test: %v", err)
	}

	return c1, c2, ip, func() {
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

func panicf(format string, a ...interface{}) {
	panic(fmt.Sprintf(format, a...))
}
