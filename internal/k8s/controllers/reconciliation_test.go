// SPDX-License-Identifier:Apache-2.0

package controllers

import (
	"context"
	"fmt"
	"net"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-kit/log"
	frrv1beta1 "github.com/metallb/frr-k8s/api/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	discovery "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	v1beta1 "go.universe.tf/metallb/api/v1beta1"
	v1beta2 "go.universe.tf/metallb/api/v1beta2"
	frrk8s "go.universe.tf/metallb/internal/bgp/frrk8s"
	"go.universe.tf/metallb/internal/config"
	"go.universe.tf/metallb/internal/layer2"
)

/*
	The tests defined here are those that check that the controllers are reacting to k8s objects events properly.
	The controllers are setup and started with the manager similarly to the real process.
*/

var (
	cfg       *rest.Config
	k8sClient client.Client
	testEnv   *envtest.Environment
	ctx       context.Context
	cancel    context.CancelFunc
	mgrDone   atomic.Bool

	configRequestHandler = requestHandler
	configMutex          sync.Mutex
	configUpdate         int

	nodeMutex        sync.Mutex
	nodeConfigUpdate int

	frrk8sReconciler       *FRRK8sReconciler
	layer2StatusReconciler *Layer2StatusReconciler
	layer2StatusUpdateChan = make(chan event.GenericEvent)
	layer2ServiceAdvs      []layer2.IPAdvertisement
	layer2ServiceAdvLock   sync.Mutex
	updateLayer2Advs       = func(advs []layer2.IPAdvertisement) {
		layer2ServiceAdvLock.Lock()
		defer layer2ServiceAdvLock.Unlock()
		layer2ServiceAdvs = advs
	}
)

const (
	testNodeName    = "testnode"
	testServiceName = "test-service"
)

func TestManager(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Manager Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	err := v1beta1.AddToScheme(scheme.Scheme)
	Expect(err).ToNot(HaveOccurred())

	err = v1beta2.AddToScheme(scheme.Scheme)
	Expect(err).ToNot(HaveOccurred())

	err = corev1.AddToScheme(scheme.Scheme)
	Expect(err).ToNot(HaveOccurred())

	err = discovery.AddToScheme(scheme.Scheme)
	Expect(err).ToNot(HaveOccurred())

	err = frrv1beta1.AddToScheme(scheme.Scheme)
	Expect(err).ToNot(HaveOccurred())

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("../../..", "config", "crd", "bases"),
			filepath.Join("../../..", "config", "crd", "bases"), "./testdata",
		},
		ErrorIfCRDPathMissing: true,
		Scheme:                scheme.Scheme,
	}

	// cfg is defined in this file globally.
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())
	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).ToNot(HaveOccurred())

	mockHandler := func(l log.Logger, n *corev1.Node) SyncState {
		nodeMutex.Lock()
		defer nodeMutex.Unlock()
		nodeConfigUpdate++
		return SyncStateSuccess
	}
	err = (&NodeReconciler{
		Client:    k8sManager.GetClient(),
		Scheme:    k8sManager.GetScheme(),
		Logger:    log.NewNopLogger(),
		Namespace: testNamespace,
		Handler:   mockHandler,
		NodeName:  testNodeName,
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	requestHandler = func(r *ConfigReconciler, ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
		configMutex.Lock()
		defer configMutex.Unlock()
		configUpdate++
		return ctrl.Result{}, nil
	}
	err = (&ConfigReconciler{
		Client:         k8sManager.GetClient(),
		Scheme:         k8sManager.GetScheme(),
		Logger:         log.NewNopLogger(),
		Namespace:      testNamespace,
		ValidateConfig: config.DontValidate,
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	frrk8sReconciler = &FRRK8sReconciler{
		Client:    k8sManager.GetClient(),
		Scheme:    k8sManager.GetScheme(),
		Logger:    log.NewNopLogger(),
		NodeName:  testNodeName,
		Namespace: testNamespace,
	}
	err = frrk8sReconciler.SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	layer2StatusReconciler = &Layer2StatusReconciler{
		Client:        k8sManager.GetClient(),
		Logger:        log.NewNopLogger(),
		NodeName:      testNodeName,
		ReconcileChan: layer2StatusUpdateChan,
		StatusFetcher: func(nn types.NamespacedName) []layer2.IPAdvertisement {
			layer2ServiceAdvLock.Lock()
			defer layer2ServiceAdvLock.Unlock()
			// only advertise for the test service to prevent
			// controller from reconcile infinitely
			if nn.Name == testServiceName {
				return layer2ServiceAdvs
			}
			return []layer2.IPAdvertisement{}
		},
	}
	err = layer2StatusReconciler.SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	ctx, cancel = context.WithCancel(context.TODO())

	go func() {
		defer func() { mgrDone.Store(true) }()
		defer GinkgoRecover()
		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
	}()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   testNodeName,
			Labels: map[string]string{"test": "e2e"},
		},
	}
	err = k8sClient.Create(ctx, node)
	Expect(err).ToNot(HaveOccurred())

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: testNamespace,
		},
	}
	err = k8sClient.Create(ctx, namespace)
	Expect(err).ToNot(HaveOccurred())
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	cancel()
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
	Eventually(mgrDone.Load, 5*time.Second, 200*time.Millisecond).Should(BeTrue())
	requestHandler = configRequestHandler
})

