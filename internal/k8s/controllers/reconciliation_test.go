// SPDX-License-Identifier:Apache-2.0

package controllers

import (
	"context"
	"fmt"
	"net"
	"path/filepath"
	"reflect"
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
	"go.universe.tf/metallb/internal/allocator"
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
	speakerPod *corev1.Pod

	bgpStatusReconcileChan = make(chan event.GenericEvent)
	bgpAdvs                = map[string]sets.Set[string]{}
	bgpAdvsMutex           = sync.Mutex{}

	poolStatusReconcileChan = make(chan event.GenericEvent)
	poolCounters            = map[string]allocator.PoolCounters{}
	poolCountersMutex       sync.Mutex
)

const (
	testNodeName     = "testnode"
	testServiceName  = "test-service"
	speakerNamespace = "metallb-system"
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

	ctx, cancel = context.WithCancel(context.TODO())
	// test service is located in testNamespace
	// speaker pod is deployed in speakerNamespace
	// the layer2 status crs are maintained in speakerNamespace
	err = func(namespaces []string) error {
		for _, namespace := range namespaces {
			namespaceObj := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespace,
				},
			}
			err := k8sClient.Create(ctx, namespaceObj)
			if err != nil {
				return err
			}
		}
		return nil
	}([]string{testNamespace, speakerNamespace})
	Expect(err).ToNot(HaveOccurred())

	speakerPod = &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "speaker", Namespace: speakerNamespace},
		Spec: corev1.PodSpec{Containers: []corev1.Container{
			{Name: "speaker", Image: "speaker"},
		}},
	}
	err = k8sClient.Create(ctx, speakerPod)
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
		Client:          k8sManager.GetClient(),
		Scheme:          k8sManager.GetScheme(),
		Logger:          log.NewNopLogger(),
		NodeName:        testNodeName,
		FRRK8sNamespace: testNamespace,
	}
	err = frrk8sReconciler.SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	layer2StatusReconciler = &Layer2StatusReconciler{
		Client:        k8sManager.GetClient(),
		Logger:        log.NewNopLogger(),
		NodeName:      testNodeName,
		Namespace:     speakerNamespace,
		SpeakerPod:    speakerPod,
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

	bgpStatusReconciler := &ServiceBGPStatusReconciler{
		Client:        k8sManager.GetClient(),
		Logger:        log.NewNopLogger(),
		NodeName:      testNodeName,
		Namespace:     speakerNamespace,
		SpeakerPod:    speakerPod,
		ReconcileChan: bgpStatusReconcileChan,
		PeersFetcher: func(key string) sets.Set[string] {
			bgpAdvsMutex.Lock()
			defer bgpAdvsMutex.Unlock()
			return bgpAdvs[key]
		},
	}
	err = bgpStatusReconciler.SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	poolStatusReconciler := &PoolStatusReconciler{
		Client: k8sManager.GetClient(),
		Logger: log.NewNopLogger(),
		CountersFetcher: func(s string) allocator.PoolCounters {
			poolCountersMutex.Lock()
			defer poolCountersMutex.Unlock()
			return poolCounters[s]
		},
		ReconcileChan: poolStatusReconcileChan,
	}
	err = poolStatusReconciler.SetupWithManager(k8sManager)

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
			statusObjFetcherFunc := func() ([]v1beta1.ServiceL2Status, error) {
				statusList := v1beta1.ServiceL2StatusList{}
				err := k8sClient.List(context.TODO(), &statusList,
					client.MatchingLabels{
						LabelServiceName:      testServiceName,
						LabelServiceNamespace: testNamespace,
						LabelAnnounceNode:     testNodeName,
					},
				)
				if err != nil {
					return nil, err
				}
				return statusList.Items, nil
			}

			statusResultCheckFunc := func(statuses []v1beta1.ServiceL2Status) error {
				if len(statuses) != 1 {
					return fmt.Errorf("expect 1 status object, but got %d", len(statuses))
				}
				if len(statuses[0].OwnerReferences) != 1 {
					return fmt.Errorf("expect 1 owner reference, but got %d", len(statuses[0].OwnerReferences))
				}
				ownRef := statuses[0].OwnerReferences[0]
				if ownRef.UID != speakerPod.UID {
					return fmt.Errorf("owner reference is not speaker pod, expect owner reference uid %s, but got %s", speakerPod.UID, ownRef.UID)
				}
				return nil
			}

			// simulate some service is advertised
			layer2ServiceAdvs = append(layer2ServiceAdvs,
				layer2.NewIPAdvertisement(net.IP("127.0.0.1"), true, sets.Set[string]{}))
			// notify reconciler to reconcile
			layer2StatusUpdateChan <- NewL2StatusEvent(testNamespace, testServiceName)
			Eventually(func() string {
				statuses, err := statusObjFetcherFunc()
				if err != nil {
					return err.Error()
				}
				err = statusResultCheckFunc(statuses)
				if err != nil {
					return err.Error()
				}
				return statuses[0].Status.Node
			}, 5*time.Second, 200*time.Millisecond).Should(Equal(testNodeName))

			// simulate the service advertisement interface changed
			interfaces := sets.Set[string]{}
			newInterface := "eth0"
			interfaces.Insert(newInterface)
			updateLayer2Advs([]layer2.IPAdvertisement{layer2.NewIPAdvertisement(net.IP("127.0.0.1"), false, interfaces)})
			layer2StatusUpdateChan <- NewL2StatusEvent(testNamespace, testServiceName)
			Eventually(func() string {
				statuses, err := statusObjFetcherFunc()
				if err != nil {
					return err.Error()
				}
				err = statusResultCheckFunc(statuses)
				if err != nil {
					return err.Error()
				}
				if len(statuses[0].Status.Interfaces) == 0 {
					return fmt.Errorf("status object has no interfaces").Error()
				}
				return statuses[0].Status.Interfaces[0].Name
			}, 5*time.Second, 200*time.Millisecond).Should(Equal(newInterface))

			// simulate the service is not advertised anymore
			updateLayer2Advs([]layer2.IPAdvertisement{})
			// notify the reconciler again
			layer2StatusUpdateChan <- NewL2StatusEvent(testNamespace, testServiceName)
			Eventually(func() bool {
				statuses, err := statusObjFetcherFunc()
				if err != nil {
					return false
				}
				return len(statuses) == 0
			}, 5*time.Second, 200*time.Millisecond).Should(BeTrue())
		})
	})
})

