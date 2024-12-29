// SPDX-License-Identifier:Apache-2.0

package allocator // import "go.universe.tf/metallb/internal/allocator"

import (
	"errors"
	"fmt"
	"math"
	"net"
	"sort"
	"strings"
	"sync"

	"go.universe.tf/metallb/internal/config"
	"go.universe.tf/metallb/internal/ipfamily"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/mikioh/ipaddr"
)

// An Allocator tracks IP address pools and allocates addresses from them.
type Allocator struct {
	pools *config.Pools

	allocated       map[string]*alloc          // svc -> alloc
	sharingKeyForIP map[string]*key            // ip.String() -> assigned sharing key
	portsInUse      map[string]map[Port]string // ip.String() -> Port -> svc
	servicesOnIP    map[string]map[string]bool // ip.String() -> svc -> allocated?
	poolIPsInUse    map[string]map[string]int  // poolName -> ip.String() -> number of users
	poolIPV4InUse   map[string]map[string]int  // poolName -> ipv4.String() -> number of users
	poolIPV6InUse   map[string]map[string]int  // poolName -> ipv6.String() -> number of users

	poolToCounters          map[string]PoolCounters // poolName -> Counters
	countersMutex           sync.RWMutex
	countersChangedCallback func(string)
}

// Port represents one port in use by a service.
type Port struct {
	Proto string
	Port  int
}

// String returns a text description of the port.
func (p Port) String() string {
	return fmt.Sprintf("%s/%d", p.Proto, p.Port)
}

type key struct {
	sharing string
	backend string
}

type alloc struct {
	pool  string
	ips   []net.IP
	ports []Port
	key
}

type PoolCounters struct {
	AssignedIPv4  int64
	AssignedIPv6  int64
	AvailableIPv4 int64
	AvailableIPv6 int64
}

// New returns an Allocator managing no pools.
func New(countersCallback func(string)) *Allocator {
	return &Allocator{
		pools: &config.Pools{ByName: map[string]*config.Pool{}},

		allocated:               map[string]*alloc{},
		sharingKeyForIP:         map[string]*key{},
		portsInUse:              map[string]map[Port]string{},
		servicesOnIP:            map[string]map[string]bool{},
		poolIPsInUse:            map[string]map[string]int{},
		poolIPV4InUse:           map[string]map[string]int{},
		poolIPV6InUse:           map[string]map[string]int{},
		poolToCounters:          map[string]PoolCounters{},
		countersMutex:           sync.RWMutex{},
		countersChangedCallback: countersCallback,
	}
}

// SetPools updates the set of address pools that the allocator owns.
func (a *Allocator) SetPools(pools *config.Pools) {
	refreshPools := []string{}
	defer func() {
		for _, p := range refreshPools {
			a.countersChangedCallback(p)
		}
	}()

	a.countersMutex.Lock()
	for n := range a.pools.ByName {
		if pools.ByName[n] == nil {
			deleteStatsFor(n)
			delete(a.poolToCounters, n)
			refreshPools = append(refreshPools, n)
		}
	}
	a.countersMutex.Unlock()

	a.pools = pools

	// Need to rearrange existing pool mappings and counts
	for svc, alloc := range a.allocated {
		pool := poolFor(a.pools.ByName, alloc.ips)
		if pool == nil {
			a.Unassign(svc)
			continue
		}
		if pool.Name != alloc.pool {
			a.Unassign(svc)
			alloc.pool = pool.Name
			// Use the internal assign, we know for a fact the IP is
			// still usable.
			a.assign(svc, alloc)
		}
	}

	// Refresh or initiate stats
	for _, p := range a.pools.ByName {
		a.updatePoolStats(p)
		refreshPools = append(refreshPools, p.Name)
	}
}

