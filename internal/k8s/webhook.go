// SPDX-License-Identifier:Apache-2.0

package k8s

import (
	"fmt"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/open-policy-agent/cert-controller/pkg/rotator"
	"go.universe.tf/metallb/internal/config"
	"go.universe.tf/metallb/internal/k8s/webhooks/webhookv1beta1"
	"go.universe.tf/metallb/internal/k8s/webhooks/webhookv1beta2"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/conversion"
)

func enableCertRotation(notifyFinished chan struct{}, cfg *Config, mgr manager.Manager) error {
	webhooks := []rotator.WebhookInfo{
		{
			Name: validatingWebhookName,
			Type: rotator.Validating,
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
			Name:      cfg.WebhookSecretName,
		},
		CertDir:        cfg.CertDir,
		CAName:         caName,
		CAOrganization: caOrganization,
		DNSName:        fmt.Sprintf("%s.%s.svc", cfg.CertServiceName, cfg.Namespace),
		IsReady:        notifyFinished,
		Webhooks:       webhooks,
		FieldOwner:     "MetalLB",
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
	webhookv1beta1.MetalLBNamespace = namespace
	webhookv1beta2.MetalLBNamespace = namespace
	webhookv1beta1.Logger = logger
	webhookv1beta2.Logger = logger
	webhookv1beta1.WebhookClient = mgr.GetAPIReader()
	webhookv1beta2.WebhookClient = mgr.GetAPIReader()
	webhookv1beta1.Validator = config.NewValidator(validate)
	webhookv1beta2.Validator = config.NewValidator(validate)

	if err := (&webhookv1beta1.IPAddressPoolValidator{}).SetupWebhookWithManager(mgr); err != nil {
		level.Error(logger).Log("op", "startup", "error", err, "msg", "unable to create webhook", "webhook", "IPAddressPool")
		return err
	}

	if err := (&webhookv1beta2.BGPPeerValidator{}).SetupWebhookWithManager(mgr); err != nil {
		level.Error(logger).Log("op", "startup", "error", err, "msg", "unable to create webhook", "webhook", "BGPPeer v1beta2")
		return err
	}

	if err := (&webhookv1beta1.BGPAdvertisementValidator{}).SetupWebhookWithManager(mgr); err != nil {
		level.Error(logger).Log("op", "startup", "error", err, "msg", "unable to create webhook", "webhook", "BGPAdvertisement")
		return err
	}

	if err := (&webhookv1beta1.L2AdvertisementValidator{}).SetupWebhookWithManager(mgr); err != nil {
		level.Error(logger).Log("op", "startup", "error", err, "msg", "unable to create webhook", "webhook", "L2Advertisement")
		return err
	}

	if err := (&webhookv1beta1.CommunityValidator{}).SetupWebhookWithManager(mgr); err != nil {
		level.Error(logger).Log("op", "startup", "error", err, "msg", "unable to create webhook", "webhook", "Community")
		return err
	}

	if err := (&webhookv1beta1.BFDProfileValidator{}).SetupWebhookWithManager(mgr); err != nil {
		level.Error(logger).Log("op", "startup", "error", err, "msg", "unable to create webhook", "webhook", "BFDProfile")
		return err
	}

	// Register conversion webhook manually since we are not directly handling the types.
	mgr.GetWebhookServer().Register("/convert", conversion.NewWebhookHandler(mgr.GetScheme()))

	return nil
}
