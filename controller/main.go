// Copyright 2017 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"flag"
	"fmt"
	"os"
	"reflect"

	"go.universe.tf/metallb/internal/allocator"
	"go.universe.tf/metallb/internal/config"
	"go.universe.tf/metallb/internal/k8s"
	"go.universe.tf/metallb/internal/k8s/controllers"
	"go.universe.tf/metallb/internal/k8s/epslices"
	"go.universe.tf/metallb/internal/logging"
	"go.universe.tf/metallb/internal/version"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	v1 "k8s.io/api/core/v1"
)

// Service offers methods to mutate a Kubernetes service object.
type service interface {
	UpdateStatus(svc *v1.Service) error
	Infof(svc *v1.Service, desc, msg string, args ...interface{})
	Errorf(svc *v1.Service, desc, msg string, args ...interface{})
}

type controller struct {
	client service
	pools  *config.Pools
	ips    *allocator.Allocator
}

func (c *controller) SetBalancer(l log.Logger, name string, svcRo *v1.Service, _ epslices.EpsOrSlices) controllers.SyncState {
	level.Debug(l).Log("event", "startUpdate", "msg", "start of service update")
	defer level.Debug(l).Log("event", "endUpdate", "msg", "end of service update")

	if svcRo == nil {
		if c.isServiceAllocated(name) {
			c.ips.Unassign(name)
			level.Info(l).Log("event", "serviceDeleted", "msg", "service deleted")
			// There might be other LBs stuck waiting for an IP, so when
			// we delete a balancer we should reprocess all of them to
			// check for newly feasible balancers.
			return controllers.SyncStateReprocessAll
		}
		return controllers.SyncStateSuccess
	}

	if c.pools == nil || c.pools.ByName == nil {
		// Config hasn't been read, nothing we can do just yet.
		level.Debug(l).Log("event", "noConfig", "msg", "not processing, still waiting for config")
		return controllers.SyncStateSuccess
	}

	// Making a copy unconditionally is a bit wasteful, since we don't
	// always need to update the service. But, making an unconditional
	// copy makes the code much easier to follow, and we have a GC for
	// a reason.
	svc := svcRo.DeepCopy()
	syncStateRes := controllers.SyncStateSuccess

	prevIPs := c.ips.IPs(name)

	if c.convergeBalancer(l, name, svc) != nil {
		syncStateRes = controllers.SyncStateErrorNoRetry
	}

	if reflect.DeepEqual(svcRo, svc) {
		level.Debug(l).Log("event", "noChange", "msg", "service converged, no change")
		return syncStateRes
	}

	if len(prevIPs) != 0 && !c.isServiceAllocated(name) {
		// Only reprocess all if the previous IP(s) are still contained within a pool.
		if c.ips.PoolForIP(prevIPs) != nil {
			// convergeBalancer may deallocate our service and this means it did it.
			// if the service was deallocated, it may have left room
			// for another one, so we reprocess
			level.Info(l).Log("event", "serviceUpdated", "msg", "removed loadbalancer from service, services will be reprocessed")
			syncStateRes = controllers.SyncStateReprocessAll
		}
	}

	toWrite := svcRo.DeepCopy()
	if !reflect.DeepEqual(svcRo.Status, svc.Status) {
		toWrite.Status = svc.Status
	}

	if !reflect.DeepEqual(svcRo.Annotations, svc.Annotations) {
		toWrite.Annotations = svc.Annotations
	}

	if !reflect.DeepEqual(toWrite, svcRo) {
		if err := c.client.UpdateStatus(svc); err != nil {
			level.Error(l).Log("op", "updateServiceStatus", "error", err, "msg", "failed to update service")
			return controllers.SyncStateError
		}
		level.Info(l).Log("event", "serviceUpdated", "msg", "updated service object")
		return syncStateRes
	}

	level.Info(l).Log("event", "serviceUpdated", "msg", "service is not updated")
	return syncStateRes
}

func (c *controller) SetPools(l log.Logger, pools *config.Pools) controllers.SyncState {
	level.Debug(l).Log("event", "startUpdate", "msg", "start of config update")
	defer level.Debug(l).Log("event", "endUpdate", "msg", "end of config update")

	if pools == nil || pools.ByName == nil {
		level.Error(l).Log("op", "setConfig", "error", "no MetalLB configuration in cluster", "msg", "configuration is missing, MetalLB will not function")
		return controllers.SyncStateErrorNoRetry
	}

	c.ips.SetPools(pools)
	c.pools = pools

	return controllers.SyncStateReprocessAll
}