// assign unconditionally updates internal state to reflect svc's
// allocation of alloc. Caller must ensure that this call is safe.
func (a *Allocator) assign(svc string, alloc *alloc) {
	a.Unassign(svc)
	a.allocated[svc] = alloc
	for _, ip := range alloc.ips {
		a.sharingKeyForIP[ip.String()] = &alloc.key
		if a.portsInUse[ip.String()] == nil {
			a.portsInUse[ip.String()] = map[Port]string{}
		}
		for _, port := range alloc.ports {
			a.portsInUse[ip.String()][port] = svc
		}
		if a.servicesOnIP[ip.String()] == nil {
			a.servicesOnIP[ip.String()] = map[string]bool{}
		}
		a.servicesOnIP[ip.String()][svc] = true
		if a.poolIPsInUse[alloc.pool] == nil {
			a.poolIPsInUse[alloc.pool] = map[string]int{}
		}
		if a.poolIPV4InUse[alloc.pool] == nil {
			a.poolIPV4InUse[alloc.pool] = map[string]int{}
		}
		if a.poolIPV6InUse[alloc.pool] == nil {
			a.poolIPV6InUse[alloc.pool] = map[string]int{}
		}

		a.poolIPsInUse[alloc.pool][ip.String()]++
		if ip.To4() == nil {
			a.poolIPV6InUse[alloc.pool][ip.String()]++
		} else {
			a.poolIPV4InUse[alloc.pool][ip.String()]++
		}
	}
	a.updatePoolStats(a.pools.ByName[alloc.pool])
	a.countersChangedCallback(alloc.pool)
}

// Assign assigns the requested ip to svc, if the assignment is
// permissible by sharingKey and backendKey.
func (a *Allocator) Assign(svcKey string, svc *v1.Service, ips []net.IP, ports []Port, sharingKey, backendKey string) error {
	pool := poolFor(a.pools.ByName, ips)
	if pool == nil {
		return fmt.Errorf("%q is not allowed in config", ips)
	}
	sk := &key{
		sharing: sharingKey,
		backend: backendKey,
	}
	if !a.isPoolCompatibleWithService(pool, svc) {
		return fmt.Errorf("pool %s not compatible for ip assignment", pool.Name)
	}
	// Check the dual-stack constraints:
	// - Two addresses
	// - Different families, ipv4 and ipv6
	if len(ips) > 2 {
		return fmt.Errorf("more than two addresses %q", ips)
	}
	if len(ips) == 2 && (ipfamily.ForAddress(ips[0]) == ipfamily.ForAddress(ips[1])) {
		return fmt.Errorf("%q %q has the same family", ips[0], ips[1])
	}

	for _, ip := range ips {
		// Does the IP already have allocs? If so, needs to be the same
		// sharing key, and have non-overlapping ports. If not, the
		// proposed IP needs to be allowed by configuration.
		if err := a.checkSharing(svcKey, ip.String(), ports, sk); err != nil {
			return err
		}
	}

	// Either the IP is entirely unused, or the requested use is
	// compatible with existing uses. Assign! But unassign first, in
	// case we're mutating an existing service (see the "already have
	// an allocation" block above). Unassigning is idempotent, so it's
	// unconditionally safe to do.
	alloc := &alloc{
		pool:  pool.Name,
		ips:   ips,
		ports: make([]Port, len(ports)),
		key:   *sk,
	}
	copy(alloc.ports, ports)
	a.assign(svcKey, alloc)
	return nil
}

