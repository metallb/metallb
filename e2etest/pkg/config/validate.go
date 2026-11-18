// SPDX-License-Identifier:Apache-2.0

package config

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"net/netip"

	"go.universe.tf/e2etest/pkg/ipfamily"
	"go.universe.tf/e2etest/pkg/iprange"
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	"go4.org/netipx"
)

func ValidateIPInRange(addressPools []metallbv1beta1.IPAddressPool, ip string) error {
	input, err := netip.ParseAddr(ip)
	if err != nil {
		return fmt.Errorf("invalid IP %q: %w", ip, err)
	}
	input = input.Unmap()
	for _, addressPool := range addressPools {
		for _, address := range addressPool.Spec.Addresses {
			cidrs, err := iprange.Parse(address)
			if err != nil {
				return fmt.Errorf("invalid address pool range %q: %w", address, err)
			}
			for _, cidr := range cidrs {
				if cidr.Contains(input) {
					return nil
				}
			}
		}
	}
	return fmt.Errorf("ip %s is not in AddressPool range", ip)
}

func GetIPFromRangeByIndex(ipRange string, index int) (string, error) {
	originalIndex := index
	if index < 0 {
		return "", fmt.Errorf("invalid negative index %d", index)
	}
	cidrs, err := iprange.Parse(ipRange)
	if err != nil {
		return "", errors.Join(err, errors.New("Failed to parse CIDR while getting IP from range by index"))
	}

	for _, cidr := range cidrs {
		cidr = cidr.Masked()
		hostBits := cidr.Addr().BitLen() - cidr.Bits()
		// Every non-negative int index fits within a prefix with at least
		// 63 host bits on supported 64-bit platforms.
		if hostBits >= 63 {
			addr, err := addressAtOffset(cidr.Addr(), index)
			if err != nil {
				return "", err
			}
			return addr.String(), nil
		}
		count := int64(1) << uint(hostBits)
		if int64(index) < count {
			addr, err := addressAtOffset(cidr.Addr(), index)
			if err != nil {
				return "", err
			}
			return addr.String(), nil
		}
		index -= int(count)
	}

	return "", fmt.Errorf("failed to get IP in index %d from range %s", originalIndex, ipRange)
}

// PoolCount returns the number of total and ipv4 and ipv6 addresses in a given Pool.
func PoolCount(p metallbv1beta1.IPAddressPool) (int64, int64, int64, error) {
	var total int64
	var ipv4 int64
	var ipv6 int64
	for _, r := range p.Spec.Addresses {
		cidrs, err := iprange.Parse(r)
		if err != nil {
			return 0, 0, 0, err
		}
		for _, cidr := range cidrs {
			cidr = cidr.Masked()
			prefixBits := cidr.Bits()
			addressBits := cidr.Addr().BitLen()
			hostBits := addressBits - prefixBits
			if hostBits >= 62 {
				// An enormous ipv6 range is allocated which will never run out.
				total = math.MaxInt64
				ipv6 = math.MaxInt64
				continue
			}
			sz := int64(1) << uint(hostBits)
			firstIP := cidr.Addr()
			lastIP := netipx.PrefixLastIP(cidr)

			if p.Spec.AvoidBuggyIPs {
				if prefixBits <= 24 {
					// A pair of buggy IPs occur for each /24 present in the range.
					buggies := (int64(1) << uint(24-prefixBits)) * 2
					sz -= buggies
				} else {
					// Ranges smaller than /24 contain 1 buggy IP if they
					// start/end on a /24 boundary, otherwise they contain
					// none.
					if ipConfusesBuggyFirmwares(firstIP) {
						sz--
					}
					if lastIP != firstIP && ipConfusesBuggyFirmwares(lastIP) {
						sz--
					}
				}
			}

			total = saturatedAdd(total, sz)
			if ipfamily.ForCIDR(cidr) == ipfamily.IPv6 {
				ipv6 = saturatedAdd(ipv6, sz)
			} else {
				ipv4 = saturatedAdd(ipv4, sz)
			}
		}
	}
	return total, ipv4, ipv6, nil
}

func ipConfusesBuggyFirmwares(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.Is4() {
		return false
	}
	addr := ip.As4()
	return addr[3] == 0 || addr[3] == 255
}

func addressAtOffset(addr netip.Addr, offset int) (netip.Addr, error) {
	if !addr.IsValid() || offset < 0 {
		return netip.Addr{}, fmt.Errorf("invalid address offset %d for %q", offset, addr)
	}
	value := new(big.Int).SetBytes(addr.AsSlice())
	value.Add(value, big.NewInt(int64(offset)))
	if value.BitLen() > addr.BitLen() {
		return netip.Addr{}, fmt.Errorf("address offset %d exceeds address range starting at %s", offset, addr)
	}
	if addr.Is4() {
		var bytes [4]byte
		value.FillBytes(bytes[:])
		return netip.AddrFrom4(bytes), nil
	}
	var bytes [16]byte
	value.FillBytes(bytes[:])
	return netip.AddrFrom16(bytes), nil
}

func saturatedAdd(a, b int64) int64 {
	if b > 0 && a > math.MaxInt64-b {
		return math.MaxInt64
	}
	return a + b
}
