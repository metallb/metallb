#Vpp-agent-ctl

The vpp-agent-ctl is testing/example utility which purpose is to store given key-value configuration to the ETCD database or read its content. The vpp-agent-ctl consists from two parts, basic crud commands and example data for every configuration type currently supported by the vpp-agent. 

The vpp-agent-ctl does not maintain ETCD connectivity, the link is established before every command execution and released after completion.

## CRUD commands

All those commands can be shown either calling binary without parameter, or with invalid parameter.

**PUT** allows to store data in the ETCD. Put requires two parameters, key and value. The value is represented by .json file. Example json files are stored inside vpp-agent-ctl ([link to directory](json))

```
vpp-agent-ctl -put <key> <data>
```

**GET** can be used to read configuration for given key. If the key does not exist, is not valid or is not set, command returns an empty value.

```
vpp-agent-ctl -get <key>
```

**DEL** removes data from the ETCD, identified with provided key. 

```
vpp-agent-ctl -del <key>
```

**LIST** prints all keys currently present in the database. The command takes no parameter.

 ```
 vpp-agent-ctl -list
 ```
 
**DUMP** returns all key-value pairs currently present in the database. The command takes no parameter.
 
```
vpp-agent-ctl -list
```

## Example pre-defined configurations

For the quick testing or as a configuration example, the vpp-agent-ctl provides special commands for every available configuration type. Commands can be shown running `vpp-agent-ctl` without parameters. They are sorted per vpp-agent plugin always in pairs; one command to crate a configuration, the second one to remove it. 

Data put using command can be edited - all of them are available in [data package](data) separated in files according to plugins, with interface at the top so the desired configuration item can be easily found. Then, just edit the field(s) needed and `go build` the main file. Then, calling respective command will put the changed data.

Example commands:

1. To add access list with IP rules:

```
vpp-agent-clt -aclip
``` 

2. To add VxLAN interface

```
vpp-agent-ctl -vxlan
```

3. To delete TAP interface

```
vpp-agent-ctl -tapd
```

All the 'delete' cases are by default set to match with creating data (so every delete removes the data created by associated create command).