// Unassign frees the IP associated with service, if any.
func (a *Allocator) Unassign(svc string) {
	if a.allocated[svc] == nil {
		return
	}

	al := a.allocated[svc]
	delete(a.allocated, svc)
	for _, ip := range al.ips {
		for _, port := range al.ports {
			if curSvc := a.portsInUse[ip.String()][port]; curSvc != svc {
				panic(fmt.Sprintf("incoherent state, I thought port %q belonged to service %q, but it seems to belong to %q", port, svc, curSvc))
			}
			delete(a.portsInUse[ip.String()], port)
		}

		delete(a.servicesOnIP[ip.String()], svc)
		if len(a.portsInUse[ip.String()]) == 0 {
			delete(a.portsInUse, ip.String())
			delete(a.sharingKeyForIP, ip.String())
		}
		a.poolIPsInUse[al.pool][ip.String()]--
		if ip.To4() == nil {
			a.poolIPV6InUse[al.pool][ip.String()]--
		} else {
			a.poolIPV4InUse[al.pool][ip.String()]--
		}
		// Explicitly delete unused IPs from the pool, so that len()
		// is an accurate count of IPs in use.
		if a.poolIPsInUse[al.pool][ip.String()] == 0 {
			delete(a.poolIPsInUse[al.pool], ip.String())
		}
		if a.poolIPV4InUse[al.pool][ip.String()] == 0 {
			delete(a.poolIPV4InUse[al.pool], ip.String())
		}
		if a.poolIPV6InUse[al.pool][ip.String()] == 0 {
			delete(a.poolIPV6InUse[al.pool], ip.String())
		}
	}

	if _, ok := a.pools.ByName[al.pool]; !ok {
		deleteStatsFor(al.pool)
		return
	}

	a.updatePoolStats(a.pools.ByName[al.pool])
	a.countersChangedCallback(al.pool)
}

// getFreeIPsFromPool determines, with best effort, an ipv4 and an ipv6 available from the provided pool.
func (a *Allocator) getFreeIPsFromPool(
	pool *config.Pool,
	svcKey string,
	ports []Port,
	sharingKey,
	backendKey string,
) *Allocation {
	allocation := &Allocation{
		PoolName: pool.Name,
		IPV4:     nil,
		IPV6:     nil,
	}
	for _, cidr := range pool.CIDR {
		cidrIPFamily := ipfamily.ForCIDR(cidr)
		if ip := allocation.getIPForFamily(cidrIPFamily); ip != nil {
			continue
		}
		if ip := a.getIPFromCIDR(cidr, pool.AvoidBuggyIPs, svcKey, ports, sharingKey, backendKey); ip != nil {
			allocation.setIPForFamily(cidrIPFamily, ip)
		}
	}
	return allocation
}

// findBestPoolForService returns the ipPool corresponding with the most suitable pool for a serviceIPFamily.
func (a *Allocator) findBestPoolForService(
	pools []*config.Pool,
	svcKey string,
	svc *v1.Service,
	serviceIPFamily ipfamily.Family,
	ports []Port,
	sharingKey, backendKey string,
) (*Allocation, error) {
	var primaryAllocationCandidate, secondaryAllocationCandidate *Allocation
	// By default, ipv4 has higher priority.
	primaryIPFamily := ipfamily.IPv4
	secondaryIPFamily := ipfamily.IPv6
	serviceIPFamilyPolicy := ipPolicyForService(svc)

	// If the ipv6 is explicitly required to be prefferable.
	if len(svc.Spec.IPFamilies) > 0 && svc.Spec.IPFamilies[0] == v1.IPv6Protocol {
		primaryIPFamily = ipfamily.IPv6
		secondaryIPFamily = ipfamily.IPv4
	}
	for _, pool := range pools {
		allocation := a.getFreeIPsFromPool(pool, svcKey, ports, sharingKey, backendKey)
		// This can happen only in case serviceIPFamily is ipv4 or ipv6.
		if ip := allocation.getIPForFamily(serviceIPFamily); ip != nil {
			return allocation, nil
		}

		primaryIP := allocation.getIPForFamily(primaryIPFamily)
		secondaryIP := allocation.getIPForFamily(secondaryIPFamily)

		if primaryIP != nil && secondaryIP != nil {
			return allocation, nil
		}

		// at this stage, we should not take this pool into account if
		// not in PreferDualStack policy.
		if !isPreferDualStack(serviceIPFamilyPolicy, serviceIPFamily) {
			continue
		}

		if primaryIP != nil && primaryAllocationCandidate == nil {
			primaryAllocationCandidate = allocation
		}
		if secondaryIP != nil && secondaryAllocationCandidate == nil {
			secondaryAllocationCandidate = allocation
		}
	}
	if primaryAllocationCandidate != nil {
		return primaryAllocationCandidate, nil
	}
	if secondaryAllocationCandidate != nil {
		return secondaryAllocationCandidate, nil
	}
	return nil, fmt.Errorf("no suitable pool for %s IPFamily", serviceIPFamily)
}

