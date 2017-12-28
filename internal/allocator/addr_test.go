package allocator // import "go.universe.tf/metallb/internal/allocator"

import (
	"net"
	"testing"
)

func TestAddrString(t *testing.T) {
	addr := &Addr{&net.TCPAddr{IP: net.ParseIP("127.0.0.1")}}

	if addr.String() != "tcp://127.0.0.1:0" {
		t.Fatal("Failed to get correct string, got %s, want %s", addr.String(), "tcp://127.0.0.1:0")
	}
}
