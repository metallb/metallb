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

package filedb

import (
	"bytes"
	"strings"
	"sync"

	"github.com/ligato/cn-infra/db/keyval/filedb/database"
	"github.com/ligato/cn-infra/db/keyval/filedb/decoder"
	"github.com/ligato/cn-infra/db/keyval/filedb/filesystem"

	"github.com/pkg/errors"

	"github.com/fsnotify/fsnotify"
	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/db/keyval"
	"github.com/ligato/cn-infra/logging"
)

// Client arranges communication between file system and internal database, and is also responsible for upstream events.
type Client struct {
	sync.Mutex
	log logging.Logger

	// A list of filesystem paths. It may be a specific file, or the whole directory. In case a path is a directory,
	// all files within are processed. If there are another directories inside, they are skipped.
	cfgPaths []string

	// Path where the status will be stored. Status will be stored either as json or yaml, determined by file suffix.
	// If the field is empty or point to a directory, status is not propagated to any file.
	statusPath string
	// Status reader is chosen according to status file extension.
	statusDecoder decoder.API

	// Internal database mirrors changes in file system. Since the configuration can be only read, it is up to client
	// to handle difference between configuration revisions. Every database entry consists from three values:
	//  - path (where the configuration is written)
	//  - data key
	//  - data value
	// Note: database holds all configuration, not only intended for agent with appropriate prefix
	db database.FilesSystemDB

	// Filesystem handler, provides methods to work with files/directories
	fsHandler filesystem.API

	// File system decoder API, grants access to methods needed for decoding files. Client expects
	// the decoder to process files with specific extension
	decoders []decoder.API

	// A set of watchers for every key.
	watchers map[string]chan keyedData
}

// NewClient initializes file watcher, database and registers paths provided via plugin configuration file
func NewClient(cfgPaths []string, statusPath string, dcs []decoder.API, fsh filesystem.API, log logging.Logger) (*Client, error) {
	// Init client object
	c := &Client{
		cfgPaths:   cfgPaths,
		statusPath: statusPath,
		fsHandler:  fsh,
		watchers:   make(map[string]chan keyedData),
		db:         database.NewDbClient(),
		decoders:   dcs,
		log:        log,
	}

	// Init filesystem handler
	filePaths, err := c.fsHandler.GetFileNames(c.cfgPaths)
	if err != nil {
		return nil, errors.Errorf("failed to read files from provided paths: %v", err)
	}
	// Decode initial configuration
	for _, filePath := range filePaths {
		if dc := c.getFileDecoder(filePath); dc != nil {
			byteFile, err := c.fsHandler.ReadFile(filePath)
			if err != nil {
				return nil, errors.Errorf("failed to read file %s content: %v", filePath, err)
			}
			fileEntries, err := dc.Decode(byteFile)
			file := &decoder.File{Path: filePath, Data: fileEntries}
			if err != nil {
				return nil, errors.Errorf("failed to decode file %s: %v", filePath, err)
			}
			// Put all the configuration to the database
			for _, data := range file.Data {
				c.db.Add(file.Path, &decoder.FileDataEntry{Key: data.Key, Value: data.Value})
			}
		}
	}
	// Validate and prepare the status file and decoder
	if c.statusPath != "" {
		c.statusDecoder = c.getFileDecoder(c.statusPath)
		if c.statusDecoder == nil {
			return nil, errors.Errorf("failed to get decoder for status file (unknown extension) %s: %v", c.statusPath, err)
		}
		filePath, err := c.fsHandler.GetFileNames([]string{c.statusPath})
		if err != nil {
			return nil, errors.Errorf("failed to read status file: %v", err)
		}
		// Expected is at most single entry
		if len(filePath) == 0 {
			if err := c.fsHandler.CreateFile(c.statusPath); err != nil {
				return nil, errors.Errorf("failed to create status file: %v", err)
			}
		} else if len(filePath) > 1 {
			return nil, errors.Errorf("failed to process status file, unexpected processing output: %v", err)
		}
	}

	return c, nil
}

// GetPaths returns client file paths
func (c *Client) GetPaths() []string {
	return c.cfgPaths
}

