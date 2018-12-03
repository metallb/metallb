package mem

import (
	"github.com/gogo/protobuf/proto"
	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/idxmap"
)

// CacheHelper is a base cache implementation reused by multiple typesafe Caches.
type CacheHelper struct {
	IDX           idxmap.NamedMappingRW
	Prefix        string
	DataPrototype proto.Message
	ParseName     func(key string) (name string, err error)
}

// DoWatching reflects data change and data resync events received from
// <watcher> into the idxmap.
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
// - Put in case of datasync.Put
// - Delete in case of data.Del
func (helper *CacheHelper) DoChange(dataChng datasync.ChangeEvent) error {
	var err error
	switch dataChng.GetChangeType() {
	case datasync.Put:
		current := proto.Clone(helper.DataPrototype)
		dataChng.GetValue(current)
		name, err := helper.ParseName(dataChng.GetKey())
		if err == nil {
			helper.IDX.Put(name, current)
		}
	case datasync.Delete:
		name, err := helper.ParseName(dataChng.GetKey())
		if err == nil {
			helper.IDX.Delete(name)
		}
	}
	return err
}

// DoResync list keys&values in ResyncEvent and then:
// - Put (for names that are part of ResyncEvent)
// - Delete (for names that are not part of ResyncEvent)
func (helper *CacheHelper) DoResync(resyncEv datasync.ResyncEvent) error {
	var wasError error
	//idx.Put()
	ifaces, found := resyncEv.GetValues()[helper.Prefix]
	if found {
		// Step 1: fill the existing things
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
				helper.IDX.Put(ifaceName, current)
				resyncNames[ifaceName] = nil
			}
		}

		// Step 2:
		existingNames := []string{} //TODO
		for _, existingName := range existingNames {
			if _, found := resyncNames[existingName]; !found {
				helper.IDX.Delete(existingName)
			}
		}
	}
	return wasError
}

// String returns the cache prefix.
func (helper *CacheHelper) String() string {
	return helper.Prefix
}
