// SPDX-License-Identifier:Apache-2.0

package provider

import (
	"fmt"

	"go.universe.tf/e2etest/pkg/executor"
	"go.universe.tf/e2etest/pkg/metallb"
	corev1 "k8s.io/api/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Provider interface {
	// FRRExecutorFor returns an executor for the frr instance corresponding to the given speaker.
	FRRExecutorFor(speakerNamespace, speakerName string) (executor.Executor, error)

	// BGPMetricsPodFor returns the pod object to be scraped for FRR BGP/BFD metrics corresponding
	// to the given speaker. It also returns the metric prefix to expect when scraping the pod directly.
	BGPMetricsPodFor(speakerNamespace, speakerName string) (pod *corev1.Pod, metricsPrefix string, err error)

	// FRRK8sBased tells if the given provider is based on frrk8s
	FRRK8sBased() bool
}

type frrModeProvider struct {
	cl *clientset.Clientset
}

// NewFRRMode returns a provider for a deployment using "frr" as its BGP mode.
// In this mode the frr instance for a node is a sidecar container of the speaker pod.
func NewFRRMode(r *rest.Config) (Provider, error) {
	cl, err := clientset.NewForConfig(r)
	if err != nil {
		return nil, err
	}

	return frrModeProvider{cl: cl}, nil
}

func (f frrModeProvider) FRRExecutorFor(ns, name string) (executor.Executor, error) {
	speakerPods, err := metallb.SpeakerPods(f.cl)
	if err != nil {
		return nil, err
	}

	speakers := map[string]*corev1.Pod{}
	for _, p := range speakerPods {
		speakers[p.Name] = p
	}

	_, ok := speakers[name]
	if !ok {
		return nil, fmt.Errorf("speakers %s/%s not found in known speakers %s", ns, name, speakers)
	}

	return executor.ForPod(ns, name, "frr"), nil
}

func (f frrModeProvider) BGPMetricsPodFor(ns, name string) (*corev1.Pod, string, error) {
	speakerPods, err := metallb.SpeakerPods(f.cl)
	if err != nil {
		return nil, "", err
	}

	speakers := map[string]*corev1.Pod{}
	for _, p := range speakerPods {
		speakers[p.Name] = p
	}

	p, ok := speakers[name]
	if !ok {
		return nil, "", fmt.Errorf("speakers %s/%s not found in in known speakers %v", ns, name, speakers)
	}

	return p, "metallb", nil
}

func (f frrModeProvider) FRRK8sBased() bool {
	return false
}

type frrk8sModeProvider struct {
	frrk8sPodForSpeakerPod map[string]*corev1.Pod
}

// NewFRRK8SMode returns a provider for a deployment using "frr-k8s" as its BGP mode.
// In this mode the frr instance for a node is a sidecar container of the frr-k8s pod.
func NewFRRK8SMode(r *rest.Config, namespace string) (Provider, error) {
	cl, err := clientset.NewForConfig(r)
	if err != nil {
		return nil, err
	}

	speakerPods, err := metallb.SpeakerPods(cl)
	if err != nil {
		return nil, err
	}

	frrk8sPods, err := metallb.FRRK8SPods(cl, namespace)
	if err != nil {
		return nil, err
	}

	frrK8SForSpeaker := map[string]*corev1.Pod{}
	for _, s := range speakerPods {
		found := false
		for _, f := range frrk8sPods {
			if s.Spec.NodeName == f.Spec.NodeName {
				frrK8SForSpeaker[s.Name] = f
				found = true
			}
		}
		if !found {
			return nil, fmt.Errorf("speaker %s/%s on node %s does not have a matching frr-k8s", s.Namespace, s.Name, s.Spec.NodeName)
		}
	}

	res := frrk8sModeProvider{
		frrk8sPodForSpeakerPod: frrK8SForSpeaker,
	}

	return res, nil
}

func (f frrk8sModeProvider) FRRExecutorFor(ns, name string) (executor.Executor, error) {
	frrk8s, ok := f.frrk8sPodForSpeakerPod[name]
	if !ok {
		return nil, fmt.Errorf("speaker %s/%s does not have a matching frr-k8s", ns, name)
	}

	return executor.ForPod(frrk8s.Namespace, frrk8s.Name, "frr"), nil
}

func (f frrk8sModeProvider) BGPMetricsPodFor(ns, name string) (*corev1.Pod, string, error) {
	p, ok := f.frrk8sPodForSpeakerPod[name]
	if !ok {
		return nil, "", fmt.Errorf("speakers %s/%s not found in map %v", ns, name, f.frrk8sPodForSpeakerPod)
	}

	return p, "frrk8s", nil
}

func (f frrk8sModeProvider) FRRK8sBased() bool {
	return true
}
