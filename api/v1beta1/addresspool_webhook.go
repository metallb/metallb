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

	"github.com/go-kit/kit/log/level"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func (addressPool *AddressPool) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(addressPool).
		Complete()
}

//+kubebuilder:webhook:verbs=create;update,path=/validate-metallb-io-v1beta1-addresspool,mutating=false,failurePolicy=fail,groups=metallb.io,resources=addresspools,versions=v1beta1,name=addresspoolvalidationwebhook.metallb.io,sideEffects=None,admissionReviewVersions=v1

var _ webhook.Validator = &AddressPool{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for AddressPool.
func (addressPool *AddressPool) ValidateCreate() error {
	level.Debug(Logger).Log("webhook", "addressPool", "action", "create", "name", addressPool.Name, "namespace", addressPool.Namespace)

	existingAddressPoolList, err := getExistingAddressPools()
	if err != nil {
		return err
	}

	existingIPAddressPoolList, err := getExistingIPAddressPools()
	if err != nil {
		return err
	}
	addressPoolList := listWithUpdate(existingAddressPoolList, addressPool)
	err = Validator.Validate(addressPoolList, existingIPAddressPoolList)
	if err != nil {
		level.Error(Logger).Log("webhook", "addressPool", "action", "create", "name", addressPool.Name, "namespace", addressPool.Namespace, "error", err)
		return err
	}
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for AddressPool.
func (addressPool *AddressPool) ValidateUpdate(old runtime.Object) error {
	level.Debug(Logger).Log("webhook", "addressPool", "action", "update", "name", addressPool.Name, "namespace", addressPool.Namespace)

	existingAddressPoolList, err := getExistingAddressPools()
	if err != nil {
		return err
	}

	existingIPAddressPoolList, err := getExistingIPAddressPools()
	if err != nil {
		return err
	}
	addressPoolList := listWithUpdate(existingAddressPoolList, addressPool)
	err = Validator.Validate(addressPoolList, existingIPAddressPoolList)
	if err != nil {
		level.Error(Logger).Log("webhook", "addressPool", "action", "update", "name", addressPool.Name, "namespace", addressPool.Namespace, "error", err)
		return err
	}
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for AddressPool.
func (addressPool *AddressPool) ValidateDelete() error {
	return nil
}

var getExistingAddressPools = func() (*AddressPoolList, error) {
	existingAddressPoolList := &AddressPoolList{}
	err := WebhookClient.List(context.Background(), existingAddressPoolList, &client.ListOptions{Namespace: MetalLBNamespace})
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to get existing AddressPool objects")
	}
	return existingAddressPoolList, nil
}

func listWithUpdate(existing *AddressPoolList, toAdd *AddressPool) *AddressPoolList {
	res := existing.DeepCopy()
	for i, item := range res.Items { // We override the element with the fresh copy
		if item.Name == toAdd.Name {
			res.Items[i] = *toAdd.DeepCopy()
			return res
		}
	}
	res.Items = append(res.Items, *toAdd.DeepCopy())
	return res
}
