// SPDX-License-Identifier:Apache-2.0

package allocator // import "go.universe.tf/metallb/internal/allocator"

import (
	"errors"
	"fmt"
	"math"
	"net"
	"strings"

	"go.universe.tf/metallb/internal/config"
	"go.universe.tf/metallb/internal/ipfamily"

	"github.com/mikioh/ipaddr"
)

// An Allocator tracks IP address pools and allocates addresses from them.
type Allocator struct {
	pools map[string]*config.Pool

	allocated       map[string]*alloc          // svc -> alloc
	sharingKeyForIP map[string]*key            // ip.String() -> assigned sharing key
	portsInUse      map[string]map[Port]string // ip.String() -> Port -> svc
	servicesOnIP    map[string]map[string]bool // ip.String() -> svc -> allocated?
	poolIPsInUse    map[string]map[string]int  // poolName -> ip.String() -> number of users
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

// New returns an Allocator managing no pools.
func New() *Allocator {
	return &Allocator{
		pools: map[string]*config.Pool{},

		allocated:       map[string]*alloc{},
		sharingKeyForIP: map[string]*key{},
		portsInUse:      map[string]map[Port]string{},
		servicesOnIP:    map[string]map[string]bool{},
		poolIPsInUse:    map[string]map[string]int{},
	}
}

var ErrCannotShareKey = errors.New("services can't share key")

// SetPools updates the set of address pools that the allocator owns.
func (a *Allocator) SetPools(pools map[string]*config.Pool) error {
	// All the fancy sharing stuff only influences how new allocations
	// can be created. For changing the underlying configuration, the
	// only question we have to answer is: can we fit all allocated
	// IPs into address pools under the new configuration?
	for svc, alloc := range a.allocated {
		if poolFor(pools, alloc.ips) == "" {
			return fmt.Errorf("new config not compatible with assigned IPs: service %q cannot own %q under new config", svc, alloc.ips)
		}
	}

	for n := range a.pools {
		if pools[n] == nil {
			stats.poolCapacity.DeleteLabelValues(n)
			stats.poolActive.DeleteLabelValues(n)
			stats.poolAllocated.DeleteLabelValues(n)
		}
	}

	a.pools = pools

	// Need to rearrange existing pool mappings and counts
	for svc, alloc := range a.allocated {
		pool := poolFor(a.pools, alloc.ips)
		if pool != alloc.pool {
			a.Unassign(svc)
			alloc.pool = pool
			// Use the internal assign, we know for a fact the IP is
			// still usable.
			a.assign(svc, alloc)
		}
	}

	// Refresh or initiate stats
	for n, p := range a.pools {
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
		a.poolIPsInUse[alloc.pool][ip.String()]++
	}
	stats.poolCapacity.WithLabelValues(alloc.pool).Set(float64(poolCount(a.pools[alloc.pool])))
	stats.poolActive.WithLabelValues(alloc.pool).Set(float64(len(a.poolIPsInUse[alloc.pool])))
}

// Assign assigns the requested ip to svc, if the assignment is
// permissible by sharingKey and backendKey.
func (a *Allocator) Assign(svc string, ips []net.IP, ports []Port, sharingKey, backendKey string) error {
	pool := poolFor(a.pools, ips)
	if pool == "" {
		return fmt.Errorf("%q is not allowed in config", ips)
	}
	sk := &key{
		sharing: sharingKey,
		backend: backendKey,
	}
	// Check the dual-stack constraints:
	// - Two addresses
	// - Different families, ipv4 and ipv6
	if len(ips) > 2 {
		return fmt.Errorf("More than two addresses %q", ips)
	}
	if len(ips) == 2 && (ipfamily.ForAddress(ips[0]) == ipfamily.ForAddress(ips[1])) {
		return fmt.Errorf("%q %q has the same family", ips[0], ips[1])
	}

	for _, ip := range ips {
		// Does the IP already have allocs? If so, needs to be the same
		// sharing key, and have non-overlapping ports. If not, the
		// proposed IP needs to be allowed by configuration.
		if err := a.checkSharing(svc, ip.String(), ports, sk); err != nil {
			return err
		}
	}

	// Either the IP is entirely unused, or the requested use is
	// compatible with existing uses. Assign! But unassign first, in
	// case we're mutating an existing service (see the "already have
	// an allocation" block above). Unassigning is idempotent, so it's
	// unconditionally safe to do.
	alloc := &alloc{
		pool:  pool,
		ips:   ips,
		ports: make([]Port, len(ports)),
		key:   *sk,
	}
	for i, port := range ports {
		alloc.ports[i] = port
	}
	a.assign(svc, alloc)
	return nil
}

// Unassign frees the IP associated with service, if any.
func (a *Allocator) Unassign(svc string) bool {
	if a.allocated[svc] == nil {
		return false
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
		if a.poolIPsInUse[al.pool][ip.String()] == 0 {
			// Explicitly delete unused IPs from the pool, so that len()
			// is an accurate count of IPs in use.
			delete(a.poolIPsInUse[al.pool], ip.String())
		}
	}
	stats.poolActive.WithLabelValues(al.pool).Set(float64(len(a.poolIPsInUse[al.pool])))
	return true
}

// AllocateFromPool assigns an available IP from pool to service.
func (a *Allocator) AllocateFromPool(svc string, serviceIPFamily ipfamily.Family, poolName string, ports []Port, sharingKey, backendKey string) ([]net.IP, error) {
	if alloc := a.allocated[svc]; alloc != nil {
		// Handle the case where the svc has already been assigned an IP but from the wrong family.
		// This "should-not-happen" since the "serviceIPFamily" is an immutable field in services.
		allocIPsFamily, err := ipfamily.ForAddressesIPs(alloc.ips)
		if err != nil {
			return nil, err
		}
		if allocIPsFamily != serviceIPFamily {
			return nil, fmt.Errorf("IP for wrong family assigned alloc %s service family %s", allocIPsFamily, serviceIPFamily)
		}
		if err := a.Assign(svc, alloc.ips, ports, sharingKey, backendKey); err != nil {
			return nil, err
		}
		return alloc.ips, nil
	}

	pool := a.pools[poolName]
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
		ip := a.getIPFromCIDR(cidr, pool.AvoidBuggyIPs, svc, ports, sharingKey, backendKey)
		if ip != nil {
			ips = append(ips, ip)
			delete(ipfamilySel, cidrIPFamily)
		}
	}

	if len(ipfamilySel) > 0 {
		// Woops, run out of IPs :( Fail.
		return nil, fmt.Errorf("no available IPs in pool %q for %s IPFamily", poolName, serviceIPFamily)
	}
	err := a.Assign(svc, ips, ports, sharingKey, backendKey)
	if err != nil {
		return nil, err
	}
	return ips, nil
}

