/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1beta1

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"reflect"
	"strconv"
	"strings"

	"github.com/mikioh/ipaddr"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging addresspool-webhook.
var (
	addressPoolLog    = logf.Log.WithName("addresspool-webhook")
	addressPoolClient client.Client
)

func (addressPool *AddressPool) SetupWebhookWithManager(mgr ctrl.Manager) error {
	addressPoolClient = mgr.GetClient()
	return ctrl.NewWebhookManagedBy(mgr).
		For(addressPool).
		Complete()
}

//+kubebuilder:webhook:verbs=create;update,path=/validate-metallb-io-v1beta1-addresspool,mutating=false,failurePolicy=fail,groups=metallb.io,resources=addresspools,versions=v1beta1,name=addresspoolvalidationwebhook.metallb.io,sideEffects=None,admissionReviewVersions=v1

var _ webhook.Validator = &AddressPool{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for AddressPool.
func (addressPool *AddressPool) ValidateCreate() error {
	addressPoolLog.Info("validate AddressPool creation", "name", addressPool.Name)

	existingAddressPoolList, err := getExistingAddressPools()
	if err != nil {
		return err
	}

	existingIPPoolList, err := getExistingIPPools()
	if err != nil {
		return err
	}

	return addressPool.ValidateAddressPool(true, existingAddressPoolList.Items, existingIPPoolList.Items)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for AddressPool.
func (addressPool *AddressPool) ValidateUpdate(old runtime.Object) error {
	addressPoolLog.Info("validate AddressPool update", "name", addressPool.Name)

	existingAddressPoolList, err := getExistingAddressPools()
	if err != nil {
		return err
	}

	existingIPPoolList, err := getExistingIPPools()
	if err != nil {
		return err
	}

	return addressPool.ValidateAddressPool(false, existingAddressPoolList.Items, existingIPPoolList.Items)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for AddressPool.
func (addressPool *AddressPool) ValidateDelete() error {
	addressPoolLog.Info("validate AddressPool deletion", "name", addressPool.Name)

	return nil
}

func (addressPool *AddressPool) ValidateAddressPool(isNewAddressPool bool, existingAddressPools []AddressPool, existingIPPools []IPPool) error {
	if addressPool.Name == "" {
		return errors.New("Missing AddressPool name")
	}

	if len(addressPool.Spec.Addresses) == 0 {
		return errors.New("AddressPool has no prefixes defined")
	}

	if addressPool.Spec.Protocol == "" {
		return errors.New("AddressPool is missing the protocol field")
	}
	if addressPool.Spec.Protocol != "bgp" && addressPool.Spec.Protocol != "layer2" {
		return fmt.Errorf("AddressPool has unknown protocol %q", addressPool.Spec.Protocol)
	}

	// Check protocol is BGP when BGPAdvertisement is used.
	err := validateBGPAdvertisements(addressPool)
	if err != nil {
		return errors.Wrapf(err, "invalid bgpadvertisement config")
	}

	addressPoolCIDRs, err := getPoolCIDRs(addressPool.Spec.Addresses, addressPool.Name)
	if err != nil {
		return errors.Wrapf(err, "Failed to parse addresses for %s", addressPool.Name)
	}

	for _, existingAddressPool := range existingAddressPools {
		if existingAddressPool.Name == addressPool.Name {
			// Check that the pool isn't already defined.
			// Avoid errors when comparing the AddressPool to itself.
			if isNewAddressPool {
				return fmt.Errorf("duplicate definition of pool %s", addressPool.Name)
			} else {
				continue
			}
		}

		existingAddressPoolCIDRs, err := getPoolCIDRs(existingAddressPool.Spec.Addresses, existingAddressPool.Name)
		if err != nil {
			return errors.Wrapf(err, "Failed to parse addresses for %s", existingAddressPool.Name)
		}

		// Check that the specified CIDR ranges are not overlapping in existing CIDRs.
		err = validateCIDRs(addressPoolCIDRs, addressPool.Name, existingAddressPoolCIDRs, existingAddressPool.Name)
		if err != nil {
			return err
		}
	}

	err = validateAddressPoolsVsIPPools([]AddressPool{*addressPool}, existingIPPools, isNewAddressPool)
	if err != nil {
		return err
	}

	return nil
}

func getExistingAddressPools() (*AddressPoolList, error) {
	existingAddressPoolList := &AddressPoolList{}
	err := addressPoolClient.List(context.Background(), existingAddressPoolList)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to get existing AddressPool objects")
	}
	return existingAddressPoolList, nil
}

func getPoolCIDRs(addresses []string, addressPoolName string) ([]*net.IPNet, error) {
	var CIDRs []*net.IPNet
	for _, cidr := range addresses {
		nets, err := ParseCIDR(cidr)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid CIDR %q in pool %s", cidr, addressPoolName)
		}
		CIDRs = append(CIDRs, nets...)
	}
	return CIDRs, nil
}

func validateCIDRs(addressPoolCIDRs []*net.IPNet, addressPoolName string, existingCIDRs []*net.IPNet, existingAddressPoolName string) error {
	// Check that the specified CIDR ranges are not overlapping in existing CIDRs.
	for _, existingCIDR := range existingCIDRs {
		for _, cidr := range addressPoolCIDRs {
			if CidrsOverlap(existingCIDR, cidr) {
				return fmt.Errorf("CIDR %q in pool %s overlaps with already defined CIDR %q in pool %s", cidr, addressPoolName, existingCIDR, existingAddressPoolName)
			}
		}

	}
	return nil
}

func validateAddressPoolsVsIPPools(addressPools []AddressPool, ipPools []IPPool, isNew bool) error {
	for _, addressPool := range addressPools {
		for _, ipPool := range ipPools {
			if addressPool.Name == ipPool.Name {
				// Check that the pool isn't already defined.
				// Avoid errors when comparing the IPPool to itself.
				if isNew {
					return fmt.Errorf("duplicate definition of pool %s", addressPool.Name)
				} else {
					continue
				}
			}

			addressPoolCIDRs, err := getPoolCIDRs(addressPool.Spec.Addresses, addressPool.Name)
			if err != nil {
				return errors.Wrapf(err, "Failed to parse addresses for %s", addressPool.Name)
			}

			ipPoolCIDRs, err := getPoolCIDRs(ipPool.Spec.Addresses, ipPool.Name)
			if err != nil {
				return errors.Wrapf(err, "Failed to parse addresses for %s", ipPool.Name)
			}

			// Check that the specified CIDR ranges are not overlapping in existing CIDRs.
			err = validateCIDRs(addressPoolCIDRs, addressPool.Name, ipPoolCIDRs, ipPool.Name)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func validateBGPAdvertisements(addressPool *AddressPool) error {
	if len(addressPool.Spec.BGPAdvertisements) == 0 {
		return nil
	}

	if addressPool.Spec.Protocol != "bgp" {
		return fmt.Errorf("bgpadvertisement config not valid for protocol %s", addressPool.Spec.Protocol)
	}

	err := validateDuplicateLegacyBgpAdvertisements(addressPool.Spec.BGPAdvertisements)
	if err != nil {
		return err
	}

	for _, adv := range addressPool.Spec.BGPAdvertisements {
		err := validateDuplicateCommunities(adv.Communities)
		if err != nil {
			return err
		}

		err = validateAggregationLengthPerPool(adv.AggregationLength, adv.AggregationLengthV6, addressPool.Spec.Addresses, addressPool.Name)
		if err != nil {
			return err
		}

		for _, community := range adv.Communities {
			fs := strings.Split(community, ":")
			if len(fs) != 2 {
				return fmt.Errorf("invalid community string %q", community)
			}

			_, err := strconv.ParseUint(fs[0], 10, 16)
			if err != nil {
				return fmt.Errorf("invalid first section of community %q: %s", fs[0], err)
			}

			_, err = strconv.ParseUint(fs[1], 10, 16)
			if err != nil {
				return fmt.Errorf("invalid second section of community %q: %s", fs[1], err)
			}
		}
	}

	return nil
}

func validateDuplicateLegacyBgpAdvertisements(bgpAdvertisements []LegacyBgpAdvertisement) error {
	for i := 0; i < len(bgpAdvertisements); i++ {
		for j := i + 1; j < len(bgpAdvertisements); j++ {
			if reflect.DeepEqual(bgpAdvertisements[i], bgpAdvertisements[j]) {
				return errors.New("duplicate definition of bgpadvertisement")
			}
		}
	}
	return nil
}

func validateDuplicateCommunities(communities []string) error {
	for i := 0; i < len(communities); i++ {
		for j := i + 1; j < len(communities); j++ {
			if strings.Compare(communities[i], communities[j]) == 0 {
				return errors.New("duplicate definition of communities")
			}
		}
	}
	return nil
}

func validateAggregationLength(aggregationLength int32, isV6 bool) error {
	if isV6 {
		if aggregationLength > 128 {
			return fmt.Errorf("invalid aggregation length %d for IPv6", aggregationLength)
		}
	} else if aggregationLength > 32 {
		return fmt.Errorf("invalid aggregation length %d for IPv4", aggregationLength)
	}
	return nil
}

func validateAggregationLengthPerPool(aggregationLength *int32, aggregationLengthV6 *int32, addresses []string, poolName string) error {
	if aggregationLength != nil && *aggregationLength > 32 {
		return fmt.Errorf("invalid aggregation length %d for IPv4", *aggregationLength)
	}
	if aggregationLengthV6 != nil && *aggregationLengthV6 > 128 {
		return fmt.Errorf("invalid aggregation length %d for IPv6", *aggregationLengthV6)
	}

	cidrsPerAddresses := map[string][]*net.IPNet{}
	for _, cidr := range addresses {
		nets, err := ParseCIDR(cidr)
		if err != nil {
			return fmt.Errorf("invalid CIDR %q in pool %q: %s", cidr, poolName, err)
		}
		cidrsPerAddresses[cidr] = nets
	}

	for addr, cidrs := range cidrsPerAddresses {
		if len(cidrs) == 0 {
			continue
		}
		var maxLength int32
		if cidrs[0].IP.To4() != nil {
			maxLength = 32
			if aggregationLength != nil {
				maxLength = *aggregationLength
			}
		} else {
			maxLength = 128
			if aggregationLengthV6 != nil {
				maxLength = *aggregationLengthV6
			}
		}

		// in case of range format, we may have a set of cidrs associated to a given address.
		// We reject if none of the cidrs are compatible with the aggregation length.
		lowest := lowestMask(cidrs)
		if maxLength < int32(lowest) {
			return fmt.Errorf("invalid aggregation length %d: prefix %d in "+
				"this pool is more specific than the aggregation length for addresses %s", maxLength, lowest, addr)
		}
	}
	return nil
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

func CidrsOverlap(a, b *net.IPNet) bool {
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
