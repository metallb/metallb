# L3 plugin

The l3plugin is a Core Agent Plugin that is designed to configure ARP entries and routes in the VPP. 
Configuration managed by this plugin is modelled by the [proto file](../model/l3/l3.proto). 

## ARP entries

The configuration must be stored in etcd using the following key:

```
/vnf-agent/<agent-label>/vpp/config/v1/arp/<interface>/<ip-address>
```

An example of configuration in json format can be found [here](../../../cmd/vpp-agent-ctl/json/arp.json).

To insert config into etcd in json format [vpp-agent-ctl](../../../cmd/vpp-agent-ctl) 
can be used. We assume that we want to configure vpp with label `vpp1` and config is stored in
the `arp.json` file
```
vpp-agent-ctl -put "/vnf-agent/vpp1/vpp/config/v1/arp/tap1/192.168.10.21" json/arp.json
```

The vpp-agent-ctl also contains a simple predefined ARP entry config. It can be used for testing purposes.
To setup the predefined ARP config run:
```
vpp-agent-ctl -arp
```
To remove it run:
```
vpp-agent-ctl -arpd
```

## Proxy ARP

Proxy ARP configuration is composed from two parts; interfaces that are enabled and IP address ranges.
Both of these configuration types are stored under separate keys.

For proxy arp interface array, use key:

```
/vnf-agent/<agent-label>/vpp/config/v1/proxyarp/interface/<if-cfg-label>
```

For proxy arp ranges:

```
/vnf-agent/<agent-label>/vpp/config/v1/proxyarp/range/<rng-cfg-label>
```

An example configuration for interfaces can be found [here](../../../cmd/vpp-agent-ctl/json/proxy-arp-interface.json).
An example configuration for IP ranges can be found [here](../../../cmd/vpp-agent-ctl/json/proxy-arp-ranges.json).

Predefined configuration in vpp-agent-ctl for interfaces:

```
vpp-agent-ctl -prxi
```

For ranges:

```
vpp-agent-ctl -prxd
```

## Routes

The configuration must be stored in etcd using the following key:

```
/vnf-agent/<agent-label>/vpp/config/v1/vrf/0/fib/
```

An example of configuration in json format can be found [here](../../../cmd/vpp-agent-ctl/json/routes.json).

Note: Value `0` in vrfID field denotes default VRF in vpp. Since it is default value it is omitted in the config above.
 If you want to configure a route for a VRF other than default, make sure that the VRF has already been created.

To insert config into etcd in json format [vpp-agent-ctl](../../../cmd/vpp-agent-ctl) can be used.
We assume that we want to configure vpp with label `vpp1` and config is stored in the `routes.json` file
```
vpp-agent-ctl -put "/vnf-agent/vpp1/vpp/config/v1/vrf/0/fib" json/routes.json
```

The vpp-agent-ctl contains a simple predefined route config also. It can be used for testing purposes.
To setup the predefined route config run:
```
vpp-agent-ctl -route
```
To remove it run:
```
vpp-agent-ctl -routed
```
