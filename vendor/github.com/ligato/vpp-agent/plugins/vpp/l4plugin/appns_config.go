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

package l4plugin

import (
	govppapi "git.fd.io/govpp.git/api"
	"github.com/go-errors/errors"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/utils/safeclose"
	"github.com/ligato/vpp-agent/idxvpp/nametoidx"
	"github.com/ligato/vpp-agent/plugins/govppmux"
	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin/ifaceidx"
	"github.com/ligato/vpp-agent/plugins/vpp/l4plugin/nsidx"
	"github.com/ligato/vpp-agent/plugins/vpp/l4plugin/vppcalls"
	"github.com/ligato/vpp-agent/plugins/vpp/model/l4"
)

// AppNsConfigurator runs in the background in its own goroutine where it watches for any changes
// in the configuration of interfaces as modelled by the proto file "../model/l4/l4.proto"
// and stored in ETCD under the keys "/vnf-agent/{vnf-agent}/vpp/config/v1/l4/l4ftEnabled"
// and "/vnf-agent/{vnf-agent}/vpp/config/v1/l4/namespaces/{namespace_id}".
// Updates received from the northbound API are compared with the VPP run-time configuration and differences
// are applied through the VPP binary API.
type AppNsConfigurator struct {
	log logging.Logger

	// In-memory mappings
	ifIndexes    ifaceidx.SwIfIndex
	appNsIndexes nsidx.AppNsIndexRW
	appNsCached  nsidx.AppNsIndexRW // the mapping stores not-configurable app namespaces with metadata
	appNsIdxSeq  uint32

	// VPP channel
	vppChan govppapi.Channel
	// VPP API handler
	l4Handler vppcalls.L4VppAPI

	// Feature flag - internal state whether the L4 features are enabled or disabled
	l4ftEnabled bool
}

// Init members (channels...) and start go routines
func (c *AppNsConfigurator) Init(logger logging.PluginLogger, goVppMux govppmux.API, swIfIndexes ifaceidx.SwIfIndex) (err error) {
	// Logger
	c.log = logger.NewLogger("l4-plugin")

	// Mappings
	c.ifIndexes = swIfIndexes
	c.appNsIndexes = nsidx.NewAppNsIndex(nametoidx.NewNameToIdx(c.log, "namespace_indexes", nil))
	c.appNsCached = nsidx.NewAppNsIndex(nametoidx.NewNameToIdx(c.log, "not_configured_namespace_indexes", nil))
	c.appNsIdxSeq = 1

	// VPP channels
	if c.vppChan, err = goVppMux.NewAPIChannel(); err != nil {
		return errors.Errorf("failed to create API channel: %v", err)
	}

	// VPP API handler
	c.l4Handler = vppcalls.NewL4VppHandler(c.vppChan, c.log)

	c.log.Debugf("L4 configurator initialized")

	return nil
}

// Close members, channels
func (c *AppNsConfigurator) Close() error {
	if err := safeclose.Close(c.vppChan); err != nil {
		return c.LogError(errors.Errorf("failed to safeclose l4 configurator: %v", err))
	}
	return nil
}

// clearMapping prepares all in-memory-mappings and other cache fields. All previous cached entries are removed.
func (c *AppNsConfigurator) clearMapping() {
	c.appNsIndexes.Clear()
	c.appNsCached.Clear()
	c.log.Debugf("l4 configurator mapping cleared")
}

// GetAppNsIndexes returns application namespace memory indexes
func (c *AppNsConfigurator) GetAppNsIndexes() nsidx.AppNsIndexRW {
	return c.appNsIndexes
}

// ConfigureL4FeatureFlag process the NB Features config and propagates it to bin api calls
func (c *AppNsConfigurator) ConfigureL4FeatureFlag(features *l4.L4Features) error {
	if features.Enabled {
		if err := c.configureL4FeatureFlag(); err != nil {
			return err
		}
		return c.resolveCachedNamespaces()
	}
	c.DeleteL4FeatureFlag()

	return nil
}

// configureL4FeatureFlag process the NB Features config and propagates it to bin api calls
func (c *AppNsConfigurator) configureL4FeatureFlag() error {
	if err := c.l4Handler.EnableL4Features(); err != nil {
		return errors.Errorf("failed to enable L4 features: %v", err)
	}
	c.l4ftEnabled = true
	c.log.Infof("L4 features enabled")

	return nil
}

