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
	"sync"

	"go.universe.tf/metallb/internal"

	"github.com/golang/glog"

	"k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
)

type speaker struct {
	nodeName string
	events   record.EventRecorder

	sync.Mutex
	svcs map[string]bool
}

func (s *speaker) UpdateBalancer(name string, svc *v1.Service, eps *v1.Endpoints) error {
	if svc.Spec.Type != "LoadBalancer" {
		return s.deleteBalancer(name, "not a LoadBalancer")
	}
	if _, ok := svc.Annotations[internal.AnnotationAssignedIP]; !ok {
		return s.deleteBalancer(name, "no IP assigned yet")
	}

	pods := map[string]bool{}
	for _, subset := range eps.Subsets {
		for _, a := range subset.Addresses {
			if a.NodeName != nil && *a.NodeName == s.nodeName {
				pods[a.IP] = true
			}
		}
	}
	for _, subset := range eps.Subsets {
		for _, a := range subset.NotReadyAddresses {
			delete(pods, a.IP)
		}
	}

	if len(pods) == 0 {
		return s.deleteBalancer(name, "no serving endpoints on this node")
	}

	s.Lock()
	defer s.Unlock()
	glog.Infof("UPDATE service %q, %v -> %s (weight %d)", name, svc.Annotations[internal.AnnotationAssignedIP], s.nodeName, len(pods))
	s.svcs[name] = true
	return nil
}

func (s *speaker) DeleteBalancer(name string) error {
	return s.deleteBalancer(name, "service deleted")
}

func (s *speaker) deleteBalancer(name, reason string) error {
	s.Lock()
	defer s.Unlock()

	if s.svcs[name] {
		glog.Infof("DELETE service %q (%s)", name, reason)
	}
	delete(s.svcs, name)
	return nil
}

func main() {
	kubeconfig := flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	master := flag.String("master", "", "master url")
	nodeName := flag.String("node-name", "", "name of the local node")
	flag.Parse()

	client, events, err := internal.Client(*master, *kubeconfig, "metallb-bgp-speaker")
	if err != nil {
		glog.Fatalf("Error getting k8s client: %s", err)
	}

	s := &speaker{
		nodeName: *nodeName,
		events:   events,
		svcs:     map[string]bool{},
	}

	glog.Fatal(internal.WatchServices(client, s))
}
