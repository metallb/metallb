// Copyright 2017 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"crypto/sha256"
	"net"
	"sort"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"go.universe.tf/metallb/internal/config"
	"go.universe.tf/metallb/internal/k8s/epslices"
	"go.universe.tf/metallb/internal/layer2"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

type layer2Controller struct {
	announcer *layer2.Announce
	myNode    string
	sList     SpeakerList
}

func (c *layer2Controller) SetConfig(log.Logger, *config.Config) error {
	return nil
}

// usableNodes returns all nodes that have at least one fully ready
// endpoint on them.
// The speakers parameter is a map containing all the nodes with active speakers.
// If the speakers map is nil, it is ignored.
func usableNodes(eps epslices.EpsOrSlices, speakers map[string]bool) []string {
	usable := map[string]bool{}
	switch eps.Type {
	case epslices.Eps:
		for _, subset := range eps.EpVal.Subsets {
			for _, ep := range subset.Addresses {
				if ep.NodeName == nil {
					continue
				}
				if speakers != nil {
					if hasSpeaker := speakers[*ep.NodeName]; !hasSpeaker {
						continue
					}
				}
				if _, ok := usable[*ep.NodeName]; !ok {
					usable[*ep.NodeName] = true
				}
			}
		}
	case epslices.Slices:
		for _, slice := range eps.SlicesVal {
			for _, ep := range slice.Endpoints {
				if !epslices.IsConditionReady(ep.Conditions) {
					continue
				}
				if ep.NodeName == nil {
					continue
				}
				nodeName := *ep.NodeName
				if speakers != nil {
					if hasSpeaker := speakers[nodeName]; !hasSpeaker {
						continue
					}
				}
				if _, ok := usable[nodeName]; !ok {
					usable[nodeName] = true
				}
			}
		}
	}

	var ret []string
	for node, ok := range usable {
		if ok {
			ret = append(ret, node)
		}
	}

	return ret
}

func (c *layer2Controller) ShouldAnnounce(l log.Logger, name string, toAnnounce []net.IP, pool *config.Pool, svc *v1.Service, eps epslices.EpsOrSlices) string {
	if !activeEndpointExists(eps) { // no active endpoints, just return
		level.Debug(l).Log("event", "shouldannounce", "protocol", "l2", "message", "failed no active endpoints", "service", name)
		return "notOwner"
	}

	if !poolMatchesNodeL2(pool, c.myNode) {
		level.Debug(l).Log("event", "skipping should announce l2", "service", name, "reason", "pool not matching my node")
		return "notOwner"
	}

	// we select the nodes with at least one matching l2 advertisement
	forPool := speakersForPool(c.sList.UsableSpeakers(), pool)
	var nodes []string
	if svc.Spec.ExternalTrafficPolicy == v1.ServiceExternalTrafficPolicyTypeLocal {
		nodes = usableNodes(eps, forPool)
	} else {
		nodes = nodesWithActiveSpeakers(forPool)
	}
	// Using the first IP should work for both single and dual stack.
	ipString := toAnnounce[0].String()
	// Sort the slice by the hash of node + load balancer ips. This
	// produces an ordering of ready nodes that is unique to all the services
	// with the same ip.
	sort.Slice(nodes, func(i, j int) bool {
		hi := sha256.Sum256([]byte(nodes[i] + "#" + ipString))
		hj := sha256.Sum256([]byte(nodes[j] + "#" + ipString))

		return bytes.Compare(hi[:], hj[:]) < 0
	})

	// Are we first in the list? If so, we win and should announce.
	if len(nodes) > 0 && nodes[0] == c.myNode {
		return ""
	}

	// Either not eligible, or lost the election entirely.
	return "notOwner"
}

func (c *layer2Controller) SetBalancer(l log.Logger, name string, lbIPs []net.IP, pool *config.Pool, client service, svc *v1.Service) error {
	ifs := c.announcer.GetInterfaces()
	for _, lbIP := range lbIPs {
		ipAdv := ipAdvertisementFor(lbIP, c.myNode, pool.L2Advertisements)
		if !ipAdv.MatchInterfaces(ifs...) {
			level.Warn(l).Log("op", "SetBalancer", "protocol", "layer2", "service", name, "IPAdvertisement", ipAdv,
				"localIfs", ifs, "msg", "the specified interfaces used to announce LB IP don't exist")
			client.Errorf(svc, "announceFailed", "the interfaces specified by LB IP %q doesn't exist in assigned node %q with protocol %q", lbIP.String(), c.myNode, config.Layer2)
			continue
		}
		c.announcer.SetBalancer(name, ipAdv)
	}
	return nil
}

func (c *layer2Controller) DeleteBalancer(l log.Logger, name, reason string) error {
	if !c.announcer.AnnounceName(name) {
		return nil
	}
	c.announcer.DeleteBalancer(name)
	return nil
}

func (c *layer2Controller) SetNode(log.Logger, *v1.Node) error {
	c.sList.Rejoin()
	return nil
}

func ipAdvertisementFor(ip net.IP, localNode string, l2Advertisements []*config.L2Advertisement) layer2.IPAdvertisement {
	ifs := sets.Set[string]{}
	for _, l2 := range l2Advertisements {
		if matchNode := l2.Nodes[localNode]; !matchNode {
			continue
		}
		if l2.AllInterfaces {
			return layer2.NewIPAdvertisement(ip, true, sets.Set[string]{})
		}
		ifs = ifs.Insert(l2.Interfaces...)
	}
	return layer2.NewIPAdvertisement(ip, false, ifs)
}

// nodesWithActiveSpeakers returns the list of nodes with active speakers.
func nodesWithActiveSpeakers(speakers map[string]bool) []string {
	var ret []string
	for node := range speakers {
		ret = append(ret, node)
	}
	return ret
}

// activeEndpointExists returns true if at least one endpoint is active.
func activeEndpointExists(eps epslices.EpsOrSlices) bool {
	switch eps.Type {
	case epslices.Eps:
		for _, subset := range eps.EpVal.Subsets {
			if len(subset.Addresses) > 0 {
				return true
			}
		}
	case epslices.Slices:
		for _, slice := range eps.SlicesVal {
			for _, ep := range slice.Endpoints {
				if !epslices.IsConditionReady(ep.Conditions) {
					continue
				}
				return true
			}
		}
	}
	return false
}

func poolMatchesNodeL2(pool *config.Pool, node string) bool {
	for _, adv := range pool.L2Advertisements {
		if adv.Nodes[node] {
			return true
		}
	}
	return false
}

func speakersForPool(speakers map[string]bool, pool *config.Pool) map[string]bool {
	res := map[string]bool{}
	for s := range speakers {
		if poolMatchesNodeL2(pool, s) {
			res[s] = true
		}
	}
	return res
}