// isPreferDualStack determines if the provided combination of serviceIPFamily and its policy
// shows that the service satisfies PreferDualStack policy.
func isPreferDualStack(serviceIPFamilyPolicy v1.IPFamilyPolicy, serviceIPFamily ipfamily.Family) bool {
	return (serviceIPFamilyPolicy == v1.IPFamilyPolicyPreferDualStack && serviceIPFamily == ipfamily.DualStack)
}

// Allocate chooses the most suitable pool and assigns an available IP from that pool
// to the service.
func (a *Allocator) Allocate(
	svcKey string,
	svc *v1.Service,
	serviceIPFamily ipfamily.Family,
	ports []Port,
	sharingKey, backendKey string,
) ([]net.IP, error) {
	if alloc := a.allocated[svcKey]; alloc != nil {
		if err := a.Assign(svcKey, svc, alloc.ips, ports, sharingKey, backendKey); err != nil {
			return nil, err
		}
		return alloc.ips, nil
	}
	// First, check the pinned pools to see if we can assign.
	pinnedPools := a.pinnedPoolsForService(svc)
	ips, err := a.allocateFromPools(pinnedPools, svcKey, svc, serviceIPFamily, ports, sharingKey, backendKey)
	if err == nil {
		return ips, nil
	}

	// No suitable IPs in pinnedPools, use all pools instead.
	allPools := []*config.Pool{}
	for _, pool := range a.pools.ByName {
		if !pool.AutoAssign || pool.ServiceAllocations != nil {
			continue
		}
		allPools = append(allPools, pool)
	}
	ips, err = a.allocateFromPools(allPools, svcKey, svc, serviceIPFamily, ports, sharingKey, backendKey)
	if err == nil {
		return ips, nil
	}

	// We will reach here only if there is really no suitable IP.
	return nil, errors.New("no available IPs")
}

// allocateFromPools picks the most suitable pool and tries to allocate its ips.
func (a *Allocator) allocateFromPools(
	pools []*config.Pool,
	svcKey string,
	svc *v1.Service,
	serviceIPFamily ipfamily.Family,
	ports []Port,
	sharingKey, backendKey string,
) ([]net.IP, error) {
	poolIps, err := a.findBestPoolForService(pools, svcKey, svc, serviceIPFamily, ports, sharingKey, backendKey)
	if err != nil {
		return nil, err
	}
	serviceIPFamilyPolicy := ipPolicyForService(svc)
	if ips, err := poolIps.selectIPsForFamilyAndPolicy(serviceIPFamily, serviceIPFamilyPolicy); err == nil {
		if assignErr := a.Assign(svcKey, svc, ips, ports, sharingKey, backendKey); assignErr == nil {
			return ips, nil
		}
	}
	return nil, errors.New("no available IPs")
}

