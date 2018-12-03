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

package l3plugin

import (
	govppapi "git.fd.io/govpp.git/api"
	"github.com/go-errors/errors"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/utils/safeclose"
	"github.com/ligato/vpp-agent/plugins/govppmux"
	"github.com/ligato/vpp-agent/plugins/vpp/l3plugin/vppcalls"
	"github.com/ligato/vpp-agent/plugins/vpp/model/l3"
)

// IPNeighConfigurator runs in the background in its own goroutine where it watches for any changes
// in the configuration of L3 IP scan neighbor, as modelled by the proto file "../model/l3/l3.proto" and stored
// in ETCD under the key "/vnf-agent/{vnf-agent}/vpp/config/v1/ipneigh". Configuration uses single key, since
// the configuration is global-like.
type IPNeighConfigurator struct {
	log logging.Logger

	// VPP channel
	vppChan govppapi.Channel
	// VPP API channel
	ipNeighHandler vppcalls.IPNeighVppAPI
}

// Init VPP channel and vppcalls handler
func (c *IPNeighConfigurator) Init(logger logging.PluginLogger, goVppMux govppmux.API) (err error) {
	// Logger
	c.log = logger.NewLogger("l3-ip-neigh-conf")
	c.log.Debugf("Initializing proxy ARP configurator")

	// VPP channel
	if c.vppChan, err = goVppMux.NewAPIChannel(); err != nil {
		return errors.Errorf("failed to create API channel: %v", err)
	}

	// VPP API handler
	c.ipNeighHandler = vppcalls.NewIPNeighVppHandler(c.vppChan, c.log)

	return nil
}

// Close VPP channel
func (c *IPNeighConfigurator) Close() error {
	if err := safeclose.Close(c.vppChan); err != nil {
		return c.LogError(errors.Errorf("failed to safeclose IP neighbor configurator: %v", err))
	}
	return nil
}

// Set puts desired IP scan neighbor configuration to the VPP
func (c *IPNeighConfigurator) Set(config *l3.IPScanNeighbor) error {
	if err := c.ipNeighHandler.SetIPScanNeighbor(config); err != nil {
		return errors.Errorf("failed to set IP neighbor: %v", err)
	}

	c.log.Infof("IP scan neighbor set to %v", config.Mode)

	return nil
}

// Unset returns IP scan neighbor configuration to default
func (c *IPNeighConfigurator) Unset() error {
	defaultCfg := &l3.IPScanNeighbor{
		Mode: l3.IPScanNeighbor_DISABLED,
	}

	if err := c.ipNeighHandler.SetIPScanNeighbor(defaultCfg); err != nil {
		return errors.Errorf("failed to set IP neighbor to default: %v", err)
	}

	c.log.Info("IP scan neighbor set to default")

	return nil
}

// LogError prints error if not nil, including stack trace. The same value is also returned, so it can be easily propagated further
func (c *IPNeighConfigurator) LogError(err error) error {
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
