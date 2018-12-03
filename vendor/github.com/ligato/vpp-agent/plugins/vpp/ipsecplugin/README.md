# IPsec plugin

The **ipsecplugin** is a Core Agent Plugin that is designed to configure
IPsec for VPP. Configuration managed by this plugin is modelled
by the [proto file](../model/ipsec/ipsec.proto).

The configuration must be stored in etcd using the following keys:

```sh
# Security Policy Database (SPD)
/vnf-agent/<agent-label>/vpp/config/v1/ipsec/spd/<spdName>
# Security Association
/vnf-agent/<agent-label>/vpp/config/v1/ipsec/sa/<saName>
```


An example of configuration in json format can be found here:
[SPD](../../../cmd/vpp-agent-ctl/json/ipsec-spd.json) and
[SA](../../../cmd/vpp-agent-ctl/json/ipsec-sa10.json).

To insert config into etcd in json format [vpp-agent-ctl](../../../cmd/vpp-agent-ctl) 
can be used. We assume that we want to configure vpp with label `vpp1`,
config for SPD is stored in the `ipsec-spd.json` file and
config for SAs is stored in the `ipsec-sa10.json` and `ipsec-sa20.json` file.

```sh
vpp-agent-ctl -put /vnf-agent/vpp1/vpp/config/v1/ipsec/sa/sa10 ipsec-sa10.json
vpp-agent-ctl -put /vnf-agent/vpp1/vpp/config/v1/ipsec/sa/sa20 ipsec-sa20.json
vpp-agent-ctl -put /vnf-agent/vpp1/vpp/config/v1/ipsec/spd/spd1 ipsec-spd.json
```

To enable IPsec in Linux as well you need to have package **ipsec-tools** installed.
Then you need to edit `/etc/ipsec-tools.conf` and add following configuration:

```sh
# Flush the SAD and SPD
flush;
spdflush;

# ESP Security associations
add 10.0.0.1 10.0.0.2 esp 0x000003e8 -E rijndael-cbc
        0x4a506a794f574265564551694d653768
        -A hmac-sha1 0x4339314b55523947594d6d3547666b45764e6a58;
add 10.0.0.2 10.0.0.1 esp 0x000003e9 -E rijndael-cbc
        0x4a506a794f574265564551694d653768
        -A hmac-sha1 0x4339314b55523947594d6d3547666b45764e6a58;

# Security policies
spdadd 10.0.0.1 10.0.0.2 any -P out ipsec
           esp/transport//require;

spdadd 10.0.0.2 10.0.0.1 any -P in ipsec
           esp/transport//require;
```

After saving the configuration file run `/etc/init.d/setkey start` to activate it.

You can find more information here: https://wiki.fd.io/view/VPP/IPSec_and_IKEv2#Ubuntu_configuration
