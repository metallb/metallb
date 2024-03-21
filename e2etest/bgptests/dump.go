// SPDX-License-Identifier:Apache-2.0

package bgptests

import (
	"errors"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.universe.tf/e2etest/pkg/frr"
	"go.universe.tf/e2etest/pkg/metallb"
	clientset "k8s.io/client-go/kubernetes"
)

func dumpBGPInfo(basePath, testName string, cs clientset.Interface, namespace string) {
	testPath := path.Join(basePath, strings.ReplaceAll(testName, " ", "-"))
	err := os.Mkdir(testPath, 0755)
	if err != nil && !errors.Is(err, os.ErrExist) {
		fmt.Fprintf(os.Stderr, "failed to create test dir: %v\n", err)
		return
	}

	for _, c := range FRRContainers {
		dump, err := frr.RawDump(c, "/etc/frr/bgpd.conf", "/tmp/frr.log", "/etc/frr/daemons")
		if err != nil {
			ginkgo.GinkgoWriter.Printf("External frr dump for container %s failed %v", c.Name, err)
			continue
		}
		f, err := logFileFor(testPath, fmt.Sprintf("frrdump-%s", c.Name))
		if err != nil {
			ginkgo.GinkgoWriter.Printf("External frr dump for container %s, failed to open file %v", c.Name, err)
			continue
		}
		fmt.Fprintf(f, "Dumping information for %s, local addresses: ipv4 - %s, ipv6 - %s\n", c.Name, c.Ipv4, c.Ipv6)
		_, err = fmt.Fprint(f, dump)
		if err != nil {
			ginkgo.GinkgoWriter.Printf("External frr dump for container %s, failed to write to file %v", c.Name, err)
			continue
		}
	}

	speakerPods, err := metallb.SpeakerPods(cs)
	Expect(err).NotTo(HaveOccurred())
	for _, pod := range speakerPods {
		if FRRProvider == nil { // we dump only in case of frr / frr-k8s
			break
		}
		podExec, err := FRRProvider.FRRExecutorFor(pod.Namespace, pod.Name)
		Expect(err).NotTo(HaveOccurred())

		dump, err := frr.RawDump(podExec, "/etc/frr/frr.conf", "/etc/frr/frr.log")
		if err != nil {
			ginkgo.GinkgoWriter.Printf("External frr dump for pod %s failed %v", pod.Name, err)
			continue
		}
		f, err := logFileFor(testPath, fmt.Sprintf("frrdump-%s", pod.Name))
		if err != nil {
			ginkgo.GinkgoWriter.Printf("External frr dump for pod %s, failed to open file %v", pod.Name, err)
			continue
		}
		fmt.Fprintf(f, "Dumping information for %s, local addresses: %s\n", pod.Name, pod.Status.PodIPs)
		_, err = fmt.Fprint(f, dump)
		if err != nil {
			ginkgo.GinkgoWriter.Printf("External frr dump for pod %s, failed to write to file %v", pod.Name, err)
			continue
		}
	}
}

func logFileFor(base string, kind string) (*os.File, error) {
	path := path.Join(base, kind) + ".log"
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	return f, nil
}
