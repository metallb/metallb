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
	"fmt"

	"github.com/go-kit/log/level"
	"go.universe.tf/metallb/api/v1beta2"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func (bfdProfile *BFDProfile) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(bfdProfile).
		Complete()
}

//+kubebuilder:webhook:verbs=delete,path=/validate-metallb-io-v1beta1-bfdprofile,mutating=false,failurePolicy=fail,groups=metallb.io,resources=bfdprofiles,versions=v1beta1,name=bfdprofilevalidationwebhook.metallb.io,sideEffects=None,admissionReviewVersions=v1

var _ webhook.Validator = &BFDProfile{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for BFDProfile.
func (bfdProfile *BFDProfile) ValidateCreate() error {
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for BFDProfile.
func (bfdProfile *BFDProfile) ValidateUpdate(old runtime.Object) error {
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for BFDProfile.
func (bfdProfile *BFDProfile) ValidateDelete() error {
	level.Debug(Logger).Log("webhook", "bfdprofile", "action", "delete", "name", bfdProfile.Name, "namespace", bfdProfile.Namespace)

	existingBGPPeers, err := v1beta2.GetExistingBGPPeers()
	if err != nil {
		return err
	}

	for _, peer := range existingBGPPeers.Items {
		if bfdProfile.Name == peer.Spec.BFDProfile {
			return fmt.Errorf("Failed to delete BFDProfile %s, used by BGPPeer %s", bfdProfile.Name, peer.Name)
		}
	}
	return nil
}
