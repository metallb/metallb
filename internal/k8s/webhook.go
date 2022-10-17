// SPDX-License-Identifier:Apache-2.0

package k8s

import (
	"fmt"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/open-policy-agent/cert-controller/pkg/rotator"
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	metallbv1beta2 "go.universe.tf/metallb/api/v1beta2"
	"go.universe.tf/metallb/internal/config"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func enableCertRotation(notifyFinished chan struct{}, cfg *Config, mgr manager.Manager) error {
	webhooks := []rotator.WebhookInfo{
		{
			Name: validatingWebhookName,
			Type: rotator.Validating,
		},
		{
			Name: addresspoolConvertingWebhookCRD,
			Type: rotator.CRDConversion,
		},
		{
			Name: bgppeerConvertingWebhookCRD,
			Type: rotator.CRDConversion,
		},
	}

	level.Info(cfg.Logger).Log("op", "startup", "action", "setting up cert rotation")
	err := rotator.AddRotator(mgr, &rotator.CertRotator{
		SecretKey: types.NamespacedName{
			Namespace: cfg.Namespace,
			Name:      webhookSecretName,
		},
		CertDir:        cfg.CertDir,
		CAName:         caName,
		CAOrganization: caOrganization,
		DNSName:        fmt.Sprintf("%s.%s.svc", cfg.CertServiceName, cfg.Namespace),
		IsReady:        notifyFinished,
		Webhooks:       webhooks,
	})
	if err != nil {
		level.Error(cfg.Logger).Log("error", err, "unable to set up", "cert rotation")
		return err
	}
	return nil
}

func enableWebhook(mgr manager.Manager, validate config.Validate, namespace string, logger log.Logger) error {
	level.Info(logger).Log("op", "startup", "action", "webhooks enabled")

	// Used by all the webhooks
	metallbv1beta1.MetalLBNamespace = namespace
	metallbv1beta2.MetalLBNamespace = namespace
	metallbv1beta1.Logger = logger
	metallbv1beta2.Logger = logger
	metallbv1beta1.WebhookClient = mgr.GetAPIReader()
	metallbv1beta2.WebhookClient = mgr.GetAPIReader()
	metallbv1beta1.Validator = config.NewValidator(validate)
	metallbv1beta2.Validator = config.NewValidator(validate)

	if err := (&metallbv1beta1.AddressPool{}).SetupWebhookWithManager(mgr); err != nil {
		level.Error(logger).Log("op", "startup", "error", err, "msg", "unable to create webhook", "webhook", "AddressPool")
		return err
	}

	if err := (&metallbv1beta1.IPAddressPool{}).SetupWebhookWithManager(mgr); err != nil {
		level.Error(logger).Log("op", "startup", "error", err, "msg", "unable to create webhook", "webhook", "IPAddressPool")
		return err
	}

	if err := (&metallbv1beta2.BGPPeer{}).SetupWebhookWithManager(mgr); err != nil {
		level.Error(logger).Log("op", "startup", "error", err, "msg", "unable to create webhook", "webhook", "BGPPeer v1beta2")
		return err
	}

	if err := (&metallbv1beta1.BGPAdvertisement{}).SetupWebhookWithManager(mgr); err != nil {
		level.Error(logger).Log("op", "startup", "error", err, "msg", "unable to create webhook", "webhook", "BGPAdvertisement")
		return err
	}

	if err := (&metallbv1beta1.L2Advertisement{}).SetupWebhookWithManager(mgr); err != nil {
		level.Error(logger).Log("op", "startup", "error", err, "msg", "unable to create webhook", "webhook", "L2Advertisement")
		return err
	}

	if err := (&metallbv1beta1.Community{}).SetupWebhookWithManager(mgr); err != nil {
		level.Error(logger).Log("op", "startup", "error", err, "msg", "unable to create webhook", "webhook", "Community")
		return err
	}

	if err := (&metallbv1beta1.BFDProfile{}).SetupWebhookWithManager(mgr); err != nil {
		level.Error(logger).Log("op", "startup", "error", err, "msg", "unable to create webhook", "webhook", "BFDProfile")
		return err
	}

	return nil
}
