// Copyright (c) 2019 Cisco and/or its affiliates.
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

package mock

import (
	"fmt"

	"git.fd.io/govpp.git/adapter"
	"git.fd.io/govpp.git/adapter/mock"
	govppapi "git.fd.io/govpp.git/api"
	"git.fd.io/govpp.git/core"
	"github.com/ligato/vpp-agent/tests/vppcallmock"
)

// GoVPPMux implements GoVPP Mux API with stats
type GoVPPMux struct {
	connection *core.Connection
	stats      *mock.StatsAdapter
}

// NewMockGoVPPMux prepares new mock of GoVPP multiplexor with given context
func NewMockGoVPPMux(ctx *vppcallmock.TestCtx) (*GoVPPMux, error) {
	connection, err := core.Connect(ctx.MockVpp)
	if err != nil {
		return nil, err
	}

	return &GoVPPMux{
		connection: connection,
		stats:      ctx.MockStats,
	}, nil
}

// NewAPIChannel calls the same method from connection
func (p *GoVPPMux) NewAPIChannel() (govppapi.Channel, error) {
	if p.connection == nil {
		return nil, fmt.Errorf("failed to create new VPP API channel, nil connection")
	}
	return p.connection.NewAPIChannel()
}

// NewAPIChannelBuffered calls the same method from connection
func (p *GoVPPMux) NewAPIChannelBuffered(reqChanBufSize, replyChanBufSize int) (govppapi.Channel, error) {
	if p.connection == nil {
		return nil, fmt.Errorf("failed to create new VPP API buffered channel, nil connection")
	}
	return p.connection.NewAPIChannelBuffered(reqChanBufSize, replyChanBufSize)
}

// ListStats lists stats from mocked stats API
func (p *GoVPPMux) ListStats(patterns ...string) ([]string, error) {
	if p.stats == nil {
		return nil, fmt.Errorf("failed to list VPP stats, nil stats adapter")
	}
	return p.stats.ListStats(patterns...)
}

// DumpStats dumps stats from mocked stats API
func (p *GoVPPMux) DumpStats(patterns ...string) ([]*adapter.StatEntry, error) {
	if p.stats == nil {
		return nil, fmt.Errorf("failed to dump VPP stats, nil stats adapter")
	}
	return p.stats.DumpStats(patterns...)
}

// MockStats allows to set required stats which are then returned by 'ListStats' or 'DumpStats'
func (p *GoVPPMux) MockStats(stats []*adapter.StatEntry) error {
	if p.stats == nil {
		return fmt.Errorf("failed to mock VPP stats, nil stats adapter")
	}
	p.stats.MockStats(stats)
	return nil
}

// Close connection
func (p *GoVPPMux) Close() {
	if p.connection != nil {
		p.connection.Disconnect()
	}
}
