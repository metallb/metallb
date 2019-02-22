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

// +build windows darwin

package vppapiclient

import (
	"git.fd.io/govpp.git/adapter"
)

// stubStatClient is just an stub adapter that does nothing. It builds only on Windows and OSX, where the real
// VPP stats API client adapter does not build. Its sole purpose is to make the compiler happy on Windows and OSX.
type stubStatClient struct{}

func NewStatClient(socketName string) adapter.StatsAPI {
	return new(stubStatClient)
}

func (*stubStatClient) Connect() error {
	return adapter.ErrNotImplemented
}

func (*stubStatClient) Disconnect() error {
	return nil
}

func (*stubStatClient) ListStats(patterns ...string) (statNames []string, err error) {
	return nil, adapter.ErrNotImplemented
}

func (*stubStatClient) DumpStats(patterns ...string) ([]*adapter.StatEntry, error) {
	return nil, adapter.ErrNotImplemented
}
