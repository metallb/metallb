// SPDX-License-Identifier:Apache-2.0

package frr

import (
	"bytes"
	"fmt"
	"net"
	"sort"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

var expectedStats = MessageStats{
	OpensSent:          1,
	OpensReceived:      2,
	NotificationsSent:  3,
	UpdatesSent:        4,
	UpdatesReceived:    5,
	KeepalivesSent:     6,
	KeepalivesReceived: 7,
	RouteRefreshSent:   8,
	TotalSent:          9,
	TotalReceived:      10,
}

func TestNeighbour(t *testing.T) {
	sample := `{
    "%s":{
      "remoteAs":%s,
      "localAs":%s,
      "nbrInternalLink":true,
      "bgpVersion":4,
      "remoteRouterId":"0.0.0.0",
      "localRouterId":"172.18.0.5",
      "bgpState":"%s",
      "bgpTimerLastRead":253000,
      "bgpTimerLastWrite":3405000,
      "bgpTimerConfiguredHoldTimeMsecs":90000,
      "bgpTimerConfiguredKeepAliveIntervalMsecs":30000,  
      "bgpInUpdateElapsedTimeMsecs":3405000,
      "bgpTimerHoldTimeMsecs":180000,
      "bgpTimerKeepAliveIntervalMsecs":60000,
      "gracefulRestartInfo":{
        "endOfRibSend":{
        },
        "endOfRibRecv":{
        },
        "localGrMode":"Helper*",
        "remoteGrMode":"NotApplicable",
        "rBit":false,
        "timers":{
          "configuredRestartTimer":120,
          "receivedRestartTimer":0
        }
      },
      "messageStats":{
        "depthInq":0,
        "depthOutq":0,
        "opensSent":1,
        "opensRecv":2,
        "notificationsSent":3,
        "notificationsRecv":0,
        "updatesSent":4,
        "updatesRecv":5,
        "keepalivesSent":6,
        "keepalivesRecv":7,
        "routeRefreshSent":8,
        "routeRefreshRecv":0,
        "capabilitySent":0,
        "capabilityRecv":0,
        "totalSent":9,
        "totalRecv":10
      },
      "minBtwnAdvertisementRunsTimerMsecs":0,
      "addressFamilyInfo":{
        "ipv4Unicast":{
          "routerAlwaysNextHop":true,
          "commAttriSentToNbr":"extendedAndStandard",
          "acceptedPrefixCounter":0,
          "sentPrefixCounter":%d
        },
        "ipv6Unicast":{
          "routerAlwaysNextHop":true,
          "commAttriSentToNbr":"extendedAndStandard",
          "acceptedPrefixCounter":0,
          "sentPrefixCounter":%d
        }
      },
      "connectionsDropped":2,
      "lastResetTimerMsecs":253000,
      "lastResetDueTo":"Waiting for peer OPEN",
      "lastResetCode":32,
      "portForeign":%d,
      "connectRetryTimer":120,
      "nextConnectTimerDueInMsecs":107000,
      "readThread":"off",
      "writeThread":"off"
    }
  }`

	tests := []struct {
		name           string
		neighborIP     string
		remoteAS       string
		localAS        string
		status         string
		ipv4PrefixSent int
		ipv6PrefixSent int
		port           int
		expectedError  string
	}{
		{
			"ipv4, connected",
			"172.18.0.5",
			"64512",
			"64512",
			"Established",
			1,
			0,
			179,
			"",
		},
		{
			"ipv4, connected",
			"172.18.0.5",
			"64512",
			"64512",
			"Active",
			0,
			0,
			180,
			"",
		},
		{
			"ipv6, connected",
			"2620:52:0:1302::8af5",
			"64512",
			"64512",
			"Established",
			2,
			1,
			181,
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, err := ParseNeighbour(fmt.Sprintf(sample, tt.neighborIP, tt.remoteAS, tt.localAS, tt.status, tt.ipv4PrefixSent, tt.ipv6PrefixSent, tt.port))
			if err != nil {
				t.Fatal("Failed to parse ", err)
			}
			if n.ID != tt.neighborIP {
				t.Fatal("Expected neighbour ip", tt.neighborIP, "got", n.ID)
			}
			if n.RemoteAS != tt.remoteAS {
				t.Fatal("Expected remote as", tt.remoteAS, "got", n.RemoteAS)
			}
			if n.LocalAS != tt.localAS {
				t.Fatal("Expected local as", tt.localAS, "got", n.LocalAS)
			}
			if tt.status == "Established" && n.Connected != true {
				t.Fatal("Expected connected", true, "got", n.Connected)
			}
			if tt.status != "Established" && n.Connected == true {
				t.Fatal("Expected connected", false, "got", n.Connected)
			}
			if tt.ipv4PrefixSent+tt.ipv6PrefixSent != n.PrefixSent {
				t.Fatal("Expected prefix sent", tt.ipv4PrefixSent+tt.ipv6PrefixSent, "got", n.PrefixSent)
			}
			if tt.port != n.Port {
				t.Fatal("Expected port", tt.port, "got", n.Port)
			}
			if n.RemoteRouterID != "0.0.0.0" {
				t.Fatal("Expected remote routerid 0.0.0.0")
			}
			if !cmp.Equal(expectedStats, n.MsgStats) {
				t.Fatal("unexpected BGP messages stats (-want +got)\n", cmp.Diff(expectedStats, n.MsgStats))
			}
			if n.ConfiguredHoldTime != 90000 {
				t.Fatal("unexpected configured hold time", n.ConfiguredHoldTime)
			}
			if n.ConfiguredKeepAliveTime != 30000 {
				t.Fatal("unexpected configured keepalivetime time", n.ConfiguredKeepAliveTime)
			}
			if n.ConnectionsDropped != 2 {
				t.Fatal("unexpected connections dropped", n.ConnectionsDropped)
			}
		})
	}
}

