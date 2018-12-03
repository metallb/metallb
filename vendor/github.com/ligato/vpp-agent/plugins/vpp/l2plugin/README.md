# L2 Plugin

The `l2plugin` is a Core Agent Plugin designed to configure Bridge Domains and L2 FIB in the VPP.
Configuration managed by this plugin is modelled by [l2 proto file](../model/l2/l2.proto). The
configuration must be stored in etcd using following keys:

```
/vnf-agent/<agent-label>/vpp/config/v1/bd/<bridge-domain-name>
```

```
/vnf-agent/<agent-label>/vpp/config/v1/bd/<bridge-domain-name>/fib/<mac-address>
```

```
/vnf-agent/<agent-label>/vpp/config/v1/xconnect/<rx-interface-name>
```

**JSON configuration example with vpp-agent-ctl**

An example of basic bridge domain configuration in JSON format can be found
[here](../../../cmd/vpp-agent-ctl/json/bridge-domain.json).
A bit more advanced example which includes ARP termination entries can be found
[here](../../../cmd/vpp-agent-ctl/json/bridge-domain-arp.json). Example of FIB tables 
is available [here](../../../cmd/vpp-agent-ctl/json/l2_fib.json)

To insert config into etcd in JSON format [vpp-agent-ctl](../../../cmd/vpp-agent-ctl) can be used.
Let's assume that we want to configure vpp with the label `vpp1` and config for bridge domain `bd1` is stored
in the `bridge-domain.json` file. Furthermore, we assume that the bridge domain `bd1` contains tap interface `tap1`
with configuration stored in `tap.json`. To convey this configuration to the agent through northbound API,
you would execute:
```
vpp-agent-ctl -put "/vnf-agent/vpp1/vpp/config/v1/interface/tap1" tap.json
vpp-agent-ctl -put "/vnf-agent/vpp1/vpp/config/v1/bd/bd1" bridge-domain.json
```

Agent operating with the microservice label `vpp1` will then create bridge domain,
assign every interface which belongs to it (`tap1` in the example above), and set BVI (if there is such an interface).
If one or more ARP entries are available, all of them are written to ARP Termination table.

Every L2 FIB entry is configured for specific bridge domain and interface, both of them have to be 
configured (in any order) to be written to FIB table.


**Inbuilt configuration example with vpp-agent-ctl**

The `vpp-agent-ctl` binary also ships with some simple predefined bridge domain configurations.
It is meant to be used solely for testing purposes.

First create a new tap interface `tap1`:
```
vpp-agent-ctl /opt/vpp-agent/dev/etcd.conf -tap
```

To configure a new bridge domain `bd1` containing the previously created tap interface `tap1`, use:
```
vpp-agent-ctl /opt/vpp-agent/dev/etcd.conf -bd
```

To create a new tap interface `tap2` and to L2-xConnect it with `tap1`, use:
```
vpp-agent-ctl /opt/vpp-agent/dev/etcd.conf -xconn
```

To create L2 FIB table, run:
```
vpp-agent-ctl /opt/vpp-agent/dev/etcd.conf -fib
```

