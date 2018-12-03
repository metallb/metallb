# Linux Plugin

The `linuxplugin` is a core Agent Plugin for the management of a subset of the Linux
network configuration. Configuration of VETH (virtual ethernet pair) interfaces, linux routes and ARP entries
is currently supported. Detailed description can be found in particular READMEs:
 - [ifplugin](ifplugin)
 - [l3plugin](l3plugin)
 - [nsplugin](nsplugin)
 
In general, the northbound configuration is translated to a sequence of Netlink API
calls (using `github.com/vishvananda/netlink` and `github.com/vishvananda/netns` libraries).