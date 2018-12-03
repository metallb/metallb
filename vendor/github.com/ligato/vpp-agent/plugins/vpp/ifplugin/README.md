# VPP interface Plugin

The `ifplugin` is a Core Agent Plugin for configuration of NICs, memifs,
VXLANs, loopback interfaces and STN rules.

The plugin watches the northbound configuration of network interfaces,
which is modelled by [interfaces proto file](../model/interfaces/interfaces.proto)
and stored in ETCD under the following key:

```
/vnf-agent/<agent-label>/vpp/config/v1/interface/<interface-name>
```

This northbound configuration is translated to a sequence of binary
API calls (using GOVPP library). Replies from the VPP are processed
sequentially, i.e. one by one.

Internally in VPP, each newly created interface is assigned a unique
integer for identification and future references. This integer is
denoted as `sw_if_index`, and the agent will learn it from a VPP
response to a successfully created interface (of any kind). The
agent, however, needs to decouple the control plane from `sw_if_index`
to be able to configure multiple inter-dependent objects in one
transaction. For example, multiple interfaces may all be created
in one transaction, together with objects that depend on them, such
as L2 FIB entries, L3 routing, ACLs, etc. It is, however, not possible
to describe the dependencies without knowing the identifiers of
interfaces in advance. Furthermore, certain interface parameters
cannot be modified once the interface was created. In order to reflect
a configuration change,  it may be necessary to re-create the interface
in VPP with the new configuration. The new instance of the interface,
however, may be assigned a different `sw_if_index`. All pre-existing
references to this interface that would be based on `sw_if_index` are
thus invalidated by this operation.

In order to address the limitations of VPP `sw_if_index`, the control
plane defines a unique logical name for each network interface and
uses it as a reference from dependent objects. The agent receives a
logical name from a northbound configuration and calls the specific
binary API (e.g. "Create NIC") to obtain `sw_if_idx`. The agent then
maintains a one-to-one mapping between the logical name and its
respective `sw_if_index` in a registry called `NameIdx`. Later,
if/when the interface configuration changes, the new `sw_if_idx`
can be looked up by its logical name and used in an up-to-date
reference.

The following sequence diagrams describe the high-level behavior
of the `ifplugin`.

*Create one MEMIF (one part of the link)*
```
... -> ifpluign : Create ietf-interface (MEMIF)
ifplugin -> GOVPP : Create MEMIF
ifplugin <-- GOVPP : sw_if_index + success/err
ifplugin -> NameIdx : register sw_if_index by name
ifplugin <-- NameIdx : success/err
ifplugin -> GOVPP : IF admin up
ifplugin <-- GOVPP : success/err
ifplugin -> GOVPP : ADD IP address
ifplugin <-- GOVPP : success/err
```

*Update MEMIF IP addresses*
```
... -> ifplugin : Update ietf-interface (MEMIF, IP addresses)
ifplugin -> NameIdx : lookup sw_if_index by name
ifplugin <-- NameIdx : sw_if_index / not found
ifplugin -> Calculate the delta (what IP address was added or deleted)
ifplugin -> GOVPP : (un)assign IP address(es) to the MEMIF with specific sw_if_idx
ifplugin <-- GOVPP : success/err
ifplugin -> GOVPP : VRF
ifplugin <-- GOVPP : success/err
```

*Delete one MEMIF interface*
```
... -> ifplugin : Remove ietf-interfaces (MEMIF)
ifplugin -> NameIdx : lookup sw_if_index by name
ifplugin <-- NameIdx : sw_if_index / not found
ifplugin -> Calculate the delta (what IP address needs to be deleted)
ifplugin -> GOVPP : delete MEMIF with the specific sw_if_idx
ifplugin <-- GOVPP : success/err
ifplugin -> GOVPP : VRF
ifplugin <-- GOVPP : success/err
```

**JSON configuration example with vpp-agent-ctl**

An example of interface configuration for MEMIF in JSON format can
be found [here](../../../cmd/vpp-agent-ctl/json/memif.json).

To insert config into etcd in JSON format [vpp-agent-ctl](../../../cmd/vpp-agent-ctl)
can be used. For example, to configure interface `memif1` in vpp
labeled `vpp1`, use the configuration in the `memif.json` file and
run the following `vpp-agent-ctl` command:
```
vpp-agent-ctl -put "/vnf-agent/vpp1/vpp/config/v1/interface/memif1" memif.json
```

**Inbuilt configuration example with vpp-agent-ctl**

The `vpp-agent-ctl` binary also ships with some simple predefined
ietf-interface configurations. This is intended solely for testing
purposes.

To create a `master` memif with IP address `192.168.42.1`, run:
```
vpp-agent-ctl -memif
```

It is not possible to change the operating mode of memif
interface once it was created, the agent must first remove the
existing interface and then create a new instance of memif in
`slave` mode.

To remove the interface, run:
```
vpp-agent-ctl -memifd
```

Similarly, `vpp-agent-ctl` offers commands to create, change and delete
VXLANs, tap and loopback interfaces with predefined configurations.
Run `vpp-agent-ctl` with no arguments to get the list of all available
commands. The documentation for `vpp-agent-ctl` is incomplete right now,
and the only way to find out what a given command does is to
[study the source code itself](../../../cmd/vpp-agent-ctl).

### Bidirectional Forwarding Detection

`iflplugin` is also able to configure BFD sessions, authentication keys
and echo function.

BFD is modelled by [bfd proto file](../model/bfd/bfd.proto). Every part of BFD
is stored in ETCD under unique. Every BFD session is stored under following
key:

