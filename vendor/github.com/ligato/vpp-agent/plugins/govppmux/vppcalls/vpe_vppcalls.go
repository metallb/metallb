//  Copyright (c) 2018 Cisco and/or its affiliates.
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at:
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package vppcalls

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	govppapi "git.fd.io/govpp.git/api"
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/memclnt"
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/vpe"
)

// VpeInfo contains information about VPP connection and process.
type VpeInfo struct {
	PID            uint32
	ClientIdx      uint32
	ModuleVersions map[string]ModuleVersion
}

type ModuleVersion struct {
	Name  string
	Major uint32
	Minor uint32
	Patch uint32
}

// GetVpeInfo retrieves vpe information.
func GetVpeInfo(vppChan govppapi.Channel) (*VpeInfo, error) {
	req := &vpe.ControlPing{}
	reply := &vpe.ControlPingReply{}

	if err := vppChan.SendRequest(req).ReceiveReply(reply); err != nil {
		return nil, err
	}

	info := &VpeInfo{
		PID:            reply.VpePID,
		ClientIdx:      reply.ClientIndex,
		ModuleVersions: make(map[string]ModuleVersion),
	}

	{
		req := &memclnt.APIVersions{}
		reply := &memclnt.APIVersionsReply{}

		if err := vppChan.SendRequest(req).ReceiveReply(reply); err != nil {
			return nil, err
		}

		for _, v := range reply.APIVersions {
			name := string(cleanBytes(v.Name))
			info.ModuleVersions[name] = ModuleVersion{
				Name:  name,
				Major: v.Major,
				Minor: v.Minor,
				Patch: v.Patch,
			}
		}
	}

	return info, nil
}

// VersionInfo contains values returned from ShowVersion
type VersionInfo struct {
	Program        string
	Version        string
	BuildDate      string
	BuildDirectory string
}

// GetVersionInfo retrieves version info
func GetVersionInfo(vppChan govppapi.Channel) (*VersionInfo, error) {
	req := &vpe.ShowVersion{}
	reply := &vpe.ShowVersionReply{}

	if err := vppChan.SendRequest(req).ReceiveReply(reply); err != nil {
		return nil, err
	} else if reply.Retval != 0 {
		return nil, fmt.Errorf("%s returned %d", reply.GetMessageName(), reply.Retval)
	}

	info := &VersionInfo{
		Program:        string(cleanBytes(reply.Program)),
		Version:        string(cleanBytes(reply.Version)),
		BuildDate:      string(cleanBytes(reply.BuildDate)),
		BuildDirectory: string(cleanBytes(reply.BuildDirectory)),
	}

	return info, nil
}

// RunCliCommand executes CLI command and returns output
func RunCliCommand(vppChan govppapi.Channel, cmd string) ([]byte, error) {
	req := &vpe.CliInband{
		Cmd:    []byte(cmd),
		Length: uint32(len(cmd)),
	}
	reply := &vpe.CliInbandReply{}

	if err := vppChan.SendRequest(req).ReceiveReply(reply); err != nil {
		return nil, err
	} else if reply.Retval != 0 {
		return nil, fmt.Errorf("%s returned %d", reply.GetMessageName(), reply.Retval)
	}

	return reply.Reply[:reply.Length], nil
}

// MemoryInfo contains values returned from 'show memory'
type MemoryInfo struct {
	Threads []MemoryThread `json:"threads"`
}

// MemoryThread represents single thread memory counters
type MemoryThread struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	Objects   uint64 `json:"objects"`
	Used      uint64 `json:"used"`
	Total     uint64 `json:"total"`
	Free      uint64 `json:"free"`
	Reclaimed uint64 `json:"reclaimed"`
	Overhead  uint64 `json:"overhead"`
	Capacity  uint64 `json:"capacity"`
}

var (
	// Regular expression to parse output from `show memory`
	memoryRe = regexp.MustCompile(`Thread\s+(\d+)\s+(\w+).?\s+(\d+) objects, ([\dkmg\.]+) of ([\dkmg\.]+) used, ([\dkmg\.]+) free, ([\dkmg\.]+) reclaimed, ([\dkmg\.]+) overhead, ([\dkmg\.]+) capacity`)
)

// GetMemory retrieves `show memory` info.
func GetMemory(vppChan govppapi.Channel) (*MemoryInfo, error) {
	data, err := RunCliCommand(vppChan, "show memory")
	if err != nil {
		return nil, err
	}

	var threads []MemoryThread

	threadMatches := memoryRe.FindAllStringSubmatch(string(data), -1)
	for _, matches := range threadMatches {
		fields := matches[1:]
		if len(fields) != 9 {
			return nil, fmt.Errorf("invalid memory data for thread: %q", matches[0])
		}
		id, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			return nil, err
		}
		thread := &MemoryThread{
			ID:        uint(id),
			Name:      fields[1],
			Objects:   strToUint64(fields[2]),
			Used:      strToUint64(fields[3]),
			Total:     strToUint64(fields[4]),
			Free:      strToUint64(fields[5]),
			Reclaimed: strToUint64(fields[6]),
			Overhead:  strToUint64(fields[7]),
			Capacity:  strToUint64(fields[8]),
		}
		threads = append(threads, *thread)
	}

	info := &MemoryInfo{
		Threads: threads,
	}

	return info, nil
}