const fourNeighbours = `
{
  "172.18.0.2":{
    "remoteAs":64512,
    "localAs":64512,
    "nbrInternalLink":true,
    "bgpVersion":4,
    "remoteRouterId":"0.0.0.0",
    "localRouterId":"172.18.0.5",
    "bgpState":"Active",
    "bgpTimerLastRead":14000,
    "bgpTimerLastWrite":3166000,
    "bgpInUpdateElapsedTimeMsecs":3166000,
    "bgpTimerHoldTimeMsecs":180000,
    "bgpTimerKeepAliveIntervalMsecs":60000,
    "gracefulRestartInfo":{
      "endOfRibSend":{
      },
      "endOfRibRecv":{
      },
      "localGrMode":"Helper*",
      "remoteGrMode":"NotApplicable",
      "rBit":false,
      "timers":{
        "configuredRestartTimer":120,
        "receivedRestartTimer":0
      }
    },
    "messageStats":{
      "depthInq":0,
      "depthOutq":0,
      "opensSent":1,
      "opensRecv":2,
      "notificationsSent":3,
      "notificationsRecv":0,
      "updatesSent":4,
      "updatesRecv":5,
      "keepalivesSent":6,
      "keepalivesRecv":7,
      "routeRefreshSent":8,
      "routeRefreshRecv":0,
      "capabilitySent":0,
      "capabilityRecv":0,
      "totalSent":9,
      "totalRecv":10
    },
    "minBtwnAdvertisementRunsTimerMsecs":0,
    "addressFamilyInfo":{
      "ipv4Unicast":{
        "routerAlwaysNextHop":true,
        "commAttriSentToNbr":"extendedAndStandard",
        "acceptedPrefixCounter":0
      }
    },
    "connectionsEstablished":0,
    "connectionsDropped":0,
    "lastResetTimerMsecs":14000,
    "lastResetDueTo":"Waiting for peer OPEN",
    "lastResetCode":32,
    "connectRetryTimer":120,
    "nextConnectTimerDueInMsecs":107000,
    "readThread":"off",
    "writeThread":"off"
  },
  "172.18.0.3":{
    "remoteAs":64512,
    "localAs":64512,
    "nbrInternalLink":true,
    "bgpVersion":4,
    "remoteRouterId":"0.0.0.0",
    "localRouterId":"172.18.0.5",
    "bgpState":"Active",
    "bgpTimerLastRead":14000,
    "bgpTimerLastWrite":3166000,
    "bgpInUpdateElapsedTimeMsecs":3166000,
    "bgpTimerHoldTimeMsecs":180000,
    "bgpTimerKeepAliveIntervalMsecs":60000,
    "gracefulRestartInfo":{
      "endOfRibSend":{
      },
      "endOfRibRecv":{
      },
      "localGrMode":"Helper*",
      "remoteGrMode":"NotApplicable",
      "rBit":false,
      "timers":{
        "configuredRestartTimer":120,
        "receivedRestartTimer":0
      }
    },
    "messageStats":{
      "depthInq":0,
      "depthOutq":0,
      "opensSent":1,
      "opensRecv":2,
      "notificationsSent":3,
      "notificationsRecv":0,
      "updatesSent":4,
      "updatesRecv":5,
      "keepalivesSent":6,
      "keepalivesRecv":7,
      "routeRefreshSent":8,
      "routeRefreshRecv":0,
      "capabilitySent":0,
      "capabilityRecv":0,
      "totalSent":9,
      "totalRecv":10
    },
    "minBtwnAdvertisementRunsTimerMsecs":0,
    "addressFamilyInfo":{
      "ipv4Unicast":{
        "routerAlwaysNextHop":true,
        "commAttriSentToNbr":"extendedAndStandard",
        "acceptedPrefixCounter":0
      }
    },
    "connectionsEstablished":0,
    "connectionsDropped":0,
    "lastResetTimerMsecs":14000,
    "lastResetDueTo":"Waiting for peer OPEN",
    "lastResetCode":32,
    "connectRetryTimer":120,
    "nextConnectTimerDueInMsecs":107000,
    "readThread":"off",
    "writeThread":"off"
  },
  "172.18.0.4":{
    "remoteAs":64512,
    "localAs":64512,
    "nbrInternalLink":true,
    "bgpVersion":4,
    "remoteRouterId":"0.0.0.0",
    "localRouterId":"172.18.0.5",
    "bgpState":"Active",
    "bgpTimerLastRead":14000,
    "bgpTimerLastWrite":3166000,
    "bgpInUpdateElapsedTimeMsecs":3166000,
    "bgpTimerHoldTimeMsecs":180000,
    "bgpTimerKeepAliveIntervalMsecs":60000,
    "gracefulRestartInfo":{
      "endOfRibSend":{
      },
      "endOfRibRecv":{
      },
      "localGrMode":"Helper*",
      "remoteGrMode":"NotApplicable",
      "rBit":false,
      "timers":{
        "configuredRestartTimer":120,
        "receivedRestartTimer":0
      }
    },
    "messageStats":{
      "depthInq":0,
      "depthOutq":0,
      "opensSent":1,
      "opensRecv":2,
      "notificationsSent":3,
      "notificationsRecv":0,
      "updatesSent":4,
      "updatesRecv":5,
      "keepalivesSent":6,
      "keepalivesRecv":7,
      "routeRefreshSent":8,
      "routeRefreshRecv":0,
      "capabilitySent":0,
      "capabilityRecv":0,
      "totalSent":9,
      "totalRecv":10
    },
    "minBtwnAdvertisementRunsTimerMsecs":0,
    "addressFamilyInfo":{
      "ipv4Unicast":{
        "routerAlwaysNextHop":true,
        "commAttriSentToNbr":"extendedAndStandard",
        "acceptedPrefixCounter":0
      }
    },
    "connectionsEstablished":0,
    "connectionsDropped":0,
    "lastResetTimerMsecs":14000,
    "lastResetDueTo":"Waiting for peer OPEN",
    "lastResetCode":32,
    "connectRetryTimer":120,
    "nextConnectTimerDueInMsecs":107000,
    "readThread":"off",
    "writeThread":"off"
  },
  "fc00:f853:ccd:e793::4":{
    "remoteAs":64512,
    "localAs":64513,
    "nbrExternalLink":true,
    "hostname":"kind-control-plane",
    "bgpVersion":4,
    "remoteRouterId":"11.11.11.11",
    "localRouterId":"172.18.0.5",
    "bgpState":"Established",
    "bgpTimerUpMsec":0,
    "bgpTimerUpString":"00:00:00",
    "bgpTimerUpEstablishedEpoch":1636386709,
    "bgpTimerLastRead":4000,
    "bgpTimerLastWrite":0,
    "bgpInUpdateElapsedTimeMsecs":78272000,
    "bgpTimerHoldTimeMsecs":90000,
    "bgpTimerKeepAliveIntervalMsecs":30000,
    "neighborCapabilities":{
      "4byteAs":"advertisedAndReceived",
      "extendedMessage":"advertisedAndReceived",
      "addPath":{
        "ipv6Unicast":{
          "rxAdvertisedAndReceived":true
        }
      },
      "routeRefresh":"advertisedAndReceivedOldNew",
      "enhancedRouteRefresh":"advertisedAndReceived",
      "multiprotocolExtensions":{
        "ipv4Unicast":{
          "received":true
        },
        "ipv6Unicast":{
          "advertisedAndReceived":true
        }
      },
      "hostName":{
        "advHostName":"85e811e29230",
        "advDomainName":"n\/a",
        "rcvHostName":"kind-control-plane",
        "rcvDomainName":"n\/a"
      },
      "gracefulRestart":"advertisedAndReceived",
      "gracefulRestartRemoteTimerMsecs":120000,
      "addressFamiliesByPeer":"none"
    },
    "gracefulRestartInfo":{
      "endOfRibSend":{
        "ipv6Unicast":true
      },
      "endOfRibRecv":{
      },
      "localGrMode":"Helper*",
      "remoteGrMode":"Helper",
      "rBit":true,
      "timers":{
        "configuredRestartTimer":120,
        "receivedRestartTimer":120
      },
      "ipv6Unicast":{
        "fBit":false,
        "endOfRibStatus":{
          "endOfRibSend":true,
          "endOfRibSentAfterUpdate":true,
          "endOfRibRecv":false
        },
        "timers":{
          "stalePathTimer":360
        }
      }
    },
    "messageStats":{
      "depthInq":0,
      "depthOutq":0,
      "opensSent":1,
      "opensRecv":2,
      "notificationsSent":3,
      "notificationsRecv":0,
      "updatesSent":4,
      "updatesRecv":5,
      "keepalivesSent":6,
      "keepalivesRecv":7,
      "routeRefreshSent":8,
      "routeRefreshRecv":0,
      "capabilitySent":0,
      "capabilityRecv":0,
      "totalSent":9,
      "totalRecv":10
    },
    "minBtwnAdvertisementRunsTimerMsecs":0,
    "addressFamilyInfo":{
      "ipv6Unicast":{
        "updateGroupId":1,
        "subGroupId":1,
        "packetQueueLength":0,
        "routerAlwaysNextHop":true,
        "commAttriSentToNbr":"extendedAndStandard",
        "acceptedPrefixCounter":0,
        "sentPrefixCounter":0
      }
    },
    "connectionsEstablished":1,
    "connectionsDropped":0,
    "lastResetTimerMsecs":4000,
    "lastResetDueTo":"No AFI\/SAFI activated for peer",
    "lastResetCode":30,
    "hostLocal":"fc00:f853:ccd:e793::5",
    "portLocal":180,
    "hostForeign":"fc00:f853:ccd:e793::4",
    "portForeign":53568,
    "nexthop":"172.18.0.5",
    "nexthopGlobal":"fc00:f853:ccd:e793::5",
    "nexthopLocal":"fe80::42:acff:fe12:5",
    "bgpConnection":"sharedNetwork",
    "connectRetryTimer":120,
    "authenticationEnabled":1,
    "readThread":"on",
    "writeThread":"on"
  }
}`

