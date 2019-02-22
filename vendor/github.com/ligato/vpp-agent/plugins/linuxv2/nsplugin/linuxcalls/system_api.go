// Copyright (c) 2018 Cisco and/or its affiliates.
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

package linuxcalls

import (
	"os"

	"github.com/vishvananda/netns"
)

// SystemAPI defines all methods required for managing network namespaces
// on the system level.
type SystemAPI interface {
	FileSystemAPI
	NetworkNamespaceAPI
}

// FileSystemAPI defines all methods used to access file system.
type FileSystemAPI interface {
	// FileExists checks whether the file exists.
	FileExists(name string) (bool, error)
	// OpenFile opens a file.
	OpenFile(name string, flag int, perm os.FileMode) (*os.File, error)
	// MkDirAll creates a directory with all parent directories.
	MkDirAll(path string, perm os.FileMode) error
	// Remove removes named file or directory.
	Remove(name string) error
	// Mount makes resources available.
	Mount(source string, target string, fsType string, flags uintptr, data string) error
	// Unmount resources.
	Unmount(target string, flags int) (err error)
}

// NetworkNamespaceAPI defines methods for low-level handling of network namespaces.
type NetworkNamespaceAPI interface {
	// NewNetworkNamespace creates a new namespace and returns a handle to manage it further.
	NewNetworkNamespace() (ns netns.NsHandle, err error)
	// DuplicateNamespaceHandle duplicates network namespace handle.
	DuplicateNamespaceHandle(ns netns.NsHandle) (netns.NsHandle, error)
	// GetCurrentNamespace gets a handle to the current threads network namespace.
	GetCurrentNamespace() (ns netns.NsHandle, err error)
	// GetNamespaceFromPath gets a handle to a network namespace identified
	// by the path.
	GetNamespaceFromPath(path string) (ns netns.NsHandle, err error)
	// GetNamespaceFromPid gets a handle to the network namespace of a given pid.
	GetNamespaceFromPid(pid int) (ns netns.NsHandle, err error)
	// GetNamespaceFromName gets a handle to a named network namespace such as one
	// created by `ip netns add`.
	GetNamespaceFromName(name string) (ns netns.NsHandle, err error)
	// SetNamespace sets the current namespace to the namespace represented by the handle.
	SetNamespace(ns netns.NsHandle) (err error)
}

// systemHandler implements SystemAPI using actual syscalls (i.e. not suitable for tests).
type systemHandler struct {
}

// NewSystemHandler returns new handler.
func NewSystemHandler() SystemAPI {
	return &systemHandler{}
}
