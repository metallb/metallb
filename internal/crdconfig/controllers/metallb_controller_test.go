/*
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	loadbalancerv1 "go.universe.tf/metallb/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("MetalLB Controller", func() {

	const timeout = time.Second * 30
	const interval = time.Second * 1

	BeforeEach(func() {
		// Add any setup steps that needs to be executed before each test
	})

	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
	})

	// Add Tests for OpenAPI validation (or additonal CRD features) specified in
	// your API definition.
	// Avoid adding tests for vanilla CRUD operations because they would
	// test Kubernetes API server, which isn't the goal here.
	Context("Creating MetalLB", func() {
		It("Should create successfully", func() {
			By("By creating a new Job")
			ctx, metalLB := context.Background(), &loadbalancerv1.MetalLB{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "loadbalancer.loadbalancer.operator.io/v1",
					Kind:       "MetalLB",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-metallb",
					Namespace: "test-metallb-namesapce",
				},
				Spec: loadbalancerv1.MetalLBSpec{
					AddressPools: []loadbalancerv1.AddressPool{{
						Name:     "test1",
						Protocol: "layer2",
						Addresses: []string{
							"1.1.1.1",
							"1.1.1.100",
						},
					}, {
						Name:     "test2",
						Protocol: "layer2",
						Addresses: []string{
							"2.2.2.1",
							"2.2.2.100",
						}},
					},
				},
			}

			// Create
			Expect(k8sClient.Create(ctx, metalLB)).Should(Succeed())
		})
	})
})
