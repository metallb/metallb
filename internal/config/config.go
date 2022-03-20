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

package config // import "go.universe.tf/metallb/internal/config"

import (
	"fmt"
	"net"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	metallbv1beta2 "go.universe.tf/metallb/api/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/labels"
)

type ClusterResources struct {
	Pools              []metallbv1beta1.IPPool
	Peers              []metallbv1beta2.BGPPeer
	BFDProfiles        []metallbv1beta1.BFDProfile
	BGPAdvs            []metallbv1beta1.BGPAdvertisement
	L2Advs             []metallbv1beta1.L2Advertisement
	LegacyAddressPools []metallbv1beta1.AddressPool
}

// Config is a parsed MetalLB configuration.
type Config struct {
	// Routers that MetalLB should peer with.
	Peers []*Peer
	// Address pools from which to allocate load balancer IPs.
	Pools map[string]*Pool
	// BFD profiles that can be used by peers.
	BFDProfiles map[string]*BFDProfile
}

// Proto holds the protocol we are speaking.
type Proto string

// MetalLB supported protocols.
const (
	BGP    Proto = "bgp"
	Layer2 Proto = "layer2"
)

var Protocols = []Proto{
	BGP, Layer2,
}

// Peer is the configuration of a BGP peering session.
type Peer struct {
	// AS number to use for the local end of the session.
	MyASN uint32
	// AS number to expect from the remote end of the session.
	ASN uint32
	// Address to dial when establishing the session.
	Addr net.IP
	// Source address to use when establishing the session.
	SrcAddr net.IP
	// Port to dial when establishing the session.
	Port uint16
	// Requested BGP hold time, per RFC4271.
	HoldTime time.Duration
	// Requested BGP keepalive time, per RFC4271.
	KeepaliveTime time.Duration
	// BGP router ID to advertise to the peer
	RouterID net.IP
	// Only connect to this peer on nodes that match one of these
	// selectors.
	NodeSelectors []labels.Selector
	// Authentication password for routers enforcing TCP MD5 authenticated sessions
	Password string
	// The optional BFD profile to be used for this BGP session
	BFDProfile string
	// Optional ebgp peer is multi-hops away.
	EBGPMultiHop bool
	// TODO: more BGP session settings
}

// Pool is the configuration of an IP address pool.
type Pool struct {
	// The addresses that are part of this pool, expressed as CIDR
	// prefixes. config.Parse guarantees that these are
	// non-overlapping, both within and between pools.
	CIDR []*net.IPNet
	// Some buggy consumer devices mistakenly drop IPv4 traffic for IP
	// addresses ending in .0 or .255, due to poor implementations of
	// smurf protection. This setting marks such addresses as
	// unusable, for maximum compatibility with ancient parts of the
	// internet.
	AvoidBuggyIPs bool
	// If false, prevents IP addresses to be automatically assigned
	// from this pool.
	AutoAssign bool

	// The list of BGPAdvertisements associated with this address pool.
	BGPAdvertisements []*BGPAdvertisement

	// The list of L2Advertisements associated with this address pool.
	L2Advertisements []*L2Advertisement

	cidrsPerAddresses map[string][]*net.IPNet
}

// BGPAdvertisement describes one translation from an IP address to a BGP advertisement.
type BGPAdvertisement struct {
	// Roll up the IP address into a CIDR prefix of this
	// length. Optional, defaults to 32 (i.e. no aggregation) if not
	// specified.
	AggregationLength int
	// Optional, defaults to 128 (i.e. no aggregation) if not
	// specified.
	AggregationLengthV6 int
	// Value of the LOCAL_PREF BGP path attribute. Used only when
	// advertising to IBGP peers (i.e. Peer.MyASN == Peer.ASN).
	LocalPref uint32
	// Value of the COMMUNITIES path attribute.
	Communities map[uint32]bool
}

type L2Advertisement struct {
}

