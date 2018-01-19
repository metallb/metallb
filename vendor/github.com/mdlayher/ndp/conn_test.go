package ndp

import (
	"net"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestConn(t *testing.T) {
	tests := []struct {
		name string
		fn   func(t *testing.T, c1, c2 *Conn, addr net.IP)
	}{
		{
			name: "echo",
			fn:   testConnEcho,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withConns(t, tt.fn)
		})
	}
}

func testConnEcho(t *testing.T, c1, c2 *Conn, addr net.IP) {
	// Echo this message between two connections.
	rs := &RouterSolicitation{}

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()

		// Read and bounce the message back to the second Conn.
		m, _, _, err := c2.ReadFrom()
		if err != nil {
			panicf("failed to read from c2: %v", err)
		}

		if err := c2.WriteTo(m, nil, addr); err != nil {
			panicf("failed to write from c2: %v", err)
		}
	}()

	if err := c1.WriteTo(rs, nil, addr); err != nil {
		t.Fatalf("failed to write from c1: %v", err)
	}

	m, _, _, err := c1.ReadFrom()
	if err != nil {
		t.Fatalf("failed to read from c1: %v", err)
	}

	wg.Wait()

	if diff := cmp.Diff(rs, m); diff != "" {
		t.Fatalf("unexpected message (-want +got):\n%s", diff)
	}
}

// withConns invokes fn once with a UDPv6 connection and again with an ICMPv6
// connection, enabling testing with both privileged and unprivileged sockets.
func withConns(t *testing.T, fn func(t *testing.T, c1, c2 *Conn, addr net.IP)) {
	var name string
	var newConn func(t *testing.T) (*Conn, *Conn, net.IP, func())

	for i := 0; i < 2; i++ {
		switch i {
		case 0:
			name = "UDP"
			newConn = testUDPConn
		case 1:
			name = "ICMP"
			newConn = testICMPConn
		default:
			t.Fatalf("unhandled withConns iteration: %d", i)
		}

		t.Run(name, func(t *testing.T) {
			c1, c2, addr, done := newConn(t)
			defer done()

			fn(t, c1, c2, addr)
		})
	}
}
