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
	"bytes"
	"fmt"
	"net"
	"reflect"
	"slices"
	"sort"
	"strings"
	"time"

	"errors"

	"github.com/mikioh/ipaddr"

	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	metallbv1beta2 "go.universe.tf/metallb/api/v1beta2"
	"go.universe.tf/metallb/internal/bgp/community"
	"go.universe.tf/metallb/internal/ipfamily"
	k8snodes "go.universe.tf/metallb/internal/k8s/nodes"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
)

type ClusterResources struct {
	Pools           []metallbv1beta1.IPAddressPool    `json:"ipaddresspools"`
	Peers           []metallbv1beta2.BGPPeer          `json:"bgppeers"`
	BFDProfiles     []metallbv1beta1.BFDProfile       `json:"bfdprofiles"`
	BGPAdvs         []metallbv1beta1.BGPAdvertisement `json:"bgpadvertisements"`
	L2Advs          []metallbv1beta1.L2Advertisement  `json:"l2advertisements"`
	Communities     []metallbv1beta1.Community        `json:"communities"`
	PasswordSecrets map[string]corev1.Secret          `json:"passwordsecrets"`
	Nodes           []corev1.Node                     `json:"nodes"`
	Namespaces      []corev1.Namespace                `json:"namespaces"`
	BGPExtras       corev1.ConfigMap                  `json:"bgpextras"`
}

// Config is a parsed MetalLB configuration.
type Config struct {
	// Routers that MetalLB should peer with.
	Peers map[string]*Peer
	// Address pools from which to allocate load balancer IPs.
	Pools *Pools
	// BFD profiles that can be used by peers.
	BFDProfiles map[string]*BFDProfile
	// Protocol dependent extra config. Currently used only by FRR
	BGPExtras string
}

// Pools contains address pools and its namespace/service specific allocations.
type Pools struct {
	// ByName a map containing all configured pools.
	ByName map[string]*Pool
	// ByNamespace contains pool names pinned to specific namespace.
	ByNamespace map[string][]string
	// ByServiceSelector contains pool names which has service selection labels.
	ByServiceSelector []string
}

// Proto holds the protocol we are speaking.
type Proto string

// MetalLB supported protocols.
const (
	BGP    Proto = "bgp"
	Layer2 Proto = "layer2"
)

const bgpExtrasField = "extras"

var Protocols = []Proto{
	BGP, Layer2,
}

// Peer is the configuration of a BGP peering session.
type Peer struct {
	// Peer name.
	Name string
	// AS number to use for the local end of the session.
	MyASN uint32
	// AS number to expect from the remote end of the session.
	ASN uint32
	// Detect the AS number to use for the remote end of the session.
	DynamicASN string
	// Address to dial when establishing the session.
	Addr net.IP
	// Source address to use when establishing the session.
	SrcAddr net.IP
	// Port to dial when establishing the session.
	Port uint16
	// Requested BGP hold time, per RFC4271.
	HoldTime *time.Duration
	// Requested BGP keepalive time, per RFC4271.
	KeepaliveTime *time.Duration
	// Requested BGP connect time, controls how long BGP waits between connection attempts to a neighbor.
	ConnectTime *time.Duration
	// BGP router ID to advertise to the peer
	RouterID net.IP
	// Only connect to this peer on nodes that match one of these
	// selectors.
	NodeSelectors []labels.Selector
	// Authentication password for routers enforcing TCP MD5 authenticated sessions
	Password string
	// The password set in the secret referenced by the secret reference, in clear text
	SecretPassword string
	// Optional reference to the secret that holds the password.
	PasswordRef corev1.SecretReference
	// The optional BFD profile to be used for this BGP session
	BFDProfile string
	// Optional EnableGracefulRestart enable BGP graceful restart functionality at the peer level.
	EnableGracefulRestart bool
	// Optional ebgp peer is multi-hops away.
	EBGPMultiHop bool
	// Optional name of the vrf to establish the session from
	VRF string
	// Option to disable MP BGP that will result in separation of IPv4 and IPv6 route exchanges into distinct BGP sessions.
	DisableMP bool
	// TODO: more BGP session settings
}

// Pool is the configuration of an IP address pool.
type Pool struct {
	// Pool Name
	Name string
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

	ServiceAllocations *ServiceAllocation
}

// ServiceAllocation makes ip pool allocation to specific namespace and/or service.
type ServiceAllocation struct {
	// The priority of ip pool for a given service allocation.
	Priority int
	// Set of namespaces on which ip pool can be attached.
	Namespaces sets.Set[string]
	// Service selectors to select service for which ip pool can be used
	// for ip allocation.
	ServiceSelectors []labels.Selector
}