// BFDProfile describes a BFD profile to be applied to a set of peers.
type BFDProfile struct {
	Name             string
	ReceiveInterval  *uint32
	TransmitInterval *uint32
	DetectMultiplier *uint32
	EchoInterval     *uint32
	EchoMode         bool
	PassiveMode      bool
	MinimumTTL       *uint32
}

// Parse loads and validates a Config from bs.
func For(resources ClusterResources, validate Validate, bgpType string) (*Config, error) {
	err := validate(resources)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		Pools:       map[string]*Pool{},
		BFDProfiles: map[string]*BFDProfile{},
	}

	for i, bfd := range resources.BFDProfiles {
		parsed, err := bfdProfileFromCR(bfd)
		if err != nil {
			return nil, fmt.Errorf("parsing bfd profile #%d: %s", i+1, err)
		}
		if _, ok := cfg.BFDProfiles[parsed.Name]; ok {
			return nil, fmt.Errorf("found duplicate bfd profile name %s", parsed.Name)
		}
		cfg.BFDProfiles[bfd.Name] = parsed
	}

	for i, p := range resources.Peers {
		err := p.ValidateBGPPeer(resources.Peers, bgpType == "frr")
		if err != nil {
			return nil, err
		}
		peer, err := peerFromCR(p)
		if err != nil {
			return nil, fmt.Errorf("parsing peer #%d: %s", i+1, err)
		}
		if peer.BFDProfile != "" {
			if _, ok := cfg.BFDProfiles[peer.BFDProfile]; !ok {
				return nil, fmt.Errorf("peer #%d referencing non existing bfd profile %s", i+1, peer.BFDProfile)
			}
		}
		for _, ep := range cfg.Peers {
			// TODO: Be smarter regarding conflicting peers. For example, two
			// peers could have a different hold time but they'd still result
			// in two BGP sessions between the speaker and the remote host.
			if reflect.DeepEqual(peer, ep) {
				return nil, fmt.Errorf("peer #%d already exists", i+1)
			}

		}
		cfg.Peers = append(cfg.Peers, peer)
	}

	communities := map[string]uint32{}
	// TODO CRDs add a CRD for communities
	/*
		for n, v := range rawConfig.BGPCommunities {
			c, err := ParseCommunity(v)
			if err != nil {
				return nil, fmt.Errorf("parsing community %q: %s", n, err)
			}
			communities[n] = c
		}
	*/

	for i, p := range resources.Pools {
		// Check that the pool isn't already defined
		if cfg.Pools[p.Name] != nil {
			return nil, fmt.Errorf("duplicate definition of pool %q", p.Name)
		}

		err = p.ValidateIPPool(false, resources.Pools, resources.LegacyAddressPools)
		if err != nil {
			return nil, err
		}

		pool, err := addressPoolFromCR(p, communities)
		if err != nil {
			return nil, fmt.Errorf("parsing address pool #%d: %s", i+1, err)
		}

		cfg.Pools[p.Name] = pool
	}

	for _, l2Adv := range resources.L2Advs {
		adv := l2AdvertisementFromCR(l2Adv)
		// No pool selector means select all pools
		if len(l2Adv.Spec.IPPools) == 0 {
			for _, pool := range cfg.Pools {
				if !containsAdvertisement(pool.L2Advertisements, adv) {
					pool.L2Advertisements = append(pool.L2Advertisements, adv)
				}
			}
			continue
		}
		for _, poolName := range l2Adv.Spec.IPPools {
			if pool, ok := cfg.Pools[poolName]; ok {
				if !containsAdvertisement(pool.L2Advertisements, adv) {
					pool.L2Advertisements = append(pool.L2Advertisements, adv)
				}
			}
		}
	}

	err = validateDuplicateBGPAdvertisements(resources.BGPAdvs)
	if err != nil {
		return nil, err
	}

	for _, bgpAdv := range resources.BGPAdvs {
		err = bgpAdv.ValidateBGPAdv(false, resources.BGPAdvs, resources.Pools)
		if err != nil {
			return nil, err
		}

		adv, err := bgpAdvertisementFromCR(bgpAdv, communities)
		if err != nil {
			return nil, err
		}
		// No pool selector means select all pools
		if len(bgpAdv.Spec.IPPools) == 0 {
			for _, pool := range cfg.Pools {
				pool.BGPAdvertisements = append(pool.BGPAdvertisements, adv)
			}
			continue
		}
		for _, poolName := range bgpAdv.Spec.IPPools {
			if pool, ok := cfg.Pools[poolName]; ok {
				pool.BGPAdvertisements = append(pool.BGPAdvertisements, adv)
			}
		}
	}

	for i, p := range resources.LegacyAddressPools {
		// Check that the pool isn't already defined
		if cfg.Pools[p.Name] != nil {
			return nil, fmt.Errorf("duplicate definition of pool %q", p.Name)
		}

		err = p.ValidateAddressPool(false, resources.LegacyAddressPools, resources.Pools)
		if err != nil {
			return nil, err
		}

		pool, err := addressPoolFromLegacyCR(p, communities)
		if err != nil {
			return nil, fmt.Errorf("parsing address pool #%d: %s", i+1, err)
		}

		cfg.Pools[p.Name] = pool
	}

	return cfg, nil
}

