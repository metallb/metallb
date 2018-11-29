package virtuakube

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"go.universe.tf/virtuakube/internal/assets"
)

var incrClusterID = make(chan int)

func init() {
	id := 1
	go func() {
		for {
			incrClusterID <- id
			id++
		}
	}()
}

// ClusterConfig is the configuration for a virtual Kubernetes
// cluster.
type ClusterConfig struct {
	// NumNodes is the number of Kubernetes worker nodes to run.
	// TODO: only supports 1 currently
	NumNodes int
	// The VMConfig template to use when creating cluster VMs.
	*VMConfig
	// NetworkAddon is the Kubernetes network addon to install. Can be
	// an absolute path to a manifest yaml, or one of the builtin
	// addons "calico" or "weave".
	//
	// TODO: that last bit is currently a lie, only paths work.
	NetworkAddon string
	// ExtraAddons is a list of Kubernetes manifest yamls to apply to
	// the cluster, in addition to the network addon.
	ExtraAddons []string
}

// Copy returns a deep copy of the cluster config.
func (c *ClusterConfig) Copy() *ClusterConfig {
	ret := &ClusterConfig{
		NumNodes:     c.NumNodes,
		VMConfig:     c.VMConfig.Copy(),
		NetworkAddon: c.NetworkAddon,
		ExtraAddons:  make([]string, 0, len(c.ExtraAddons)),
	}
	for _, addon := range c.ExtraAddons {
		c.ExtraAddons = append(c.ExtraAddons, addon)
	}
	return ret
}

// Cluster is a virtual Kubernetes cluster.
type Cluster struct {
	cfg        *ClusterConfig
	tmpdir     string
	kubeconfig string
	client     *kubernetes.Clientset

	controller *VM
	nodes      []*VM

	startedMu sync.Mutex
	started   bool
}

func validateClusterConfig(cfg *ClusterConfig) (*ClusterConfig, error) {
	cfg = cfg.Copy()

	if cfg.NumNodes != 1 {
		return nil, errors.New("clusters with >1 node not supported yet")
	}

	if cfg.VMConfig == nil {
		return nil, errors.New("missing VMConfig")
	}

	var err error
	cfg.VMConfig, err = validateVMConfig(cfg.VMConfig)
	if err != nil {
		return nil, err
	}

	if cfg.NetworkAddon == "" {
		return nil, errors.New("must specify network addon")
	}

	for i, extra := range cfg.ExtraAddons {
		eap, err := filepath.Abs(extra)
		if err != nil {
			return nil, err
		}
		if _, err := os.Stat(eap); err != nil {
			return nil, err
		}
		cfg.ExtraAddons[i] = eap
	}

	return cfg, nil
}

// NewCluster creates an unstarted Kubernetes cluster with the given
// configuration.
func (u *Universe) NewCluster(cfg *ClusterConfig) (*Cluster, error) {
	cfg, err := validateClusterConfig(cfg)
	if err != nil {
		return nil, err
	}

	p, err := u.Tmpdir("cluster")
	if err != nil {
		return nil, err
	}

	ret := &Cluster{
		cfg:    cfg,
		tmpdir: p,
	}

	clusterID := <-incrClusterID

	controllerCfg := cfg.VMConfig.Copy()
	controllerCfg.Hostname = fmt.Sprintf("cluster%d-controller", clusterID)
	controllerCfg.BootScript = assets.MustAsset("controller.sh")
	controllerCfg.PortForwards[30000] = true
	controllerCfg.PortForwards[6443] = true
	ret.controller, err = u.NewVM(controllerCfg)
	if err != nil {
		return nil, err
	}

	for i := 0; i < cfg.NumNodes; i++ {
		nodeCfg := cfg.VMConfig.Copy()
		nodeCfg.Hostname = fmt.Sprintf("cluster%d-node%d", clusterID, i+1)
		nodeCfg.BootScript = assets.MustAsset("node.sh")
		node, err := u.NewVM(nodeCfg)
		if err != nil {
			return nil, err
		}
		ret.nodes = append(ret.nodes, node)
	}

	return ret, nil
}

// Start boots the virtual cluster. The universe is destroyed if any
// VM in the cluster shuts down.
func (c *Cluster) Start() error {
	c.startedMu.Lock()
	defer c.startedMu.Unlock()

	if c.started {
		return errors.New("already started")
	}
	c.started = true

	bs, err := assembleAddons(c.cfg.NetworkAddon, c.cfg.ExtraAddons)
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(filepath.Join(c.controller.Dir(), "addons.yaml"), bs, 0644); err != nil {
		return err
	}

	if err := c.controller.Start(); err != nil {
		return err
	}
	for _, node := range c.nodes {
		if err := node.Start(); err != nil {
			return err
		}
	}

	return nil
}

