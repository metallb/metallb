package allocator // import "go.universe.tf/metallb/internal/allocator"

import (
	"errors"
	"fmt"
	"math"
	"net"

	"go.universe.tf/metallb/internal/config"
)

// An Allocator tracks IP address pools and allocates addresses from them.
type Allocator struct {
	pools map[string]*config.Pool

	svcToIP       map[string]net.IP
	svcToPool     map[string]string
	ipToSvc       map[string]string
	poolAllocated map[string]int64
}

// New returns an Allocator managing no pools.
func New() *Allocator {
	return &Allocator{
		pools: map[string]*config.Pool{},

		svcToIP:       map[string]net.IP{},
		svcToPool:     map[string]string{},
		ipToSvc:       map[string]string{},
		poolAllocated: map[string]int64{},
	}
}

// SetPools updates the set of address pools that the allocator owns.
func (a *Allocator) SetPools(pools map[string]*config.Pool) error {
	for svc, ip := range a.svcToIP {
		if poolFor(pools, svc, ip) == "" {
			return fmt.Errorf("new config not compatible with assigned IPs: service %q cannot own %q under new config", svc, ip)
		}
	}

	for n := range a.pools {
		if pools[n] == nil {
			stats.poolCapacity.DeleteLabelValues(n)
			stats.poolActive.DeleteLabelValues(n)
		}
	}

	a.pools = pools

	// Need to readjust the existing pool mappings and counts
	for svc, ip := range a.svcToIP {
		pool := poolFor(a.pools, svc, ip)
		if a.svcToPool[svc] != pool {
			a.poolAllocated[a.svcToPool[svc]]--
			a.svcToPool[svc] = pool
			a.poolAllocated[a.svcToPool[svc]]++
		}
	}

	for n, pool := range a.pools {
		stats.poolCapacity.WithLabelValues(n).Set(float64(poolCount(pool)))
		stats.poolActive.WithLabelValues(n).Set(float64(a.poolAllocated[n]))
	}

	return nil
}

// poolCount returns the number of addresses in the pool.
func poolCount(p *config.Pool) int64 {
	var total int64
	for _, cidr := range p.CIDR {
		o, b := cidr.Mask.Size()
		sz := int64(math.Pow(2, float64(b-o)))
		avoidedFirst, avoidedLast := false, false
		if p.AvoidBuggyIPs {
			if o <= 24 {
				// A pair of buggy IPs occur for each /24 present in the range.
				buggies := int64(math.Pow(2, float64(24-o))) * 2
				sz -= buggies
				avoidedFirst, avoidedLast = true, true
			} else {
				// Ranges smaller than /24 contain 1 buggy IP if they
				// start/end on a /24 boundary, otherwise they contain
				// none.
				if ipConfusesBuggyFirmwares(firstIP(cidr)) {
					avoidedFirst = true
					sz--
				}
				if ipConfusesBuggyFirmwares(lastIP(cidr)) {
					avoidedLast = true
					sz--
				}
			}
		}
		if p.ARPNetwork != nil {
			if ipForbiddenByARPNetwork(firstIP(cidr), p.ARPNetwork) && !avoidedFirst {
				avoidedFirst = true
				sz--
			}
			if ipForbiddenByARPNetwork(lastIP(cidr), p.ARPNetwork) && !avoidedLast {
				avoidedLast = true
				sz--
			}
		}
		total += sz
	}
	return total
}

// poolFor returns the pool that owns the requested IP, or "" if none.
func poolFor(pools map[string]*config.Pool, service string, ip net.IP) string {
	for pname, p := range pools {
		if p.AvoidBuggyIPs && ipConfusesBuggyFirmwares(ip) {
			continue
		}
		if p.ARPNetwork != nil && ipForbiddenByARPNetwork(ip, p.ARPNetwork) {
			continue
		}
		for _, cidr := range p.CIDR {
			if cidr.Contains(ip) {
				return pname
			}
		}
	}
	return ""
}

// assign records an assignment. It is the caller's responsibility to
// verify that the assignment is permissible.
func (a *Allocator) assign(service, pool string, ip net.IP) {
	if !a.svcToIP[service].Equal(ip) {
		a.poolAllocated[pool]++
		stats.poolActive.WithLabelValues(pool).Inc()
	}
	a.svcToIP[service] = ip
	a.svcToPool[service] = pool
	a.ipToSvc[ip.String()] = service
}