func peerFromCR(p metallbv1beta2.BGPPeer) (*Peer, error) {
	ip := net.ParseIP(p.Spec.Address)
	holdTime := p.Spec.HoldTime.Duration
	if holdTime == 0 {
		holdTime = 90 * time.Second
	}
	keepaliveTime := p.Spec.KeepaliveTime.Duration
	if keepaliveTime == 0 {
		keepaliveTime = holdTime / 3
	}

	// Ideally we would set a default RouterID here, instead of having
	// to do it elsewhere in the code. Unfortunately, we don't know
	// the node IP here.
	var routerID net.IP
	if p.Spec.RouterID != "" {
		routerID = net.ParseIP(p.Spec.RouterID)
	}
	var src net.IP
	if p.Spec.SrcAddress != "" {
		src = net.ParseIP(p.Spec.SrcAddress)
	}

	var nodeSels []labels.Selector
	for _, s := range p.Spec.NodeSelectors {
		labelSelector, err := metav1.LabelSelectorAsSelector(&s)
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to convert peer %s node selector", p.Name)
		}
		nodeSels = append(nodeSels, labelSelector)
	}
	if len(nodeSels) == 0 {
		nodeSels = []labels.Selector{labels.Everything()}
	}

	var password string
	if p.Spec.Password != "" {
		password = p.Spec.Password
	}

	return &Peer{
		MyASN:         p.Spec.MyASN,
		ASN:           p.Spec.ASN,
		Addr:          ip,
		SrcAddr:       src,
		Port:          p.Spec.Port,
		HoldTime:      holdTime,
		KeepaliveTime: keepaliveTime,
		RouterID:      routerID,
		NodeSelectors: nodeSels,
		Password:      password,
		BFDProfile:    p.Spec.BFDProfile,
		EBGPMultiHop:  p.Spec.EBGPMultiHop,
	}, nil
}

func addressPoolFromCR(p metallbv1beta1.IPPool, bgpCommunities map[string]uint32) (*Pool, error) {
	ret := &Pool{
		AvoidBuggyIPs: p.Spec.AvoidBuggyIPs,
		AutoAssign:    true,
	}

	if p.Spec.AutoAssign != nil {
		ret.AutoAssign = *p.Spec.AutoAssign
	}

	ret.cidrsPerAddresses = map[string][]*net.IPNet{}
	for _, cidr := range p.Spec.Addresses {
		nets, err := metallbv1beta1.ParseCIDR(cidr)
		if err != nil {
			return nil, fmt.Errorf("invalid CIDR %q in pool %q: %s", cidr, p.Name, err)
		}
		ret.CIDR = append(ret.CIDR, nets...)
		ret.cidrsPerAddresses[cidr] = nets
	}

	return ret, nil
}