// GetDataForFile returns data gor given file
func (c *Client) GetDataForFile(path string) []*decoder.FileDataEntry {
	return c.db.GetDataForFile(path)
}

// GetDataForKey returns data gor given file
func (c *Client) GetDataForKey(key string) (*decoder.FileDataEntry, bool) {
	return c.db.GetDataForKey(key)
}

// Represents data with both identifiers, file path and key.
type keyedData struct {
	path string
	watchResp
}

// NewBroker provides BytesBroker object with client and given prefix
func (c *Client) NewBroker(prefix string) keyval.BytesBroker {
	return &BrokerWatcher{
		Client: c,
		prefix: prefix,
	}
}

// NewWatcher provides BytesWatcher object with client and given prefix
func (c *Client) NewWatcher(prefix string) keyval.BytesWatcher {
	return &BrokerWatcher{
		Client: c,
		prefix: prefix,
	}
}

// Put reads status file, add data to it and performs write
func (c *Client) Put(key string, data []byte, opts ...datasync.PutOption) error {
	newEntry := &decoder.FileDataEntry{Key: key, Value: data}
	statusDataEntries := c.db.GetDataForFile(c.statusPath)
	// Add/update data
	var updated bool
	for _, statusDataEntry := range statusDataEntries {
		if statusDataEntry.Key == key {
			statusDataEntry.Value = data
			updated = true
			break
		}
	}
	if !updated {
		statusDataEntries = append(statusDataEntries, newEntry)
	}
	c.db.Add(c.statusPath, newEntry)
	// Encode and write
	stFileEntries, err := c.statusDecoder.Encode(statusDataEntries)
	if err != nil {
		return errors.Errorf("failed to write status to fileDB: unable to encode status file %s: %v", c.statusPath, err)
	}
	err = c.fsHandler.WriteFile(c.statusPath, stFileEntries)
	if err != nil {
		return errors.Errorf("failed to write status %s to fileDB: %v", c.statusPath, err)
	}
	return nil
}

// NewTxn is not supported, filesystem plugin does not allow to do changes to the configuration
func (c *Client) NewTxn() keyval.BytesTxn {
	c.log.Warnf("creating transaction chains in fileDB is currently not allowed")
	return nil
}

// GetValue returns a value for given key
func (c *Client) GetValue(key string) (data []byte, found bool, revision int64, err error) {
	var entry *decoder.FileDataEntry
	entry, found = c.db.GetDataForKey(key)
	data = entry.Value
	return
}

// ListValues returns a list of values for given prefix
func (c *Client) ListValues(prefix string) (keyval.BytesKeyValIterator, error) {
	keyValues := c.db.GetDataForPrefix(prefix)
	data := make([]*decoder.FileDataEntry, 0, len(keyValues))
	for _, entry := range keyValues {
		data = append(data, &decoder.FileDataEntry{
			Key:   entry.Key,
			Value: entry.Value,
		})
	}
	return &bytesKeyValIterator{len: len(data), data: data}, nil
}

// ListKeys returns a set of keys for given prefix
func (c *Client) ListKeys(prefix string) (keyval.BytesKeyIterator, error) {
	entries := c.db.GetDataForPrefix(prefix)
	var keysWithoutPrefix []string
	for _, entry := range entries {
		keysWithoutPrefix = append(keysWithoutPrefix, entry.Key)
	}
	return &bytesKeyIterator{len: len(keysWithoutPrefix), keys: keysWithoutPrefix, prefix: prefix}, nil
}

// Delete is not allowed for fileDB, configuration file is read-only
func (c *Client) Delete(key string, opts ...datasync.DelOption) (existed bool, err error) {
	c.log.Warnf("deleting configuration from fileDB is currently not allowed")
	return false, nil
}

// Watch starts single watcher for every key prefix. Every watcher listens on its own data channel.
func (c *Client) Watch(resp func(response keyval.BytesWatchResp), closeChan chan string, keys ...string) error {
	c.Lock()
	defer c.Unlock()

	for _, key := range keys {
		dc := make(chan keyedData)
		go c.watch(resp, dc, closeChan, key)
		c.watchers[key] = dc
	}

	return nil
}

// Close closes all readers
func (c *Client) Close() error {
	if c.fsHandler != nil {
		return c.fsHandler.Close()
	}
	return nil
}

