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
	"go.universe.tf/metallb/internal/config"
	v1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
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
		warning, err := validatePeerCreate(&peer)
		if err != nil {
			return admission.Denied(err.Error())
		}
		if warning != "" {
			return admission.Allowed("").WithWarnings(warning)
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
func validatePeerCreate(bgpPeer *v1beta2.BGPPeer) (string, error) {
	level.Debug(Logger).Log("webhook", "bgppeer", "action", "create", "name", bgpPeer.Name, "namespace", bgpPeer.Namespace)

	if bgpPeer.Spec.DisableMP {
		return "disable mp is deprecated and has no effect since it's the default behavior now", nil
	}

	if bgpPeer.Namespace != MetalLBNamespace {
		return "", fmt.Errorf("resource must be created in %s namespace", MetalLBNamespace)
	}
	validateArgs, err := buildValidateArgs(bgpPeer)
	if err != nil {
		return "", err
	}
	err = Validator.Validate(validateArgs...)
	if err != nil {
		level.Error(Logger).Log("webhook", "bgppeer", "action", "create", "name", bgpPeer.Name, "namespace", bgpPeer.Namespace, "error", err)
		return "", err
	}
	return "", nil
}

// validatePeerUpdate implements webhook.Validator so a webhook will be registered for BGPPeer.
func validatePeerUpdate(bgpPeer *v1beta2.BGPPeer, _ *v1beta2.BGPPeer) error {
	level.Debug(Logger).Log("webhook", "bgppeer", "action", "update", "name", bgpPeer.Name, "namespace", bgpPeer.Namespace)

	validateArgs, err := buildValidateArgs(bgpPeer)
	if err != nil {
		return err
	}
	err = Validator.Validate(validateArgs...)
	if err != nil {
		level.Error(Logger).Log("webhook", "bgppeer", "action", "update", "name", bgpPeer.Name, "namespace", bgpPeer.Namespace, "error", err)
		return err
	}
	return nil
}

// buildValidateArgs fetches existing BGPPeers and, lazily, the Node list (only
// when duplicate peerIdentifiers exist) to assemble the argument slice for
// Validator.Validate.
func buildValidateArgs(bgpPeer *v1beta2.BGPPeer) ([]client.ObjectList, error) {
	existingBGPPeers, err := GetExistingBGPPeers()
	if err != nil {
		return nil, err
	}
	toValidate := bgpPeerListWithUpdate(existingBGPPeers, bgpPeer)
	args := []client.ObjectList{toValidate}
	if hasDuplicatePeerIdentifiers(toValidate) {
		existingNodes, err := GetExistingNodes()
		if err != nil {
			return nil, err
		}
		args = append(args, existingNodes)
	}
	return args, nil
}

// hasDuplicatePeerIdentifiers returns true when toValidate contains at least
// two peers that share the same peerIdentifier (address-or-interface + VRF).
func hasDuplicatePeerIdentifiers(peers *v1beta2.BGPPeerList) bool {
	seen := make(map[string]struct{})
	for _, p := range peers.Items {
		key := config.PeerIdentifier(p.Spec)
		if _, exists := seen[key]; exists {
			return true
		}
		seen[key] = struct{}{}
	}
	return false
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

var GetExistingNodes = func() (*corev1.NodeList, error) {
	nodeList := &corev1.NodeList{}
	err := WebhookClient.List(context.Background(), nodeList)
	if err != nil {
		return nil, errors.Join(err, errors.New("failed to get existing Node objects"))
	}
	return nodeList, nil
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
