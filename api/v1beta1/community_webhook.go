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
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func (community *Community) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(community).
		Complete()
}

//+kubebuilder:webhook:verbs=create;update,path=/validate-metallb-io-v1beta1-community,mutating=false,failurePolicy=fail,groups=metallb.io,resources=communities,versions=v1beta1,name=communityvalidationwebhook.metallb.io,sideEffects=None,admissionReviewVersions=v1

var _ webhook.Validator = &Community{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for Community.
func (community *Community) ValidateCreate() (admission.Warnings, error) {
	level.Debug(Logger).Log("webhook", "community", "action", "create", "name", community.Name, "namespace", community.Namespace)

	if community.Namespace != MetalLBNamespace {
		return nil, fmt.Errorf("resource must be created in %s namespace", MetalLBNamespace)
	}

	existingCommunityList, err := getExistingCommunities()
	if err != nil {
		return nil, err
	}

	communityList := communitylistWithUpdate(existingCommunityList, community)
	err = Validator.Validate(communityList)
	if err != nil {
		level.Error(Logger).Log("webhook", "community", "action", "create", "name", community.Name, "namespace", community.Namespace, "error", err)
		return nil, err
	}
	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for Community.
func (community *Community) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	level.Debug(Logger).Log("webhook", "community", "action", "update", "name", community.Name, "namespace", community.Namespace)

	existingCommunityList, err := getExistingCommunities()
	if err != nil {
		return nil, err
	}

	communityList := communitylistWithUpdate(existingCommunityList, community)
	err = Validator.Validate(communityList)
	if err != nil {
		level.Error(Logger).Log("webhook", "community", "action", "update", "name", community.Name, "namespace", community.Namespace, "error", err)
		return nil, err
	}
	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for Community.
func (community *Community) ValidateDelete() (admission.Warnings, error) {
	return nil, nil
}

var getExistingCommunities = func() (*CommunityList, error) {
	existingCommunityList := &CommunityList{}
	err := WebhookClient.List(context.Background(), existingCommunityList, &client.ListOptions{Namespace: MetalLBNamespace})
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to get existing Community objects")
	}
	return existingCommunityList, nil
}

func communitylistWithUpdate(existing *CommunityList, toAdd *Community) *CommunityList {
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
