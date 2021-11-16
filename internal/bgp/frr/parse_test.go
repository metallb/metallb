// SPDX-License-Identifier:Apache-2.0

package frr

import (
	"bytes"
	"fmt"
	"net"
	"sort"
	"testing"
)

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
        "opensSent":0,
        "opensRecv":0,
        "notificationsSent":0,
        "notificationsRecv":0,
        "updatesSent":%d,
        "updatesRecv":0,
        "keepalivesSent":0,
        "keepalivesRecv":0,
        "routeRefreshSent":0,
        "routeRefreshRecv":0,
        "capabilitySent":0,
        "capabilityRecv":0,
        "totalSent":0,
        "totalRecv":0
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
      "connectionsEstablished":0,
      "connectionsDropped":0,
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
		updatesSent    int
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
			1,
			181,
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, err := ParseNeighbour(fmt.Sprintf(sample, tt.neighborIP, tt.remoteAS, tt.localAS, tt.status, tt.updatesSent, tt.ipv4PrefixSent, tt.ipv6PrefixSent, tt.port))
			if err != nil {
				t.Fatal("Failed to parse ", err)
			}
			if !n.Ip.Equal(net.ParseIP(tt.neighborIP)) {
				t.Fatal("Expected neighbour ip", tt.neighborIP, "got", n.Ip.String())
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
			if tt.updatesSent != n.UpdatesSent {
				t.Fatal("Expected updates sent", tt.updatesSent, "got", n.UpdatesSent)
			}
			if tt.ipv4PrefixSent+tt.ipv6PrefixSent != n.PrefixSent {
				t.Fatal("Expected prefix sent", tt.ipv4PrefixSent+tt.ipv6PrefixSent, "got", n.PrefixSent)
			}
			if tt.port != n.Port {
				t.Fatal("Expected port", tt.port, "got", n.Port)
			}
		})
	}
}

const threeNeighbours = `
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
      "opensSent":0,
      "opensRecv":0,
      "notificationsSent":0,
      "notificationsRecv":0,
      "updatesSent":0,
      "updatesRecv":0,
      "keepalivesSent":0,
      "keepalivesRecv":0,
      "routeRefreshSent":0,
      "routeRefreshRecv":0,
      "capabilitySent":0,
      "capabilityRecv":0,
      "totalSent":0,
      "totalRecv":0
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
      "opensSent":0,
      "opensRecv":0,
      "notificationsSent":0,
      "notificationsRecv":0,
      "updatesSent":0,
      "updatesRecv":0,
      "keepalivesSent":0,
      "keepalivesRecv":0,
      "routeRefreshSent":0,
      "routeRefreshRecv":0,
      "capabilitySent":0,
      "capabilityRecv":0,
      "totalSent":0,
      "totalRecv":0
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
      "opensSent":0,
      "opensRecv":0,
      "notificationsSent":0,
      "notificationsRecv":0,
      "updatesSent":0,
      "updatesRecv":0,
      "keepalivesSent":0,
      "keepalivesRecv":0,
      "routeRefreshSent":0,
      "routeRefreshRecv":0,
      "capabilitySent":0,
      "capabilityRecv":0,
      "totalSent":0,
      "totalRecv":0
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
      "opensRecv":1,
      "notificationsSent":0,
      "notificationsRecv":0,
      "updatesSent":1,
      "updatesRecv":0,
      "keepalivesSent":1,
      "keepalivesRecv":1,
      "routeRefreshSent":0,
      "routeRefreshRecv":0,
      "capabilitySent":0,
      "capabilityRecv":0,
      "totalSent":3,
      "totalRecv":2
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
	nn, err := ParseNeighbours(threeNeighbours)
	if err != nil {
		t.Fatalf("Failed to parse %s", err)
	}
	if len(nn) != 4 {
		t.Fatalf("Expected 4 neighbours, got %d", len(nn))
	}
	sort.Slice(nn, func(i, j int) bool {
		return (bytes.Compare(nn[i].Ip, nn[j].Ip) < 0)
	})

	if !nn[0].Ip.Equal(net.ParseIP("172.18.0.2")) {
		t.Fatal("neighbour ip not matching")
	}
	if !nn[1].Ip.Equal(net.ParseIP("172.18.0.3")) {
		t.Fatal("neighbour ip not matching")
	}
	if !nn[2].Ip.Equal(net.ParseIP("172.18.0.4")) {
		t.Fatal("neighbour ip not matching")
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
	p, ok := peers["172.18.0.3"]
	if !ok {
		t.Fatal("Peer not found")
	}
	if p.Status != "up" {
		t.Fatal("wrong status")
	}
	if p.RemoteEchoInterval != 50 {
		t.Fatal("wrong echo interval")
	}
}
