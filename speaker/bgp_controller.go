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
	"fmt"
	"net"
	"reflect"
	"sort"
	"strconv"

	"go.universe.tf/metallb/internal/bgp"
	bgpfrr "go.universe.tf/metallb/internal/bgp/frr"
	bgpfrrk8s "go.universe.tf/metallb/internal/bgp/frrk8s"
	bgpnative "go.universe.tf/metallb/internal/bgp/native"
	"go.universe.tf/metallb/internal/config"
	"go.universe.tf/metallb/internal/k8s/epslices"
	k8snodes "go.universe.tf/metallb/internal/k8s/nodes"
	v1 "k8s.io/api/core/v1"
	discovery "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/pkg/errors"
)

type bgpImplementation string

const (
	bgpNative bgpImplementation = "native"
	bgpFrr    bgpImplementation = "frr"
	bgpFrrK8s bgpImplementation = "frr-k8s"
)

type peer struct {
	cfg     *config.Peer
	session bgp.Session
}

type bgpController struct {
	logger          log.Logger
	myNode          string
	nodeLabels      labels.Set
	peers           []*peer
	svcAds          map[string][]*bgp.Advertisement
	bgpType         bgpImplementation
	sessionManager  bgp.SessionManager
	ignoreExcludeLB bool
}

func (c *bgpController) SetConfig(l log.Logger, cfg *config.Config) error {
	newPeers := make([]*peer, 0, len(cfg.Peers))
newPeers:
	for _, p := range cfg.Peers {
		for i, ep := range c.peers {
			if ep == nil {
				continue
			}
			if reflect.DeepEqual(p, ep.cfg) {
				newPeers = append(newPeers, ep)
				c.peers[i] = nil
				continue newPeers
			}
		}
		// No existing peers match, create a new one.
		newPeers = append(newPeers, &peer{
			cfg: p,
		})
	}

	oldPeers := c.peers
	c.peers = newPeers

	for _, p := range oldPeers {
		if p == nil {
			continue
		}
		level.Info(l).Log("event", "peerRemoved", "peer", p.cfg.Addr, "reason", "removedFromConfig", "msg", "peer deconfigured, closing BGP session")

		if p.session != nil {
			if err := p.session.Close(); err != nil {
				level.Error(l).Log("op", "setConfig", "error", err, "peer", p.cfg.Addr, "msg", "failed to shut down BGP session")
			}
		}
		level.Debug(l).Log("event", "peerRemoved", "peer", p.cfg.Addr, "reason", "removedFromConfig", "msg", "peer deconfigured, BGP session closed")
	}

	err := c.syncBFDProfiles(cfg.BFDProfiles)
	if err != nil {
		return errors.Wrap(err, "failed to sync bfd profiles")
	}
	err = c.sessionManager.SyncExtraInfo(cfg.BGPExtras)
	if err != nil {
		return errors.Wrap(err, "failed to sync extra info")
	}

	return c.syncPeers(l)
}

func (c *bgpController) SetEventCallback(callback func(interface{})) {
	c.sessionManager.SetEventCallback(callback)
}

// hasHealthyEndpoint return true if this node has at least one healthy endpoint.
// It only checks nodes matching the given filterNode function.
func hasHealthyEndpoint(eps []discovery.EndpointSlice, filterNode func(*string) bool) bool {
	ready := map[string]bool{}
	for _, slice := range eps {
		for _, ep := range slice.Endpoints {
			node := ep.NodeName
			if filterNode(node) {
				continue
			}
			for _, addr := range ep.Addresses {
				if _, ok := ready[addr]; !ok && epslices.IsConditionServing(ep.Conditions) {
					// Only set true if nothing else has expressed an
					// opinion. This means that false will take precedence
					// if there's any unready ports for a given endpoint.
					ready[addr] = true
				}
				if !epslices.IsConditionServing(ep.Conditions) {
					ready[addr] = false
				}
			}
		}
	}

	for _, r := range ready {
		if r {
			// At least one fully healthy endpoint on this machine.
			return true
		}
	}
	return false
}

