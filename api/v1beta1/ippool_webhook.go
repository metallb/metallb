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

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging IPPool-webhook.
var (
	ipPoolLog    = logf.Log.WithName("IPPool-webhook")
	ipPoolClient client.Client
)

func (ipPool *IPPool) SetupWebhookWithManager(mgr ctrl.Manager) error {
	ipPoolClient = mgr.GetClient()
	return ctrl.NewWebhookManagedBy(mgr).
		For(ipPool).
		Complete()
}

//+kubebuilder:webhook:verbs=create;update,path=/validate-metallb-io-v1beta1-ippool,mutating=false,failurePolicy=fail,groups=metallb.io,resources=ippools,versions=v1beta1,name=ippoolvalidationwebhook.metallb.io,sideEffects=None,admissionReviewVersions=v1

var _ webhook.Validator = &IPPool{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for IPPool.
func (ipPool *IPPool) ValidateCreate() error {
	ipPoolLog.Info("validate IPPool creation", "name", ipPool.Name)

	existingIPPoolList, err := getExistingIPPools()
	if err != nil {
		return err
	}

	existingAddressPoolList, err := getExistingAddressPools()
	if err != nil {
		return err
	}

	return ipPool.ValidateIPPool(true, existingIPPoolList.Items, existingAddressPoolList.Items)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for IPPool.
func (ipPool *IPPool) ValidateUpdate(old runtime.Object) error {
	ipPoolLog.Info("validate IPPool update", "name", ipPool.Name)

	existingIPPoolList, err := getExistingIPPools()
	if err != nil {
		return err
	}

	existingAddressPoolList, err := getExistingAddressPools()
	if err != nil {
		return err
	}

	return ipPool.ValidateIPPool(false, existingIPPoolList.Items, existingAddressPoolList.Items)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for IPPool.
func (ipPool *IPPool) ValidateDelete() error {
	ipPoolLog.Info("validate IPPool deletion", "name", ipPool.Name)

	return nil
}

func (ipPool *IPPool) ValidateIPPool(isNewIPPool bool, existingIPPools []IPPool, existingAddressPools []AddressPool) error {
	if ipPool.Name == "" {
		return errors.New("Missing IPPool name")
	}

	if len(ipPool.Spec.Addresses) == 0 {
		return errors.New("IPPool has no prefixes defined")
	}

	IPPoolCIDRs, err := getPoolCIDRs(ipPool.Spec.Addresses, ipPool.Name)
	if err != nil {
		return errors.Wrapf(err, "Failed to parse addresses for %s", ipPool.Name)
	}

	for _, existingIPPool := range existingIPPools {
		if existingIPPool.Name == ipPool.Name {
			// Check that the pool isn't already defined.
			// Avoid errors when comparing the IPPool to itself.
			if isNewIPPool {
				return fmt.Errorf("duplicate definition of pool %s", ipPool.Name)
			} else {
				continue
			}
		}

		existingIPPoolCIDRS, err := getPoolCIDRs(existingIPPool.Spec.Addresses, existingIPPool.Name)
		if err != nil {
			return errors.Wrapf(err, "Failed to parse addresses for %s", existingIPPool.Name)
		}

		// Check that the specified CIDR ranges are not overlapping in existing CIDRs.
		err = validateCIDRs(IPPoolCIDRs, ipPool.Name, existingIPPoolCIDRS, existingIPPool.Name)
		if err != nil {
			return err
		}
	}

	err = validateAddressPoolsVsIPPools(existingAddressPools, []IPPool{*ipPool}, isNewIPPool)
	if err != nil {
		return err
	}

	return nil
}

func getExistingIPPools() (*IPPoolList, error) {
	existingIPPoolList := &IPPoolList{}
	err := ipPoolClient.List(context.Background(), existingIPPoolList)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to get existing IPPool objects")
	}
	return existingIPPoolList, nil
}
