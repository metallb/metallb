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

const communityWebhookPath = "/validate-metallb-io-v1beta1-community"

func (v *CommunityValidator) SetupWebhookWithManager(mgr ctrl.Manager) error {
	v.client = mgr.GetClient()
	v.decoder = admission.NewDecoder(mgr.GetScheme())

	mgr.GetWebhookServer().Register(
		communityWebhookPath,
		&webhook.Admission{Handler: v})

	return nil
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-metallb-io-v1beta1-community,mutating=false,failurePolicy=fail,groups=metallb.io,resources=communities,versions=v1beta1,name=communityvalidationwebhook.metallb.io,sideEffects=None,admissionReviewVersions=v1
type CommunityValidator struct {
	ClusterResourceNamespace string

	client  client.Client
	decoder admission.Decoder
}

// Handle handled incoming admission requests for Community objects.
func (v *CommunityValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	var community v1beta1.Community
	var oldCommunity v1beta1.Community
	if req.Operation == v1.Delete {
		if err := v.decoder.DecodeRaw(req.OldObject, &community); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
	} else {
		if err := v.decoder.Decode(req, &community); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		if req.OldObject.Size() > 0 {
			if err := v.decoder.DecodeRaw(req.OldObject, &oldCommunity); err != nil {
				return admission.Errored(http.StatusBadRequest, err)
			}
		}
	}

	switch req.Operation {
	case v1.Create:
		err := validateCommunityCreate(&community)
		if err != nil {
			return admission.Denied(err.Error())
		}
	case v1.Update:
		err := validateCommunityUpdate(&community, &oldCommunity)
		if err != nil {
			return admission.Denied(err.Error())
		}
	case v1.Delete:
		err := validateCommunityDelete(&community)
		if err != nil {
			return admission.Denied(err.Error())
		}
	}
	return admission.Allowed("")
}

// validateCommunityCreate implements webhook.Validator so a webhook will be registered for Community.
func validateCommunityCreate(community *v1beta1.Community) error {
	level.Debug(Logger).Log("webhook", "community", "action", "create", "name", community.Name, "namespace", community.Namespace)

	if community.Namespace != MetalLBNamespace {
		return fmt.Errorf("resource must be created in %s namespace", MetalLBNamespace)
	}

	existingCommunityList, err := getExistingCommunities()
	if err != nil {
		return err
	}

	communityList := communitylistWithUpdate(existingCommunityList, community)
	err = Validator.Validate(communityList)
	if err != nil {
		level.Error(Logger).Log("webhook", "community", "action", "create", "name", community.Name, "namespace", community.Namespace, "error", err)
		return err
	}
	return nil
}

// validateCommunityUpdate implements webhook.Validator so a webhook will be registered for Community.
func validateCommunityUpdate(community *v1beta1.Community, _ *v1beta1.Community) error {
	level.Debug(Logger).Log("webhook", "community", "action", "update", "name", community.Name, "namespace", community.Namespace)

	existingCommunityList, err := getExistingCommunities()
	if err != nil {
		return err
	}

	communityList := communitylistWithUpdate(existingCommunityList, community)
	err = Validator.Validate(communityList)
	if err != nil {
		level.Error(Logger).Log("webhook", "community", "action", "update", "name", community.Name, "namespace", community.Namespace, "error", err)
		return err
	}
	return nil
}

// validateCommunityDelete implements webhook.Validator so a webhook will be registered for Community.
func validateCommunityDelete(community *v1beta1.Community) error {
	return nil
}

var getExistingCommunities = func() (*v1beta1.CommunityList, error) {
	existingCommunityList := &v1beta1.CommunityList{}
	err := WebhookClient.List(context.Background(), existingCommunityList, &client.ListOptions{Namespace: MetalLBNamespace})
	if err != nil {
		return nil, errors.Join(err, errors.New("failed to get existing Community objects"))
	}
	return existingCommunityList, nil
}

func communitylistWithUpdate(existing *v1beta1.CommunityList, toAdd *v1beta1.Community) *v1beta1.CommunityList {
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
