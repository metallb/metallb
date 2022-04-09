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

func (ipPool *IPPool) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(ipPool).
		Complete()
}

//+kubebuilder:webhook:verbs=create;update,path=/validate-metallb-io-v1beta1-ippool,mutating=false,failurePolicy=fail,groups=metallb.io,resources=ippools,versions=v1beta1,name=ippoolvalidationwebhook.metallb.io,sideEffects=None,admissionReviewVersions=v1

var _ webhook.Validator = &IPPool{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for IPPool.
func (ipPool *IPPool) ValidateCreate() error {
	level.Debug(Logger).Log("webhook", "ippool", "action", "create", "name", ipPool.Name, "namespace", ipPool.Namespace)

	existingIPPoolList, err := getExistingIPPools()
	if err != nil {
		return err
	}

	existingAddressPoolList, err := getExistingAddressPools()
	if err != nil {
		return err
	}
	toValidate := ipPoolListWithUpdate(existingIPPoolList, ipPool)
	err = Validator.Validate(existingAddressPoolList, toValidate)
	if err != nil {
		level.Error(Logger).Log("webhook", "ipPool", "action", "create", "name", ipPool.Name, "namespace", ipPool.Namespace, "error", err)
		return err
	}
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for IPPool.
func (ipPool *IPPool) ValidateUpdate(old runtime.Object) error {
	level.Debug(Logger).Log("webhook", "ippool", "action", "update", "name", ipPool.Name, "namespace", ipPool.Namespace)
	existingAddressPoolList, err := getExistingAddressPools()
	if err != nil {
		return err
	}

	existingIPPoolList, err := getExistingIPPools()
	if err != nil {
		return err
	}

	toValidate := ipPoolListWithUpdate(existingIPPoolList, ipPool)
	err = Validator.Validate(existingAddressPoolList, toValidate)
	if err != nil {
		level.Error(Logger).Log("webhook", "ipPool", "action", "update", "name", ipPool.Name, "namespace", ipPool.Namespace, "error", err)
		return err
	}
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for IPPool.
func (ipPool *IPPool) ValidateDelete() error {
	return nil
}

var getExistingIPPools = func() (*IPPoolList, error) {
	existingIPPoolList := &IPPoolList{}
	err := WebhookClient.List(context.Background(), existingIPPoolList, &client.ListOptions{Namespace: MetalLBNamespace})
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to get existing IPPool objects")
	}
	return existingIPPoolList, nil
}

func ipPoolListWithUpdate(existing *IPPoolList, toAdd *IPPool) *IPPoolList {
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
