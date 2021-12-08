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
	"context"
	"fmt"
	"net"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging addresspool-webhook.
var (
	addresspoollog    = logf.Log.WithName("addresspool-webhook")
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
	addresspoollog.Info("validate AddressPool creation", "name", addressPool.Name)

	existingAddressPoolList, err := getExistingAddressPools()
	if err != nil {
		return err
	}

	return addressPool.validateAddressPool(true, existingAddressPoolList)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for AddressPool.
func (addressPool *AddressPool) ValidateUpdate(old runtime.Object) error {
	addresspoollog.Info("validate AddressPool update", "name", addressPool.Name)

	existingAddressPoolList, err := getExistingAddressPools()
	if err != nil {
		return err
	}

	return addressPool.validateAddressPool(false, existingAddressPoolList)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for AddressPool.
func (addressPool *AddressPool) ValidateDelete() error {
	addresspoollog.Info("validate AddressPool deletion", "name", addressPool.Name)

	return nil
}

func (addressPool *AddressPool) validateAddressPool(isNewAddressPool bool, existingAddressPoolList *AddressPoolList) error {
	addressPoolCIDRS, err := getAddressPoolCIDRs(addressPool)
	if err != nil {
		return errors.Wrapf(err, "Failed to parse addresses for %s", addressPool.Name)
	}

	// Check protocol is BGP when BGPAdvertisement is used.
	if len(addressPool.Spec.BGPAdvertisements) != 0 {
		if addressPool.Spec.Protocol != "bgp" {
			return fmt.Errorf("bgpadvertisement config not valid for protocol %s", addressPool.Spec.Protocol)
		}
	}

	for _, existingAddressPool := range existingAddressPoolList.Items {
		if existingAddressPool.Name == addressPool.Name {
			// Check that the pool isn't already defined.
			// Avoid errors when comparing the AddressPool to itself.
			if isNewAddressPool {
				return fmt.Errorf("duplicate definition of pool %s", addressPool.Name)
			} else {
				continue
			}
		}

		existingAddressPoolCIDRS, err := getAddressPoolCIDRs(&existingAddressPool)
		if err != nil {
			return errors.Wrapf(err, "Failed to parse addresses for %s", existingAddressPool.Name)
		}

		// Check that the specified CIDR ranges are not overlapping in existing CIDRs.
		for _, existingCIDR := range existingAddressPoolCIDRS {
			for _, cidr := range addressPoolCIDRS {
				if cidrsOverlap(existingCIDR, cidr) {
					return fmt.Errorf("CIDR %q in pool %s overlaps with already defined CIDR %q in pool %s", cidr, addressPool.Name, existingCIDR, existingAddressPool.Name)
				}
			}
		}
	}
	return nil
}

func getExistingAddressPools() (*AddressPoolList, error) {
	existingAddressPoolList := &AddressPoolList{}
	err := addressPoolClient.List(context.Background(), existingAddressPoolList)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to get existing addresspool objects")
	}
	return existingAddressPoolList, nil
}

func getAddressPoolCIDRs(addressPool *AddressPool) ([]*net.IPNet, error) {
	var CIDRs []*net.IPNet
	for _, cidr := range addressPool.Spec.Addresses {
		nets, err := parseCIDR(cidr)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid CIDR %q in pool %s", cidr, addressPool.Name)
		}
		CIDRs = append(CIDRs, nets...)
	}
	return CIDRs, nil
}