func addressPoolFromLegacyCR(p metallbv1beta1.AddressPool, bgpCommunities map[string]uint32) (*Pool, error) {
	ret := &Pool{
		AvoidBuggyIPs: p.Spec.AvoidBuggyIPs,
		AutoAssign:    true,
	}

	if p.Spec.AutoAssign != nil {
		ret.AutoAssign = *p.Spec.AutoAssign
	}

	cidrsPerAddresses := map[string][]*net.IPNet{}
	for _, cidr := range p.Spec.Addresses {
		nets, err := metallbv1beta1.ParseCIDR(cidr)
		if err != nil {
			return nil, fmt.Errorf("invalid CIDR %q in pool %q: %s", cidr, p.Name, err)
		}
		ret.CIDR = append(ret.CIDR, nets...)
		cidrsPerAddresses[cidr] = nets
	}
	switch Proto(p.Spec.Protocol) {
	case Layer2:
		ret.L2Advertisements = []*L2Advertisement{{}}
	case BGP:
		ads, err := bgpAdvertisementsFromLegacyCR(p.Spec.BGPAdvertisements, cidrsPerAddresses, bgpCommunities)
		if err != nil {
			return nil, errors.Wrapf(err, "parsing BGP advertisements for %s", p.Name)
		}
		ret.BGPAdvertisements = ads
		if len(ads) == 0 { // Fill an empty bgpadvertisement to declare we want to advertise this pool
			ret.BGPAdvertisements = []*BGPAdvertisement{{}}
		}
	}

	return ret, nil
}

func bfdProfileFromCR(p metallbv1beta1.BFDProfile) (*BFDProfile, error) {
	if p.Name == "" {
		return nil, fmt.Errorf("missing bfd profile name")
	}
	res := &BFDProfile{}
	res.Name = p.Name
	var err error
	res.DetectMultiplier, err = bfdIntFromConfig(p.Spec.DetectMultiplier, 2, 255)
	if err != nil {
		return nil, errors.Wrap(err, "invalid detect multiplier value")
	}
	res.ReceiveInterval, err = bfdIntFromConfig(p.Spec.ReceiveInterval, 10, 60000)
	if err != nil {
		return nil, errors.Wrap(err, "invalid receive interval value")
	}
	res.TransmitInterval, err = bfdIntFromConfig(p.Spec.TransmitInterval, 10, 60000)
	if err != nil {
		return nil, errors.Wrap(err, "invalid transmit interval value")
	}
	res.MinimumTTL, err = bfdIntFromConfig(p.Spec.MinimumTTL, 1, 254)
	if err != nil {
		return nil, errors.Wrap(err, "invalid minimum ttl value")
	}
	res.EchoInterval, err = bfdIntFromConfig(p.Spec.EchoInterval, 10, 60000)
	if err != nil {
		return nil, errors.Wrap(err, "invalid echo interval value")
	}
	if p.Spec.EchoMode != nil {
		res.EchoMode = *p.Spec.EchoMode
	}
	if p.Spec.PassiveMode != nil {
		res.PassiveMode = *p.Spec.PassiveMode
	}

	return res, nil
}

func l2AdvertisementFromCR(crdAd metallbv1beta1.L2Advertisement) *L2Advertisement {
	return &L2Advertisement{}
}

func bgpAdvertisementFromCR(crdAd metallbv1beta1.BGPAdvertisement, communities map[string]uint32) (*BGPAdvertisement, error) {
	ad := &BGPAdvertisement{
		AggregationLength:   32,
		AggregationLengthV6: 128,
		LocalPref:           0,
		Communities:         map[uint32]bool{},
	}

	if crdAd.Spec.AggregationLength != nil {
		ad.AggregationLength = int(*crdAd.Spec.AggregationLength) // TODO CRD cast
	}

	if crdAd.Spec.AggregationLengthV6 != nil {
		ad.AggregationLengthV6 = int(*crdAd.Spec.AggregationLengthV6) // TODO CRD cast
	}

	ad.LocalPref = crdAd.Spec.LocalPref

	for _, c := range crdAd.Spec.Communities {
		if v, ok := communities[c]; ok {
			ad.Communities[v] = true
		} else {
			v, err := ParseCommunity(c)
			if err != nil {
				return nil, fmt.Errorf("invalid community %q in BGP advertisement: %s", c, err)
			}
			ad.Communities[v] = true
		}
	}
	return ad, nil
}

