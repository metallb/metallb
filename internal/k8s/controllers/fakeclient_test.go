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
	"fmt"

	v1beta1 "go.universe.tf/metallb/api/v1beta1"
	v1beta2 "go.universe.tf/metallb/api/v1beta2"
	"go.universe.tf/metallb/internal/config"
	"go.universe.tf/metallb/internal/k8s/epslices"
	corev1 "k8s.io/api/core/v1"
	discovery "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newFakeClient(initObjects []client.Object) (client.WithWatch, error) {
	scheme := runtime.NewScheme()
	if err := v1beta1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("v1beta1: add to scheme failed: %v", err)
	}

	if err := v1beta2.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("v1beta2: add to scheme failed: %v", err)
	}

	if err := corev1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("corev1: add to scheme failed: %v", err)
	}

	if err := discovery.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("discovery: add to scheme failed: %v", err)
	}

	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(initObjects...).
		WithIndex(&discovery.EndpointSlice{}, epslices.SlicesServiceIndexName, func(o client.Object) []string {
			res, err := epslices.SlicesServiceIndex(o)
			if err != nil {
				return []string{}
			}
			return res
		}).
		Build(), nil
}

func objectsFromResources(r config.ClusterResources) []client.Object {
	objects := make([]client.Object, 0)
	for _, pool := range r.Pools {
		objects = append(objects, pool.DeepCopy())
	}

	for _, secret := range r.PasswordSecrets {
		objects = append(objects, secret.DeepCopy())
	}

	for _, peer := range r.Peers {
		objects = append(objects, peer.DeepCopy())
	}

	for _, bfdProfile := range r.BFDProfiles {
		objects = append(objects, bfdProfile.DeepCopy())
	}

	for _, bgpAdv := range r.BGPAdvs {
		objects = append(objects, bgpAdv.DeepCopy())
	}

	for _, l2Adv := range r.L2Advs {
		objects = append(objects, l2Adv.DeepCopy())
	}

	for _, community := range r.Communities {
		objects = append(objects, community.DeepCopy())
	}

	return objects
}
