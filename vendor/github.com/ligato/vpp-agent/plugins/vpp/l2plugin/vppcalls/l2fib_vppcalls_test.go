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
	"log"
	"os"
	"testing"
	"time"

	govppcore "git.fd.io/govpp.git/core"
	"github.com/ligato/cn-infra/logging/logrus"
	"github.com/ligato/vpp-agent/idxvpp/nametoidx"
	l2ba "github.com/ligato/vpp-agent/plugins/vpp/binapi/l2"
	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin/ifaceidx"
	"github.com/ligato/vpp-agent/plugins/vpp/l2plugin/l2idx"
	"github.com/ligato/vpp-agent/plugins/vpp/l2plugin/vppcalls"
	"github.com/ligato/vpp-agent/tests/vppcallmock"
	. "github.com/onsi/gomega"
	logrus2 "github.com/sirupsen/logrus"
)

var testDataInFib = []struct {
	mac    string
	bdID   uint32
	ifIdx  uint32
	bvi    bool
	static bool
}{
	{"FF:FF:FF:FF:FF:FF", 5, 55, true, true},
	{"FF:FF:FF:FF:FF:FF", 5, 55, false, true},
	{"FF:FF:FF:FF:FF:FF", 5, 55, true, false},
	{"FF:FF:FF:FF:FF:FF", 5, 55, false, false},
}

var createTestDatasOutFib = []*l2ba.L2fibAddDel{
	{BdID: 5, IsAdd: 1, SwIfIndex: 55, BviMac: 1, Mac: []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}, StaticMac: 1},
	{BdID: 5, IsAdd: 1, SwIfIndex: 55, BviMac: 0, Mac: []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}, StaticMac: 1},
	{BdID: 5, IsAdd: 1, SwIfIndex: 55, BviMac: 1, Mac: []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}, StaticMac: 0},
	{BdID: 5, IsAdd: 1, SwIfIndex: 55, BviMac: 0, Mac: []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}, StaticMac: 0},
}

var deleteTestDataOutFib = &l2ba.L2fibAddDel{
	BdID: 5, IsAdd: 0, SwIfIndex: 55, Mac: []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
}

func TestL2FibAdd(t *testing.T) {
	ctx, fibHandler, _, _ := fibTestSetup(t)
	defer ctx.TeardownTestCtx()

	go fibHandler.WatchFIBReplies()

	errc := make(chan error, len(testDataInFib))
	cb := func(err error) {
		errc <- err
	}
	for i := 0; i < len(testDataInFib); i++ {
		ctx.MockVpp.MockReply(&l2ba.L2fibAddDelReply{})
		fibHandler.Add(testDataInFib[i].mac, testDataInFib[i].bdID, testDataInFib[i].ifIdx,
			testDataInFib[i].bvi, testDataInFib[i].static, cb)
		err := <-errc
		Expect(err).ShouldNot(HaveOccurred())
		Expect(ctx.MockChannel.Msg).To(Equal(createTestDatasOutFib[i]))
	}
}

func TestL2FibAddError(t *testing.T) {
	ctx, fibHandler, _, _ := fibTestSetup(t)
	defer ctx.TeardownTestCtx()

	go fibHandler.WatchFIBReplies()

	errc := make(chan error, len(testDataInFib))
	cb := func(err error) {
		errc <- err
	}

	fibHandler.Add("not:mac:addr", 4, 10, false, false, cb)
	err := <-errc
	Expect(err).Should(HaveOccurred())

	ctx.MockVpp.MockReply(&l2ba.L2fibAddDelReply{Retval: 1})
	fibHandler.Add("FF:FF:FF:FF:FF:FF", 4, 10, false, false, cb)
	err = <-errc
	Expect(err).Should(HaveOccurred())

	ctx.MockVpp.MockReply(&l2ba.BridgeDomainAddDelReply{})
	fibHandler.Add("FF:FF:FF:FF:FF:FF", 4, 10, false, false, cb)
	err = <-errc
	Expect(err).Should(HaveOccurred())
}

