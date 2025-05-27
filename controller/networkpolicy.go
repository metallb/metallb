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
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
)

func checkNetworkPolicies(namespace string) (string, error) {
	c, err := kubernetes.NewForConfig(ctrl.GetConfigOrDie())
	if err != nil {
		return "", fmt.Errorf("creating Kubernetes client: %s", err)
	}

	networkPolicies, err := c.NetworkingV1().NetworkPolicies(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("cannot list kubernetes networkpolicies client: %s", err)
	}

	if len(networkPolicies.Items) == 0 {
		return "", nil
	}

	return "NetworkPolicies detected - these may interfere with MetalLB traffic flow", nil
}
