// SPDX-License-Identifier:Apache-2.0

package config

import (
	"errors"
	"fmt"
	"reflect"

	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	metallbv1beta2 "go.universe.tf/metallb/api/v1beta2"
	"go.universe.tf/metallb/internal/bgp/community"
	"go.universe.tf/metallb/internal/ipfamily"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type Validate func(ClusterResources) error

func ValidationFor(bgpImpl string) Validate {
	switch bgpImpl {
	case "frr":
		return DiscardNativeOnly
	case "frr-k8s":
		return DiscardNativeOnly
	case "native":
		return DiscardFRROnly
	}
	return DontValidate
}

// DiscardFRROnly returns an error if the current configFile contains
// any options that are available only in the FRR implementation.
func DiscardFRROnly(c ClusterResources) error {
	for _, p := range c.Peers {
		if p.Spec.BFDProfile != "" {
			return fmt.Errorf("peer %s has bfd-profile set on native bgp mode", p.Spec.Address)
		}
		if p.Spec.KeepaliveTime != nil && p.Spec.KeepaliveTime.Duration != 0 {
			return fmt.Errorf("peer %s has keepalive-time set on native bgp mode", p.Spec.Address)
		}
		if p.Spec.VRFName != "" {
			return fmt.Errorf("peer %s has vrf set on native bgp mode", p.Spec.Address)
		}
		if p.Spec.ConnectTime != nil {
			return fmt.Errorf("peer %s has connect time set on native bgp mode", p.Spec.Address)
		}
		if p.Spec.EnableGracefulRestart {
			return fmt.Errorf("peer %s has EnableGracefulRestart flag set on native bgp mode", p.Spec.Address)
		}
		if p.Spec.DisableMP {
			return fmt.Errorf("peer %s has disable MP flag set on native bgp mode", p.Spec.Address)
		}
		if p.Spec.DualStackAddressFamily {
			return fmt.Errorf("peer %s has dualstackaddressfamily flag set on native bgp mode", p.Spec.Address)
		}
		if p.Spec.DynamicASN != "" {
			return fmt.Errorf("peer %s has dynamicASN set on native bgp mode", p.Spec.Address)
		}
		if p.Spec.Interface != "" {
			return fmt.Errorf("peer %s has interface set on native bgp mode", p.Spec.Address)
		}
	}
	if len(c.BFDProfiles) > 0 {
		return errors.New("bfd profiles section set")
	}
	// Only IPv4 BGP advertisements are supported in native mode.
	if err := findIPv6BGPAdvertisement(c); err != nil {
		return err
	}
	// Only legacy type communities are supported in native mode.
	return findNonLegacyCommunity(c)
}

// findIPv6BGPAdvertisement checks for IPv6 addresses. If it finds at least one IPv6 BGP advertisement, it will throw
// and error as it's not supported in native mode.
func findIPv6BGPAdvertisement(c ClusterResources) error {
	if len(c.BGPAdvs) == 0 {
		return nil
	}

	bgpSelectors, err := poolSelectorsForBGP(c)
	if err != nil {
		return err
	}

	for _, p := range c.Pools {
		if !bgpSelectors.matchesPool(p) {
			continue
		}

		for _, cidr := range p.Spec.Addresses {
			nets, err := ParseCIDR(cidr)
			if err != nil {
				return fmt.Errorf("invalid CIDR %q in pool %q: %s", cidr, p.Name, err)
			}
			for _, n := range nets {
				if n.IP.To4() == nil {
					return fmt.Errorf("pool %q has ipv6 CIDR %s, native bgp mode does not support ipv6", p.Name, n)
				}
			}
		}
	}
	return nil
}

// findNonLegacyCommunity returns an error if it can find a non legacy community. If a community string can not be
// parsed, the string will be ignored.
func findNonLegacyCommunity(c ClusterResources) error {
	for _, adv := range c.BGPAdvs {
		for _, cs := range adv.Spec.Communities {
			c, err := community.New(cs)
			if err != nil {
				// Skip aliases.
				continue
			}
			if !community.IsLegacy(c) {
				return fmt.Errorf("native BGP mode only supports legacy communities, BGP advertisement %q "+
					"has non legacy community %q", adv.Name, cs)
			}
		}
	}
	for _, co := range c.Communities {
		for _, cs := range co.Spec.Communities {
			c, err := community.New(cs.Value)
			if err != nil {
				// Skip it if we cannot parse it - the purpose of this very verification is not to make sure that
				// a string can be parsed or not.
				continue
			}
			if !community.IsLegacy(c) {
				return fmt.Errorf("native BGP mode only supports legacy communities, BGP community CR %q "+
					"has non legacy community %q", co.Name, cs)
			}
		}
	}
	return nil
}

// DontValidate is a Validate function that always returns
// success.
func DontValidate(c ClusterResources) error {
	return nil
}

// DiscardNativeOnly returns an error if the current configFile contains
// any options that are available only in the native implementation.
func DiscardNativeOnly(c ClusterResources) error {
	if len(c.Peers) > 1 {
		// Group peers by their identifier (address/interface + VRF)
		peersByID := make(map[string][]metallbv1beta2.BGPPeer)
		routerID := c.Peers[0].Spec.RouterID

		for _, p := range c.Peers {
			if p.Spec.RouterID != routerID {
				return fmt.Errorf("peer %s has RouterID different from %s, in FRR mode all RouterID must be equal", p.Spec.RouterID, routerID)
			}
			peerID := peerIdentifier(p.Spec)
			peersByID[peerID] = append(peersByID[peerID], p)
		}

		// Check duplicate peers
		for peerID, peers := range peersByID {
			if len(peers) > 1 {
				// Multiple peers with the same address/interface + VRF
				if err := validateDuplicatePeers(peerID, peers); err != nil {
					return err
				}
			}
		}
	}
	for _, p := range c.Peers {
		for _, p1 := range c.Peers[1:] {
			if p.Spec.MyASN != p1.Spec.MyASN &&
				p.Spec.VRFName == p1.Spec.VRFName {
				return fmt.Errorf("peer %s has myAsn different from %s, in FRR mode all myAsn must be equal for the same VRF", p.Spec.Address, p1.Spec.Address)
			}
		}
	}
	return nil
}

// validateConfig is meant to validate all the inter-dependencies of a parsed configuration.
// In this case, we ensure that bfd echo is not enabled on a v6 pool.
func validateConfig(cfg *Config) error {
	for _, p := range cfg.Pools.ByName {
		containsV6 := false
		for _, cidr := range p.CIDR {
			if ipfamily.ForCIDR(cidr) == ipfamily.IPv6 {
				containsV6 = true
				break
			}
		}
		if !containsV6 { // we only care about v6 advertisements
			continue
		}
		for _, a := range p.BGPAdvertisements {
			if len(a.Peers) == 0 { // all peers
				for _, peer := range cfg.Peers {
					if hasBFDEcho(peer, cfg.BFDProfiles) {
						return fmt.Errorf("pool %s has bgpadvertisement %s which references peer %s which has bfd echo enabled, which is not possible", p.Name, a.Name, peer.Name)
					}
				}
				continue
			}
			for _, peerName := range a.Peers {
				if peer, ok := cfg.Peers[peerName]; ok {
					if hasBFDEcho(peer, cfg.BFDProfiles) {
						return fmt.Errorf("pool %s has bgpadvertisement %s which references peer %s which has bfd echo enabled, which is not possible", p.Name, a.Name, peer.Name)
					}
				}
			}
		}
	}
	return nil
}

func hasBFDEcho(peer *Peer, bfdProfiles map[string]*BFDProfile) bool {
	profile, ok := bfdProfiles[peer.BFDProfile]
	if !ok {
		return false
	}
	if profile.EchoMode {
		return true
	}
	return false
}

func peerIdentifier(peer metallbv1beta2.BGPPeerSpec) string {
	id := peer.Address
	if peer.Address == "" {
		id = peer.Interface
	}
	return fmt.Sprintf("%s-%s", id, peer.VRFName)
}

// validateDuplicatePeers validates that duplicate peers (same address/interface + VRF)
// either have non-overlapping node selectors, or are compatible if they might overlap.
func validateDuplicatePeers(peerID string, peers []metallbv1beta2.BGPPeer) error {
	// Check if all duplicate peers have node selectors
	for _, p := range peers {
		if len(p.Spec.NodeSelectors) == 0 {
			return fmt.Errorf("duplicate peer %s has no nodeSelectors, in FRR mode each duplicate peer must have nodeSelectors to differentiate", p.Spec.Address)
		}
	}

	// For each pair of peers, check if they could overlap
	for i := range peers {
		for j := i + 1; j < len(peers); j++ {
			peer1 := peers[i].Spec
			peer2 := peers[j].Spec

			// Check if the node selectors might overlap
			// Note: We can't definitively determine overlap without knowing all node labels,
			// so we check if selectors are obviously disjoint. If uncertain, we require compatibility.
			canOverlap := nodeSelectorsCanOverlap(peer1.NodeSelectors, peer2.NodeSelectors)

			if canOverlap {
				// Selectors might overlap, so peers must be compatible
				if !arePeersCompatible(peer1, peer2) {
					return fmt.Errorf("duplicate peers with address/interface %s might select the same nodes but have incompatible configurations (different ASN, ports, timers, BFD profiles, etc.)", peerID)
				}
			}
		}
	}

	return nil
}

// arePeersCompatible returns true if two peer configurations are compatible
// (i.e., it would be safe for them to run on the same node).
// Two peers are compatible if they would create the exact same BGP session configuration.
// This means all fields must match except NodeSelectors (which determine node eligibility).
func arePeersCompatible(p1, p2 metallbv1beta2.BGPPeerSpec) bool {
	// Create copies and clear the NodeSelectors since those don't affect session compatibility
	p1Copy := p1
	p2Copy := p2
	p1Copy.NodeSelectors = nil
	p2Copy.NodeSelectors = nil

	return reflect.DeepEqual(p1Copy, p2Copy)
}

// nodeSelectorsCanOverlap returns true if two sets of node selectors might select
// overlapping nodes. This is a best-effort check - it returns true (assume overlap)
// unless it can definitively prove the selectors are mutually exclusive.
func nodeSelectorsCanOverlap(selectors1, selectors2 []metav1.LabelSelector) bool {
	// If either selector list is empty (matches all nodes), they definitely overlap
	if len(selectors1) == 0 || len(selectors2) == 0 {
		return true
	}

	// Each peer matches nodes that satisfy ANY of its selectors (OR logic)
	// Two peers can overlap if ANY selector from peer1 can match the same node as ANY selector from peer2

	// We can only prove non-overlap in simple cases:
	// - If selectors have contradicting labels (e.g., zone=a vs zone=b)

	// For now, we use a conservative approach: assume overlap unless we can prove otherwise
	// This means users need to ensure compatibility or use clearly disjoint selectors

	// Check each pair of selectors for obvious conflicts
	for _, sel1 := range selectors1 {
		for _, sel2 := range selectors2 {
			if !areLabelSelectorsObviouslyDisjoint(sel1, sel2) {
				// Can't prove these are disjoint, so they might overlap
				return true
			}
		}
	}

	// All selector pairs are obviously disjoint
	return false
}

// areLabelSelectorsObviouslyDisjoint returns true if two label selectors are
// clearly mutually exclusive (i.e., no node could match both).
// This only detects simple cases like: key=value1 vs key=value2 where value1 != value2
func areLabelSelectorsObviouslyDisjoint(sel1, sel2 metav1.LabelSelector) bool {
	// Check for contradicting MatchLabels
	for key, value1 := range sel1.MatchLabels {
		if value2, exists := sel2.MatchLabels[key]; exists && value1 != value2 {
			// Same key, different values = mutually exclusive
			return true
		}
	}

	// TODO: Could add more sophisticated checks for MatchExpressions
	// For now, we only detect the simple MatchLabels case

	return false
}

type poolSelector struct {
	byName   map[string]struct{}
	byLabels []labels.Selector
}

func (s poolSelector) matchesPool(p metallbv1beta1.IPAddressPool) bool {
	if len(s.byLabels) == 0 && len(s.byName) == 0 {
		return true
	}

	if _, ok := s.byName[p.Name]; ok {
		return true
	}
	for _, l := range s.byLabels {
		if l.Matches(labels.Set(p.Labels)) {
			return true
		}
	}
	return false
}

func poolSelectorsForBGP(c ClusterResources) (poolSelector, error) {
	selectedPools := make(map[string]struct{})
	poolsSelectors := []labels.Selector{}
	for _, adv := range c.BGPAdvs {
		if len(adv.Spec.IPAddressPools) == 0 &&
			len(adv.Spec.IPAddressPoolSelectors) == 0 {
			return poolSelector{}, nil // no selectors, let's catch em all!
		}
		for _, p := range adv.Spec.IPAddressPools {
			selectedPools[p] = struct{}{}
		}
		for _, selector := range adv.Spec.IPAddressPoolSelectors {
			l, err := metav1.LabelSelectorAsSelector(&selector)
			if err != nil {
				return poolSelector{}, fmt.Errorf("invalid label selector %v", selector)
			}
			poolsSelectors = append(poolsSelectors, l)
		}
	}
	return poolSelector{
		byName:   selectedPools,
		byLabels: poolsSelectors,
	}, nil
}
