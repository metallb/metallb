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

package v1beta2

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

func (bgpPeer *BGPPeer) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(bgpPeer).
		Complete()
}

//+kubebuilder:webhook:verbs=create;update,path=/validate-metallb-io-v1beta2-bgppeer,mutating=false,failurePolicy=fail,groups=metallb.io,resources=bgppeers,versions=v1beta2,name=bgppeersvalidationwebhook.metallb.io,sideEffects=None,admissionReviewVersions=v1

var _ webhook.Validator = &BGPPeer{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for BGPPeer.
func (bgpPeer *BGPPeer) ValidateCreate() error {
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

// ValidateUpdate implements webhook.Validator so a webhook will be registered for AddressPool.
func (bgpPeer *BGPPeer) ValidateUpdate(old runtime.Object) error {
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

// ValidateDelete implements webhook.Validator so a webhook will be registered for AddressPool.
func (bgpPeer *BGPPeer) ValidateDelete() error {
	return nil
}

var GetExistingBGPPeers = func() (*BGPPeerList, error) {
	existingBGPPeerslList := &BGPPeerList{}
	err := WebhookClient.List(context.Background(), existingBGPPeerslList, &client.ListOptions{Namespace: MetalLBNamespace})
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to get existing BGPPeer objects")
	}
	return existingBGPPeerslList, nil
}

func bgpPeerListWithUpdate(existing *BGPPeerList, toAdd *BGPPeer) *BGPPeerList {
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
