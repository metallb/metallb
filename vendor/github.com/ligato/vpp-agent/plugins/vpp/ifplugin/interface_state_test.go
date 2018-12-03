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

package ifplugin_test

import (
	"net"
	"testing"

	"git.fd.io/govpp.git/core"

	"git.fd.io/govpp.git/adapter/mock"
	govppapi "git.fd.io/govpp.git/api"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/logging/logrus"
	"github.com/ligato/vpp-agent/idxvpp/nametoidx"
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/interfaces"
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/stats"
	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin"
	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin/ifaceidx"
	intf "github.com/ligato/vpp-agent/plugins/vpp/model/interfaces"
	"github.com/ligato/vpp-agent/tests/vppcallmock"
	. "github.com/onsi/gomega"
	"golang.org/x/net/context"
)

func testPluginDataInitialization(t *testing.T) (*core.Connection, ifaceidx.SwIfIndexRW, *ifplugin.InterfaceStateUpdater,
	chan govppapi.Message, chan *intf.InterfaceNotification) {
	RegisterTestingT(t)

	// Initialize notification channel
	notifChan := make(chan govppapi.Message, 100)

	// Initialize index
	nameToIdx := nametoidx.NewNameToIdx(logrus.DefaultLogger(), "interface_state_test", ifaceidx.IndexMetadata)
	index := ifaceidx.NewSwIfIndex(nameToIdx)
	names := nameToIdx.ListNames()
	Expect(names).To(BeEmpty())

	// Create publish state function
	publishChan := make(chan *intf.InterfaceNotification, 100)
	publishIfState := func(notification *intf.InterfaceNotification) {
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

	// Prepare Init VPP replies
	mockCtx.MockVpp.MockReply(&interfaces.WantInterfaceEventsReply{})
	mockCtx.MockVpp.MockReply(&stats.WantStatsReply{})

	// Create plugin logger
	pluginLogger := logging.ForPlugin("testname")

	// Test initialization
	ifPlugin := &ifplugin.InterfaceStateUpdater{}
	err = ifPlugin.Init(ctx, pluginLogger, connection, index, notifChan, publishIfState)
	Expect(err).To(BeNil())
	err = ifPlugin.AfterInit()
	Expect(err).To(BeNil())

	return connection, index, ifPlugin, notifChan, publishChan
}

func testPluginDataTeardown(plugin *ifplugin.InterfaceStateUpdater, connection *core.Connection) {
	connection.Disconnect()
	Expect(plugin.Close()).To(BeNil())
	logging.DefaultRegistry.ClearRegistry()
}

// Test UPDOWN notification
func TestInterfaceStateUpdaterUpDownNotif(t *testing.T) {
	conn, index, ifPlugin, notifChan, publishChan := testPluginDataInitialization(t)
	defer testPluginDataTeardown(ifPlugin, conn)

	// Register name
	index.RegisterName("test", 0, &intf.Interfaces_Interface{
		Name:        "test",
		Enabled:     true,
		Type:        intf.InterfaceType_MEMORY_INTERFACE,
		IpAddresses: []string{"192.168.0.1/24"},
	})

	// Test notifications
	notifChan <- &interfaces.SwInterfaceEvent{
		PID:         0,
		SwIfIndex:   0,
		AdminUpDown: 1,
		LinkUpDown:  1,
		Deleted:     0,
	}

	var notif *intf.InterfaceNotification

	Eventually(publishChan).Should(Receive(&notif))
	Expect(notif.Type).To(Equal(intf.InterfaceNotification_UPDOWN))
	Expect(notif.State.AdminStatus).Should(BeEquivalentTo(intf.InterfacesState_Interface_UP))
}

// Test simple counter notification
func TestInterfaceStateUpdaterVnetSimpleCounterNotif(t *testing.T) {
	conn, index, ifPlugin, notifChan, publishChan := testPluginDataInitialization(t)
	defer testPluginDataTeardown(ifPlugin, conn)

	// Register name
	index.RegisterName("test", 0, &intf.Interfaces_Interface{
		Name:        "test",
		Enabled:     true,
		Type:        intf.InterfaceType_MEMORY_INTERFACE,
		IpAddresses: []string{"192.168.0.1/24"},
	})

	// Test notifications
	notifChan <- &stats.VnetInterfaceSimpleCounters{
		VnetCounterType: 0, // Drop,
		Count:           1,
		FirstSwIfIndex:  0,
		Data:            []uint64{32768},
	}

	notifChan <- &stats.VnetInterfaceSimpleCounters{
		VnetCounterType: 1, // Punt,
		Count:           1,
		FirstSwIfIndex:  0,
		Data:            []uint64{32769},
	}
	notifChan <- &stats.VnetInterfaceSimpleCounters{
		VnetCounterType: 2, // Ipv4,
		Count:           1,
		FirstSwIfIndex:  0,
		Data:            []uint64{32770},
	}
	notifChan <- &stats.VnetInterfaceSimpleCounters{
		VnetCounterType: 3, // Ipv6,
		Count:           3,
		FirstSwIfIndex:  0,
		Data:            []uint64{32771},
	}
	notifChan <- &stats.VnetInterfaceSimpleCounters{
		VnetCounterType: 4, // RxNoBuf,
		Count:           1,
		FirstSwIfIndex:  0,
		Data:            []uint64{32772},
	}
	notifChan <- &stats.VnetInterfaceSimpleCounters{
		VnetCounterType: 5, // RxMiss,
		Count:           1,
		FirstSwIfIndex:  0,
		Data:            []uint64{32773},
	}
	notifChan <- &stats.VnetInterfaceSimpleCounters{
		VnetCounterType: 6, // RxError,
		Count:           1,
		FirstSwIfIndex:  0,
		Data:            []uint64{32774},
	}
	notifChan <- &stats.VnetInterfaceSimpleCounters{
		VnetCounterType: 7, // TxError,
		Count:           1,
		FirstSwIfIndex:  0,
		Data:            []uint64{32775},
	}

	// Send interface event notification to propagate update from counter to publish channel
	notifChan <- &interfaces.SwInterfaceEvent{
		PID:         0,
		SwIfIndex:   0,
		AdminUpDown: 1,
		LinkUpDown:  1,
		Deleted:     0,
	}

	var notif *intf.InterfaceNotification
	Eventually(publishChan).Should(Receive(&notif))
	Expect(notif.Type).To(Equal(intf.InterfaceNotification_UPDOWN))
	Expect(notif.State.AdminStatus).Should(BeEquivalentTo(intf.InterfacesState_Interface_UP))
	Expect(notif.State.Statistics).To(BeEquivalentTo(&intf.InterfacesState_Interface_Statistics{
		DropPackets:     32768,
		PuntPackets:     32769,
		Ipv4Packets:     32770,
		Ipv6Packets:     32771,
		InNobufPackets:  32772,
		InMissPackets:   32773,
		InErrorPackets:  32774,
		OutErrorPackets: 32775,
	}))
	Expect(notif.State.Statistics.DropPackets).Should(BeEquivalentTo(32768))
	Expect(notif.State.Statistics.PuntPackets).Should(BeEquivalentTo(32769))
	Expect(notif.State.Statistics.Ipv4Packets).Should(BeEquivalentTo(32770))
	Expect(notif.State.Statistics.Ipv6Packets).Should(BeEquivalentTo(32771))
	Expect(notif.State.Statistics.InNobufPackets).Should(BeEquivalentTo(32772))
	Expect(notif.State.Statistics.InMissPackets).Should(BeEquivalentTo(32773))
	Expect(notif.State.Statistics.InErrorPackets).Should(BeEquivalentTo(32774))
	Expect(notif.State.Statistics.OutErrorPackets).Should(BeEquivalentTo(32775))
}

// Test VnetIntCombined notification
func TestInterfaceStateUpdaterVnetIntCombinedNotif(t *testing.T) {
	conn, index, ifPlugin, notifChan, publishChan := testPluginDataInitialization(t)
	defer testPluginDataTeardown(ifPlugin, conn)

	// Register name
	index.RegisterName("test0", 0, &intf.Interfaces_Interface{
		Name:        "test0",
		Enabled:     true,
		Type:        intf.InterfaceType_MEMORY_INTERFACE,
		IpAddresses: []string{"192.168.0.1/24"},
	})

	index.RegisterName("test1", 1, &intf.Interfaces_Interface{
		Name:        "test1",
		Enabled:     true,
		Type:        intf.InterfaceType_MEMORY_INTERFACE,
		IpAddresses: []string{"192.168.0.2/24"},
	})

	// Test notifications
	notifChan <- &stats.VnetInterfaceCombinedCounters{
		VnetCounterType: 1, // TX
		FirstSwIfIndex:  0,
		Count:           2,
		Data: []stats.VlibCounter{
			{Packets: 1000, Bytes: 3000},
			{Packets: 2000, Bytes: 5000},
		},
	}

	var notif *intf.InterfaceNotification
	var outPackets, outBytes uint64

	Eventually(publishChan).Should(Receive(&notif))
	Expect(notif.Type).To(Equal(intf.InterfaceNotification_UPDOWN))
	outPackets += notif.GetState().GetStatistics().GetOutPackets()
	outBytes += notif.GetState().GetStatistics().GetOutBytes()

	Eventually(publishChan).Should(Receive(&notif))
	Expect(notif.Type).To(Equal(intf.InterfaceNotification_UPDOWN))
	outPackets += notif.GetState().GetStatistics().GetOutPackets()
	outBytes += notif.GetState().GetStatistics().GetOutBytes()

	// NOTE: Notifications from publishChan cannot gurantee
	// to be in same order as the ones going into notifChan.
	Expect(outPackets).Should(BeEquivalentTo(3000))
	Expect(outBytes).Should(BeEquivalentTo(8000))
}

// Test SwInterfaceDetails notification
func TestInterfaceStateUpdaterSwInterfaceDetailsNotif(t *testing.T) {
	conn, index, ifPlugin, notifChan, publishChan := testPluginDataInitialization(t)
	defer testPluginDataTeardown(ifPlugin, conn)

	// Register name
	index.RegisterName("test", 0, &intf.Interfaces_Interface{
		Name:        "test",
		Enabled:     true,
		Type:        intf.InterfaceType_MEMORY_INTERFACE,
		IpAddresses: []string{"192.168.0.1/24"},
	})

	// Test notifications
	hwAddr1Parse, err := net.ParseMAC("01:23:45:67:89:ab")
	Expect(err).To(BeNil())

	notifChan <- &interfaces.SwInterfaceDetails{
		InterfaceName:   []byte("if0"),
		AdminUpDown:     1,    // adm up
		LinkUpDown:      0,    // oper down
		LinkMtu:         9216, // Default MTU
		L2Address:       hwAddr1Parse,
		L2AddressLength: uint32(len(hwAddr1Parse)),
		LinkSpeed:       2, // 100MB, full duplex
	}

	var notif *intf.InterfaceNotification

	Eventually(publishChan).Should(Receive(&notif))
	Expect(notif.Type).To(Equal(intf.InterfaceNotification_UNKNOWN))
	Expect(notif.State.AdminStatus).To(Equal(intf.InterfacesState_Interface_UP))
	Expect(notif.State.OperStatus).To(Equal(intf.InterfacesState_Interface_DOWN))
	Expect(notif.State.InternalName).To(Equal("if0"))
	Expect(notif.State.Mtu).To(BeEquivalentTo(9216))
	Expect(notif.State.PhysAddress).To(Equal("01:23:45:67:89:ab"))
	Expect(notif.State.Duplex).To(Equal(intf.InterfacesState_Interface_FULL))
	Expect(notif.State.Speed).To(BeEquivalentTo(100 * 1000000))
}

// Test deleted notification
func TestInterfaceStateUpdaterIfStateDeleted(t *testing.T) {
	conn, index, ifPlugin, notifChan, publishChan := testPluginDataInitialization(t)
	defer testPluginDataTeardown(ifPlugin, conn)

	// Register name
	index.RegisterName("test", 0, &intf.Interfaces_Interface{
		Name:        "test",
		Enabled:     true,
		Type:        intf.InterfaceType_MEMORY_INTERFACE,
		IpAddresses: []string{"192.168.0.1/24"},
	})

	// Test notifications
	notifChan <- &interfaces.SwInterfaceEvent{
		PID:         0,
		SwIfIndex:   0,
		AdminUpDown: 1,
		LinkUpDown:  1,
		Deleted:     0,
	}

	var notif *intf.InterfaceNotification

	Eventually(publishChan).Should(Receive(&notif))
	Expect(notif.Type).To(Equal(intf.InterfaceNotification_UPDOWN))
	Expect(notif.State.AdminStatus).Should(BeEquivalentTo(intf.InterfacesState_Interface_UP))

	// Unregister name
	index.UnregisterName("test")

	Eventually(publishChan).Should(Receive(&notif))
	Expect(notif.Type).To(Equal(intf.InterfaceNotification_UNKNOWN))
	Expect(notif.State.AdminStatus).Should(BeEquivalentTo(intf.InterfacesState_Interface_DELETED))
	Expect(notif.State.OperStatus).Should(BeEquivalentTo(intf.InterfacesState_Interface_DELETED))
}