// AllocateFromPool assigns an available IP from pool to service.
func (a *Allocator) AllocateFromPool(
	svcKey string,
	svc *v1.Service,
	serviceIPFamily ipfamily.Family,
	poolName string,
	ports []Port,
	sharingKey,
	backendKey string,
) ([]net.IP, error) {
	if alloc := a.allocated[svcKey]; alloc != nil {
		// Handle the case where the svc has already been assigned an IP but from the wrong family.
		// This "should-not-happen" since the "serviceIPFamily" is an immutable field in services.
		allocIPsFamily, err := ipfamily.ForAddressesIPs(alloc.ips)
		if err != nil {
			return nil, err
		}
		serviceIPFamilyPolicy := ipPolicyForService(svc)
		if allocIPsFamily == ipfamily.Unknown {
			return nil, fmt.Errorf("unknown allocated IP Family %s", allocIPsFamily)
		}
		if serviceIPFamilyPolicy != v1.IPFamilyPolicyPreferDualStack && allocIPsFamily != serviceIPFamily {
			return nil, fmt.Errorf("IP for wrong family assigned alloc %s service family %s", allocIPsFamily, serviceIPFamily)
		}
		if err := a.Assign(svcKey, svc, alloc.ips, ports, sharingKey, backendKey); err != nil {
			return nil, err
		}
		return alloc.ips, nil
	}

	serviceIPFamilyPolicy := ipPolicyForService(svc)
	pool := a.pools.ByName[poolName]
	if pool == nil {
		return nil, fmt.Errorf("unknown pool %q", poolName)
	}

	poolIps := a.getFreeIPsFromPool(pool, svcKey, ports, sharingKey, backendKey)
	ips, err := poolIps.selectIPsForFamilyAndPolicy(serviceIPFamily, serviceIPFamilyPolicy)
	if err != nil {
		return nil, err
	}

	err = a.Assign(svcKey, svc, ips, ports, sharingKey, backendKey)
	if err != nil {
		return nil, err
	}

	return ips, nil
}

// AllocateIPFromPoolForAdditionalFamily works specially for the preferDualStack
// ipfamily policy in case there is only 1 assigned ip. It tries to allocate an
// additional ip from the missing family while retaining the ip already allocated to the svc.
func (a *Allocator) AllocateFromPoolForAdditionalFamily(
	svcKey string,
	svc *v1.Service,
	existingIP net.IP,
	poolName string,
	ports []Port,
	sharingKey,
	backendKey string,
) (net.IP, error) {
	additionalFamily := ipfamily.IPv4
	existingFamily := ipfamily.ForAddress(existingIP)
	if existingFamily == ipfamily.IPv4 {
		additionalFamily = ipfamily.IPv6
	}
	pool := a.pools.ByName[poolName]
	if pool == nil {
		return nil, fmt.Errorf("unknown pool %q", poolName)
	}

	poolIps := a.getFreeIPsFromPool(pool, svcKey, ports, sharingKey, backendKey)
	additionalIPs, err := poolIps.selectIPsForFamilyAndPolicy(additionalFamily, v1.IPFamilyPolicySingleStack)
	if err != nil {
		return nil, err
	}
	newIps := []net.IP{existingIP, additionalIPs[0]}
	err = a.Assign(svcKey, svc, newIps, ports, sharingKey, backendKey)
	if err != nil {
		return nil, err
	}

	return additionalIPs[0], nil
}

// This method returns sorted ip pools which are allocatable for given service.
// When ip pool is not set with priority, then just append it after sorted
// priority ip pools.
func (a *Allocator) pinnedPoolsForService(svc *v1.Service) []*config.Pool {
	var pools []*config.Pool
	if svc == nil {
		return pools
	}
	for _, nsPoolName := range a.pools.ByNamespace[svc.Namespace] {
		if nsPool, ok := a.pools.ByName[nsPoolName]; ok {
			if !nsPool.AutoAssign || !a.isPoolCompatibleWithService(nsPool, svc) {
				continue
			}
			pools = append(pools, nsPool)
		}
	}
	for _, svcPoolName := range a.pools.ByServiceSelector {
		if svcPool, ok := a.pools.ByName[svcPoolName]; ok {
			if !svcPool.AutoAssign || !a.isPoolCompatibleWithService(svcPool, svc) {
				continue
			}
			pools = append(pools, svcPool)
		}
	}
	sortPools(pools)
	return pools
}

func (a *Allocator) isPoolCompatibleWithService(p *config.Pool, svc *v1.Service) bool {
	if p.ServiceAllocations != nil && p.ServiceAllocations.Namespaces.Len() > 0 &&
		!p.ServiceAllocations.Namespaces.Has(svc.Namespace) {
		return false
	}
	if p.ServiceAllocations != nil && len(p.ServiceAllocations.ServiceSelectors) > 0 {
		svcLabels := labels.Set(svc.Labels)
		for _, svcSelector := range p.ServiceAllocations.ServiceSelectors {
			if svcSelector.Matches(svcLabels) {
				return true
			}
		}
		return false
	}
	return true
}

