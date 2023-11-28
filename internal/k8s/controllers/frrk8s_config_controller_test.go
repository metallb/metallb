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
	"path/filepath"
	"testing"
	"time"

	"github.com/go-kit/log"
	frrv1beta1 "github.com/metallb/frr-k8s/api/v1beta1"
	. "github.com/onsi/gomega"
	v1beta1 "go.universe.tf/metallb/api/v1beta1"
	v1beta2 "go.universe.tf/metallb/api/v1beta2"
	frrk8s "go.universe.tf/metallb/internal/bgp/frrk8s"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

func TestFRRK8sReconcileReconciler_SetupWithManager(t *testing.T) {
	g := NewGomegaWithT(t)
	testEnv := &envtest.Environment{
		// we should keep the frrk8s up to date. However, for the purpouse of testing the controller
		// it is fine to keep this version and change one stable field (the router's asn).
		CRDDirectoryPaths:     []string{filepath.Join("../../..", "config", "crd", "bases"), "./testdata"},
		ErrorIfCRDPathMissing: true,
		Scheme:                scheme,
	}
	cfg, err := testEnv.Start()
	g.Expect(err).ToNot(HaveOccurred())
	defer func() {
		err = testEnv.Stop()
		g.Expect(err).ToNot(HaveOccurred())
	}()
	err = v1beta1.AddToScheme(k8sscheme.Scheme)
	g.Expect(err).ToNot(HaveOccurred())
	err = v1beta2.AddToScheme(k8sscheme.Scheme)
	g.Expect(err).ToNot(HaveOccurred())
	err = frrv1beta1.AddToScheme(k8sscheme.Scheme)
	g.Expect(err).ToNot(HaveOccurred())
	m, err := manager.New(cfg, manager.Options{Metrics: metricsserver.Options{BindAddress: "0"}})
	g.Expect(err).ToNot(HaveOccurred())

	r := &FRRK8sReconciler{
		Client:    m.GetClient(),
		Logger:    log.NewNopLogger(),
		Scheme:    scheme,
		NodeName:  "node",
		Namespace: testNamespace,
	}

	err = r.SetupWithManager(m)
	g.Expect(err).ToNot(HaveOccurred())
	ctx := context.Background()
	go func() {
		err = m.Start(ctx)
		g.Expect(err).ToNot(HaveOccurred())
	}()

	ns := corev1.Namespace{}
	ns.Name = testNamespace
	err = m.GetClient().Create(ctx, &ns)
	g.Expect(err).ToNot(HaveOccurred())

	frrConfig := frrv1beta1.FRRConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      frrk8s.ConfigName("node"),
			Namespace: testNamespace,
		},
		Spec: frrv1beta1.FRRConfigurationSpec{
			BGP: frrv1beta1.BGPConfig{
				Routers: []frrv1beta1.Router{
					{
						ASN: 25,
					},
				},
			},
		},
	}

	// Create a config when desired is empty
	err = m.GetClient().Create(ctx, &frrConfig)
	g.Expect(err).ToNot(HaveOccurred())

	g.Eventually(func() bool {
		newConfig := frrv1beta1.FRRConfiguration{}
		err := m.GetClient().Get(context.TODO(), client.ObjectKey{Name: frrConfig.Name, Namespace: testNamespace}, &newConfig)
		return apierrors.IsNotFound(err)
	}, 5*time.Second, 200*time.Millisecond).Should(BeTrue())

	r.UpdateConfig(frrConfig)
	g.Eventually(func() uint32 {
		newConfig := frrv1beta1.FRRConfiguration{}
		err := m.GetClient().Get(context.TODO(), client.ObjectKey{Name: frrConfig.Name, Namespace: testNamespace}, &newConfig)
		if err != nil {
			return 0
		}
		return newConfig.Spec.BGP.Routers[0].ASN
	}, 5*time.Second, 200*time.Millisecond).Should(Equal(uint32(25)))

	// Notifying that the configuration changed
	frrConfig.Spec.BGP.Routers[0].ASN = 26
	r.UpdateConfig(frrConfig)

	g.Eventually(func() uint32 {
		newConfig := frrv1beta1.FRRConfiguration{}
		err := m.GetClient().Get(context.TODO(), client.ObjectKey{Name: frrConfig.Name, Namespace: testNamespace}, &newConfig)
		if err != nil {
			return 0
		}
		return newConfig.Spec.BGP.Routers[0].ASN
	}, 5*time.Second, 200*time.Millisecond).Should(Equal(uint32(26)))

	// Changing the configuration from outside, we expect metallb to reconcile

	toChange := frrv1beta1.FRRConfiguration{}
	err = m.GetClient().Get(context.TODO(), client.ObjectKey{Name: frrConfig.Name, Namespace: testNamespace}, &toChange)
	g.Expect(err).ToNot(HaveOccurred())
	toChange.Spec.BGP.Routers[0].ASN = 25
	err = m.GetClient().Update(ctx, &toChange)
	g.Expect(err).ToNot(HaveOccurred())

	g.Eventually(func() int64 {
		toCheck := frrv1beta1.FRRConfiguration{}
		err := m.GetClient().Get(context.TODO(), client.ObjectKey{Name: frrConfig.Name, Namespace: testNamespace}, &toCheck)
		if err != nil {
			return 0
		}
		return toCheck.Generation
	}, 5*time.Second, 200*time.Millisecond).Should(BeNumerically(">", toChange.Generation))

	g.Eventually(func() uint32 {
		toCheck := frrv1beta1.FRRConfiguration{}
		err := m.GetClient().Get(context.TODO(), client.ObjectKey{Name: frrConfig.Name, Namespace: testNamespace}, &toCheck)
		if err != nil {
			return 0
		}
		return toCheck.Spec.BGP.Routers[0].ASN
	}, 5*time.Second, 200*time.Millisecond).Should(Equal(uint32(26)))

	storedConfig := frrv1beta1.FRRConfiguration{}
	err = m.GetClient().Get(context.TODO(), client.ObjectKey{Name: frrConfig.Name, Namespace: testNamespace}, &storedConfig)
	g.Expect(err).ToNot(HaveOccurred())

	withNoChanges := frrv1beta1.FRRConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      frrk8s.ConfigName("node"),
			Namespace: testNamespace,
		},
	}
	withNoChanges.Spec = *storedConfig.Spec.DeepCopy()

	// Not changing the spec Should not change the generation
	r.UpdateConfig(withNoChanges)

	g.Consistently(func() int64 {
		toCheck := frrv1beta1.FRRConfiguration{}
		err := m.GetClient().Get(context.TODO(), client.ObjectKey{Name: frrConfig.Name, Namespace: testNamespace}, &toCheck)
		if err != nil {
			return 0
		}
		return toCheck.Generation
	}, 5*time.Second, 200*time.Millisecond).Should(Equal(storedConfig.Generation))
}
