// SPDX-License-Identifier:Apache-2.0

package metallb

import (
	"context"
	"fmt"
	"os"
	"slices"
	"time"

	"errors"

	. "github.com/onsi/gomega"
	"go.universe.tf/e2etest/pkg/k8s"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
)

var (
	ControllerLabelSelector = "component=controller"
	speakerLabelgSelector   = "component=speaker"
)

func init() {
	if v, ok := os.LookupEnv("CONTROLLER_SELECTOR"); ok {
		ControllerLabelSelector = v
	}

	if v, ok := os.LookupEnv("SPEAKER_SELECTOR"); ok {
		speakerLabelgSelector = v
	}
}

func RestartSpeakerPods(cs clientset.Interface) error {
	pods, err := SpeakerPods(cs)
	if err != nil {
		return err
	}
	oldNames := []string{}
	for _, p := range pods {
		oldNames = append(oldNames, p.Name)
		err := cs.CoreV1().Pods(Namespace).Delete(context.Background(), p.Name, metav1.DeleteOptions{})
		if err != nil {
			return err
		}
	}

	return wait.PollUntilContextTimeout(context.Background(), 100*time.Millisecond, 10*time.Second, false, func(context.Context) (bool, error) {
		npods, err := SpeakerPods(cs)
		if err != nil {
			return false, err
		}
		for _, p := range npods {
			if slices.Contains(oldNames, p.Name) {
				return false, nil
			}
		}
		return true, nil
	})
}

// SpeakerPods returns the set of pods running the speakers.
func SpeakerPods(cs clientset.Interface) ([]*corev1.Pod, error) {
	speakers, err := cs.CoreV1().Pods(Namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: speakerLabelgSelector,
	})
	if err != nil {
		return nil, errors.Join(err, errors.New("failed to fetch speaker pods"))
	}
	if len(speakers.Items) == 0 {
		return nil, errors.New("no speaker pods found")
	}
	speakerPods := make([]*corev1.Pod, 0)
	for _, item := range speakers.Items {
		i := item
		speakerPods = append(speakerPods, &i)
	}
	return speakerPods, nil
}

// ControllerPod returns the metallb controller pod.
func ControllerPod(cs clientset.Interface) (*corev1.Pod, error) {
	pods, err := cs.CoreV1().Pods(Namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: ControllerLabelSelector,
	})
	if err != nil {
		Expect(err).NotTo(HaveOccurred(), "failed to fetch controller pods")
	}
	if len(pods.Items) != 1 {
		return nil, fmt.Errorf("expected one controller pod, found %d", len(pods.Items))
	}
	return &pods.Items[0], nil
}

// SpeakerPodInNode returns the speaker pod running in the given node.
func SpeakerPodInNode(cs clientset.Interface, node string) (*corev1.Pod, error) {
	speakerPods, err := SpeakerPods(cs)
	if err != nil {
		return nil, err
	}
	for _, pod := range speakerPods {
		if pod.Spec.NodeName == node {
			return pod, nil
		}
	}
	return nil, fmt.Errorf("no speaker pod run in the node %s", node)
}

// RestartController restarts metallb's controller pod and waits for it to be running and ready.
func RestartController(cs clientset.Interface) {
	controllerPod, err := ControllerPod(cs)
	Expect(err).NotTo(HaveOccurred())

	err = cs.CoreV1().Pods(controllerPod.Namespace).Delete(context.TODO(), controllerPod.Name, metav1.DeleteOptions{})
	Expect(err).NotTo(HaveOccurred())

	err = wait.PollImmediate(5*time.Second, 3*time.Minute, func() (bool, error) {
		pod, err := ControllerPod(cs)
		if err != nil {
			return false, nil
		}
		if controllerPod.Name == pod.Name {
			return false, nil
		}
		isReady := (pod.Status.Phase == corev1.PodRunning) && (k8s.PodIsReady(pod))

		return isReady, nil
	})
	Expect(err).NotTo(HaveOccurred())
}

// FRRK8SPods returns the set of pods related to FRR-K8s.
func FRRK8SPods(cs clientset.Interface, namespace string) ([]*corev1.Pod, error) {
	pods, err := cs.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: "app=frr-k8s",
	})
	if err != nil {
		return nil, errors.Join(err, errors.New("Failed to fetch frr-k8s pods"))
	}
	if len(pods.Items) == 0 {
		return nil, errors.New("No frr-k8s pods found")
	}
	res := make([]*corev1.Pod, 0)
	for _, item := range pods.Items {
		i := item
		res = append(res, &i)
	}
	return res, nil
}