// WaitReady waits until the Kubernetes cluster is ready to use. A
// Cluster is ready when all configured nodes are in the cluster and
// in the "Ready" state, and all deployments and daemonsets have all
// replicas available.
func (c *Cluster) WaitReady(ctx context.Context) error {
	if err := c.controller.WaitReady(ctx); err != nil {
		return err
	}
	for _, node := range c.nodes {
		if err := node.WaitReady(ctx); err != nil {
			return err
		}
	}

	if err := adjustKubeconfig(filepath.Join(c.controller.Dir(), "kubeconfig"), c.Kubeconfig(), c.controller.ForwardedPort(6443)); err != nil {
		return err
	}

	config, err := clientcmd.BuildConfigFromFlags("", c.Kubeconfig())
	if err != nil {
		return err
	}

	c.client, err = kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	err = waitFor(ctx, func() (bool, error) {
		nodes, err := c.client.CoreV1().Nodes().List(metav1.ListOptions{})
		if err != nil {
			return false, err
		}

		if len(nodes.Items) != c.cfg.NumNodes+1 {
			return false, nil
		}
		for _, node := range nodes.Items {
			if !nodeReady(node) {
				return false, nil
			}
		}

		deploys, err := c.client.AppsV1().Deployments("").List(metav1.ListOptions{})
		if err != nil {
			return false, err
		}
		for _, deploy := range deploys.Items {
			if deploy.Status.AvailableReplicas != deploy.Status.Replicas {
				return false, nil
			}
		}

		daemons, err := c.client.AppsV1().DaemonSets("").List(metav1.ListOptions{})
		if err != nil {
			return false, err
		}
		for _, daemon := range daemons.Items {
			if daemon.Status.NumberAvailable != daemon.Status.DesiredNumberScheduled {
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

// Kubeconfig returns the path to a kubectl configuration file with
// administrator credentials for the cluster.
func (c *Cluster) Kubeconfig() string {
	return filepath.Join(c.tmpdir, "kubeconfig")
}

// KubernetesClient returns a kubernetes client connected to the
// cluster.
func (c *Cluster) KubernetesClient() *kubernetes.Clientset {
	return c.client
}

// Controller returns the VM for the cluster controller node.
func (c *Cluster) Controller() *VM {
	return c.controller
}

// Nodes returns the VMs for the cluster nodes.
func (c *Cluster) Nodes() []*VM {
	return c.nodes
}

// Registry returns the port on localhost for the in-cluster
// registry. Within the cluster, the registry is reachable at
// localhost:30000 on all nodes.
func (c *Cluster) Registry() int {
	return c.controller.ForwardedPort(30000)
}

func networkAddonBytes(addon string) ([]byte, error) {
	bs, err := assets.Asset("net/" + addon + ".yaml")
	if err == nil {
		return bs, nil
	}

	return ioutil.ReadFile(addon)
}

func assembleAddons(networkAddon string, extraAddons []string) ([]byte, error) {
	var out [][]byte

	bs, err := networkAddonBytes(networkAddon)
	if err != nil {
		return nil, err
	}
	out = append(out, bs)
	out = append(out, assets.MustAsset("registry.yaml"))

	for _, extra := range extraAddons {
		bs, err = ioutil.ReadFile(extra)
		if err != nil {
			return nil, err
		}
		out = append(out, bs)
	}

	return bytes.Join(out, []byte("\n---\n")), nil
}

var addrRe = regexp.MustCompile("https://.*:6443")

func adjustKubeconfig(src, dst string, localPort int) error {
	bs, err := ioutil.ReadFile(src)
	if err != nil {
		return err
	}

	rep := addrRe.ReplaceAll(bs, []byte("https://127.0.0.1:"+strconv.Itoa(localPort)))

	return ioutil.WriteFile(dst, rep, 0644)
}

func nodeReady(node corev1.Node) bool {
	for _, cond := range node.Status.Conditions {
		if cond.Type != corev1.NodeReady {
			continue
		}
		if cond.Status == corev1.ConditionTrue {
			return true
		}
		return false
	}

	return false
}

func copyFile(src, dst string) error {
	bs, err := ioutil.ReadFile(src)
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(dst, bs, 0644); err != nil {
		return err
	}
	return nil
}

func waitFor(ctx context.Context, test func() (bool, error)) error {
	done := ctx.Done()
	for {
		select {
		case <-done:
			return errors.New("timeout")
		default:
		}

		ok, err := test()
		if err != nil {
			return err
		}
		if ok {
			return nil
		}

		time.Sleep(100 * time.Millisecond)
	}
}
