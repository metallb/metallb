# Key-value datastore

The `keyval` package defines the client API to access a key-value data 
store. It comprises two sub-APIs: the `Broker` interface supports reading 
and manipulation of key-value pairs; the `Watcher` API provides functions 
for monitoring of changes in a data store. Both interfaces are available
with arguments of type `[]bytes` (raw data) and `proto.Message` (protobuf
formatted data).

The `keyval` package also provides a skeleton for a key-value plugin.
A particular data store is selected in the `NewSkeleton` constructor
using an argument of type `CoreBrokerWatcher`. The skeleton handles
the plugin's life-cycle and provides unified access to datastore
implementing the `KvPlugin` interface.
