// SPDX-License-Identifier:Apache-2.0

package config

import (
	"fmt"
	"math"
	"net"

	"errors"

	"github.com/mikioh/ipaddr"
	. "github.com/onsi/gomega"
	"go.universe.tf/e2etest/pkg/iprange"
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
)

func ValidateIPInRange(addressPools []metallbv1beta1.IPAddressPool, ip string) error {
	input := net.ParseIP(ip)
	for _, addressPool := range addressPools {
		for _, address := range addressPool.Spec.Addresses {
			cidrs, err := iprange.Parse(address)
			Expect(err).NotTo(HaveOccurred())
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
	cidrs, err := iprange.Parse(ipRange)
	if err != nil {
		return "", errors.Join(err, errors.New("Failed to parse CIDR while getting IP from range by index"))
	}

	i := 0
	var c *ipaddr.Cursor
	for _, cidr := range cidrs {
		c = ipaddr.NewCursor([]ipaddr.Prefix{*ipaddr.NewPrefix(cidr)})
		for i < index && c.Next() != nil {
			i++
		}
		if i == index {
			return c.Pos().IP.String(), nil
		}
		i++
	}

	return "", fmt.Errorf("failed to get IP in index %d from range %s", index, ipRange)
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

			if p.Spec.AvoidBuggyIPs {
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
	}
	return total, ipv4, ipv6, nil
}

func ipConfusesBuggyFirmwares(ip net.IP) bool {
	ip = ip.To4()
	if ip == nil {
		return false
	}
	return ip[3] == 0 || ip[3] == 255
}
