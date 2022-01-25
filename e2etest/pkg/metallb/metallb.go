// SPDX-License-Identifier:Apache-2.0

package metallb

import (
	"context"
	"fmt"
	"os"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
)

var (
	controllerLabelSelector = "app.kubernetes.io/component=controller"
	speakerLabelgSelector   = "app.kubernetes.io/component=speaker"
)

func init() {
	if v, ok := os.LookupEnv("CONTROLLER_SELECTOR"); ok {
		controllerLabelSelector = v
	}

	if v, ok := os.LookupEnv("SPEAKER_SELECTOR"); ok {
		speakerLabelgSelector = v
	}
}

// SpeakerPods returns the set of pods running the speakers.
func SpeakerPods(cs clientset.Interface) ([]*corev1.Pod, error) {
	speakers, err := cs.CoreV1().Pods(Namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: speakerLabelgSelector,
	})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to fetch speaker pods")
	}
	if len(speakers.Items) == 0 {
		return nil, errors.New("No speaker pods found")
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
		LabelSelector: controllerLabelSelector,
	})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to fetch speaker pods")
	}
	if len(pods.Items) != 1 {
		return nil, fmt.Errorf("Expected one controller pod, found %d", len(pods.Items))
	}
	return &pods.Items[0], nil
}
