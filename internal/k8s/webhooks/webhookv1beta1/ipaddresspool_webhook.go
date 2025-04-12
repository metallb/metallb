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
	"net"
	"net/http"
	"slices"

	"errors"

	"github.com/go-kit/log/level"
	"go.universe.tf/metallb/api/v1beta1"
	"go.universe.tf/metallb/internal/config"
	v1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const ipAddressPoolWebhookPath = "/validate-metallb-io-v1beta1-ipaddresspool"

func (v *IPAddressPoolValidator) SetupWebhookWithManager(mgr ctrl.Manager) error {
	v.client = mgr.GetClient()
	v.decoder = admission.NewDecoder(mgr.GetScheme())

	mgr.GetWebhookServer().Register(
		ipAddressPoolWebhookPath,
		&webhook.Admission{Handler: v})

	return nil
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-metallb-io-v1beta1-ipaddresspool,mutating=false,failurePolicy=fail,groups=metallb.io,resources=ipaddresspools,versions=v1beta1,name=ipaddresspoolvalidationwebhook.metallb.io,sideEffects=None,admissionReviewVersions=v1
type IPAddressPoolValidator struct {
	ClusterResourceNamespace string

	client  client.Client
	decoder admission.Decoder
}

// Handle handled incoming admission requests for IPAddressPool objects.
func (v *IPAddressPoolValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	var pool v1beta1.IPAddressPool
	var oldPool v1beta1.IPAddressPool
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
		err := validateIPAddressPoolCreate(&pool)
		if err != nil {
			return admission.Denied(err.Error())
		}
	case v1.Update:
		err := validateIPAddressPoolUpdate(&pool, &oldPool)
		if err != nil {
			return admission.Denied(err.Error())
		}
	case v1.Delete:
		err := validateIPAddressPoolDelete(&pool)
		if err != nil {
			return admission.Denied(err.Error())
		}
	}
	return admission.Allowed("")
}

// validateIPAddressPoolCreate implements webhook.Validator so a webhook will be registered for IPAddressPool.
func validateIPAddressPoolCreate(ipAddress *v1beta1.IPAddressPool) error {
	level.Debug(Logger).Log("webhook", "ipaddresspool", "action", "create", "name", ipAddress.Name, "namespace", ipAddress.Namespace)

	if ipAddress.Namespace != MetalLBNamespace {
		return fmt.Errorf("resource must be created in %s namespace", MetalLBNamespace)
	}
	existingIPAddressPoolList, err := getExistingIPAddressPools()
	if err != nil {
		return err
	}

	nodes, err := getExistingNodes()
	if err != nil {
		return err
	}

	toValidate := ipAddressListWithUpdate(existingIPAddressPoolList, ipAddress)
	err = Validator.Validate(toValidate, nodes)
	if err != nil {
		level.Error(Logger).Log("webhook", "ipAddress", "action", "create", "name", ipAddress.Name, "namespace", ipAddress.Namespace, "error", err)
		return err
	}

	err = validatePoolPerBGPAdv(ipAddress)
	if err != nil {
		return err
	}

	return nil
}

// validateIPAddressPoolUpdate implements webhook.Validator so a webhook will be registered for IPAddressPool.
func validateIPAddressPoolUpdate(ipAddress *v1beta1.IPAddressPool, _ *v1beta1.IPAddressPool) error {
	level.Debug(Logger).Log("webhook", "ipaddresspool", "action", "update", "name", ipAddress.Name, "namespace", ipAddress.Namespace)

	existingIPAddressPoolList, err := getExistingIPAddressPools()
	if err != nil {
		return err
	}

	nodes, err := getExistingNodes()
	if err != nil {
		return err
	}

	toValidate := ipAddressListWithUpdate(existingIPAddressPoolList, ipAddress)
	err = Validator.Validate(toValidate, nodes)
	if err != nil {
		level.Error(Logger).Log("webhook", "ipAddress", "action", "update", "name", ipAddress.Name, "namespace", ipAddress.Namespace, "error", err)
		return err
	}

	err = validatePoolPerBGPAdv(ipAddress)
	if err != nil {
		return err
	}

	return nil
}

// validateIPAddressPoolDelete implements webhook.Validator so a webhook will be registered for IPAddressPool.
func validateIPAddressPoolDelete(ipAddress *v1beta1.IPAddressPool) error {
	return nil
}

var getExistingIPAddressPools = func() (*v1beta1.IPAddressPoolList, error) {
	existingIPAddressPoolList := &v1beta1.IPAddressPoolList{}
	err := WebhookClient.List(context.Background(), existingIPAddressPoolList, &client.ListOptions{Namespace: MetalLBNamespace})
	if err != nil {
		return nil, errors.Join(err, errors.New("failed to get existing IPAddressPool objects"))
	}
	return existingIPAddressPoolList, nil
}

func ipAddressListWithUpdate(existing *v1beta1.IPAddressPoolList, toAdd *v1beta1.IPAddressPool) *v1beta1.IPAddressPoolList {
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

var getExistingBGPAdvertisements = func() (*v1beta1.BGPAdvertisementList, error) {
	existingBGPAdvertisementList := &v1beta1.BGPAdvertisementList{}
	err := WebhookClient.List(context.Background(), existingBGPAdvertisementList, &client.ListOptions{Namespace: MetalLBNamespace})
	if err != nil {
		return nil, errors.Join(err, errors.New("failed to get existing BGPAdvertisement objects"))
	}
	return existingBGPAdvertisementList, nil
}

func validatePoolPerBGPAdv(pool *v1beta1.IPAddressPool) error {
	existingBGPAdvs, err := getExistingBGPAdvertisements()
	if err != nil {
		return fmt.Errorf("failed to get existing BGPAdvertisements: %w", err)
	}

	poolCIDRsPerAddress := make(map[string][]*net.IPNet)
	for _, addr := range pool.Spec.Addresses {
		cidrs, err := config.ParseCIDR(addr)
		if err != nil {
			return fmt.Errorf("invalid address %q in pool %s: %w", addr, pool.Name, err)
		}
		poolCIDRsPerAddress[addr] = cidrs
	}

	poolLabels := labels.Set(pool.Labels)

	for _, adv := range existingBGPAdvs.Items {
		referencesPool := slices.Contains(adv.Spec.IPAddressPools, pool.Name)

		if !referencesPool {
			for _, selector := range adv.Spec.IPAddressPoolSelectors {
				labelSelector, err := metav1.LabelSelectorAsSelector(&selector)
				if err != nil {
					return fmt.Errorf("pool %s/%s validation failed: BGPAdvertisement %s/%s contains invalid ipAddressPoolSelector (%v): %w", pool.Namespace, pool.Name, adv.Namespace, adv.Name, selector, err)
				}
				if labelSelector.Matches(poolLabels) {
					referencesPool = true
					break
				}
			}
		}

		if !referencesPool {
			continue
		}

		for addr, cidrs := range poolCIDRsPerAddress {
			if len(cidrs) == 0 {
				continue
			}

			maxLength := *adv.Spec.AggregationLength
			if cidrs[0].IP.To4() == nil {
				maxLength = *adv.Spec.AggregationLengthV6
			}

			lowestPoolMask := config.LowestMask(cidrs)

			if int(maxLength) < lowestPoolMask {
				return fmt.Errorf("pool %s address %s (prefix %d) is incompatible with BGPAdvertisement %s in namespace %s (aggregation length %d)", pool.Name, addr, lowestPoolMask, adv.Name, adv.Namespace, maxLength)
			}
		}
	}

	return nil
}
