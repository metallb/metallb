// SPDX-License-Identifier:Apache-2.0

package v1beta2

import (
	"github.com/go-kit/kit/log"

	"go.universe.tf/metallb/api/validate"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// log is for logging addresspool-webhook.
var (
	Logger           log.Logger
	WebhookClient    client.Reader
	Validator        validate.ClusterObjects
	MetalLBNamespace string
)
