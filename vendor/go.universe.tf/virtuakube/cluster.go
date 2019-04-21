package virtuakube

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"

	"go.universe.tf/virtuakube/internal/config"
)

const (
	podNetwork  = "10.32.0.0/12"
	serviceCIDR = "10.96.0.0/12"
)

// ClusterConfig is the configuration for a virtual Kubernetes
// cluster.
type ClusterConfig struct {
	Name string
	// NumNodes is the number of Kubernetes worker nodes to run.
	NumNodes int
	// The VMConfig template to use when creating cluster VMs. The
	// first configured VM network will be used for Kubernetes control
	// traffic.
	VMConfig *VMConfig
}

// Cluster is a virtual Kubernetes cluster.
type Cluster struct {
	mu sync.Mutex

	tmpdir string

	cfg *config.Cluster

	// Kubernetes client connected to the cluster.
	client *kubernetes.Clientset

	// Cluster VMs.
	controller *VM
	nodes      []*VM

	started bool
}

func randomClusterName() string {
	rnd := make([]byte, 6)
	if _, err := rand.Read(rnd); err != nil {
		panic("system ran out of randomness")
	}
	return fmt.Sprintf("cluster%x", rnd)
}

// NewCluster creates an unstarted Kubernetes cluster with the given
// configuration.
func (u *Universe) NewCluster(cfg *ClusterConfig) (*Cluster, error) {
	u.mu.Lock()
	defer u.mu.Unlock()

	if cfg == nil {
		return nil, errors.New("no ClusterConfig specified")
	}

	if cfg.VMConfig == nil {
		return nil, errors.New("ClusterConfig is missing VMConfig")
	}

	if len(cfg.VMConfig.Networks) == 0 {
		return nil, errors.New("ClusterConfig's VMConfig does not specify any networks")
	}

	if u.clusters[cfg.Name] != nil {
		return nil, fmt.Errorf("universe already has a cluster named %q", cfg.Name)
	}

	tmp, err := ioutil.TempDir(u.tmpdir, cfg.Name)
	if err != nil {
		return nil, fmt.Errorf("creating temporary directory: %v", err)
	}

	ret := &Cluster{
		tmpdir: tmp,
		cfg: &config.Cluster{
			Name:     cfg.Name,
			NumNodes: cfg.NumNodes,
		},
	}

	controllerCfg := &VMConfig{
		Name:      fmt.Sprintf("%s-controller", cfg.Name),
		Image:     cfg.VMConfig.Image,
		MemoryMiB: cfg.VMConfig.MemoryMiB,
		Networks:  cfg.VMConfig.Networks,
		PortForwards: map[int]bool{
			30000: true,
			6443:  true,
		},
	}
	for fwd := range cfg.VMConfig.PortForwards {
		controllerCfg.PortForwards[fwd] = true
	}
	ctrl, err := u.newVMWithLock(controllerCfg)
	if err != nil {
		return nil, fmt.Errorf("creating controller VM: %v", err)
	}
	ret.controller = ctrl

	for i := 0; i < cfg.NumNodes; i++ {
		nodeCfg := &VMConfig{
			Name:         fmt.Sprintf("%s-node%d", cfg.Name, i+1),
			Image:        cfg.VMConfig.Image,
			MemoryMiB:    cfg.VMConfig.MemoryMiB,
			Networks:     cfg.VMConfig.Networks,
			PortForwards: cfg.VMConfig.PortForwards,
		}
		node, err := u.newVMWithLock(nodeCfg)
		if err != nil {
			return nil, fmt.Errorf("creating node %d: %v", i+1, err)
		}
		ret.nodes = append(ret.nodes, node)
	}

	u.clusters[cfg.Name] = ret
	return ret, nil
}

func (u *Universe) resumeCluster(cfg *config.Cluster) error {
	tmp, err := ioutil.TempDir(u.tmpdir, cfg.Name)
	if err != nil {
		return fmt.Errorf("creating temporary directory: %v", err)
	}

	ret := &Cluster{
		tmpdir:     tmp,
		cfg:        cfg,
		controller: u.vms[fmt.Sprintf("%s-controller", cfg.Name)],
		started:    true,
	}
	for i := 0; i < ret.cfg.NumNodes; i++ {
		ret.nodes = append(ret.nodes, u.vms[fmt.Sprintf("%s-node%d", cfg.Name, i+1)])
	}

	if err := ret.mkKubeClient(); err != nil {
		return err
	}

	u.clusters[cfg.Name] = ret
	return nil
}

