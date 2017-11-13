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
	"reflect"

	"go.universe.tf/metallb/internal"
	"go.universe.tf/metallb/internal/config"

	"github.com/golang/glog"

	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

type controller struct {
	client *kubernetes.Clientset
	events record.EventRecorder

	queue workqueue.RateLimitingInterface

	svcIndexer  cache.Indexer
	svcInformer cache.Controller
	cmIndexer   cache.Indexer
	cmInformer  cache.Controller

	config  *config.Config
	ipToSvc map[string]string
	svcToIP map[string]string
}

func (c *controller) UpdateBalancer(name string, svcRo *v1.Service) error {
	if svcRo.Spec.Type != "LoadBalancer" {
		return nil
	}

	if c.config == nil {
		// Config hasn't been read yet, nothing we can do just yet.
		glog.Infof("%q skipped, no config loaded", name)
		return nil
	}

	// Making a copy unconditionally is a bit wasteful, since we don't
	// always need to update the service. But, making an unconditional
	// copy makes the code much easier to follow, and we have a GC for
	// a reason.
	svc := svcRo.DeepCopy()
	c.convergeService(name, svc)
	if reflect.DeepEqual(svcRo, svc) {
		glog.Infof("%q converged, no change", name)
		return nil
	}

	var err error
	if !(reflect.DeepEqual(svcRo.Annotations, svc.Annotations) && reflect.DeepEqual(svcRo.Spec, svc.Spec)) {
		svcRo, err = c.client.CoreV1().Services(svc.Namespace).Update(svc)
		if err != nil {
			return fmt.Errorf("updating service: %s", err)
		}
		glog.Infof("updated service %q", name)
	}
	if !reflect.DeepEqual(svcRo.Status, svc.Status) {
		st, svc := svc.Status, svcRo.DeepCopy()
		svc.Status = st
		svc, err = c.client.CoreV1().Services(svcRo.Namespace).UpdateStatus(svc)
		if err != nil {
			return fmt.Errorf("updating status: %s", err)
		}
		glog.Infof("updated service status %q", name)
	}

	return nil
}

func (c *controller) DeleteBalancer(name string) error {
	ip, ok := c.svcToIP[name]
	if ok {
		delete(c.svcToIP, name)
		delete(c.ipToSvc, ip)
		glog.Infof("%q deleted", name)
	}
	return nil
}

func (c *controller) UpdateConfig(cm *v1.ConfigMap) error {
	var (
		cfg *config.Config
		err error
	)
	if cm != nil {
		cfg, err = config.Parse([]byte(cm.Data["config"]))
		if err != nil {
			c.events.Eventf(cm, v1.EventTypeWarning, "InvalidConfig", "%s", err)
			return nil
		}
	}

	c.config = cfg
	// Reprocess all services on config change
	glog.Infof("config changed, reconverging all services")
	for _, k := range c.svcIndexer.ListKeys() {
		c.queue.AddRateLimited(svcKey(k))
	}

	return nil
}

func main() {
	kubeconfig := flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	master := flag.String("master", "", "master url")
	flag.Parse()

	client, events, err := internal.Client(*master, *kubeconfig, "metallb-controller")
	if err != nil {
		glog.Fatalf("Error getting k8s client: %s", err)
	}

	c := &controller{
		client:  client,
		events:  events,
		ipToSvc: map[string]string{},
		svcToIP: map[string]string{},
	}

	glog.Fatal(c.watch())
}