// Pool returns the pool from which service's IP was allocated. If
// service has no IP allocated, "" is returned.
func (a *Allocator) Pool(svc string) string {
	if alloc := a.allocated[svc]; alloc != nil {
		return alloc.pool
	}
	return ""
}

// IPs returns the allocated IPs of a service.
func (a *Allocator) IPs(svc string) []net.IP {
	if alloc := a.allocated[svc]; alloc != nil {
		return alloc.ips
	}
	return nil
}

// PoolForIP returns the pool structure associated with an IP.
func (a *Allocator) PoolForIP(ips []net.IP) *config.Pool {
	return poolFor(a.pools.ByName, ips)
}

func (a *Allocator) CountersForPool(name string) PoolCounters {
	a.countersMutex.RLock()
	defer a.countersMutex.RUnlock()
	return a.poolToCounters[name]
}

func sortPools(pools []*config.Pool) {
	// A lower value for pool priority equals a higher priority and sort
	// pools from higher to low priority. when no priority (0) set on
	// the pool, then that is considered as lowest priority.
	sort.Slice(pools, func(i, j int) bool {
		if pools[i].ServiceAllocations.Priority > 0 &&
			pools[j].ServiceAllocations.Priority > 0 {
			return pools[i].ServiceAllocations.Priority <
				pools[j].ServiceAllocations.Priority
		}
		if pools[i].ServiceAllocations.Priority == 0 &&
			pools[j].ServiceAllocations.Priority > 0 {
			return false
		}
		return true
	})
}

func sharingOK(existing, new *key) error {
	if existing.sharing == "" {
		return errors.New("existing service does not allow sharing")
	}
	if new.sharing == "" {
		return errors.New("new service does not allow sharing")
	}
	if existing.sharing != new.sharing {
		return fmt.Errorf("sharing key %q does not match existing sharing key %q", new.sharing, existing.sharing)
	}
	if existing.backend != new.backend {
		return fmt.Errorf("backend key %q does not match existing sharing key %q", new.backend, existing.backend)
	}
	return nil
}

// poolCount returns the number of addresses in the pool.
func poolCount(p *config.Pool) (int64, int64, int64) {
	var total int64
	var ipv4 int64
	var ipv6 int64
	for _, cidr := range p.CIDR {
		o, b := cidr.Mask.Size()
		if b-o >= 62 {
			// An enormous ipv6 range is allocated which will never run out.
			total = math.MaxInt64
			ipv6 = math.MaxInt64
			continue
		}
		sz := int64(math.Pow(2, float64(b-o)))

		cur := ipaddr.NewCursor([]ipaddr.Prefix{*ipaddr.NewPrefix(cidr)})
		firstIP := cur.First().IP
		lastIP := cur.Last().IP

		if p.AvoidBuggyIPs {
			if o <= 24 {
				// A pair of buggy IPs occur for each /24 present in the range.
				buggies := int64(math.Pow(2, float64(24-o))) * 2
				sz -= buggies
			} else {
				// Ranges smaller than /24 contain 1 buggy IP if they
				// start/end on a /24 boundary, otherwise they contain
				// none.
				if ipConfusesBuggyFirmwares(firstIP) {
					sz--
				}
				if ipConfusesBuggyFirmwares(lastIP) {
					sz--
				}
			}
		}
		total += sz
		if cidr.IP.To4() == nil {
			ipv6 += sz
		} else {
			ipv4 += sz
		}
	}
	return total, ipv4, ipv6
}

// poolFor returns the pool that owns the requested IPs, or "" if none.
func poolFor(pools map[string]*config.Pool, ips []net.IP) *config.Pool {
	for _, p := range pools {
		cnt := 0
		for _, ip := range ips {
			if p.AvoidBuggyIPs && ipConfusesBuggyFirmwares(ip) {
				continue
			}
			for _, cidr := range p.CIDR {
				if cidr.Contains(ip) {
					cnt++
					break
				}
			}
		}
		if cnt == len(ips) {
			return p
		}
	}
	return nil
}