// BGPAdvertisement describes one translation from an IP address to a BGP advertisement.
type BGPAdvertisement struct {
	// The name of the advertisement
	Name string
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
	Communities map[community.BGPCommunity]bool
	// The map of nodes allowed for this advertisement
	Nodes map[string]bool
	// Used to declare the intent of announcing IPs
	// only to the BGPPeers in this list.
	Peers []string
}

type L2Advertisement struct {
	// The map of nodes allowed for this advertisement
	Nodes map[string]bool
	// The interfaces in Nodes allowed for this advertisement
	Interfaces []string
	// AllInterfaces tells if all the interfaces are allowed for this advertisement
	AllInterfaces bool
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

func (p *Pools) IsEmpty(pool string) bool {
	return p.ByName[pool] == nil
}

// Parse loads and validates a Config from bs.
func For(resources ClusterResources, validate Validate) (*Config, error) {
	err := validate(resources)
	if err != nil {
		return nil, err
	}

	cfg := &Config{}

	cfg.BFDProfiles, err = bfdProfilesFor(resources)
	if err != nil {
		return nil, err
	}

	cfg.Peers, err = peersFor(resources, cfg.BFDProfiles)
	if err != nil {
		return nil, err
	}

	cfg.Pools, err = poolsFor(resources)
	if err != nil {
		return nil, err
	}

	cfg.BGPExtras = bgpExtrasFor(resources)
	if err != nil {
		return nil, err
	}

	err = validateConfig(cfg)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func bfdProfilesFor(resources ClusterResources) (map[string]*BFDProfile, error) {
	res := make(map[string]*BFDProfile)
	for i, bfd := range resources.BFDProfiles {
		parsed, err := bfdProfileFromCR(bfd)
		if err != nil {
			return nil, fmt.Errorf("parsing bfd profile #%d: %s", i+1, err)
		}
		if _, ok := res[parsed.Name]; ok {
			return nil, fmt.Errorf("found duplicate bfd profile name %s", parsed.Name)
		}
		res[bfd.Name] = parsed
	}
	return res, nil
}

func peersFor(resources ClusterResources, BFDProfiles map[string]*BFDProfile) (map[string]*Peer, error) {
	var res = make(map[string]*Peer)
	for _, p := range resources.Peers {
		peer, err := peerFromCR(p, resources.PasswordSecrets)
		if err != nil {
			return nil, errors.Join(err, fmt.Errorf("parsing peer %s", p.Name))
		}
		if peer.BFDProfile != "" {
			if _, ok := BFDProfiles[peer.BFDProfile]; !ok {
				return nil, TransientError{fmt.Sprintf("peer %s referencing non existing bfd profile %s", p.Name, peer.BFDProfile)}
			}
		}
		for _, ep := range res {
			// TODO: Be smarter regarding conflicting peers. For example, two
			// peers could have a different hold time but they'd still result
			// in two BGP sessions between the speaker and the remote host.
			if reflect.DeepEqual(peer, ep) {
				return nil, fmt.Errorf("peer %s already exists", p.Name)
			}
		}
		res[peer.Name] = peer
	}
	return res, nil
}

func poolsFor(resources ClusterResources) (*Pools, error) {
	pools := make(map[string]*Pool)
	communities, err := communitiesFromCrs(resources.Communities)
	if err != nil {
		return nil, err
	}

	var allCIDRs []*net.IPNet
	for _, p := range resources.Pools {
		pool, err := addressPoolFromCR(p, resources.Namespaces)
		if err != nil {
			return nil, fmt.Errorf("parsing address pool %s: %s", p.Name, err)
		}

		// Check that the pool isn't already defined
		if pools[p.Name] != nil {
			return nil, fmt.Errorf("duplicate definition of pool %q", p.Name)
		}

		// Check that all specified CIDR ranges are non-overlapping.
		for _, cidr := range pool.CIDR {
			for _, m := range allCIDRs {
				if cidrsOverlap(cidr, m) {
					return nil, fmt.Errorf("CIDR %q in pool %q overlaps with already defined CIDR %q", cidr, p.Name, m)
				}
			}
			// Check pool CIDR is not overlapping with Node IPs
			nodeIps := k8snodes.NodeIPsForFamily(resources.Nodes, ipfamily.ForCIDR(cidr))
			for _, nodeIP := range nodeIps {
				if cidr.Contains(nodeIP) {
					return nil, fmt.Errorf("pool cidr %q contains nodeIp %q", cidr, nodeIP)
				}
			}
			allCIDRs = append(allCIDRs, cidr)
		}

		pools[p.Name] = pool
	}

	err = setL2AdvertisementsToPools(resources.Pools, resources.L2Advs, resources.Nodes, pools)
	if err != nil {
		return nil, err
	}

	err = validateDuplicateBGPAdvertisements(resources.BGPAdvs)
	if err != nil {
		return nil, err
	}

	err = setBGPAdvertisementsToPools(resources.Pools, resources.BGPAdvs, resources.Nodes, pools, communities)
	if err != nil {
		return nil, err
	}

	return &Pools{ByName: pools, ByNamespace: poolsByNamespace(pools),
		ByServiceSelector: poolsByServiceSelector(pools)}, nil
}

func bgpExtrasFor(resources ClusterResources) string {
	if resources.BGPExtras.Data == nil {
		return ""
	}
	return resources.BGPExtras.Data[bgpExtrasField]
}

func communitiesFromCrs(cs []metallbv1beta1.Community) (map[string]community.BGPCommunity, error) {
	communities := map[string]community.BGPCommunity{}
	for _, c := range cs {
		for _, communityAlias := range c.Spec.Communities {
			v, err := community.New(communityAlias.Value)
			if err != nil {
				return nil, fmt.Errorf("parsing community %q: %s", communityAlias.Name, err)
			}
			if _, ok := communities[communityAlias.Name]; ok {
				return nil, fmt.Errorf("duplicate definition of community %q", communityAlias.Name)
			}
			communities[communityAlias.Name] = v
		}
	}
	return communities, nil
}

func peerFromCR(p metallbv1beta2.BGPPeer, passwordSecrets map[string]corev1.Secret) (*Peer, error) {
	if p.Spec.MyASN == 0 {
		return nil, errors.New("missing local ASN")
	}
	if p.Spec.ASN == 0 && p.Spec.DynamicASN == "" {
		return nil, errors.New("missing peer ASN and dynamicASN")
	}
	if p.Spec.ASN != 0 && p.Spec.DynamicASN != "" {
		return nil, errors.New("both peer ASN and dynamicASN specified")
	}
	if p.Spec.DynamicASN != "" && p.Spec.DynamicASN != metallbv1beta2.InternalASNMode && p.Spec.DynamicASN != metallbv1beta2.ExternalASNMode {
		return nil, fmt.Errorf("invalid dynamicASN %s", p.Spec.DynamicASN)
	}
	if p.Spec.ASN == p.Spec.MyASN && p.Spec.EBGPMultiHop {
		return nil, errors.New("invalid ebgp-multihop parameter set for an ibgp peer")
	}
	ip := net.ParseIP(p.Spec.Address)
	if ip == nil {
		return nil, fmt.Errorf("invalid BGPPeer address %q", p.Spec.Address)
	}

	holdTime, keepaliveTime, err := parseTimers(p.Spec.HoldTime, p.Spec.KeepaliveTime)
	if err != nil {
		return nil, fmt.Errorf("invalid BGPPeer timers: %w", err)
	}

	// Ideally we would set a default RouterID here, instead of having
	// to do it elsewhere in the code. Unfortunately, we don't know
	// the node IP here.
	var routerID net.IP
	if p.Spec.RouterID != "" {
		routerID = net.ParseIP(p.Spec.RouterID)
		if routerID == nil {
			return nil, fmt.Errorf("invalid router ID %q", p.Spec.RouterID)
		}
	}
	src := net.ParseIP(p.Spec.SrcAddress)
	if p.Spec.SrcAddress != "" && src == nil {
		return nil, fmt.Errorf("invalid source IP %q", p.Spec.SrcAddress)
	}

	err = validateLabelSelectorDuplicate(p.Spec.NodeSelectors, "nodeSelectors")
	if err != nil {
		return nil, err
	}

	var nodeSels []labels.Selector
	for _, s := range p.Spec.NodeSelectors {
		s := s // so we can use &s
		labelSelector, err := metav1.LabelSelectorAsSelector(&s)
		if err != nil {
			return nil, errors.Join(err, fmt.Errorf("failed to convert peer %s node selector", p.Name))
		}
		nodeSels = append(nodeSels, labelSelector)
	}
	if len(nodeSels) == 0 {
		nodeSels = []labels.Selector{labels.Everything()}
	}

	if p.Spec.Password != "" && p.Spec.PasswordSecret.Name != "" {
		return nil, fmt.Errorf("can not have both password and secret ref set in peer config %q/%q", p.Namespace, p.Name)
	}

	secretPassword := ""
	if p.Spec.PasswordSecret.Name != "" {
		secretPassword, err = passwordFromSecretForPeer(p, passwordSecrets)
		if err != nil {
			return nil, errors.Join(err, fmt.Errorf("failed to parse peer %s password secret", p.Name))
		}
	}

	var connectTime *time.Duration
	if p.Spec.ConnectTime != nil {
		connectTime = ptr.To(p.Spec.ConnectTime.Duration)
	}

	return &Peer{
		Name:                  p.Name,
		MyASN:                 p.Spec.MyASN,
		ASN:                   p.Spec.ASN,
		DynamicASN:            string(p.Spec.DynamicASN),
		Addr:                  ip,
		SrcAddr:               src,
		Port:                  p.Spec.Port,
		HoldTime:              holdTime,
		KeepaliveTime:         keepaliveTime,
		ConnectTime:           connectTime,
		RouterID:              routerID,
		NodeSelectors:         nodeSels,
		SecretPassword:        secretPassword,
		Password:              p.Spec.Password,
		PasswordRef:           p.Spec.PasswordSecret,
		BFDProfile:            p.Spec.BFDProfile,
		EnableGracefulRestart: p.Spec.EnableGracefulRestart,
		EBGPMultiHop:          p.Spec.EBGPMultiHop,
		VRF:                   p.Spec.VRFName,
		DisableMP:             p.Spec.DisableMP,
	}, nil
}

func passwordFromSecretForPeer(p metallbv1beta2.BGPPeer, passwordSecrets map[string]corev1.Secret) (string, error) {
	secret, ok := passwordSecrets[p.Spec.PasswordSecret.Name]
	if !ok {
		return "", TransientError{Message: fmt.Sprintf("secret ref not found for peer config %q/%q", p.Namespace, p.Name)}
	}

	if secret.Type != corev1.SecretTypeBasicAuth {
		return "", fmt.Errorf("secret type mismatch on %q/%q, type %q is expected ", secret.Namespace,
			secret.Name, corev1.SecretTypeBasicAuth)
	}

	srcPass, ok := secret.Data["password"]
	if !ok {
		return "", fmt.Errorf("password not specified in the secret %q/%q", secret.Namespace, secret.Name)
	}

	return string(srcPass), nil
}

func addressPoolFromCR(p metallbv1beta1.IPAddressPool, namespaces []corev1.Namespace) (*Pool, error) {
	if p.Name == "" {
		return nil, errors.New("missing pool name")
	}

	ret := &Pool{
		Name:          p.Name,
		AvoidBuggyIPs: p.Spec.AvoidBuggyIPs,
		AutoAssign:    true,
	}

	if p.Spec.AutoAssign != nil {
		ret.AutoAssign = *p.Spec.AutoAssign
	}

	if len(p.Spec.Addresses) == 0 {
		return nil, errors.New("pool has no prefixes defined")
	}

	ret.cidrsPerAddresses = map[string][]*net.IPNet{}
	for _, cidr := range p.Spec.Addresses {
		nets, err := ParseCIDR(cidr)
		if err != nil {
			return nil, fmt.Errorf("invalid CIDR %q in pool %q: %s", cidr, p.Name, err)
		}
		ret.CIDR = append(ret.CIDR, nets...)
		ret.cidrsPerAddresses[cidr] = nets
	}

	serviceAllocations, err := addressPoolServiceAllocationsFromCR(p, namespaces)
	if err != nil {
		return nil, err
	}
	ret.ServiceAllocations = serviceAllocations

	return ret, nil
}

func addressPoolServiceAllocationsFromCR(p metallbv1beta1.IPAddressPool, namespaces []corev1.Namespace) (*ServiceAllocation, error) {
	if p.Spec.AllocateTo == nil {
		return nil, nil
	}
	poolNamespaces := sets.Set[string]{}
	for _, poolNs := range p.Spec.AllocateTo.Namespaces {
		if poolNamespaces.Has(poolNs) {
			return nil, errors.New("duplicate definition in namespaces field")
		}
		poolNamespaces.Insert(poolNs)
	}
	err := validateLabelSelectorDuplicate(p.Spec.AllocateTo.NamespaceSelectors, "namespaceSelectors")
	if err != nil {
		return nil, err
	}
	err = validateLabelSelectorDuplicate(p.Spec.AllocateTo.ServiceSelectors, "serviceSelectors")
	if err != nil {
		return nil, err
	}
	serviceAllocations := &ServiceAllocation{Priority: p.Spec.AllocateTo.Priority, Namespaces: poolNamespaces}
	for i := range p.Spec.AllocateTo.NamespaceSelectors {
		l, err := metav1.LabelSelectorAsSelector(&p.Spec.AllocateTo.NamespaceSelectors[i])
		if err != nil {
			return nil, errors.Join(err, fmt.Errorf("invalid namespace label selector %v in ip pool %s", &p.Spec.AllocateTo.NamespaceSelectors[i], p.Name))
		}
		for _, ns := range namespaces {
			nsLabels := labels.Set(ns.Labels)
			if l.Matches(nsLabels) {
				serviceAllocations.Namespaces.Insert(ns.Name)
			}
		}
	}
	for i := range p.Spec.AllocateTo.ServiceSelectors {
		l, err := metav1.LabelSelectorAsSelector(&p.Spec.AllocateTo.ServiceSelectors[i])
		if err != nil {
			return nil, errors.Join(err, fmt.Errorf("invalid service label selector %v in ip pool %s", p.Spec.AllocateTo.ServiceSelectors[i], p.Name))
		}
		serviceAllocations.ServiceSelectors = append(serviceAllocations.ServiceSelectors, l)
	}
	return serviceAllocations, nil
}

func poolsByNamespace(pools map[string]*Pool) map[string][]string {
	var poolsForNamespace map[string][]string
	for _, pool := range pools {
		if pool.ServiceAllocations == nil {
			continue
		}
		if poolsForNamespace == nil && len(pool.ServiceAllocations.Namespaces) > 0 {
			poolsForNamespace = make(map[string][]string)
		}
		for namespace := range pool.ServiceAllocations.Namespaces {
			poolsForNamespace[namespace] = append(poolsForNamespace[namespace], pool.Name)
		}
	}
	return poolsForNamespace
}

func poolsByServiceSelector(pools map[string]*Pool) []string {
	var poolsByServiceSelector []string
	for _, pool := range pools {
		if pool.ServiceAllocations == nil || len(pool.ServiceAllocations.ServiceSelectors) == 0 {
			continue
		}
		poolsByServiceSelector = append(poolsByServiceSelector, pool.Name)
	}
	sort.Strings(poolsByServiceSelector)
	return poolsByServiceSelector
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
		return nil, errors.Join(err, fmt.Errorf("invalid detect multiplier value"))
	}
	res.ReceiveInterval, err = bfdIntFromConfig(p.Spec.ReceiveInterval, 10, 60000)
	if err != nil {
		return nil, errors.Join(err, fmt.Errorf("invalid receive interval value"))
	}
	res.TransmitInterval, err = bfdIntFromConfig(p.Spec.TransmitInterval, 10, 60000)
	if err != nil {
		return nil, errors.Join(err, fmt.Errorf("invalid transmit interval value"))
	}
	res.MinimumTTL, err = bfdIntFromConfig(p.Spec.MinimumTTL, 1, 254)
	if err != nil {
		return nil, errors.Join(err, fmt.Errorf("invalid minimum ttl value"))
	}
	res.EchoInterval, err = bfdIntFromConfig(p.Spec.EchoInterval, 10, 60000)
	if err != nil {
		return nil, errors.Join(err, fmt.Errorf("invalid echo interval value"))
	}
	if p.Spec.EchoMode != nil {
		res.EchoMode = *p.Spec.EchoMode
	}
	if p.Spec.PassiveMode != nil {
		res.PassiveMode = *p.Spec.PassiveMode
	}

	return res, nil
}

func setL2AdvertisementsToPools(ipPools []metallbv1beta1.IPAddressPool, l2Advs []metallbv1beta1.L2Advertisement,
	nodes []corev1.Node, ipPoolMap map[string]*Pool) error {
	for _, l2Adv := range l2Advs {
		adv, err := l2AdvertisementFromCR(l2Adv, nodes)
		if err != nil {
			return err
		}
		ipPoolsSelected, err := selectedPools(ipPools, l2Adv.Spec.IPAddressPoolSelectors)
		if err != nil {
			return err
		}
		// No pool selector means select all pools
		if len(l2Adv.Spec.IPAddressPools) == 0 && len(l2Adv.Spec.IPAddressPoolSelectors) == 0 {
			for _, pool := range ipPoolMap {
				if !containsAdvertisement(pool.L2Advertisements, adv) {
					pool.L2Advertisements = append(pool.L2Advertisements, adv)
				}
			}
			continue
		}
		for _, poolName := range append(l2Adv.Spec.IPAddressPools, ipPoolsSelected...) {
			if pool, ok := ipPoolMap[poolName]; ok {
				if !containsAdvertisement(pool.L2Advertisements, adv) {
					pool.L2Advertisements = append(pool.L2Advertisements, adv)
				}
			}
		}
	}
	return nil
}

func setBGPAdvertisementsToPools(ipPools []metallbv1beta1.IPAddressPool, bgpAdvs []metallbv1beta1.BGPAdvertisement,
	nodes []corev1.Node, ipPoolMap map[string]*Pool, communities map[string]community.BGPCommunity) error {
	for _, bgpAdv := range bgpAdvs {
		adv, err := bgpAdvertisementFromCR(bgpAdv, communities, nodes)
		if err != nil {
			return err
		}
		ipPoolsSelected, err := selectedPools(ipPools, bgpAdv.Spec.IPAddressPoolSelectors)
		if err != nil {
			return err
		}
		// No pool selector means select all pools
		if len(bgpAdv.Spec.IPAddressPools) == 0 && len(bgpAdv.Spec.IPAddressPoolSelectors) == 0 {
			for _, pool := range ipPoolMap {
				err := validateBGPAdvPerPool(adv, pool)
				if err != nil {
					return err
				}
				pool.BGPAdvertisements = append(pool.BGPAdvertisements, adv)
			}
			continue
		}
		for _, poolName := range append(bgpAdv.Spec.IPAddressPools, ipPoolsSelected...) {
			if pool, ok := ipPoolMap[poolName]; ok {
				err := validateBGPAdvPerPool(adv, pool)
				if err != nil {
					return err
				}
				pool.BGPAdvertisements = append(pool.BGPAdvertisements, adv)
			}
		}
	}
	return nil
}

func l2AdvertisementFromCR(crdAd metallbv1beta1.L2Advertisement, nodes []corev1.Node) (*L2Advertisement, error) {
	err := validateDuplicate(crdAd.Spec.IPAddressPools, "ipAddressPools")
	if err != nil {
		return nil, err
	}
	err = validateLabelSelectorDuplicate(crdAd.Spec.IPAddressPoolSelectors, "ipAddressPoolSelectors")
	if err != nil {
		return nil, err
	}
	err = validateLabelSelectorDuplicate(crdAd.Spec.NodeSelectors, "nodeSelectors")
	if err != nil {
		return nil, err
	}
	selected, err := selectedNodes(nodes, crdAd.Spec.NodeSelectors)
	if err != nil {
		return nil, errors.Join(err, fmt.Errorf("failed to parse node selector for %s", crdAd.Name))
	}
	l2 := &L2Advertisement{
		Nodes:      selected,
		Interfaces: crdAd.Spec.Interfaces,
	}
	if len(crdAd.Spec.Interfaces) == 0 {
		l2.AllInterfaces = true
	}
	return l2, nil
}

func bgpAdvertisementFromCR(crdAd metallbv1beta1.BGPAdvertisement, communities map[string]community.BGPCommunity, nodes []corev1.Node) (*BGPAdvertisement, error) {
	err := validateDuplicate(crdAd.Spec.IPAddressPools, "ipAddressPools")
	if err != nil {
		return nil, err
	}
	err = validateDuplicate(crdAd.Spec.Communities, "community")
	if err != nil {
		return nil, err
	}
	err = validateDuplicate(crdAd.Spec.Peers, "peers")
	if err != nil {
		return nil, err
	}
	err = validateLabelSelectorDuplicate(crdAd.Spec.IPAddressPoolSelectors, "ipAddressPoolSelectors")
	if err != nil {
		return nil, err
	}
	err = validateLabelSelectorDuplicate(crdAd.Spec.NodeSelectors, "nodeSelectors")
	if err != nil {
		return nil, err
	}

	ad := &BGPAdvertisement{
		Name:                crdAd.Name,
		AggregationLength:   32,
		AggregationLengthV6: 128,
		LocalPref:           0,
		Communities:         map[community.BGPCommunity]bool{},
	}

	if crdAd.Spec.AggregationLength != nil {
		ad.AggregationLength = int(*crdAd.Spec.AggregationLength) // TODO CRD cast
	}
	if ad.AggregationLength > 32 {
		return nil, fmt.Errorf("invalid aggregation length %q for IPv4", ad.AggregationLength)
	}
	if crdAd.Spec.AggregationLengthV6 != nil {
		ad.AggregationLengthV6 = int(*crdAd.Spec.AggregationLengthV6) // TODO CRD cast
		if ad.AggregationLengthV6 > 128 {
			return nil, fmt.Errorf("invalid aggregation length %q for IPv6", ad.AggregationLengthV6)
		}
	}

	ad.LocalPref = crdAd.Spec.LocalPref

	if len(crdAd.Spec.Peers) > 0 {
		ad.Peers = make([]string, 0, len(crdAd.Spec.Peers))
		ad.Peers = append(ad.Peers, crdAd.Spec.Peers...)
	}

	for _, c := range crdAd.Spec.Communities {
		v, err := getCommunityValue(c, communities)
		if err != nil {
			return nil, errors.Join(err, fmt.Errorf("invalid community %q in BGP advertisement", c))
		}
		ad.Communities[v] = true
	}

	selected, err := selectedNodes(nodes, crdAd.Spec.NodeSelectors)
	if err != nil {
		return nil, errors.Join(err, fmt.Errorf("failed to parse node selector for %s", crdAd.Name))
	}
	ad.Nodes = selected
	return ad, nil
}

// getCommunityValue returns the BGPCommunity from the communities map if it exists there. Otherwise, it creates a
// new BGP community object from the provided communityString.
func getCommunityValue(communityString string, communities map[string]community.BGPCommunity) (community.BGPCommunity, error) {
	if v, ok := communities[communityString]; ok {
		return v, nil
	}

	var c community.BGPCommunity
	c, err := community.New(communityString)
	if errors.Is(err, community.ErrInvalidCommunityValue) {
		return c, err
	}
	// Return TransientError on invalidCommunityFormat, in case it refers
	// a Community resource that doesn't exist yet.
	if errors.Is(err, community.ErrInvalidCommunityFormat) {
		return c, TransientError{err.Error()}
	}
	return c, nil
}

func parseTimers(ht, ka *metav1.Duration) (*time.Duration, *time.Duration, error) {
	if ht == nil && ka == nil {
		return nil, nil, nil
	}

	var holdTime *time.Duration
	var keepaliveTime *time.Duration
	if ht != nil && ka != nil {
		holdTime = &ht.Duration
		keepaliveTime = &ka.Duration
	}
	if ht != nil && ka == nil {
		holdTime = &ht.Duration
		keepaliveTime = ptr.To(ht.Duration / 3)
	}
	if ht == nil && ka != nil {
		holdTime = ptr.To(ka.Duration * 3)
		keepaliveTime = &ka.Duration
	}

	rounded := time.Duration(int(holdTime.Seconds())) * time.Second
	if rounded != 0 && rounded < 3*time.Second {
		return nil, nil, fmt.Errorf("invalid hold time %q: must be 0 or >=3s", ht)
	}

	if *keepaliveTime > *holdTime {
		return nil, nil, fmt.Errorf("invalid keepaliveTime %q, must be lower than holdTime %q", ka, ht)
	}

	return holdTime, keepaliveTime, nil
}

func validateBGPAdvPerPool(adv *BGPAdvertisement, pool *Pool) error {
	for addr, cidrs := range pool.cidrsPerAddresses {
		if len(cidrs) == 0 {
			continue
		}
		maxLength := adv.AggregationLength
		if cidrs[0].IP.To4() == nil {
			maxLength = adv.AggregationLengthV6
		}

		// in case of range format, we may have a set of cidrs associated to a given address.
		// We reject if none of the cidrs are compatible with the aggregation length.
		lowest := lowestMask(cidrs)
		if maxLength < lowest {
			return fmt.Errorf("invalid aggregation length %d: prefix %d in "+
				"this pool is more specific than the aggregation length for addresses %s", adv.AggregationLength, lowest, addr)
		}
	}

	// Verify that BGP ADVs set a unique local preference value per BGP update.
	for _, bgpAdv := range pool.BGPAdvertisements {
		if adv.LocalPref != bgpAdv.LocalPref {
			if !advertisementsAreCompatible(adv, bgpAdv) {
				return fmt.Errorf("invalid local preference %d: local preferernce %d was "+
					"already set for the same type of BGP update. Check existing BGP advertisements "+
					"with common pools and aggregation lengths", adv.LocalPref, bgpAdv.LocalPref)
			}
		}
	}

	return nil
}

func advertisementsAreCompatible(newAdv, adv *BGPAdvertisement) bool {
	if adv.AggregationLength != newAdv.AggregationLength && adv.AggregationLengthV6 != newAdv.AggregationLengthV6 {
		return true
	}

	// BGP ADVs with different set of BGP peers do not collide.
	if len(newAdv.Peers) != 0 && len(adv.Peers) != 0 {
		equalPeer := false
		for _, peer := range newAdv.Peers {
			if slices.Contains(adv.Peers, peer) {
				equalPeer = true
				break
			}
		}
		if !equalPeer {
			return true
		}
	}

	// BGP ADVs with different set of nodes do not collide.
	for node := range newAdv.Nodes {
		if _, ok := adv.Nodes[node]; ok {
			return false
		}
	}

	return true
}

func ParseCIDR(cidr string) ([]*net.IPNet, error) {
	if !strings.Contains(cidr, "-") {
		_, n, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, fmt.Errorf("invalid CIDR %q", cidr)
		}
		return []*net.IPNet{n}, nil
	}

	fs := strings.SplitN(cidr, "-", 2)
	if len(fs) != 2 {
		return nil, fmt.Errorf("invalid IP range %q", cidr)
	}
	start := net.ParseIP(strings.TrimSpace(fs[0]))
	if start == nil {
		return nil, fmt.Errorf("invalid IP range %q: invalid start IP %q", cidr, fs[0])
	}
	end := net.ParseIP(strings.TrimSpace(fs[1]))
	if end == nil {
		return nil, fmt.Errorf("invalid IP range %q: invalid end IP %q", cidr, fs[1])
	}

	if bytes.Compare(start, end) > 0 {
		return nil, fmt.Errorf("invalid IP range %q: start IP %q is after the end IP %q", cidr, start, end)
	}

	var ret []*net.IPNet
	for _, pfx := range ipaddr.Summarize(start, end) {
		n := &net.IPNet{
			IP:   pfx.IP,
			Mask: pfx.Mask,
		}
		ret = append(ret, n)
	}
	return ret, nil
}

