---
title: Troubleshooting
weight: 20
---

Once you setup a loadbalancer on Kubernetes and it doesn't _seem_ to work right away, here are a few tips to troubleshoot.

- Check that the LoadBalancer service has endpoints (`kubectl -n <namespace> get endpoints <service>`) that are not pending - if they are, MetalLB will not respond to ARP requests for that service's external IP. That means if you have only one pod for your service and you lost the node hosting it, MetalLB will stop responding to ARP request until the replicaset will schedule the pod on another reachable node.
- SSH into one or more of your nodes and use `arping` and `tcpdump` to verify the ARP requests pass through your network

---

Below assumes you setup a loadbalancer with an IP of `192.168.1.240` and then we show usage of the commands and successful
requests going back and forth.

TL;DR - We found out that the IP `192.168.1.240` is located at the mac `FA:16:3E:5A:39:4C`.

If these requests fail, your network stack does not allow ARP to pass, therefore - we won't know about MetalLB's
assignment of the IP and can't connect.

## Tools

### `arping`

In this example, `arping` is used to trigger a request and it should receive a response.

The output should look similar to the following:

```bash
$ arping -I ens3 192.168.1.240
ARPING 192.168.1.240 from 192.168.1.35 ens3
Unicast reply from 192.168.1.240 [FA:16:3E:5A:39:4C]  1.077ms
Unicast reply from 192.168.1.240 [FA:16:3E:5A:39:4C]  1.321ms
Unicast reply from 192.168.1.240 [FA:16:3E:5A:39:4C]  0.883ms
Unicast reply from 192.168.1.240 [FA:16:3E:5A:39:4C]  0.968ms
^CSent 4 probes (1 broadcast(s))
Received 4 response(s)
```

`192.168.1.35` is the IP of one of the worker nodes and part of the same subnet.

> Make sure to use `arping` on a node which is not homing the `metallb-speaker` that ARPs for the address. ARP requests from the same host will be ignored. 

### `tcpdump`

`tcpdump` can be used on the same worker node or another one in order to verify ARP requests go back and forth.

Capture all replies from `192.168.1.240`:

```bash
$ tcpdump -n -i ens3 arp src host 192.168.1.240
tcpdump: verbose output suppressed, use -v or -vv for full protocol decode
listening on ens3, link-type EN10MB (Ethernet), capture size 262144 bytes
17:04:40.667263 ARP, Reply 192.168.1.240 is-at fa:16:3e:5a:39:4c, length 46
17:04:41.667485 ARP, Reply 192.168.1.240 is-at fa:16:3e:5a:39:4c, length 46
17:04:42.667572 ARP, Reply 192.168.1.240 is-at fa:16:3e:5a:39:4c, length 46
17:04:43.667545 ARP, Reply 192.168.1.240 is-at fa:16:3e:5a:39:4c, length 46
^C
4 packets captured
6 packets received by filter
0 packets dropped by kernel
```

### speaker logs

In addition to the above, make sure to watch the logs of MetalLB's speaker component for ARP requests and responses (using [kubetail](https://github.com/johanhaleby/kubetail)):

```bash
$ kubetail -l app.kubernetes.io/component=speaker -n metallb-system
...
```

{{% notice note %}}
Pinging the loadbalancer IP will not work! Since the IP is not owned by an host interface, the OS will not respond to ICMP packets. The service might be reachable with `curl` even if you cannot ping the loadbalancer IP. 
{{% /notice %}}

