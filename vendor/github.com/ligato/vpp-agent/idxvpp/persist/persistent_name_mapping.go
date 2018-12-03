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

// Package persist asynchronously writes changes in the map (name->idx) to
// file.
package persist

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"math/rand"
	"os"
	"path"
	"sync"
	"time"

	"github.com/ligato/cn-infra/logging"
	log "github.com/ligato/cn-infra/logging/logrus"
	"github.com/ligato/vpp-agent/idxvpp"
	"github.com/ligato/vpp-agent/idxvpp/nametoidx"
)

var (
	// configuration file path
	idxMapConfigFile string

	// configuration common to all mappings
	gConfig *nametoidx.Config
)

// init serves for parsing the program's arguments.
func init() {
	flag.StringVar(&idxMapConfigFile, "idxmap-config", "",
		"Location of the configuration file for index-to-name maps; also set via 'IDXMAP_CONFIG' env variable.")

	if gConfig == nil {
		var err error
		gConfig, err = nametoidx.ConfigFromFile(idxMapConfigFile)
		if err != nil {
			log.DefaultLogger().WithFields(logging.Fields{"filepath": idxMapConfigFile, "err": err}).Warn(
				"Failed to load idxmap configuration file")
		} else {
			log.DefaultLogger().WithFields(logging.Fields{"filepath": idxMapConfigFile}).Debug(
				"Loaded idxmap configuration file")
		}
	}
}

// Marshalling loads the config and starts watching for changes.
func Marshalling(agentLabel string, idxMap idxvpp.NameToIdx, loadedFromFile idxvpp.NameToIdxRW) error {
	log.DefaultLogger().Debug("Persistence")

	changes := make(chan idxvpp.NameToIdxDto, 1000)
	fileName := idxMap.GetRegistryTitle() + ".json"
	persist := NewNameToIdxPersist(fileName, gConfig, agentLabel, changes)
	err := persist.Init()
	if err != nil {
		return err
	}

	idxMap.Watch("idxpersist", nametoidx.ToChan(changes))

	err = persist.loadIdxMapFile(loadedFromFile)
	if err != nil {
		return err
	}

	return nil
}

// NameToIdxPersist is a decorator for NameToIdxRW implementing persistent storage.
type NameToIdxPersist struct {
	// registrations notifies about the changes made in the mapping to be persisted
	registrations chan idxvpp.NameToIdxDto

	// Configuration associated with this mapping.
	// Unless this is a unit test, it is the same as the global configuration.
	config *nametoidx.Config

	// (de)serialization of mapping data
	nameToIdx map[string]uint32

	// persistent storage location
	fileDir  string
	filePath string

	// Synchronization between the underlying registry and the persistent storage.
	syncLock  sync.Mutex
	syncCh    chan bool
	syncAckCh chan error
}

// NewNameToIdxPersist initializes decorator for persistent storage of index-to-name mapping.
func NewNameToIdxPersist(fileName string, config *nametoidx.Config, namespace string,
	registrations chan idxvpp.NameToIdxDto) *NameToIdxPersist {

	persist := NameToIdxPersist{}
	persist.config = config
	persist.nameToIdx = map[string]uint32{}

	persist.fileDir = path.Join(persist.config.PersistentStorage.Location, namespace)
	persist.filePath = path.Join(persist.fileDir, fileName)

	persist.registrations = registrations

	return &persist
}

// Init starts Go routine that watches chan idxvpp.NameToIdxDto.
func (persist *NameToIdxPersist) Init() error {
	persist.syncCh = make(chan bool)
	persist.syncAckCh = make(chan error)

	offset := rand.Int63n(int64(persist.config.PersistentStorage.MaxSyncStartDelay))
	go persist.periodicIdxMapSync(time.Duration(offset))

	return nil
}

// loadIdxMapFile loads persistently stored entries of the associated registry.
func (persist *NameToIdxPersist) loadIdxMapFile(loadedFromFile idxvpp.NameToIdxRW) error {
	if _, err := os.Stat(persist.filePath); os.IsNotExist(err) {
		log.DefaultLogger().WithFields(logging.Fields{"Filepath": persist.filePath}).Debug(
			"Persistent storage for name-to-index mapping doesn't exist yet")
		return nil
	}
	idxMapData, err := ioutil.ReadFile(persist.filePath)
	if err != nil {
		return err
	}

	err = json.Unmarshal(idxMapData, &persist.nameToIdx)
	if err != nil {
		return err
	}

	for name, idx := range persist.nameToIdx {
		loadedFromFile.RegisterName(name, idx, nil)
	}
	return nil
}

// periodicIdxMapSync periodically synchronizes the underlying registry with the persistent storage.
func (persist *NameToIdxPersist) periodicIdxMapSync(offset time.Duration) error {
	for {
		select {
		case reg := <-persist.registrations:
			if reg.Del {
				persist.unregisterName(reg.Name)
			} else {
				persist.registerName(reg.Name, reg.Idx)
			}
		case <-persist.syncCh:
			persist.syncAckCh <- persist.syncMapping()
		case <-time.After(persist.config.PersistentStorage.SyncInterval + offset):
			offset = 0
			err := persist.syncMapping()
			if err != nil {
				log.DefaultLogger().WithFields(logging.Fields{"Error": err, "Filepath": persist.filePath}).Error(
					"Failed to sync idxMap with the persistent storage")
			}
		}
	}
}

// syncMapping updates the persistent storage with the new mappings.
// Current implementation simply re-builds the file content from the scratch.
// TODO: NICE-TO-HAVE incremental update
func (persist *NameToIdxPersist) syncMapping() error {
	persist.syncLock.Lock()
	defer persist.syncLock.Unlock()

	idxMapData, err := json.Marshal(persist.nameToIdx)
	if err != nil {
		return err
	}

	err = os.MkdirAll(persist.fileDir, 0777)
	if err != nil {
		return err
	}

	//log.Debug("Persist len=", len(persist.nameToIdx)," ", persist.filePath)

	return ioutil.WriteFile(persist.filePath, idxMapData, 0644)
}

// RegisterName from NameToIdxPersist allows to add a name-to-index mapping into both the underlying registry and
// the persistent storage (with some delay in synchronization).
func (persist *NameToIdxPersist) registerName(name string, idx uint32) {
	persist.nameToIdx[name] = idx
}

// UnregisterName from NameToIdxPersist allows to remove mapping from both
// the underlying registry and the persistent storage.
func (persist *NameToIdxPersist) unregisterName(name string) {
	delete(persist.nameToIdx, name)
}

// Close triggers explicit synchronization and closes the underlying mapping.
func (persist *NameToIdxPersist) Close() error {
	err := persist.Sync()
	if err != nil {
		return err
	}
	return nil
}

// Sync triggers immediate synchronization between the underlying registry and the persistent storage.
// The function doesn't return until the operation has fully finished.
func (persist *NameToIdxPersist) Sync() error {
	persist.syncCh <- true
	return <-persist.syncAckCh
}
