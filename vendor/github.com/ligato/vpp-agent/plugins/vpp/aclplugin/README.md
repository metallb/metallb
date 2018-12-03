# ACL plugin

The `aclplugin` is a Core Agent Plugin designed to configure ACL in the VPP.
Configuration managed by this plugin is modelled by [acl proto file](../model/acl/acl.proto).

The configuration must be stored in ETCD using following keys:

```
/vnf-agent/<agent-label>/vpp/config/v1/acl/<acl-name>
```

**JSON configuration example with vpp-agent-ctl**

An example of basic ACL configuration in JSON format can be found with rules for
[MACIP](../../../cmd/vpp-agent-ctl/json/acl-macip.json), [TCP](../../../cmd/vpp-agent-ctl/json/acl-tcp.json), [UDP](../../../cmd/vpp-agent-ctl/json/l2_fib.json)

**Built-in configuration example with vpp-agent-ctl**

The `vpp-agent-ctl` binary also ships with some simple predefined acl configurations.
It is meant to be used solely for testing purposes.

To configure a new acl `acl1`, use:
```
vpp-agent-ctl /opt/vpp-agent/dev/etcd.conf -acl
```

To delete the acl, use:
```
vpp-agent-ctl /opt/vpp-agent/dev/etcd.conf -acld
```
