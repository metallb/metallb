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

package main

import (
	"time"

	govppapi "git.fd.io/govpp.git/api"
	"github.com/ligato/cn-infra/agent"
	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/datasync/kvdbsync/local"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/logging/logrus"
	"github.com/ligato/cn-infra/utils/safeclose"
	"github.com/ligato/vpp-agent/plugins/govppmux"
	"github.com/ligato/vpp-agent/plugins/vpp"
	l2Api "github.com/ligato/vpp-agent/plugins/vpp/binapi/l2"
	"github.com/ligato/vpp-agent/plugins/vpp/model/l2"
	"log"
)

// *************************************************************************
// This file contains examples of GOVPP operations, conversion of a proto
// data to a binary api message and demonstration of sending the message
// to the VPP with:
//
// requestContext = goVppChannel.SendRequest(requestMessage)
// requestContext.ReceiveReply(replyMessage)
//
// Note: this example shows how to work with VPP, so real proto message
// structure is used (bridge domains).
// ************************************************************************/

// Main allows running Example Plugin as a statically linked binary with Agent Core Plugins. Close channel and plugins
// required for the example are initialized. Agent is instantiated with generic plugins (etcd, Kafka, Status check,
// HTTP and Log), and GOVPP, and resync plugin, and example plugin which demonstrates GOVPP call functionality.
func main() {
	//Init close channel to stop the example.
	closeChannel := make(chan struct{}, 1)
	// Prepare all the dependencies for example plugin
	watcher := datasync.KVProtoWatchers{
		local.Get(),
	}
	vppPlugin := vpp.NewPlugin(vpp.UseDeps(func(deps *vpp.Deps) {
		deps.Watcher = watcher
	}))

	// Inject dependencies to example plugin
	ep := &ExamplePlugin{
		Log:          logrus.DefaultLogger(),
		closeChannel: closeChannel,
	}
	ep.Deps.VPP = vppPlugin
	ep.Deps.GoVppMux = &govppmux.DefaultPlugin

	// Start Agent
	a := agent.NewAgent(
		agent.AllPlugins(ep),
		agent.QuitOnClose(closeChannel),
	)
	if err := a.Run(); err != nil {
		log.Fatal()
	}
}

// PluginName represents name of plugin.
const PluginName = "govpp-example"

// ExamplePlugin implements Plugin interface which is used to pass custom plugin instances to the Agent.
type ExamplePlugin struct {
	Deps

	exampleIDSeq uint32           // Plugin-specific ID initialization
	vppChannel   govppapi.Channel // Vpp channel to communicate with VPP
	// Fields below are used to properly finish the example.
	closeChannel chan struct{}
	Log          logging.Logger
}

// Deps is example plugin dependencies.
type Deps struct {
	GoVppMux *govppmux.Plugin
	VPP      *vpp.Plugin
}

// Init members of plugin.
func (plugin *ExamplePlugin) Init() (err error) {
	// NewAPIChannel returns a new API channel for communication with VPP via govpp core.
	// It uses default buffer sizes for the request and reply Go channels.
	plugin.vppChannel, err = plugin.Deps.GoVppMux.NewAPIChannel()

	plugin.Log.Info("Default plugin plugin ready")

	plugin.VPP.DisableResync(l2.BdPrefix)

	// Make VPP call
	go plugin.VppCall()

	return err
}

// Close is called by Agent Core when the Agent is shutting down. It is supposed
// to clean up resources that were allocated by the plugin during its lifetime.
func (plugin *ExamplePlugin) Close() error {
	return safeclose.Close(plugin.vppChannel)
}

// String returns plugin name
func (plugin *ExamplePlugin) String() string {
	return PluginName
}

/***********
 * VPPCall *
 ***********/

// VppCall uses created data to convert it to the binary api call. In the example,
// a bridge domain data are built and transformed to the BridgeDomainAddDel binary api call
// which is then sent to the VPP.
func (plugin *ExamplePlugin) VppCall() {
	time.Sleep(3 * time.Second)

	// Prepare a simple data.
	plugin.Log.Info("Preparing data ...")
	bds1 := buildData("br1")
	bds2 := buildData("br2")
	bds3 := buildData("br3")

	// Prepare binary api message from the data.
	req1 := buildBinapiMessage(bds1, plugin.exampleIDSeq)
	plugin.exampleIDSeq++ // Change (raise) index to ensure every message uses unique ID.
	req2 := buildBinapiMessage(bds2, plugin.exampleIDSeq)
	plugin.exampleIDSeq++
	req3 := buildBinapiMessage(bds3, plugin.exampleIDSeq)
	plugin.exampleIDSeq++

	// Generic bin api reply (request: BridgeDomainAddDel)
	reply := &l2Api.BridgeDomainAddDelReply{}

	plugin.Log.Info("Sending data to VPP ...")

	// 1. Send the request and receive a reply directly (in one line).
	plugin.vppChannel.SendRequest(req1).ReceiveReply(reply)

	// 2. Send multiple different requests. Every request returns it's own request context.
	reqCtx2 := plugin.vppChannel.SendRequest(req2)
	reqCtx3 := plugin.vppChannel.SendRequest(req3)
	// The context can be used later to get reply.
	reqCtx2.ReceiveReply(reply)
	reqCtx3.ReceiveReply(reply)

	plugin.Log.Info("Data successfully sent to VPP")
	// End the example.
	plugin.Log.Infof("etcd/datasync example finished, sending shutdown ...")
	close(plugin.closeChannel)
}

// Auxiliary function to build bridge domain data
func buildData(name string) *l2.BridgeDomains {
	return &l2.BridgeDomains{
		BridgeDomains: []*l2.BridgeDomains_BridgeDomain{
			{
				Name:                name,
				Flood:               false,
				UnknownUnicastFlood: true,
				Forward:             true,
				Learn:               true,
				ArpTermination:      true,
				MacAge:              0,
				Interfaces: []*l2.BridgeDomains_BridgeDomain_Interfaces{
					{
						Name: "memif1",
					},
				},
			},
		},
	}
}

// Auxiliary method to transform agent model data to binary api format
func buildBinapiMessage(data *l2.BridgeDomains, id uint32) *l2Api.BridgeDomainAddDel {
	req := &l2Api.BridgeDomainAddDel{}
	req.IsAdd = 1
	req.BdID = id
	req.Flood = boolToInt(data.BridgeDomains[0].Flood)
	req.UuFlood = boolToInt(data.BridgeDomains[0].UnknownUnicastFlood)
	req.Forward = boolToInt(data.BridgeDomains[0].Forward)
	req.Learn = boolToInt(data.BridgeDomains[0].Learn)
	req.ArpTerm = boolToInt(data.BridgeDomains[0].ArpTermination)
	req.MacAge = uint8(data.BridgeDomains[0].MacAge)

	return req
}

func boolToInt(input bool) uint8 {
	if input {
		return uint8(1)
	}
	return uint8(0)
}
