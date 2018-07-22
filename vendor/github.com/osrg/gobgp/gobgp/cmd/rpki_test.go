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

package cmd

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	api "github.com/osrg/gobgp/api"
	"github.com/osrg/gobgp/config"
	"github.com/osrg/gobgp/server"
)

func TestShowRPKITable(test *testing.T) {
	assert := assert.New(test)

	s := server.NewBgpServer()
	go s.Serve()

	g := api.NewGrpcServer(s, ":50051")
	go g.Serve()

	err := s.Start(&config.Global{
		Config: config.GlobalConfig{
			As:       1,
			RouterId: "1.1.1.1",
			Port:     -1,
		},
	})
	assert.Nil(err)
	defer s.Stop()

	// MF RPKI Project
	// http://www.mfeed.ad.jp/rpki/en/roa_cache/technical_info.html
	rpki := &config.RpkiServerConfig{
		Address: "210.173.170.254",
		Port:    323,
	}
	err = s.AddRpki(rpki)
	assert.Nil(err)

	globalOpts.Host = "127.0.0.1"
	globalOpts.Port = 50051
	client = newClient()
	defer client.Close()

	// Wait for downloading ROA info
	for i := 0; ; i++ {
		if servers, err := s.GetRpki(); err == nil && len(servers) > 0 {
			if servers[0].State.RecordsV4 > 0 {
				break
			}
		}
		if i > 10 {
			test.Error("timeout to download ROA info")
			break
		}
		time.Sleep(1 * time.Second)
	}

	err = showRPKITable(nil)
	assert.Nil(err)
}