// NodeCounterInfo contains values returned from 'show node counters'
type NodeCounterInfo struct {
	Counters []NodeCounter `json:"counters"`
}

// NodeCounter represents single node counter
type NodeCounter struct {
	Count  uint64 `json:"count"`
	Node   string `json:"node"`
	Reason string `json:"reason"`
}

var (
	// Regular expression to parse output from `show node counters`
	nodeCountersRe = regexp.MustCompile(`^\s+(\d+)\s+([\w-\/]+)\s+(.+)$`)
)

// GetNodeCounters retrieves node counters info.
func GetNodeCounters(vppChan govppapi.Channel) (*NodeCounterInfo, error) {
	data, err := RunCliCommand(vppChan, "show node counters")
	if err != nil {
		return nil, err
	}

	var counters []NodeCounter

	for i, line := range strings.Split(string(data), "\n") {
		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			continue
		}
		// Check first line
		if i == 0 {
			fields := strings.Fields(line)
			// Verify header
			if len(fields) != 3 || fields[0] != "Count" {
				return nil, fmt.Errorf("invalid header for `show node counters` received: %q", line)
			}
			continue
		}

		// Parse lines using regexp
		matches := nodeCountersRe.FindStringSubmatch(line)
		if len(matches)-1 != 3 {
			return nil, fmt.Errorf("parsing failed for `show node counters` line: %q", line)
		}
		fields := matches[1:]

		counters = append(counters, NodeCounter{
			Count:  strToUint64(fields[0]),
			Node:   fields[1],
			Reason: fields[2],
		})
	}

	info := &NodeCounterInfo{
		Counters: counters,
	}

	return info, nil
}

// RuntimeInfo contains values returned from 'show runtime'
type RuntimeInfo struct {
	Threads []RuntimeThread `json:"threads"`
}

// RuntimeThread represents single runtime thread
type RuntimeThread struct {
	ID                  uint          `json:"id"`
	Name                string        `json:"name"`
	Time                float64       `json:"time"`
	AvgVectorsPerNode   float64       `json:"avg_vectors_per_node"`
	LastMainLoops       uint64        `json:"last_main_loops"`
	VectorsPerMainLoop  float64       `json:"vectors_per_main_loop"`
	VectorLengthPerNode float64       `json:"vector_length_per_node"`
	VectorRatesIn       float64       `json:"vector_rates_in"`
	VectorRatesOut      float64       `json:"vector_rates_out"`
	VectorRatesDrop     float64       `json:"vector_rates_drop"`
	VectorRatesPunt     float64       `json:"vector_rates_punt"`
	Items               []RuntimeItem `json:"items"`
}

// RuntimeItem represents single runtime item
type RuntimeItem struct {
	Name           string  `json:"name"`
	State          string  `json:"state"`
	Calls          uint64  `json:"calls"`
	Vectors        uint64  `json:"vectors"`
	Suspends       uint64  `json:"suspends"`
	Clocks         float64 `json:"clocks"`
	VectorsPerCall float64 `json:"vectors_per_call"`
}

var (
	// Regular expression to parse output from `show runtime`
	runtimeRe = regexp.MustCompile(`(?:-+\n)?(?:Thread (\d+) (\w+)(?: \(lcore \d+\))?\n)?` +
		`Time ([0-9\.e]+), average vectors/node ([0-9\.e]+), last (\d+) main loops ([0-9\.e]+) per node ([0-9\.e]+)\s+` +
		`vector rates in ([0-9\.e]+), out ([0-9\.e]+), drop ([0-9\.e]+), punt ([0-9\.e]+)\n` +
		`\s+Name\s+State\s+Calls\s+Vectors\s+Suspends\s+Clocks\s+Vectors/Call\s+` +
		`((?:[\w-:\.]+\s+\w+(?:[ -]\w+)*\s+\d+\s+\d+\s+\d+\s+[0-9\.e]+\s+[0-9\.e]+\s+)+)`)
	runtimeItemsRe = regexp.MustCompile(`([\w-:\.]+)\s+(\w+(?:[ -]\w+)*)\s+(\d+)\s+(\d+)\s+(\d+)\s+([0-9\.e]+)\s+([0-9\.e]+)\s+`)
)