// Allocate assigns any available and assignable IP to service.
func (a *Allocator) Allocate(svc string, serviceIPFamily ipfamily.Family, ports []Port, sharingKey, backendKey string) ([]net.IP, error) {
	if alloc := a.allocated[svc]; alloc != nil {
		if err := a.Assign(svc, alloc.ips, ports, sharingKey, backendKey); err != nil {
			return nil, err
		}
		return alloc.ips, nil
	}

	for poolName := range a.pools {
		if !a.pools[poolName].AutoAssign {
			continue
		}
		if ips, err := a.AllocateFromPool(svc, serviceIPFamily, poolName, ports, sharingKey, backendKey); err == nil {
			return ips, nil
		}
	}

	return nil, errors.New("no available IPs")
}

// Pool returns the pool from which service's IP was allocated. If
// service has no IP allocated, "" is returned.
func (a *Allocator) Pool(svc string) string {
	if alloc := a.allocated[svc]; alloc != nil {
		return poolFor(a.pools, alloc.ips)
	}
	return ""
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
	}
	return total
}

// poolFor returns the pool that owns the requested IPs, or "" if none.
func poolFor(pools map[string]*config.Pool, ips []net.IP) string {
	for pname, p := range pools {
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
			return pname
		}
	}
	return ""
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
				return fmt.Errorf("can't change sharing key for %q, address also in use by %s: %w", svc, strings.Join(otherSvcs, ","), ErrCannotShareKey)
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
