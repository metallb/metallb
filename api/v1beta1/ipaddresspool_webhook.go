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

	"github.com/go-kit/log/level"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func (ipAddress *IPAddressPool) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(ipAddress).
		Complete()
}

//+kubebuilder:webhook:verbs=create;update,path=/validate-metallb-io-v1beta1-ipaddresspool,mutating=false,failurePolicy=fail,groups=metallb.io,resources=ipaddresspools,versions=v1beta1,name=ipaddresspoolvalidationwebhook.metallb.io,sideEffects=None,admissionReviewVersions=v1

var _ webhook.Validator = &IPAddressPool{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for IPAddressPool.
func (ipAddress *IPAddressPool) ValidateCreate() (admission.Warnings, error) {
	level.Debug(Logger).Log("webhook", "ipaddresspool", "action", "create", "name", ipAddress.Name, "namespace", ipAddress.Namespace)

	if ipAddress.Namespace != MetalLBNamespace {
		return nil, fmt.Errorf("resource must be created in %s namespace", MetalLBNamespace)
	}
	existingIPAddressPoolList, err := getExistingIPAddressPools()
	if err != nil {
		return nil, err
	}

	existingAddressPoolList, err := getExistingAddressPools()
	if err != nil {
		return nil, err
	}
	toValidate := ipAddressListWithUpdate(existingIPAddressPoolList, ipAddress)
	err = Validator.Validate(existingAddressPoolList, toValidate)
	if err != nil {
		level.Error(Logger).Log("webhook", "ipAddress", "action", "create", "name", ipAddress.Name, "namespace", ipAddress.Namespace, "error", err)
		return nil, err
	}
	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for IPAddressPool.
func (ipAddress *IPAddressPool) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	level.Debug(Logger).Log("webhook", "ipaddresspool", "action", "update", "name", ipAddress.Name, "namespace", ipAddress.Namespace)
	existingAddressPoolList, err := getExistingAddressPools()
	if err != nil {
		return nil, err
	}

	existingIPAddressPoolList, err := getExistingIPAddressPools()
	if err != nil {
		return nil, err
	}

	toValidate := ipAddressListWithUpdate(existingIPAddressPoolList, ipAddress)
	err = Validator.Validate(existingAddressPoolList, toValidate)
	if err != nil {
		level.Error(Logger).Log("webhook", "ipAddress", "action", "update", "name", ipAddress.Name, "namespace", ipAddress.Namespace, "error", err)
		return nil, err
	}
	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for IPAddressPool.
func (ipAddress *IPAddressPool) ValidateDelete() (admission.Warnings, error) {
	return nil, nil
}

var getExistingIPAddressPools = func() (*IPAddressPoolList, error) {
	existingIPAddressPoolList := &IPAddressPoolList{}
	err := WebhookClient.List(context.Background(), existingIPAddressPoolList, &client.ListOptions{Namespace: MetalLBNamespace})
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to get existing IPAddressPool objects")
	}
	return existingIPAddressPoolList, nil
}

func ipAddressListWithUpdate(existing *IPAddressPoolList, toAdd *IPAddressPool) *IPAddressPoolList {
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