// Start starts the virtual cluster and waits for it to finish
// initializing.
func (c *Cluster) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.started {
		return errors.New("already started")
	}
	c.started = true

	if err := c.startController(); err != nil {
		return err
	}

	for _, node := range c.nodes {
		// TODO: scatter-gather startup
		if err := c.startNode(node); err != nil {
			return err
		}
	}

	err := c.WaitFor(context.Background(), func() (bool, error) {
		nodes, err := c.client.CoreV1().Nodes().List(metav1.ListOptions{})
		if err != nil {
			return false, err
		}

		if len(nodes.Items) != c.cfg.NumNodes+1 {
			return false, nil
		}

		return true, nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (c *Cluster) mkKubeClient() error {
	if err := ioutil.WriteFile(filepath.Join(c.tmpdir, "kubeconfig"), c.cfg.Kubeconfig, 0600); err != nil {
		return fmt.Errorf("writing kubeconfig to tmpdir: %v", err)
	}

	config, err := clientcmd.NewClientConfigFromBytes(c.cfg.Kubeconfig)
	if err != nil {
		return err
	}

	restcfg, err := config.ClientConfig()
	if err != nil {
		return err
	}

	c.client, err = kubernetes.NewForConfig(restcfg)
	if err != nil {
		return err
	}

	return nil
}

var addrRe = regexp.MustCompile("https://.*:6443")

func (c *Cluster) startController() error {
	if err := c.controller.Start(); err != nil {
		return err
	}

	controllerConfig := fmt.Sprintf(`
apiVersion: kubeadm.k8s.io/v1beta1
kind: InitConfiguration
bootstrapTokens:
- token: "000000.0000000000000000"
  ttl: "24h"
localAPIEndpoint:
  advertiseAddress: %s
nodeRegistration:
  kubeletExtraArgs:
    node-ip: %s
---
apiVersion: kubeadm.k8s.io/v1beta1
kind: ClusterConfiguration
networking:
  podSubnet: "10.32.0.0/12"
kubernetesVersion: "1.14.0"
clusterName: "virtuakube"
apiServer:
  certSANs:
  - "127.0.0.1"
`, c.controller.IPv4(c.controller.Networks()[0]), c.controller.IPv4(c.controller.Networks()[0]))
	if err := c.controller.WriteFile("/tmp/k8s.conf", []byte(controllerConfig)); err != nil {
		return err
	}

	err := c.controller.RunMultiple(
		"kubeadm init --config=/tmp/k8s.conf --ignore-preflight-errors=NumCPU",
		"KUBECONFIG=/etc/kubernetes/admin.conf kubectl taint nodes --all node-role.kubernetes.io/master-",
	)
	if err != nil {
		return err
	}

	kubeconfig, err := c.controller.ReadFile("/etc/kubernetes/admin.conf")
	if err != nil {
		return err
	}
	c.cfg.Kubeconfig = addrRe.ReplaceAll(kubeconfig, []byte("https://127.0.0.1:"+strconv.Itoa(c.controller.ForwardedPort(6443))))

	return c.mkKubeClient()
}

func (c *Cluster) startNode(node *VM) error {
	if err := node.Start(); err != nil {
		return err
	}

	controllerAddr := &net.TCPAddr{
		IP:   c.controller.IPv4(c.controller.Networks()[0]),
		Port: 6443,
	}
	nodeConfig := fmt.Sprintf(`
apiVersion: kubeadm.k8s.io/v1beta1
kind: JoinConfiguration
discovery:
  bootstrapToken:
    token: "000000.0000000000000000"
    unsafeSkipCAVerification: true
    apiServerEndpoint: %s
nodeRegistration:
  kubeletExtraArgs:
    node-ip: %s
`, controllerAddr, node.IPv4(node.Networks()[0]))
	if err := node.WriteFile("/tmp/k8s.conf", []byte(nodeConfig)); err != nil {
		return err
	}

	if _, err := node.Run("kubeadm join --config=/tmp/k8s.conf"); err != nil {
		return err
	}

	return nil
}

func (c *Cluster) Name() string {
	return c.cfg.Name
}

func (c *Cluster) Kubeconfig() string {
	return filepath.Join(c.tmpdir, "kubeconfig")
}

func getDeploymentsAndDaemonsets(manifestBytes []byte) (deployments []metav1.ObjectMeta, daemons []metav1.ObjectMeta, err error) {
	var docs [][]byte
	manifest := ioutil.NopCloser(bytes.NewBuffer(manifestBytes))
	buf := make([]byte, 64*1024)

	for {
		n, err := manifest.Read(buf)
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, nil, err
		}
		docs = append(docs, append([]byte(nil), buf[:n]...))
	}

	decode := scheme.Codecs.UniversalDeserializer().Decode

	for _, doc := range docs {
		obj, _, err := decode(doc, nil, nil)
		if err != nil {
			return nil, nil, err
		}

		switch o := obj.(type) {
		case *appsv1.Deployment:
			deployments = append(deployments, o.ObjectMeta)
		case *appsv1.DaemonSet:
			daemons = append(daemons, o.ObjectMeta)
		}
	}

	return deployments, daemons, nil
}

func (c *Cluster) ApplyManifest(bs []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.started {
		return errors.New("cluster not started yet")
	}

	deployNames, daemonNames, err := getDeploymentsAndDaemonsets(bs)
	if err != nil {
		return err
	}

	if err := c.controller.WriteFile("/tmp/addon.yaml", bs); err != nil {
		return err
	}

	if _, err := c.controller.Run("KUBECONFIG=/etc/kubernetes/admin.conf kubectl apply -f /tmp/addon.yaml"); err != nil {
		return err
	}

	return c.WaitFor(context.Background(), func() (bool, error) {
		for _, deployName := range deployNames {
			deploy, err := c.client.AppsV1().Deployments(deployName.Namespace).Get(deployName.Name, metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			if deploy.Status.AvailableReplicas != deploy.Status.Replicas {
				return false, nil
			}
		}
		for _, daemonName := range daemonNames {
			daemon, err := c.client.AppsV1().DaemonSets(daemonName.Namespace).Get(daemonName.Name, metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			if daemon.Status.DesiredNumberScheduled == 0 {
				return false, nil
			}
			if daemon.Status.NumberAvailable != daemon.Status.DesiredNumberScheduled {
				return false, nil
			}
		}

		return true, nil
	})
}

// KubernetesClient returns a kubernetes client connected to the
// cluster.
func (c *Cluster) KubernetesClient() *kubernetes.Clientset {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.client
}

// WaitFor invokes the test function repeatedly until it returns true,
// or the context times out.
func (c *Cluster) WaitFor(ctx context.Context, test func() (bool, error)) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
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

// PushImages extracts the named images from the host's docker daemon,
// and pushes them to the docker daemons on all nodes in the cluster.
func (c *Cluster) PushImages(images ...string) error {
	nodes := append(c.Nodes(), c.Controller())
	errs := make(chan error, len(nodes)*len(images))

	for _, image := range images {
		for _, node := range nodes {
			go func(node *VM, image string) {
				pr, pw, err := os.Pipe()
				if err != nil {
					errs <- err
					return
				}
				defer pr.Close()
				defer pw.Close()

				pusher := exec.Command("docker", "save", image)
				pusher.Stdout = pw
				pusher.Stderr = os.Stderr
				if err := pusher.Start(); err != nil {
					errs <- fmt.Errorf("running %q: %v", strings.Join(pusher.Args, " "), err)
					return
				}
				go func() {
					pusher.Wait()
					pw.Close()
				}()

				if _, err := node.RunWithInput("docker load", pr); err != nil {
					errs <- err
					return
				}

				errs <- nil
			}(node, image)
		}
	}

	var retErr error
	for i := 0; i < len(nodes); i++ {
		if err := <-errs; err != nil {
			retErr = err
		}
	}

	return retErr
}

// Controller returns the VM for the cluster controller node.
func (c *Cluster) Controller() *VM {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.controller
}

// Nodes returns the VMs for the cluster nodes.
func (c *Cluster) Nodes() []*VM {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.nodes
}

// NodesReady returns true if all nodes in the cluster are in the
// Ready state.
func (c *Cluster) NodesReady() (bool, error) {
	nodes, err := c.KubernetesClient().CoreV1().Nodes().List(metav1.ListOptions{})
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

	return true, nil
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
