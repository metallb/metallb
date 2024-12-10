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

package webhookv1beta2

import (
	"context"
	"fmt"
	"net/http"

	"errors"

	"github.com/go-kit/log/level"
	"go.universe.tf/metallb/api/v1beta2"
	v1 "k8s.io/api/admission/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const bgpPeerWebhookPath = "/validate-metallb-io-v1beta2-bgppeer"

func (v *BGPPeerValidator) SetupWebhookWithManager(mgr ctrl.Manager) error {
	v.client = mgr.GetClient()
	v.decoder = admission.NewDecoder(mgr.GetScheme())

	mgr.GetWebhookServer().Register(
		bgpPeerWebhookPath,
		&webhook.Admission{Handler: v})

	return nil
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-metallb-io-v1beta2-bgppeer,mutating=false,failurePolicy=fail,groups=metallb.io,resources=bgppeers,versions=v1beta2,name=bgppeersvalidationwebhook.metallb.io,sideEffects=None,admissionReviewVersions=v1
type BGPPeerValidator struct {
	ClusterResourceNamespace string

	client  client.Client
	decoder admission.Decoder
}

// Handle handled incoming admission requests for BGPPeer objects.
func (v *BGPPeerValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	var peer v1beta2.BGPPeer
	var oldPeer v1beta2.BGPPeer
	if req.Operation == v1.Delete {
		if err := v.decoder.DecodeRaw(req.OldObject, &peer); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
	} else {
		if err := v.decoder.Decode(req, &peer); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		if req.OldObject.Size() > 0 {
			if err := v.decoder.DecodeRaw(req.OldObject, &oldPeer); err != nil {
				return admission.Errored(http.StatusBadRequest, err)
			}
		}
	}
	switch req.Operation {
	case v1.Create:
		err := validatePeerCreate(&peer)
		if err != nil {
			return admission.Denied(err.Error())
		}
	case v1.Update:
		err := validatePeerUpdate(&peer, &oldPeer)
		if err != nil {
			return admission.Denied(err.Error())
		}
	case v1.Delete:
		err := validatePeerDelete(&peer)
		if err != nil {
			return admission.Denied(err.Error())
		}
	}
	return admission.Allowed("")
}

// validatePeerCreate implements webhook.Validator so a webhook will be registered for BGPPeer.
func validatePeerCreate(bgpPeer *v1beta2.BGPPeer) error {
	level.Debug(Logger).Log("webhook", "bgppeer", "action", "create", "name", bgpPeer.Name, "namespace", bgpPeer.Namespace)

	if bgpPeer.Namespace != MetalLBNamespace {
		return fmt.Errorf("resource must be created in %s namespace", MetalLBNamespace)
	}
	existingBGPPeers, err := GetExistingBGPPeers()
	if err != nil {
		return err
	}

	toValidate := bgpPeerListWithUpdate(existingBGPPeers, bgpPeer)
	err = Validator.Validate(toValidate)
	if err != nil {
		level.Error(Logger).Log("webhook", "bgppeer", "action", "create", "name", bgpPeer.Name, "namespace", bgpPeer.Namespace, "error", err)
		return err
	}
	return nil
}

// validatePeerUpdate implements webhook.Validator so a webhook will be registered for BGPPeer.
func validatePeerUpdate(bgpPeer *v1beta2.BGPPeer, _ *v1beta2.BGPPeer) error {
	level.Debug(Logger).Log("webhook", "bgppeer", "action", "update", "name", bgpPeer.Name, "namespace", bgpPeer.Namespace)

	existingBGPPeers, err := GetExistingBGPPeers()
	if err != nil {
		return err
	}

	toValidate := bgpPeerListWithUpdate(existingBGPPeers, bgpPeer)
	err = Validator.Validate(toValidate)
	if err != nil {
		level.Error(Logger).Log("webhook", "bgppeer", "action", "update", "name", bgpPeer.Name, "namespace", bgpPeer.Namespace, "error", err)
		return err
	}
	return nil
}

// validatePeerDelete implements webhook.Validator so a webhook will be registered for BGPPeer.
func validatePeerDelete(bgpPeer *v1beta2.BGPPeer) error {
	return nil
}

var GetExistingBGPPeers = func() (*v1beta2.BGPPeerList, error) {
	existingBGPPeerslList := &v1beta2.BGPPeerList{}
	err := WebhookClient.List(context.Background(), existingBGPPeerslList, &client.ListOptions{Namespace: MetalLBNamespace})
	if err != nil {
		return nil, errors.Join(err, errors.New("failed to get existing BGPPeer objects"))
	}
	return existingBGPPeerslList, nil
}

func bgpPeerListWithUpdate(existing *v1beta2.BGPPeerList, toAdd *v1beta2.BGPPeer) *v1beta2.BGPPeerList {
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