var _ = Describe("BGP Status Controller", func() {
	Context("SetupWithManager", func() {
		It("Should Reconcile correctly", func() {
			serviceKey := types.NamespacedName{Namespace: testNamespace, Name: testServiceName}.String()
			getStatus := func() (*v1beta1.ServiceBGPStatus, error) {
				list := v1beta1.ServiceBGPStatusList{}
				err := k8sClient.List(context.TODO(), &list)
				if err != nil {
					return nil, err
				}

				if len(list.Items) != 1 {
					return nil, fmt.Errorf("expected 1 status, got %v", list.Items)
				}

				status := list.Items[0]
				if len(status.OwnerReferences) != 1 {
					return nil, fmt.Errorf("expected 1 owner reference, got %v", status.OwnerReferences)
				}

				ownerRef := status.OwnerReferences[0]
				if ownerRef.UID != speakerPod.UID {
					return nil, fmt.Errorf("owner reference is not speaker pod, got %v", ownerRef)
				}

				if status.Labels[LabelAnnounceNode] != testNodeName {
					return nil, fmt.Errorf("labels do not match node, got %v", status.Labels)
				}

				if status.Status.Node != testNodeName {
					return nil, fmt.Errorf("status does not match node, got %v", status.Status)
				}

				if status.Labels[LabelServiceNamespace] != testNamespace {
					return nil, fmt.Errorf("labels do not match namespace, got %v", status.Labels)
				}

				if status.Status.ServiceNamespace != testNamespace {
					return nil, fmt.Errorf("status does not match namespace, got %v", status.Status)
				}

				if status.Labels[LabelServiceName] != testServiceName {
					return nil, fmt.Errorf("labels do not match service name, got %v", status.Labels)
				}

				if status.Status.ServiceName != testServiceName {
					return nil, fmt.Errorf("status does not match service name, got %v", status.Status)
				}

				return &status, nil
			}

			bgpAdvsMutex.Lock()
			bgpAdvs[serviceKey] = sets.New[string]("peer1")
			bgpAdvsMutex.Unlock()
			bgpStatusReconcileChan <- NewBGPStatusEvent(testNamespace, testServiceName)
			expectedPeers := []string{"peer1"}
			Eventually(func() error {
				s, err := getStatus()
				if err != nil {
					return err
				}

				if !reflect.DeepEqual(expectedPeers, s.Status.Peers) {
					return fmt.Errorf("expected peers to be %v, got %v", expectedPeers, s.Status.Peers)
				}

				return nil
			}, 5*time.Second, 200*time.Millisecond).ShouldNot(HaveOccurred())

			bgpAdvsMutex.Lock()
			bgpAdvs[serviceKey] = sets.New[string]("peer1", "peer2")
			bgpAdvsMutex.Unlock()
			bgpStatusReconcileChan <- NewBGPStatusEvent(testNamespace, testServiceName)
			expectedPeers = []string{"peer1", "peer2"}
			Eventually(func() error {
				s, err := getStatus()
				if err != nil {
					return err
				}

				if !reflect.DeepEqual(expectedPeers, s.Status.Peers) {
					return fmt.Errorf("expected peers to be %v, got %v", expectedPeers, s.Status.Peers)
				}

				return nil
			}, 5*time.Second, 200*time.Millisecond).ShouldNot(HaveOccurred())

			// Manual updates should be reverted by the controller
			status, err := getStatus()
			Expect(err).ToNot(HaveOccurred())
			status.Status.Peers = []string{"manual"}
			err = k8sClient.Status().Update(context.TODO(), status)
			Expect(err).To(Not(HaveOccurred()))
			Eventually(func() error {
				s, err := getStatus()
				if err != nil {
					return err
				}

				if !reflect.DeepEqual(expectedPeers, s.Status.Peers) {
					return fmt.Errorf("expected peers to be %v, got %v", expectedPeers, s.Status.Peers)
				}

				return nil
			}, 5*time.Second, 200*time.Millisecond).ShouldNot(HaveOccurred())

			bgpAdvsMutex.Lock()
			delete(bgpAdvs, serviceKey)
			bgpAdvsMutex.Unlock()
			bgpStatusReconcileChan <- NewBGPStatusEvent(testNamespace, testServiceName)
			Eventually(func() error {
				list := v1beta1.ServiceBGPStatusList{}
				err := k8sClient.List(context.TODO(), &list)
				if err != nil {
					return err
				}

				if len(list.Items) != 0 {
					return fmt.Errorf("expected no statuses, got %v", list.Items)
				}

				return nil
			}, 5*time.Second, 200*time.Millisecond).ShouldNot(HaveOccurred())

			Consistently(func() error {
				list := v1beta1.ServiceBGPStatusList{}
				err := k8sClient.List(context.TODO(), &list)
				if err != nil {
					return err
				}

				if len(list.Items) != 0 {
					return fmt.Errorf("expected no statuses, got %v", list.Items)
				}

				return nil
			}, 1*time.Second, 200*time.Millisecond).ShouldNot(HaveOccurred())
		})
	})
})