var _ = Describe("Config controller", func() {
	BeforeEach(func() {

	})
	Context("SetupWithManager", func() {
		It("Should Reconcile correctly", func() {
			// count for update on namespace events
			var initialConfigUpdateCount int
			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()
				time.Sleep(5 * time.Second)
				configMutex.Lock()
				initialConfigUpdateCount = configUpdate
				configMutex.Unlock()
			}()
			wg.Wait()

			// test new node event
			node := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "another-node",
					Labels: map[string]string{"test": "e2e"},
				},
			}
			err := k8sClient.Create(ctx, node)
			Expect(err).ToNot(HaveOccurred())

			Eventually(func() int {
				configMutex.Lock()
				defer configMutex.Unlock()
				return configUpdate
			}, 5*time.Second, 200*time.Millisecond).Should(Equal(initialConfigUpdateCount + 1))

			// test update node event with no changes into node label.
			Eventually(func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "another-node"}, node)
				if err != nil {
					return err
				}
				node.Labels = make(map[string]string)
				node.Spec.PodCIDR = "192.168.10.0/24"
				node.Labels["test"] = "e2e"
				err = k8sClient.Update(ctx, node)
				if err != nil {
					return err
				}
				return nil
			}, 5*time.Second, 200*time.Millisecond).ShouldNot(HaveOccurred())
			Eventually(func() int {
				configMutex.Lock()
				defer configMutex.Unlock()
				return configUpdate
			}, 5*time.Second, 200*time.Millisecond).Should(Equal(initialConfigUpdateCount + 1))

			// test update node event with changes into node label.
			Eventually(func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "another-node"}, node)
				if err != nil {
					return err
				}
				node.Labels = map[string]string{"test": "update"}
				err = k8sClient.Update(ctx, node)
				if err != nil {
					return err
				}
				return nil
			}, 5*time.Second, 200*time.Millisecond).ShouldNot(HaveOccurred())
			Eventually(func() int {
				configMutex.Lock()
				defer configMutex.Unlock()
				return configUpdate
			}, 5*time.Second, 200*time.Millisecond).Should(Equal(initialConfigUpdateCount + 2))
		})
	})
})

