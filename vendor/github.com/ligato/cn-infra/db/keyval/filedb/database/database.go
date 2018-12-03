//  Copyright (c) 2018 Cisco and/or its affiliates.
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at:
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package database

import (
	"bytes"
	"strings"
	"sync"

	"github.com/ligato/cn-infra/db/keyval/filedb/decoder"
)

const initialRev = 0

// FilesSystemDB provides methods to manipulate internal filesystem database
type FilesSystemDB interface {
	// Add new key-value data under provided path (path represents file). Newly added data are stored with initial
	// revision, existing entries are updated
	Add(path string, entry *decoder.FileDataEntry)
	// Delete removes key-value data from provided file
	Delete(path, key string)
	// Delete file removes file entry from database, together with all underlying key-value data
	DeleteFile(path string)
	// GetValuesForPrefix filters the whole database and returns a map of key-value data
	GetDataForPrefix(prefix string) []*decoder.FileDataEntry
	// GetDataFromFile returns all the configuration for specific file
	GetDataForFile(path string) []*decoder.FileDataEntry
	// GetDataForKey returns data for key with flag whether the data was found or not
	GetDataForKey(key string) (*decoder.FileDataEntry, bool)
}

// DbClient is database client
type DbClient struct {
	sync.Mutex
	db map[string]map[string]*dbEntry // Path + Key + Data/Rev
}

// Single database entry without key - data and revision
type dbEntry struct {
	data []byte
	rev  int
}

// NewDbClient returns new database client
func NewDbClient() *DbClient {
	return &DbClient{
		db: make(map[string]map[string]*dbEntry),
	}
}

// Add puts new entry to the database, or updates the old one if given key already exists
func (c *DbClient) Add(path string, entry *decoder.FileDataEntry) {
	c.Lock()
	defer c.Unlock()

	if entry == nil {
		return
	}

	fileData, ok := c.db[path]
	if ok {
		value, ok := fileData[entry.Key]
		if ok {
			if !bytes.Equal(value.data, entry.Value) {
				rev := value.rev + 1
				fileData[entry.Key] = &dbEntry{entry.Value, rev}
			}
		} else {
			fileData[entry.Key] = &dbEntry{entry.Value, initialRev}
		}
	} else {
		fileData = map[string]*dbEntry{entry.Key: {entry.Value, initialRev}}
	}

	c.db[path] = fileData
}

// Delete removes key in given path.
func (c *DbClient) Delete(path, key string) {
	c.Lock()
	defer c.Unlock()

	fileData, ok := c.db[path]
	if !ok {
		return
	}
	delete(fileData, key)
}

// DeleteFile removes file entry including all keys within
func (c *DbClient) DeleteFile(path string) {
	c.Lock()
	defer c.Unlock()

	delete(c.db, path)
}

// GetDataForPrefix returns all values which match provided prefix
func (c *DbClient) GetDataForPrefix(prefix string) []*decoder.FileDataEntry {
	c.Lock()
	defer c.Unlock()

	var keyValues []*decoder.FileDataEntry
	for _, file := range c.db {
		for key, value := range file {
			if strings.HasPrefix(key, prefix) {
				keyValues = append(keyValues, &decoder.FileDataEntry{
					Key:   key,
					Value: value.data,
				})
			}
		}
	}
	return keyValues
}

// GetDataForFile returns a map of key-value entries from given file
func (c *DbClient) GetDataForFile(path string) []*decoder.FileDataEntry {
	c.Lock()
	defer c.Unlock()

	var keyValues []*decoder.FileDataEntry
	if dbKeyValues, ok := c.db[path]; ok {
		for key, value := range dbKeyValues {
			keyValues = append(keyValues, &decoder.FileDataEntry{
				Key:   key,
				Value: value.data,
			})
		}
	}
	return keyValues
}

// GetDataForKey returns data for given key.
func (c *DbClient) GetDataForKey(key string) (*decoder.FileDataEntry, bool) {
	c.Lock()
	defer c.Unlock()

	for _, file := range c.db {
		value, ok := file[key]
		if ok {
			return &decoder.FileDataEntry{
				Key:   key,
				Value: value.data,
			}, true
		}
	}
	return nil, false
}
