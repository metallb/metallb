package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	vk "go.universe.tf/virtuakube"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const bgpService = `
apiVersion: v1
kind: Service
metadata:
  name: mirror-cluster
spec:
  ports:
  - port: 80
    targetPort: 8080
  selector:
    app: mirror
  type: LoadBalancer
  loadBalancerIP: 10.249.0.1
  externalTrafficPolicy: Cluster
---
apiVersion: v1
kind: Service
metadata:
  name: mirror-local
spec:
  ports:
  - port: 80
    targetPort: 8080
  selector:
    app: mirror
  type: LoadBalancer
  loadBalancerIP: 10.249.0.2
  externalTrafficPolicy: Local
---
apiVersion: v1
kind: Service
metadata:
  name: mirror-shared1
  annotations:
    metallb.universe.tf/allow-shared-ip: mirror
spec:
  ports:
  - port: 80
    targetPort: 8080
  selector:
    app: mirror
  type: LoadBalancer
  loadBalancerIP: 10.249.0.3
---
apiVersion: v1
kind: Service
metadata:
  name: mirror-shared2
  annotations:
    metallb.universe.tf/allow-shared-ip: mirror
spec:
  ports:
  - port: 81
    targetPort: 8081
  selector:
    app: mirror
  type: LoadBalancer
  loadBalancerIP: 10.249.0.3
`

func TestBGP(t *testing.T) { testAll(t, testBGP) }
func testBGP(t *testing.T, u *vk.Universe) {
	configureBGP(t, u)

	cluster := u.Cluster("cluster")
	client := u.VM("client")
	kube := cluster.KubernetesClient()

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	// Create a service, and wait for it to get an IP.
	if err := cluster.ApplyManifest([]byte(bgpService)); err != nil {
		t.Fatalf("creating LB service: %v", err)
	}
	waitForAllocation(ctx, t, kube, "mirror-cluster", "10.249.0.1", 2)
	waitForAllocation(ctx, t, kube, "mirror-local", "10.249.0.2", 2)
	waitForAllocation(ctx, t, kube, "mirror-shared1", "10.249.0.3", 2)
	waitForAllocation(ctx, t, kube, "mirror-shared2", "10.249.0.3", 2)

	waitForRoutes(ctx, t, client, "10.249.0.1", append(cluster.Nodes(), cluster.Controller()))
	waitForRoutes(ctx, t, client, "10.249.0.2", append(cluster.Nodes(), cluster.Controller()))
	waitForRoutes(ctx, t, client, "10.249.0.3", append(cluster.Nodes(), cluster.Controller()))

	// From the client, probe the service IP. We should get a
	// successful response.
	waitForService(ctx, t, client, "http://10.249.0.1")
	waitForService(ctx, t, client, "http://10.249.0.2")
	waitForService(ctx, t, client, "http://10.249.0.3")
	waitForService(ctx, t, client, "http://10.249.0.3:81")

	// Traffic should be evenly distributed across the 2 backend pods.
	checkServiceBalance(ctx, t, client, "http://10.249.0.1")
	checkServiceBalance(ctx, t, client, "http://10.249.0.2")
	checkServiceBalance(ctx, t, client, "http://10.249.0.3")
	checkServiceBalance(ctx, t, client, "http://10.249.0.3:81")
}

// Helpers

