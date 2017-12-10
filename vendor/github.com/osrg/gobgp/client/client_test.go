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

package client

import (
	"testing"
	"time"

	api "github.com/osrg/gobgp/api"
	"github.com/osrg/gobgp/config"
	"github.com/osrg/gobgp/server"
	"github.com/stretchr/testify/assert"
)

func TestGetNeighbor(test *testing.T) {
	assert := assert.New(test)
	s := server.NewBgpServer()
	go s.Serve()
	g := api.NewGrpcServer(s, ":50051")
	go g.Serve()
	time.Sleep(time.Second)
	cli, err := New("")
	assert.Nil(err)
	err = cli.StartServer(&config.Global{
		Config: config.GlobalConfig{
			As:       1,
			RouterId: "1.1.1.1",
			Port:     1790,
		},
	})
	assert.Nil(err)
	err = cli.AddNeighbor(&config.Neighbor{
		Config: config.NeighborConfig{
			NeighborAddress: "10.0.0.1",
			PeerAs:          2,
		},
	})
	assert.Nil(err)
	_, err = cli.GetNeighbor("10.0.0.1")
	assert.Nil(err)
	_, err = cli.GetNeighbor("10.0.0.2")
	assert.Equal(err.Error(), "not found neighbor 10.0.0.2")
}
