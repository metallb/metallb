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

package cacheutil

import (
	"github.com/gogo/protobuf/proto"
	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/vpp-agent/idxvpp"
)

// CacheHelper is a helper the implementation of which is reused among multiple typesafe Caches.
// Beware: index stored in cached mapping is not valid. The meaningful values are the name and metadata.
type CacheHelper struct {
	IDX           idxvpp.NameToIdxRW
	Prefix        string
	DataPrototype proto.Message
	ParseName     func(key string) (name string, err error)
}

const placeHolderIndex uint32 = 0

// DoWatching is supposed to be used as a go routine. It selects the data from the channels in arguments.
func (helper *CacheHelper) DoWatching(resyncName string, watcher datasync.KeyValProtoWatcher) {
	changeChan := make(chan datasync.ChangeEvent, 100)
	resyncChan := make(chan datasync.ResyncEvent, 100)

	watcher.Watch(resyncName, changeChan, resyncChan, helper.Prefix)

	for {
		select {
		case resyncEv := <-resyncChan:
			err := helper.DoResync(resyncEv)
			resyncEv.Done(err)
		case dataChng := <-changeChan:
			err := helper.DoChange(dataChng)
			dataChng.Done(err)
		}
	}
}

// DoChange calls:
// - RegisterName in case of db.Put
// - UnregisterName in case of data.Del
func (helper *CacheHelper) DoChange(dataChng datasync.ChangeEvent) error {
	var err error
	switch dataChng.GetChangeType() {
	case datasync.Put:
		current := proto.Clone(helper.DataPrototype)
		dataChng.GetValue(current)
		name, err := helper.ParseName(dataChng.GetKey())
		if err == nil {
			helper.IDX.RegisterName(name, placeHolderIndex, current)
		}
	case datasync.Delete:
		name, err := helper.ParseName(dataChng.GetKey())
		if err == nil {
			helper.IDX.UnregisterName(name)
		}
	}
	return err
}

// DoResync lists keys&values in ResyncEvent and then:
// - RegisterName (for names that are a part of ResyncEvent)
// - UnregisterName (for names that are not a part of ResyncEvent)
func (helper *CacheHelper) DoResync(resyncEv datasync.ResyncEvent) error {
	var wasError error

	ifaces, found := resyncEv.GetValues()[helper.Prefix]
	if found {
		// Step 1: fill the existing items.
		resyncNames := map[string]interface{}{}
		for {
			item, stop := ifaces.GetNext()
			if stop {
				break
			}
			ifaceName, err := helper.ParseName(item.GetKey())
			if err != nil {
				wasError = err
			} else {
				current := proto.Clone(helper.DataPrototype)
				item.GetValue(current)
				helper.IDX.RegisterName(ifaceName, placeHolderIndex, current)
				resyncNames[ifaceName] = nil
			}
		}

		// Step 2:
		existingNames := []string{} //TODO
		for _, existingName := range existingNames {
			if _, found := resyncNames[existingName]; !found {
				helper.IDX.UnregisterName(existingName)
			}
		}
	}
	return wasError
}

func (helper *CacheHelper) String() string {
	return helper.Prefix
}
