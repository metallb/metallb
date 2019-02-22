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

package nsplugin

import (
	"fmt"
	"strconv"

	"github.com/pkg/errors"
	"github.com/vishvananda/netns"

	"github.com/ligato/cn-infra/infra"
	"github.com/ligato/cn-infra/logging"
	kvs "github.com/ligato/vpp-agent/plugins/kvscheduler/api"

	nsmodel "github.com/ligato/vpp-agent/api/models/linux/namespace"
	"github.com/ligato/vpp-agent/plugins/linuxv2/nsplugin/descriptor"
	nsLinuxcalls "github.com/ligato/vpp-agent/plugins/linuxv2/nsplugin/linuxcalls"
)

// NsPlugin is a plugin to handle namespaces and microservices for other linux
// plugins (ifplugin, l3plugin ...).
// It does not follow the standard concept of CRUD, but provides a set of methods
// other plugins can use to manage namespaces.
type NsPlugin struct {
	Deps

	// From configuration file
	disabled bool

	// Default namespace
	defaultNs netns.NsHandle

	// Handlers
	sysHandler     nsLinuxcalls.SystemAPI
	namedNsHandler nsLinuxcalls.NamedNetNsAPI

	// Descriptor
	msDescriptor *descriptor.MicroserviceDescriptor
}

// Deps lists dependencies of the NsPlugin.
type Deps struct {
	infra.PluginDeps
	KVScheduler kvs.KVScheduler
}

// Config holds the nsplugin configuration.
type Config struct {
	Disabled bool `json:"disabled"`
}

// unavailableMicroserviceErr is error implementation used when a given microservice is not deployed.
type unavailableMicroserviceErr struct {
	label string
}

func (e *unavailableMicroserviceErr) Error() string {
	return fmt.Sprintf("Microservice '%s' is not available", e.label)
}

// Init namespace handler caches and create config namespace
func (p *NsPlugin) Init() error {
	// Parse configuration file
	config, err := p.retrieveConfig()
	if err != nil {
		return err
	}
	if config != nil {
		if config.Disabled {
			p.disabled = true
			p.Log.Infof("Disabling Linux Namespace plugin")
			return nil
		}
	}

	// Handlers
	p.sysHandler = nsLinuxcalls.NewSystemHandler()
	p.namedNsHandler = nsLinuxcalls.NewNamedNetNsHandler(p.sysHandler, p.Log)

	// Default namespace
	p.defaultNs, err = p.sysHandler.GetCurrentNamespace()
	if err != nil {
		return errors.Errorf("failed to init default namespace: %v", err)
	}

	// Microservice descriptor
	p.msDescriptor, err = descriptor.NewMicroserviceDescriptor(p.KVScheduler, p.Log)
	if err != nil {
		return err
	}
	p.KVScheduler.RegisterKVDescriptor(p.msDescriptor.GetDescriptor())
	p.msDescriptor.StartTracker()

	p.Log.Infof("Namespace plugin initialized")

	return nil
}

// Close stops microservice tracker
func (p *NsPlugin) Close() error {
	if p.disabled {
		return nil
	}
	p.msDescriptor.StopTracker()

	return nil
}

// GetNamespaceHandle returns low-level run-time handle for the given namespace
// to be used with Netlink API. Do not forget to eventually close the handle using
// the netns.NsHandle.Close() method.
func (p *NsPlugin) GetNamespaceHandle(ctx nsLinuxcalls.NamespaceMgmtCtx, namespace *nsmodel.NetNamespace) (handle netns.NsHandle, err error) {
	if p.disabled {
		return 0, errors.New("NsPlugin is disabled")
	}
	// Convert microservice namespace
	if namespace != nil && namespace.Type == nsmodel.NetNamespace_MICROSERVICE {
		// Convert namespace
		reference := namespace.Reference
		namespace = p.convertMicroserviceNsToPidNs(reference)
		if namespace == nil {
			return 0, &unavailableMicroserviceErr{label: reference}
		}
	}

	// Get network namespace file descriptor
	ns, err := p.getOrCreateNs(ctx, namespace)
	if err != nil {
		return 0, errors.Errorf("failed to get or create namespace (%v): %v", namespace, err)
	}

	return ns, nil
}

