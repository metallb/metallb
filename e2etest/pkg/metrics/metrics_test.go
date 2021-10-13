// SPDX-License-Identifier:Apache-2.0

package metrics

import (
	"testing"
)

const sample = `# HELP metallb_bgp_announced_prefixes_total Number of prefixes currently being advertised on the BGP session
# TYPE metallb_bgp_announced_prefixes_total gauge
metallb_bgp_announced_prefixes_total{peer="172.18.0.5:179"} 1
# HELP metallb_bgp_pending_prefixes_total Number of prefixes that should be advertised on the BGP session
# TYPE metallb_bgp_pending_prefixes_total gauge
metallb_bgp_pending_prefixes_total{peer="172.18.0.5:179"} 1
# HELP metallb_bgp_session_up BGP session state (1 is up, 0 is down)
# TYPE metallb_bgp_session_up gauge
metallb_bgp_session_up{peer="172.18.0.5:179"} 1
# HELP metallb_bgp_updates_total Number of BGP UPDATE messages sent
# TYPE metallb_bgp_updates_total counter
metallb_bgp_updates_total{peer="172.18.0.5:179"} 7
# HELP metallb_k8s_client_config_loaded_bool 1 if the MetalLB configuration was successfully loaded at least once.
# TYPE metallb_k8s_client_config_loaded_bool gauge
metallb_k8s_client_config_loaded_bool 1
# HELP metallb_k8s_client_config_stale_bool 1 if running on a stale configuration, because the latest config failed to load.
# TYPE metallb_k8s_client_config_stale_bool gauge
metallb_k8s_client_config_stale_bool 0
# HELP metallb_k8s_client_update_errors_total Number of k8s object updates that failed for some reason.
# TYPE metallb_k8s_client_update_errors_total counter
metallb_k8s_client_update_errors_total 0
# HELP metallb_k8s_client_updates_total Number of k8s object updates that have been processed.
# TYPE metallb_k8s_client_updates_total counter
metallb_k8s_client_updates_total 56
# HELP metallb_speaker_announced Services being announced from this node. This is desired state, it does not guarantee that the routing protocols have converged.
# TYPE metallb_speaker_announced gauge
metallb_speaker_announced{ip="192.168.10.0",node="kind-worker",protocol="bgp",service="bgp-8081/external-local-lb"} 1
`

func TestParsing(t *testing.T) {
	m, err := metricsFromString(sample)
	if err != nil {
		t.Fail()
	}
	updatesTotal, err := CounterForLabels("metallb_bgp_updates_total", map[string]string{"peer": "172.18.0.5:179"}, m)
	if err != nil {
		t.Fail()
	}
	if updatesTotal != 7 {
		t.Fatalf("expected value was 7, got %d", updatesTotal)
	}
	speakerAnnounced, err := GaugeForLabels("metallb_speaker_announced", map[string]string{
		"ip":       "192.168.10.0",
		"node":     "kind-worker",
		"protocol": "bgp",
		"service":  "bgp-8081/external-local-lb",
	}, m)
	if err != nil {
		t.Fail()
	}
	if speakerAnnounced != 1 {
		t.Fatalf("expected speakerAnnounced was 1, got %d", speakerAnnounced)
	}
	_, err = GaugeForLabels("metallb_speaker_announced", map[string]string{
		"ip":          "192.168.10.0",
		"node":        "kind-worker",
		"protocol":    "bgp",
		"service":     "bgp-8081/external-local-lb",
		"unnecessary": "label",
	}, m)
	if err == nil {
		t.Fail()
	}
}