func TestNeighbours(t *testing.T) {
	nn, err := ParseNeighbours(fourNeighbours)
	if err != nil {
		t.Fatalf("Failed to parse %s", err)
	}
	if len(nn) != 4 {
		t.Fatalf("Expected 4 neighbours, got %d", len(nn))
	}
	sort.Slice(nn, func(i, j int) bool {
		return (strings.Compare(nn[i].ID, nn[j].ID) < 0)
	})

	if nn[0].ID != "172.18.0.2" {
		t.Fatal("neighbour ip not matching")
	}
	if nn[1].ID != "172.18.0.3" {
		t.Fatal("neighbour ip not matching")
	}
	if nn[2].ID != "172.18.0.4" {
		t.Fatal("neighbour ip not matching")
	}

	for i, n := range nn {
		if !cmp.Equal(expectedStats, n.MsgStats) {
			t.Fatal("unexpected BGP messages stats for neightbor", i, "(-want +got)\n", cmp.Diff(expectedStats, n.MsgStats))
		}
	}
}

const routes = `{
  "vrfId": 0,
  "vrfName": "default",
  "tableVersion": 7,
  "routerId": "172.18.0.5",
  "defaultLocPrf": 100,
  "localAS": 64512,
  "routes": { "192.168.10.0/32": [
   {
     "valid":true,
     "multipath":true,
     "pathFrom":"internal",
     "prefix":"192.168.10.0",
     "prefixLen":32,
     "network":"192.168.10.0\/32",
     "locPrf":0,
     "weight":0,
     "peerId":"172.18.0.4",
     "path":"",
     "origin":"incomplete",
     "nexthops":[
       {
         "ip":"172.18.0.4",
         "afi":"ipv4",
         "used":true
       }
     ]
   },
   {
     "valid":true,
     "bestpath":true,
     "pathFrom":"internal",
     "prefix":"192.168.10.0",
     "prefixLen":32,
     "network":"192.168.10.0\/32",
     "locPrf":0,
     "weight":0,
     "peerId":"172.18.0.2",
     "path":"",
     "origin":"incomplete",
     "nexthops":[
       {
         "ip":"172.18.0.2",
         "afi":"ipv4",
         "used":true
       }
     ]
   },
   {
     "valid":true,
     "multipath":true,
     "pathFrom":"internal",
     "prefix":"192.168.10.0",
     "prefixLen":32,
     "network":"192.168.10.0\/32",
     "locPrf":0,
     "weight":0,
     "peerId":"172.18.0.3",
     "path":"",
     "origin":"incomplete",
     "nexthops":[
       {
         "ip":"172.18.0.3",
         "afi":"ipv4",
         "used":true
       }
     ]
   }
 ] }  }`

