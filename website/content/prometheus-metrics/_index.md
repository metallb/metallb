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

## MetalLB BGP metrics
#### Note: all the metrics related to a BGP session contain a label that refers to the bgppeer the session is opened against. For example, with 4 BGP peers, the `metallb_bgp_updates_total` metric could appear as the following:
```bash
# HELP metallb_bgp_updates_total Number of BGP UPDATE messages sent
# TYPE metallb_bgp_updates_total counter
metallb_bgp_updates_total{peer="172.23.0.5:0"} 1
metallb_bgp_updates_total{peer="172.23.0.6:0"} 1
metallb_bgp_updates_total{peer="172.30.0.2:0"} 1
metallb_bgp_updates_total{peer="172.30.0.3:0"} 1
```

| Name                                 | Description                                                      |
| ------------------------------------ | ---------------------------------------------------------------- |
| metallb_bgp_session_up               | BGP session state (1 is up, 0 is down)                           |
| metallb_bgp_updates_total            | Number of BGP UPDATE messages sent                               |
| metallb_bgp_announced_prefixes_total | Number of prefixes currently being advertised on the BGP session |

## MetalLB BGP metrics (on FRR mode only)

| Name                               | Description                               |
| ---------------------------------- | ----------------------------------------- |
| metallb_bgp_opens_sent             | Number of BGP open messages sent          |
| metallb_bgp_opens_received         | Number of BGP open messages received      |
| metallb_bgp_notifications_sent     | Number of BGP notification messages sent  |
| metallb_bgp_updates_total_received | Number of BGP UPDATE messages received    |
| metallb_bgp_keepalives_sent        | Number of BGP keepalive messages sent     |
| metallb_bgp_keepalives_received    | Number of BGP keepalive messages received |
| metallb_bgp_route_refresh_sent     | Number of BGP route refresh messages sent |
| metallb_bgp_total_sent             | Number of total BGP messages sent         |
| metallb_bgp_total_received         | Number of total BGP messages received     |

## MetalLB BFD Metrics (on FRR mode only)
| Name                                    | Description                            |
| --------------------------------------- | -------------------------------------- |
| metallb_bfd_session_up                  | BFD session state (1 is up, 0 is down) |
| metallb_bfd_control_packet_input        | Number of received BFD control packets |
| metallb_bfd_control_packet_output       | Number of sent BFD control packet      |
| metallb_bfd_echo_packet_input           | Number of received BFD echo packets    |
| metallb_bfd_echo_packet_output          | Number of sent BFD echo packets        |
| metallb_bfd_session_up_events           | Number of BFD session up events        |
| metallb_bfd_session_down_events         | Number of BFD session down events      |
| metallb_bfd_session_zebra_notifications | Number of BFD zebra notifications      |