func waitForAllocation(ctx context.Context, t *testing.T, kube *kubernetes.Clientset, svcname, ip string, numEndpoints int) {
	t.Helper()
	waitFor(ctx, t, func() bool {
		svc, err := kube.CoreV1().Services("default").Get(svcname, metav1.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if len(svc.Status.LoadBalancer.Ingress) != 1 {
			return false
		}
		if svc.Status.LoadBalancer.Ingress[0].IP != ip {
			t.Fatalf("unexpected IP %q allocated", svc.Status.LoadBalancer.Ingress[0].IP)
		}

		eps, err := kube.CoreV1().Endpoints("default").Get(svcname, metav1.GetOptions{})
		if len(eps.Subsets) != 1 {
			t.Fatal("malformed endpoints")
		}
		if len(eps.Subsets[0].Addresses) != numEndpoints {
			return false
		}
		return true
	})
}

func waitForRoutes(ctx context.Context, t *testing.T, client *vk.VM, ip string, nexthops []*vk.VM) {
	waitFor(ctx, t, func() bool {
		bs, err := client.Run(fmt.Sprintf("birdc show route %s/32", ip))
		if err != nil {
			t.Fatal(err)
		}
		routes := string(bs)
		if !strings.Contains(routes, ip+"/32") {
			return false
		}
		if strings.Count(routes, "via ") != len(nexthops) {
			return false
		}
		for _, nexthop := range nexthops {
			if !strings.Contains(routes, nexthop.IPv4("net1").String()) {
				return false
			}
		}
		return true
	})
}

// waitForService tries uses vm to fetch from ip until 10 requests in
// a row succeed, or the context times out.
func waitForService(ctx context.Context, t *testing.T, vm *vk.VM, url string) {
	transport := &http.Transport{
		Dial: func(network, addr string) (net.Conn, error) {
			return vm.Dial(network, addr)
		},
		DisableKeepAlives: true,
	}
	waitFor(ctx, t, func() bool {
		for i := 0; i < 10; i++ {
			ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
			defer cancel()
			req, err := http.NewRequest("GET", url, nil)
			if err != nil {
				t.Fatal(err)
			}
			req = req.WithContext(ctx)
			resp, err := transport.RoundTrip(req)
			if err != nil {
				return false
			}
			resp.Body.Close()
		}
		return true
	})
}

func checkServiceBalance(ctx context.Context, t *testing.T, vm *vk.VM, url string) {
	hits := map[string]int{}
	runs := 200
	transport := &http.Transport{
		Dial: func(network, addr string) (net.Conn, error) {
			return vm.Dial(network, addr)
		},
		DisableKeepAlives: true,
	}
	for i := 0; i < runs; i++ {
		ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		defer cancel()
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			t.Fatal(err)
		}
		req = req.WithContext(ctx)
		resp, err := transport.RoundTrip(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		bs, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		node := strings.Split(string(bs), "\n")[0]
		hits[node]++
	}
	perNode := float64(runs) / float64(len(hits))
	for node, n := range hits {
		t.Logf("node %q got %d hits", node, n)
		fraction := (float64(n) / perNode) - 1
		if fraction < 0 {
			fraction = -fraction
		}
		if fraction > 0.2 {
			t.Errorf("Traffic to node %q is %f%% out from the ideal balance, want <=20%%", node, fraction*100)
		}
	}
}

func waitFor(ctx context.Context, t *testing.T, test func() bool) {
	t.Helper()
	for {
		select {
		case <-ctx.Done():
			t.Fatalf("timed out")
		default:
		}

		if test() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// configureBGP installs a MetalLB configuration for BGP, and waits
// for peering to come up on the client.
func configureBGP(t *testing.T, u *vk.Universe) {
	cluster := u.Cluster("cluster")
	client := u.VM("client")

	cfg := []byte(fmt.Sprintf(`
apiVersion: v1
kind: ConfigMap
metadata:
  namespace: metallb-system
  name: config
data:
  config: |
    peers:
    - peer-address: %s
      peer-asn: 64513
      my-asn: 64512
    address-pools:
    - name: default
      protocol: bgp
      addresses:
      - 10.249.0.0/24
      avoid-buggy-ips: true
`, client.IPv4("net1")))
	if err := cluster.ApplyManifest(cfg); err != nil {
		t.Fatalf("Applying MetalLB configuration: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	var stilldown []string
	for ctx.Err() == nil {
		stilldown = []string{}
		bs, err := client.Run("birdc show protocol")
		if err != nil {
			t.Fatalf("running birdc failed: %v", err)
		}

		// First 2 lines are a header, skip that. All other protocols
		// should be in state "up"
		//
		// Sample birdc output:
		//   BIRD 1.6.3 ready.
		//   name     proto    table    state  since       info
		//   kernel1  Kernel   master   up     17:53:32
		//   device1  Device   master   up     17:53:32
		//   direct1  Direct   master   up     17:53:32
		//   controller BGP      master   start  17:53:32    Passive
		//   node0    BGP      master   start  17:53:32    Passive
		for _, line := range strings.Split(string(bs), "\n")[2:] {
			if strings.TrimSpace(line) == "" {
				continue
			}
			fs := strings.Fields(line)
			if fs[3] != "up" {
				stilldown = append(stilldown, fs[0])
			}
		}

		if len(stilldown) == 0 {
			break
		}
	}
	if len(stilldown) > 0 {
		t.Fatalf("Some BGP sessions did not establish: %s", strings.Join(stilldown, ", "))
	}
}
