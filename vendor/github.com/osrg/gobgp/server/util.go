// Copyright (C) 2016 Nippon Telegraph and Telephone Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package server

import (
	"net"
	"os"
	"strings"
	"syscall"

	"github.com/eapache/channels"

	"github.com/osrg/gobgp/packet/bgp"
)

func cleanInfiniteChannel(ch *channels.InfiniteChannel) {
	ch.Close()
	// drain all remaining items
	for range ch.Out() {
	}
}

// Returns the binary formatted Administrative Shutdown Communication from the
// given string value.
func newAdministrativeCommunication(communication string) (data []byte) {
	if communication == "" {
		return nil
	}
	com := []byte(communication)
	if len(com) > bgp.BGP_ERROR_ADMINISTRATIVE_COMMUNICATION_MAX {
		data = []byte{bgp.BGP_ERROR_ADMINISTRATIVE_COMMUNICATION_MAX}
		data = append(data, com[:bgp.BGP_ERROR_ADMINISTRATIVE_COMMUNICATION_MAX]...)
	} else {
		data = []byte{byte(len(com))}
		data = append(data, com...)
	}
	return data
}

// Parses the given NOTIFICATION message data as a binary value and returns
// the Administrative Shutdown Communication in string and the rest binary.
func decodeAdministrativeCommunication(data []byte) (string, []byte) {
	if len(data) == 0 {
		return "", data
	}
	communicationLen := int(data[0])
	if communicationLen > bgp.BGP_ERROR_ADMINISTRATIVE_COMMUNICATION_MAX {
		communicationLen = bgp.BGP_ERROR_ADMINISTRATIVE_COMMUNICATION_MAX
	}
	if communicationLen > len(data)+1 {
		communicationLen = len(data) + 1
	}
	return string(data[1 : communicationLen+1]), data[communicationLen+1:]
}

func extractFileAndFamilyFromTCPListener(l *net.TCPListener) (*os.File, int, error) {
	// Note #1: TCPListener.File() has the unexpected side-effect of putting
	// the original socket into blocking mode. See Note #2.
	fi, err := l.File()
	if err != nil {
		return nil, 0, err
	}

	// Note #2: Call net.FileListener() to put the original socket back into
	// non-blocking mode.
	fl, err := net.FileListener(fi)
	if err != nil {
		fi.Close()
		return nil, 0, err
	}
	fl.Close()

	family := syscall.AF_INET
	if strings.Contains(l.Addr().String(), "[") {
		family = syscall.AF_INET6
	}

	return fi, family, nil
}

func extractFileAndFamilyFromTCPConn(conn *net.TCPConn) (*os.File, int, error) {
	// Note #1: TCPConn.File() has the unexpected side-effect of putting
	// the original socket into blocking mode. See Note #2.
	fi, err := conn.File()
	if err != nil {
		return nil, 0, err
	}

	// Note #2: Call net.FileConn() to put the original socket back into
	// non-blocking mode.
	fc, err := net.FileConn(fi)
	if err != nil {
		fi.Close()
		return nil, 0, err
	}
	fc.Close()

	family := syscall.AF_INET
	if strings.Contains(conn.RemoteAddr().String(), "[") {
		family = syscall.AF_INET6
	}

	return fi, family, nil
}