// Awaits changes from data channel, prepares responses and sends them to the response function
func (c *Client) watch(resp func(response keyval.BytesWatchResp), dataChan chan keyedData, closeChan chan string, key string) {
	for {
		select {
		case keyedData, ok := <-dataChan:
			if !ok {
				return
			}
			if keyedData.Op == datasync.Delete || !bytes.Equal(keyedData.PrevValue, keyedData.Value) {
				resp(&keyedData.watchResp)
			}
		case <-closeChan:
			return
		}
	}
}

// Event watcher starts file system watcher for every reader available.
func (c *Client) eventWatcher() {
	c.fsHandler.Watch(c.cfgPaths, c.onEvent, c.onClose)
}

// OnEvent is common method called when new event from file system arrives. Different files may require different
// reader, but the data processing is the same.
func (c *Client) onEvent(event fsnotify.Event) {
	// If file was removed, delete all configuration associated with it. Do the same action for
	// rename, following action will be create with the new name which re-applies the configuration
	// (if new name is in scope of the defined path)
	if (event.Op == fsnotify.Rename || event.Op == fsnotify.Remove) && !c.fsHandler.FileExists(event.Name) {
		entries := c.db.GetDataForFile(event.Name)
		for _, entry := range entries {
			// Value from DB does not need to be checked
			keyed := keyedData{
				path:      event.Name,
				watchResp: watchResp{Op: datasync.Delete, Key: entry.Key, Value: nil, PrevValue: entry.Value},
			}
			c.sendToChannel(keyed)
			c.db.DeleteFile(event.Name)
		}
		return
	}

	// Read data from file
	dc := c.getFileDecoder(event.Name)
	if dc == nil {
		return
	}
	byteFile, err := c.fsHandler.ReadFile(event.Name)
	if err != nil {
		c.log.Errorf("failed to process filesystem event: file cannot be read %s: %v", event.Name, err)
		return
	}
	decodedFileEntries, err := dc.Decode(byteFile)
	if err != nil {
		c.log.Errorf("failed to process filesystem event: file cannot be decoded %s: %v", event.Name, err)
		return
	}
	file := &decoder.File{Path: event.Name, Data: decodedFileEntries}
	latestFile := &decoder.File{Path: event.Name, Data: c.db.GetDataForFile(event.Name)}
	changed, removed := file.CompareTo(latestFile)
	// Update database and propagate data to channel
	for _, fileDataEntry := range removed {
		keyed := keyedData{
			path:      event.Name,
			watchResp: watchResp{Op: datasync.Delete, Key: fileDataEntry.Key, Value: nil, PrevValue: fileDataEntry.Value},
		}
		c.sendToChannel(keyed)
		c.db.Delete(event.Name, keyed.Key)
	}
	for _, fileDataEntry := range changed {
		// Get last key-val configuration item if exists
		var prevVal []byte
		if prevValEntry, ok := c.db.GetDataForKey(fileDataEntry.Key); ok {
			prevVal = prevValEntry.Value
		}
		keyed := keyedData{
			path:      event.Name,
			watchResp: watchResp{Op: datasync.Put, Key: fileDataEntry.Key, Value: fileDataEntry.Value, PrevValue: prevVal},
		}
		c.sendToChannel(keyed)
		c.db.Add(event.Name, &decoder.FileDataEntry{
			Key:   keyed.Key,
			Value: keyed.Value,
		})
	}
}

// OnClose is called from filesystem watcher when the file system data channel is closed.
func (c *Client) onClose() {
	for _, channel := range c.watchers {
		close(channel)
	}
}

// Send data to correct channel
func (c *Client) sendToChannel(keyed keyedData) {
	c.Lock()
	defer c.Unlock()

	for key, channel := range c.watchers {
		if strings.Contains(keyed.Key, key) {
			channel <- keyed
			return
		}
	}
}

// Use known decoders to decide whether the file can or cannot be processed. If so, return proper decoder.
func (c *Client) getFileDecoder(file string) decoder.API {
	for _, dc := range c.decoders {
		if dc.IsProcessable(file) {
			return dc
		}
	}
	return nil
}
