// Copyright (C) 2015 Nippon Telegraph and Telephone Corporation.
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
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/osrg/gobgp/packet/mrt"
	"github.com/osrg/gobgp/table"
	"github.com/spf13/cobra"
)

func injectMrt() error {

	file, err := os.Open(mrtOpts.Filename)
	if err != nil {
		return fmt.Errorf("failed to open file: %s", err)
	}

	if mrtOpts.NextHop != nil && !mrtOpts.SkipV4 && !mrtOpts.SkipV6 {
		fmt.Println("You should probably specify either --no-ipv4 or --no-ipv6 when overwriting nexthop, unless your dump contains only one type of routes")
	}

	idx := 0
	if mrtOpts.QueueSize < 1 {
		return fmt.Errorf("Specified queue size is smaller than 1, refusing to run with unbounded memory usage")
	}

	ch := make(chan []*table.Path, mrtOpts.QueueSize)
	go func() {

		var peers []*mrt.Peer
		for {
			buf := make([]byte, mrt.MRT_COMMON_HEADER_LEN)
			_, err := file.Read(buf)
			if err == io.EOF {
				break
			} else if err != nil {
				exitWithError(fmt.Errorf("failed to read: %s", err))
			}

			h := &mrt.MRTHeader{}
			err = h.DecodeFromBytes(buf)
			if err != nil {
				exitWithError(fmt.Errorf("failed to parse"))
			}

			buf = make([]byte, h.Len)
			_, err = file.Read(buf)
			if err != nil {
				exitWithError(fmt.Errorf("failed to read"))
			}

			msg, err := mrt.ParseMRTBody(h, buf)
			if err != nil {
				printError(fmt.Errorf("failed to parse: %s", err))
				continue
			}

			if globalOpts.Debug {
				fmt.Println(msg)
			}

			if msg.Header.Type == mrt.TABLE_DUMPv2 {
				subType := mrt.MRTSubTypeTableDumpv2(msg.Header.SubType)
				switch subType {
				case mrt.PEER_INDEX_TABLE:
					peers = msg.Body.(*mrt.PeerIndexTable).Peers
					continue
				case mrt.RIB_IPV4_UNICAST, mrt.RIB_IPV4_UNICAST_ADDPATH:
					if mrtOpts.SkipV4 {
						continue
					}
				case mrt.RIB_IPV6_UNICAST, mrt.RIB_IPV6_UNICAST_ADDPATH:
					if mrtOpts.SkipV6 {
						continue
					}
				case mrt.GEO_PEER_TABLE:
					fmt.Printf("WARNING: Skipping GEO_PEER_TABLE: %s", msg.Body.(*mrt.GeoPeerTable))
				default:
					exitWithError(fmt.Errorf("unsupported subType: %v", subType))
				}

				if peers == nil {
					exitWithError(fmt.Errorf("not found PEER_INDEX_TABLE"))
				}

				rib := msg.Body.(*mrt.Rib)
				nlri := rib.Prefix

				paths := make([]*table.Path, 0, len(rib.Entries))

				for _, e := range rib.Entries {
					if len(peers) < int(e.PeerIndex) {
						exitWithError(fmt.Errorf("invalid peer index: %d (PEER_INDEX_TABLE has only %d peers)\n", e.PeerIndex, len(peers)))
					}
					source := &table.PeerInfo{
						AS: peers[e.PeerIndex].AS,
						ID: peers[e.PeerIndex].BgpId,
					}
					t := time.Unix(int64(e.OriginatedTime), 0)
					paths = append(paths, table.NewPath(source, nlri, false, e.PathAttributes, t, false))
				}
				if mrtOpts.NextHop != nil {
					for _, p := range paths {
						p.SetNexthop(mrtOpts.NextHop)
					}
				}

				if mrtOpts.Best {
					dst := table.NewDestination(nlri, 0)
					for _, p := range paths {
						dst.AddNewPath(p)
					}
					best, _, _ := dst.Calculate().GetChanges(table.GLOBAL_RIB_NAME, false)
					if best == nil {
						exitWithError(fmt.Errorf("Can't find the best %v", nlri))
					}
					paths = []*table.Path{best}
				}

				if idx >= mrtOpts.RecordSkip {
					ch <- paths
				}

				idx += 1
				if idx == mrtOpts.RecordCount+mrtOpts.RecordSkip {
					break
				}
			}
		}

		close(ch)
	}()

	stream, err := client.AddPathByStream()
	if err != nil {
		return fmt.Errorf("failed to add path: %s", err)
	}

	for paths := range ch {
		err = stream.Send(paths...)
		if err != nil {
			return fmt.Errorf("failed to send: %s", err)
		}
	}

	if err := stream.Close(); err != nil {
		return fmt.Errorf("failed to send: %s", err)
	}
	return nil
}

func NewMrtCmd() *cobra.Command {
	globalInjectCmd := &cobra.Command{
		Use: CMD_GLOBAL,
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) < 1 {
				exitWithError(fmt.Errorf("usage: gobgp mrt inject global <filename> [<count> [<skip>]]"))
			}
			mrtOpts.Filename = args[0]
			if len(args) > 1 {
				var err error
				mrtOpts.RecordCount, err = strconv.Atoi(args[1])
				if err != nil {
					exitWithError(fmt.Errorf("invalid count value: %s", args[1]))
				}
				if len(args) > 2 {
					mrtOpts.RecordSkip, err = strconv.Atoi(args[2])
					if err != nil {
						exitWithError(fmt.Errorf("invalid skip value: %s", args[2]))
					}
				}
			} else {
				mrtOpts.RecordCount = -1
				mrtOpts.RecordSkip = 0
			}
			err := injectMrt()
			if err != nil {
				exitWithError(err)
			}
		},
	}

	injectCmd := &cobra.Command{
		Use: CMD_INJECT,
	}
	injectCmd.AddCommand(globalInjectCmd)

	mrtCmd := &cobra.Command{
		Use: CMD_MRT,
	}
	mrtCmd.AddCommand(injectCmd)

	mrtCmd.PersistentFlags().BoolVarP(&mrtOpts.Best, "only-best", "", false, "inject only best paths")
	mrtCmd.PersistentFlags().BoolVarP(&mrtOpts.SkipV4, "no-ipv4", "", false, "Do not import IPv4 routes")
	mrtCmd.PersistentFlags().BoolVarP(&mrtOpts.SkipV6, "no-ipv6", "", false, "Do not import IPv6 routes")
	mrtCmd.PersistentFlags().IntVarP(&mrtOpts.QueueSize, "queue-size", "", 1<<10, "Maximum number of updates to keep queued")
	mrtCmd.PersistentFlags().IPVarP(&mrtOpts.NextHop, "nexthop", "", nil, "Overwrite nexthop")
	return mrtCmd
}
