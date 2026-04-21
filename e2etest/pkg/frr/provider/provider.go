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
	cl              *clientset.Clientset
	frrk8sNamespace string
}

// NewFRRK8SMode returns a provider for a deployment using "frr-k8s" as its BGP mode.
// In this mode the frr instance for a node is a sidecar container of the frr-k8s pod.
func NewFRRK8SMode(r *rest.Config, namespace string) (Provider, error) {
	cl, err := clientset.NewForConfig(r)
	if err != nil {
		return nil, err
	}

	return frrk8sModeProvider{cl: cl, frrk8sNamespace: namespace}, nil
}

// frrK8SPodForSpeaker resolves the frr-k8s pod that shares a node with the named speaker pod.
func (f frrk8sModeProvider) frrK8SPodForSpeaker(speakerNS, speakerName string) (*corev1.Pod, error) {
	speakerPods, err := metallb.SpeakerPods(f.cl)
	if err != nil {
		return nil, err
	}

	var speaker *corev1.Pod
	for _, p := range speakerPods {
		if p.Namespace == speakerNS && p.Name == speakerName {
			speaker = p
			break
		}
	}
	if speaker == nil {
		return nil, fmt.Errorf("speaker %s/%s not found", speakerNS, speakerName)
	}

	frrk8sPods, err := metallb.FRRK8SPods(f.cl, f.frrk8sNamespace)
	if err != nil {
		return nil, err
	}

	for _, fp := range frrk8sPods {
		if fp.Spec.NodeName == speaker.Spec.NodeName {
			return fp, nil
		}
	}

	return nil, fmt.Errorf("speaker %s/%s on node %s does not have a matching frr-k8s", speaker.Namespace, speaker.Name, speaker.Spec.NodeName)
}

func (f frrk8sModeProvider) FRRExecutorFor(ns, name string) (executor.Executor, error) {
	frrk8s, err := f.frrK8SPodForSpeaker(ns, name)
	if err != nil {
		return nil, err
	}

	return executor.ForPod(frrk8s.Namespace, frrk8s.Name, "frr"), nil
}

func (f frrk8sModeProvider) BGPMetricsPodFor(ns, name string) (*corev1.Pod, string, error) {
	p, err := f.frrK8SPodForSpeaker(ns, name)
	if err != nil {
		return nil, "", err
	}

	return p, "frrk8s", nil
}

func (f frrk8sModeProvider) FRRK8sBased() bool {
	return true
}
