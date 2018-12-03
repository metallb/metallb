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

package vppcalls_test

import (
	"testing"

	"github.com/ligato/cn-infra/logging/logrus"
	"github.com/ligato/vpp-agent/idxvpp/nametoidx"
	l2ba "github.com/ligato/vpp-agent/plugins/vpp/binapi/l2"
	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin/ifaceidx"
	"github.com/ligato/vpp-agent/plugins/vpp/l2plugin/vppcalls"
	"github.com/ligato/vpp-agent/plugins/vpp/model/l2"
	"github.com/ligato/vpp-agent/tests/vppcallmock"
	. "github.com/onsi/gomega"
)

const (
	dummyBridgeDomain     = 4
	dummyBridgeDomainName = "bridge_domain"
)

// Input test data for creating bridge domain
var createTestDataInBD *l2.BridgeDomains_BridgeDomain = &l2.BridgeDomains_BridgeDomain{
	Name:                dummyBridgeDomainName,
	Flood:               true,
	UnknownUnicastFlood: true,
	Forward:             true,
	Learn:               true,
	ArpTermination:      true,
	MacAge:              45,
}

// Output test data for creating bridge domain
var createTestDataOutBD *l2ba.BridgeDomainAddDel = &l2ba.BridgeDomainAddDel{
	BdID:    dummyBridgeDomain,
	Flood:   1,
	UuFlood: 1,
	Forward: 1,
	Learn:   1,
	ArpTerm: 1,
	MacAge:  45,
	BdTag:   []byte(dummyBridgeDomainName),
	IsAdd:   1,
}

// Input test data for updating bridge domain
var updateTestDataInBd *l2.BridgeDomains_BridgeDomain = &l2.BridgeDomains_BridgeDomain{
	Name:                dummyBridgeDomainName,
	Flood:               false,
	UnknownUnicastFlood: false,
	Forward:             false,
	Learn:               false,
	ArpTermination:      false,
	MacAge:              50,
}

// Output test data for updating bridge domain
var updateTestDataOutBd *l2ba.BridgeDomainAddDel = &l2ba.BridgeDomainAddDel{
	BdID:    dummyBridgeDomain,
	Flood:   0,
	UuFlood: 0,
	Forward: 0,
	Learn:   0,
	ArpTerm: 0,
	MacAge:  50,
	IsAdd:   1,
}

// Output test data for deleting bridge domain
var deleteTestDataOutBd *l2ba.BridgeDomainAddDel = &l2ba.BridgeDomainAddDel{
	BdID:  dummyBridgeDomain,
	IsAdd: 0,
}

func TestVppAddBridgeDomain(t *testing.T) {
	ctx, bdHandler, _ := bdTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&l2ba.BridgeDomainAddDelReply{})
	err := bdHandler.VppAddBridgeDomain(dummyBridgeDomain, createTestDataInBD)

	Expect(err).ShouldNot(HaveOccurred())
	Expect(ctx.MockChannel.Msg).To(Equal(createTestDataOutBD))
}

func TestVppAddBridgeDomainError(t *testing.T) {
	ctx, bdHandler, _ := bdTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&l2ba.BridgeDomainAddDelReply{Retval: 1})
	ctx.MockVpp.MockReply(&l2ba.SwInterfaceSetL2Bridge{})

	err := bdHandler.VppAddBridgeDomain(dummyBridgeDomain, createTestDataInBD)
	Expect(err).Should(HaveOccurred())

	err = bdHandler.VppAddBridgeDomain(dummyBridgeDomain, createTestDataInBD)
	Expect(err).Should(HaveOccurred())
}

func TestVppDeleteBridgeDomain(t *testing.T) {
	ctx, bdHandler, _ := bdTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&l2ba.BridgeDomainAddDelReply{})
	err := bdHandler.VppDeleteBridgeDomain(dummyBridgeDomain)

	Expect(err).ShouldNot(HaveOccurred())
	Expect(ctx.MockChannel.Msg).To(Equal(deleteTestDataOutBd))
}

func TestVppDeleteBridgeDomainError(t *testing.T) {
	ctx, bdHandler, _ := bdTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&l2ba.BridgeDomainAddDelReply{Retval: 1})
	ctx.MockVpp.MockReply(&l2ba.SwInterfaceSetL2Bridge{})

	err := bdHandler.VppDeleteBridgeDomain(dummyBridgeDomain)
	Expect(err).Should(HaveOccurred())

	err = bdHandler.VppDeleteBridgeDomain(dummyBridgeDomain)
	Expect(err).Should(HaveOccurred())
}

func bdTestSetup(t *testing.T) (*vppcallmock.TestCtx, vppcalls.BridgeDomainVppAPI, ifaceidx.SwIfIndexRW) {
	ctx := vppcallmock.SetupTestCtx(t)
	log := logrus.NewLogger("test-log")
	ifIndex := ifaceidx.NewSwIfIndex(nametoidx.NewNameToIdx(log, "bd-test-ifidx", nil))
	bdHandler := vppcalls.NewBridgeDomainVppHandler(ctx.MockChannel, ifIndex, log)
	return ctx, bdHandler, ifIndex
}
