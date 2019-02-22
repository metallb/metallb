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
	"runtime"

	"github.com/ligato/cn-infra/logging"
	"github.com/vishvananda/netns"
)

// NamedNetNsAPI defines methods related to management of named network namespaces.
type NamedNetNsAPI interface {
	// CreateNamedNetNs creates a new named Linux network namespace.
	// It does exactly the same thing as the command "ip netns add NAMESPACE".
	CreateNamedNetNs(ctx NamespaceMgmtCtx, nsName string) (netns.NsHandle, error)
	// DeleteNamedNetNs deletes an existing named Linux network namespace.
	// It does exactly the same thing as the command "ip netns del NAMESPACE".
	DeleteNamedNetNs(nsName string) error
	// NamedNetNsExists checks whether named namespace exists.
	NamedNetNsExists(nsName string) (bool, error)
}

// NamespaceMgmtCtx represents context of an ongoing management of Linux namespaces.
// The same context should not be used concurrently.
type NamespaceMgmtCtx interface {
	// LockOSThread wires the calling goroutine to its current operating system thread.
	// The method should implement re-entrant lock always called from a single go routine.
	LockOSThread()
	// UnlockOSThread unwires the calling goroutine from its fixed operating system thread.
	// The method should implement re-entrant lock always called from a single go routine.
	UnlockOSThread()
}

// namespaceMgmtCtx implements NamespaceMgmtCtx.
type namespaceMgmtCtx struct {
	lockOsThreadCnt int
}

// LockOSThread wires the calling goroutine to its current operating system thread.
func (ctx *namespaceMgmtCtx) LockOSThread() {
	if ctx.lockOsThreadCnt == 0 {
		runtime.LockOSThread()
	}
	ctx.lockOsThreadCnt++
}

// UnlockOSThread unwires the calling goroutine from its fixed operating system thread.
func (ctx *namespaceMgmtCtx) UnlockOSThread() {
	ctx.lockOsThreadCnt--
	if ctx.lockOsThreadCnt == 0 {
		runtime.UnlockOSThread()
	}
}

// NewNamespaceMgmtCtx creates and returns a new context for management of Linux
// namespaces.
func NewNamespaceMgmtCtx() NamespaceMgmtCtx {
	return &namespaceMgmtCtx{}
}

// namedNetNsHandler implements NamedNetNsAPI using provided system handler.
type namedNetNsHandler struct {
	log        logging.Logger
	sysHandler SystemAPI
}

// NewNamedNetNsHandler creates new instance of namespace handler
func NewNamedNetNsHandler(sysHandler SystemAPI, log logging.Logger) NamedNetNsAPI {
	return &namedNetNsHandler{
		log:        log,
		sysHandler: sysHandler,
	}
}
