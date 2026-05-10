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

const ospfInstanceWebhookPath = "/validate-metallb-io-v1beta1-ospfinstance"

func (v *OSPFInstanceValidator) SetupWebhookWithManager(mgr ctrl.Manager) error {
	v.client = mgr.GetClient()
	v.decoder = admission.NewDecoder(mgr.GetScheme())
	mgr.GetWebhookServer().Register(ospfInstanceWebhookPath, &webhook.Admission{Handler: v})
	return nil
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-metallb-io-v1beta1-ospfinstance,mutating=false,failurePolicy=fail,groups=metallb.io,resources=ospfinstances,versions=v1beta1,name=ospfinstancevalidationwebhook.metallb.io,sideEffects=None,admissionReviewVersions=v1
type OSPFInstanceValidator struct {
	ClusterResourceNamespace string
	client                   client.Client
	decoder                  admission.Decoder
}

func (v *OSPFInstanceValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	var instance v1beta1.OSPFInstance
	var oldInstance v1beta1.OSPFInstance
	if req.Operation == admissionv1.Delete {
		if err := v.decoder.DecodeRaw(req.OldObject, &instance); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
	} else {
		if err := v.decoder.Decode(req, &instance); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		if req.OldObject.Size() > 0 {
			if err := v.decoder.DecodeRaw(req.OldObject, &oldInstance); err != nil {
				return admission.Errored(http.StatusBadRequest, err)
			}
		}
	}
	switch req.Operation {
	case admissionv1.Create:
		if err := validateOSPFInstanceCreate(&instance); err != nil {
			return admission.Denied(err.Error())
		}
	case admissionv1.Update:
		if err := validateOSPFInstanceUpdate(&instance, &oldInstance); err != nil {
			return admission.Denied(err.Error())
		}
	}
	return admission.Allowed("")
}

func validateOSPFInstanceCreate(inst *v1beta1.OSPFInstance) error {
	level.Debug(Logger).Log("webhook", "ospfinstance", "action", "create", "name", inst.Name)
	if inst.Namespace != MetalLBNamespace {
		return fmt.Errorf("resource must be created in %s namespace", MetalLBNamespace)
	}
	existing, err := getExistingOSPFInstances()
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
	toValidate := ospfInstanceListWithUpdate(existing, inst)
	if err := Validator.Validate(toValidate, ipPools, nodes); err != nil {
		level.Error(Logger).Log("webhook", "ospfinstance", "action", "create", "name", inst.Name, "error", err)
		return err
	}
	return nil
}

func validateOSPFInstanceUpdate(inst *v1beta1.OSPFInstance, _ *v1beta1.OSPFInstance) error {
	level.Debug(Logger).Log("webhook", "ospfinstance", "action", "update", "name", inst.Name)
	existing, err := getExistingOSPFInstances()
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
	toValidate := ospfInstanceListWithUpdate(existing, inst)
	if err := Validator.Validate(toValidate, ipPools, nodes); err != nil {
		level.Error(Logger).Log("webhook", "ospfinstance", "action", "update", "name", inst.Name, "error", err)
		return err
	}
	return nil
}

var getExistingOSPFInstances = func() (*v1beta1.OSPFInstanceList, error) {
	list := &v1beta1.OSPFInstanceList{}
	err := WebhookClient.List(context.Background(), list, &client.ListOptions{Namespace: MetalLBNamespace})
	if err != nil {
		return nil, errors.Join(err, errors.New("failed to get existing OSPFInstance objects"))
	}
	return list, nil
}

func ospfInstanceListWithUpdate(existing *v1beta1.OSPFInstanceList, toAdd *v1beta1.OSPFInstance) *v1beta1.OSPFInstanceList {
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
