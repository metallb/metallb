// SPDX-License-Identifier:Apache-2.0

package collector

import (
	"bytes"
	"testing"
	"text/template"

	"github.com/go-kit/kit/log"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

var (
	metricsTmpl = `
	# HELP metallb_bgp_announced_prefixes_total Number of prefixes currently being advertised on the BGP session
	# TYPE metallb_bgp_announced_prefixes_total gauge
	metallb_bgp_announced_prefixes_total{peer="{{ .NeighborIP }}"} {{ .AnnouncedPrefixes }}
	# HELP metallb_bgp_session_up BGP session state (1 is up, 0 is down)
	# TYPE metallb_bgp_session_up gauge
	metallb_bgp_session_up{peer="{{ .NeighborIP }}"} {{ .SessionUp }}
	# HELP metallb_bgp_updates_total Number of BGP UPDATE messages sent
	# TYPE metallb_bgp_updates_total counter
	metallb_bgp_updates_total{peer="{{ .NeighborIP }}"} {{ .UpdatesTotal }}
	`

	tests = []struct {
		desc              string
		vtyshOutput       string
		neighborIP        string
		announcedPrefixes int
		sessionUp         int
		updatesTotal      int
	}{
		{
			desc:              "Output contains only IPv4 advertisements",
			vtyshOutput:       neighborsIPv4Only,
			neighborIP:        "172.18.0.4:179",
			announcedPrefixes: 3,
			sessionUp:         1,
			updatesTotal:      2,
		},
		{
			desc:              "Output contains mixed IPv4 and IPv6 advertisements",
			vtyshOutput:       neighborsDual,
			neighborIP:        "172.18.0.4:180",
			announcedPrefixes: 6,
			sessionUp:         1,
			updatesTotal:      5,
		},
	}
	neighborsIPv4Only = `
	{
		"172.18.0.4":{
		  "remoteAs":64512,
		  "localAs":64513,
		  "nbrExternalLink":true,
		  "hostname":"bgpd",
		  "bgpVersion":4,
		  "remoteRouterId":"172.18.0.4",
		  "localRouterId":"172.18.0.3",
		  "bgpState":"Established",
		  "bgpTimerUpMsec":1082000,
		  "bgpTimerUpString":"00:18:02",
		  "bgpTimerUpEstablishedEpoch":1632032518,
		  "bgpTimerLastRead":2000,
		  "bgpTimerLastWrite":2000,
		  "bgpInUpdateElapsedTimeMsecs":1081000,
		  "bgpTimerHoldTimeMsecs":180000,
		  "bgpTimerKeepAliveIntervalMsecs":60000,
		  "neighborCapabilities":{
			"4byteAs":"advertisedAndReceived",
			"addPath":{
			  "ipv4Unicast":{
				"rxAdvertisedAndReceived":true
			  }
			},
			"routeRefresh":"advertisedAndReceivedOldNew",
			"multiprotocolExtensions":{
			  "ipv4Unicast":{
				"advertisedAndReceived":true
			  }
			},
			"hostName":{
			  "advHostName":"kind-control-plane",
			  "advDomainName":"n\/a",
			  "rcvHostName":"bgpd",
			  "rcvDomainName":"n\/a"
			},
			"gracefulRestart":"advertisedAndReceived",
			"gracefulRestartRemoteTimerMsecs":120000,
			"addressFamiliesByPeer":"none"
		  },
		  "gracefulRestartInfo":{
			"endOfRibSend":{
			  "ipv4Unicast":true
			},
			"endOfRibRecv":{
			  "ipv4Unicast":true
			},
			"localGrMode":"Helper*",
			"remoteGrMode":"Helper",
			"rBit":true,
			"timers":{
			  "configuredRestartTimer":120,
			  "receivedRestartTimer":120
			},
			"ipv4Unicast":{
			  "fBit":false,
			  "endOfRibStatus":{
				"endOfRibSend":true,
				"endOfRibSentAfterUpdate":true,
				"endOfRibRecv":true
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
			"updatesSent":2,
			"updatesRecv":2,
			"keepalivesSent":19,
			"keepalivesRecv":19,
			"routeRefreshSent":0,
			"routeRefreshRecv":0,
			"capabilitySent":0,
			"capabilityRecv":0,
			"totalSent":22,
			"totalRecv":22
		  },
		  "minBtwnAdvertisementRunsTimerMsecs":0,
		  "addressFamilyInfo":{
			"ipv4Unicast":{
			  "updateGroupId":1,
			  "subGroupId":1,
			  "packetQueueLength":0,
			  "commAttriSentToNbr":"extendedAndStandard",
			  "acceptedPrefixCounter":0,
			  "sentPrefixCounter":3
			}
		  },
		  "connectionsEstablished":1,
		  "connectionsDropped":0,
		  "lastResetTimerMsecs":1083000,
		  "lastResetDueTo":"Waiting for peer OPEN",
		  "lastResetCode":32,
		  "hostLocal":"172.18.0.3",
		  "portLocal":42692,
		  "hostForeign":"172.18.0.4",
		  "portForeign":179,
		  "nexthop":"172.18.0.3",
		  "nexthopGlobal":"fc00:f853:ccd:e793::3",
		  "nexthopLocal":"fe80::42:acff:fe12:3",
		  "bgpConnection":"sharedNetwork",
		  "connectRetryTimer":120,
		  "estimatedRttInMsecs":2,
		  "readThread":"on",
		  "writeThread":"on"
		}
	  }	  
	`
	neighborsDual = `
	{
		"172.18.0.4":{
		  "remoteAs":64512,
		  "localAs":64513,
		  "nbrExternalLink":true,
		  "hostname":"bgpd",
		  "bgpVersion":4,
		  "remoteRouterId":"172.18.0.4",
		  "localRouterId":"172.18.0.3",
		  "bgpState":"Established",
		  "bgpTimerUpMsec":1082000,
		  "bgpTimerUpString":"00:18:02",
		  "bgpTimerUpEstablishedEpoch":1632032518,
		  "bgpTimerLastRead":2000,
		  "bgpTimerLastWrite":2000,
		  "bgpInUpdateElapsedTimeMsecs":1081000,
		  "bgpTimerHoldTimeMsecs":180000,
		  "bgpTimerKeepAliveIntervalMsecs":60000,
		  "neighborCapabilities":{
			"4byteAs":"advertisedAndReceived",
			"addPath":{
			  "ipv4Unicast":{
				"rxAdvertisedAndReceived":true
			  },
			  "ipv6Unicast":{
				"rxAdvertisedAndReceived":true
			  }
			},
			"routeRefresh":"advertisedAndReceivedOldNew",
			"multiprotocolExtensions":{
			  "ipv4Unicast":{
				"advertisedAndReceived":true
			  },
			  "ipv6Unicast":{
				"advertisedAndReceived":true
			  }
			},
			"hostName":{
			  "advHostName":"kind-control-plane",
			  "advDomainName":"n\/a",
			  "rcvHostName":"bgpd",
			  "rcvDomainName":"n\/a"
			},
			"gracefulRestart":"advertisedAndReceived",
			"gracefulRestartRemoteTimerMsecs":120000,
			"addressFamiliesByPeer":"none"
		  },
		  "gracefulRestartInfo":{
			"endOfRibSend":{
			  "ipv4Unicast":true,
			  "ipv6Unicast":true
			},
			"endOfRibRecv":{
			  "ipv4Unicast":true,
			  "ipv6Unicast":true
			},
			"localGrMode":"Helper*",
			"remoteGrMode":"Helper",
			"rBit":true,
			"timers":{
			  "configuredRestartTimer":120,
			  "receivedRestartTimer":120
			},
			"ipv4Unicast":{
			  "fBit":false,
			  "endOfRibStatus":{
				"endOfRibSend":true,
				"endOfRibSentAfterUpdate":true,
				"endOfRibRecv":true
			  },
			  "timers":{
				"stalePathTimer":360
			  }
			},
			"ipv6Unicast":{
			  "fBit":false,
			  "endOfRibStatus":{
				"endOfRibSend":true,
				"endOfRibSentAfterUpdate":true,
				"endOfRibRecv":true
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
			"updatesSent":5,
			"updatesRecv":5,
			"keepalivesSent":19,
			"keepalivesRecv":19,
			"routeRefreshSent":0,
			"routeRefreshRecv":0,
			"capabilitySent":0,
			"capabilityRecv":0,
			"totalSent":25,
			"totalRecv":25
		  },
		  "minBtwnAdvertisementRunsTimerMsecs":0,
		  "addressFamilyInfo":{
			"ipv4Unicast":{
			  "updateGroupId":1,
			  "subGroupId":1,
			  "packetQueueLength":0,
			  "commAttriSentToNbr":"extendedAndStandard",
			  "acceptedPrefixCounter":0,
			  "sentPrefixCounter":3
			},
			"ipv6Unicast":{
			  "peerGroupMember":"uplink",
			  "updateGroupId":2,
			  "subGroupId":2,
			  "packetQueueLength":0,
			  "commAttriSentToNbr":"extendedAndStandard",
			  "outboundPathPolicyConfig":true,
			  "outgoingUpdatePrefixFilterList":"only-host-prefixes",
			  "acceptedPrefixCounter":13,
			  "sentPrefixCounter":3
			}
		  },
		  "connectionsEstablished":1,
		  "connectionsDropped":0,
		  "lastResetTimerMsecs":1083000,
		  "lastResetDueTo":"Waiting for peer OPEN",
		  "lastResetCode":32,
		  "hostLocal":"172.18.0.3",
		  "portLocal":42692,
		  "hostForeign":"172.18.0.4",
		  "portForeign":180,
		  "nexthop":"172.18.0.3",
		  "nexthopGlobal":"fc00:f853:ccd:e793::3",
		  "nexthopLocal":"fe80::42:acff:fe12:3",
		  "bgpConnection":"sharedNetwork",
		  "connectRetryTimer":120,
		  "estimatedRttInMsecs":2,
		  "readThread":"on",
		  "writeThread":"on"
		}
	  }	  
	`
)

func TestCollect(t *testing.T) {
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			tmpl, err := template.New(tc.desc).Parse(metricsTmpl)
			if err != nil {
				t.Errorf("expected no error but got %s", err)
			}

			var w bytes.Buffer
			err = tmpl.Execute(&w, map[string]interface{}{
				"NeighborIP":        tc.neighborIP,
				"AnnouncedPrefixes": tc.announcedPrefixes,
				"SessionUp":         tc.sessionUp,
				"UpdatesTotal":      tc.updatesTotal,
			})

			if err != nil {
				t.Errorf("expected no error but got %s", err)
			}

			l := log.NewNopLogger()
			collector := NewBGP(l)
			collector.frrCli = func(args ...string) (string, error) {
				return tc.vtyshOutput, nil
			}
			buf := bytes.NewReader(w.Bytes())
			err = testutil.CollectAndCompare(collector, buf)
			if err != nil {
				t.Errorf("expected no error but got %s", err)
			}
		})

	}
}
