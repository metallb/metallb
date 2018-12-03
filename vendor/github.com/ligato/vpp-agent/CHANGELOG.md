# Release v1.7-vpp18.10 (2018-10-2)

## Compatibility
- VPP 18.10-rc0~505-ge23edac
- cn-infra v1.6
- GO 1.11

## New Features
- [vpp-ifplugin](plugins/vpp/ifplugin)
  * It is now possible to dump unnumbered interface data
  * Rx-placement now uses specific binary API to configure instead of generic CLI API
- [vpp-l2plugin](plugins/vpp/l2plugin)
  * Bridge domain ARP termination table can now be dumped  
- [vpp-ifplugin](plugins/linux/ifplugin)
  * Linux interface watcher was reintroduced.
  * Linux interfaces can be now dumped.
- [vpp-l3plugin](plugins/linux/l3plugin)
  * Linux ARP entries and routes can be dumped.   

## Improvements
- [vpp-plugins](plugins/vpp)
  * Improved error propagation in all the VPP plugins. Majority of errors now print the stack trace to
  the log output allowing better error tracing and debugging.
  * Stopwatch was removed from all vppcalls
- [linux-plugins](plugins/linux)
  * Improved error propagation in all Linux plugins (same way as for VPP)
  * Stopwatch was removed from all linuxcalls
- [govpp-plugn](plugins/govppmux)
  * Tracer (introduced in cn-infra 1.6) added to VPP message processing, replacing stopwatch. 
  The measurement should be more precise and logged for all binary API calls. Also the rest
  plugin now allows showing traced entries.
  
## Docker image
  * The image can now be built on ARM64 platform  
  
## Bugfix
  * Corrected several cases where various errors were silently ignored
  * GRPC registration is now done in Init() phase, ensuring that it finishes before GRPC server
    is started
  * Removed occasional cases where Linux tap interface was not configured correctly
  * Fixed FIB configuration failures caused by wrong updating of the metadata after several 
    modifications
  * No additional characters are added to NAT tag and can be now configured with the full length 
    without index out of range errors
  * Linux interface resync registers all VETH-type interfaces, despite the peer is not known
  * Status publishing to ETCD/Consul now should work properly
  * Fixed occasional failure caused by concurrent map access inside Linux plugin interface 
    configurator
  * VPP route dump now correctly recognizes route type  
    
# Release v1.6-vpp18.10 (2018-08-24)

## Compatibility
- VPP 18.10-rc0~169-gb11f903a
- cn-infra v1.5

## Major Topics

**New Flavors**

  Cn-infra 1.5 brought new and better flavors and it would be a shame not to implement it
  in the vpp-agent. The old flavors package was removed and replaced with this new concept,
  visible in [app package vpp-agent](app/vpp_agent.go). All examples were also updated.
  
## Breaking Changes
- Flavors were replaced with [new way](app) of managing plugins.
- REST interface URLs were changed, see [readme](plugins/rest/README.md) for complete list.

## New Features
- [rest plugin](plugins/rest)
  * All VPP configuration types are now supported to be dumped using REST. The output consists of two parts;
    data formatted as NB proto model, and metadata with VPP specific configuration (interface indexes,
    different counters, etc.).
  * REST prefix was changed. The new URL now contains API version and purpose (dump, put). The list of all 
    URLs can be found in the [readme](plugins/rest/README.md)
- [ifplugin](plugins/vpp/ifplugin)
  * Added support for NAT virtual reassembly for both, IPv4 and IPv6. See change in 
    [nat proto file](plugins/vpp/model/nat/nat.proto)
- [l3plugin](plugins/vpp/l3plugin)
  * Vpp-agent now knows about DROP-type routes. They can be configured and also dumped. VPP default routes, which are
    DROP-type is recognized and registered. Currently, resync does not remove or correlate such a route type
    automatically, so no default routes are unintentionally removed.
  * New configurator for L3 IP scan neighbor was added, allowing to set/unset IP scan neigh parameters to the VPP.
  
## Docker Images
- using Ubuntu 18.04 as the base image
  
## Improvements
- [vpp plugins](plugins/vpp)
  * all vppcalls were unified under API defined for every configuration type (e.g. interfaces, l2, l3, ...).
  Configurators now use special handler object to access vppcalls. This should prevent duplicates and make
  vppcalls cleaner and more understandable.
