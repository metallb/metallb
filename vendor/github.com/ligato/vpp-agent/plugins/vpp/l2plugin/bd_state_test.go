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

package l2plugin_test

import (
	"testing"

	"git.fd.io/govpp.git/core"

	"git.fd.io/govpp.git/adapter/mock"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/logging/logrus"
	"github.com/ligato/vpp-agent/idxvpp/nametoidx"
	l2api "github.com/ligato/vpp-agent/plugins/vpp/binapi/l2"
	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin/ifaceidx"
	"github.com/ligato/vpp-agent/plugins/vpp/l2plugin"
	"github.com/ligato/vpp-agent/plugins/vpp/l2plugin/l2idx"
	intf "github.com/ligato/vpp-agent/plugins/vpp/model/interfaces"
	"github.com/ligato/vpp-agent/plugins/vpp/model/l2"
	"github.com/ligato/vpp-agent/tests/vppcallmock"
	. "github.com/onsi/gomega"
	"golang.org/x/net/context"
)

func bdStateTestInitialization(t *testing.T) (*l2plugin.BridgeDomainStateUpdater, *core.Connection, l2idx.BDIndexRW, ifaceidx.SwIfIndexRW, chan l2plugin.BridgeDomainStateMessage, chan *l2plugin.BridgeDomainStateNotification) {
	RegisterTestingT(t)

	// Initialize notification channel
	notifChan := make(chan l2plugin.BridgeDomainStateMessage, 100)

	// Initialize index
	nameToIdx := nametoidx.NewNameToIdx(logrus.DefaultLogger(), "bd_index_test", l2idx.IndexMetadata)
	index := l2idx.NewBDIndex(nameToIdx)
	names := nameToIdx.ListNames()

	// Check if names were empty
	Expect(names).To(BeEmpty())

	// Initialize sw if index
	nameToIdxSW := nametoidx.NewNameToIdx(logrus.DefaultLogger(), "ifaceidx_test", ifaceidx.IndexMetadata)
	swIfIndex := ifaceidx.NewSwIfIndex(nameToIdxSW)
	names = nameToIdxSW.ListNames()

	// Check if names were empty
	Expect(names).To(BeEmpty())

	// Create publish state function
	publishChan := make(chan *l2plugin.BridgeDomainStateNotification, 100)
	publishIfState := func(notification *l2plugin.BridgeDomainStateNotification) {
		t.Logf("Received notification change %v", notification)
		publishChan <- notification
	}

	// Create context
	ctx, _ := context.WithCancel(context.Background())

	// Create connection
	mockCtx := &vppcallmock.TestCtx{
		MockVpp: mock.NewVppAdapter(),
	}
	connection, err := core.Connect(mockCtx.MockVpp)
	Expect(err).To(BeNil())

	// Create plugin logger
	pluginLogger := logging.ForPlugin("testname")

	// Test initialization
	plugin := &l2plugin.BridgeDomainStateUpdater{}
	err = plugin.Init(ctx, pluginLogger, connection, index, swIfIndex, notifChan, publishIfState)
	Expect(err).To(BeNil())

	return plugin, connection, index, swIfIndex, notifChan, publishChan
}

func bdStateTestTeardown(plugin *l2plugin.BridgeDomainStateUpdater, conn *core.Connection) {
	conn.Disconnect()
	logging.DefaultRegistry.ClearRegistry()
}

// Tests notification processing in bridge domain state updater with zero index
func TestBridgeDomainStateUpdater_watchVppNotificationsZero(t *testing.T) {
	plugin, conn, _, _, notifChan, publishChan := bdStateTestInitialization(t)
	defer bdStateTestTeardown(plugin, conn)

	// Test notifications
	notifChan <- l2plugin.BridgeDomainStateMessage{
		Name: "test",
		Message: &l2api.BridgeDomainDetails{
			BdID: 0,
		},
	}

	var notif *l2plugin.BridgeDomainStateNotification

	Eventually(publishChan).Should(Receive(&notif))
}

// Tests notification processing in bridge domain state updater with zero index and no name (invalid)
func TestBridgeDomainStateUpdater_watchVppNotificationsZeroNoName(t *testing.T) {
	plugin, conn, _, _, notifChan, publishChan := bdStateTestInitialization(t)
	defer bdStateTestTeardown(plugin, conn)

	// Test notifications
	notifChan <- l2plugin.BridgeDomainStateMessage{
		Message: &l2api.BridgeDomainDetails{
			BdID: 0,
		},
	}

	var notif *l2plugin.BridgeDomainStateNotification

	Eventually(publishChan).ShouldNot(Receive(&notif))
}

// Tests notification processing in bridge domain state updater
func TestBridgeDomainStateUpdater_watchVppNotifications(t *testing.T) {
	plugin, conn, bdIndex, swIfIndex, notifChan, publishChan := bdStateTestInitialization(t)
	defer bdStateTestTeardown(plugin, conn)

	// Register interface name
	swIfIndex.RegisterName("test", 1, &intf.Interfaces_Interface{
		Name:        "test",
		Enabled:     true,
		Type:        intf.InterfaceType_MEMORY_INTERFACE,
		IpAddresses: []string{"192.168.0.1/24"},
	})

	// Register bridge domain name
	bdIndex.RegisterName("bdTest", 1, &l2idx.BdMetadata{
		ConfiguredInterfaces: []string{"test"},
		BridgeDomain:         &l2.BridgeDomains_BridgeDomain{},
	})

	// Test notifications
	notifChan <- l2plugin.BridgeDomainStateMessage{
		Name: "test",
		Message: &l2api.BridgeDomainDetails{
			BdID:         1,
			Flood:        1,
			UuFlood:      2,
			Forward:      3,
			Learn:        4,
			ArpTerm:      5,
			MacAge:       6,
			BdTag:        []byte("test"),
			BviSwIfIndex: 1,
			NSwIfs:       1,
			SwIfDetails: []l2api.BridgeDomainSwIf{
				{
					SwIfIndex: 1,
					Context:   0,
					Shg:       20,
				},
			},
		},
	}

	var notif *l2plugin.BridgeDomainStateNotification
	Eventually(publishChan).Should(Receive(&notif))
	Expect(notif.State).To(Not(BeNil()))
	Expect(notif.State.BviInterface).To(Equal("test"))
	Expect(notif.State.BviInterfaceIndex).To(BeEquivalentTo(1))
	Expect(notif.State.InterfaceCount).To(BeEquivalentTo(1))
	Expect(notif.State.Interfaces).To(Not(BeEmpty()))
	Expect(notif.State.Interfaces[0].Name).To(Equal("test"))
	Expect(notif.State.Interfaces[0].SplitHorizonGroup).To(BeEquivalentTo(20))
}
