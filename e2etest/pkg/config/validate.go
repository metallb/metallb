// SPDX-License-Identifier:Apache-2.0

package config

import (
	"fmt"
	"math"
	"net"

	"github.com/mikioh/ipaddr"
	"github.com/pkg/errors"
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	internalconfig "go.universe.tf/metallb/internal/config"
	"k8s.io/kubernetes/test/e2e/framework"
)

func ValidateIPInRange(addressPools []metallbv1beta1.IPAddressPool, ip string) error {
	input := net.ParseIP(ip)
	for _, addressPool := range addressPools {
		for _, address := range addressPool.Spec.Addresses {
			cidrs, err := internalconfig.ParseCIDR(address)
			framework.ExpectNoError(err)
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
	cidrs, err := internalconfig.ParseCIDR(ipRange)
	if err != nil {
		return "", errors.Wrapf(err, "Failed to parse CIDR while getting IP from range by index")
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

// PoolCount returns the number of addresses in a given Pool.
func PoolCount(p metallbv1beta1.IPAddressPool) (int64, error) {
	var total int64
	for _, r := range p.Spec.Addresses {
		cidrs, err := internalconfig.ParseCIDR(r)
		if err != nil {
			return 0, err
		}
		for _, cidr := range cidrs {
			o, b := cidr.Mask.Size()
			if b-o >= 62 {
				// An enormous ipv6 range is allocated which will never run out.
				// Just return max to avoid any math errors.
				return math.MaxInt64, nil
			}
			sz := int64(math.Pow(2, float64(b-o)))
			total += sz
		}
	}
	return total, nil
}
