package main

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	vk "go.universe.tf/virtuakube"
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

// TODO notes:
//
//   Does net/http leak connections when there is a context timeout
//   during an uninterruptible Dial? Sometimes we seem to hit a wall
//   where SSH refuses to open more connections, implying we're
//   dropping some.
//
//   Need a better way to track, on a fine granularity, "expected
//   broken" tests, so we can verify that they don't start
//   unexpectedly passing, and can produce a compatibility matrix.
//   Should I write my own test framework? That seems fraught with
//   peril, but maybe something that wraps testing to provide some
//   extra smarts?

func TestBGP(t *testing.T) { testAll(t, testBGP) }
func testBGP(t *testing.T, u *vk.Universe) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	configureBGP(ctx, t, u)

	cluster := u.Cluster("cluster")
	client := u.VM("client")
	//	kube := cluster.KubernetesClient()

	// Create a service, and wait for it to get an IP.
	if err := cluster.ApplyManifest([]byte(bgpService)); err != nil {
		t.Fatalf("creating LB service: %v", err)
	}

	clusterURL := "http://10.249.0.1"
	localURL := "http://10.249.0.2"
	sharedURL1 := "http://10.249.0.3:80"
	sharedURL2 := "http://10.249.0.3:81"

	load := NewLoadGenerator(
		client,
		clusterURL,
		localURL,
		sharedURL1,
		sharedURL2,
	)
	defer load.Close()

	t.Run("cluster", func(t *testing.T) {
		var stats *Stats
		waitFor(ctx, t, func() error {
			stats = load.Stats(clusterURL, stats)

			if stats.Errors > 0 {
				return fmt.Errorf("%s still returning errors", clusterURL)
			}
			if stats.Pods() != 2 {
				return fmt.Errorf("%s has %d pods, waiting for 2", clusterURL, stats.Pods())
			}
			if err := stats.BalancedByPod(0.2); err != nil {
				return fmt.Errorf("%s %s", clusterURL, err)
			}
			return nil
		})

		if stats.Nodes() != 2 {
			t.Errorf("want 2 nodes processing traffic, got %d", stats.Nodes())
		}
		if err := stats.BalancedByNode(0.2); err != nil {
			t.Error(err)
		}

		// Client source IP assignment is broken in kube-proxy, in
		// that the masquerade IP is not stable. This is reproducible
		// with multiple network addons. Therefore we don't verify it
		// here.
	})

	t.Run("local", func(t *testing.T) {
		var stats *Stats
		waitFor(ctx, t, func() error {
			stats = load.Stats(localURL, stats)

			if stats.Errors > 0 {
				return fmt.Errorf("%s still returning errors", localURL)
			}
			if stats.Pods() != 2 {
				return fmt.Errorf("%s has %d pods, waiting for 2", localURL, stats.Pods())
			}
			if err := stats.BalancedByPod(0.2); err != nil {
				return fmt.Errorf("%s %s", localURL, err)
			}

			return nil
		})

		if stats.Nodes() != 2 {
			t.Errorf("want 2 nodes processing traffic, got %d", stats.Nodes())
		}
		if err := stats.BalancedByNode(0.2); err != nil {
			t.Error(err)
		}
		if t.Name() == "TestBGP/weave/local" {
			// Weave is broken and masquerades even
			// externalTrafficPolicy=Local traffic.
			return
		}

		if stats.Clients() != 1 {
			t.Errorf("want 1 client sending traffic, got %d", stats.Clients())
		}
		if err := stats.BalancedByClient(0.2); err != nil {
			t.Error(err)
		}
	})

	t.Run("shared", func(t *testing.T) {
		var stats1, stats2 *Stats
		waitFor(ctx, t, func() error {
			stats1 = load.Stats(sharedURL1, stats2)

			if stats1.Errors > 0 {
				return fmt.Errorf("%s still returning errors", sharedURL1)
			}
			if stats1.Pods() != 2 {
				return fmt.Errorf("%s has %d pods, waiting for 2", sharedURL1, stats1.Pods())
			}
			if err := stats1.BalancedByPod(0.2); err != nil {
				return fmt.Errorf("%s %s", sharedURL1, err)
			}

			stats2 = load.Stats(sharedURL2, stats2)

			if stats2.Errors > 0 {
				return fmt.Errorf("%s still returning errors", sharedURL2)
			}
			if stats2.Pods() != 2 {
				return fmt.Errorf("%s has %d pods, waiting for 2", sharedURL2, stats2.Pods())
			}
			if err := stats2.BalancedByPod(0.2); err != nil {
				return fmt.Errorf("%s %s", sharedURL2, err)
			}

			return nil
		})

		if stats1.Nodes() != 2 {
			t.Errorf("want 2 nodes processing traffic, got %d", stats1.Nodes())
		}
		if stats2.Nodes() != 2 {
			t.Errorf("want 2 nodes processing traffic, got %d", stats2.Nodes())
		}
		if err := stats1.BalancedByNode(0.2); err != nil {
			t.Error(err)
		}
		if err := stats2.BalancedByNode(0.2); err != nil {
			t.Error(err)
		}

		// Client source IP assignment is broken in kube-proxy, in
		// that the masquerade IP is not stable. This is reproducible
		// with multiple network addons. Therefore we don't verify it
		// here.
	})
}

// Helpers

// func waitForRoutes(ctx context.Context, t *testing.T, client *vk.VM, ip string, nexthops []*vk.VM) {
// 	waitFor(ctx, t, func() bool {
// 		bs, err := client.Run(fmt.Sprintf("birdc show route %s/32", ip))
// 		if err != nil {
// 			t.Fatal(err)
// 		}
// 		routes := string(bs)
// 		if !strings.Contains(routes, ip+"/32") {
// 			return false
// 		}
// 		if strings.Count(routes, "via ") != len(nexthops) {
// 			return false
// 		}
// 		for _, nexthop := range nexthops {
// 			if !strings.Contains(routes, nexthop.IPv4("net1").String()) {
// 				return false
// 			}
// 		}
// 		return true
// 	})
// }

// configureBGP installs a MetalLB configuration for BGP, and waits
// for peering to come up on the client.
func configureBGP(ctx context.Context, t *testing.T, u *vk.Universe) {
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

	waitFor(ctx, t, func() error {
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
		stilldown := []string{}
		for _, line := range strings.Split(string(bs), "\n")[2:] {
			if strings.TrimSpace(line) == "" {
				continue
			}
			fs := strings.Fields(line)
			if fs[3] != "up" {
				stilldown = append(stilldown, fs[0])
			}
		}

		if len(stilldown) > 0 {
			return fmt.Errorf("BGP sessions still down: %s", strings.Join(stilldown, ", "))
		}

		return nil
	})
}
