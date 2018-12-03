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

package filesystem

import (
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/ligato/cn-infra/logging"
	"github.com/pkg/errors"
	"io/ioutil"
	"os"
	"path/filepath"
)

// API defines filesystem-related method with emphasis on the fileDB needs
type API interface {
	// CreateFile creates a new file. Returns error if provided parameter is directory.
	CreateFile(file string) error
	// ReadFile returns an content of given file
	ReadFile(file string) ([]byte, error)
	// WriteFile writes data to file
	WriteFile(file string, data []byte) error
	// FileExists returns true if given path was found
	FileExists(file string) bool
	// GetFiles takes a list of file system paths an returns a list of individual file paths
	GetFileNames(paths []string) ([]string, error)
	// Watch given paths, call 'onEvent' function on every event or 'onClose' if watcher is closed
	Watch(paths []string, onEvent func(event fsnotify.Event), onClose func()) error
	// Close allows to terminate watcher from outside which calls 'onClose'
	Close() error
}

// Handler is helper struct to manipulate with filesystem API
type Handler struct {
	log     logging.Logger
	watcher *fsnotify.Watcher
}

// NewFsHandler creates a new instance of file system handler
func NewFsHandler() *Handler {
	return &Handler{}
}

// CreateFile is an implementation of the file system API interface
func (fsh *Handler) CreateFile(file string) error {
	path, _ := filepath.Split(file)
	// Create path at first if necessary
	if path != "" {
		if err := os.MkdirAll(path, os.ModePerm); err != nil {
			return errors.Errorf("failed to create path for file %s: %v", file, err)
		}
	}
	sf, err := os.Create(file)
	if err != nil {
		return errors.Errorf("failed to create file %s: %v", file, err)
	}
	return sf.Close()
}

// ReadFile is an implementation of the file system API interface
func (fsh *Handler) ReadFile(file string) ([]byte, error) {
	return ioutil.ReadFile(file)
}

// WriteFile is an implementation of the file system API interface
func (fsh *Handler) WriteFile(file string, data []byte) error {
	fileObj, err := os.OpenFile(file, os.O_TRUNC|os.O_WRONLY, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to open status file %s for writing: %v", file, err)
	}
	defer fileObj.Close()
	if _, err := fileObj.Write(data); err != nil {
		return fmt.Errorf("failed to write status file %s for writing: %v", file, err)
	}
	return nil
}

// FileExists is an implementation of the file system API interface
func (fsh *Handler) FileExists(file string) bool {
	if _, err := os.Stat(file); os.IsNotExist(err) {
		return false
	}
	return true
}

// GetFileNames is an implementation of the file system API interface
func (fsh *Handler) GetFileNames(paths []string) (files []string, err error) {
	for _, path := range paths {
		err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info == nil || info.IsDir() {
				return nil
			}
			files = append(files, path)
			return nil
		})
		if err != nil {
			err = errors.Errorf("failed to traverse through %s: %v", path, err)
		}
	}
	return files, err
}

// Watch starts new filesystem notification watcher. All events from files are passed to 'onEvent' function.
// Function 'onClose' is called when event channel is closed.
func (fsh *Handler) Watch(paths []string, onEvent func(event fsnotify.Event), onClose func()) error {
	var err error
	fsh.watcher, err = fsnotify.NewWatcher()
	if err != nil {
		return errors.Errorf("failed to init fileDB file system watcher: %v", err)
	}
	for _, path := range paths {
		fsh.watcher.Add(path)
	}

	go func() {
		for {
			select {
			case event, ok := <-fsh.watcher.Events:
				if !ok {
					onClose()
					return
				}
				onEvent(event)
			case err := <-fsh.watcher.Errors:
				if err != nil {
					fsh.log.Errorf("filesystem notification error %v", err)
				}
			}
		}
	}()

	return nil
}

// Close the file watcher
func (fsh *Handler) Close() error {
	return fsh.watcher.Close()
}

// Processes given path. If the target is a file, it is stored in the file list. If the target
// is a directory, function is called recursively on nested paths in order to process the whole tree.
func (fsh *Handler) getFilesInPath(files []string, path string) error {
	pathInfo, err := os.Stat(path)
	if err != nil {
		return errors.Errorf("failed to read path %s: %v", path, err)
	}
	if pathInfo.IsDir() {
		pathList, err := ioutil.ReadDir(path)
		if err != nil {
			return errors.Errorf("failed to read directory %s: %v", path, err)
		}
		for _, nested := range pathList {
			// Recursive call to process all the tree
			if err := fsh.getFilesInPath(files, path+nested.Name()); err != nil {
				return err
			}
		}
	}
	files = append(files, path)

	return nil
}