func bgpAdvertisementsFromLegacyCR(ads []metallbv1beta1.LegacyBgpAdvertisement, cidrsPerAddresses map[string][]*net.IPNet, communities map[string]uint32) ([]*BGPAdvertisement, error) {
	if len(ads) == 0 {
		return []*BGPAdvertisement{
			{
				AggregationLength:   32,
				AggregationLengthV6: 128,
				LocalPref:           0,
				Communities:         map[uint32]bool{},
			},
		}, nil
	}

	var ret []*BGPAdvertisement
	for _, crdAd := range ads {
		ad := &BGPAdvertisement{
			AggregationLength:   32,
			AggregationLengthV6: 128,
			LocalPref:           0,
			Communities:         map[uint32]bool{},
		}

		if crdAd.AggregationLength != nil {
			ad.AggregationLength = int(*crdAd.AggregationLength)
		}
		if crdAd.AggregationLengthV6 != nil {
			ad.AggregationLengthV6 = int(*crdAd.AggregationLengthV6)
		}

		ad.LocalPref = crdAd.LocalPref

		for _, c := range crdAd.Communities {
			if v, ok := communities[c]; ok {
				ad.Communities[v] = true
			} else {
				v, err := ParseCommunity(c)
				if err != nil {
					return nil, fmt.Errorf("invalid community %q in BGP advertisement: %s", c, err)
				}
				ad.Communities[v] = true
			}
		}

		ret = append(ret, ad)
	}

	return ret, nil
}

func ParseCommunity(c string) (uint32, error) {
	fs := strings.Split(c, ":")
	if len(fs) != 2 {
		return 0, fmt.Errorf("invalid community string %q", c)
	}
	a, err := strconv.ParseUint(fs[0], 10, 16)
	if err != nil {
		return 0, fmt.Errorf("invalid first section of community %q: %s", fs[0], err)
	}
	b, err := strconv.ParseUint(fs[1], 10, 16)
	if err != nil {
		return 0, fmt.Errorf("invalid second section of community %q: %s", fs[1], err)
	}

	return (uint32(a) << 16) + uint32(b), nil
}

func CommunityToString(c uint32) string {
	upperVal := c >> 16
	lowerVal := c & 0xFFFF
	return fmt.Sprintf("%d:%d", upperVal, lowerVal)
}

func bfdIntFromConfig(value *uint32, min, max uint32) (*uint32, error) {
	if value == nil {
		return nil, nil
	}
	if *value < min || *value > max {
		return nil, fmt.Errorf("invalid value %d, must be in %d-%d range", *value, min, max)
	}
	return value, nil
}

func validateDuplicateBGPAdvertisements(ads []metallbv1beta1.BGPAdvertisement) error {
	for i := 0; i < len(ads); i++ {
		for j := i + 1; j < len(ads); j++ {
			if reflect.DeepEqual(ads[i], ads[j]) {
				return fmt.Errorf("duplicate definition of bgpadvertisements. advertisement %d and %d are equal", i+1, j+1)
			}
		}
	}
	return nil
}

// TODO: Currently there are no fields in the L2Advertisement, so it is enough to check
// if the list is not empty. This must be extended if we are going to add new fields to the l2 advertisements.
func containsAdvertisement(advs []*L2Advertisement, toCheck *L2Advertisement) bool {
	for _, adv := range advs {
		if *adv == *toCheck {
			return true
		}
	}
	return false
}