// GetRuntimeInfo retrieves how runtime info.
func GetRuntimeInfo(vppChan govppapi.Channel) (*RuntimeInfo, error) {
	data, err := RunCliCommand(vppChan, "show runtime")
	if err != nil {
		return nil, err
	}

	var threads []RuntimeThread

	threadMatches := runtimeRe.FindAllStringSubmatch(string(data), -1)
	for _, matches := range threadMatches {
		fields := matches[1:]
		if len(fields) != 12 {
			return nil, fmt.Errorf("invalid runtime data for thread: %q", matches[0])
		}
		thread := RuntimeThread{
			ID:                  uint(strToUint64(fields[0])),
			Name:                fields[1],
			Time:                strToFloat64(fields[2]),
			AvgVectorsPerNode:   strToFloat64(fields[3]),
			LastMainLoops:       strToUint64(fields[4]),
			VectorsPerMainLoop:  strToFloat64(fields[5]),
			VectorLengthPerNode: strToFloat64(fields[6]),
			VectorRatesIn:       strToFloat64(fields[7]),
			VectorRatesOut:      strToFloat64(fields[8]),
			VectorRatesDrop:     strToFloat64(fields[9]),
			VectorRatesPunt:     strToFloat64(fields[10]),
		}

		itemMatches := runtimeItemsRe.FindAllStringSubmatch(fields[11], -1)
		for _, matches := range itemMatches {
			fields := matches[1:]
			if len(fields) != 7 {
				return nil, fmt.Errorf("invalid runtime data for thread item: %q", matches[0])
			}
			thread.Items = append(thread.Items, RuntimeItem{
				Name:           fields[0],
				State:          fields[1],
				Calls:          strToUint64(fields[2]),
				Vectors:        strToUint64(fields[3]),
				Suspends:       strToUint64(fields[4]),
				Clocks:         strToFloat64(fields[5]),
				VectorsPerCall: strToFloat64(fields[6]),
			})
		}

		threads = append(threads, thread)
	}

	info := &RuntimeInfo{
		Threads: threads,
	}

	return info, nil
}

// BuffersInfo contains values returned from 'show buffers'
type BuffersInfo struct {
	Items []BuffersItem `json:"items"`
}

// BuffersItem represents single buffers item
type BuffersItem struct {
	ThreadID uint   `json:"thread_id"`
	Name     string `json:"name"`
	Index    uint   `json:"index"`
	Size     uint64 `json:"size"`
	Alloc    uint64 `json:"alloc"`
	Free     uint64 `json:"free"`
	NumAlloc uint64 `json:"num_alloc"`
	NumFree  uint64 `json:"num_free"`
}

var (
	// Regular expression to parse output from `show buffers`
	buffersRe = regexp.MustCompile(`^\s+(\d+)\s+(\w+(?:[ \-]\w+)*)\s+(\d+)\s+(\d+)\s+([\dkmg\.]+)\s+([\dkmg\.]+)\s+(\d+)\s+(\d+).*$`)
)

// GetBuffersInfo retrieves buffers info
func GetBuffersInfo(vppChan govppapi.Channel) (*BuffersInfo, error) {
	data, err := RunCliCommand(vppChan, "show buffers")
	if err != nil {
		return nil, err
	}

	var items []BuffersItem

	for i, line := range strings.Split(string(data), "\n") {
		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			continue
		}
		// Check first line
		if i == 0 {
			fields := strings.Fields(line)
			// Verify header
			if len(fields) != 8 || fields[0] != "Thread" {
				return nil, fmt.Errorf("invalid header for `show buffers` received: %q", line)
			}
			continue
		}

		// Parse lines using regexp
		matches := buffersRe.FindStringSubmatch(line)
		if len(matches)-1 != 8 {
			return nil, fmt.Errorf("parsing failed for `show buffers` line: %q", line)
		}
		fields := matches[1:]

		items = append(items, BuffersItem{
			ThreadID: uint(strToUint64(fields[0])),
			Name:     fields[1],
			Index:    uint(strToUint64(fields[2])),
			Size:     strToUint64(fields[3]),
			Alloc:    strToUint64(fields[4]),
			Free:     strToUint64(fields[5]),
			NumAlloc: strToUint64(fields[6]),
			NumFree:  strToUint64(fields[7]),
		})
	}

	info := &BuffersInfo{
		Items: items,
	}

	return info, nil
}

func strToFloat64(s string) float64 {
	// Replace 'k' (thousands) with 'e3' to make it parsable with strconv
	s = strings.Replace(s, "k", "e3", 1)
	s = strings.Replace(s, "m", "e6", 1)
	s = strings.Replace(s, "g", "e9", 1)

	num, err := strconv.ParseFloat(s, 10)
	if err != nil {
		return 0
	}
	return num
}

func strToUint64(s string) uint64 {
	return uint64(strToFloat64(s))
}

func cleanBytes(b []byte) []byte {
	return bytes.SplitN(b, []byte{0x00}, 2)[0]
}
