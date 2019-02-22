# IPSec plugin

**Written for: v1.7-vpp18.10**

The IPSec plugin allows to configure **security policy databases** and **security associations** to the VPP, and also handles relations between the SPD and SA or between SPD and an assigned interface. Note that the IPSec tunnel interfaces are not a part of IPSec plugin (their configuration is handled in [VPP interface plugin](../ifplugin)).

- [Security policy database](#spd)
  * [Model](#spd-model)
  * [Configuration](#spd-config)
  * [Limitations](#spd-limit)
  * [Known issues](#spd-issues)
- [Security association](#sa)
  * [Model](#sa-model)
  * [Configuration](#sa-config)
  * [Limitations](#sa-limit)
  * [Known issues](#sa-issues)
  
## <a name="spd">Security policy database</a>

Security policy database (SPD) specifies policies that determine the disposition of all the inbound or outbound IP traffic from either the host or the security gateway. The SPD is bound to an SPD interface and contains a list of policy entries (security table). Every policy entry points to VPP security association.

### <a name="spd-model">Model</a>

Security policy database is defined in the common IPSec [model](../model/ipsec/ipsec.proto). The generated object is defined as `SecurityPolicyDatabase`. 

Key format:
```
vpp/config/v2/ipsec/spd/<index>
```

The SPD defines its own unique index within VPP. The user has an option to set its own index (it is not generated in the VPP, as for example for access lists) in `uint32` range. The index is a mandatory field in the model because it serves as a unique identifier also in the vpp-agent and is a part of the SPD key as well. The attention has to be paid defining an index in the model. Despite the field is a `string` format, it only accepts plain numbers. Attempt to set any non-numeric characters ends up with an error.     

Every policy entry has field `sa_index`. This is the security association reference in the security policy database. The field is mandatory, missing value causes an error during configuration.

The SPD defines two bindings: the security association (for every entry) and the interface. The interface binding is important since VPP needs to create an empty SPD first, which cannot be done without it. All the policy entries are configured after that, where the SA is expected to exist.
Since every security policy database entry is configured independently, vpp-agent can configure IPSec SPD only partially (depending on available binding).

All the binding can be resolved by the vpp-agent. 

### <a name="spd-config">Configuration</a>

How to configure the security policy database:

**1. Using Key-value database:** put proto-modeled data with the correct key for SPD. If the used database is the ETCD, it is possible to use [vpp-agent-ctl](../../../cmd/vpp-agent-ctlv2) utility. Let's try to configure an SPD with index 1, interface `tap1` and two security policy database entries depending on security associations with indexes 10 and 20. The agent label is defaulted to `vpp1`:

```
// configure tap1 interface
vpp-agent-ctl -put "/vnf-agent/vpp1/vpp/config/v2/interface/tap1" json/tap.json

// configure two SA with indexes 10 and 20
vpp-agent-ctl -put "/vnf-agent/vpp1/vpp/config/v2/ipsec/sa/10" json/ipsec-sa10.json
vpp-agent-ctl -put "/vnf-agent/vpp1/vpp/config/v2/ipsec/sa/20" json/ipsec-sa20.json

// configure IPSec SPD
vpp-agent-ctl -put "/vnf-agent/vpp1/vpp/config/v2/ipsec/spd/1" json/ipsec-spd.json
```  

The order is not important here, vpp-agent can cache config waiting on binding and configure when available.

Another way is to use the configuration example in the `vpp-agent-ctl` utility. The example contains pre-defined SPD data which can be put to the ETCD, together with bindings:

```
vpp-agent-ctl -tap          // interface
vpp-agent-ctl -sa           // puts both security associations
vpp-agent-ctl -spd          
```

To remove data, use:

```
vpp-agent-ctl -spdd
vpp-agent-ctl -sad
vpp-agent-ctl -tapd
```

The order of putting/removal does not matter here as well.

**2. Using REST** not yet implemented

**3. Using GRPC:** not yet implemented

The VPP cli has a command to show the SPD IPSec configuration: `sh ipsec`

### <a name="spd-limit">Limitations</a>

* There are currently no limitations in configuring SPD.

### <a name="spd-issues">Known issues</a>

* No issues are known with the used VPP version.

## <a name="sa">Security associations</a>

The VPP security association (SA) is a set of IPSec specifications negotiated between devices establishing and IPSec relationship. The SA includes preferences for the authentication type, IPSec protocol or encryption used when the IPSec connection is established.

### <a name="sa-model">Model</a>

Security association is defined in the common IPSec [model](../model/ipsec/ipsec.proto). The generated object is defined as `SecurityAssociation`. 

Key format:
```
vpp/config/v2/ipsec/sa/<index>
```

The SA uses the same indexing system as SPD. The index is a user-defined unique identifier of the security association in `uint32` range. Similarly to SPD, the SA index is also defined as `string` type field in the model but can be set only to numerical values. Attempt to set other values causes processing errors.

The SA has no dependencies on other configuration types.

### <a name="sa-config">Configuration</a>

How to configure the security association:

**1. Using Key-value database:** use proto-modeled data with the correct security association key. If the ETCD database is used, the example configuration put via [vpp-agent-ctl](../../../cmd/vpp-agent-ctlv2) utility is also available. Use it to configure an SA with index 10, and agent label set to `vpp1`:

```
vpp-agent-ctl -put "/vnf-agent/vpp1/vpp/config/v2/ipsec/sa/10" json/ipsec-sa10.json
```  

Another way how to configure SA is to use the pre-defined configuration in the `vpp-agent-ctl` utility. The example configures two security associations with indexes 10 and 20:

```
vpp-agent-ctl -sa           
```

To remove the SA entries, use:

```
vpp-agent-ctl -sad
```

**2. Using REST** not yet implemented

**3. Using GRPC:** not yet implemented

Show the IPSec configuration with the VPP cli command: `sh ipsec`

### <a name="sa-limit">Limitations</a>

* No limitations are known in configuring SA.

### <a name="sa-issues">Known issues</a>

* No issues are known with the referenced VPP version.