func TestRoutes(t *testing.T) {
	rr, err := ParseRoutes(routes)
	if err != nil {
		t.Fatalf("Failed to parse %s", err)
	}

	ipRoutes, ok := rr["192.168.10.0"]
	if !ok {
		t.Fatalf("Routes for 192.168.10.0/32 not found")
	}

	ips := make([]net.IP, 0)
	ips = append(ips, ipRoutes.NextHops...)

	sort.Slice(ips, func(i, j int) bool {
		return (bytes.Compare(ips[i], ips[j]) < 0)
	})
	if !ips[0].Equal(net.ParseIP("172.18.0.2")) {
		t.Fatal("neighbour ip not matching")
	}
	if !ips[1].Equal(net.ParseIP("172.18.0.3")) {
		t.Fatal("neighbour ip not matching")
	}
	if !ips[2].Equal(net.ParseIP("172.18.0.4")) {
		t.Fatal("neighbour ip not matching")
	}
}

const bfdPeers = `[
   {
      "multihop":false,
      "peer":"172.18.0.4",
      "local":"172.18.0.5",
      "vrf":"default",
      "interface":"eth0",
      "id":632314921,
      "remote-id":2999817552,
      "passive-mode":false,
      "status":"up",
      "uptime":52,
      "diagnostic":"ok",
      "remote-diagnostic":"ok",
      "receive-interval":300,
      "transmit-interval":300,
      "echo-receive-interval":50,
      "echo-transmit-interval":0,
      "detect-multiplier":3,
      "remote-receive-interval":300,
      "remote-transmit-interval":300,
      "remote-echo-receive-interval":50,
      "remote-detect-multiplier":3
   },
   {
      "multihop":false,
      "peer":"172.18.0.2",
      "local":"172.18.0.5",
      "vrf":"default",
      "interface":"eth0",
      "id":3048501273,
      "remote-id":2977557242,
      "passive-mode":false,
      "status":"up",
      "uptime":52,
      "diagnostic":"ok",
      "remote-diagnostic":"ok",
      "receive-interval":300,
      "transmit-interval":300,
      "echo-receive-interval":50,
      "echo-transmit-interval":0,
      "detect-multiplier":3,
      "remote-receive-interval":300,
      "remote-transmit-interval":300,
      "remote-echo-receive-interval":50,
      "remote-detect-multiplier":3
   },
   {
      "multihop":false,
      "peer":"172.18.0.3",
      "local":"172.18.0.5",
      "vrf":"default",
      "interface":"eth0",
      "id":2114932580,
      "remote-id":493597049,
      "passive-mode":false,
      "status":"up",
      "uptime":52,
      "diagnostic":"ok",
      "remote-diagnostic":"ok",
      "receive-interval":300,
      "transmit-interval":300,
      "echo-receive-interval":50,
      "echo-transmit-interval":0,
      "detect-multiplier":3,
      "remote-receive-interval":300,
      "remote-transmit-interval":300,
      "remote-echo-interval":50,
      "remote-detect-multiplier":3
   }
]`

