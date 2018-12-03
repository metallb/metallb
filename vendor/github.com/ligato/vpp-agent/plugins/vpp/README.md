# VPP plugins
 
 VPP plugins manage basic configuration of the VPP. The management of configuration is split among multiple
 packages. Detailed description can be found in particular READMEs:
 - [aclplugin](aclplugin)
 - [ifplugin](ifplugin)
 - [l2plugin](l2plugin)
 - [l3plugin](l3plugin)
 - [l4plugin](l4plugin)
 - [srplugin](srplugin)
 
# Config file 

 The default plugins can use configuration file `vpp.conf` to:
  * set global maximum transmission unit 
  * set VPP resync strategy
  * to enable/disable status publishers
  
  To run the vpp-agent with vpp.conf:
   
   `vpp-agent --vpp-plugin-config=/opt/vpp-agent/dev/vpp.conf`
  
 *MTU*
 
 If there is an interface without MTU set, the value from configuration file will be used. MTU is written in config 
 as follows:
 
 `mtu: <value>`
 
 *Resync Strategy*
 
 There are two strategies available for VPP resync:
 * **full** always performs the full resync of all VPP plugins. This is the default strategy. 
 * **optimize-cold-start** evaluates the existing configuration in the VPP at first. The state of interfaces is the 
 decision-maker: if there is any interface configured except local0, the resync is performed normally. Otherwise 
 it is skipped.  
 IMPORTANT NOTE: Use it carefully because the state of the ETCD is not taken into consideration .
 
 Strategy can be set in vpp.conf:
 
 `strategy: full` or  `strategy: optimize`
 
 To **skip resync** completely, start vpp-agent with `--skip-vpp-resync` parameter. In such a case the resync is skipped 
 completly (config file resync strategy is not taken into account). 

 *Status Publishers*

 Status Publishers define list of data syncs to which status is published.

 `status-publishers: [<datasync>, <datasync2>, ...]`

 Currently supported data syncs for publishing state: `etcd` and `redis`.