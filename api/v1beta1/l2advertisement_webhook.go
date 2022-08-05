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
)

func (l2Adv *L2Advertisement) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(l2Adv).
		Complete()
}

//+kubebuilder:webhook:verbs=create;update,path=/validate-metallb-io-v1beta1-l2advertisement,mutating=false,failurePolicy=fail,groups=metallb.io,resources=l2advertisements,versions=v1beta1,name=l2advertisementvalidationwebhook.metallb.io,sideEffects=None,admissionReviewVersions=v1

var _ webhook.Validator = &L2Advertisement{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for L2Advertisement.
func (l2Adv *L2Advertisement) ValidateCreate() error {
	level.Debug(Logger).Log("webhook", "l2advertisement", "action", "create", "name", l2Adv.Name, "namespace", l2Adv.Namespace)

	if l2Adv.Namespace != MetalLBNamespace {
		return fmt.Errorf("resource must be created in %s namespace", MetalLBNamespace)
	}

	existingL2AdvList, err := getExistingL2Advs()
	if err != nil {
		return err
	}

	addressPools, err := getExistingAddressPools()
	if err != nil {
		return err
	}

	ipAddressPools, err := getExistingIPAddressPools()
	if err != nil {
		return err
	}

	toValidate := l2AdvListWithUpdate(existingL2AdvList, l2Adv)
	err = Validator.Validate(toValidate, addressPools, ipAddressPools)
	if err != nil {
		level.Error(Logger).Log("webhook", "l2advertisement", "action", "create", "name", l2Adv.Name, "namespace", l2Adv.Namespace, "error", err)
		return err
	}
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for L2Advertisement.
func (l2Adv *L2Advertisement) ValidateUpdate(old runtime.Object) error {
	level.Debug(Logger).Log("webhook", "l2advertisement", "action", "update", "name", l2Adv.Name, "namespace", l2Adv.Namespace)

	l2Advs, err := getExistingL2Advs()
	if err != nil {
		return err
	}

	addressPools, err := getExistingAddressPools()
	if err != nil {
		return err
	}

	ipAddressPools, err := getExistingIPAddressPools()
	if err != nil {
		return err
	}

	toValidate := l2AdvListWithUpdate(l2Advs, l2Adv)
	err = Validator.Validate(toValidate, addressPools, ipAddressPools)
	if err != nil {
		level.Error(Logger).Log("webhook", "l2advertisement", "action", "create", "name", l2Adv.Name, "namespace", l2Adv.Namespace, "error", err)
		return err
	}
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for L2Advertisement.
func (l2Adv *L2Advertisement) ValidateDelete() error {
	return nil
}

var getExistingL2Advs = func() (*L2AdvertisementList, error) {
	existingL2AdvList := &L2AdvertisementList{}
	err := WebhookClient.List(context.Background(), existingL2AdvList, &client.ListOptions{Namespace: MetalLBNamespace})
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to get existing L2Advertisement objects")
	}
	return existingL2AdvList, nil
}

func l2AdvListWithUpdate(existing *L2AdvertisementList, toAdd *L2Advertisement) *L2AdvertisementList {
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
