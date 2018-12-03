# Linux L3 plugin

The linux l3plugin is a Core Agent Plugin that is designed to configure ARP entries and routes in the 
linux environment. Configuration managed by this plugin is modelled by the [proto file](../model/l3/l3.proto). 

## ARP entries

The configuration must be stored in ETCD using the following key:

```
/vnf-agent/<agent-label>/linux/config/v1/arp/<label>
```

An example of configuration in json format can be found [here](../../../cmd/vpp-agent-ctl/json/arp-linux.json).

To insert config into etcd in json format [vpp-agent-ctl](../../../cmd/vpp-agent-ctl) 
can be used. We assume that we want to configure vpp with label `vpp1` and config is stored in
the `arp-json.json` file
```
vpp-agent-ctl -put "/vnf-agent/vpp1/linux/config/v1/arp/arp1" json/arp-linux.json
```

The vpp-agent-ctl also contains a simple predefined linux ARP entry config. It can be used for testing purposes.
To setup the predefined ARP config run:
```
vpp-agent-ctl -larp
```
To remove it run:
```
vpp-agent-ctl -larpd
```

## Routes

The configuration must be stored in etcd using the following key:

```
/vnf-agent/<agent-label>/linux/config/v1/route/<label>
```

An example of configuration in json format can be found [here](../../../cmd/vpp-agent-ctl/json/routes-linux-static.json)
for static routes and [here](../../../cmd/vpp-agent-ctl/json/routes-linux-default.json) for default routes.

To insert config into etcd in json format [vpp-agent-ctl](../../../cmd/vpp-agent-ctl) can be used.
We assume that we want to configure vpp with label `vpp1` and config is stored in the `routes-linux-static.json` 
or `routes-linux-default.json` file
```
vpp-agent-ctl -put "/vnf-agent/vpp1/linux/config/v1/route/route1" json/routes-linux-static.json
vpp-agent-ctl -put "/vnf-agent/vpp1/linux/config/v1/route/defRoute" json/routes-linux-default.json
```

The vpp-agent-ctl contains a simple predefined route config also. It can be used for testing purposes.
To setup the predefined route config run:
```
vpp-agent-ctl -lrte
```
To remove it run:
```
vpp-agent-ctl -lrted
```

To create a route, appropriate interface is required.