// SwitchToNamespace switches the network namespace of the current thread.
// Caller should eventually call the returned "revert" function in order to get back to the original
// network namespace (for example using "defer revert()").
func (p *NsPlugin) SwitchToNamespace(ctx nsLinuxcalls.NamespaceMgmtCtx, ns *nsmodel.NetNamespace) (revert func(), err error) {
	if p.disabled {
		return func() {}, errors.New("NsPlugin is disabled")
	}

	// Save the current network namespace.
	origns, err := netns.Get()
	if err != nil {
		return func() {}, err
	}

	// Get network namespace file descriptor.
	nsHandle, err := p.GetNamespaceHandle(ctx, ns)
	if err != nil {
		origns.Close()
		return func() {}, err
	}
	defer nsHandle.Close()

	// Lock the OS Thread so we don't accidentally switch namespaces later.
	ctx.LockOSThread()

	// Switch the namespace.
	l := p.Log.WithFields(logging.Fields{"ns": nsHandle.String(), "ns-fd": int(nsHandle)})
	if err := p.sysHandler.SetNamespace(nsHandle); err != nil {
		ctx.UnlockOSThread()
		origns.Close()
		l.Errorf("Failed to switch Linux network namespace (%v): %v", ns, err)
		return func() {}, err
	}

	return func() {
		l := p.Log.WithFields(logging.Fields{"orig-ns": origns.String(), "orig-ns-fd": int(origns)})
		if err := p.sysHandler.SetNamespace(origns); err != nil {
			l.Errorf("Failed to switch Linux network namespace: %v", err)
		}
		origns.Close()
		ctx.UnlockOSThread()
	}, nil
}

// retrieveConfig loads NsPlugin configuration file.
func (p *NsPlugin) retrieveConfig() (*Config, error) {
	config := &Config{}
	found, err := p.Cfg.LoadValue(config)
	if !found {
		p.Log.Debug("Linux NsPlugin config not found")
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p.Log.Debug("Linux NsPlugin config found")
	return config, err
}

// getOrCreateNs returns an existing Linux network namespace or creates a new one if it doesn't exist yet.
// It is, however, only possible to create "named" namespaces. For PID-based namespaces, process with
// the given PID must exists, otherwise the function returns an error.
func (p *NsPlugin) getOrCreateNs(ctx nsLinuxcalls.NamespaceMgmtCtx, ns *nsmodel.NetNamespace) (netns.NsHandle, error) {
	var nsHandle netns.NsHandle
	var err error

	if ns == nil {
		return p.sysHandler.DuplicateNamespaceHandle(p.defaultNs)
	}

	switch ns.Type {
	case nsmodel.NetNamespace_PID:
		pid, err := strconv.Atoi(ns.Reference)
		if err != nil {
			return netns.None(), errors.Errorf("failed to parse network namespace PID reference: %v", err)
		}
		nsHandle, err = p.sysHandler.GetNamespaceFromPid(int(pid))
		if err != nil {
			return netns.None(), errors.Errorf("failed to get namespace handle from PID: %v", err)
		}

	case nsmodel.NetNamespace_NSID:
		nsHandle, err = p.sysHandler.GetNamespaceFromName(ns.Reference)
		if err != nil {
			// Create named namespace if it doesn't exist yet.
			_, err = p.namedNsHandler.CreateNamedNetNs(ctx, ns.Reference)
			if err != nil {
				return netns.None(), errors.Errorf("failed to create named net namspace: %v", err)
			}
			nsHandle, err = p.sysHandler.GetNamespaceFromName(ns.Reference)
			if err != nil {
				return netns.None(), errors.Errorf("unable to get namespace by name")
			}
		}

	case nsmodel.NetNamespace_FD:
		if ns.Reference == "" {
			return p.sysHandler.DuplicateNamespaceHandle(p.defaultNs)
		}
		nsHandle, err = p.sysHandler.GetNamespaceFromPath(ns.Reference)
		if err != nil {
			return netns.None(), errors.Errorf("failed to get file %s from path: %v", ns.Reference, err)
		}

	case nsmodel.NetNamespace_MICROSERVICE:
		return netns.None(), errors.Errorf("unable to convert microservice label to PID at this level")

	default:
		return netns.None(), errors.Errorf("undefined network namespace reference")
	}

	return nsHandle, nil
}

// convertMicroserviceNsToPidNs converts microservice-referenced namespace into the PID-referenced namespace.
func (p *NsPlugin) convertMicroserviceNsToPidNs(microserviceLabel string) (pidNs *nsmodel.NetNamespace) {
	if microservice, found := p.msDescriptor.GetMicroserviceStateData(microserviceLabel); found {
		pidNamespace := &nsmodel.NetNamespace{}
		pidNamespace.Type = nsmodel.NetNamespace_PID
		pidNamespace.Reference = strconv.Itoa(microservice.PID)
		return pidNamespace
	}
	return nil
}
