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
	"go.universe.tf/e2etest/pkg/executor"
	"go.universe.tf/e2etest/pkg/frr"
	frrcontainer "go.universe.tf/e2etest/pkg/frr/container"
	"go.universe.tf/e2etest/pkg/metallb"
	corev1 "k8s.io/api/core/v1"
	clientset "k8s.io/client-go/kubernetes"
)

func dumpBGPInfo(basePath, testName string, cs clientset.Interface, namespace string, extraContainers ...*frrcontainer.FRR) {
	testPath := path.Join(basePath, strings.ReplaceAll(testName, " ", "-"))
	err := os.Mkdir(testPath, 0755)
	if err != nil && !errors.Is(err, os.ErrExist) {
		fmt.Fprintf(os.Stderr, "failed to create test dir: %v\n", err)
		return
	}

	summaryFile, err := logFileFor(testPath, "summary")
	if err != nil {
		ginkgo.GinkgoWriter.Printf("External frr dump, failed to open summary file %v", err)
		return
	}

	for _, c := range append(FRRContainers, extraContainers...) {
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
		writeSummaryForContainer(summaryFile, c)
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
		writeSummaryForSpeaker(summaryFile, pod)
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

func writeSummaryForSpeaker(summaryFile *os.File, s *corev1.Pod) {
	fmt.Fprintf(summaryFile, "Speaker %s running on node %s\n", s.Name, s.Spec.NodeName)
	exec := executor.ForPod(s.Name, s.Namespace, "frr")
	ipAddr, err := exec.Exec("bash", "-c", "ip address show")
	_, err = fmt.Fprint(summaryFile, ipAddr)
	if err != nil {
		ginkgo.GinkgoWriter.Printf("External frr dump for pod %s, failed to write to file %v", s.Name, err)
	}
}

func writeSummaryForContainer(summaryFile *os.File, c *frrcontainer.FRR) {
	fmt.Fprintf(summaryFile, "Container %s\n", c.Name)
	ipAddr, err := c.Exec("bash", "-c", "ip address show")
	_, err = fmt.Fprint(summaryFile, ipAddr)
	if err != nil {
		ginkgo.GinkgoWriter.Printf("External frr dump for container %s, failed to write to file %v", c.Name, err)
	}
}
