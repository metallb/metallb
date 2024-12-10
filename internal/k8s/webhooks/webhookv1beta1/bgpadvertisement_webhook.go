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
	admissionv1 "k8s.io/api/admission/v1"
	v1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const bgpAdvertisementWebhookPath = "/validate-metallb-io-v1beta1-bgpadvertisement"

func (v *BGPAdvertisementValidator) SetupWebhookWithManager(mgr ctrl.Manager) error {
	v.client = mgr.GetClient()
	v.decoder = admission.NewDecoder(mgr.GetScheme())

	mgr.GetWebhookServer().Register(
		bgpAdvertisementWebhookPath,
		&webhook.Admission{Handler: v})

	return nil
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-metallb-io-v1beta1-bgpadvertisement,mutating=false,failurePolicy=fail,groups=metallb.io,resources=bgpadvertisements,versions=v1beta1,name=bgpadvertisementvalidationwebhook.metallb.io,sideEffects=None,admissionReviewVersions=v1
type BGPAdvertisementValidator struct {
	ClusterResourceNamespace string

	client  client.Client
	decoder admission.Decoder
}

// Handle handled incoming admission requests for BGPAdvertisement objects.
func (v *BGPAdvertisementValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	var advertisement v1beta1.BGPAdvertisement
	var oldAdvertisement v1beta1.BGPAdvertisement
	if req.Operation == admissionv1.Delete {
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
	case admissionv1.Create:
		err := validateBGPAdvCreate(&advertisement)
		if err != nil {
			return admission.Denied(err.Error())
		}
	case admissionv1.Update:
		err := validateBGPAdvUpdate(&advertisement, &oldAdvertisement)
		if err != nil {
			return admission.Denied(err.Error())
		}
	case admissionv1.Delete:
		err := validateBGPAdvDelete(&advertisement)
		if err != nil {
			return admission.Denied(err.Error())
		}
	}
	return admission.Allowed("")
}

// validateBGPAdvCreate implements webhook.Validator so a webhook will be registered for BGPAdvertisement.
func validateBGPAdvCreate(bgpAdv *v1beta1.BGPAdvertisement) error {
	level.Debug(Logger).Log("webhook", "bgpadvertisement", "action", "create", "name", bgpAdv.Name, "namespace", bgpAdv.Namespace)

	if bgpAdv.Namespace != MetalLBNamespace {
		return fmt.Errorf("resource must be created in %s namespace", MetalLBNamespace)
	}

	existingBGPAdvList, err := getExistingBGPAdvs()
	if err != nil {
		return err
	}

	ipAddressPools, err := getExistingIPAddressPools()
	if err != nil {
		return err
	}

	nodes, err := getExistingNodes()
	if err != nil {
		return err
	}

	toValidate := bgpAdvListWithUpdate(existingBGPAdvList, bgpAdv)
	err = Validator.Validate(toValidate, ipAddressPools, nodes)
	if err != nil {
		level.Error(Logger).Log("webhook", "bgpadvertisement", "action", "create", "name", bgpAdv.Name, "namespace", bgpAdv.Namespace, "error", err)
		return err
	}
	return nil
}

// validateBGPAdvUpdate implements webhook.Validator so a webhook will be registered for BGPAdvertisement.
func validateBGPAdvUpdate(bgpAdv *v1beta1.BGPAdvertisement, _ *v1beta1.BGPAdvertisement) error {
	level.Debug(Logger).Log("webhook", "bgpadvertisement", "action", "update", "name", bgpAdv.Name, "namespace", bgpAdv.Namespace)

	bgpAdvs, err := getExistingBGPAdvs()
	if err != nil {
		return err
	}

	ipAddressPools, err := getExistingIPAddressPools()
	if err != nil {
		return err
	}

	nodes, err := getExistingNodes()
	if err != nil {
		return err
	}

	toValidate := bgpAdvListWithUpdate(bgpAdvs, bgpAdv)
	err = Validator.Validate(toValidate, ipAddressPools, nodes)
	if err != nil {
		level.Error(Logger).Log("webhook", "bgpadvertisement", "action", "create", "name", bgpAdv.Name, "namespace", bgpAdv.Namespace, "error", err)
		return err
	}
	return nil
}

// validateBGPAdvDelete implements webhook.Validator so a webhook will be registered for BGPAdvertisement.
func validateBGPAdvDelete(bgpAdv *v1beta1.BGPAdvertisement) error {
	return nil
}

var getExistingBGPAdvs = func() (*v1beta1.BGPAdvertisementList, error) {
	existingBGPAdvList := &v1beta1.BGPAdvertisementList{}
	err := WebhookClient.List(context.Background(), existingBGPAdvList, &client.ListOptions{Namespace: MetalLBNamespace})
	if err != nil {
		return nil, errors.Join(err, errors.New("failed to get existing BGPAdvertisement objects"))
	}
	return existingBGPAdvList, nil
}

var getExistingNodes = func() (*v1.NodeList, error) {
	existingNodeList := &v1.NodeList{}
	err := WebhookClient.List(context.Background(), existingNodeList)
	if err != nil {
		return nil, errors.Join(err, errors.New("failed to get existing Node objects"))
	}
	return existingNodeList, nil
}

func bgpAdvListWithUpdate(existing *v1beta1.BGPAdvertisementList, toAdd *v1beta1.BGPAdvertisement) *v1beta1.BGPAdvertisementList {
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
