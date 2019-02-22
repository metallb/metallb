# Punt plugin

**Written for: v1.7-vpp18.10**

The punt plugin provides several options for how to configure the VPP to allow a specific IP traffic to be punted to the host TCP/IP stack. The plugin supports **punt to the host** (either directly, or **via Unix domain socket**) and registration of **IP punt redirect** rules.

- [Punt to host stack](#pths)
  * [Model](#pths-model)
  * [Requirements](#pths-req)
  * [Configuration](#pths-config)
  * [Limitations](#pths-limit)
  * [Known issues](#pths-issues)
- [IP redirect](#ipr)
  * [Model](#ipr-model)
  * [Configuration](#ipr-config)
  * [Limitations](#ipr-limit)
  * [Known issues](#ipr-issues)

## <a name="pths">Punt to host stack</a>

All the incoming traffic matching one of the VPP interface addresses, and also matching defined L3 protocol, L4 protocol, and port - and would be otherwise dropped - will be instead punted to the host. If a Unix domain socket path is defined (optional), the traffic will be punted via socket. All the fields which serve as a traffic filter are mandatory.

### <a name="pths-model">Model</a>

The punt plugin defines the following [model](../model/punt/punt.proto) which grants support for two main configuration items defined by different northbound keys.

The punt to host is defined as `ToHost` object in the generated proto model. 

The key has a following format (for both, with or without socket):
```
vpp/config/v2/punt/tohost/l3/<L3_protocol>/l4/<L4_protocol>/port/<port_number>
```

L3/L4 protocol in the key is defined as a `string` value, however, the value is transformed to numeric representation in the VPP binary API.

The usage of L3 protocol `ALL` is exclusive for IP punt to host (without socket registration) in the VPP API. If used for the IP punt with socket registration, the vpp-agent calls the binary API twice with the same parameters for both, IPv4 and IPv6.

### <a name="pths-req">Requirements</a>

**Important note:** in order to configure a punt to host via Unix domain socket, a specific VPP startup-config is required. The attempt to set punt without it results in errors in VPP. Startup-config:

```
punt {
  socket /tmp/socket/punt
}
```

The path has to match with the one in the northbound data. 

### <a name="pths-config">Configuration</a>

How to configure punt to host:

**1. Using Key-value database:** put proto-modeled data to the database under the correct key for punt to host configuration. If the database is the ETCD, it is possible to use [vpp-agent-ctl](../../../cmd/vpp-agent-ctlv2) utility. Let's assume that we want to configure a punt to host with socket registration (with default agent label `vpp1`):

```
vpp-agent-ctl -put "/vnf-agent/vpp1/vpp/config/v2/punt/tohost/l3/IPv4/l4/UDP/port/9000" json/punt-to-host.json
```  

The more simple way is to use the configuration example directly in the `vpp-agent-ctl`. The example contains pre-defined data which can be put to the ETCD with a single command. Commands are different for a punt to host alone, or registered via socket:

```
vpp-agent-ctl -puntr        // simple punt to host
vpp-agent-ctl -rsocket      // register punt to host via socket
```

To remove/deregister data, use:

```
vpp-agent-ctl -puntd
vpp-agent-ctl -dsocket
```

**2. Using REST** not yet implemented

**3. Using GRPC:** not yet implemented

### <a name="pths-limit">Limitations</a>

Current limitations for a punt to host:
* The UDP configuration cannot be shown (or even configured) in the VPP cli.
* The VPP does not provide API to dump configuration, which takes the vpp-agent the opportunity to read existing entries and may case issues with resync.
* Although the vpp-agent supports the TCP protocol as the L4 protocol to filter incoming traffic, the current VPP version don't.
* Configured punt to host entry cannot be removed since the VPP does not support this option. The attempt to do so exits with an error.

Current limitations for a punt to host via unix domain socket:
* The configuration cannot be shown (or even configured) in the VPP cli.
* The vpp-agent cannot read registered entries since the VPP does not provide an API to dump.
* The VPP startup config punt section requires unix domain socket path defined. The limitation is that only one path can be defined at a time.

### <a name="pths-issues">Known issues</a>

* VPP issue: if the Unix domain socket path is defined in the startup config, the path has to exist, otherwise the VPP fails to start. The file itself can be created by the VPP.

## <a name="ipr">IP redirect</a>

Defined as the IP punt, IP redirect allows a traffic matching given IP protocol to be punted to the defined TX interface and next hop IP address. All those fields have to be defined in the northbound proto-modeled data. Optionally, the RX interface can be also defined as an input filter.  

### <a name="ipr-model">Model</a>

IP redirect is defined as `IpRedirect` object in the generated proto model.

The key has a following format:
```
vpp/config/v2/punt/ipredirect/l3/<L3_protocol>/tx/<tx_interface_name>
```

L3 protocol is defined as `string` value (transformed to numeric in VPP API call). The table is the same as before.

If L3 protocol is set to `ALL`, the respective API is called for IPv4 and IPv6 separately.

### <a name="ipr-config">Configuration</a>

How to configure IP redirect:

**1. Using the key-value database:** put proto-modeled data to the database under the correct key for IP redirect configuration. If the database is the ETCD, it is possible to use [vpp-agent-ctl](../../../cmd/vpp-agent-ctlv2) utility. Let's assume that we want to configure following IP redirect (with default agent label `vpp1`). Since the configuration counts with an existing interface (tap1 in this case), the interface has to be configured as well (the order does not matter).
                                
```
vpp-agent-ctl -put "/vnf-agent/vpp1/vpp/config/v2/interface/tap1" json/tap.json
vpp-agent-ctl -put "/vnf-agent/vpp1/vpp/config/v2/punt/ipredirect/l3/IPv4/tx/tap1" json/ip-redirect.json
```  
                                
The simple way is to use an configuration example directly in the `vpp-agent-ctl`. The example contains pre-defined data which can be put to ETCD with a single command. Let's configure interface and IP redirect:
                                
```
vpp-agent-ctl -tap
vpp-agent-ctl -ipredir
```

To remove IP redirect entry, use:

```
vpp-agent-ctl -ipredird
```

In case the interface is not configured, the IP redirect data are cached and marked as pending, awaiting the interface to appear in order to be re-tried.

**2. Using REST** not yet implemented

**3. Using GRPC:** not yet implemented

The VPP cli command (for configuration verification) is `show ip punt redirect `.

### <a name="ipr-limit">Limitations</a>

* The VPP does not provide API calls to dump existing IP redirect entries. It may cause resync not to work properly.

### <a name="ipr-issues">Known issues</a> 

* No issues are known with the current VPP version.