// DeleteL4FeatureFlag process the NB Features config and propagates it to bin api calls
func (c *AppNsConfigurator) DeleteL4FeatureFlag() error {
	if err := c.l4Handler.DisableL4Features(); err != nil {
		return errors.Errorf("failed to disable L4 features: %v", err)
	}

	c.l4ftEnabled = false
	c.log.Infof("L4 features disabled")

	return nil
}

// ConfigureAppNamespace process the NB AppNamespace config and propagates it to bin api calls
func (c *AppNsConfigurator) ConfigureAppNamespace(ns *l4.AppNamespaces_AppNamespace) error {
	// Validate data
	if ns.Interface == "" {
		return errors.Errorf("application namespace %s does not contain interface", ns.NamespaceId)
	}

	// Check whether L4 l4ftEnabled are enabled. If not, all namespaces created earlier are added to cache
	if !c.l4ftEnabled {
		c.appNsCached.RegisterName(ns.NamespaceId, c.appNsIdxSeq, ns)
		c.appNsIdxSeq++
		c.log.Debugf("cannot configure application namespace %s: L4 features are disabled, moved to cache",
			ns.NamespaceId)
		return nil
	}

	// Find interface. If not found, add to cache for not configured namespaces
	ifIdx, _, found := c.ifIndexes.LookupIdx(ns.Interface)
	if !found {
		c.appNsCached.RegisterName(ns.NamespaceId, c.appNsIdxSeq, ns)
		c.appNsIdxSeq++
		c.log.Infof("cannot configure application namespace %s: interface %s is missing",
			ns.NamespaceId, ns.Interface)
		return nil
	}

	if err := c.configureAppNamespace(ns, ifIdx); err != nil {
		return err
	}

	c.log.Info("application namespace %s configured", ns.NamespaceId)

	return nil
}

// ModifyAppNamespace process the NB AppNamespace config and propagates it to bin api calls
func (c *AppNsConfigurator) ModifyAppNamespace(newNs *l4.AppNamespaces_AppNamespace, oldNs *l4.AppNamespaces_AppNamespace) error {
	// Validate data
	if newNs.Interface == "" {
		return errors.Errorf("modified application namespace %s does not contain interface", newNs.NamespaceId)
	}

	// At first, unregister the old configuration from both mappings (if exists)
	c.appNsIndexes.UnregisterName(oldNs.NamespaceId)
	c.appNsCached.UnregisterName(oldNs.NamespaceId)
	c.log.Debugf("application namespace %s removed from index map and cache", oldNs.NamespaceId)

	// Check whether L4 l4ftEnabled are enabled. If not, all namespaces created earlier are added to cache
	if !c.l4ftEnabled {
		c.appNsCached.RegisterName(newNs.NamespaceId, c.appNsIdxSeq, newNs)
		c.appNsIdxSeq++
		c.log.Debugf("cannot modify application namespace %s: L4 features are disabled, moved to cache",
			newNs.NamespaceId)
		return nil
	}

	// Check interface
	ifIdx, _, found := c.ifIndexes.LookupIdx(newNs.Interface)
	if !found {
		c.appNsCached.RegisterName(newNs.NamespaceId, c.appNsIdxSeq, newNs)
		c.appNsIdxSeq++
		c.log.Infof("cannot modify application namespace %s: interface %s is missing",
			newNs.NamespaceId, newNs.Interface)
		return nil
	}

	// TODO: remove namespace
	if err := c.configureAppNamespace(newNs, ifIdx); err != nil {
		return err
	}

	c.log.Info("application namespace %s modified", newNs.NamespaceId)

	return nil
}

// DeleteAppNamespace process the NB AppNamespace config and propagates it to bin api calls. This case is not currently
// supported by VPP
func (c *AppNsConfigurator) DeleteAppNamespace(ns *l4.AppNamespaces_AppNamespace) error {
	// TODO: implement
	c.log.Warn("cannot remove application namespace %s: unsupported", ns.NamespaceId)
	return nil
}