func (c *bgpController) ShouldAnnounce(l log.Logger, name string, _ []net.IP, pool *config.Pool, svc *v1.Service, epSlices []discovery.EndpointSlice, nodes map[string]*v1.Node) string {
	if !poolMatchesNodeBGP(pool, c.myNode) {
		level.Debug(l).Log("event", "skipping should announce bgp", "service", name, "reason", "pool not matching my node")
		return "notOwner"
	}

	if k8snodes.IsNetworkUnavailable(nodes[c.myNode]) {
		level.Debug(l).Log("event", "skipping should announce bgp", "service", name, "reason", "speaker's node has NodeNetworkUnavailable condition")
		return "nodeNetworkUnavailable"
	}

	if !c.ignoreExcludeLB && k8snodes.IsNodeExcludedFromBalancers(nodes[c.myNode]) {
		level.Debug(l).Log("event", "skipping should announce bgp", "service", name, "reason", "speaker's node has labeled 'node.kubernetes.io/exclude-from-external-load-balancers'")
		return "nodeLabeledExcludeBalancers"
	}

	// Should we advertise?
	// Yes, if externalTrafficPolicy is
	//  Cluster && any healthy endpoint exists
	// or
	//  Local && there's a ready local endpoint.
	filterNode := func(toFilter *string) bool {
		if toFilter == nil || *toFilter != c.myNode {
			return true
		}
		return false
	}

	if svc.Spec.ExternalTrafficPolicy == v1.ServiceExternalTrafficPolicyTypeLocal && !hasHealthyEndpoint(epSlices, filterNode) {
		return "noLocalEndpoints"
	} else if !hasHealthyEndpoint(epSlices, func(toFilter *string) bool { return false }) {
		return "noEndpoints"
	}
	return ""
}

// Called when either the peer list or node labels have changed,
// implying that the set of running BGP sessions may need tweaking.
func (c *bgpController) syncPeers(l log.Logger) error {
	var (
		errs          int
		needUpdateAds bool
	)
	for _, p := range c.peers {
		// First, determine if the peering should be active for this
		// node.
		shouldRun := false
		if len(p.cfg.NodeSelectors) == 0 {
			shouldRun = true
		}
		for _, ns := range p.cfg.NodeSelectors {
			if ns.Matches(c.nodeLabels) {
				shouldRun = true
				break
			}
		}

		// Now, compare current state to intended state, and correct.
		if p.session != nil && !shouldRun {
			// Oops, session is running but shouldn't be. Shut it down.
			level.Info(l).Log("event", "peerRemoved", "peer", p.cfg.Addr, "reason", "filteredByNodeSelector", "msg", "peer deconfigured, closing BGP session")
			if err := p.session.Close(); err != nil {
				level.Error(l).Log("op", "syncPeers", "error", err, "peer", p.cfg.Addr, "msg", "failed to shut down BGP session")
			}
			p.session = nil
		} else if p.session == nil && shouldRun {
			// Session doesn't exist, but should be running. Create
			// it.
			level.Info(l).Log("event", "peerAdded", "peer", p.cfg.Addr, "msg", "peer configured, starting BGP session")
			var routerID net.IP
			if p.cfg.RouterID != nil {
				routerID = p.cfg.RouterID
			}
			s, err := c.sessionManager.NewSession(c.logger,
				bgp.SessionParameters{
					PeerAddress:   net.JoinHostPort(p.cfg.Addr.String(), strconv.Itoa(int(p.cfg.Port))),
					SourceAddress: p.cfg.SrcAddr,
					MyASN:         p.cfg.MyASN,
					RouterID:      routerID,
					PeerASN:       p.cfg.ASN,
					HoldTime:      p.cfg.HoldTime,
					KeepAliveTime: p.cfg.KeepaliveTime,
					ConnectTime:   p.cfg.ConnectTime,
					Password:      p.cfg.Password,
					PasswordRef:   p.cfg.PasswordRef,
					CurrentNode:   c.myNode,
					BFDProfile:    p.cfg.BFDProfile,
					EBGPMultiHop:  p.cfg.EBGPMultiHop,
					SessionName:   p.cfg.Name,
					VRFName:       p.cfg.VRF,
				},
			)

			if err != nil {
				level.Error(l).Log("op", "syncPeers", "error", err, "peer", p.cfg.Addr, "msg", "failed to create BGP session")
				errs++
			} else {
				p.session = s
				needUpdateAds = true
			}
		}
	}
	if needUpdateAds {
		// Some new sessions came up, resync advertisement state.
		if err := c.updateAds(); err != nil {
			level.Error(l).Log("op", "updateAds", "error", err, "msg", "failed to update BGP advertisements")
			return err
		}
	}
	if errs > 0 {
		return fmt.Errorf("%d BGP sessions failed to start", errs)
	}
	return nil
}

