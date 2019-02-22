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

package status

import (
	"bufio"
	"encoding/hex"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/ligato/cn-infra/logging"

	"github.com/pkg/errors"
)

// Plugin states which require special handling
const (
	// Those are common process statuses, defined as reference
	Sleeping = "sleeping"
	Running  = "running"
	Idle     = "idle"
	Zombie   = "zombie" // If child process is terminated with parent still running. Needs to be cleaned up.
	// Plugin-defined process statuses (as addition to other process statuses)
	Unavailable = "unavailable" // If process status cannot be obtained
	Terminated  = "terminated"  // If process is not running (while tested by zero signal)
)

// ProcessStatus is string representation of process status
type ProcessStatus string

// Reader provides safe process status manipulation
type Reader struct {
	sync.Mutex
	Log logging.Logger
}

// File mirrors process status file
type File struct {
	Name                     string        // Name of the executable
	UMask                    int           // Mode creation mask
	State                    ProcessStatus // State (running, sleeping, uninterruptible wait, zombie, etc...)
	LState                   string        // Proces state label (single letter)
	Tgid                     int           // Thread group ID
	Ngid                     int           // NUMA group ID
	Pid                      int           // Process id
	PPid                     int           // Process id of the parent process
	TracerPid                int           // PID of process tracing this process
	UID                      *GUID         // Set of UIDs
	GID                      *GUID         // Set of GIDs
	FDSize                   int           // Number of file descriptor slots currently allocated
	Groups                   []int         // Supplementary group list
	NStgid                   int           // Descendant namespace thread group ID hierarchy
	NSpid                    int           // Descendant namespace process ID hierarchy
	NSpgid                   int           // Descendant namespace process group ID hierarchy
	NSsid                    int           // Descendant namespace session ID hierarchy
	VMPeak                   string        // Peak virtual memory size
	VMSize                   string        // Total program size
	VMLck                    string        // Locked memory size
	VMPin                    string        // Pinned memory size
	VMHWM                    string        // Peak resident set size
	VMRSS                    string        // Size of memory portions (RssAnon + RssFile + RssShmem)
	RssAnon                  string        // Size of resident anonymous memory
	RssFile                  string        // Size of resident file mappings
	RssShmem                 string        // Size of resident shmem memory
	VMData                   string        // Size of private data segments
	VMStk                    string        // Size of stack segments
	VMExe                    string        // Size of text segment
	VMLib                    string        // Size of shared library code
	VMPTE                    string        // Size of page table entries
	VMSwap                   string        // Amount of swap used by anonymous private data
	HugetlbPages             string        // Size of hugetlb memory portions
	CoreDumping              int           // Process's memory is currently being dumped
	Threads                  int           // Number of threads
	SigQueued                int           // Number of signals queued/
	SigMax                   int           // Max. number for queue
	SigPnd                   []byte        // Bitmap of pending signals for the thread
	ShdPnd                   []byte        // Bitmap of shared pending signals for the process
	SigBlk                   []byte        // Bitmap of blocked signals
	SigIgn                   []byte        // Bitmap of ignored signals
	SigCgt                   []byte        // Bitmap of caught signals
	CapInh                   []byte        // Bitmap of inheritable capabilities
	CapPrm                   []byte        // Bitmap of permitted capabilities
	CapEff                   []byte        // Bitmap of effective capabilities
	CapBnd                   []byte        // Bitmap of capabilities bounding set
	Seccomp                  int           // Seccomp mode
	CpusAllowed              string        // Mask of CPUs on which this process may run
	MemsAllowedList          int           // Mask of memory nodes allowed to this process
	VoluntaryCtxtSwitches    int           // Number of voluntary context switches
	NonvoluntaryCtxtSwitches int           // Number of non voluntary context switches
}

// GUID helper struct for process UID and GID
type GUID struct {
	Real       int
	Effective  int
	SavedSet   int
	FileSystem int
}

// ReadStatusFromPID returns status file object. Data are read from /proc and process ID of existing process is required.
// Error is returned if file (process) does not exist
func (r *Reader) ReadStatusFromPID(pid int) (*File, error) {
	r.Lock()
	defer r.Unlock()

	file, err := os.Open("/proc/" + strconv.Itoa(pid) + "/status")
	if err != nil {
		return &File{}, errors.Errorf("failed to read process %d status file: %v", pid, err)
	}
	defer file.Close()

	return r.parse(file), nil
}

// ReadStatusFromFile allows to eventually read status from custom location and parse it directly
func (r *Reader) ReadStatusFromFile(file *os.File) *File {
	return r.parse(file)
}

