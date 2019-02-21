package layer2

import (
	"net"
	"testing"
)

func Test_SetBalancer_AddsToAnnouncedServices(t *testing.T) {
	announce := &Announce{
		ips:      map[string]net.IP{},
		ipRefcnt: map[string]int{},
	}

	services := []struct {
		name string
		ip   net.IP
	}{
		{
			name: "foo",
			ip:   net.IPv4(192, 168, 1, 20),
		},
		{
			name: "bar",
			ip:   net.IPv4(192, 168, 1, 20),
		},
	}

	for _, service := range services {
		announce.SetBalancer(service.name, service.ip)

		if !announce.AnnounceName(service.name) {
			t.Fatalf("service %v is not anounced", service.name)
		}
	}
}
