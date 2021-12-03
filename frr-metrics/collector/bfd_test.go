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
	bfdMetricsTmpl = `
	# HELP metallb_bfd_session_up BFD session state (1 is up, 0 is down)
	# TYPE metallb_bfd_session_up gauge
	metallb_bfd_session_up{peer="{{ .Peer }}"} {{ .SessionUp }}
	# HELP metallb_bfd_control_packet_input Number of received BFD control packets
	# TYPE metallb_bfd_control_packet_input counter
	metallb_bfd_control_packet_input{peer="{{ .Peer }}"} {{ .ControlPacketInput }}
	# HELP metallb_bfd_control_packet_output Number of sent BFD control packets
	# TYPE metallb_bfd_control_packet_output counter
	metallb_bfd_control_packet_output{peer="{{ .Peer }}"} {{ .ControlPacketOutput }}
	# HELP metallb_bfd_echo_packet_input Number of received BFD echo packets
	# TYPE metallb_bfd_echo_packet_input counter
	metallb_bfd_echo_packet_input{peer="{{ .Peer }}"} {{ .EchoPacketInput }}
	# HELP metallb_bfd_echo_packet_output Number of sent BFD echo packets
	# TYPE metallb_bfd_echo_packet_output counter
	metallb_bfd_echo_packet_output{peer="{{ .Peer }}"} {{ .EchoPacketOutput }}
	# HELP metallb_bfd_session_down_events Number of BFD session down events
	# TYPE metallb_bfd_session_down_events counter
	metallb_bfd_session_down_events{peer="{{ .Peer }}"} {{ .SessionDownEvents }}
	# HELP metallb_bfd_session_up_events Number of BFD session up events
	# TYPE metallb_bfd_session_up_events counter
	metallb_bfd_session_up_events{peer="{{ .Peer }}"} {{ .SessionUpEvents }}
	# HELP metallb_bfd_zebra_notifications Number of BFD zebra notifications
	# TYPE metallb_bfd_zebra_notifications counter
	metallb_bfd_zebra_notifications{peer="{{ .Peer }}"} {{ .ZebraNotifications }}
	`

	bfdTests = []struct {
		desc                     string
		vtyshPeersOutput         string
		vtyshPeersCountersOutput string
		peer                     string
		sessionUp                int
		controlPacketInput       int
		controlPacketOutput      int
		echoPacketInput          int
		echoPacketOutput         int
		sessionUpEvents          int
		sessionDownEvents        int
		zebraNotifications       int
	}{
		{
			desc:                     "Output contains IPv4",
			vtyshPeersOutput:         peersIPv4,
			vtyshPeersCountersOutput: peersCountersIPv4,
			peer:                     "172.18.0.4",
			sessionUp:                1,
			controlPacketInput:       5,
			controlPacketOutput:      5,
			echoPacketInput:          0,
			echoPacketOutput:         0,
			sessionUpEvents:          1,
			sessionDownEvents:        0,
			zebraNotifications:       4,
		},
		{
			desc:                     "Output contains IPv6",
			vtyshPeersOutput:         peersIPv6,
			vtyshPeersCountersOutput: peersCountersIPv6,
			peer:                     "fc00:f853:ccd:e793::4",
			sessionUp:                0,
			controlPacketInput:       10,
			controlPacketOutput:      10,
			echoPacketInput:          0,
			echoPacketOutput:         0,
			sessionUpEvents:          1,
			sessionDownEvents:        0,
			zebraNotifications:       4,
		},
	}
	peersIPv4 = `
	[
		{
			"multihop":false,
			"peer":"172.18.0.4",
			"vrf":"default",
			"interface":"eth0",
			"id":2508913041,
			"remote-id":3444899611,
			"passive-mode":false,
			"status":"up",
			"uptime":13,
			"diagnostic":"ok",
			"remote-diagnostic":"ok",
			"receive-interval":300,
			"transmit-interval":300,
			"echo-interval":0,
			"detect-multiplier":3,
			"remote-receive-interval":300,
			"remote-transmit-interval":300,
			"remote-echo-interval":50,
			"remote-detect-multiplier":3
		}
	]
	`
	peersIPv6 = `
	[
		{
			"multihop":false,
			"peer":"fc00:f853:ccd:e793::4",
			"local":"fc00:f853:ccd:e793::6",
			"vrf":"default",
			"interface":"eth0",
			"id":1975516641,
			"remote-id":505304921,
			"passive-mode":false,
			"status":"down",
			"uptime":33,
			"diagnostic":"ok",
			"remote-diagnostic":"ok",
			"receive-interval":300,
			"transmit-interval":300,
			"echo-interval":0,
			"detect-multiplier":3,
			"remote-receive-interval":300,
			"remote-transmit-interval":300,
			"remote-echo-interval":50,
			"remote-detect-multiplier":3
		}
	]
	`
	peersCountersIPv4 = `
	[
		{
			"multihop":false,
			"peer":"172.18.0.4",
			"vrf":"default",
			"interface":"eth0",
			"control-packet-input":5,
			"control-packet-output":5,
			"echo-packet-input":0,
			"echo-packet-output":0,
			"session-up":1,
			"session-down":0,
			"zebra-notifications":4
		}
	]
	`
	peersCountersIPv6 = `
	[
		{
			"multihop":false,
			"peer":"fc00:f853:ccd:e793::4",
			"local":"fc00:f853:ccd:e793::6",
			"vrf":"default",
			"interface":"eth0",
			"control-packet-input":10,
			"control-packet-output":10,
			"echo-packet-input":0,
			"echo-packet-output":0,
			"session-up":1,
			"session-down":0,
			"zebra-notifications":4
		}
	]
	`
)

func TestBFDCollect(t *testing.T) {
	for _, test := range bfdTests {
		t.Run(test.desc, func(t *testing.T) {
			tmpl, err := template.New(test.desc).Parse(bfdMetricsTmpl)
			if err != nil {
				t.Errorf("expected no error but got %s", err)
			}

			var w bytes.Buffer
			err = tmpl.Execute(&w, map[string]interface{}{
				"Peer":                test.peer,
				"SessionUp":           test.sessionUp,
				"ControlPacketInput":  test.controlPacketInput,
				"ControlPacketOutput": test.controlPacketOutput,
				"EchoPacketInput":     test.echoPacketInput,
				"EchoPacketOutput":    test.echoPacketOutput,
				"SessionUpEvents":     test.sessionUpEvents,
				"SessionDownEvents":   test.sessionDownEvents,
				"ZebraNotifications":  test.zebraNotifications,
			})

			if err != nil {
				t.Errorf("expected no error but got %s", err)
			}

			l := log.NewNopLogger()
			collector := NewBFD(l)
			collector.frrCli = func(args ...string) (string, error) {
				if args[0] == "show bfd peers json" {
					return test.vtyshPeersOutput, nil
				}
				return test.vtyshPeersCountersOutput, nil
			}
			buf := bytes.NewReader(w.Bytes())
			err = testutil.CollectAndCompare(collector, buf)
			if err != nil {
				t.Errorf("expected no error but got %s", err)
			}
		})

	}
}