var _ = Describe("PoolStatus Controller", func() {
	Context("SetupWithManager", func() {
		testPoolName := "test"
		It("Should Reconcile correctly", func() {
			pool := v1beta1.IPAddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testPoolName,
					Namespace: testNamespace,
				},
				Spec: v1beta1.IPAddressPoolSpec{
					Addresses: []string{
						"1.2.3.4/30",
						"1000::4/126",
					},
				},
			}
			validateStatus := func(expected v1beta1.IPAddressPoolStatus) {
				Eventually(func() error {
					newPool := v1beta1.IPAddressPool{}
					err := k8sClient.Get(context.TODO(), client.ObjectKey{Name: pool.Name, Namespace: testNamespace}, &newPool)
					if err != nil {
						return err
					}
					if !reflect.DeepEqual(newPool.Status, expected) {
						return fmt.Errorf("pool status does not match, got [%v] expected [%v]", newPool.Status, expected)
					}
					return nil
				}, 5*time.Second, 200*time.Millisecond).ShouldNot(HaveOccurred())
			}

			poolCountersMutex.Lock()
			poolCounters[testPoolName] = allocator.PoolCounters{
				AvailableIPv4: 4,
				AvailableIPv6: 4,
				AssignedIPv4:  0,
				AssignedIPv6:  0,
			}
			poolCountersMutex.Unlock()
			err := k8sClient.Create(ctx, &pool)
			Expect(err).ToNot(HaveOccurred())

			expectedStatus := v1beta1.IPAddressPoolStatus{
				AvailableIPv4: 4,
				AvailableIPv6: 4,
				AssignedIPv4:  0,
				AssignedIPv6:  0,
			}
			validateStatus(expectedStatus)

			// Generate a status event
			poolCountersMutex.Lock()
			poolCounters[testPoolName] = allocator.PoolCounters{
				AvailableIPv4: 3,
				AvailableIPv6: 3,
				AssignedIPv4:  1,
				AssignedIPv6:  1,
			}
			poolCountersMutex.Unlock()
			poolStatusReconcileChan <- NewPoolStatusEvent(testNamespace, testPoolName)

			expectedStatus = v1beta1.IPAddressPoolStatus{
				AvailableIPv4: 3,
				AvailableIPv6: 3,
				AssignedIPv4:  1,
				AssignedIPv6:  1,
			}
			validateStatus(expectedStatus)

			// Manual updates should be reverted by the controller
			err = k8sClient.Get(context.TODO(), client.ObjectKey{Name: pool.Name, Namespace: testNamespace}, &pool)
			Expect(err).To(Not(HaveOccurred()))
			pool.Status = v1beta1.IPAddressPoolStatus{
				AvailableIPv4: 0,
				AvailableIPv6: 0,
				AssignedIPv4:  1,
				AssignedIPv6:  1,
			}
			err = k8sClient.Status().Update(context.TODO(), &pool)
			Expect(err).To(Not(HaveOccurred()))
			validateStatus(expectedStatus)
		})
	})
})