// Parser scans process status file and creates a structure with all available values representing process
// overall status. Any parse errors are logged but ignored since parser tries to fetch as much status data as possible
func (r *Reader) parse(file *os.File) *File {
	status := &File{}

	sc := bufio.NewScanner(file)
	for sc.Scan() {
		parts := strings.Split(sc.Text(), ":")
		if len(parts) < 2 {
			continue
		}
		label := prune(parts[0])
		var err error
		switch label {
		case "Name":
			status.Name = prune(parts[1])
		case "Umask":
			if status.UMask, err = strconv.Atoi(prune(parts[1])); err != nil {

			}
		case "State":
			valuePts := strings.Split(parts[1], " ")
			if len(valuePts) == 2 {
				status.LState = prune(valuePts[0])
				state := strings.Replace(valuePts[1], "(", "", 1)
				state = strings.Replace(state, ")", "", 1)
				status.State = ProcessStatus(prune(state))
			}
		case "Tgid":
			status.Tgid = r.toInt(parts[1])
		case "Ngid":
			status.Ngid = r.toInt(parts[1])
		case "Pid":
			status.Pid = r.toInt(parts[1])
		case "PPid":
			status.PPid = r.toInt(parts[1])
		case "TracerPid":
			status.TracerPid = r.toInt(parts[1])
		case "Uid":
			status.UID = r.guid(parts[1])
		case "Gid":
			status.GID = r.guid(parts[1])
		case "FDSize":
			status.FDSize = r.toInt(parts[1])
		case "Groups":
			groups := strings.Split(parts[1], " ")
			if len(groups) == 0 {
				status.Groups = []int{0}
			} else {
				for _, group := range groups {
					if group == "" {
						continue
					}
					groupIdx := r.toInt(group)
					status.Groups = append(status.Groups, groupIdx)
				}
			}
		case "NStgid":
			status.NStgid = r.toInt(parts[1])
		case "NSpid":
			status.NSpid = r.toInt(parts[1])
		case "NSpgid":
			status.NSpgid = r.toInt(parts[1])
		case "NSsid":
			status.NSsid = r.toInt(parts[1])
		case "VmPeak":
			status.VMPeak = prune(parts[1])
		case "VmSize":
			status.VMSize = prune(parts[1])
		case "VmLck":
			status.VMLck = prune(parts[1])
		case "VmPin":
			status.VMPin = prune(parts[1])
		case "VmHWM":
			status.VMHWM = prune(parts[1])
		case "VmRSS":
			status.VMRSS = prune(parts[1])
		case "RssAnon":
			status.RssAnon = prune(parts[1])
		case "RssFile":
			status.RssFile = prune(parts[1])
		case "RssShmem":
			status.RssShmem = prune(parts[1])
		case "VmData":
			status.VMData = prune(parts[1])
		case "VmStk":
			status.VMStk = prune(parts[1])
		case "VmExe":
			status.VMExe = prune(parts[1])
		case "VmLib":
			status.VMLib = prune(parts[1])
		case "VmPTE":
			status.VMPTE = prune(parts[1])
		case "VmSwap":
			status.VMSwap = prune(parts[1])
		case "HugetlbPages":
			status.HugetlbPages = prune(parts[1])
		case "CoreDumping":
			status.CoreDumping = r.toInt(parts[1])
		case "Threads":
			status.Threads = r.toInt(parts[1])
		case "SigQ":
			valuePts := strings.Split(parts[1], "/")
			if len(valuePts) == 2 {
				status.SigQueued = r.toInt(valuePts[0])
				status.SigMax = r.toInt(valuePts[1])
			}
		case "SigPnd":
			status.SigPnd = r.toHex(parts[1])
		case "ShdPnd":
			status.ShdPnd = r.toHex(parts[1])
		case "SigBlk":
			status.SigBlk = r.toHex(parts[1])
		case "SigIgn":
			status.SigIgn = r.toHex(parts[1])
		case "SigCgt":
			status.SigCgt = r.toHex(parts[1])
		case "CapInh":
			status.CapInh = r.toHex(parts[1])
		case "CapPrm":
			status.CapPrm = r.toHex(parts[1])
		case "CapEff":
			status.CapEff = r.toHex(parts[1])
		case "CapBnd":
			status.CapBnd = r.toHex(parts[1])
		case "Seccomp":
			status.Seccomp = r.toInt(parts[1])
		case "Cpus_allowed":
			status.CpusAllowed = prune(parts[1])
		case "MemsAllowed_list":
			status.MemsAllowedList = r.toInt(parts[1])
		case "voluntary_ctxt_switches":
			status.VoluntaryCtxtSwitches = r.toInt(parts[1])
		case "nonvoluntary_ctxt_switches":
			status.NonvoluntaryCtxtSwitches = r.toInt(parts[1])
		}
	}

	return status
}

// This method should save a few lines, converting provided string to int while error is logged but not returned
func (r *Reader) toInt(input string) int {
	result, err := strconv.Atoi(prune(input))
	if err != nil {
		r.Log.Warnf("error parsing process status value %s to int: %v", prune(input), err)
	}
	return result
}

// This method should save a few lines, converting provided string to byte array (hex) while error is logged but
// not returned
func (r *Reader) toHex(input string) []byte {
	result, err := hex.DecodeString(prune(input))
	if err != nil {
		r.Log.Warnf("error parsing process status value %s to hex: %v", prune(input), err)
		return []byte{}
	}
	return result
}

// Converts input into UID/GID data
func (r *Reader) guid(input string) *GUID {
	valuePts := strings.Split(input, "\t")
	var guidData []string
	// Remove potential empty strings
	for _, value := range valuePts {
		if value != "" {
			guidData = append(guidData, value)
		}
	}
	// UID/GID requires exactly 4 entries
	if len(guidData) == 4 {
		return &GUID{r.toInt(guidData[0]), r.toInt(guidData[1]), r.toInt(guidData[2]), r.toInt(guidData[3])}
	}
	return &GUID{}
}

// Removes all tabs and whitespaces from input
func prune(input string) string {
	return strings.Replace(strings.Replace(input, "\t", "", -1), " ", "", -1)
}
