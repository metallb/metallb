# Linux interface Plugin


The plugin watches the northbound configuration of Linux network interfaces,
which is modelled by [interfaces proto file](../model/interfaces/interfaces.proto)
and stored in ETCD under the following key:

```
/vnf-agent/<agent-label>/linux/config/v1/interface/<interface-name>
```

Linux interface is uniquely identified in the northbound configuration by its name. The same string is also used
in the Linux network stack to label the interface and it has to be unique across all network namespaces.
It is therefore recommended for the northbound applications to prefix interface names with namespace
identifiers in case there is a chance of collisions across namespaces.

The re-synchronization procedure is a bit simpler than that from the VPP plugins. Linux plugin will not touch existing
Linux interfaces which are not part of the northbound configuration. Managed interfaces which are already present when
the agent starts will be re-created from the scratch.

The current version does not yet support interface notifications and statistics.

Linux plugin is also an optional dependency for `ifplugin` from VPP plugins. If `linuxplugin` is loaded, `ifplugin` will
be watching for newly created and removed Linux interfaces. This is useful because `AFPACKET` interface will not function
properly if it gets created when the target host interface is not available. `ifplugin` will ensure that `AFPACKET` interfaces
are always created *after* the associated host interfaces. If the host interface gets removed so will the associated
afpacket. `ifplugin` will keep the afpacket configuration in the cache and re-create it once the host interface is available again.
To enable this feature, `linuxplugin` must be loaded *before* VPP default plugins.

*VETH*

Virtual Ethernet interfaces come in pairs, and they are connected like a tube — whatever comes in one VETH
interface will come out the other peer VETH interface. As a result, you can use VETH interfaces to connect
a network namespace to the outside world via the “default” or “global” namespace where physical interfaces
exist.

VETH pair is configured through the northbound API as two separate interfaces, both of the type `LinuxInterfaces_VETH`
and pointing to each other through the `veth.peer_if_name` reference.
Agent will physically create the pair only after both sides are configured and the target namespaces are available. 
Similarly, to maintain the symmetry, VETH pair gets removed from the Linux network stack as soon as any of the sides is
un-configured or a target namespace disappears (e.g. the destination microservice has terminated). The agent, however,
will not forget a partial configuration and once all the requirements are met again the VETH will get automatically recreated. 

*VETH usage example*

Consider a scenario in which we need to connect VPP running in the host (i.e. "default")
namespace with a VPP running inside a Docker container. This can be achieved by both memif interface
as well as through a combination of a Linux VETH pair with AF packet interfaces from VPP (confusingly called `host` interface in VPP).
First you would supply northbound configurations for both sides of the VETH pair. That is two interfaces
of type `LinuxInterfaces_VETH` with one end in the default namespace (`namespace.Name=""`), making it visible for the host VPP,
and the other end inserted into the namespace of the container with the other VPP. This can be achieved by either directly referencing
the PID of the container (`namespace.type=PID_REF_NS; namespace.pid=<PID>`), or, if the container is actually a microservice,
a more convenient way is to reference the namespace by the microservice label
(`namespace.type=MICROSERVICE_REF_NS; namespace.microservice=<LABEL>`).
Peer references (`veth.peer_if_name`) need to be configured on both interfaces such that they point to each other.
Next step is to create one AF packet interface on both VPPs and assign them to their respective sides of the VETH pair.
Packet leaving `host` interface of one of these VPPs will get sent to its VETH counterpart via `AF_PACKET` socket, then forwarded
to the other side of the VETH pair inside the Linux network stack, crossing namespaces in the process, and finally it is transferred
to the destination and originally opposite VPP through a `AF_PACKET` socket once again.

**JSON configuration example with vpp-agent-ctl**

An example configuration for both ends of VETH in JSON format can
be found [here](../../../cmd/vpp-agent-ctl/json/veth1.json) and [here](../../../cmd/vpp-agent-ctl/json/veth2.json).

To insert config into etcd in JSON format [vpp-agent-ctl](../../../cmd/vpp-agent-ctl)
can be used. For example, to configure interface `veth1`, use the configuration in the `veth1.json` file and
run the following `vpp-agent-ctl` command:
```
vpp-agent-ctl -put "/vnf-agent/my-agent/linux/config/v1/interface/veth1" json/veth1.json
```

**Inbuilt configuration example with vpp-agent-ctl**

The `vpp-agent-ctl` binary also ships with some simple predefined VETH configurations.
This is intended solely for testing purposes.

To create the `veth1-veth2` pair where `veth1` in the named namespace `ns1` with IP address `192.168.22.1` 
and `veth` is in the named namespace `ns2` with IP address `192.168.22.2`, run:
```
vpp-agent-ctl -veth
```

To remove the both interfaces, run:
```
vpp-agent-ctl -vethd
```

Run `vpp-agent-ctl` with no arguments to get the list of all available commands and options.
The documentation for `vpp-agent-ctl` is incomplete right now, and the only way to find out
what a given command does is to [study the source code itself](../../../cmd/vpp-agent-ctl).