// ResolveCreatedInterface looks for application namespace this interface is assigned to and configures them
func (c *AppNsConfigurator) ResolveCreatedInterface(ifName string, ifIdx uint32) error {
	// If L4 features are not enabled, skip (and keep all in cache)
	if !c.l4ftEnabled {
		return nil
	}

	// Search mapping for unregistered application namespaces using the new interface
	cachedAppNs := c.appNsCached.LookupNamesByInterface(ifName)
	if len(cachedAppNs) == 0 {
		return nil
	}

	for _, appNs := range cachedAppNs {
		if err := c.configureAppNamespace(appNs, ifIdx); err != nil {
			return errors.Errorf("failed to configure application namespace %s with registered interface %s: %v",
				appNs.NamespaceId, ifName, err)
		}
		// Remove from cache
		c.appNsCached.UnregisterName(appNs.NamespaceId)
		c.log.Debugf("application namespace %s removed from cache", appNs.NamespaceId)
	}
	return nil
}

// ResolveDeletedInterface looks for application namespace this interface is assigned to and removes
func (c *AppNsConfigurator) ResolveDeletedInterface(ifName string, ifIdx uint32) error {
	// Search mapping for configured application namespaces using the new interface
	cachedAppNs := c.appNsIndexes.LookupNamesByInterface(ifName)
	if len(cachedAppNs) == 0 {
		return nil
	}
	for _, appNs := range cachedAppNs {
		// TODO: remove namespace. Also check whether it can be done while L4Features are disabled
		// Unregister from configured namespaces mapping
		c.appNsIndexes.UnregisterName(appNs.NamespaceId)
		// Add to un-configured. If the interface will be recreated, all namespaces are configured back
		c.appNsCached.RegisterName(appNs.NamespaceId, c.appNsIdxSeq, appNs)
		c.log.Debugf("application namespace %s removed from mapping and added to cache (unregistered interface %s)",
			appNs.NamespaceId, ifName)
		c.appNsIdxSeq++
	}

	return nil
}

func (c *AppNsConfigurator) configureAppNamespace(ns *l4.AppNamespaces_AppNamespace, ifIdx uint32) error {
	// Namespace ID
	nsID := []byte(ns.NamespaceId)

	appNsIdx, err := c.l4Handler.AddAppNamespace(ns.Secret, ifIdx, ns.Ipv4FibId, ns.Ipv6FibId, nsID)
	if err != nil {
		return errors.Errorf("failed to add application namespace %s: %v", ns.NamespaceId, err)
	}

	// register namespace
	c.appNsIndexes.RegisterName(ns.NamespaceId, appNsIdx, ns)
	c.log.Debugf("Application namespace %s registered", ns.NamespaceId)

	return nil
}

// An application namespace can be cached from two reasons:
// 		- the required interface was missing
//      - the L4 features were disabled
// Namespaces skipped due to the second case are configured here
func (c *AppNsConfigurator) resolveCachedNamespaces() error {
	cachedAppNs := c.appNsCached.ListNames()
	if len(cachedAppNs) == 0 {
		return nil
	}

	c.log.Debugf("Configuring %d cached namespaces after L4 features were enabled", len(cachedAppNs))

	// Scan all registered indexes in mapping for un-configured application namespaces
	for _, name := range cachedAppNs {
		_, ns, found := c.appNsCached.LookupIdx(name)
		if !found {
			continue
		}

		// Check interface. If still missing, continue (keep namespace in cache)
		ifIdx, _, found := c.ifIndexes.LookupIdx(ns.Interface)
		if !found {
			continue
		}

		if err := c.configureAppNamespace(ns, ifIdx); err != nil {
			return err
		}
		// AppNamespace was configured, remove from cache
		c.appNsCached.UnregisterName(ns.NamespaceId)
		c.log.Debugf("Application namespace %s unregistered from cache", ns.NamespaceId)
	}

	return nil
}

// LogError prints error if not nil, including stack trace. The same value is also returned, so it can be easily propagated further
func (c *AppNsConfigurator) LogError(err error) error {
	if err == nil {
		return nil
	}
	switch err.(type) {
	case *errors.Error:
		c.log.WithField("logger", c.log).Errorf(string(err.Error() + "\n" + string(err.(*errors.Error).Stack())))
	default:
		c.log.Error(err)
	}
	return err
}
