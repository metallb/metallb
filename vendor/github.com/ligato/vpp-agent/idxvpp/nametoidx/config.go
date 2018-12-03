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

package nametoidx

import (
	"io/ioutil"
	"time"

	"github.com/ghodss/yaml"
)

// PersistentStorageConfig defines the configuration section dedicated for persistent storage.
type PersistentStorageConfig struct {
	Location          string        `json:"location"`
	SyncInterval      time.Duration `json:"sync-interval"`
	MaxSyncStartDelay time.Duration `json:"max-sync-start-delay"`
}

// Config defines configuration for index-to-name maps.
type Config struct {
	PersistentStorage PersistentStorageConfig `json:"persistent-storage"`
}

const (
	/* Default location for the persistent storage of index-name maps */
	defaultPersistentStorageLocation = "/var/vnf-agent/idxmap"

	/* This is the default value for how often (in nanoseconds) to flush the underlying registry into the persistent storage. */
	defaultSyncInterval = 300 * time.Millisecond

	/* To evenly distribute I/O load, the start of the periodic synchronization for a given
	index-name map gets delayed by a random time duration. This constant defines the maximum
	allowed delay in nanoseconds as used by default. */
	defaultMaxSyncStartDelay = 3 * time.Second
)

// ConfigFromFile loads the idxmap configuration from the specified file.
// If the specified file exists and contains valid configuration, the parsed configuration is returned.
// In case of an error, the default configuration is returned instead.
func ConfigFromFile(fpath string) (*Config, error) {
	// default configuration
	persistentStorageConfig := PersistentStorageConfig{}
	persistentStorageConfig.Location = defaultPersistentStorageLocation
	persistentStorageConfig.SyncInterval = defaultSyncInterval
	persistentStorageConfig.MaxSyncStartDelay = defaultMaxSyncStartDelay
	config := &Config{}
	config.PersistentStorage = persistentStorageConfig

	if fpath == "" {
		return config, nil
	}

	b, err := ioutil.ReadFile(fpath)
	if err != nil {
		return config, err
	}

	yamlConfig := Config{}
	err = yaml.Unmarshal(b, &yamlConfig)
	if err != nil {
		return config, err
	}

	if yamlConfig.PersistentStorage.Location != "" {
		config.PersistentStorage.Location = yamlConfig.PersistentStorage.Location
	}
	if yamlConfig.PersistentStorage.SyncInterval != 0 {
		config.PersistentStorage.SyncInterval = yamlConfig.PersistentStorage.SyncInterval
	}
	if yamlConfig.PersistentStorage.MaxSyncStartDelay != 0 {
		config.PersistentStorage.MaxSyncStartDelay = yamlConfig.PersistentStorage.MaxSyncStartDelay
	}
	return config, nil
}
