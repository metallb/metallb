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

package status_test

import (
	"os"
	"testing"

	"github.com/ligato/cn-infra/logging/logrus"
	"github.com/ligato/cn-infra/process/status"
	. "github.com/onsi/gomega"
)

// TestParseStatusFile parses test status file and verifies every parsed field
func TestParseStatusFile(t *testing.T) {
	RegisterTestingT(t)

	file, err := os.Open("test-state")
	Expect(err).To(BeNil())
	Expect(file).ToNot(BeNil())

	r := status.Reader{
		Log: logrus.DefaultLogger(),
	}

	statusFile := r.ReadStatusFromFile(file)
	Expect(statusFile).ToNot(BeNil())

	Expect(statusFile.Name).To(Equal("vpp_main"))
	Expect(statusFile.UMask).To(Equal(22))
	Expect(statusFile.State).To(Equal(status.ProcessStatus("sleeping")))
	Expect(statusFile.LState).To(Equal("S"))
	Expect(statusFile.Tgid).To(Equal(23986))
	Expect(statusFile.Ngid).To(Equal(0))
	Expect(statusFile.Pid).To(Equal(23986))
	Expect(statusFile.PPid).To(Equal(23985))
	Expect(statusFile.TracerPid).To(Equal(0))
	Expect(statusFile.UID).ToNot(BeNil())
	Expect(statusFile.UID.Real).To(BeZero())
	Expect(statusFile.UID.Effective).To(BeZero())
	Expect(statusFile.UID.SavedSet).To(BeZero())
	Expect(statusFile.UID.FileSystem).To(BeZero())
	Expect(statusFile.GID).ToNot(BeNil())
	Expect(statusFile.GID.Real).To(BeZero())
	Expect(statusFile.GID.Effective).To(Equal(997))
	Expect(statusFile.GID.SavedSet).To(BeZero())
	Expect(statusFile.GID.FileSystem).To(Equal(997))
	Expect(statusFile.FDSize).To(Equal(64))
	Expect(statusFile.Groups).To(HaveLen(1))
	Expect(statusFile.NStgid).To(Equal(23986))
	Expect(statusFile.NSpid).To(Equal(23986))
	Expect(statusFile.NSpgid).To(Equal(23984))
	Expect(statusFile.NSsid).To(Equal(8037))
	Expect(statusFile.VMPeak).To(Equal("5348632kB"))
	Expect(statusFile.VMSize).To(Equal("5348632kB"))
	Expect(statusFile.VMLck).To(Equal("0kB"))
	Expect(statusFile.VMPin).To(Equal("0kB"))
	Expect(statusFile.VMHWM).To(Equal("37880kB"))
	Expect(statusFile.VMRSS).To(Equal("37880kB"))
	Expect(statusFile.RssAnon).To(Equal("20004kB"))
	Expect(statusFile.RssFile).To(Equal("15980kB"))
	Expect(statusFile.RssShmem).To(Equal("1896kB"))
	Expect(statusFile.VMData).To(Equal("5058816kB"))
	Expect(statusFile.VMStk).To(Equal("132kB"))
	Expect(statusFile.VMExe).To(Equal("844kB"))
	Expect(statusFile.VMLib).To(Equal("22088kB"))
	Expect(statusFile.VMPTE).To(Equal("592kB"))
	Expect(statusFile.VMSwap).To(Equal("0kB"))
	Expect(statusFile.HugetlbPages).To(Equal("32768kB"))
	Expect(statusFile.CoreDumping).To(Equal(0))
	Expect(statusFile.Threads).To(Equal(2))
	Expect(statusFile.SigQueued).To(Equal(0))
	Expect(statusFile.SigMax).To(Equal(31666))
	Expect(statusFile.SigPnd).To(Equal([]uint8{0, 0, 0, 0, 0, 0, 0, 0}))
	Expect(statusFile.ShdPnd).To(Equal([]uint8{0, 0, 0, 0, 0, 0, 0, 0}))
	Expect(statusFile.SigBlk).To(Equal([]uint8{0, 0, 0, 0, 0, 0, 0, 0}))
	Expect(statusFile.SigIgn).To(Equal([]uint8{0, 0, 0, 0, 0, 1, 16, 0}))
	Expect(statusFile.SigCgt).To(Equal([]uint8{0, 0, 0, 1, 255, 250, 228, 255}))
	Expect(statusFile.CapInh).To(Equal([]uint8{0, 0, 0, 0, 0, 0, 0, 0}))
	Expect(statusFile.CapPrm).To(Equal([]uint8{0, 0, 0, 63, 255, 255, 255, 255}))
	Expect(statusFile.CapEff).To(Equal([]uint8{0, 0, 0, 63, 255, 255, 255, 255}))
	Expect(statusFile.CapBnd).To(Equal([]uint8{0, 0, 0, 63, 255, 255, 255, 255}))
	Expect(statusFile.Seccomp).To(Equal(0))
	Expect(statusFile.CpusAllowed).To(Equal("2"))
	Expect(statusFile.MemsAllowedList).To(Equal(0))
	Expect(statusFile.VoluntaryCtxtSwitches).To(Equal(1377))
	Expect(statusFile.NonvoluntaryCtxtSwitches).To(Equal(80))
}
