/*
Copyright 2016 The Kubernetes Authors.

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

package service

import (
	"context"
	"fmt"
	"time"

	"github.com/onsi/ginkgo/v2"
	v1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

const agnostImage = "registry.k8s.io/e2e-test-images/agnhost:2.45"
const ServiceEndpointsTimeout = 2 * time.Minute
const loadbalancerCreateTimeout = 5 * time.Minute

// NodePortRange should match whatever the default/configured range is
var NodePortRange = utilnet.PortRange{Base: 30000, Size: 2768}

// TestJig is a test jig to help service testing.
type TestJig struct {
	Client    clientset.Interface
	Namespace string
	Name      string
	ID        string
	Labels    map[string]string
	// ExternalIPs should be false for Conformance test
	// Don't check nodeport on external addrs in conformance test, but in e2e test.
	ExternalIPs bool
	Image       string
}

// NewTestJig allocates and inits a new TestJig.
func NewTestJig(client clientset.Interface, namespace, name string) *TestJig {
	j := &TestJig{}
	j.Client = client
	j.Namespace = namespace
	j.Name = name
	j.ID = j.Name + "-" + string(uuid.NewUUID())
	j.Labels = map[string]string{"testid": j.ID}
	j.Image = agnostImage

	return j
}

// newServiceTemplate returns the default v1.Service template for this j, but
// does not actually create the Service.  The default Service has the same name
// as the j and exposes the given port.
func (j *TestJig) newServiceTemplate(proto v1.Protocol, port int32) *v1.Service {
	service := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: j.Namespace,
			Name:      j.Name,
			Labels:    j.Labels,
		},
		Spec: v1.ServiceSpec{
			Selector: j.Labels,
			Ports: []v1.ServicePort{
				{
					Protocol: proto,
					Port:     port,
				},
			},
		},
	}
	return service
}

// CreateTCPServiceWithPort creates a new TCP Service with given port based on the
// j's defaults. Callers can provide a function to tweak the Service object before
// it is created.
func (j *TestJig) CreateTCPServiceWithPort(ctx context.Context, tweak func(svc *v1.Service), port int32) (*v1.Service, error) {
	svc := j.newServiceTemplate(v1.ProtocolTCP, port)
	if tweak != nil {
		tweak(svc)
	}
	result, err := j.Client.CoreV1().Services(j.Namespace).Create(ctx, svc, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create TCP Service %q: %w", svc.Name, err)
	}
	return j.sanityCheckService(result, svc.Spec.Type)
}

// CreateTCPService creates a new TCP Service based on the j's
// defaults.  Callers can provide a function to tweak the Service object before
// it is created.
func (j *TestJig) CreateTCPService(ctx context.Context, tweak func(svc *v1.Service)) (*v1.Service, error) {
	return j.CreateTCPServiceWithPort(ctx, tweak, 80)
}

// CreateUDPService creates a new UDP Service based on the j's
// defaults.  Callers can provide a function to tweak the Service object before
// it is created.
func (j *TestJig) CreateUDPService(ctx context.Context, tweak func(svc *v1.Service)) (*v1.Service, error) {
	svc := j.newServiceTemplate(v1.ProtocolUDP, 80)
	if tweak != nil {
		tweak(svc)
	}
	result, err := j.Client.CoreV1().Services(j.Namespace).Create(ctx, svc, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create UDP Service %q: %w", svc.Name, err)
	}
	return j.sanityCheckService(result, svc.Spec.Type)
}

// CreateOnlyLocalLoadBalancerService creates a loadbalancer service with
// ExternalTrafficPolicy set to Local and waits for it to acquire an ingress IP.
// If createPod is true, it also creates an RC with 1 replica of
// the standard netexec container used everywhere in this test.
func (j *TestJig) CreateOnlyLocalLoadBalancerService(ctx context.Context, createPod bool,
	tweak func(svc *v1.Service)) (*v1.Service, error) {
	_, err := j.CreateLoadBalancerService(ctx, func(svc *v1.Service) {
		ginkgo.By("setting ExternalTrafficPolicy=Local")
		svc.Spec.ExternalTrafficPolicy = v1.ServiceExternalTrafficPolicyLocal
		if tweak != nil {
			tweak(svc)
		}
	})
	if err != nil {
		return nil, err
	}

	if createPod {
		ginkgo.By("creating a pod to be part of the service " + j.Name)
		_, err = j.Run(ctx, nil)
		if err != nil {
			return nil, err
		}
	}
	ginkgo.By("waiting for loadbalancer for service " + j.Namespace + "/" + j.Name)
	return j.WaitForLoadBalancer(ctx, loadbalancerCreateTimeout)
}

// CreateLoadBalancerService creates a loadbalancer service and waits
// for it to acquire an ingress IP.
func (j *TestJig) CreateLoadBalancerService(ctx context.Context, tweak func(svc *v1.Service)) (*v1.Service, error) {
	return j.CreateLoadBalancerServiceWithTimeout(ctx, loadbalancerCreateTimeout, tweak)
}

// CreateLoadBalancerServiceWithTimeout creates a loadbalancer service and waits
// for it to acquire an ingress IP.
func (j *TestJig) CreateLoadBalancerServiceWithTimeout(ctx context.Context, timeout time.Duration, tweak func(svc *v1.Service)) (*v1.Service, error) {
	ginkgo.By("creating a service " + j.Namespace + "/" + j.Name + " with type=LoadBalancer")
	svc := j.newServiceTemplate(v1.ProtocolTCP, 80)
	svc.Spec.Type = v1.ServiceTypeLoadBalancer
	// We need to turn affinity off for our LB distribution tests
	svc.Spec.SessionAffinity = v1.ServiceAffinityNone
	if tweak != nil {
		tweak(svc)
	}
	_, err := j.Client.CoreV1().Services(j.Namespace).Create(ctx, svc, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create LoadBalancer Service %q: %w", svc.Name, err)
	}

	ginkgo.By("waiting for loadbalancer for service " + j.Namespace + "/" + j.Name)
	return j.WaitForLoadBalancer(ctx, timeout)
}

// ListNodesWithEndpoint returns a list of nodes on which the
// endpoints of the given Service are running.
func (j *TestJig) ListNodesWithEndpoint(ctx context.Context) ([]v1.Node, error) {
	nodeNames, err := j.GetEndpointNodeNames(ctx)
	if err != nil {
		return nil, err
	}
	allNodes, err := j.Client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	epNodes := make([]v1.Node, 0, nodeNames.Len())
	for _, node := range allNodes.Items {
		if nodeNames.Has(node.Name) {
			epNodes = append(epNodes, node)
		}
	}
	return epNodes, nil
}

// GetEndpointNodeNames returns a string set of node names on which the
// endpoints of the given Service are running.
func (j *TestJig) GetEndpointNodeNames(ctx context.Context) (sets.String, error) {
	err := j.waitForAvailableEndpoint(ctx, ServiceEndpointsTimeout)
	if err != nil {
		return nil, err
	}
	endpoints, err := j.Client.CoreV1().Endpoints(j.Namespace).Get(ctx, j.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get endpoints for service %s/%s failed (%s)", j.Namespace, j.Name, err)
	}
	if len(endpoints.Subsets) == 0 {
		return nil, fmt.Errorf("endpoint has no subsets, cannot determine node addresses")
	}
	epNodes := sets.NewString()
	for _, ss := range endpoints.Subsets {
		for _, e := range ss.Addresses {
			if e.NodeName != nil {
				epNodes.Insert(*e.NodeName)
			}
		}
	}
	return epNodes, nil
}

// waitForAvailableEndpoint waits for at least 1 endpoint to be available till timeout
func (j *TestJig) waitForAvailableEndpoint(ctx context.Context, timeout time.Duration) error {
	//Wait for endpoints to be created, this may take longer time if service backing pods are taking longer time to run
	endpointSelector := fields.OneTermEqualSelector("metadata.name", j.Name)
	stopCh := make(chan struct{})
	endpointAvailable := false
	endpointSliceAvailable := false

	var controller cache.Controller
	_, controller = cache.NewInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				options.FieldSelector = endpointSelector.String()
				obj, err := j.Client.CoreV1().Endpoints(j.Namespace).List(ctx, options)
				return runtime.Object(obj), err
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				options.FieldSelector = endpointSelector.String()
				return j.Client.CoreV1().Endpoints(j.Namespace).Watch(ctx, options)
			},
		},
		&v1.Endpoints{},
		0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				if e, ok := obj.(*v1.Endpoints); ok {
					if len(e.Subsets) > 0 && len(e.Subsets[0].Addresses) > 0 {
						endpointAvailable = true
					}
				}
			},
			UpdateFunc: func(old, cur interface{}) {
				if e, ok := cur.(*v1.Endpoints); ok {
					if len(e.Subsets) > 0 && len(e.Subsets[0].Addresses) > 0 {
						endpointAvailable = true
					}
				}
			},
		},
	)
	defer func() {
		close(stopCh)
	}()

	go controller.Run(stopCh)

	var esController cache.Controller
	_, esController = cache.NewInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				options.LabelSelector = "kubernetes.io/service-name=" + j.Name
				obj, err := j.Client.DiscoveryV1().EndpointSlices(j.Namespace).List(ctx, options)
				return runtime.Object(obj), err
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				options.LabelSelector = "kubernetes.io/service-name=" + j.Name
				return j.Client.DiscoveryV1().EndpointSlices(j.Namespace).Watch(ctx, options)
			},
		},
		&discoveryv1.EndpointSlice{},
		0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				if es, ok := obj.(*discoveryv1.EndpointSlice); ok {
					// TODO: currently we only consider addresses in 1 slice, but services with
					// a large number of endpoints (>1000) may have multiple slices. Some slices
					// with only a few addresses. We should check the addresses in all slices.
					if len(es.Endpoints) > 0 && len(es.Endpoints[0].Addresses) > 0 {
						endpointSliceAvailable = true
					}
				}
			},
			UpdateFunc: func(old, cur interface{}) {
				if es, ok := cur.(*discoveryv1.EndpointSlice); ok {
					// TODO: currently we only consider addresses in 1 slice, but services with
					// a large number of endpoints (>1000) may have multiple slices. Some slices
					// with only a few addresses. We should check the addresses in all slices.
					if len(es.Endpoints) > 0 && len(es.Endpoints[0].Addresses) > 0 {
						endpointSliceAvailable = true
					}
				}
			},
		},
	)

	go esController.Run(stopCh)

	err := wait.PollWithContext(ctx, 1*time.Second, timeout, func(ctx context.Context) (bool, error) {
		return endpointAvailable && endpointSliceAvailable, nil
	})
	if err != nil {
		return fmt.Errorf("no subset of available IP address found for the endpoint %s within timeout %v", j.Name, timeout)
	}
	return nil
}

// sanityCheckService performs sanity checks on the given service; in particular, ensuring
// that creating/updating a service allocates IPs, ports, etc, as needed. It does not
// check for ingress assignment as that happens asynchronously after the Service is created.
func (j *TestJig) sanityCheckService(svc *v1.Service, svcType v1.ServiceType) (*v1.Service, error) {
	if svcType == "" {
		svcType = v1.ServiceTypeClusterIP
	}
	if svc.Spec.Type != svcType {
		return nil, fmt.Errorf("unexpected Spec.Type (%s) for service, expected %s", svc.Spec.Type, svcType)
	}

	if svcType != v1.ServiceTypeExternalName {
		if svc.Spec.ExternalName != "" {
			return nil, fmt.Errorf("unexpected Spec.ExternalName (%s) for service, expected empty", svc.Spec.ExternalName)
		}
		if svc.Spec.ClusterIP == "" {
			return nil, fmt.Errorf("didn't get ClusterIP for non-ExternalName service")
		}
	} else {
		if svc.Spec.ClusterIP != "" {
			return nil, fmt.Errorf("unexpected Spec.ClusterIP (%s) for ExternalName service, expected empty", svc.Spec.ClusterIP)
		}
	}

	expectNodePorts := needsNodePorts(svc)
	for i, port := range svc.Spec.Ports {
		hasNodePort := (port.NodePort != 0)
		if hasNodePort != expectNodePorts {
			return nil, fmt.Errorf("unexpected Spec.Ports[%d].NodePort (%d) for service", i, port.NodePort)
		}
		if hasNodePort {
			if !NodePortRange.Contains(int(port.NodePort)) {
				return nil, fmt.Errorf("out-of-range nodePort (%d) for service", port.NodePort)
			}
		}
	}

	// FIXME: this fails for tests that were changed from LoadBalancer to ClusterIP.
	// if svcType != v1.ServiceTypeLoadBalancer {
	// 	if len(svc.Status.LoadBalancer.Ingress) != 0 {
	// 		return nil, fmt.Errorf("unexpected Status.LoadBalancer.Ingress on non-LoadBalancer service")
	// 	}
	// }

	return svc, nil
}

func needsNodePorts(svc *v1.Service) bool {
	if svc == nil {
		return false
	}
	// Type NodePort
	if svc.Spec.Type == v1.ServiceTypeNodePort {
		return true
	}
	if svc.Spec.Type != v1.ServiceTypeLoadBalancer {
		return false
	}
	// Type LoadBalancer
	if svc.Spec.AllocateLoadBalancerNodePorts == nil {
		return true //back-compat
	}
	return *svc.Spec.AllocateLoadBalancerNodePorts
}

// UpdateService fetches a service, calls the update function on it, and
// then attempts to send the updated service. It tries up to 3 times in the
// face of timeouts and conflicts.
func (j *TestJig) UpdateService(ctx context.Context, update func(*v1.Service)) (*v1.Service, error) {
	for i := 0; i < 3; i++ {
		service, err := j.Client.CoreV1().Services(j.Namespace).Get(ctx, j.Name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to get Service %q: %w", j.Name, err)
		}
		update(service)
		result, err := j.Client.CoreV1().Services(j.Namespace).Update(ctx, service, metav1.UpdateOptions{})
		if err == nil {
			return j.sanityCheckService(result, service.Spec.Type)
		}
		if !apierrors.IsConflict(err) && !apierrors.IsServerTimeout(err) {
			return nil, fmt.Errorf("failed to update Service %q: %w", j.Name, err)
		}
	}
	return nil, fmt.Errorf("too many retries updating Service %q", j.Name)
}

// WaitForLoadBalancer waits the given service to have a LoadBalancer, or returns an error after the given timeout
func (j *TestJig) WaitForLoadBalancer(ctx context.Context, timeout time.Duration) (*v1.Service, error) {
	ginkgo.GinkgoWriter.Printf("\tWaiting up to %v for service %q to have a LoadBalancer\n", timeout, j.Name)
	service, err := j.waitForCondition(ctx, timeout, "have a load balancer", func(svc *v1.Service) bool {
		return len(svc.Status.LoadBalancer.Ingress) > 0
	})
	if err != nil {
		return nil, err
	}

	for i, ing := range service.Status.LoadBalancer.Ingress {
		if ing.IP == "" && ing.Hostname == "" {
			return nil, fmt.Errorf("unexpected Status.LoadBalancer.Ingress[%d] for service: %#v", i, ing)
		}
	}

	return j.sanityCheckService(service, v1.ServiceTypeLoadBalancer)
}

func (j *TestJig) waitForCondition(ctx context.Context, timeout time.Duration, message string, conditionFn func(*v1.Service) bool) (*v1.Service, error) {
	var service *v1.Service
	pollFunc := func(ctx context.Context) (bool, error) {
		svc, err := j.Client.CoreV1().Services(j.Namespace).Get(ctx, j.Name, metav1.GetOptions{})
		if err != nil {
			ginkgo.GinkgoWriter.Printf("Retrying .... error trying to get Service %s: %v", j.Name, err)
			return false, nil
		}
		if conditionFn(svc) {
			service = svc
			return true, nil
		}
		return false, nil
	}
	if err := wait.PollUntilContextTimeout(ctx, 2*time.Second, timeout, true, pollFunc); err != nil {
		return nil, fmt.Errorf("timed out waiting for service %q to %s: %w", j.Name, message, err)
	}
	return service, nil
}

// newRCTemplate returns the default v1.ReplicationController object for
// this j, but does not actually create the RC.  The default RC has the same
// name as the j and runs the "netexec" container.
func (j *TestJig) newRCTemplate() *v1.ReplicationController {
	var replicas int32 = 1
	var grace int64 = 3 // so we don't race with kube-proxy when scaling up/down

	rc := &v1.ReplicationController{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: j.Namespace,
			Name:      j.Name,
			Labels:    j.Labels,
		},
		Spec: v1.ReplicationControllerSpec{
			Replicas: &replicas,
			Selector: j.Labels,
			Template: &v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: j.Labels,
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "netexec",
							Image: j.Image,
							Args:  []string{"netexec", "--http-port=80", "--udp-port=80"},
							ReadinessProbe: &v1.Probe{
								PeriodSeconds: 3,
								ProbeHandler: v1.ProbeHandler{
									HTTPGet: &v1.HTTPGetAction{
										Port: intstr.FromInt32(80),
										Path: "/hostName",
									},
								},
							},
						},
					},
					TerminationGracePeriodSeconds: &grace,
				},
			},
		},
	}
	return rc
}

// Run creates a ReplicationController and Pod(s) and waits for the
// Pod(s) to be running. Callers can provide a function to tweak the RC object
// before it is created.
func (j *TestJig) Run(ctx context.Context, tweak func(rc *v1.ReplicationController)) (*v1.ReplicationController, error) {
	rc := j.newRCTemplate()
	if tweak != nil {
		tweak(rc)
	}
	result, err := j.Client.CoreV1().ReplicationControllers(j.Namespace).Create(ctx, rc, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create RC %q: %w", rc.Name, err)
	}
	pods, err := j.waitForPodsCreated(ctx, int(*(rc.Spec.Replicas)))
	if err != nil {
		return nil, fmt.Errorf("failed to create pods: %w", err)
	}
	if err := j.waitForPodsReady(ctx, pods); err != nil {
		return nil, fmt.Errorf("failed waiting for pods to be running: %w", err)
	}
	return result, nil
}

// Scale scales pods to the given replicas
func (j *TestJig) Scale(ctx context.Context, replicas int) error {
	rc := j.Name
	scale, err := j.Client.CoreV1().ReplicationControllers(j.Namespace).GetScale(ctx, rc, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get scale for RC %q: %w", rc, err)
	}

	scale.ResourceVersion = "" // indicate the scale update should be unconditional
	scale.Spec.Replicas = int32(replicas)
	_, err = j.Client.CoreV1().ReplicationControllers(j.Namespace).UpdateScale(ctx, rc, scale, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to scale RC %q: %w", rc, err)
	}
	pods, err := j.waitForPodsCreated(ctx, replicas)
	if err != nil {
		return fmt.Errorf("failed waiting for pods: %w", err)
	}
	if err := j.waitForPodsReady(ctx, pods); err != nil {
		return fmt.Errorf("failed waiting for pods to be running: %w", err)
	}
	return nil
}

func (j *TestJig) waitForPodsCreated(ctx context.Context, replicas int) ([]string, error) {
	// TODO (pohly): replace with gomega.Eventually
	timeout := 2 * time.Minute
	// List the pods, making sure we observe all the replicas.
	label := labels.SelectorFromSet(labels.Set(j.Labels))
	ginkgo.GinkgoWriter.Printf("\tWaiting up to %v for %d pods to be created\n", timeout, replicas)
	for start := time.Now(); time.Since(start) < timeout && ctx.Err() == nil; time.Sleep(2 * time.Second) {
		options := metav1.ListOptions{LabelSelector: label.String()}
		pods, err := j.Client.CoreV1().Pods(j.Namespace).List(ctx, options)
		if err != nil {
			return nil, err
		}

		found := []string{}
		for _, pod := range pods.Items {
			if pod.DeletionTimestamp != nil {
				continue
			}
			found = append(found, pod.Name)
		}
		if len(found) == replicas {
			ginkgo.GinkgoWriter.Printf("\t\tFound all %d pods\n", replicas)
			return found, nil
		}
		ginkgo.GinkgoWriter.Printf("\t\tFound %d/%d pods - will retry\n", len(found), replicas)
	}
	return nil, fmt.Errorf("timeout waiting for %d pods to be created", replicas)
}

func (j *TestJig) waitForPodsReady(ctx context.Context, pods []string) error {
	timeout := 2 * time.Minute
	for _, pod := range pods {
		if err := waitForPhase(j.Client, pod, j.Namespace, v1.PodRunning, timeout); err != nil {
			return fmt.Errorf("timeout waiting for pod %s to be ready: %w", pod, err)
		}
	}
	return nil
}

// WaitForPhase waits until the pod will be in specified phase
func waitForPhase(cs clientset.Interface, pod, namespace string, phaseType v1.PodPhase, timeout time.Duration) error {
	return wait.PollImmediate(time.Second, timeout, func() (bool, error) {
		updatePod, err := cs.CoreV1().Pods(namespace).Get(context.Background(), pod, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}

		return updatePod.Status.Phase == phaseType, nil
	})
}

// CreateLoadBalancerServiceWaitForClusterIPOnly creates a loadbalancer service and waits
// for it to acquire a cluster IP
func (j *TestJig) CreateLoadBalancerServiceWaitForClusterIPOnly(tweak func(svc *v1.Service)) (*v1.Service, error) {
	ginkgo.By("creating a service " + j.Namespace + "/" + j.Name + " with type=LoadBalancer")
	svc := j.newServiceTemplate(v1.ProtocolTCP, 80)
	svc.Spec.Type = v1.ServiceTypeLoadBalancer
	// We need to turn affinity off for our LB distribution tests
	svc.Spec.SessionAffinity = v1.ServiceAffinityNone
	if tweak != nil {
		tweak(svc)
	}
	result, err := j.Client.CoreV1().Services(j.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create LoadBalancer Service %q: %w", svc.Name, err)
	}

	return j.sanityCheckService(result, v1.ServiceTypeLoadBalancer)
}