func TestBFDPeers(t *testing.T) {
	peers, err := ParseBFDPeers(bfdPeers)
	if err != nil {
		t.Fatalf("Failed to parse %s", err)
	}
	if len(peers) != 3 {
		t.Fatal("Unexpected peer number", len(peers))
	}
	if peers[2].Peer != "172.18.0.3" {
		t.Fatal("Peer not found")
	}
	if peers[2].Status != "up" {
		t.Fatal("wrong status")
	}
	if peers[2].RemoteEchoInterval != 50 {
		t.Fatal("wrong echo interval")
	}
}

const vrfs = `{
"default":{
 "vrfId": 0,
 "vrfName": "default",
 "tableVersion": 1,
 "routerId": "172.18.0.3",
 "defaultLocPrf": 100,
 "localAS": 64512,
 "routes": { "192.168.10.0/32": [
  {
    "valid":true,
    "bestpath":true,
    "pathFrom":"external",
    "prefix":"192.168.10.0",
    "prefixLen":32,
    "network":"192.168.10.0\/32",
    "metric":0,
    "weight":32768,
    "peerId":"(unspec)",
    "path":"",
    "origin":"IGP",
    "nexthops":[
      {
        "ip":"0.0.0.0",
        "hostname":"kind-control-plane",
        "afi":"ipv4",
        "used":true
      }
    ]
  }
] }  }
,
"red":{
 "vrfId": 5,
 "vrfName": "red",
 "tableVersion": 1,
 "routerId": "172.31.0.4",
 "defaultLocPrf": 100,
 "localAS": 64512,
 "routes": { "192.168.10.0/32": [
  {
    "valid":true,
    "bestpath":true,
    "pathFrom":"external",
    "prefix":"192.168.10.0",
    "prefixLen":32,
    "network":"192.168.10.0\/32",
    "metric":0,
    "weight":32768,
    "peerId":"(unspec)",
    "path":"",
    "origin":"IGP",
    "nexthops":[
      {
        "ip":"0.0.0.0",
        "hostname":"kind-control-plane",
        "afi":"ipv4",
        "used":true
      }
    ]
  }
] }  }
}`

func TestVRFs(t *testing.T) {
	parsed, err := ParseVRFs(vrfs)
	if err != nil {
		t.Fatalf("Failed to parse %s", err)
	}
	expected := []string{"default", "red"}
	if !cmp.Equal(parsed, expected) {
		t.Fatalf("unexpected vrf list: %s", cmp.Diff(parsed, expected))
	}
}