- [ifplugin](plugins/vpp/ifplugin)
  * VPP interface DHCP configuration can now be dumped and added to resync processing
  * Interfaces and also L3 routes can be configured for non-zero VRF table if IPv6 is used. 
- [examples](examples)
  * All examples were reworked to use new flavors concept. The purpose was not changed.   

## Bugfix
* if VPP routes are dumped, all paths are returned
* NAT load-balanced static mappings should be resynced correctly  
* telemetry plugin now correctly parses parentheses for `show node counters`
* telemetry plugin will not hide an error caused by value loading if the config file is not present
* Linux plugin namespace handler now correctly handles namespace switching for interfaces with IPv6 addresses.
  Default IPv6 address (link local) will not be moved to the new namespace if there are no more IPv6 addresses
  configured within the interface. This should prevent failures in some cases where IPv6 is not enabled in the
  destination namespace.
* VxLAN with non-zero VRF can be successfully removed
* Lint is now working again    
* VPP route resync works correctly if next hop IP address is not defined

# Release v1.5.2 (2018-07-23)

## Compatibility
- VPP 18.07-rc0~358-ga5ee900
- cn-infra v1.4.1 (minor version fixes bug in Consul)

## Bugfix
- [Telemetry](plugins/telemetry)
  * Fixed bug where lack of config file could cause continuous polling. The interval now also
  cannot be changed to a value less than 5 seconds.
  * Telemetry plugin is now closed properly

# Release v1.5.1 (2018-07-20)

## Compatibility
- VPP 18.07-rc0~358-ga5ee900
- cn-infra v1.4

## New Features
- [Telemetry](plugins/telemetry)
  * Default polling interval was raised to 30s.
  * Added option to use telemetry config file to change polling interval, or turn the polling off,
  disabling the telemetry plugin. The change was added due to several reports where often polling
  is suspicious of interrupting VPP worker threads and causing packet drops and/or other
  negative impacts. More information how to use the config file can be found 
  in the [readme](plugins/telemetry/README.md)

# Release v1.5 (2018-07-16)

## Compatibility
- VPP 18.07-rc0~358-ga5ee900
- cn-infra v1.4

## Breaking Changes
- The package `etcdv3` was renamed to `etcd`, along with its flag and configuration file.
- The package `defaultplugins` was renamed to `vpp` to make the purpose of the package clear

## New Features
- [LinuxPlugin](plugins/linux)
  * Is now optional and can be disabled via configuration file.
- [ifplugin](plugins/vpp/ifplugin)
  * Added support for VxLAN multicast
  * Rx-placement can be configured on VPP interfaces
- [IPsec](plugins/vpp/ipsecplugin)
  * IPsec UDP encapsulation can now be set (NAT traversal)  
    

## Bugfix
- Fixed a few issues with parsing VPP metrics from CLI for [Telemetry](plugins/telemetry).
- Fixed bug in GoVPP occurring after some request timed out, causing
  the channel to receive replies from the previous request and always returning an error.
- Fixed issue which prevented setting interface to non-existing VRF.
- Fixed bug where removal of an af-packet interface caused attached Veth to go DOWN.
- Fixed NAT44 address pool resolution which was not correct in some cases.
- Fixed bug with adding SR policies causing incomplete configuration.

## Docker Images
- Replace `START_AGENT` with `OMIT_AGENT` to match `RETAIN_SUPERVISOR`
  and keep both unset by default.
- Refactored and cleaned up execute scripts and remove unused scripts.
- Fixed some issues with `RETAIN_SUPERVISOR` option.
- Location of supervisord pid file is now explicitly set to
  `/run/supervisord.pid` in *supervisord.conf* file.
- The vpp-agent is now started  with single flag `--config-dir=/opt/vpp-agent/dev`,
  and will automatically load all configuration from that directory.

# Release v1.4.1 (2018-06-11)

## Compatibility
- VPP v18.04 (2302d0d)
- cn-infra v1.3

A minor release using newer VPP v18.04 version.

## Bugfix
- VPP submodule was removed from the project. It should prevent various problems with dependency
  resolution.
