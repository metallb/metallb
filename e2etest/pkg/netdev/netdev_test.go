// SPDX-License-Identifier:Apache-2.0

package netdev

import (
	"testing"
)

func TestInterfacesForAddr(t *testing.T) {
	output :=
		`[{"ifindex":1,"ifname":"lo","flags":["LOOPBACK","UP","LOWER_UP"],"mtu":65536,"qdisc":"noqueue","operstate":"UNKNOWN","group":"default","txqlen":1000,"link_type":"loopback","address":"00:00:00:00:00:00","broadcast":"00:00:00:00:00:00","addr_info":[{"family":"inet","local":"127.0.0.1","prefixlen":8,"scope":"host","label":"lo","valid_life_time":4294967295,"preferred_life_time":4294967295},{"family":"inet6","local":"::1","prefixlen":128,"scope":"host","valid_life_time":4294967295,"preferred_life_time":4294967295}]},{"ifindex":2,"link_index":2,"ifname":"veth4b6ac7a6","flags":["BROADCAST","MULTICAST","UP","LOWER_UP"],"mtu":1500,"qdisc":"noqueue","operstate":"UP","group":"default","link_type":"ether","address":"c6:ba:7c:49:bf:05","broadcast":"ff:ff:ff:ff:ff:ff","link_netnsid":1,"addr_info":[{"family":"inet","local":"10.244.1.1","prefixlen":32,"scope":"global","label":"veth4b6ac7a6","valid_life_time":4294967295,"preferred_life_time":4294967295}]},{"ifindex":6,"link_index":7,"ifname":"eth0","flags":["BROADCAST","MULTICAST","UP","LOWER_UP"],"mtu":1500,"qdisc":"noqueue","operstate":"UP","group":"default","link_type":"ether","address":"02:42:ac:12:00:02","broadcast":"ff:ff:ff:ff:ff:ff","link_netnsid":0,"addr_info":[{"family":"inet","local":"172.18.0.2","prefixlen":16,"broadcast":"172.18.255.255","scope":"global","label":"eth0","valid_life_time":4294967295,"preferred_life_time":4294967295},{"family":"inet6","local":"fc00:f853:ccd:e793::2","prefixlen":64,"scope":"global","nodad":true,"valid_life_time":4294967295,"preferred_life_time":4294967295},{"family":"inet6","local":"fe80::42:acff:fe12:2","prefixlen":64,"scope":"link","valid_life_time":4294967295,"preferred_life_time":4294967295}]},{"ifindex":14,"link_index":15,"ifname":"eth1","flags":["BROADCAST","MULTICAST","UP","LOWER_UP"],"mtu":1500,"qdisc":"noqueue","operstate":"UP","group":"default","link_type":"ether","address":"02:42:ac:13:00:03","broadcast":"ff:ff:ff:ff:ff:ff","link_netnsid":0,"addr_info":[{"family":"inet","local":"172.19.0.3","prefixlen":16,"broadcast":"172.19.255.255","scope":"global","label":"eth1","valid_life_time":4294967295,"preferred_life_time":4294967295},{"family":"inet6","local":"fc00:f853:ccd:e791::3","prefixlen":64,"scope":"global","nodad":true,"valid_life_time":4294967295,"preferred_life_time":4294967295},{"family":"inet6","local":"fe80::42:acff:fe13:3","prefixlen":64,"scope":"link","valid_life_time":4294967295,"preferred_life_time":4294967295}]}]`

	cases := []struct {
		description       string
		ipv4Address       string
		ipv6Address       string
		expectedInterface string
		expectsError      bool
	}{
		{
			description:       "eth1, should find",
			ipv4Address:       "172.19.0.3",
			ipv6Address:       "fc00:f853:ccd:e791::3",
			expectedInterface: "eth1",
		},
		{
			description:  "eth1, v4 not matching",
			ipv4Address:  "172.19.0.4",
			ipv6Address:  "fc00:f853:ccd:e791::3",
			expectsError: true,
		},
		{
			description:  "eth1, v6 not matching",
			ipv4Address:  "172.19.0.3",
			ipv6Address:  "fc00:f853:ccd:e791::4",
			expectsError: true,
		},
	}

	for _, tt := range cases {
		t.Run(tt.description, func(t *testing.T) {
			intf, err := findInterfaceWithAddresses(output, tt.ipv4Address, tt.ipv6Address)
			if tt.expectsError && err == nil {
				t.Errorf("expected error, but got nil")
			}
			if !tt.expectsError && err != nil {
				t.Errorf("did not expect error but got %v", err)
			}
			if intf != tt.expectedInterface {
				t.Errorf("interface different from what expected want: %s - expected %s", intf, tt.expectedInterface)
			}
		})
	}
}