var _ = Describe("Node controller", func() {
	Context("SetupWithManager", func() {
		It("Should Reconcile correctly", func() {
			var initialConfigUpdateCount int
			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()
				time.Sleep(5 * time.Second)
				nodeMutex.Lock()
				initialConfigUpdateCount = nodeConfigUpdate
				nodeMutex.Unlock()
			}()
			wg.Wait()

			node := &corev1.Node{}
			// test update node event with no changes into node label.
			Eventually(func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: testNodeName}, node)
				if err != nil {
					return err
				}
				node.Labels = make(map[string]string)
				node.Spec.PodCIDR = "192.168.10.0/24"
				node.Labels["test"] = "e2e"
				err = k8sClient.Update(ctx, node)
				if err != nil {
					return err
				}
				return nil
			}, 5*time.Second, 200*time.Millisecond).ShouldNot(HaveOccurred())
			Eventually(func() int {
				nodeMutex.Lock()
				defer nodeMutex.Unlock()
				return nodeConfigUpdate
			}, 5*time.Second, 200*time.Millisecond).Should(Equal(initialConfigUpdateCount))

			// test update node event with changes into node label.
			Eventually(func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: testNodeName}, node)
				if err != nil {
					return err
				}
				node.Labels = map[string]string{"test": "update"}

				err = k8sClient.Update(ctx, node)
				if err != nil {
					return err
				}
				return nil
			}, 5*time.Second, 200*time.Millisecond).ShouldNot(HaveOccurred())
			Eventually(func() int {
				nodeMutex.Lock()
				defer nodeMutex.Unlock()
				return nodeConfigUpdate
			}, 5*time.Second, 200*time.Millisecond).Should(Equal(initialConfigUpdateCount + 1))
		})
	})
})

var _ = Describe("FRRK8S Controller", func() {
	Context("SetupWithManager", func() {
		It("Should Reconcile correctly", func() {
			frrConfig := frrv1beta1.FRRConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name:      frrk8s.ConfigName(testNodeName),
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
			err := k8sClient.Create(ctx, &frrConfig)
			Expect(err).ToNot(HaveOccurred())

			Eventually(func() bool {
				newConfig := frrv1beta1.FRRConfiguration{}
				err := k8sClient.Get(context.TODO(), client.ObjectKey{Name: frrConfig.Name, Namespace: testNamespace}, &newConfig)
				return apierrors.IsNotFound(err)
			}, 5*time.Second, 200*time.Millisecond).Should(BeTrue())

			frrk8sReconciler.UpdateConfig(frrConfig)
			Eventually(func() uint32 {
				newConfig := frrv1beta1.FRRConfiguration{}
				err := k8sClient.Get(context.TODO(), client.ObjectKey{Name: frrConfig.Name, Namespace: testNamespace}, &newConfig)
				if err != nil {
					return 0
				}
				return newConfig.Spec.BGP.Routers[0].ASN
			}, 5*time.Second, 200*time.Millisecond).Should(Equal(uint32(25)))

			// Notifying that the configuration changed
			frrConfig.Spec.BGP.Routers[0].ASN = 26
			frrk8sReconciler.UpdateConfig(frrConfig)

			Eventually(func() uint32 {
				newConfig := frrv1beta1.FRRConfiguration{}
				err := k8sClient.Get(context.TODO(), client.ObjectKey{Name: frrConfig.Name, Namespace: testNamespace}, &newConfig)
				if err != nil {
					return 0
				}
				return newConfig.Spec.BGP.Routers[0].ASN
			}, 5*time.Second, 200*time.Millisecond).Should(Equal(uint32(26)))

			// Changing the configuration from outside, we expect metallb to reconcile

			toChange := frrv1beta1.FRRConfiguration{}
			err = k8sClient.Get(context.TODO(), client.ObjectKey{Name: frrConfig.Name, Namespace: testNamespace}, &toChange)
			Expect(err).ToNot(HaveOccurred())
			toChange.Spec.BGP.Routers[0].ASN = 25
			err = k8sClient.Update(ctx, &toChange)
			Expect(err).ToNot(HaveOccurred())

			Eventually(func() int64 {
				toCheck := frrv1beta1.FRRConfiguration{}
				err := k8sClient.Get(context.TODO(), client.ObjectKey{Name: frrConfig.Name, Namespace: testNamespace}, &toCheck)
				if err != nil {
					return 0
				}
				return toCheck.Generation
			}, 5*time.Second, 200*time.Millisecond).Should(BeNumerically(">", toChange.Generation))

			Eventually(func() uint32 {
				toCheck := frrv1beta1.FRRConfiguration{}
				err := k8sClient.Get(context.TODO(), client.ObjectKey{Name: frrConfig.Name, Namespace: testNamespace}, &toCheck)
				if err != nil {
					return 0
				}
				return toCheck.Spec.BGP.Routers[0].ASN
			}, 5*time.Second, 200*time.Millisecond).Should(Equal(uint32(26)))

			storedConfig := frrv1beta1.FRRConfiguration{}
			err = k8sClient.Get(context.TODO(), client.ObjectKey{Name: frrConfig.Name, Namespace: testNamespace}, &storedConfig)
			Expect(err).ToNot(HaveOccurred())

			withNoChanges := frrv1beta1.FRRConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name:      frrk8s.ConfigName("node"),
					Namespace: testNamespace,
				},
			}
			withNoChanges.Spec = *storedConfig.Spec.DeepCopy()

			// Not changing the spec Should not change the generation
			frrk8sReconciler.UpdateConfig(withNoChanges)

			Consistently(func() int64 {
				toCheck := frrv1beta1.FRRConfiguration{}
				err := k8sClient.Get(context.TODO(), client.ObjectKey{Name: frrConfig.Name, Namespace: testNamespace}, &toCheck)
				if err != nil {
					return 0
				}
				return toCheck.Generation
			}, 5*time.Second, 200*time.Millisecond).Should(Equal(storedConfig.Generation))
		})
	})
})

