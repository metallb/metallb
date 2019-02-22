//  Copyright (c) 2018 Cisco and/or its affiliates.
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at:
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package vpp_punt

/*func TestPuntToHostKey(t *testing.T) {
	tests := []struct {
		name        string
		l3Protocol  L3Protocol
		l4Protocol  L4Protocol
		port        uint32
		expectedKey string
	}{
		{
			name:        "valid Punt case (IPv4/UDP)",
			l3Protocol:  L3Protocol_IPv4,
			l4Protocol:  L4Protocol_UDP,
			port:        9000,
			expectedKey: "vpp/config/v2/punt/tohost/l3/IPv4/l4/UDP/port/9000",
		},
		{
			name:        "valid Punt case (IPv4/TCP)",
			l3Protocol:  L3Protocol_IPv4,
			l4Protocol:  L4Protocol_TCP,
			port:        9000,
			expectedKey: "vpp/config/v2/punt/tohost/l3/IPv4/l4/TCP/port/9000",
		},
		{
			name:        "valid Punt case (IPv6/UDP)",
			l3Protocol:  L3Protocol_IPv6,
			l4Protocol:  L4Protocol_UDP,
			port:        9000,
			expectedKey: "vpp/config/v2/punt/tohost/l3/IPv6/l4/UDP/port/9000",
		},
		{
			name:        "valid Punt case (IPv6/TCP)",
			l3Protocol:  L3Protocol_IPv6,
			l4Protocol:  L4Protocol_TCP,
			port:        0,
			expectedKey: "vpp/config/v2/punt/tohost/l3/IPv6/l4/TCP/port/<invalid>",
		},
		{
			name:        "invalid Punt case (zero port)",
			l3Protocol:  L3Protocol_IPv4,
			l4Protocol:  L4Protocol_UDP,
			port:        0,
			expectedKey: "vpp/config/v2/punt/tohost/l3/IPv4/l4/UDP/port/<invalid>",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			key := ToHostKey(test.l3Protocol, test.l4Protocol, test.port)
			if key != test.expectedKey {
				t.Errorf("failed for: puntName=%s\n"+
					"expected key:\n\t%q\ngot key:\n\t%q",
					test.name, test.expectedKey, key)
			}
		})
	}
}*/

/*func TestParsePuntToHostKey(t *testing.T) {
	tests := []struct {
		name            string
		key             string
		expectedL3      L3Protocol
		expectedL4      L4Protocol
		expectedPort    uint32
		isPuntToHostKey bool
	}{
		{
			name:            "valid Punt key",
			key:             "vpp/config/v2/punt/tohost/l3/IPv4/l4/TCP/port/9000",
			expectedL3:      L3Protocol(4),
			expectedL4:      L4Protocol(6),
			expectedPort:    9000,
			isPuntToHostKey: true,
		},
		{
			name:            "invalid Punt L3",
			key:             "vpp/config/v2/punt/tohost/l3/4/l4/TCP/port/9000",
			expectedL3:      L3Protocol(0),
			expectedL4:      L4Protocol(6),
			expectedPort:    9000,
			isPuntToHostKey: true,
		},
		{
			name:            "invalid Punt L3 and L4",
			key:             "vpp/config/v2/punt/tohost/l3/4/l4/6/port/9000",
			expectedL3:      L3Protocol(0),
			expectedL4:      L4Protocol(0),
			expectedPort:    9000,
			isPuntToHostKey: true,
		},
		{
			name:            "invalid Punt L4 and port",
			key:             "vpp/config/v2/punt/tohost/l3/IPv6/l4/17/port/port1",
			expectedL3:      L3Protocol(6),
			expectedL4:      L4Protocol(0),
			expectedPort:    0,
			isPuntToHostKey: true,
		},
		{
			name:            "invalid all",
			key:             "vpp/config/v2/punt/tohost/l3/4/l4/17/port/port1",
			expectedL3:      L3Protocol(0),
			expectedL4:      L4Protocol(0),
			expectedPort:    0,
			isPuntToHostKey: true,
		},
		{
			name:            "not a Punt to host key",
			key:             "vpp/config/v2/punt/ipredirect/l3/IPv6/tx/if1",
			expectedL3:      L3Protocol(0),
			expectedL4:      L4Protocol(0),
			expectedPort:    0,
			isPuntToHostKey: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			l3Proto, l4Proto, port, isPuntToHostKey := ParsePuntToHostKey(test.key)
			if l3Proto != test.expectedL3 {
				t.Errorf("expected l3PuntKey: %v\tgot: %v", test.expectedL3, l3Proto)
			}
			if l4Proto != test.expectedL4 {
				t.Errorf("expected l4PuntKey: %v\tgot: %v", test.expectedL4, l4Proto)
			}
			if port != test.expectedPort {
				t.Errorf("expected portPuntKey: %v\tgot: %v", test.expectedPort, port)
			}
			if isPuntToHostKey != test.isPuntToHostKey {
				t.Errorf("expected isPuntKey: %v\tgot: %v", test.isPuntToHostKey, isPuntToHostKey)
			}
		})
	}
}*/

