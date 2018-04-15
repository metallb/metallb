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
	"errors"
	"flag"
	"fmt"
	"os"
	"reflect"

	"go.universe.tf/metallb/internal/allocator"
	"go.universe.tf/metallb/internal/config"
	"go.universe.tf/metallb/internal/k8s"
	"go.universe.tf/metallb/internal/logging"
	"go.universe.tf/metallb/internal/version"

	"github.com/golang/glog"
	"k8s.io/api/core/v1"
)

// Service offers methods to mutate a Kubernetes service object.
type service interface {
	Update(svc *v1.Service) (*v1.Service, error)
	UpdateStatus(svc *v1.Service) error
	Infof(svc *v1.Service, desc, msg string, args ...interface{})
	Errorf(svc *v1.Service, desc, msg string, args ...interface{})
}

type controller struct {
	client service
	synced bool
	config *config.Config
	ips    *allocator.Allocator
}

func (c *controller) SetBalancer(name string, svcRo *v1.Service) error {
	glog.Infof("%s: start update", name)
	defer glog.Infof("%s: end update", name)

	if svcRo == nil {
		return c.deleteBalancer(name)
	}

	if c.config == nil {
		// Config hasn't been read, nothing we can do just yet.
		glog.Infof("%s: skipped, waiting for config", name)
		return nil
	}

	// Making a copy unconditionally is a bit wasteful, since we don't
	// always need to update the service. But, making an unconditional
	// copy makes the code much easier to follow, and we have a GC for
	// a reason.
	svc := svcRo.DeepCopy()
	if err := c.convergeBalancer(name, svc); err != nil {
		return err
	}
	if reflect.DeepEqual(svcRo, svc) {
		glog.Infof("%s: converged, no change", name)
		return nil
	}

	var err error
	if !(reflect.DeepEqual(svcRo.Annotations, svc.Annotations) && reflect.DeepEqual(svcRo.Spec, svc.Spec)) {
		svcRo, err = c.client.Update(svc)
		if err != nil {
			return fmt.Errorf("updating service %q: %s", name, err)
		}
		glog.Infof("%s: updated service", name)
	}
	if !reflect.DeepEqual(svcRo.Status, svc.Status) {
		var st v1.ServiceStatus
		st, svc = svc.Status, svcRo.DeepCopy()
		svc.Status = st
		if err = c.client.UpdateStatus(svc); err != nil {
			return fmt.Errorf("updating status on service %q: %s", name, err)
		}
		glog.Infof("%s: updated service status", name)
	}

	return nil
}

func (c *controller) deleteBalancer(name string) error {
	if c.ips.Unassign(name) {
		glog.Infof("%s: service deleted", name)
	}
	return nil
}

func (c *controller) SetConfig(cfg *config.Config) error {
	glog.Infof("Start config update")
	defer glog.Infof("End config update")

	if cfg == nil {
		glog.Errorf("No MetalLB configuration in cluster")
		return errors.New("configuration missing")
	}

	if err := c.ips.SetPools(cfg.Pools); err != nil {
		glog.Errorf("Applying new configuration failed: %s", err)
		return fmt.Errorf("configuration rejected: %s", err)
	}
	c.config = cfg
	return nil
}

func (c *controller) MarkSynced() {
	c.synced = true
	glog.Infof("Controller synced, can allocate IPs now")
}

func main() {
	_, err := logging.Init()
	if err != nil {
		fmt.Printf("failed to initialize logging: %s\n", err)
		os.Exit(1)
	}

	var (
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
		master     = flag.String("master", "", "master url")
		port       = flag.Int("port", 7472, "HTTP listening port for Prometheus metrics")
		config     = flag.String("config", "config", "Kubernetes ConfigMap containing MetalLB's configuration")
	)
	flag.Parse()

	glog.Infof("MetalLB controller %s", version.String())

	c := &controller{
		ips: allocator.New(),
	}

	client, err := k8s.New("metallb-controller", *master, *kubeconfig)
	if err != nil {
		glog.Fatalf("Error getting k8s client: %s", err)
	}
	client.HandleService(c.SetBalancer)
	client.HandleConfig(*config, c.SetConfig)
	client.HandleSynced(c.MarkSynced)

	c.client = client

	glog.Fatal(client.Run(*port))
}
