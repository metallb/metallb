// SPDX-License-Identifier:Apache-2.0

package service

import (
	"context"
	"fmt"

	. "github.com/onsi/gomega"
	jigservice "go.universe.tf/e2etest/pkg/jigservice"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	clientset "k8s.io/client-go/kubernetes"
)

var (
	TestServicePort = 80
)

func CreateWithBackend(cs clientset.Interface, namespace string, jigName string, tweak ...func(svc *corev1.Service)) (*corev1.Service, *jigservice.TestJig) {
	return CreateWithBackendPort(cs, namespace, jigName, TestServicePort, tweak...)
}

func CreateWithBackendPort(cs clientset.Interface, namespace string, jigName string, port int, tweak ...func(svc *corev1.Service)) (*corev1.Service, *jigservice.TestJig) {
	var svc *corev1.Service
	var err error

	jig := jigservice.NewTestJig(cs, namespace, jigName)
	svc, err = jig.CreateLoadBalancerService(context.TODO(), func(svc *corev1.Service) {
		tweakServicePort(svc, port)
		for _, f := range tweak {
			f(svc)
		}
	})

	Expect(err).NotTo(HaveOccurred())
	_, err = jig.Run(context.TODO(), func(rc *corev1.ReplicationController) {
		if port != 0 {
			tweakRCPort(rc, port)
		}
	})
	Expect(err).NotTo(HaveOccurred())
	return svc, jig
}

func Delete(cs clientset.Interface, svc *corev1.Service) {
	err := cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
	Expect(err).NotTo(HaveOccurred())
}

func tweakServicePort(svc *corev1.Service, port int) {
	svc.Spec.Ports[0].TargetPort = intstr.FromInt(port)
}

func tweakRCPort(rc *corev1.ReplicationController, port int) {
	rc.Spec.Template.Spec.Containers[0].Args = []string{"netexec", fmt.Sprintf("--http-port=%d", port), fmt.Sprintf("--udp-port=%d", port)}
	rc.Spec.Template.Spec.Containers[0].ReadinessProbe.HTTPGet.Port = intstr.FromInt(port)
}
