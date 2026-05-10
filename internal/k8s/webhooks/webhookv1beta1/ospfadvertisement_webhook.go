// SPDX-License-Identifier:Apache-2.0

package webhookv1beta1

import (
	"context"
	"fmt"
	"net/http"

	"errors"

	"github.com/go-kit/log/level"
	"go.universe.tf/metallb/api/v1beta1"
	admissionv1 "k8s.io/api/admission/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const ospfAdvertisementWebhookPath = "/validate-metallb-io-v1beta1-ospfadvertisement"

func (v *OSPFAdvertisementValidator) SetupWebhookWithManager(mgr ctrl.Manager) error {
	v.client = mgr.GetClient()
	v.decoder = admission.NewDecoder(mgr.GetScheme())
	mgr.GetWebhookServer().Register(ospfAdvertisementWebhookPath, &webhook.Admission{Handler: v})
	return nil
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-metallb-io-v1beta1-ospfadvertisement,mutating=false,failurePolicy=fail,groups=metallb.io,resources=ospfadvertisements,versions=v1beta1,name=ospfadvertisementvalidationwebhook.metallb.io,sideEffects=None,admissionReviewVersions=v1
type OSPFAdvertisementValidator struct {
	ClusterResourceNamespace string
	client                   client.Client
	decoder                  admission.Decoder
}

func (v *OSPFAdvertisementValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	var adv v1beta1.OSPFAdvertisement
	var oldAdv v1beta1.OSPFAdvertisement
	if req.Operation == admissionv1.Delete {
		if err := v.decoder.DecodeRaw(req.OldObject, &adv); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
	} else {
		if err := v.decoder.Decode(req, &adv); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		if req.OldObject.Size() > 0 {
			if err := v.decoder.DecodeRaw(req.OldObject, &oldAdv); err != nil {
				return admission.Errored(http.StatusBadRequest, err)
			}
		}
	}
	switch req.Operation {
	case admissionv1.Create:
		if err := validateOSPFAdvCreate(&adv); err != nil {
			return admission.Denied(err.Error())
		}
	case admissionv1.Update:
		if err := validateOSPFAdvUpdate(&adv, &oldAdv); err != nil {
			return admission.Denied(err.Error())
		}
	}
	return admission.Allowed("")
}

func validateOSPFAdvCreate(adv *v1beta1.OSPFAdvertisement) error {
	level.Debug(Logger).Log("webhook", "ospfadvertisement", "action", "create", "name", adv.Name)
	if adv.Namespace != MetalLBNamespace {
		return fmt.Errorf("resource must be created in %s namespace", MetalLBNamespace)
	}
	existing, err := getExistingOSPFAdvs()
	if err != nil {
		return err
	}
	ipPools, err := getExistingIPAddressPools()
	if err != nil {
		return err
	}
	nodes, err := getExistingNodes()
	if err != nil {
		return err
	}
	toValidate := ospfAdvListWithUpdate(existing, adv)
	if err := Validator.Validate(toValidate, ipPools, nodes); err != nil {
		level.Error(Logger).Log("webhook", "ospfadvertisement", "action", "create", "name", adv.Name, "error", err)
		return err
	}
	return nil
}

func validateOSPFAdvUpdate(adv *v1beta1.OSPFAdvertisement, _ *v1beta1.OSPFAdvertisement) error {
	level.Debug(Logger).Log("webhook", "ospfadvertisement", "action", "update", "name", adv.Name)
	existing, err := getExistingOSPFAdvs()
	if err != nil {
		return err
	}
	ipPools, err := getExistingIPAddressPools()
	if err != nil {
		return err
	}
	nodes, err := getExistingNodes()
	if err != nil {
		return err
	}
	toValidate := ospfAdvListWithUpdate(existing, adv)
	if err := Validator.Validate(toValidate, ipPools, nodes); err != nil {
		level.Error(Logger).Log("webhook", "ospfadvertisement", "action", "update", "name", adv.Name, "error", err)
		return err
	}
	return nil
}

var getExistingOSPFAdvs = func() (*v1beta1.OSPFAdvertisementList, error) {
	list := &v1beta1.OSPFAdvertisementList{}
	err := WebhookClient.List(context.Background(), list, &client.ListOptions{Namespace: MetalLBNamespace})
	if err != nil {
		return nil, errors.Join(err, errors.New("failed to get existing OSPFAdvertisement objects"))
	}
	return list, nil
}

func ospfAdvListWithUpdate(existing *v1beta1.OSPFAdvertisementList, toAdd *v1beta1.OSPFAdvertisement) *v1beta1.OSPFAdvertisementList {
	res := existing.DeepCopy()
	for i, item := range res.Items {
		if item.Name == toAdd.Name {
			res.Items[i] = *toAdd.DeepCopy()
			return res
		}
	}
	res.Items = append(res.Items, *toAdd.DeepCopy())
	return res
}
