---
title: Prometheus Metrics
---

MetalLB exposes different Prometheus metrics that are listed below.

## MetalLB Allocator Addresses metrics

| Name                                     | Description                              |
| ---------------------------------------- | ---------------------------------------- |
| metallb_allocator_addresses_in_use_total | Number of IP addresses in use, per pool  |
| metallb_allocator_addresses_total        | Number of usable IP addresses, per pool  |

## MetalLB K8S client metrics

| Name                                   | Description                                                                      |
| -------------------------------------- | -------------------------------------------------------------------------------- |
| metallb_k8s_client_updates_total       | Number of k8s object updates that have been processed                            |
| metallb_k8s_client_update_errors_total | Number of k8s object updates that failed for some reason                         |
| metallb_k8s_client_config_loaded_bool  | 1 if the MetalLB configuration was successfully loaded at least once             |
| metallb_k8s_client_config_stale_bool   | 1 if running on a stale configuration, because the latest config failed to load  |

## MetalLB BGP metrics (native mode only)

These metrics are emitted by the MetalLB speaker and are only populated when running in native BGP mode.
In FRR-based modes, the equivalent metrics are exposed either by the separate FRR-K8s
pods (FRR-K8s mode) or by the FRR sidecar container (deprecated FRR mode)
(see [FRR-K8s BGP metrics](#frr-k8s-bgp-metrics) below).

| Name                                 | Description                                                      |
| ------------------------------------ | ---------------------------------------------------------------- |
| metallb_bgp_session_up               | BGP session state (1 is up, 0 is down)                           |
| metallb_bgp_updates_total            | Number of BGP UPDATE messages sent                               |
| metallb_bgp_announced_prefixes_total | Number of prefixes currently being advertised on the BGP session |

## FRR-K8s BGP and BFD metrics

When running in the default [FRR-K8s mode]({{% relref "concepts/bgp.md" %}}#frr-k8s-mode), additional BGP and BFD metrics
are exposed by the FRR-K8s pods. These metrics use the `frrk8s_` prefix and the `peer` label contains
only the peer IP address (without port).

For example:

```bash
# HELP frrk8s_bgp_updates_total Number of BGP UPDATE messages sent
# TYPE frrk8s_bgp_updates_total counter
frrk8s_bgp_updates_total{peer="172.23.0.5"} 1
frrk8s_bgp_updates_total{peer="172.23.0.6"} 1
frrk8s_bgp_updates_total{peer="172.30.0.2"} 1
frrk8s_bgp_updates_total{peer="172.30.0.3"} 1
```

When using BGP with a VRF, an additional label with the name of the VRF is added:

```bash
frrk8s_bgp_updates_total{peer="172.23.0.5"} 1
frrk8s_bgp_updates_total{peer="172.23.0.5",vrf="red"} 1
```

### FRR-K8s BGP metrics

| Name                                     | Description                                                      |
| ---------------------------------------- | ---------------------------------------------------------------- |
| frrk8s_bgp_session_up                    | BGP session state (1 is up, 0 is down)                           |
| frrk8s_bgp_announced_prefixes_total      | Number of prefixes currently being advertised on the BGP session  |
| frrk8s_bgp_received_prefixes_total       | Number of prefixes currently being received on the BGP session    |
| frrk8s_bgp_opens_sent                    | Number of BGP open messages sent                                 |
| frrk8s_bgp_opens_received                | Number of BGP open messages received                             |
| frrk8s_bgp_notifications_sent            | Number of BGP notification messages sent                         |
| frrk8s_bgp_updates_total                 | Number of BGP UPDATE messages sent                               |
| frrk8s_bgp_updates_total_received        | Number of BGP UPDATE messages received                           |
| frrk8s_bgp_keepalives_sent               | Number of BGP keepalive messages sent                            |
| frrk8s_bgp_keepalives_received           | Number of BGP keepalive messages received                        |
| frrk8s_bgp_route_refresh_sent            | Number of BGP route refresh messages sent                        |
| frrk8s_bgp_total_sent                    | Number of total BGP messages sent                                |
| frrk8s_bgp_total_received                | Number of total BGP messages received                            |

### FRR-K8s BFD metrics

| Name                                     | Description                             |
| ---------------------------------------- | --------------------------------------- |
| frrk8s_bfd_session_up                    | BFD session state (1 is up, 0 is down)  |
| frrk8s_bfd_control_packet_input          | Number of received BFD control packets  |
| frrk8s_bfd_control_packet_output         | Number of sent BFD control packets      |
| frrk8s_bfd_echo_packet_input             | Number of received BFD echo packets     |
| frrk8s_bfd_echo_packet_output            | Number of sent BFD echo packets         |
| frrk8s_bfd_session_up_events             | Number of BFD session up events         |
| frrk8s_bfd_session_down_events           | Number of BFD session down events       |
| frrk8s_bfd_zebra_notifications           | Number of BFD zebra notifications       |

### Backward-compatible metric relabeling

In previous versions of MetalLB, the FRR-based BGP and BFD metrics were exposed with the `metallb_` prefix
(e.g. `metallb_bgp_session_up`) and the BGP `peer` label included the port (e.g. `peer="172.23.0.5:179"`).

If you are migrating from the deprecated FRR mode and need to preserve the `metallb_` prefix for compatibility
with existing dashboards or alerts, you can configure Prometheus
[metric relabeling](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#metric_relabel_configs)
on the FRR-K8s ServiceMonitor to rename the metrics at ingestion time:

```yaml
metricRelabelings:
- sourceLabels: [__name__]
  regex: "frrk8s_bgp_(.*)"
  targetLabel: "__name__"
  replacement: "metallb_bgp_$1"
- sourceLabels: [__name__]
  regex: "frrk8s_bfd_(.*)"
  targetLabel: "__name__"
  replacement: "metallb_bfd_$1"
```

When deploying via Helm, this can be configured on the FRR-K8s subchart's ServiceMonitor
(see the commented example in `values.yaml`).
When deploying via kustomize, the `config/prometheus-frr-k8s` overlay includes these relabeling rules.

{{% notice note %}}
Note that the `peer` label format difference (`IP` vs `IP:port`) cannot be addressed via metric relabeling.
If your queries filter on `peer` with a port suffix, they will need to be updated.
{{% /notice %}}
