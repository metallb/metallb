// Copyright (C) 2018 Nippon Telegraph and Telephone Corporation.
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

package gobgpapi

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/osrg/gobgp/packet/bgp"
)

func Test_MultiProtocolCapability(t *testing.T) {
	assert := assert.New(t)

	input := &MultiProtocolCapability{
		Family: Family_IPv4,
	}

	n, err := input.ToNative()
	assert.Nil(err)
	assert.Equal(bgp.RF_IPv4_UC, n.CapValue)

	output := NewMultiProtocolCapability(n)
	assert.Equal(input, output)
}

func Test_RouteRefreshCapability(t *testing.T) {
	assert := assert.New(t)

	input := &RouteRefreshCapability{}

	n, err := input.ToNative()
	assert.Nil(err)

	output := NewRouteRefreshCapability(n)
	assert.Equal(input, output)
}

func Test_CarryingLabelInfoCapability(t *testing.T) {
	assert := assert.New(t)

	input := &CarryingLabelInfoCapability{}

	n, err := input.ToNative()
	assert.Nil(err)

	output := NewCarryingLabelInfoCapability(n)
	assert.Equal(input, output)
}

func Test_ExtendedNexthopCapability(t *testing.T) {
	assert := assert.New(t)

	input := &ExtendedNexthopCapability{
		Tuples: []*ExtendedNexthopCapabilityTuple{
			{
				NlriFamily:    Family_IPv4,
				NexthopFamily: Family_IPv6,
			},
		},
	}

	n, err := input.ToNative()
	assert.Nil(err)
	assert.Equal(1, len(n.Tuples))
	assert.Equal(uint16(bgp.AFI_IP), n.Tuples[0].NLRIAFI)
	assert.Equal(uint16(bgp.SAFI_UNICAST), n.Tuples[0].NLRISAFI)
	assert.Equal(uint16(bgp.AFI_IP6), n.Tuples[0].NexthopAFI)

	output := NewExtendedNexthopCapability(n)
	assert.Equal(input, output)
}

func Test_GracefulRestartCapability(t *testing.T) {
	assert := assert.New(t)

	input := &GracefulRestartCapability{
		Flags: 0x08 | 0x04, // restarting|notification
		Time:  90,
		Tuples: []*GracefulRestartCapabilityTuple{
			{
				Family: Family_IPv4,
				Flags:  0x80, // forward
			},
		},
	}

	n, err := input.ToNative()
	assert.Nil(err)
	assert.Equal(1, len(n.Tuples))
	assert.Equal(uint8(0x08|0x04), n.Flags)
	assert.Equal(uint16(90), n.Time)
	assert.Equal(uint16(bgp.AFI_IP), n.Tuples[0].AFI)
	assert.Equal(uint8(bgp.SAFI_UNICAST), n.Tuples[0].SAFI)
	assert.Equal(uint8(0x80), n.Tuples[0].Flags)

	output := NewGracefulRestartCapability(n)
	assert.Equal(input, output)
}

func Test_FourOctetASNumberCapability(t *testing.T) {
	assert := assert.New(t)

	input := &FourOctetASNumberCapability{
		As: 100,
	}

	n, err := input.ToNative()
	assert.Nil(err)
	assert.Equal(uint32(100), n.CapValue)

	output := NewFourOctetASNumberCapability(n)
	assert.Equal(input, output)
}

func Test_AddPathCapability(t *testing.T) {
	assert := assert.New(t)

	input := &AddPathCapability{
		Tuples: []*AddPathCapabilityTuple{
			{
				Family: Family_IPv4,
				Mode:   AddPathMode_MODE_BOTH,
			},
		},
	}

	n, err := input.ToNative()
	assert.Nil(err)
	assert.Equal(1, len(n.Tuples))
	assert.Equal(bgp.RF_IPv4_UC, n.Tuples[0].RouteFamily)
	assert.Equal(bgp.BGP_ADD_PATH_BOTH, n.Tuples[0].Mode)

	output := NewAddPathCapability(n)
	assert.Equal(input, output)
}

func Test_EnhancedRouteRefreshCapability(t *testing.T) {
	assert := assert.New(t)

	input := &EnhancedRouteRefreshCapability{}

	n, err := input.ToNative()
	assert.Nil(err)

	output := NewEnhancedRouteRefreshCapability(n)
	assert.Equal(input, output)
}

func Test_LongLivedGracefulRestartCapability(t *testing.T) {
	assert := assert.New(t)

	input := &LongLivedGracefulRestartCapability{
		Tuples: []*LongLivedGracefulRestartCapabilityTuple{
			{
				Family: Family_IPv4,
				Flags:  0x80, // forward
				Time:   90,
			},
		},
	}

	n, err := input.ToNative()
	assert.Nil(err)
	assert.Equal(1, len(n.Tuples))
	assert.Equal(uint16(bgp.AFI_IP), n.Tuples[0].AFI)
	assert.Equal(uint8(bgp.SAFI_UNICAST), n.Tuples[0].SAFI)
	assert.Equal(uint8(0x80), n.Tuples[0].Flags)
	assert.Equal(uint32(90), n.Tuples[0].RestartTime)

	output := NewLongLivedGracefulRestartCapability(n)
	assert.Equal(input, output)
}

func Test_RouteRefreshCiscoCapability(t *testing.T) {
	assert := assert.New(t)

	input := &RouteRefreshCiscoCapability{}

	n, err := input.ToNative()
	assert.Nil(err)

	output := NewRouteRefreshCiscoCapability(n)
	assert.Equal(input, output)
}

func Test_UnknownCapability(t *testing.T) {
	assert := assert.New(t)

	input := &UnknownCapability{
		Code:  0xff,
		Value: []byte{0x11, 0x22, 0x33, 0x44},
	}

	n, err := input.ToNative()
	assert.Nil(err)
	assert.Equal(bgp.BGPCapabilityCode(0xff), n.CapCode)
	assert.Equal([]byte{0x11, 0x22, 0x33, 0x44}, n.CapValue)

	output := NewUnknownCapability(n)
	assert.Equal(input, output)
}