func TestL2FibDelete(t *testing.T) {
	ctx, fibHandler, _, _ := fibTestSetup(t)
	defer ctx.TeardownTestCtx()

	go fibHandler.WatchFIBReplies()

	errc := make(chan error, len(testDataInFib))
	cb := func(err error) {
		errc <- err
	}
	for i := 0; i < len(testDataInFib); i++ {
		ctx.MockVpp.MockReply(&l2ba.L2fibAddDelReply{})
		fibHandler.Delete(testDataInFib[i].mac, testDataInFib[i].bdID, testDataInFib[i].ifIdx, cb)
		err := <-errc
		Expect(err).ShouldNot(HaveOccurred())
		Expect(ctx.MockChannel.Msg).To(Equal(deleteTestDataOutFib))
	}
}

func TestWatchFIBReplies(t *testing.T) {
	ctx, fibHandler, _, _ := fibTestSetup(t)
	defer ctx.TeardownTestCtx()

	go fibHandler.WatchFIBReplies()

	ctx.MockVpp.MockReply(&l2ba.L2fibAddDelReply{})

	errc := make(chan error)
	cb := func(err error) {
		log.Println("dummyCallback:", err)
		errc <- err
	}
	fibHandler.Add("FF:FF:FF:FF:FF:FF", 4, 45, false, false, cb)

	select {
	case err := <-errc:
		Expect(err).ShouldNot(HaveOccurred())
	case <-time.After(time.Second):
		t.Fail()
	}
}

func benchmarkWatchFIBReplies(reqN int, b *testing.B) {
	ctx, fibHandler, _, _ := fibTestSetup(nil)
	defer ctx.TeardownTestCtx()

	// debug logs slow down benchmarks
	govpplogger := logrus2.New()
	govpplogger.Out = os.Stdout
	govpplogger.Level = logrus2.WarnLevel
	govppcore.SetLogger(govpplogger)

	go fibHandler.WatchFIBReplies()

	errc := make(chan error, reqN)
	cb := func(err error) {
		errc <- err
	}

	for n := 0; n < b.N; n++ {
		for i := 0; i < reqN; i++ {
			ctx.MockVpp.MockReply(&l2ba.L2fibAddDelReply{})
			fibHandler.Add("FF:FF:FF:FF:FF:FF", 4, 45, false, false, cb)
		}

		count := 0
		for {
			select {
			case err := <-errc:
				if err != nil {
					b.FailNow()
				}
				count++
			case <-time.After(time.Second):
				b.FailNow()
			}
			if count == reqN {
				break
			}
		}
	}
}

func BenchmarkWatchFIBReplies1(b *testing.B)    { benchmarkWatchFIBReplies(1, b) }
func BenchmarkWatchFIBReplies10(b *testing.B)   { benchmarkWatchFIBReplies(10, b) }
func BenchmarkWatchFIBReplies100(b *testing.B)  { benchmarkWatchFIBReplies(100, b) }
func BenchmarkWatchFIBReplies1000(b *testing.B) { benchmarkWatchFIBReplies(1000, b) }

func fibTestSetup(t *testing.T) (*vppcallmock.TestCtx, vppcalls.FibVppAPI, ifaceidx.SwIfIndexRW, l2idx.BDIndexRW) {
	ctx := vppcallmock.SetupTestCtx(t)
	logger := logrus.NewLogger("test-log")
	ifIndexes := ifaceidx.NewSwIfIndex(nametoidx.NewNameToIdx(logger, "fib-if-idx", nil))
	bdIndexes := l2idx.NewBDIndex(nametoidx.NewNameToIdx(logger, "fib-bd-idx", nil))
	fibHandler := vppcalls.NewFibVppHandler(ctx.MockChannel, ctx.MockChannel, ifIndexes, bdIndexes, logger)
	return ctx, fibHandler, ifIndexes, bdIndexes
}
