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

package linuxcalls

import (
	"os"
	"syscall"

	"github.com/pkg/errors"
	"github.com/vishvananda/netns"
)

/* File system */

// FileExists checks whether the file exists.
func (sh *systemHandler) FileExists(name string) (bool, error) {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, errors.Errorf("failed to stat file %s: %v", name, err)
	}
	return true, nil
}

// OpenFile opens a file.
func (sh *systemHandler) OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	return os.OpenFile(name, flag, perm)
}

// MkDirAll creates a directory with all parent directories.
func (sh *systemHandler) MkDirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

// Remove removes named file or directory.
func (sh *systemHandler) Remove(name string) error {
	return os.Remove(name)
}

// Mount makes resources available.
func (sh *systemHandler) Mount(source string, target string, fsType string, flags uintptr, data string) error {
	return syscall.Mount(source, target, fsType, flags, data)
}

// Unmount resources.
func (sh *systemHandler) Unmount(target string, flags int) error {
	return syscall.Unmount(target, flags)
}

/* Network Namespace */

// NewNetworkNamespace creates a new namespace and returns a handle to manage it further.
func (sh *systemHandler) NewNetworkNamespace() (ns netns.NsHandle, err error) {
	return netns.New()
}

// DuplicateNamespaceHandle duplicates network namespace handle.
func (sh *systemHandler) DuplicateNamespaceHandle(ns netns.NsHandle) (netns.NsHandle, error) {
	dup, err := syscall.Dup(int(ns))
	return netns.NsHandle(dup), err
}

// GetCurrentNamespace gets a handle to the current threads network namespace.
func (sh *systemHandler) GetCurrentNamespace() (ns netns.NsHandle, err error) {
	return netns.Get()
}

// GetNamespaceFromPath gets a handle to a network namespace identified
// by the path.
func (sh *systemHandler) GetNamespaceFromPath(path string) (ns netns.NsHandle, err error) {
	return netns.GetFromPath(path)
}

// GetNamespaceFromPid gets a handle to the network namespace of a given pid.
func (sh *systemHandler) GetNamespaceFromPid(pid int) (ns netns.NsHandle, err error) {
	return netns.GetFromPid(pid)
}

// GetNamespaceFromName gets a handle to a named network namespace such as one
// created by `ip netns add`.
func (sh *systemHandler) GetNamespaceFromName(name string) (ns netns.NsHandle, err error) {
	return netns.GetFromName(name)
}

// SetNamespace sets the current namespace to the namespace represented by the handle.
func (sh *systemHandler) SetNamespace(ns netns.NsHandle) (err error) {
	return netns.Set(ns)
}
