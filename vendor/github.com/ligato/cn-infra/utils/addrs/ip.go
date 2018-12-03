package addrs

import (
	"bytes"
	"fmt"
	"net"
	"sort"
	"strings"
)

// SortedIPs type is used to implement sort.Interface for slice of IPNet
type SortedIPs []*net.IPNet

// Returns length of slice
// Implements sort.Interface
func (arr SortedIPs) Len() int {
	return len(arr)
}

// Swap swaps two items in slice identified by indexes
// Implements sort.Interface
func (arr SortedIPs) Swap(i, j int) {
	arr[i], arr[j] = arr[j], arr[i]
}

// Less returns true if the item in slice at index i in slice
// should be sorted before the element with index j
// Implements sort.Interface
func (arr SortedIPs) Less(i, j int) bool {
	return lessAdrr(arr[i], arr[j])
}

func eqAddr(a *net.IPNet, b *net.IPNet) bool {
	return bytes.Equal(a.IP, b.IP) && bytes.Equal(a.Mask, b.Mask)
}

func lessAdrr(a *net.IPNet, b *net.IPNet) bool {
	if bytes.Equal(a.IP, b.IP) {
		return bytes.Compare(a.Mask, b.Mask) < 0
	}
	return bytes.Compare(a.IP, b.IP) < 0

}

// DiffAddr calculates the difference between two slices of AddrWithPrefix configuration.
// Returns a list of addresses that should be deleted and added to the current configuration to match newConfig.
func DiffAddr(newConfig []*net.IPNet, oldConfig []*net.IPNet) (toBeDeleted []*net.IPNet, toBeAdded []*net.IPNet) {
	var add []*net.IPNet
	var del []*net.IPNet
	//sort
	n := SortedIPs(newConfig)
	sort.Sort(&n)
	o := SortedIPs(oldConfig)
	sort.Sort(&o)

	//compare
	i := 0
	j := 0
	for i < len(n) && j < len(o) {
		if eqAddr(n[i], o[j]) {
			i++
			j++
		} else {
			if lessAdrr(n[i], o[j]) {
				add = append(add, n[i])
				i++
			} else {
				del = append(del, o[j])
				j++
			}
		}
	}

	for ; i < len(n); i++ {
		add = append(add, n[i])
	}

	for ; j < len(o); j++ {
		del = append(del, o[j])
	}
	return del, add
}

// StrAddrsToStruct converts slice of strings representing ipv4 addresses to IPNet structures
func StrAddrsToStruct(addrs []string) ([]*net.IPNet, error) {
	var result []*net.IPNet
	for _, addressWithPrefix := range addrs {
		if addressWithPrefix == "" {
			continue
		}
		parsedIPWithPrefix, _, err := ParseIPWithPrefix(addressWithPrefix)
		if err != nil {
			return result, err
		}
		result = append(result, parsedIPWithPrefix)
	}

	return result, nil
}

// ParseIPWithPrefix parses string representation of ip address into net.IPNet structure.
// If the prefix is missing default one is added (/32 for IPv4, /128 for IPv6)
func ParseIPWithPrefix(input string) (addr *net.IPNet, isIpv6 bool, err error) {
	defaultIpv4Mask := net.CIDRMask(32, 32)
	defaultIpv6Mask := net.CIDRMask(128, 128)

	hasPrefix := strings.Contains(input, "/")

	if hasPrefix {
		ip, network, err := net.ParseCIDR(input)
		if err != nil {
			return nil, false, err
		}
		network.IP = ip
		isIpv6, err = IsIPv6(ip.String())
		return network, isIpv6, err
	}

	// Ip prefix was not set
	ip := net.ParseIP(input)
	if ip == nil {
		return nil, false, fmt.Errorf("Unable to parse IP address: %v", input)
	}
	isIpv6, err = IsIPv6(ip.String())
	if err != nil {
		return
	}

	network := net.IPNet{}
	network.IP = ip
	if isIpv6 {
		network.Mask = defaultIpv6Mask
	} else {
		network.Mask = defaultIpv4Mask
	}
	return &network, isIpv6, nil
}

// IsIPv6 returns true if provided IP address is IPv6, false otherwise
func IsIPv6(addr string) (bool, error) {
	ip := net.ParseIP(addr)
	if ip == nil {
		return false, fmt.Errorf("invalid IP address: %q", addr)
	}
	return ip.To4() == nil, nil
}