func (c *bgpController) syncBFDProfiles(profiles map[string]*config.BFDProfile) error {
	if len(profiles) == 0 {
		return nil
	}

	return c.sessionManager.SyncBFDProfiles(profiles)
}

func (c *bgpController) SetBalancer(l log.Logger, name string, lbIPs []net.IP, pool *config.Pool, _ service, _ *v1.Service) error {
	c.svcAds[name] = nil
	for _, lbIP := range lbIPs {
		for _, adCfg := range pool.BGPAdvertisements {
			// skipping if this node is not enabled for this advertisement
			if !adCfg.Nodes[c.myNode] {
				continue
			}
			m := net.CIDRMask(adCfg.AggregationLength, 32)
			if lbIP.To4() == nil {
				m = net.CIDRMask(adCfg.AggregationLengthV6, 128)
			}
			ad := &bgp.Advertisement{
				Prefix: &net.IPNet{
					IP:   lbIP.Mask(m),
					Mask: m,
				},
				LocalPref: adCfg.LocalPref,
			}
			if len(adCfg.Peers) > 0 {
				ad.Peers = make([]string, 0, len(adCfg.Peers))
				ad.Peers = append(ad.Peers, adCfg.Peers...)
			}
			for comm := range adCfg.Communities {
				ad.Communities = append(ad.Communities, comm)
			}
			sort.Slice(ad.Communities, func(i, j int) bool { return ad.Communities[i].LessThan(ad.Communities[j]) })
			c.svcAds[name] = append(c.svcAds[name], ad)
		}
	}

	if err := c.updateAds(); err != nil {
		return err
	}

	level.Info(l).Log("event", "updatedAdvertisements", "numAds", len(c.svcAds[name]), "msg", "making advertisements using BGP")
	return nil
}

func (c *bgpController) updateAds() error {
	var allAds []*bgp.Advertisement
	for _, ads := range c.svcAds {
		// This list might contain duplicates, but that's fine,
		// they'll get compacted by the session code when it's
		// calculating advertisements.
		//
		// TODO: be more intelligent about compacting advertisements
		// and detecting conflicting advertisements.
		allAds = append(allAds, ads...)
	}
	for _, peer := range c.peers {
		if peer.session == nil {
			continue
		}
		ads := adsForPeer(peer.cfg.Name, allAds)
		if err := peer.session.Set(ads...); err != nil {
			return err
		}
	}
	return nil
}

func adsForPeer(peerName string, ads []*bgp.Advertisement) []*bgp.Advertisement {
	res := []*bgp.Advertisement{}
	for _, a := range ads {
		if a.MatchesPeer(peerName) {
			res = append(res, a)
		}
	}
	if len(res) == 0 {
		return nil
	}
	return res
}

func (c *bgpController) DeleteBalancer(l log.Logger, name, reason string) error {
	if _, ok := c.svcAds[name]; !ok {
		return nil
	}
	delete(c.svcAds, name)
	return c.updateAds()
}

func (c *bgpController) SetNode(l log.Logger, node *v1.Node) error {
	if c.myNode != node.Name {
		return nil
	}
	nodeLabels := node.Labels
	if nodeLabels == nil {
		nodeLabels = map[string]string{}
	}
	ns := labels.Set(nodeLabels)
	if c.nodeLabels != nil && labels.Equals(c.nodeLabels, ns) {
		// Node labels unchanged, no action required.
		return nil
	}
	c.nodeLabels = ns
	level.Info(l).Log("event", "nodeLabelsChanged", "msg", "Node labels changed, resyncing BGP peers")
	return c.syncPeers(l)
}

// Create a new 'bgp.SessionManager' of type 'bgpType'.
var newBGP = func(cfg controllerConfig) bgp.SessionManager {
	switch cfg.bgpType {
	case bgpNative:
		return bgpnative.NewSessionManager(cfg.Logger)
	case bgpFrr:
		return bgpfrr.NewSessionManager(cfg.Logger, cfg.LogLevel)
	case bgpFrrK8s:
		return bgpfrrk8s.NewSessionManager(cfg.Logger, cfg.LogLevel, cfg.MyNode, cfg.Namespace)
	default:
		panic(fmt.Sprintf("unsupported BGP implementation type: %s", cfg.bgpType))
	}
}

func poolMatchesNodeBGP(pool *config.Pool, node string) bool {
	for _, adv := range pool.BGPAdvertisements {
		if adv.Nodes[node] {
			return true
		}
	}
	return false
}
