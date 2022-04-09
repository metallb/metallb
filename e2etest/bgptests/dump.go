// SPDX-License-Identifier:Apache-2.0

package bgptests

import (
	"fmt"

	"go.universe.tf/metallb/e2etest/pkg/executor"
	"go.universe.tf/metallb/e2etest/pkg/frr"
	"go.universe.tf/metallb/e2etest/pkg/k8s"
	"go.universe.tf/metallb/e2etest/pkg/metallb"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
)

func dumpBGPInfo(cs clientset.Interface, f *framework.Framework) {
	for _, c := range FRRContainers {
		address := c.Ipv4
		if address == "" {
			address = c.Ipv6
		}
		peerAddr := address + fmt.Sprintf(":%d", c.RouterConfig.BGPPort)
		dump, err := frr.RawDump(c, "/etc/frr/bgpd.conf", "/tmp/frr.log", "/etc/frr/daemons")
		framework.Logf("External frr dump for %s:%s\n%s\nerrors:%v", c.Name, peerAddr, dump, err)
	}

	speakerPods, err := metallb.SpeakerPods(cs)
	framework.ExpectNoError(err)
	for _, pod := range speakerPods {
		if len(pod.Spec.Containers) == 1 { // we dump only in case of frr
			break
		}
		podExec := executor.ForPod(pod.Namespace, pod.Name, "frr")
		dump, err := frr.RawDump(podExec, "/etc/frr/frr.conf", "/etc/frr/frr.log")
		framework.Logf("External frr dump for pod %s\n%s %v", pod.Name, dump, err)
	}
	k8s.DescribeSvc(f.Namespace.Name)
}