// Assign marks service as the owner of ip, if that address is available.
func (a *Allocator) Assign(service string, ip net.IP) error {
	if other, ok := a.ipToSvc[ip.String()]; ok {
		if other != service {
			return fmt.Errorf("cannot assign %q to %q, already owned by %q", ip, service, other)
		}
		// IP already allocated correctly, nothing to do.
		return nil
	}
	pool := poolFor(a.pools, service, ip)
	if pool == "" {
		return fmt.Errorf("cannot assign %q to %q, no pool owns that IP", ip, service)
	}

	// If the service already has another assignment, clear it. This
	// is idempotent, so won't do harm if there's no allocation.
	a.Unassign(service)
	a.assign(service, pool, ip)
	return nil
}

// allocateFromPool tries to allocate an IP from pool. Returns nil if no IPs are available.
func (a *Allocator) allocateFromPool(service, pname string) net.IP {
	pool := a.pools[pname]
	for _, cidr := range pool.CIDR {
		for ip := cidr.IP; cidr.Contains(ip); ip = nextIP(ip) {
			if pool.AvoidBuggyIPs && ipConfusesBuggyFirmwares(ip) {
				continue
			}
			if pool.ARPNetwork != nil && ipForbiddenByARPNetwork(ip, pool.ARPNetwork) {
				continue
			}
			if a.ipToSvc[ip.String()] == "" {
				a.assign(service, pname, ip)
				return ip
			}
		}
	}

	// No IPs available.
	return nil
}

// AllocateFromPool assigns an available IP from pool to service.
func (a *Allocator) AllocateFromPool(service, pool string) (net.IP, error) {
	if a.pools[pool] == nil {
		return nil, fmt.Errorf("pool %q does not exist", pool)
	}
	ip := a.allocateFromPool(service, pool)
	if ip == nil {
		return nil, fmt.Errorf("no addresses available in pool %q", pool)
	}
	return ip, nil
}

// Allocate assigns any available and assignable IP to service.
func (a *Allocator) Allocate(service string) (net.IP, error) {
	for pname := range a.pools {
		if !a.pools[pname].AutoAssign {
			continue
		}
		if ip := a.allocateFromPool(service, pname); ip != nil {
			return ip, nil
		}
	}
	return nil, errors.New("no addresses available in any pool")
}

// Unassign frees the IP associated with service, if any.
func (a *Allocator) Unassign(service string) bool {
	ip := a.svcToIP[service]
	if ip == nil {
		// Service didn't have an assignment, nothing to do.
		return false
	}
	a.poolAllocated[a.svcToPool[service]]--
	stats.poolActive.WithLabelValues(a.svcToPool[service]).Dec()
	delete(a.svcToIP, service)
	delete(a.svcToPool, service)
	delete(a.ipToSvc, ip.String())
	return true
}

// IP returns the IP address allocated to service, or nil if none are allocated.
func (a *Allocator) IP(service string) net.IP {
	return a.svcToIP[service]
}

// Pool returns the pool from which service's IP was allocated. If
// service has no IP allocated, "" is returned.
func (a *Allocator) Pool(service string) string {
	return a.svcToPool[service]
}

// nextIP returns the next IP in sequence after prev.
func nextIP(prev net.IP) net.IP {
	var ip net.IP
	ip = append(ip, prev...)
	if ip.To4() != nil {
		ip = ip.To4()
	}
	for o := 0; o < len(ip); o++ {
		if ip[len(ip)-o-1] != 255 {
			ip[len(ip)-o-1]++
			return ip
		}
		ip[len(ip)-o-1] = 0
	}
	return ip
}

// lastIP returns the last IP in the given IPNet.
func lastIP(ipnet *net.IPNet) net.IP {
	var ip net.IP
	ip = append(ip, ipnet.IP...)
	if ip4 := ip.To4(); ip4 != nil {
		ip = ip4
	}
	for i := range ip {
		ip[i] |= ^ipnet.Mask[i]
	}
	return ip
}

// firstIP returns the first IP in the given IPNet.
func firstIP(ipnet *net.IPNet) net.IP {
	return ipnet.IP.Mask(ipnet.Mask)
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

// ipForbiddenByARPNetwork returns true if ip is the network or
// broadcast address of arpNet.
func ipForbiddenByARPNetwork(ip net.IP, arpNet *net.IPNet) bool {
	return ip.Equal(firstIP(arpNet)) || ip.Equal(lastIP(arpNet))
}
