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

// Package mem provides in-memory implementation of the mapping with multiple
// indexes.
//
// To create new mapping run:
//	   import "github.com/ligato/cn-infra/idxmap/mem"
//
//     mapping := mem.NewNamedMapping(logger, "title", indexFunc)
//
// Title is used for identification of the mapping.
// IndexFunc extracts secondary indexes from the stored item.
//
// To insert a new item into the mapping, execute:
//
//     mapping.Put(name, value)
//
// Put can also be used to overwrite an existing item associated with the name
// and to rebuild secondary indexes for that item.
//
// To retrieve a particular item identified by name run:
//
//    value, found := mapping.GetValue(name)
//
// To lookup items by secondary indexes, execute:
//
//    names := mapping.ListNames(indexName, indexValue)
//
// names of all matching items are returned.
//
// To retrieve all currently registered names, run:
//
//   names := mapping.ListAllNames()
//
// If you want to remove an item from the mapping, call:
//
//    mapping.Delete(name)
//
// To monitor changes, run:
//    callback := func(notif idxmap.NamedMappingDto) {
// 		   // process notification
//	  }
//
//    mapping.Watch("NameOfWatcher", callback)
//
// If you prefer processing changes through channels:
//
//    ch := make(chan idxmap.NamedMappingGenericEvent)
//    mapping.Watch("NameOfWatcher", ToChan(ch))
//
package mem
