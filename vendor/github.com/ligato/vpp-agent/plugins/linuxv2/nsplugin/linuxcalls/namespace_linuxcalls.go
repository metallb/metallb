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

// +build !windows,!darwin

package linuxcalls

import (
	"os"
	"path"
	"strconv"
	"syscall"

	"github.com/pkg/errors"
	"github.com/vishvananda/netns"

	"github.com/ligato/cn-infra/logging"
)

const (
	// Network namespace mount directory.
	netNsMountDir = "/var/run/netns"
)

// CreateNamedNetNs creates a new named Linux network namespace.
// It does exactly the same thing as the command "ip netns add NAMESPACE".
func (nh *namedNetNsHandler) CreateNamedNetNs(ctx NamespaceMgmtCtx, nsName string) (netns.NsHandle, error) {
	// Lock the OS Thread so we don't accidentally switch namespaces.
	ctx.LockOSThread()
	defer ctx.UnlockOSThread()

	// Save the current network namespace.
	origns, err := nh.sysHandler.GetCurrentNamespace()
	if err != nil {
		return netns.None(), errors.Errorf("failed to get original namespace: %v", err)
	}
	defer origns.Close()

	// Create directory for namespace mounts.
	err = nh.sysHandler.MkDirAll(netNsMountDir, 0755)
	if err != nil {
		return netns.None(), errors.Errorf("failed to create directory for namespace mounts: %v", err)
	}

	/* Make it possible for network namespace mounts to propagate between
	   mount namespaces.  This makes it likely that unmounting a network
	   namespace file in one namespace will unmount the network namespace
	   file in all namespaces allowing the network namespace to be freed
	   sooner.
	*/
	mountedNetnsDir := false
	for {
		err = nh.sysHandler.Mount("", netNsMountDir, "none", syscall.MS_SHARED|syscall.MS_REC, "")
		if err == nil {
			break
		}
		if e, ok := err.(syscall.Errno); !ok || e != syscall.EINVAL || mountedNetnsDir {
			return netns.None(), errors.Errorf("%s mount --make-shared failed: %v", nsName, err)
		}
		/* Upgrade netNsMountDir to a mount point */
		err = nh.sysHandler.Mount(netNsMountDir, netNsMountDir, "none", syscall.MS_BIND, "")
		if err != nil {
			return netns.None(), errors.Errorf("%s mount --bind failed: %v", nsName, err)
		}
		mountedNetnsDir = true
	}

	// Create file path for the mount.
	netnsMountFile := path.Join(netNsMountDir, nsName)
	file, err := nh.sysHandler.OpenFile(netnsMountFile, os.O_RDONLY|os.O_CREATE|os.O_EXCL, 0444)
	if err != nil {
		return netns.None(), errors.Errorf("failed to create destination path for the namespace %s mount: %v",
			nsName, err)
	}
	file.Close()

	// Create and switch to a new namespace.
	newNsHandle, err := nh.sysHandler.NewNetworkNamespace()
	if err != nil {
		nh.log.WithFields(logging.Fields{"namespace": nsName}).
			Error("failed to create namespace")
		return netns.None(), errors.Errorf("failed to create namespace %s: %v", nsName, err)
	}
	nh.sysHandler.SetNamespace(newNsHandle)

	// Create a bind-mount for the namespace.
	tid := syscall.Gettid()
	err = nh.sysHandler.Mount("/proc/self/task/"+strconv.Itoa(tid)+"/ns/net", netnsMountFile, "none", syscall.MS_BIND, "")

	// Switch back to the original namespace.
	nh.sysHandler.SetNamespace(origns)

	if err != nil {
		newNsHandle.Close()
		return netns.None(), errors.Errorf("failed to create namespace %s bind-mount: %v", nsName, err)
	}

	return newNsHandle, nil
}

// DeleteNamedNetNs deletes an existing named Linux network namespace.
// It does exactly the same thing as the command "ip netns del NAMESPACE".
func (nh *namedNetNsHandler) DeleteNamedNetNs(nsName string) error {
	// Unmount the namespace.
	netnsMountFile := path.Join(netNsMountDir, nsName)
	err := nh.sysHandler.Unmount(netnsMountFile, syscall.MNT_DETACH)
	if err != nil {
		return errors.Errorf("failed to unmount namespace %s: %v", nsName, err)
	}

	// Remove file path used for the mount.
	err = nh.sysHandler.Remove(netnsMountFile)
	if err != nil {
		return errors.Errorf("failed to remove namespace %s: %v", nsName, err)
	}

	return err
}

// NamedNetNsExists checks whether named  namespace exists.
func (nh *namedNetNsHandler) NamedNetNsExists(nsName string) (bool, error) {
	netnsMountFile := path.Join(netNsMountDir, nsName)
	return nh.sysHandler.FileExists(netnsMountFile)
}