var _ = Describe("Layer2 Status Controller", func() {
	Context("SetupWithManager", func() {
		It("Should Reconcile correctly", func() {

			statusObjName := fmt.Sprintf("%s-%s", testServiceName, layer2StatusReconciler.NodeName)

			// simulate some service is advertised
			layer2ServiceAdvs = append(layer2ServiceAdvs,
				layer2.NewIPAdvertisement(net.IP("127.0.0.1"), true, sets.Set[string]{}))
			// notify reconciler to reconcile
			layer2StatusUpdateChan <- NewL2StatusEvent(testNamespace, testServiceName)
			Eventually(func() string {
				toCheck := v1beta1.ServiceL2Status{}
				err := k8sClient.Get(context.TODO(), client.ObjectKey{Name: statusObjName, Namespace: testNamespace}, &toCheck)
				if err != nil {
					return ""
				}
				return toCheck.Status.Node
			}, 5*time.Second, 200*time.Millisecond).Should(Equal(testNodeName))

			// simulate the service advertisement interface changed
			interfaces := sets.Set[string]{}
			newInterface := "eth0"
			interfaces.Insert(newInterface)
			updateLayer2Advs([]layer2.IPAdvertisement{layer2.NewIPAdvertisement(net.IP("127.0.0.1"), false, interfaces)})
			layer2StatusUpdateChan <- NewL2StatusEvent(testNamespace, testServiceName)
			Eventually(func() string {
				toCheck := v1beta1.ServiceL2Status{}
				err := k8sClient.Get(context.TODO(), client.ObjectKey{Name: statusObjName, Namespace: testNamespace}, &toCheck)
				if err != nil || len(toCheck.Status.Interfaces) != 1 {
					return ""
				}
				return toCheck.Status.Interfaces[0].Name
			}, 5*time.Second, 200*time.Millisecond).Should(Equal(newInterface))

			// simulate the service is not advertised anymore
			updateLayer2Advs([]layer2.IPAdvertisement{})
			// notify the reconciler again
			layer2StatusUpdateChan <- NewL2StatusEvent(testNamespace, testServiceName)
			Eventually(func() bool {
				toCheck := v1beta1.ServiceL2Status{}
				err := k8sClient.Get(context.TODO(), client.ObjectKey{Name: statusObjName, Namespace: testNamespace}, &toCheck)
				return apierrors.IsNotFound(err)
			}, 5*time.Second, 200*time.Millisecond).Should(BeTrue())
		})
	})
})
