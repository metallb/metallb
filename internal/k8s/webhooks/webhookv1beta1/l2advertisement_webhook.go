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

	"errors"

	"github.com/go-kit/log/level"
	"go.universe.tf/metallb/api/v1beta1"
	v1 "k8s.io/api/admission/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const l2AdvertisementWebhookPath = "/validate-metallb-io-v1beta1-l2advertisement"

func (v *L2AdvertisementValidator) SetupWebhookWithManager(mgr ctrl.Manager) error {
	v.client = mgr.GetClient()
	v.decoder = admission.NewDecoder(mgr.GetScheme())

	mgr.GetWebhookServer().Register(
		l2AdvertisementWebhookPath,
		&webhook.Admission{Handler: v})

	return nil
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-metallb-io-v1beta1-v1beta1.L2Advertisement,mutating=false,failurePolicy=fail,groups=metallb.io,resources=v1beta1.L2Advertisements,versions=v1beta1,name=v1beta1.L2Advertisementvalidationwebhook.metallb.io,sideEffects=None,admissionReviewVersions=v1
type L2AdvertisementValidator struct {
	ClusterResourceNamespace string

	client  client.Client
	decoder admission.Decoder
}

// Handle handled incoming admission requests for L2Advertisement objects.
func (v *L2AdvertisementValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	var advertisement v1beta1.L2Advertisement
	var oldAdvertisement v1beta1.L2Advertisement
	if req.Operation == v1.Delete {
		if err := v.decoder.DecodeRaw(req.OldObject, &advertisement); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
	} else {
		if err := v.decoder.Decode(req, &advertisement); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		if req.OldObject.Size() > 0 {
			if err := v.decoder.DecodeRaw(req.OldObject, &oldAdvertisement); err != nil {
				return admission.Errored(http.StatusBadRequest, err)
			}
		}
	}

	switch req.Operation {
	case v1.Create:
		err := validateL2AdvCreate(&advertisement)
		if err != nil {
			return admission.Denied(err.Error())
		}
	case v1.Update:
		err := validateL2AdvUpdate(&advertisement, &oldAdvertisement)
		if err != nil {
			return admission.Denied(err.Error())
		}
	case v1.Delete:
		err := validateL2AdvDelete(&advertisement)
		if err != nil {
			return admission.Denied(err.Error())
		}
	}
	return admission.Allowed("")
}

// validateL2AdvCreate implements webhook.Validator so a webhook will be registered for v1beta1.L2Advertisement.
func validateL2AdvCreate(l2Adv *v1beta1.L2Advertisement) error {
	level.Debug(Logger).Log("webhook", "v1beta1.L2Advertisement", "action", "create", "name", l2Adv.Name, "namespace", l2Adv.Namespace)

	if l2Adv.Namespace != MetalLBNamespace {
		return fmt.Errorf("resource must be created in %s namespace", MetalLBNamespace)
	}

	existingL2AdvList, err := getExistingL2Advs()
	if err != nil {
		return err
	}

	ipAddressPools, err := getExistingIPAddressPools()
	if err != nil {
		return err
	}

	toValidate := l2AdvListWithUpdate(existingL2AdvList, l2Adv)
	err = Validator.Validate(toValidate, ipAddressPools)
	if err != nil {
		level.Error(Logger).Log("webhook", "v1beta1.L2Advertisement", "action", "create", "name", l2Adv.Name, "namespace", l2Adv.Namespace, "error", err)
		return err
	}
	return nil
}

// validateL2AdvUpdate implements webhook.Validator so a webhook will be registered for v1beta1.L2Advertisement.
func validateL2AdvUpdate(l2Adv *v1beta1.L2Advertisement, _ *v1beta1.L2Advertisement) error {
	level.Debug(Logger).Log("webhook", "v1beta1.L2Advertisement", "action", "update", "name", l2Adv.Name, "namespace", l2Adv.Namespace)

	l2Advs, err := getExistingL2Advs()
	if err != nil {
		return err
	}

	ipAddressPools, err := getExistingIPAddressPools()
	if err != nil {
		return err
	}

	toValidate := l2AdvListWithUpdate(l2Advs, l2Adv)
	err = Validator.Validate(toValidate, ipAddressPools)
	if err != nil {
		level.Error(Logger).Log("webhook", "v1beta1.L2Advertisement", "action", "create", "name", l2Adv.Name, "namespace", l2Adv.Namespace, "error", err)
		return err
	}
	return nil
}

// validateL2AdvDelete implements webhook.Validator so a webhook will be registered for v1beta1.L2Advertisement.
func validateL2AdvDelete(l2Adv *v1beta1.L2Advertisement) error {
	return nil
}

var getExistingL2Advs = func() (*v1beta1.L2AdvertisementList, error) {
	existingL2AdvList := &v1beta1.L2AdvertisementList{}
	err := WebhookClient.List(context.Background(), existingL2AdvList, &client.ListOptions{Namespace: MetalLBNamespace})
	if err != nil {
		return nil, errors.Join(err, errors.New("failed to get existing v1beta1.L2Advertisement objects"))
	}
	return existingL2AdvList, nil
}

func l2AdvListWithUpdate(existing *v1beta1.L2AdvertisementList, toAdd *v1beta1.L2Advertisement) *v1beta1.L2AdvertisementList {
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
