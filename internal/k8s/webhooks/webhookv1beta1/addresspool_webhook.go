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

package webhookv1beta1

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-kit/log/level"
	"github.com/pkg/errors"
	"go.universe.tf/metallb/api/v1beta1"
	v1 "k8s.io/api/admission/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const addressPoolWebhookPath = "/validate-metallb-io-v1beta1-addresspool"

func (v *AddressPoolValidator) SetupWebhookWithManager(mgr ctrl.Manager) error {
	v.client = mgr.GetClient()
	v.decoder = admission.NewDecoder(mgr.GetScheme())

	mgr.GetWebhookServer().Register(
		addressPoolWebhookPath,
		&webhook.Admission{Handler: v})

	return nil
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-metallb-io-v1beta1-addresspool,mutating=false,failurePolicy=fail,groups=metallb.io,resources=addresspools,versions=v1beta1,name=addresspoolvalidationwebhook.metallb.io,sideEffects=None,admissionReviewVersions=v1
type AddressPoolValidator struct {
	ClusterResourceNamespace string

	client  client.Client
	decoder *admission.Decoder
}

// Handle handled incoming admission requests for AddressPool objects.
func (v *AddressPoolValidator) Handle(ctx context.Context, req admission.Request) (resp admission.Response) {
	var pool v1beta1.AddressPool
	var oldPool v1beta1.AddressPool
	if req.Operation == v1.Delete {
		if err := v.decoder.DecodeRaw(req.OldObject, &pool); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
	} else {
		if err := v.decoder.Decode(req, &pool); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		if req.OldObject.Size() > 0 {
			if err := v.decoder.DecodeRaw(req.OldObject, &oldPool); err != nil {
				return admission.Errored(http.StatusBadRequest, err)
			}
		}
	}

	switch req.Operation {
	case v1.Create:
		err := validateAddressPoolCreate(&pool)
		if err != nil {
			return admission.Denied(err.Error())
		}
	case v1.Update:
		err := validateAddressPoolUpdate(&pool, &oldPool)
		if err != nil {
			return admission.Denied(err.Error())
		}
	case v1.Delete:
		err := validateAddressPoolDelete(&pool)
		if err != nil {
			return admission.Denied(err.Error())
		}
	}
	return admission.Allowed("")
}

// validateAddressPoolCreate implements webhook.Validator so a webhook will be registered for AddressPool.
func validateAddressPoolCreate(addressPool *v1beta1.AddressPool) error {
	level.Debug(Logger).Log("webhook", "addressPool", "action", "create", "name", addressPool.Name, "namespace", addressPool.Namespace)

	if addressPool.Namespace != MetalLBNamespace {
		return fmt.Errorf("resource must be created in %s namespace", MetalLBNamespace)
	}

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

// validateAddressPoolUpdate implements webhook.Validator so a webhook will be registered for AddressPool.
func validateAddressPoolUpdate(addressPool *v1beta1.AddressPool, _ *v1beta1.AddressPool) error {
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

// validateAddressPoolDelete implements webhook.Validator so a webhook will be registered for AddressPool.
func validateAddressPoolDelete(addressPool *v1beta1.AddressPool) error {
	return nil
}

var getExistingAddressPools = func() (*v1beta1.AddressPoolList, error) {
	existingAddressPoolList := &v1beta1.AddressPoolList{}
	err := WebhookClient.List(context.Background(), existingAddressPoolList, &client.ListOptions{Namespace: MetalLBNamespace})
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to get existing AddressPool objects")
	}
	return existingAddressPoolList, nil
}

func listWithUpdate(existing *v1beta1.AddressPoolList, toAdd *v1beta1.AddressPool) *v1beta1.AddressPoolList {
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
