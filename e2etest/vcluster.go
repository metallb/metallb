package main

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	vk "go.universe.tf/virtuakube"
)

const cachedUniverse = `../e2etest-cached-universe`

var mkUniverseCmd = &cobra.Command{
	Use:   "mkuniverse",
	Short: "build the multiverse used for tests",
	Args:  cobra.NoArgs,
	Run: func(*cobra.Command, []string) {
		if err := mkUniverse(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	},
}

var mkUniverseSteps []string

func init() {
	rootCmd.AddCommand(mkUniverseCmd)
	mkUniverseCmd.Flags().StringSliceVar(&mkUniverseSteps, "steps", []string{}, "steps to forcibly regenerate")
}

func testAll(t *testing.T, f func(t *testing.T, u *vk.Universe)) {
	for _, base := range []string{"calico", "flannel", "weave"} {
		t.Run(base, func(t *testing.T) {
			cfg := &vk.UniverseConfig{}
			if os.Getenv("E2E_VERBOSE") != "" {
				cfg.CommandLog = os.Stdout
			}
			if os.Getenv("E2E_USERSPACE") != "" {
				cfg.NoAcceleration = true
			}

			u, err := vk.Open(cachedUniverse, base, cfg)
			if err != nil {
				t.Fatalf("Opening universe at snapshot %q: %v", base, err)
			}
			defer u.Close()

			c := u.Cluster("cluster")

			err = c.PushImages(
				"metallb/controller:e2e-amd64",
				"metallb/speaker:e2e-amd64",
				"metallb/e2etest-mirror-server:e2e-amd64",
			)
			if err != nil {
				t.Fatal(err)
			}

			bs, err := ioutil.ReadFile("../manifests/metallb.yaml")
			if err != nil {
				t.Fatalf("reading metallb manifest: %v", err)
			}
			manifest := string(bs)
			manifest = strings.Replace(manifest, "metallb/speaker:master", "metallb/speaker:e2e-amd64", -1)
			manifest = strings.Replace(manifest, "metallb/controller:master", "metallb/controller:e2e-amd64", -1)
			manifest = strings.Replace(manifest, "PullPolicy: Always", "PullPolicy: IfNotPresent", -1)
			if err := c.ApplyManifest([]byte(manifest)); err != nil {
				t.Fatalf("applying metallb manifest: %v", err)
			}

			bs, err = ioutil.ReadFile("manifests/mirror-server.yaml")
			if err != nil {
				t.Fatalf("getting registry manifest bytes: %v", err)
			}
			if err := c.ApplyManifest(bs); err != nil {
				t.Fatalf("deploying mirror server: %v", err)
			}

			f(t, u)
		})
	}
}

func mkUniverse() error {
	buildStep("", mkImage, "image")
	buildStep("image", mkCluster, "cluster")
	buildStep("cluster", mkClusterNet("calico"), "calico")
	buildStep("cluster", mkClusterNet("weave"), "weave")
	buildStep("cluster", mkClusterNet("flannel"), "flannel")

	// TODO: IPVS mode? Can we build separate clusters for that?

	return nil
}

func buildStep(basesnap string, do func(*vk.Universe) error, resultsnap string) {
	if err := buildStepInternal(basesnap, do, resultsnap); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func buildStepInternal(basesnap string, do func(*vk.Universe) error, resultsnap string) error {
	cfg := &vk.UniverseConfig{
		CommandLog: os.Stdout,
	}

	if os.Getenv("E2E_USERSPACE") != "" {
		cfg.NoAcceleration = true
	}

	fmt.Printf("Running build step %q\n", resultsnap)
	defer fmt.Printf("Build step %q done\n", resultsnap)

	var universe *vk.Universe
	_, err := os.Stat(cachedUniverse)
	if os.IsNotExist(err) {
		universe, err = vk.Create(cachedUniverse, cfg)
	} else if err != nil {
		return fmt.Errorf("stat of universe: %v", err)
	} else {
		universe, err = vk.Open(cachedUniverse, basesnap, cfg)
	}
	if err != nil {
		return fmt.Errorf("opening universe: %v", err)
	}
	defer universe.Close()

	if !hasSnapshot(universe, basesnap) {
		return fmt.Errorf("universe doesn't have base snapshot %q", basesnap)
	}

	if hasSnapshot(universe, resultsnap) {
		return nil
	}

	if err := do(universe); err != nil {
		return fmt.Errorf("building %q from %q failed: %v", resultsnap, basesnap, err)
	}

	if err := universe.Save(resultsnap); err != nil {
		return fmt.Errorf("saving result snapshot %q: %v", resultsnap, err)
	}

	return nil
}

func hasSnapshot(u *vk.Universe, snap string) bool {
	for _, name := range u.Snapshots() {
		if name == snap {
			return true
		}
	}
	return false
}

func mkImage(u *vk.Universe) error {
	cfg := &vk.ImageConfig{
		Name: "image",
		CustomizeFuncs: []vk.ImageCustomizeFunc{
			vk.CustomizeInstallK8s,
			vk.CustomizePreloadK8sImages,
			func(v *vk.VM) error {
				return v.RunMultiple(
					"apt-get install bird",
					"systemctl disable bird",
					"systemctl disable bird6",
				)
			},
		},
	}

	if err := u.NewImage(cfg); err != nil {
		return fmt.Errorf("creating VM base image: %v", err)
	}

	return nil
}

func mkCluster(u *vk.Universe) error {
	// Two virtual networks, for testing some L2 mode things.
	if err := u.NewNetwork(&vk.NetworkConfig{Name: "net1"}); err != nil {
		return fmt.Errorf("creating net1: %v", err)
	}
	if err := u.NewNetwork(&vk.NetworkConfig{Name: "net2"}); err != nil {
		return fmt.Errorf("creating net2: %v", err)
	}

	// Kubernetes cluster.
	clusterCfg := &vk.ClusterConfig{
		Name:     "cluster",
		NumNodes: 1,
		VMConfig: &vk.VMConfig{
			Image:     "image",
			MemoryMiB: 1024,
			Networks:  []string{"net1", "net2"},
		},
	}
	cluster, err := u.NewCluster(clusterCfg)
	if err != nil {
		return fmt.Errorf("creating cluster: %v", err)
	}
	if err := cluster.Start(); err != nil {
		return fmt.Errorf("starting cluster: %v", err)
	}

	// Client VM that lives outside the k8s cluster.
	vmCfg := &vk.VMConfig{
		Name:      "client",
		Image:     "image",
		MemoryMiB: 1024,
		Networks:  []string{"net1", "net2"},
	}
	client, err := u.NewVM(vmCfg)
	if err != nil {
		return fmt.Errorf("creating client VM: %v", err)
	}
	if err := client.Start(); err != nil {
		return fmt.Errorf("starting client VM: %v", err)
	}

	var bird4, bird6 bytes.Buffer
	fmt.Fprintf(&bird4, birdBaseCfg, client.IPv4("net1"))
	fmt.Fprintf(&bird6, birdBaseCfg, client.IPv4("net1"))

	fmt.Fprintf(&bird4, birdPeerCfg, "controller", cluster.Controller().IPv4("net1"))
	fmt.Fprintf(&bird6, birdPeerCfg, "controller", cluster.Controller().IPv6("net1"))

	for i, vm := range cluster.Nodes() {
		fmt.Fprintf(&bird4, birdPeerCfg, "node"+strconv.Itoa(i), vm.IPv4("net1"))
		fmt.Fprintf(&bird6, birdPeerCfg, "node"+strconv.Itoa(i), vm.IPv6("net1"))
	}

	if err := client.WriteFile("/etc/bird/bird.conf", bird4.Bytes()); err != nil {
		return err
	}
	if err := client.WriteFile("/etc/bird/bird6.conf", bird6.Bytes()); err != nil {
		return err
	}
	err = client.RunMultiple(
		"systemctl enable bird",
		"systemctl enable bird6",
		"systemctl restart bird",
		"systemctl restart bird6",
	)
	if err != nil {
		return err
	}

	return nil
}

func mkClusterNet(addon string) func(*vk.Universe) error {
	return func(u *vk.Universe) error {
		c := u.Cluster("cluster")

		bs, err := ioutil.ReadFile(fmt.Sprintf("manifests/%s.yaml", addon))
		if err != nil {
			return fmt.Errorf("getting network addon %q: %v", addon, err)
		}

		if err := c.ApplyManifest(bs); err != nil {
			return fmt.Errorf("installing network addon %q: %v", addon, err)
		}

		if err := c.WaitFor(context.Background(), c.NodesReady); err != nil {
			return fmt.Errorf("waiting for nodes to become ready: %v", err)
		}

		// Wait for all deployments to schedule, which signals that
		// the network addon's finished setting up.
		err = c.WaitFor(context.Background(), func() (bool, error) {
			deploys, err := c.KubernetesClient().AppsV1().Deployments("").List(metav1.ListOptions{})
			if err != nil {
				return false, err
			}
			for _, deploy := range deploys.Items {
				if deploy.Status.AvailableReplicas != deploy.Status.Replicas {
					return false, nil
				}
			}
			return true, nil
		})
		if err != nil {
			return err
		}

		return nil
	}
}

func buildUniverse() error {
	universe, err := vk.Open(cachedUniverse, "", nil)
	if err != nil {
		return fmt.Errorf("opening universe: %v", err)
	}
	defer universe.Close()

	vmCfg := &vk.VMConfig{
		Name:  "client",
		Image: "base",
	}

	clusterCfg := &vk.ClusterConfig{
		Name:     "cluster",
		NumNodes: 2,
		VMConfig: vmCfg,
	}

	cluster, err := universe.NewCluster(clusterCfg)
	if err != nil {
		return fmt.Errorf("creating cluster: %v", err)
	}
	if err := cluster.Start(); err != nil {
		return fmt.Errorf("starting cluster: %v", err)
	}

	client, err := universe.NewVM(vmCfg)
	if err != nil {
		return fmt.Errorf("creating client VM: %v", err)
	}
	if err := client.Start(); err != nil {
		return fmt.Errorf("starting client VM: %v", err)
	}

	var bird4, bird6 bytes.Buffer
	fmt.Fprintf(&bird4, birdBaseCfg, client.IPv4(""))
	fmt.Fprintf(&bird6, birdBaseCfg, client.IPv4(""))

	fmt.Fprintf(&bird4, birdPeerCfg, "controller", cluster.Controller().IPv4(""))
	fmt.Fprintf(&bird6, birdPeerCfg, "controller", cluster.Controller().IPv6(""))

	for i, vm := range cluster.Nodes() {
		fmt.Fprintf(&bird4, birdPeerCfg, "node"+strconv.Itoa(i), vm.IPv4(""))
		fmt.Fprintf(&bird6, birdPeerCfg, "node"+strconv.Itoa(i), vm.IPv6(""))
	}

	if err := client.WriteFile("/etc/bird/bird.conf", bird4.Bytes()); err != nil {
		return err
	}
	if err := client.WriteFile("/etc/bird/bird6.conf", bird6.Bytes()); err != nil {
		return err
	}
	err = client.RunMultiple(
		"systemctl restart bird",
		"systemctl restart bird6",
	)
	if err != nil {
		return err
	}

	cmd := exec.Command(
		"make", "push-images",
		"REGISTRY=localhost:"+strconv.Itoa(cluster.Controller().ForwardedPort(30000)),
		"TAG=e2e",
		"ARCH=amd64",
		"BINARIES=e2etest-mirror-server",
	)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pushing mirror server image: %v", err)
	}

	cmd = exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = bytes.NewBufferString(mirrorServerManifest)
	cmd.Env = []string{"KUBECONFIG=" + cluster.Kubeconfig()}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("starting mirror server daemonset: %v", err)
	}

	err = cluster.WaitFor(context.Background(), func() (bool, error) {
		ds, err := cluster.KubernetesClient().AppsV1().DaemonSets("default").Get("mirror", metav1.GetOptions{})
		if err != nil {
			if kerr.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}

		if int(ds.Status.DesiredNumberScheduled) != clusterCfg.NumNodes+1 {
			return false, err
		}

		if ds.Status.NumberAvailable != ds.Status.DesiredNumberScheduled {
			return false, err
		}

		return true, nil
	})
	if err != nil {
		return fmt.Errorf("waiting for mirror server to start: %v", err)
	}

	if err := universe.Save(""); err != nil {
		return fmt.Errorf("saving universe: %v", err)
	}

	return nil
}

const (
	birdBaseCfg = `
router id %s;

protocol kernel {
  persist;
  merge paths;
  scan time 60;
  import none;
  export all;
}

protocol device {
  scan time 60;
}

protocol direct {
  interface "ens4";
  check link;
  import all;
}`

	birdPeerCfg = `
protocol bgp %s {
  local as 64513;
  neighbor %s as 64512;
  passive;
  import keep filtered;
  import all;
  export none;  
}`
	mirrorServerManifest = `
apiVersion: apps/v1beta2
kind: DaemonSet
metadata:
  namespace: default
  name: mirror
  labels:
    app: mirror
spec:
  selector:
    matchLabels:
      app: mirror
  template:
    metadata:
      labels:
        app: mirror
    spec:
      containers:
      - name: mirror
        image: 127.0.0.1:30000/e2etest-mirror-server:e2e-amd64
        env:
        - name: NODE_NAME
          valueFrom:
            fieldRef:
              fieldPath: spec.nodeName
        - name: POD_UID
          valueFrom:
            fieldRef:
              fieldPath: metadata.uid
        ports:
        - name: http
          containerPort: 8080
`
)
