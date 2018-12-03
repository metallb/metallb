# Telemetry Plugin

The `telemetry` plugin is a core Agent Plugin for exporting telemetry
statistics from VPP for Prometheus.

Statistics are published via registry path `/vpp` on port `9191` and
updated every 30 seconds.

### Exported data

- VPP runtime (`show runtime`)

    ```
                 Name                 State         Calls          Vectors        Suspends         Clocks       Vectors/Call
    acl-plugin-fa-cleaner-process  event wait                0               0               1          4.24e4            0.00
    api-rx-from-ring                any wait                 0               0              18          2.92e5            0.00
    avf-process                    event wait                0               0               1          1.18e4            0.00
    bfd-process                    event wait                0               0               1          1.21e4            0.00
    cdp-process                     any wait                 0               0               1          1.34e5            0.00
    dhcp-client-process             any wait                 0               0               1          4.88e3            0.00
    dns-resolver-process            any wait                 0               0               1          5.88e3            0.00
    fib-walk                        any wait                 0               0               4          1.67e4            0.00
    flow-report-process             any wait                 0               0               1          3.19e3            0.00
    flowprobe-timer-process         any wait                 0               0               1          1.40e4            0.00
    igmp-timer-process             event wait                0               0               1          1.29e4            0.00
    ikev2-manager-process           any wait                 0               0               7          4.58e3            0.00
    ioam-export-process             any wait                 0               0               1          3.49e3            0.00
    ip-route-resolver-process       any wait                 0               0               1          7.07e3            0.00
    ip4-reassembly-expire-walk      any wait                 0               0               1          3.92e3            0.00
    ip6-icmp-neighbor-discovery-ev  any wait                 0               0               7          4.78e3            0.00
    ip6-reassembly-expire-walk      any wait                 0               0               1          5.16e3            0.00
    l2fib-mac-age-scanner-process  event wait                0               0               1          4.57e3            0.00
    lacp-process                   event wait                0               0               1          2.46e5            0.00
    lisp-retry-service              any wait                 0               0               4          1.05e4            0.00
    lldp-process                   event wait                0               0               1          6.79e4            0.00
    memif-process                  event wait                0               0               1          1.94e4            0.00
    nat-det-expire-walk               done                   1               0               0          5.68e3            0.00
    nat64-expire-walk              event wait                0               0               1          5.01e7            0.00
    rd-cp-process                   any wait                 0               0          174857          4.04e2            0.00
    send-rs-process                 any wait                 0               0               1          3.22e3            0.00
    startup-config-process            done                   1               0               1          4.99e3            0.00
    udp-ping-process                any wait                 0               0               1          2.65e4            0.00
    unix-cli-127.0.0.1:38288         active                  0               0               9          4.62e8            0.00
    unix-epoll-input                 polling          12735239               0               0          1.14e3            0.00
    vhost-user-process              any wait                 0               0               1          5.66e3            0.00
    vhost-user-send-interrupt-proc  any wait                 0               0               1          1.95e3            0.00
    vpe-link-state-process         event wait                0               0               1          2.27e3            0.00
    vpe-oam-process                 any wait                 0               0               4          1.11e4            0.00
    vxlan-gpe-ioam-export-process   any wait                 0               0               1          4.04e3            0.00
    wildcard-ip4-arp-publisher-pro event wait                0               0               1          5.49e4            0.00

    ```

    Example:

    ```sh
    # HELP vpp_runtime_calls Number of calls
    # TYPE vpp_runtime_calls gauge
    ...
    vpp_runtime_calls{agent="agent1",item="unix-epoll-input",thread="",threadID="0"} 7.65806939e+08
    ...
    # HELP vpp_runtime_clocks Number of clocks
    # TYPE vpp_runtime_clocks gauge
    ...
    vpp_runtime_clocks{agent="agent1",item="unix-epoll-input",thread="",threadID="0"} 1150
    ...
    # HELP vpp_runtime_suspends Number of suspends
    # TYPE vpp_runtime_suspends gauge
    ...
    vpp_runtime_suspends{agent="agent1",item="unix-epoll-input",thread="",threadID="0"} 0
    ...
    # HELP vpp_runtime_vectors Number of vectors
    # TYPE vpp_runtime_vectors gauge
    ...
    vpp_runtime_vectors{agent="agent1",item="unix-epoll-input",thread="",threadID="0"} 0
    ...
    # HELP vpp_runtime_vectors_per_call Number of vectors per call
    # TYPE vpp_runtime_vectors_per_call gauge
    ...
    vpp_runtime_vectors_per_call{agent="agent1",item="unix-epoll-input",thread="",threadID="0"} 0
    ...
    ```

