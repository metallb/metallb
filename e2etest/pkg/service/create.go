// SPDX-License-Identifier:Apache-2.0

package service

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2eservice "k8s.io/kubernetes/test/e2e/framework/service"
)

var TestServicePort = 80

func CreateWithBackend(cs clientset.Interface, namespace string, jigName string, tweak ...func(svc *corev1.Service)) (*corev1.Service, *e2eservice.TestJig) {
	var svc *corev1.Service
	var err error

	jig := e2eservice.NewTestJig(cs, namespace, jigName)
	timeout := e2eservice.GetServiceLoadBalancerCreationTimeout(cs)
	svc, err = jig.CreateLoadBalancerService(timeout, func(svc *corev1.Service) {
		tweakServicePort(svc)
		for _, f := range tweak {
			f(svc)
		}
	})

	framework.ExpectNoError(err)
	_, err = jig.Run(func(rc *corev1.ReplicationController) {
		tweakRCPort(rc)
	})
	framework.ExpectNoError(err)
	return svc, jig
}

func Delete(cs clientset.Interface, svc *corev1.Service) {
	err := cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
	framework.ExpectNoError(err)
}

func tweakServicePort(svc *v1.Service) {
	if TestServicePort != 80 {
		// if TestServicePort is non default, then change service spec.
		svc.Spec.Ports[0].TargetPort = intstr.FromInt(int(TestServicePort))
	}
}

func tweakRCPort(rc *v1.ReplicationController) {
	if TestServicePort != 80 {
		// if TestServicePort is non default, then change pod's spec
		rc.Spec.Template.Spec.Containers[0].Args = []string{"netexec", fmt.Sprintf("--http-port=%d", TestServicePort), fmt.Sprintf("--udp-port=%d", TestServicePort)}
		rc.Spec.Template.Spec.Containers[0].ReadinessProbe.HTTPGet.Port = intstr.FromInt(int(TestServicePort))
	}
}
