// Copyright (c) 2017 Cisco and/or its affiliates.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package etcd implements the key-value Data Broker client API for the
// etcd key-value data store.  See cn-infra/db/keyval for the definition
// of the key-value Data Broker client API.
//
// The entity that provides access to the data store is called BytesConnectionEtcd.
//
//      +-----------------------+       crud/watch         ______
//      |  BytesConnectionEtcd  |          ---->          | ETCD |
//      +-----------------------+        []byte           +------+
//
// To create a BytesConnectionEtcd, use the following function
//
//   import  "github.com/ligato/cn-infra/db/keyval/etcd"
//
//   db := etcd.NewEtcdConnectionWithBytes(config)
//
// config is a path to a file with the following format:
//
//  key-file: <filepath>
//  ca-file: <filepath>
//  cert-file: <filepath>
//  insecure-skip-tls-verify: <bool>
//  insecure-transport: <bool>
//  dial-timeout: <nanoseconds>
//  operation-timeout: <nanoseconds>
//  endpoints:
//    - <address_1>:<port>
//    - <address_2>:<port>
//    - ..
//    - <address_n>:<port>
//
// Connection to etcd is established using the provided config behind the scenes.
//
// Alternatively, you may connect to etcd by yourself and initialize the
// connection object with a given client.
//
//    db := etcd.NewEtcdConnectionUsingClient(client)
//
// Created BytesConnectionEtcd implements Broker and KeyValProtoWatcher
// interfaces. The example of use can be seen below.
//
// To insert single key-value pair into etcd run:
//		db.Put(key, data)
// To remove a value identified by key:
//      datasync.Delete(key)
//
// In addition to single key-value pair approach, the transaction API is
// provided. Transaction executes multiple operations in a more efficient
// way than one by one execution.
//
//    // create new transaction
//    txn := db.NewTxn()
//
//    // add put operation into the transaction
//    txn.Put(key, value)
//
//    // add delete operation into the transaction
//    txn.Delete(key, value)
//
//    // try to commit the transaction
//    err := txn.Commit()
//
// To retrieve a value identified by key:
//    data, found, rev, err := db.GetValue(key)
//    if err == nil && found {
//       ...
//    }
//
// To retrieve all values matching a key prefix:
//    itr, err := db.ListValues(key)
//    if err != nil {
//       for {
//          data, allReceived, rev, err := itr.GetNext()
//          if allReceived {
//              break
//          }
//          if err != nil {
//              return err
//          }
//          process data...
//       }
//    }
//
// To retrieve values in specified key range:
//    itr, err := db.ListValues(key)
//    if err != nil {
//       for {
//          data, rev, allReceived := itr.GetNext()
//          if allReceived {
//              break
//          }
//          process data...
//       }
//    }
//
// To list keys without fetching the values:
//    itr, err := db.ListKeys(prefix)
//    if err != nil {
//       for {
//          key, rev, allReceived := itr.GetNext()
//          if allReceived {
//              break
//          }
//          process key...
//       }
//    }
//
// To start watching changes in etcd:
//     respChan := make(chan keyval.BytesWatchResp, 0)
//     err = dbw.Watch(respChan, key)
//     if err != nil {
//         os.Exit(1)
//     }
//     for {
//          select {
//              case resp := <-respChan:
//                 switch resp.GetChangeType() {
//                 case data.Put:
//                 key := resp.GetKey()
//                     value := resp.GetValue()
//                     rev := resp.GetRevision()
//                 case data.Delete:
//                     ...
//                 }
//          }
//     }
//
//
// BytesConnectionEtcd also allows to create proxy instances
// (BytesBrokerWatcherEtcd) using NewBroker and NewWatcher methods. Both of
// them accept the prefix argument. The prefix will be automatically
// prepended to all keys in put/delete requests made from the proxy instances.
// In case of get-like calls (GetValue, ListValues, ...) the prefix is trimmed
// from the key of the returned values. They contain only the part following the
// prefix in the key field. The created proxy instances share the connection of
// the BytesConnectionEtcd.
//
//      +-----------------------+
//      | BytesBrokerWatcherEtcd |
//      +-----------------------+
//              |
//              |
//               ----------------->   +-----------------------+       crud/watch         ______
//			                          |  BytesConnectionEtcd  |       ---->             | ETCD |
//               ----------------->   +-----------------------+        ([]byte)         +------+
//              |
//              |
//      +------------------------+
//      | BytesBrokerWatcherEtcd |
//      +------------------------+
//
// To create proxy instances, type:
//    prefixedBroker := db.NewBroker(prefix)
//    prefixedWatcher := db.NewWatcher(prefix)
//
// The usage is the same as shown above.
//
// The package also provides a proto decorator that simplifies the manipulation
// of proto modelled data. The proto decorator accepts arguments of type
// proto.message and marshals them into []byte slices.
//
//
//      +-------------------+--------------------+       crud/watch         ______
//      |  ProtoWrapperEtcd |  ProtoWrapperEtcd  |       ---->             | ETCD |
//      +-------------------+--------------------+        ([]byte)         +------+
//        (proto.Message)
//
// The ProtoWrapperEtcd API is very similar to the BytesConnectionEtcd API.
// The difference is that arguments of type []byte are replaced by arguments
// of type proto.Message, and in some case one of the return values is
// transformed into an output argument.
//
// Example of the decorator initialization:
//
//    // conn is BytesConnectionEtcd initialized as shown at the top of the page
//    protoBroker := etcd.NewProtoWrapperEtcd(conn)
//
// The only difference in Put/Delete functions is the type of the argument;
// apart from that the usage is the same as described above.
//
// Example of retrieving single key-value pair using proto decorator:
//   // if the value exists it is unmarshalled into the msg
//   found, rev, err := protoBroker.GetValue(key, msg)
//
//
// To retrieve all values matching the key prefix use
//   resp, err := protoDb.ListValues(path)
//   if err != nil {
//      os.Exit(1)
//   }
//
//   for {
//      // phonebook.Contact is a proto modelled structure (implementing proto.Message interface)
//      contact := &phonebook.Contact{}
//      // the value is unmarshaled into the contact variable
//      kv, allReceived  := resp.GetNext()
//      if allReceived {
//         break
//      }
//      err = kv.GetValue(contact)
//      if err != nil {
//          os.Exit(1)
//      }
//      ... use contact
//  }
//
// The Etcd plugin
//
//    plugin := etcd.Plugin{}
//    // initialization by agent core
//
// Plugin allows to create a broker
//    broker := plugin.NewBroker(prefix)
//
// and watcher
//    watcher := plugin.NewWatcher(prefix)
package etcd