```
/vnf-agent/{agent-label}/vpp/config/v1/bfd/session/{session-name}
```

Every created authentication key, which can be used in sessions is stored under:

```
/vnf-agent/{agent-label}/vpp/config/v1/bfd/auth-key/{key-name}
```

If echo function is configured, it can be found under key:

```
/vnf-agent/{agent-label}/vpp/config/v1/bfd/echo-function
```

Each newly created BFD element is assigned an integer for identification (the same
concept as with interfaces). There are several mappings used for every BFD configuration
part. `bfd_session_index` is used for BFD sessions, `bfd_keys_index` for authentication
keys and echo function index is stored in `bfd_echo_function_index`.

**Configuration example with vpp-agent-ctl using JSON**

// todo

**Inbuilt configuration example with vpp-agent-ctl**

Use predefined `vpp-agent-ctl` configurations:

*Create BFD session*
```
vpp-agent-ctl -bfds
```

**Note:** BFD session requires interface over which session will be created. This interface
has to contain IP address defined also as BFD session source address. Authentication is assigned 
only if particular key (defined in BFD session) already exists
 
*Create BFD authentication key*
```
vpp-agent-ctl -bfdk
```

*Set up Echo Function*
```
vpp-agent-ctl -bfde
```
 
To remove any part of BFD configuration, just add `d` before vpp-agent-ctl suffix (for example
`-dbfds` to remove BFD session). Keep in mind that authentication key cannot be removed (or modified)
if it is used in any BFD session.

### Network address translation

NAT configuration can be set up on the VPP using `ifplugin`.

NAT is modelled by [nat proto file](../model/nat/nat.proto). Model is divided to two parts; the 
general configuration with defined interfaces and enabled IP address pools, and DNAT configuration 
with a set of static and/or identity mappings. 

NAT global configuration is stored under single key. There is no unique name or label to distinguish different
configurations (only one global setting can be stored in the ETCD at a time): 
```
/vnf-agent/{agent-lanbel}/vpp/config/v1/nat/global/
```

NAT DNAT case has the following key:
```
/vnf-agent/vpp1/vpp/config/v1/nat/dnat/{label}
```

**JSON configuration example with vpp-agent-ctl**

To inset NAT global config into ETCD in JSON format, use [vpp-agent-ctl](../../../cmd/vpp-agent-ctl)
with [nat-global.json](../../../cmd/vpp-agent-ctl/json/nat-global.json) file. 
Use the following command:
```
vpp-agent-ctl -put "/vnf-agent/vpp1/vpp/config/v1/nat/global/" json/nat-global.json
```

To put DNAT configuration, use [vpp-agent-ctl](../../../cmd/vpp-agent-ctl) with 
[nat-dnat.json](../../../cmd/vpp-agent-ctl/json/nat-dnat.json) file.
Use the following command:
```
vpp-agent-ctl -put "/vnf-agent/vpp1/vpp/config/v1/nat/dnat/dnat1" json/nat-dnat.json
```

**Inbuilt configuration example with vpp-agent-ctl**

The `vpp-agent-ctl` binary also ships with some simple predefined
ietf-interface configurations. This is intended solely for testing
purposes.

To create a global NAT config, run:
```
vpp-agent-ctl -gnat
```

To create a DNAT config, run:
```
vpp-agent-ctl -dnat
```

### STN Rules

`iflplugin` is also able to configure STN rules.

STN is modelled by [stn proto file](../model/stn/stn.proto). Every part of STN
is stored in ETCD under unique. Every STN rule is store under following key:
```
/vnf-agent/{agent-lanbel}/vpp/config/v1/stn/rules/{rule-name}
```

**JSON configuration example with vpp-agent-ctl**

An example of interface configuration for STN rule in JSON format can
be found [here](../../../cmd/vpp-agent-ctl/json/stn-rule.json).

To insert config into etcd in JSON format [vpp-agent-ctl](../../../cmd/vpp-agent-ctl)
can be used. For example, to configure stn rule `rule1` in vpp
labeled `vpp1`, use the configuration in the `stn-rule.json` file and
run the following `vpp-agent-ctl` command:
```
vpp-agent-ctl -put "/vnf-agent/vpp1/vpp/config/v1/stn/rules/" stn-rule.json
```

**Inbuilt configuration example with vpp-agent-ctl**

The `vpp-agent-ctl` binary also ships with some simple predefined
ietf-interface configurations. This is intended solely for testing
purposes.

To create a `rule1` stn rule with IP address `10.1.1.3/32`, run:
```
vpp-agent-ctl -stn
```

To remove the stn rule, run:
```
vpp-agent-ctl -stnd
```

## State of implementation of rx-mode for various interface types

| interface type | rx-modes | implemented | how to check on VPP | example of creation of interface |
| ---- | ---- | ---- | ---- | ---- |
| tap interface | PIA | yes  | ? | _#tap connect tap1_|
| memory interface |  PIA | yes | both sides of memif (slave and master) has to be configured = 2 VPPs.</br>_#sh memif_ | _#create memif master_ |
| vxlan tunnel | PIA | yes | ? | #_create vxlan tunnel src 192.168.168.168 dst 192.168.168.170 vni 40_
| software loopback | PIA | yes | ? | _#create loopback interface_
| ethernet csmad | P | yes | _#show interface rx-placement_ | vpp will adopt interfaces on start up
| af packet | PIA | yes | _#show interface rx-placement_ | _#create host-interface name <ifname>_

Legend:

- P - polling
- I - interrupt
- A - adaptive
