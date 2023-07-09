// SPDX-License-Identifier:Apache-2.0

package allocator // import "go.universe.tf/metallb/internal/allocator"

import (
	"errors"
	"fmt"
	"math"
	"net"
	"net/netip"
	"reflect"
	"sort"
	"strings"

	"go.universe.tf/metallb/internal/config"
	"go.universe.tf/metallb/internal/ipfamily"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/mikioh/ipaddr"
)

// An Allocator tracks IP address pools and allocates addresses from them.
type Allocator struct {
	pools *config.Pools

	allocated       map[string]*alloc              // svc -> alloc
	sharingKeyForIP map[netip.Addr]*key            // ip -> assigned sharing key
	portsInUse      map[netip.Addr]map[Port]string // ip -> Port -> svc
	servicesOnIP    map[netip.Addr]map[string]bool // ip -> svc -> allocated?
	poolIPsInUse    map[string]map[netip.Addr]int  // poolName -> ip -> number of users
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
	ipPools map[netip.Addr]string
	ports   []Port
	key
}

func (a *alloc) ips() []netip.Addr {
	var out []netip.Addr
	for ip := range a.ipPools {
		out = append(out, ip)
	}
	return out
}

func (a *alloc) poolsUnique() []string {
	keys := make(map[string]bool)
	var pools []string
	for _, pool := range a.ipPools {
		if _, value := keys[pool]; !value {
			keys[pool] = true
			pools = append(pools, pool)
		}
	}
	return pools
}

func uniqifyPools(pools map[netip.Addr]*config.Pool) []string {
	keys := make(map[string]bool)
	var out []string
	for _, pool := range pools {
		if _, value := keys[pool.Name]; !value {
			keys[pool.Name] = true
			out = append(out, pool.Name)
		}
	}
	return out
}

// New returns an Allocator managing no pools.
func New() *Allocator {
	return &Allocator{
		pools: &config.Pools{ByName: map[string]*config.Pool{}},

		allocated:       map[string]*alloc{},
		sharingKeyForIP: map[netip.Addr]*key{},
		portsInUse:      map[netip.Addr]map[Port]string{},
		servicesOnIP:    map[netip.Addr]map[string]bool{},
		poolIPsInUse:    map[string]map[netip.Addr]int{},
	}
}

// SetPools updates the set of address pools that the allocator owns.
func (a *Allocator) SetPools(pools *config.Pools) error {
	// All the fancy sharing stuff only influences how new allocations
	// can be created. For changing the underlying configuration, the
	// only question we have to answer is: can we fit all allocated
	// IPs into address pools under the new configuration?
	for svc, alloc := range a.allocated {
		pools := poolsFor(pools.ByName, alloc.ips())
		if len(pools) == 0 {
			return fmt.Errorf("new config not compatible with assigned IPs: service %q cannot own %q under new config", svc, alloc.ips())
		}
	}

	for n := range a.pools.ByName {
		if pools.ByName[n] == nil {
			stats.poolCapacity.DeleteLabelValues(n)
			stats.poolActive.DeleteLabelValues(n)
			stats.poolAllocated.DeleteLabelValues(n)
		}
	}

	a.pools = pools

	// Need to rearrange existing pool mappings and counts
	for svc, alloc := range a.allocated {
		pools := poolsFor(a.pools.ByName, alloc.ips())
		if len(pools) == 0 {
			return fmt.Errorf("can't retrieve new pool for assigned IPs: service %q cannot own %q under new config", svc, alloc.ips())
		}
		allocPools := alloc.poolsUnique()
		sort.Strings(allocPools)
		newPools := uniqifyPools(pools)
		sort.Strings(newPools)
		if !reflect.DeepEqual(allocPools, newPools) {
			alloc.ipPools = make(map[netip.Addr]string, 0)
			for ip, pool := range pools {
				alloc.ipPools[ip] = pool.Name
			}
			// Use the internal assign, we know for a fact the IP is
			// still usable.
			a.assign(svc, alloc)
		}
	}

	// Refresh or initiate stats
	for n, p := range a.pools.ByName {
		stats.poolCapacity.WithLabelValues(n).Set(float64(poolCount(p)))
		stats.poolActive.WithLabelValues(n).Set(float64(len(a.poolIPsInUse[n])))
	}

	return nil
}

// assign unconditionally updates internal state to reflect svc's
// allocation of alloc. Caller must ensure that this call is safe.
func (a *Allocator) assign(svc string, alloc *alloc) {
	a.Unassign(svc)
	a.allocated[svc] = alloc
	for ip, pool := range alloc.ipPools {
		a.sharingKeyForIP[ip] = &alloc.key
		if a.portsInUse[ip] == nil {
			a.portsInUse[ip] = map[Port]string{}
		}
		for _, port := range alloc.ports {
			a.portsInUse[ip][port] = svc
		}
		if a.servicesOnIP[ip] == nil {
			a.servicesOnIP[ip] = map[string]bool{}
		}
		a.servicesOnIP[ip][svc] = true
		if a.poolIPsInUse[pool] == nil {
			a.poolIPsInUse[pool] = map[netip.Addr]int{}
		}
		a.poolIPsInUse[pool][ip]++
	}
	for _, pool := range alloc.poolsUnique() {
		stats.poolCapacity.WithLabelValues(pool).Set(float64(poolCount(a.pools.ByName[pool])))
		stats.poolActive.WithLabelValues(pool).Set(float64(len(a.poolIPsInUse[pool])))
	}
}