// ipConfusesBuggyFirmwares returns true if ip is an IPv4 address ending in 0 or 255.
//
// Such addresses can confuse smurf protection on crappy CPE
// firmwares, leading to packet drops.
func ipConfusesBuggyFirmwares(ip net.IP) bool {
	ip = ip.To4()
	if ip == nil {
		return false
	}
	return ip[3] == 0 || ip[3] == 255
}

func (a *Allocator) getIPFromCIDR(cidr *net.IPNet, avoidBuggyIPs bool, svc string, ports []Port, sharingKey, backendKey string) net.IP {
	sk := &key{
		sharing: sharingKey,
		backend: backendKey,
	}
	c := ipaddr.NewCursor([]ipaddr.Prefix{*ipaddr.NewPrefix(cidr)})
	for pos := c.First(); pos != nil; pos = c.Next() {
		if avoidBuggyIPs && ipConfusesBuggyFirmwares(pos.IP) {
			continue
		}
		if a.checkSharing(svc, pos.IP.String(), ports, sk) != nil {
			continue
		}
		return pos.IP
	}
	return nil
}

func (a *Allocator) checkSharing(svc string, ip string, ports []Port, sk *key) error {
	if existingSK := a.sharingKeyForIP[ip]; existingSK != nil {
		if err := sharingOK(existingSK, sk); err != nil {
			// Sharing key is incompatible. However, if the owner is
			// the same service, and is the only user of the IP, we
			// can just update its sharing key in place.
			var otherSvcs []string
			for otherSvc := range a.servicesOnIP[ip] {
				if otherSvc != svc {
					otherSvcs = append(otherSvcs, otherSvc)
				}
			}
			if len(otherSvcs) > 0 {
				return fmt.Errorf("can't change sharing key for %q, address also in use by %s", svc, strings.Join(otherSvcs, ","))
			}
		}

		for _, port := range ports {
			if curSvc, ok := a.portsInUse[ip][port]; ok && curSvc != svc {
				return fmt.Errorf("port %s is already in use on %q", port, ip)
			}
		}
	}
	return nil
}

// ipPolicyForService determines the IPFamilyPolicy of a given svc.
func ipPolicyForService(svc *v1.Service) v1.IPFamilyPolicy {
	serviceIPFamilyPolicy := v1.IPFamilyPolicySingleStack
	if svc.Spec.IPFamilyPolicy != nil {
		serviceIPFamilyPolicy = *(svc.Spec.IPFamilyPolicy)
	}
	return serviceIPFamilyPolicy
}

func (a *Allocator) updatePoolStats(p *config.Pool) {
	a.countersMutex.Lock()
	defer a.countersMutex.Unlock()
	total, ipv4, ipv6 := poolCount(p)
	stats.poolCapacity.WithLabelValues(p.Name).Set(float64(total))
	stats.ipv4PoolCapacity.WithLabelValues(p.Name).Set(float64(ipv4))
	stats.ipv6PoolCapacity.WithLabelValues(p.Name).Set(float64(ipv6))
	stats.poolActive.WithLabelValues(p.Name).Set(float64(len(a.poolIPsInUse[p.Name])))
	stats.ipv4PoolActive.WithLabelValues(p.Name).Set(float64(len(a.poolIPV4InUse[p.Name])))
	stats.ipv6PoolActive.WithLabelValues(p.Name).Set(float64(len(a.poolIPV6InUse[p.Name])))
	a.poolToCounters[p.Name] = PoolCounters{
		AvailableIPv4: ipv4 - int64(len(a.poolIPV4InUse[p.Name])),
		AvailableIPv6: ipv6 - int64(len(a.poolIPV6InUse[p.Name])),
		AssignedIPv4:  int64(len(a.poolIPV4InUse[p.Name])),
		AssignedIPv6:  int64(len(a.poolIPV6InUse[p.Name])),
	}
}
