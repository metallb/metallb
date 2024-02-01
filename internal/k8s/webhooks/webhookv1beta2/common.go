// SPDX-License-Identifier:Apache-2.0

package webhookv1beta2

import (
	"github.com/go-kit/log"

	"go.universe.tf/metallb/internal/k8s/webhooks/validate"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	Logger           log.Logger
	WebhookClient    client.Reader
	Validator        validate.ClusterObjects
	MetalLBNamespace string
)