func cidrsOverlap(a, b *net.IPNet) bool {
	return cidrContainsCIDR(a, b) || cidrContainsCIDR(b, a)
}

func cidrContainsCIDR(outer, inner *net.IPNet) bool {
	ol, _ := outer.Mask.Size()
	il, _ := inner.Mask.Size()
	if ol == il && outer.IP.Equal(inner.IP) {
		return true
	}
	if ol < il && outer.Contains(inner.IP) {
		return true
	}
	return false
}

func lowestMask(cidrs []*net.IPNet) int {
	if len(cidrs) == 0 {
		return 0
	}
	lowest, _ := cidrs[0].Mask.Size()
	for _, c := range cidrs {
		s, _ := c.Mask.Size()
		if lowest > s {
			lowest = s
		}
	}
	return lowest
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

func containsAdvertisement(advs []*L2Advertisement, toCheck *L2Advertisement) bool {
	for _, adv := range advs {
		if adv.AllInterfaces != toCheck.AllInterfaces {
			continue
		}
		if !reflect.DeepEqual(adv.Nodes, toCheck.Nodes) {
			continue
		}
		if !sets.New(adv.Interfaces...).Equal(sets.New(toCheck.Interfaces...)) {
			continue
		}
		return true
	}
	return false
}

func selectedNodes(nodes []corev1.Node, selectors []metav1.LabelSelector) (map[string]bool, error) {
	labelSelectors := []labels.Selector{}
	for _, selector := range selectors {
		selector := selector // so we can use &selector
		l, err := metav1.LabelSelectorAsSelector(&selector)
		if err != nil {
			return nil, errors.Join(err, fmt.Errorf("invalid label selector %v", selector))
		}
		labelSelectors = append(labelSelectors, l)
	}

	res := make(map[string]bool)
OUTER:
	for _, node := range nodes {
		if len(labelSelectors) == 0 { // no selector mean all nodes are valid
			res[node.Name] = true
		}
		for _, s := range labelSelectors {
			nodeLabels := labels.Set(node.Labels)
			if s.Matches(nodeLabels) {
				res[node.Name] = true
				continue OUTER
			}
		}
	}
	return res, nil
}

func selectedPools(pools []metallbv1beta1.IPAddressPool, selectors []metav1.LabelSelector) ([]string, error) {
	labelSelectors := []labels.Selector{}
	for _, selector := range selectors {
		selector := selector // so we can use &selector
		l, err := metav1.LabelSelectorAsSelector(&selector)
		if err != nil {
			return nil, errors.Join(err, fmt.Errorf("invalid label selector %v", selector))
		}
		labelSelectors = append(labelSelectors, l)
	}
	var ipPools []string
OUTER:
	for _, pool := range pools {
		for _, s := range labelSelectors {
			poolLabels := labels.Set(pool.Labels)
			if s.Matches(poolLabels) {
				ipPools = append(ipPools, pool.Name)
				continue OUTER
			}
		}
	}
	return ipPools, nil
}

func validateLabelSelectorDuplicate(labelSelectors []metav1.LabelSelector, labelSelectorType string) error {
	for _, ls := range labelSelectors {
		for _, me := range ls.MatchExpressions {
			sort.Strings(me.Values)
		}
	}
	for i := 0; i < len(labelSelectors); i++ {
		err := validateLabelSelectorMatchExpressions(labelSelectors[i].MatchExpressions)
		if err != nil {
			return err
		}
		for j := i + 1; j < len(labelSelectors); j++ {
			if labelSelectors[i].String() == labelSelectors[j].String() {
				return fmt.Errorf("duplicate definition of %s %q", labelSelectorType, labelSelectors[i])
			}
		}
	}
	return nil
}

func validateLabelSelectorMatchExpressions(matchExpressions []metav1.LabelSelectorRequirement) error {
	for i := 0; i < len(matchExpressions); i++ {
		err := validateDuplicate(matchExpressions[i].Values, "match expression value in label selector")
		if err != nil {
			return err
		}
		for j := i + 1; j < len(matchExpressions); j++ {
			if matchExpressions[i].String() == matchExpressions[j].String() {
				return fmt.Errorf("duplicate definition of %s %q", "match expressions", matchExpressions[i])
			}
		}
	}
	return nil
}

func validateDuplicate(strSlice []string, sliceType string) error {
	for i := 0; i < len(strSlice); i++ {
		for j := i + 1; j < len(strSlice); j++ {
			if strSlice[i] == strSlice[j] {
				return fmt.Errorf("duplicate definition of %s %q", sliceType, strSlice[i])
			}
		}
	}
	return nil
}