func main() {
	var (
		port                = flag.Int("port", 7472, "HTTP listening port for Prometheus metrics")
		namespace           = flag.String("namespace", os.Getenv("METALLB_NAMESPACE"), "config / memberlist secret namespace")
		mlSecret            = flag.String("ml-secret-name", os.Getenv("METALLB_ML_SECRET_NAME"), "name of the memberlist secret to create")
		deployName          = flag.String("deployment", os.Getenv("METALLB_DEPLOYMENT"), "name of the MetalLB controller Deployment")
		logLevel            = flag.String("log-level", "info", fmt.Sprintf("log level. must be one of: [%s]", logging.Levels.String()))
		disableEpSlices     = flag.Bool("disable-epslices", false, "Disable the usage of EndpointSlices and default to Endpoints instead of relying on the autodiscovery mechanism")
		enablePprof         = flag.Bool("enable-pprof", false, "Enable pprof profiling")
		disableCertRotation = flag.Bool("disable-cert-rotation", false, "disable automatic generation and rotation of webhook TLS certificates/keys")
		certDir             = flag.String("cert-dir", "/tmp/k8s-webhook-server/serving-certs", "The directory where certs are stored")
		certServiceName     = flag.String("cert-service-name", "webhook-service", "The service name used to generate the TLS cert's hostname")
		loadBalancerClass   = flag.String("lb-class", "", "load balancer class. When enabled, metallb will handle only services whose spec.loadBalancerClass matches the given lb class")
		webhookMode         = flag.String("webhook-mode", "enabled", "webhook mode: can be enabled, disabled or only webhook if we want the controller to act as webhook endpoint only")
		webhookSecretName   = flag.String("webhook-secret", "webhook-server-cert", "webhook secret: the name of webhook secret, default is webhook-server-cert")
		webhookHTTP2        = flag.Bool("webhook-http2", false, "enables http2 for the webhook endpoint")
	)
	flag.Parse()

	logger, err := logging.Init(*logLevel)
	if err != nil {
		fmt.Printf("failed to initialize logging: %s\n", err)
		os.Exit(1)
	}

	level.Info(logger).Log("version", version.Version(), "commit", version.CommitHash(), "branch", version.Branch(), "goversion", version.GoString(), "msg", "MetalLB controller starting "+version.String())

	if *namespace == "" {
		bs, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
		if err != nil {
			level.Error(logger).Log("op", "startup", "msg", "Unable to get namespace from pod service account data, please specify --namespace or METALLB_NAMESPACE", "error", err)
			os.Exit(1)
		}
		*namespace = string(bs)
	}

	c := &controller{
		ips: allocator.New(),
	}

	bgpType, present := os.LookupEnv("METALLB_BGP_TYPE")
	if !present {
		bgpType = "native"
	}

	validation := config.ValidationFor(bgpType)

	cfg := &k8s.Config{
		ProcessName:     "metallb-controller",
		MetricsPort:     *port,
		EnablePprof:     *enablePprof,
		Logger:          logger,
		DisableEpSlices: *disableEpSlices,

		Namespace: *namespace,
		Listener: k8s.Listener{
			ServiceChanged: c.SetBalancer,
			PoolChanged:    c.SetPools,
		},
		ValidateConfig:      validation,
		EnableWebhook:       true,
		WebhookWithHTTP2:    *webhookHTTP2,
		DisableCertRotation: *disableCertRotation,
		WebhookSecretName:   *webhookSecretName,
		CertDir:             *certDir,
		CertServiceName:     *certServiceName,
		LoadBalancerClass:   *loadBalancerClass,
	}
	switch *webhookMode {
	case "enabled":
		cfg.EnableWebhook = true
	case "disabled":
		cfg.EnableWebhook = false
	case "onlywebhook":
		cfg.EnableWebhook = true
		cfg.Listener = k8s.Listener{}
	default:
		level.Error(logger).Log("op", "startup", "error", "invalid webhookmode value", "value", *webhookMode)
		os.Exit(1)
	}

	client, err := k8s.New(cfg)
	if err != nil {
		level.Error(logger).Log("op", "startup", "error", err, "msg", "failed to create k8s client")
		os.Exit(1)
	}

	if *mlSecret != "" {
		err = client.CreateMlSecret(*namespace, *deployName, *mlSecret)
		if err != nil {
			level.Error(logger).Log("op", "startup", "error", err, "msg", "failed to create memberlist secret")
			os.Exit(1)
		}
	}

	c.client = client
	if err := client.Run(nil); err != nil {
		level.Error(logger).Log("op", "startup", "error", err, "msg", "failed to run k8s client")
		os.Exit(1)
	}
}
