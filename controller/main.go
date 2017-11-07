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
	"net"
	"reflect"

	"go.universe.tf/metallb/internal"

	"github.com/golang/glog"

	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
)

type controller struct {
	client *kubernetes.Clientset
	events record.EventRecorder
}

func (c *controller) UpdateBalancer(name string, svcRo *v1.Service, eps *v1.Endpoints) error {
	// Check service type
	// Set assigned-ip
	//   from LoadBalancerIP
	//   from IP allocation
	// Check policies/permissions
	// Program nodes
	//   Set ExternalIPs
	//   Set ExternalTrafficPolicy
	// Set status
	//   Update Ingress
	// Set advertise-after

	if svcRo.Spec.Type != "LoadBalancer" {
		return nil
	}

	// Making a copy unconditionally is a bit wasteful, since we don't
	// always need to update the service. But, making an unconditional
	// copy makes the code much easier to follow, and we have a GC for
	// a reason.
	svc := svcRo.DeepCopy()
	c.convergeBalancer(svc)
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

func (c *controller) convergeBalancer(svc *v1.Service) {
	lbIP := net.ParseIP(svc.Annotations[internal.AnnotationAssignedIP]).To4()
	if lbIP == nil {
		clearServiceState(svc)
	}

	if svc.Spec.LoadBalancerIP != "" && svc.Spec.LoadBalancerIP != svc.Annotations[internal.AnnotationAssignedIP] {
		clearServiceState(svc)
		lbIP = net.ParseIP(svc.Spec.LoadBalancerIP).To4()
		if lbIP == nil {
			c.events.Eventf(svc, v1.EventTypeWarning, "BadIP", "Invalid IPv4 address %q given as LoadBalancer", svc.Spec.LoadBalancerIP)
			return
		}
		svc.Annotations[internal.AnnotationAssignedIP] = svc.Spec.LoadBalancerIP
		c.events.Eventf(svc, v1.EventTypeNormal, "IPAllocated", "Using loadBalancerIP %q", svc.Spec.LoadBalancerIP)
	}

	if svc.Spec.LoadBalancerIP == "" && lbIP != nil {
		clearServiceState(svc)
		c.events.Eventf(svc, v1.EventTypeNormal, "UnassignIP", "IP %q unassigned", lbIP.String())
		lbIP = nil
	}

	// TODO: IP allocation
	if lbIP == nil {
		c.events.Event(svc, v1.EventTypeWarning, "ActionRequired", "Must manually set LoadBalancerIP, automatic allocation not supported yet")
		return
	}

	if err := c.checkPolicy(lbIP); err != nil {
		clearServiceState(svc)
		c.events.Eventf(svc, v1.EventTypeWarning, "PolicyError", "Policy check error: %s", err)
		return
	}

	if len(svc.Spec.ExternalIPs) != 1 || svc.Spec.ExternalIPs[0] != lbIP.String() || svc.Spec.ExternalTrafficPolicy != v1.ServiceExternalTrafficPolicyTypeLocal {
		svc.Spec.ExternalIPs = []string{lbIP.String()}
		svc.Spec.ExternalTrafficPolicy = v1.ServiceExternalTrafficPolicyTypeLocal
		c.events.Eventf(svc, v1.EventTypeNormal, "NodeConfig", "Configured node routing for %q", lbIP.String())
	}
	svc.Status.LoadBalancer.Ingress = []v1.LoadBalancerIngress{{IP: lbIP.String()}}
}

func clearServiceState(svc *v1.Service) {
	delete(svc.Annotations, internal.AnnotationAssignedIP)
	delete(svc.Annotations, internal.AnnotationAutoAllocated)
	svc.Spec.ExternalIPs = nil
	svc.Status.LoadBalancer = v1.LoadBalancerStatus{}
}

func (c *controller) checkPolicy(ip net.IP) error {
	// TODO: check policies
	return nil
}

func (c *controller) DeleteBalancer(name string) error { return nil }

func main() {
	kubeconfig := flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	master := flag.String("master", "", "master url")
	flag.Parse()

	client, events, err := internal.Client(*master, *kubeconfig, "metallb-controller")
	if err != nil {
		glog.Fatalf("Error getting k8s client: %s", err)
	}

	s := &controller{
		client: client,
		events: events,
	}

	glog.Fatal(internal.WatchServices(client, s))
}
