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
	"go.universe.tf/metallb/api/v1beta1"
	"go.universe.tf/metallb/internal/k8s/webhooks/webhookv1beta2"
	v1 "k8s.io/api/admission/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const bfdProfileWebhookPath = "/validate-metallb-io-v1beta1-bfdprofile"

func (v *BFDProfileValidator) SetupWebhookWithManager(mgr ctrl.Manager) error {
	v.client = mgr.GetClient()
	v.decoder = admission.NewDecoder(mgr.GetScheme())

	mgr.GetWebhookServer().Register(
		bfdProfileWebhookPath,
		&webhook.Admission{Handler: v})

	return nil
}

// +kubebuilder:webhook:verbs=create;delete,path=/validate-metallb-io-v1beta1-bfdprofile,mutating=false,failurePolicy=fail,groups=metallb.io,resources=bfdprofiles,versions=v1beta1,name=bfdprofilevalidationwebhook.metallb.io,sideEffects=None,admissionReviewVersions=v1
type BFDProfileValidator struct {
	ClusterResourceNamespace string

	client  client.Client
	decoder admission.Decoder
}

// Handle handled incoming admission requests for BFDProfile objects.
func (v *BFDProfileValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	var bfdProfile v1beta1.BFDProfile
	var oldBFDProfile v1beta1.BFDProfile
	if req.Operation == v1.Delete {
		if err := v.decoder.DecodeRaw(req.OldObject, &bfdProfile); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
	} else {
		if err := v.decoder.Decode(req, &bfdProfile); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		if req.OldObject.Size() > 0 {
			if err := v.decoder.DecodeRaw(req.OldObject, &oldBFDProfile); err != nil {
				return admission.Errored(http.StatusBadRequest, err)
			}
		}
	}

	switch req.Operation {
	case v1.Create:
		err := validateBFDCreate(&bfdProfile)
		if err != nil {
			return admission.Denied(err.Error())
		}
	case v1.Update:
		err := validateBFDUpdate(&bfdProfile, &oldBFDProfile)
		if err != nil {
			return admission.Denied(err.Error())
		}
	case v1.Delete:
		err := validateBFDDelete(&bfdProfile)
		if err != nil {
			return admission.Denied(err.Error())
		}
	}
	return admission.Allowed("")
}

// validateBFDCreate implements webhook.Validator so a webhook will be registered for BFDProfile.
func validateBFDCreate(bfdProfile *v1beta1.BFDProfile) error {
	level.Debug(Logger).Log("webhook", "bfdProfile", "action", "create", "name", bfdProfile.Name, "namespace", bfdProfile.Namespace)

	if bfdProfile.Namespace != MetalLBNamespace {
		return fmt.Errorf("resource must be created in %s namespace", MetalLBNamespace)
	}

	return nil
}

// validateBFDUpdate implements webhook.Validator so a webhook will be registered for BFDProfile.
func validateBFDUpdate(bfdProfile *v1beta1.BFDProfile, _ *v1beta1.BFDProfile) error {
	return nil
}

// validateBFDDelete implements webhook.Validator so a webhook will be registered for BFDProfile.
func validateBFDDelete(bfdProfile *v1beta1.BFDProfile) error {
	level.Debug(Logger).Log("webhook", "bfdprofile", "action", "delete", "name", bfdProfile.Name, "namespace", bfdProfile.Namespace)

	existingBGPPeers, err := webhookv1beta2.GetExistingBGPPeers()
	if err != nil {
		return err
	}

	for _, peer := range existingBGPPeers.Items {
		if bfdProfile.Name == peer.Spec.BFDProfile {
			return fmt.Errorf("failed to delete BFDProfile %s, used by BGPPeer %s", bfdProfile.Name, peer.Name)
		}
	}
	return nil
}
