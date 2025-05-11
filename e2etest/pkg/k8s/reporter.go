// SPDX-License-Identifier:Apache-2.0

package k8s

import (
	"log"
	"strings"
	"time"

	frrk8sv1beta1 "github.com/metallb/frr-k8s/api/v1beta1"
	"github.com/onsi/ginkgo/v2"
	"github.com/openshift-kni/k8sreporter"
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	metallbv1beta2 "go.universe.tf/metallb/api/v1beta2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func InitReporter(kubeconfig, path string, namespaces ...string) *k8sreporter.KubernetesReporter {
	// When using custom crds, we need to add them to the scheme
	addToScheme := func(s *runtime.Scheme) error {
		err := metallbv1beta1.AddToScheme(s)
		if err != nil {
			return err
		}
		err = metallbv1beta2.AddToScheme(s)
		if err != nil {
			return err
		}
		err = frrk8sv1beta1.AddToScheme(s)
		if err != nil {
			return err
		}

		return nil
	}

	// The namespaces we want to dump resources for (including pods and pod logs)
	dumpNamespace := func(ns string) bool {
		for _, n := range namespaces {
			if n == ns {
				return true
			}
		}

		switch {
		case strings.HasPrefix(ns, "l2"):
			return true
		case strings.HasPrefix(ns, "bgp"):
			return true
		}
		return false
	}

	// The list of CRDs we want to dump
	crds := []k8sreporter.CRData{
		{Cr: &metallbv1beta1.IPAddressPoolList{}},
		{Cr: &metallbv1beta2.BGPPeerList{}},
		{Cr: &metallbv1beta1.L2AdvertisementList{}},
		{Cr: &metallbv1beta1.BGPAdvertisementList{}},
		{Cr: &metallbv1beta1.BFDProfileList{}},
		{Cr: &metallbv1beta1.CommunityList{}},
		{Cr: &corev1.ServiceList{}},
		{Cr: &frrk8sv1beta1.FRRConfigurationList{}},
		{Cr: &frrk8sv1beta1.FRRNodeStateList{}},
		{Cr: &metallbv1beta1.ServiceBGPStatusList{}},
	}

	reporter, err := k8sreporter.New(kubeconfig, addToScheme, dumpNamespace, path, crds...)
	if err != nil {
		log.Fatalf("Failed to initialize the reporter %s", err)
	}
	return reporter
}

func DumpInfo(reporter *k8sreporter.KubernetesReporter, testName string) {
	testNameNoSpaces := strings.ReplaceAll(ginkgo.CurrentSpecReport().LeafNodeText, " ", "-")
	reporter.Dump(10*time.Minute, testNameNoSpaces)
}
