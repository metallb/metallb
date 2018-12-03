# SR plugin

The `srplugin` is a Core Agent Plugin designed to configure Segment routing for IPv6 (SRv6) in the VPP.
Configuration managed by this plugin is modelled by [srv6 proto file](../model/srv6/srv6.proto).

All configuration must be stored in ETCD using the srv6 key prefix:

```
/vnf-agent/<agent-label>/vpp/config/v1/srv6
```

## Configuring Local SIDs
The local SID can be configured using this key:
```
/vnf-agent/<agent-label>/vpp/config/v1/srv6/localsid/<SID>
```
where ```<SID>``` (Segment ID) is a unique ID of local sid and it must be an valid IPv6 address. The SID is excluded from 
the json configuration for this key because it is already present as part of the key. 

## Configuring Policy
The segment routing policy can be configured using this key:
```
/vnf-agent/<agent-label>/vpp/config/v1/srv6/policy/<bsid>
```
where ```<bsid>``` is  unique binding SID of the policy. As any other SRv6 SID it must be an valid IPv6 address. Also 
the binding SID is excluded from the json configuration because it is already part of the key.\
The policy can have multiple segments (each segment defines one segment routing path and each segment has its own weight). 
It can be configured using this key:
```
/vnf-agent/<agent-label>/vpp/config/v1/srv6/policy/<bsid>/segment/<name> 
```
where ```<bsid>``` is the binding SID of policy to which segment belongs and ```name``` is a unique string name of the segment.

The VPP implementation doesn't allow to have empty segment routing policy (policy must have always at least one segment). 
Therefore adding the policy configuration without at least one segment won't write into the VPP anything. The configuration 
of VPP is postponed until the first policy segment is configured. Similar rules apply for the policy/policy segment removal. 
When the last policy segment is removed, nothing happens. Only after the removal of policy is everything correctly removed. 
It is also possible to remove only the policy and the VPP will be configured to remove the policy with all its segments. 


## Configuring Steering
The steering (the VPP's policy for steering traffic into SR policy) can be configured using this key:
```
/vnf-agent/<agent-label>/vpp/config/v1/srv6/steering/<name>
```
where ```<name>``` is a unique name of steering.

