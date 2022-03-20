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
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func (bgpAdv *BGPAdvertisement) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(bgpAdv).
		Complete()
}

//+kubebuilder:webhook:verbs=create;update,path=/validate-metallb-io-v1beta1-bgpadvertisement,mutating=false,failurePolicy=fail,groups=metallb.io,resources=bgpadvertisements,versions=v1beta1,name=bgpadvertisementvalidationwebhook.metallb.io,sideEffects=None,admissionReviewVersions=v1

var _ webhook.Validator = &BGPAdvertisement{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for BGPAdvertisement.
func (bgpAdv *BGPAdvertisement) ValidateCreate() error {
	level.Debug(Logger).Log("webhook", "bgpadvertisement", "action", "create", "name", bgpAdv.Name, "namespace", bgpAdv.Namespace)

	existingBGPAdvList, err := getExistingBGPAdvs()
	if err != nil {
		return err
	}

	addressPools, err := getExistingAddressPools()
	if err != nil {
		return err
	}

	ipPools, err := getExistingIPPools()
	if err != nil {
		return err
	}

	toValidate := bgpAdvListWithUpdate(existingBGPAdvList, bgpAdv)
	err = Validator.Validate(toValidate, addressPools, ipPools)
	if err != nil {
		level.Error(Logger).Log("webhook", "bgpadvertisement", "action", "create", "name", bgpAdv.Name, "namespace", bgpAdv.Namespace, "error", err)
		return err
	}
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for BGPAdvertisement.
func (bgpAdv *BGPAdvertisement) ValidateUpdate(old runtime.Object) error {
	level.Debug(Logger).Log("webhook", "bgpadvertisement", "action", "update", "name", bgpAdv.Name, "namespace", bgpAdv.Namespace)

	bgpAdvs, err := getExistingBGPAdvs()
	if err != nil {
		return err
	}

	addressPools, err := getExistingAddressPools()
	if err != nil {
		return err
	}

	ipPools, err := getExistingIPPools()
	if err != nil {
		return err
	}

	toValidate := bgpAdvListWithUpdate(bgpAdvs, bgpAdv)
	err = Validator.Validate(toValidate, addressPools, ipPools)
	if err != nil {
		level.Error(Logger).Log("webhook", "bgpadvertisement", "action", "create", "name", bgpAdv.Name, "namespace", bgpAdv.Namespace, "error", err)
		return err
	}
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for BGPAdvertisement.
func (bgpAdv *BGPAdvertisement) ValidateDelete() error {
	return nil
}

var getExistingBGPAdvs = func() (*BGPAdvertisementList, error) {
	existingBGPAdvList := &BGPAdvertisementList{}
	err := WebhookClient.List(context.Background(), existingBGPAdvList)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to get existing BGPAdvertisement objects")
	}
	return existingBGPAdvList, nil
}

func bgpAdvListWithUpdate(existing *BGPAdvertisementList, toAdd *BGPAdvertisement) *BGPAdvertisementList {
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