- Fixed known bug present in the previous version of the VPP, issued as
  [VPP-1280](https://jira.fd.io/browse/VPP-1280). Current version contains appropriate fix.  

# Release v1.4 (2018-05-24)

## Compatibility
- VPP v18.04 (ac2b736)
- cn-infra v1.3

## New Features
- [Consul](https://www.consul.io/)
  * Consul is now supported as a key-value store alternative to ETCD.
    More information in the [readme](https://github.com/ligato/cn-infra/blob/master/db/keyval/consul/README.md).
- [Telemetry](plugins/telemetry)
  * New plugin for collecting telemetry data about VPP metrics
    and serving them via HTTP server for Prometheus.
    More information in the [readme](plugins/telemetry/README.md).
- [Ipsecplugin](plugins/vpp/ipsecplugin)
  * Now supports tunnel interface for encrypting all the data
    passing through that interface.
- [GRPC](plugins/vpp/rpc) 
  * Vpp-agent itself can act as a GRPC server (no need for external executable)
  * All configuration types are supported (incl. Linux interfaces, routes and ARP)
  * Client can read VPP notifications via vpp-agent.
- [SR plugin](plugins/vpp/srplugin)
  * New plugin with support for Segment Routing.
    More information in the [readme](plugins/vpp/srplugin/README.md).

## Improvements
- [ifplugin](plugins/vpp/ifplugin) 
  * Added support for self-twice-NAT
- __vpp-agent-grpc__ executable merged with [vpp-agent](cmd/vpp-agent) command.
- [govppmux](plugins/govppmux) 
  * [configure reply timeout](plugins/govppmux/README.md) can be configured.
  * Support for VPP started with custom shared memory prefix. SHM may be configured via the GoVPP
  plugin config file. More info in the [readme](plugins/govppmux/README.md)

## Examples
- [localclient_linux](examples/localclient_vpp) now contains two examples, the old one demonstrating
  basic plugin functionality was moved to [plugin](examples/localclient_vpp/plugins) package, and 
  specialised example for [NAT](examples/localclient_vpp/nat) was added.
- [localclient_linux](examples/localclient_linux) now contains two examples, the old one demonstrating 
  [veth](examples/localclient_linux/veth) interface usage was moved to package and new example for linux
  [tap](examples/localclient_linux/tap) was added.

## Bugfix
  * Fixed case where the creation of the Linux route with unreachable gateway threw an error. 
    The route is now appropriately cached and created when possible. 
  * Fixed issue with GoVPP channels returning errors after a timeout.
  * Fixed various issues related to caching and resync in L2 cross-connect
  * Split horizon group is now correctly assigned if an interface is created after bridge domain
  * Fixed issue where the creation of FIB while the interface was not a part of
    the bridge domain returned an error.

## Other
  * Overall redundancy cleanup and corrected naming for all proto models.
  * Added more unit tests for increased coverage and code stability. 

## Known issues
  * VPP crash may occur if there is interface with non-default VRF (>0). There is an 
    [VPP-1280](https://jira.fd.io/browse/VPP-1280) issue created with more details 

# Release v1.3 (2018-03-22)

## Compatibility
- VPP v18.01-rc0~605-g954d437
- cn-infra v1.2

The vpp-agent is now using custom VPP branch [stable-1801-contiv](https://github.com/vpp-dev/vpp/tree/stable-1801-contiv).

## New Features
- [ipsecplugin](plugins/vpp/ipsecplugin)
  * New plugin for IPSec added. The IPSec is supported for VPP only
    with Linux set manually for now. IKEv2 is not yet supported.
    More information in the [readme](plugins/vpp/ipsecplugin/README.md).
- [nsplugin](plugins/linux/nsplugin)
  * New namespace plugin added. The configurator handles common namespace
    and microservice processing and communication with other Linux plugins.
- [ifplugin](plugins/vpp/ifplugin)
  * Added support for Network address translation. NAT plugin supports
    a configuration of NAT44 interfaces, address pools and DNAT.
    More information in the [readme](plugins/vpp/ifplugin/README.md).
  * DHCP can now be configured for the interface  
- [l2plugin](plugins/vpp/l2plugin)
  * Split-horizon group can be configured for bridge domain interface.
- [l3plugin](plugins/vpp/l3plugin)
  * Added support for proxy ARP. For more information and configuration 
    example, please see [readme](plugins/vpp/l3plugin/README.md).
- [linux ifplugin](plugins/linux/ifplugin)
  * Support for automatic interface configuration (currently only TAP).
        
## Improvements
- [aclplugin](plugins/vpp/aclplugin)
  * Removed configuration order of interfaces. The access list can be now 
    configured even if interfaces do not exist yet, and add them later.
- [vpp-agent-ctl](cmd/vpp-agent-ctl) 
  * The vpp-agent-ctl was refactored and command info was updated.

## Docker Images
  * VPP can be built and run in the release or debug mode.
  Read more information in the [readme](https://github.com/ligato/vpp-agent/blob/pantheon-dev/docker/dev/README.md).
  * Production image is now smaller by roughly 40% (229MB).

## Bugfix
  * Resync of ifplugin in both, VPP and Linux, was improved. Interfaces
    with the same configuration data are not recreated during resync.
  * STN does not fail if IP address with a mask is provided.
  * Fixed ingress/egress interface resolution in ACL.
  * Linux routes now check network reachability for gateway address b
    before configuration. It should prevent "network unreachable" errors
    during config.
  * Corrected bridge domain crash in case non-bvi interface was added to
    another non-bvi interface.
  * Fixed several bugs related to VETH and AF-PACKET configuration and resync.

# Release v1.2 (2018-02-07)

## Compatibility
- VPP v18.04-rc0~90-gd95c39e
- cn-infra v1.1

### Improvements
- [aclplugin](plugins/vpp/aclplugin) 
  * Improved resync of ACL entries. Every new ACL entry is correctly configured in the VPP and all obsolete entries are read and removed. 
- [ifplugin](plugins/vpp/ifplugin) 
  * Improved resync of interfaces, BFD sessions, authentication keys, echo functions and STN. Better resolution of persistence config for interfaces. 
- [l2plugin](plugins/vpp/l2plugin) 
  * Improved resync of bridge domains, FIB entries, and xConnect pairs. Resync now better correlates configuration present on the VPP with the NB setup.
- (Linux) [ifplugin](plugins/linux/l3plugin) 
  * ARP does not need the interface to be present on the VPP. Configuration is cached and put to the VPP if requirements are fulfilled. 

### Fixes
  * [vpp-agent-grpc](cmd/vpp-agent) (removed in 1.4 release, since then it is a part of the vpp-agent) now compiles properly
    together with other commands.

### Dependencies
  * Migrated from glide to dep

### Docker Images
  * VPP compilation now skips building of Java/C++ APIs,
    this saves build time and final image size.
  * Development image now runs VPP in debug mode with
    various debug options added in [VPP config file](docker/dev/vpp.conf).

## Bugfix
- Fixed interface assignment in ACLs
- Fixed bridge domain BVI modification resolution

## Known Issues
- VPP can occasionally cause a deadlock during checksum calculation (https://jira.fd.io/browse/VPP-1134)
- VPP-Agent might not properly handle initialization across plugins (this is not occurring currently, but needs to be tested more)

# Release v1.1 (2018-01-22)

## Compatibility
- VPP version v18.04-rc0~33-gb59bd65
- cn-infra v1.0.8

### New Features
- [ifplugin](plugins/vpp/ifplugin)
    - added support for un-numbered interfaces. The nterface can be marked as un-numbered with information
    about another interface containing required IP address. A un-numbered interface does not need to have 
    IP address set.
    - added support for virtio-based TAPv2 interfaces.
    - interface status is no longer stored in the ETCD by default and it can be turned on using the appropriate
    setting in vpp-plugin.conf. See  [readme](plugins/vpp/README.md) for more details.  
- [l2plugin](plugins/vpp/l2plugin)
    - bridge domain status is no longer stored in the ETCD by default and it can be turned on using the appropriate
    setting in vpp-plugin.conf. See  [readme](plugins/vpp/README.md) for more details.  

### Improvements
- [ifplugin](plugins/vpp/ifplugin)
    - default MTU value was removed in order to be able to just pass empty MTU field. MTU now can be
    set only in interface configuration (preferred) or defined in vpp-plugin.conf. If none of them
    is set, MTU value will be empty.
    - interface state data are stored in statuscheck readiness probe
- [l3plugin](plugins/vpp/l3plugin)
    - removed strict configuration order for VPP ARP entries and routes. Both ARP entry or route can 
    be configured without interface already present.
- [l4plugin](plugins/vpp/l4plugin)
   - removed strict configuration order for application namespaces. Application namespace can 
    be configured without interface already present.

### Localclient
- added API for ARP entries, L4 features, Application namespaces, and STN rules.

### Logging
- consolidated and improved logging in vpp and Linux plugins.

### Bugfix
- fixed skip-resync parameter if vpp-plugin.conf is not provided.
- corrected af_packet type interface behavior if veth interface is created/removed.
- several fixes related to the af_packet and veth interface type configuration.
- microservice and veth-interface related events are synchronized.

## Known Issues
- VPP can occasionally cause a deadlock during checksum calculation (https://jira.fd.io/browse/VPP-1134)
- VPP-Agent might not properly handle initialization across plugins (this is not occurring currently, but needs to be tested more)

# Release v1.0.8 (2017-11-21)

## Compatibility
- VPP v18.01-rc0-309-g70bfcaf
- cn-infra v1.0.7

### New Features
- [ifplugin](plugins/vpp/ifplugin)
   - ability to configure STN rules.  See respective
   [readme](plugins/vpp/ifplugin/README.md) in interface plugin for more details.
   - rx-mode settings can be set on interface. Ethernet-type interface can be set to POLLING mode, 
   other types of interfaces supports also INTERRUPT and ADAPTIVE. Fields to set QueueID/QueueIDValid
   are also available
- [l4plugin](plugins/vpp/l4plugin)
   - added new l4 plugin to the VPP plugins. It can be used to enable/disable L4 features
   and configure application namespaces. See respective
    [readme](plugins/vpp/l4plugin/README.md) in L4 plugin for more details.
   - support for VPP plugins/l3plugin ARP configuration. The configurator can perform the
   basic CRUD operation with ARP config.
   
### VPP plugin
- [ifplugin](plugins/vpp/ifplugin)
  - added possibility to add interface to any VRF table.
- [resync](plugins/vpp/data_resync.go)
  - resync error propagation improved. If any resynced configuration fails, rest of the resync
  completes and will not be interrupted. All errors which appear during resync are logged after. 
- added defaultplugins API.
- API contains new Method `DisableResync(keyPrefix ...string)`. One or more ETCD key prefixes 
  can be used as a parameter to disable resync for that specific key(s).
     
### Linux plugin
- [l3plugin](plugins/linux/l3plugin)
  - route configuration does not return an error if the required interface is missing. Instead, the 
  route data are internally stored and configured when the interface appears.
     
### GOVPP
- delay flag removed from GoVPP plugin

### Documentation
- improved in multiple vpp-agent packages

### Minor fixes/improvements
- removed dead links from README files

# Release v1.0.7 (2017-10-30)

## Compatibility
- VPP version v18.01-rc0~154-gfc1c612
- cn-infra v1.0.6

### Major Themes

- [Default VPP plugin](plugins/vpp)
    - added resync strategies. Resync of VPP plugins can be set using
    defaultpluigns config file; Resync can be set to full (always resync everything) or
    dependent on VPP configuration (if there is none, skip resync). Resync can be also
    forced to skip using the parameter. See appropriate changelog in 
    [VPP plugins](plugins/vpp) for details.

### New Features

- [Linuxplugins L3Plugin](plugins/linux/l3plugin)
    - added support for basic CRUD operations with the static Address resolution protocol 
    entries and static Routes.

# Release v1.0.6 (2017-10-17)

## Compatibility
- cn-infra v1.0.5

### Major Themes

- [LinuxPlugin](plugins/linux)
   - The configuration of vEth interfaces modified. Veth configuration defines
   two names: symbolic used internally and the one used in host OS.
   `HostIfName` field is optional. If it is not defined, the name in the host OS
   will be the same as the symbolic one - defined by `Name` field.

# Release v1.0.5 (2017-09-26)

## Compatibility
- VPP version v17.10-rc0~334-gce41a5c
- cn-infra v1.0.4

### Major Themes

- [GoVppMux](plugins/govppmux)
    - configuration file for govpp added
- [Kafka Partitions](vendor/github.com/ligato/cn-infra/messaging/kafka)
    - Changes in offset handling, only automatically partitioned messages (hash, random)
      have their offset marked. Manually partitioned messages are not marked.
    - Implemented post-init consumer (for manual partitioner only) which allows starting
      consuming after kafka-plugin Init()
    - Minimalistic examples & documentation for Kafka API will be improved in a later release.

# Release v1.0.4 (2017-09-08)

### Major Themes

- [Kafka Partitions](vendor/github.com/ligato/cn-infra/messaging/kafka)
    - Implemented new methods that allow to specify partitions & offset parameters:
      * publish: Mux.NewSyncPublisherToPartition() & Mux.NewAsyncPublisherToPartition()
      * watch: ProtoWatcher.WatchPartition()
    - Minimalistic examples & documentation for Kafka API will be improved in a later release.
- [Flavors](flavors)
    - reduced to only [local.FlavorVppLocal](flavors/local/local_flavor.go) & [vpp.Flavor](flavors/vpp/vpp_flavor.go)
- [goVpp]
    - updated version waits until the VPP is ready to accept a new connection

# Release v1.0.3 (2017-09-05)

### Major Themes

Enabled support for wathing data store `OfDifferentAgent()` - see:
* [examples/idx_iface_cache](examples/idx_iface_cache/main.go)
* [examples/examples/idx_bd_cache](examples/idx_bd_cache/main.go)
* [examples/idx_veth_cache](examples/idx_veth_cache/main.go)

Preview of new Kafka client API methods that allows to fill also partition and offset argument. New methods implementation ignores these new parameters for now (fallback to existing implementation based on `github.com/bsm/sarama-cluster` and `github.com/Shopify/sarama`).

## Compatibility
- VPP version v17.10-rc0~265-g809bc74 (upgraded because of VPP MEMIF fixes).

# Release v1.0.2 (2017-08-28)

## Compatibility
- VPP version v17.10-rc0~203

### Major Themes

Algorithms for applying northbound configuration (stored in ETCD key-value data store)
to VPP in the proper order of VPP binary API calls implemented in [Default VPP plugin](plugins/vpp):
- network interfaces, especially:
  - MEMIFs (optimized data plane network interface tailored for a container to container network connectivity)
  - VETHs (standard Linux Virtual Ethernet network interface)
  - AF_Packets (for accessing VETHs and similar type of interface)
  - VXLANs, Physical Network Interfaces, loopbacks ...
- L2 BD & X-Connects
- L3 IP Routes & VRFs
- ACL (Access Control List)

Support for Linux VETH northbound configuration implemented in [Linux Plugin](plugins/linux)
applied in proper order with VPP AF_Packet configuration.

Data Synchronization during startup for network interfaces & L2 BD
(support for the situation when ETCD contain configuration before VPP Agent starts).

Data replication and events:
- Updating operational data in ETCD (VPP indexes such as  sw_if_index) and statistics (port counters).
- Updating statistics in Redis (optional once redis.conf available - see flags).
- Publishing links up/down events to Kafka message bus.

Miscellaneous:
- [Examples](examples)
- Tools:
  - [agentctl CLI tool](cmd/agentctl) that show state & configuration of VPP agents
  - [docker](docker): container-based development environment for the VPP agent
- other features inherited from cn-infra:
  - [health](https://github.com/ligato/cn-infra/tree/master/health): status check & k8s HTTP/REST probes
  - [logging](https://github.com/ligato/cn-infra/tree/master/logging): changing log level at runtime

### Extensibility & integration
Ability to extend the behavior of the VPP Agent by creating new plugins on top of [VPP Agent flavor](flavors/vpp).
New plugins can access API for configured:
[VPP Network interfaces](plugins/vpp/ifplugin/ifaceidx),
[Bridge domains](plugins/vpp/l2plugin/l2idx) and [VETHs](plugins/linux/ifplugin/ifaceidx)
based on [idxvpp](idxvpp) threadsafe map tailored for VPP data
with advanced features (multiple watchers, secondary indexes).

VPP Agent is embeddable in different software projects and with different systems
by using [Local Flavor](flavors/local) to reuse VPP Agent algorithms.
For doing this there is [VPP Agent client version 1](clientv1):
* local client - for embedded VPP Agent (communication inside one operating system process, VPP Agent effectively used as a library)
* remote client - for remote configuration of VPP Agent (while integrating for example with control plane)

## Known Issues
A rarely occurring problem during startup with binary API connectivity.
VPP rejects binary API connectivity when VPP Agent tries to connect
too early (plan fix this behavior in next release).
