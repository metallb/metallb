package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strconv"

	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	vk "go.universe.tf/virtuakube"
)

const cachedUniverse = `e2etest-cached-universe`

func runCluster() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle ctrl+C by cancelling the context, which will shut down
	// everything in the universe.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	go func() {
		defer cancel()
		select {
		case <-stop:
		case <-ctx.Done():
		}
	}()

	universe, err := vk.Open(cachedUniverse)
	if err != nil {
		return fmt.Errorf("opening universe: %v", err)
	}
	defer universe.Close()

	fmt.Println("Hit ctrl+C to close.")
	<-ctx.Done()
	return nil
}

func buildImage() error {
	_, err := os.Stat(cachedUniverse)
	if err == nil {
		if err := os.RemoveAll(cachedUniverse); err != nil {
			return fmt.Errorf("removing old universe: %v", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("checking for the existence of the cached universe: %v", err)
	}

	universe, err := vk.Create(cachedUniverse)
	if err != nil {
		return fmt.Errorf("creating universe: %v", err)
	}
	defer universe.Close()

	imageCfg := &vk.ImageConfig{
		Name: "base",
		CustomizeFuncs: []vk.ImageCustomizeFunc{
			vk.CustomizeInstallK8s,
			func(v *vk.VM) error {
				return v.RunMultiple(
					"apt-get install bird",
					"systemctl enable bird",
					"systemctl enable bird6",
				)
			},
		},
		BuildLog: os.Stdout,
	}
	if _, err := universe.NewImage(imageCfg); err != nil {
		return fmt.Errorf("creating VM base image: %v", err)
	}

	if err := universe.Save(); err != nil {
		return fmt.Errorf("saving universe: %v", err)
	}

	return nil
}

func buildUniverse() error {
	universe, err := vk.Open(cachedUniverse)
	if err != nil {
		return fmt.Errorf("opening universe: %v", err)
	}
	defer universe.Close()

	vmCfg := &vk.VMConfig{
		ImageName:  "base",
		Hostname:   "client",
		CommandLog: os.Stdout,
	}

	clusterCfg := &vk.ClusterConfig{
		Name:         "cluster",
		NumNodes:     2,
		VMConfig:     vmCfg,
		NetworkAddon: "calico",
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
	fmt.Fprintf(&bird4, birdBaseCfg, client.IPv4())
	fmt.Fprintf(&bird6, birdBaseCfg, client.IPv4())

	fmt.Fprintf(&bird4, birdPeerCfg, "controller", cluster.Controller().IPv4())
	fmt.Fprintf(&bird6, birdPeerCfg, "controller", cluster.Controller().IPv6())

	for i, vm := range cluster.Nodes() {
		fmt.Fprintf(&bird4, birdPeerCfg, "node"+strconv.Itoa(i), vm.IPv4())
		fmt.Fprintf(&bird6, birdPeerCfg, "node"+strconv.Itoa(i), vm.IPv6())
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
		"REGISTRY=localhost:"+strconv.Itoa(cluster.Registry()),
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

	if err := universe.Save(); err != nil {
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