- VPP buffers (`show buffers`)

    ```
     Thread             Name                 Index       Size        Alloc       Free       #Alloc       #Free
          0                       default           0        2048      0           0           0           0
          0                 lacp-ethernet           1         256      0           0           0           0
          0               marker-ethernet           2         256      0           0           0           0
          0                       ip4 arp           3         256      0           0           0           0
          0        ip6 neighbor discovery           4         256      0           0           0           0
          0                  cdp-ethernet           5         256      0           0           0           0
          0                 lldp-ethernet           6         256      0           0           0           0
          0           replication-recycle           7        1024      0           0           0           0
          0                       default           8        2048      0           0           0           0
    ```

    Example:

    ```sh
    # HELP vpp_buffers_alloc Allocated
    # TYPE vpp_buffers_alloc gauge
    vpp_buffers_alloc{agent="agent1",index="0",item="default",threadID="0"} 0
    vpp_buffers_alloc{agent="agent1",index="1",item="lacp-ethernet",threadID="0"} 0
    ...
    # HELP vpp_buffers_free Free
    # TYPE vpp_buffers_free gauge
    vpp_buffers_free{agent="agent1",index="0",item="default",threadID="0"} 0
    vpp_buffers_free{agent="agent1",index="1",item="lacp-ethernet",threadID="0"} 0
    ...
    # HELP vpp_buffers_num_alloc Number of allocated
    # TYPE vpp_buffers_num_alloc gauge
    vpp_buffers_num_alloc{agent="agent1",index="0",item="default",threadID="0"} 0
    vpp_buffers_num_alloc{agent="agent1",index="1",item="lacp-ethernet",threadID="0"} 0
    ...
    # HELP vpp_buffers_num_free Number of free
    # TYPE vpp_buffers_num_free gauge
    vpp_buffers_num_free{agent="agent1",index="0",item="default",threadID="0"} 0
    vpp_buffers_num_free{agent="agent1",index="1",item="lacp-ethernet",threadID="0"} 0
    ...
    # HELP vpp_buffers_size Size of buffer
    # TYPE vpp_buffers_size gauge
    vpp_buffers_size{agent="agent1",index="0",item="default",threadID="0"} 2048
    vpp_buffers_size{agent="agent1",index="1",item="lacp-ethernet",threadID="0"} 256
    ...
    ...
    ```

- VPP memory (`show memory`)

    ```
    Thread 0 vpp_main
    20071 objects, 14276k of 14771k used, 21k free, 12k reclaimed, 315k overhead, 1048572k capacity
    ```

    Example:

    ```sh
    # HELP vpp_memory_capacity Capacity
    # TYPE vpp_memory_capacity gauge
    vpp_memory_capacity{agent="agent1",thread="vpp_main",threadID="0"} 1.048572e+09
    # HELP vpp_memory_free Free memory
    # TYPE vpp_memory_free gauge
    vpp_memory_free{agent="agent1",thread="vpp_main",threadID="0"} 4000
    # HELP vpp_memory_objects Number of objects
    # TYPE vpp_memory_objects gauge
    vpp_memory_objects{agent="agent1",thread="vpp_main",threadID="0"} 20331
    # HELP vpp_memory_overhead Overhead
    # TYPE vpp_memory_overhead gauge
    vpp_memory_overhead{agent="agent1",thread="vpp_main",threadID="0"} 319000
    # HELP vpp_memory_reclaimed Reclaimed memory
    # TYPE vpp_memory_reclaimed gauge
    vpp_memory_reclaimed{agent="agent1",thread="vpp_main",threadID="0"} 0
    # HELP vpp_memory_total Total memory
    # TYPE vpp_memory_total gauge
    vpp_memory_total{agent="agent1",thread="vpp_main",threadID="0"} 1.471e+07
    # HELP vpp_memory_used Used memory
    # TYPE vpp_memory_used gauge
    vpp_memory_used{agent="agent1",thread="vpp_main",threadID="0"} 1.4227e+07
    ```

- VPP node counters (`show node counters`)

    ```
       Count                    Node                  Reason
        120406            ipsec-output-ip4            IPSec policy protect
        120406               esp-encrypt              ESP pkts received
        123692             ipsec-input-ip4            IPSEC pkts received
          3286             ip4-icmp-input             unknown type
        120406             ip4-icmp-input             echo replies sent
            14             ethernet-input             l3 mac mismatch
           102                arp-input               ARP replies sent
    ```

    Example:

    ```sh
    # HELP vpp_node_counter_count Count
    # TYPE vpp_node_counter_count gauge
    vpp_node_counter_count{agent="agent1",item="arp-input",reason="ARP replies sent"} 103
    vpp_node_counter_count{agent="agent1",item="esp-encrypt",reason="ESP pkts received"} 124669
    vpp_node_counter_count{agent="agent1",item="ethernet-input",reason="l3 mac mismatch"} 14
    vpp_node_counter_count{agent="agent1",item="ip4-icmp-input",reason="echo replies sent"} 124669
    vpp_node_counter_count{agent="agent1",item="ip4-icmp-input",reason="unknown type"} 3358
    vpp_node_counter_count{agent="agent1",item="ipsec-input-ip4",reason="IPSEC pkts received"} 128027
    vpp_node_counter_count{agent="agent1",item="ipsec-output-ip4",reason="IPSec policy protect"} 124669
    ```
    
### Configuration file

Telemetry plugin configuration file allows to change polling interval, or turn the polling off. 

`polling-interval` is time in nanoseconds between reads from the VPP.

`disabled` can be set to `true` in order to disable the plugin.    