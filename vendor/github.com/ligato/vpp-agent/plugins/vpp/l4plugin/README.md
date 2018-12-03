# L4 plugin

The l4plugin is a Core Agent Plugin that is designed to configure VPP application namespaces.
Configuration managed by this plugin is modelled by the [proto file](../model/l4/l4.proto). The configuration
must be stored in etcd using the following key:

```
/vnf-agent/<agent-label>/vpp/config/v1/l4/namespaces/<namespaceID>

/vnf-agent/<agent-label>/vpp/config/v1/l4/features/feature
```


An example of configuration in json format can be found [here](../../../cmd/vpp-agent-ctl/json/app-ns.json).

Note: the l4 features need to be enabled on the VPP. To do so, use second json configuration file
which can be found here [here](../../../cmd/vpp-agent-ctl/json/enable-l4.json) 

To insert config into etcd in json format [vpp-agent-ctl](../../../cmd/vpp-agent-ctl) 
can be used. We assume that we want to configure vpp with label `vpp1` and config is stored 
in the `app-ns.json` file. At first, enable L4 features, then configure the application namespace.
```
vpp-agent-ctl -put "/vnf-agent/vpp1/vpp/config/v1/l4/features/feature" json/enable-l4.json
vpp-agent-ctl -put "/vnf-agent/vpp1/vpp/config/v1/l4/namespaces/ns1" json/app-ns.json
```

The vpp-agent-ctl contains a simple predefined application namespace config also. It can be used 
for testing purposes. To enable l4 features, run:
```
vpp-agent-ctl -el4
```

To disable it:
```
vpp-agent-ctl -dl4
```

To configure application namespace:
```
vpp-agent-ctl -appns
```

Note: application namespaces cannot be removed. Currently it is not supported by the VPP.