/*func TestIPredirectKey(t *testing.T) {
	tests := []struct {
		name        string
		l3Protocol  L3Protocol
		txInterface string
		expectedKey string
	}{
		{
			name:        "valid IP redirect case (IPv4)",
			l3Protocol:  L3Protocol_IPv4,
			txInterface: "if1",
			expectedKey: "vpp/config/v2/punt/ipredirect/l3/IPv4/tx/if1",
		},
		{
			name:        "valid IP redirect case (IPv6)",
			l3Protocol:  L3Protocol_IPv6,
			txInterface: "if1",
			expectedKey: "vpp/config/v2/punt/ipredirect/l3/IPv6/tx/if1",
		},
		{
			name:        "invalid IP redirect case (undefined interface)",
			l3Protocol:  L3Protocol_IPv4,
			txInterface: "",
			expectedKey: "vpp/config/v2/punt/ipredirect/l3/IPv4/tx/<invalid>",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			key := IPRedirectKey(test.l3Protocol, test.txInterface)
			if key != test.expectedKey {
				t.Errorf("failed for: puntName=%s\n"+
					"expected key:\n\t%q\ngot key:\n\t%q",
					test.name, test.expectedKey, key)
			}
		})
	}
}*/

/*func TestParseIPRedirectKey(t *testing.T) {
	tests := []struct {
		name            string
		key             string
		expectedL3      L3Protocol
		expectedIf      string
		isIPRedirectKey bool
	}{
		{
			name:            "valid IP redirect key (IPv4)",
			key:             "vpp/config/v2/punt/ipredirect/l3/IPv4/tx/if1",
			expectedL3:      L3Protocol(4),
			expectedIf:      "if1",
			isIPRedirectKey: true,
		},
		{
			name:            "valid IP redirect key (IPv6)",
			key:             "vpp/config/v2/punt/ipredirect/l3/IPv6/tx/if1",
			expectedL3:      L3Protocol(6),
			expectedIf:      "if1",
			isIPRedirectKey: true,
		},
		{
			name:            "invalid IP redirect key (invalid interface)",
			key:             "vpp/config/v2/punt/ipredirect/l3/IPv4/tx/<invalid>",
			expectedL3:      L3Protocol(4),
			expectedIf:      "<invalid>",
			isIPRedirectKey: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			l3Proto, ifName, isIPRedirectKey := ParseIPRedirectKey(test.key)
			if l3Proto != test.expectedL3 {
				t.Errorf("expected l3IPRedirectKey L3: %v\tgot: %v", test.expectedL3, l3Proto)
			}
			if ifName != test.expectedIf {
				t.Errorf("expected l3IPRedirectKey ifName: %v\tgot: %v", test.expectedIf, ifName)
			}
			if isIPRedirectKey != test.isIPRedirectKey {
				t.Errorf("expected isIPRedirectKey: %v\tgot: %v", test.isIPRedirectKey, isIPRedirectKey)
			}
		})
	}
}*/