func ConvertIpsToAddrs(ips []net.IP) []netip.Addr {
	var out []netip.Addr
	for _, ip := range ips {
		addr, _ := netip.AddrFromSlice(ip)
		out = append(out, addr)
	}
	return out
}

func ConvertAddrsToIps(ips []netip.Addr) []net.IP {
	var out []net.IP
	for _, ip := range ips {
		out = append(out, ip.AsSlice())
	}
	return out
}

// Assign assigns the requested ip to svc, if the assignment is
// permissible by sharingKey and backendKey.
func (a *Allocator) Assign(svcKey string, svc *v1.Service, ips []netip.Addr, ports []Port, sharingKey, backendKey string) error {
	pools := poolsFor(a.pools.ByName, ips)
	if len(pools) == 0 {
		return fmt.Errorf("%q is not allowed in config", ips)
	}
	sk := &key{
		sharing: sharingKey,
		backend: backendKey,
	}
	for _, pool := range pools {
		if !a.isPoolCompatibleWithService(pool, svc) {
			return fmt.Errorf("pool %s not compatible for ip assignment", pool.Name)
		}
	}

	for _, ip := range ips {
		// Does the IP already have allocs? If so, needs to be the same
		// sharing key, and have non-overlapping ports. If not, the
		// proposed IP needs to be allowed by configuration.
		if err := a.checkSharing(svcKey, ip, ports, sk); err != nil {
			return err
		}
	}

	ipPools := make(map[netip.Addr]string, 0)
	for ip, pool := range pools {
		ipPools[ip] = pool.Name
	}

	// Either the IP is entirely unused, or the requested use is
	// compatible with existing uses. Assign! But unassign first, in
	// case we're mutating an existing service (see the "already have
	// an allocation" block above). Unassigning is idempotent, so it's
	// unconditionally safe to do.
	alloc := &alloc{
		ipPools: ipPools,
		ports:   make([]Port, len(ports)),
		key:     *sk,
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
	for ip, pool := range al.ipPools {
		for _, port := range al.ports {
			if curSvc := a.portsInUse[ip][port]; curSvc != svc {
				panic(fmt.Sprintf("incoherent state, I thought port %q belonged to service %q, but it seems to belong to %q", port, svc, curSvc))
			}
			delete(a.portsInUse[ip], port)
		}

		delete(a.servicesOnIP[ip], svc)
		if len(a.portsInUse[ip]) == 0 {
			delete(a.portsInUse, ip)
			delete(a.sharingKeyForIP, ip)
		}
		a.poolIPsInUse[pool][ip]--
		if a.poolIPsInUse[pool][ip] == 0 {
			// Explicitly delete unused IPs from the pool, so that len()
			// is an accurate count of IPs in use.
			delete(a.poolIPsInUse[pool], ip)
		}
	}
	for _, pool := range al.poolsUnique() {
		stats.poolActive.WithLabelValues(pool).Set(float64(len(a.poolIPsInUse[pool])))
	}
}

// AllocateFromPool assigns an available IP from pool to service.
func (a *Allocator) AllocateFromPool(svcKey string, svc *v1.Service, serviceIPFamily ipfamily.Family, poolName string, ports []Port, sharingKey, backendKey string) ([]net.IP, error) {
	if alloc := a.allocated[svcKey]; alloc != nil {
		// Handle the case where the svc has already been assigned an IP but from the wrong family.
		// This "should-not-happen" since the "serviceIPFamily" is an immutable field in services.
		ips := alloc.ips()
		allocIPsFamily, err := ipfamily.ForAddrs(ips)
		if err != nil {
			return nil, err
		}
		if allocIPsFamily != serviceIPFamily {
			return nil, fmt.Errorf("IP for wrong family assigned alloc %s service family %s", allocIPsFamily, serviceIPFamily)
		}
		if err := a.Assign(svcKey, svc, ips, ports, sharingKey, backendKey); err != nil {
			return nil, err
		}
		return ConvertAddrsToIps(ips), nil
	}

	pool := a.pools.ByName[poolName]
	if pool == nil {
		return nil, fmt.Errorf("unknown pool %q", poolName)
	}

	ips := []net.IP{}
	ipfamilySel := make(map[ipfamily.Family]bool)

	switch serviceIPFamily {
	case ipfamily.DualStack:
		ipfamilySel[ipfamily.IPv4], ipfamilySel[ipfamily.IPv6] = true, true
	default:
		ipfamilySel[serviceIPFamily] = true
	}

	for _, cidr := range pool.CIDR {
		cidrIPFamily := ipfamily.ForCIDR(cidr)
		if _, ok := ipfamilySel[cidrIPFamily]; !ok {
			// Not the right ip-family
			continue
		}
		ip := a.getIPFromCIDR(cidr, pool.AvoidBuggyIPs, svcKey, ports, sharingKey, backendKey)
		if ip != nil {
			ips = append(ips, ip)
			delete(ipfamilySel, cidrIPFamily)
		}
	}

	if len(ipfamilySel) > 0 {
		// Woops, run out of IPs :( Fail.
		return nil, fmt.Errorf("no available IPs in pool %q for %s IPFamily", poolName, serviceIPFamily)
	}
	err := a.Assign(svcKey, svc, ConvertIpsToAddrs(ips), ports, sharingKey, backendKey)
	if err != nil {
		return nil, err
	}
	return ips, nil
}

// Allocate assigns any available and assignable IP to service.
func (a *Allocator) Allocate(svcKey string, svc *v1.Service, serviceIPFamily ipfamily.Family, ports []Port, sharingKey, backendKey string) ([]net.IP, error) {
	if alloc := a.allocated[svcKey]; alloc != nil {
		if err := a.Assign(svcKey, svc, alloc.ips(), ports, sharingKey, backendKey); err != nil {
			return nil, err
		}
		return ConvertAddrsToIps(alloc.ips()), nil
	}
	pinnedPools := a.pinnedPoolsForService(svc)
	for _, pool := range pinnedPools {
		if ips, err := a.AllocateFromPool(svcKey, svc, serviceIPFamily, pool.Name, ports, sharingKey, backendKey); err == nil {
			return ips, nil
		}
	}
	for _, pool := range a.pools.ByName {
		if !pool.AutoAssign || pool.ServiceAllocations != nil {
			continue
		}
		if ips, err := a.AllocateFromPool(svcKey, svc, serviceIPFamily, pool.Name, ports, sharingKey, backendKey); err == nil {
			return ips, nil
		}
	}

	return nil, errors.New("no available IPs")
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

// Pools returns the pools from which service's IP was allocated. If
// service has no IP allocated, nil is returned.
func (a *Allocator) Pools(svc string) *map[netip.Addr]string {
	if alloc := a.allocated[svc]; alloc != nil {
		return &alloc.ipPools
	}
	return nil
}

// PoolsUnique returns the unique set of pools a given service uses, or an empty slice
func (a *Allocator) PoolsUnique(svc string) []string {
	if alloc := a.allocated[svc]; alloc != nil {
		return alloc.poolsUnique()
	}
	return make([]string, 0)
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
func poolCount(p *config.Pool) int64 {
	var total int64
	for _, cidr := range p.CIDR {
		o, b := cidr.Mask.Size()
		if b-o >= 62 {
			// An enormous ipv6 range is allocated which will never run out.
			// Just return max to avoid any math errors.
			return math.MaxInt64
		}
		sz := int64(math.Pow(2, float64(b-o)))

		cur := ipaddr.NewCursor([]ipaddr.Prefix{*ipaddr.NewPrefix(cidr)})
		firstIP, _ := netip.AddrFromSlice(cur.First().IP)
		lastIP, _ := netip.AddrFromSlice(cur.Last().IP)

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
	}
	return total
}

// poolFor returns the pool that owns the requested IPs, or "" if none.
func poolsFor(pools map[string]*config.Pool, ips []netip.Addr) map[netip.Addr]*config.Pool {
	out := make(map[netip.Addr]*config.Pool, 0)
	for _, ip := range ips {
		p := func() *config.Pool {
			for _, p := range pools {
				if p.AvoidBuggyIPs && ipConfusesBuggyFirmwares(ip) {
					continue
				}
				for _, cidr := range p.CIDR {
					if cidr.Contains(ip.AsSlice()) {
						return p
					}
				}
			}
			return nil
		}()
		if p != nil {
			out[ip] = p
		} else {
			return nil
		}
	}
	return out
}

// ipConfusesBuggyFirmwares returns true if ip is an IPv4 address ending in 0 or 255.
//
// Such addresses can confuse smurf protection on crappy CPE
// firmwares, leading to packet drops.
func ipConfusesBuggyFirmwares(ip netip.Addr) bool {
	if ip.Is6() && !ip.Is4In6() {
		return false
	}
	raw := ip.As4()
	return raw[3] == 0 || raw[3] == 255
}

func (a *Allocator) getIPFromCIDR(cidr *net.IPNet, avoidBuggyIPs bool, svc string, ports []Port, sharingKey, backendKey string) net.IP {
	sk := &key{
		sharing: sharingKey,
		backend: backendKey,
	}
	c := ipaddr.NewCursor([]ipaddr.Prefix{*ipaddr.NewPrefix(cidr)})
	for pos := c.First(); pos != nil; pos = c.Next() {
		addr, _ := netip.AddrFromSlice(pos.IP)
		if avoidBuggyIPs && ipConfusesBuggyFirmwares(addr) {
			continue
		}
		if a.checkSharing(svc, addr, ports, sk) != nil {
			continue
		}
		return pos.IP
	}
	return nil
}

func (a *Allocator) checkSharing(svc string, ip netip.Addr, ports []Port, sk *key) error {
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
