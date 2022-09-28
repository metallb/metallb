// SPDX-License-Identifier:Apache-2.0

package container

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"go.universe.tf/metallb/e2etest/pkg/executor"
	"go.universe.tf/metallb/e2etest/pkg/frr"
	"go.universe.tf/metallb/e2etest/pkg/frr/config"
	frrconfig "go.universe.tf/metallb/e2etest/pkg/frr/config"
	"go.universe.tf/metallb/e2etest/pkg/frr/consts"
	"go.universe.tf/metallb/internal/ipfamily"
	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
)

const (
	// BGP configuration directory.
	frrConfigDir = "config/frr"
	// FRR routing image.
	frrImage = "frrouting/frr:v7.5.1"
	// Host network name.
	hostNetwork = "host"
	// FRR container mount destination path.
	frrMountPath = "/etc/frr"
)

type FRR struct {
	executor.Executor
	Name           string
	configDir      string
	NeighborConfig config.NeighborConfig
	RouterConfig   config.RouterConfig
	Ipv4           string
	Ipv6           string
	Network        string
	MultiProtocol  config.MultiProtocol
}

type Config struct {
	Name        string
	Neighbor    config.NeighborConfig
	Router      config.RouterConfig
	HostIPv4    string
	HostIPv6    string
	IPv4Address string
	IPv6Address string
	Network     string
}

// Create creates a set of frr containers corresponding to the given configurations.
func Create(c ...Config) ([]*FRR, error) {
	m := sync.Mutex{}
	frrContainers := make([]*FRR, 0)
	g := new(errgroup.Group)
	for _, conf := range c {
		conf := conf
		g.Go(func() error {
			toFind := map[string]bool{
				"zebra":    true,
				"watchfrr": true,
				"bgpd":     true,
				"bfdd":     true,
			}
			c, err := start(conf)
			if c != nil {
				err = wait.PollImmediate(time.Second, 5*time.Minute, func() (bool, error) {
					daemons, err := frr.Daemons(c)
					if err != nil {
						return false, err
					}
					for _, d := range daemons {
						delete(toFind, d)
					}
					if len(toFind) > 0 {
						return false, nil
					}
					return true, nil
				})
				m.Lock()
				defer m.Unlock()
				frrContainers = append(frrContainers, c)
			}
			if err != nil {
				return errors.Wrapf(err, "Failed to wait for daemons %v", toFind)
			}

			return nil
		})
	}
	err := g.Wait()

	return frrContainers, err
}

func Delete(containers []*FRR) error {
	g := new(errgroup.Group)
	for _, c := range containers {
		c := c
		g.Go(func() error {
			err := c.delete()
			return err
		})
	}

	return g.Wait()
}

// PairWithNodes pairs the given frr instance with all the cluster nodes.
func PairWithNodes(cs clientset.Interface, c *FRR, ipFamily ipfamily.Family, modifiers ...func(c *FRR)) error {
	config := *c
	for _, m := range modifiers {
		m(&config)
	}
	bgpConfig, err := frrconfig.BGPPeersForAllNodes(cs, config.NeighborConfig, config.RouterConfig, ipFamily, config.MultiProtocol)
	if err != nil {
		return err
	}

	err = c.UpdateBGPConfigFile(bgpConfig)
	if err != nil {
		return err
	}
	return nil
}

// ConfigureExisting validates that the existing frr containers that correspond to the
// given configurations are up and running, and returns the corresponding *FRRs.
func ConfigureExisting(c ...Config) ([]*FRR, error) {
	frrContainers := make([]*FRR, 0)
	for _, cfg := range c {
		err := containerIsRunning(cfg.Name)
		if err != nil {
			return nil, fmt.Errorf("Failed to use an existing container %s. %w", cfg.Name, err)
		}

		frr, err := configureContainer(cfg, cfg.Name)
		if err != nil {
			return nil, fmt.Errorf("Failed to create container configurations for %s. %w", cfg.Name, err)
		}

		frrContainers = append(frrContainers, frr)
	}

	return frrContainers, nil
}

// start creates a new FRR container on the host and returns the corresponding *FRR.
// A situation where a non-nil container and an error are returned is possible.
func start(cfg Config) (*FRR, error) {
	testDirName, err := ioutil.TempDir("", "frr-conf")
	if err != nil {
		return nil, err
	}
	srcFiles := fmt.Sprintf("%s/.", frrConfigDir)
	res, err := exec.Command("cp", "-r", srcFiles, testDirName).CombinedOutput()
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to copy FRR config directory. %s", string(res))
	}

	err = config.SetDaemonsConfig(testDirName, cfg.Router)
	if err != nil {
		return nil, err
	}

	volume := fmt.Sprintf("%s:%s", testDirName, frrMountPath)
	args := []string{"run", "-d", "--privileged", "--network", cfg.Network, "--rm", "--ulimit", "core=-1", "--name", cfg.Name, "--volume", volume, frrImage}
	if cfg.IPv4Address != "" {
		args = append(args, "--ip", cfg.IPv4Address)
	}
	if cfg.IPv6Address != "" {
		args = append(args, "--ip", cfg.IPv6Address)
	}
	out, err := exec.Command(executor.ContainerRuntime, args...).CombinedOutput()
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to start %s container. %s", cfg.Name, out)
	}

	frr, err := configureContainer(cfg, testDirName)
	if err != nil {
		return frr, fmt.Errorf("Failed to create container configurations for %s. %w", cfg.Name, err)
	}

	return frr, nil
}

// configureContainer creates the corresponding *FRR for a given container.
// A situation where a non-nil container and an error are returned is possible.
func configureContainer(cfg Config, configDir string) (*FRR, error) {
	exc := executor.ForContainer(cfg.Name)

	frr := &FRR{
		Executor:       exc,
		Name:           cfg.Name,
		configDir:      configDir,
		NeighborConfig: cfg.Neighbor,
		RouterConfig:   cfg.Router,
		Network:        cfg.Network,
	}

	if cfg.Network == hostNetwork {
		if net.ParseIP(cfg.HostIPv4) == nil {
			return nil, errors.New("Invalid hostIPv4")
		}
		if net.ParseIP(cfg.HostIPv6) == nil {
			return nil, errors.New("Invalid hostIPv6")
		}

		frr.Ipv4 = cfg.HostIPv4
		frr.Ipv6 = cfg.HostIPv6
	} else {
		err := frr.updateIPS()
		if err != nil {
			return frr, err
		}
	}

	// setting routerid after calculating ips
	frr.RouterConfig.RouterID = frr.Ipv4

	err := frr.updateVolumePermissions()
	if err != nil {
		return frr, err
	}

	return frr, nil
}

// Sets the IPv4 and IPv6 addresses of the *FRR.
func (c *FRR) updateIPS() (err error) {
	containerIP, err := exec.Command(executor.ContainerRuntime, "inspect", "-f", "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}",
		c.Name).CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, "Failed to get FRR IPv4 address")
	}

	containerIPv4 := strings.TrimSuffix(string(containerIP), "\n")

	containerIP, err = exec.Command(executor.ContainerRuntime, "inspect", "-f", "{{range .NetworkSettings.Networks}}{{.GlobalIPv6Address}}{{end}}",
		c.Name).CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, "Failed to get FRR IPv6 address")
	}

	containerIPv6 := strings.TrimSuffix(string(containerIP), "\n")

	if containerIPv4 == "" && containerIPv6 == "" {
		return errors.Errorf("Failed to get FRR IP addresses")
	}
	c.Ipv4 = containerIPv4
	c.Ipv6 = containerIPv6

	return nil
}

// Updating the BGP config file.
func (c *FRR) UpdateBGPConfigFile(bgpConfig string) error {
	err := config.SetBGPConfig(c.configDir, bgpConfig)
	if err != nil {
		return errors.Wrapf(err, "Failed to update BGP config file")
	}

	err = reloadFRRConfig(consts.BGPConfigFile, c)
	if err != nil {
		return errors.Wrapf(err, "Failed to reload BGP config file")
	}

	return nil
}

// Delete the BGP router container configuration.
func (c *FRR) delete() error {
	// Kill the BGP router container.
	out, err := exec.Command(executor.ContainerRuntime, "kill", c.Name).CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, "Failed to kill %s container. %s", c.Name, out)
	}

	err = os.RemoveAll(c.configDir)
	if err != nil {
		return errors.Wrapf(err, "Failed to delete FRR config directory.")
	}

	return nil
}

func (c *FRR) AddressesForFamily(ipFamily ipfamily.Family) []string {
	addresses := []string{c.Ipv4}
	switch ipFamily {
	case ipfamily.IPv6:
		addresses = []string{c.Ipv6}
	case ipfamily.DualStack:
		addresses = []string{c.Ipv4, c.Ipv6}
	}
	return addresses
}

// Change volume permissions.
// Allows deleting the test directory or updating the files in the volume.
func (c *FRR) updateVolumePermissions() error {
	var uid, gid int

	if isPodman() {
		// Rootless Podman containers run as non-root user.
		// Volumes are mapped container UID:GID <-> host user UID:GID.
		uid = 0
		gid = 0
	} else {
		// Rootful Docker containers run as host root user.
		// Volumes are mapped container UID:GID <-> host *root* UID:GID.
		uid = os.Getuid()
		gid = os.Getgid()
	}

	cmd := fmt.Sprintf("chown -R %d:%d %s", uid, gid, frrMountPath)
	out, err := exec.Command(executor.ContainerRuntime, "exec", c.Name, "sh", "-c", cmd).CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, "Failed to change %s container volume permissions. %s", c.Name, string(out))
	}

	return nil
}

// Checking and reloading the specified config file to the frr container.
func reloadFRRConfig(configFile string, exec executor.Executor) error {
	// Checking the configuration file syntax.
	cmd := fmt.Sprintf("python3 /usr/lib/frr/frr-reload.py --test --stdout %s/%s", frrMountPath, configFile)
	out, err := exec.Exec("sh", "-c", cmd)
	if err != nil {
		return errors.Wrapf(err, "Failed to check configuration file. %s", string(out))
	}

	// Applying the configuration file.
	cmd = fmt.Sprintf("python3 /usr/lib/frr/frr-reload.py --reload --overwrite --stdout %s/%s", frrMountPath, configFile)
	out, err = exec.Exec("sh", "-c", cmd)
	if err != nil {
		return errors.Wrapf(err, "Failed to apply configuration file. %s", string(out))
	}

	return nil
}

// Returns true if docker is a symlink to podman.
func isPodman() bool {
	dockerPath, _ := exec.LookPath("docker")
	symLink, _ := os.Readlink(dockerPath)
	return strings.Contains(symLink, "podman")
}

// containerIsRunning validates that the given container is up and running.
func containerIsRunning(containerName string) error {
	out, err := exec.Command(executor.ContainerRuntime, "ps", "--format", "{{.Status}}", "--filter", fmt.Sprintf("name=%s", containerName)).CombinedOutput()
	if err != nil {
		return fmt.Errorf("Failed to validate container %s is running. %w", containerName, err)
	}

	if len(out) == 0 {
		return fmt.Errorf("Container %s doesn't exist.", containerName)
	}

	if string(out[:2]) != "Up" {
		return fmt.Errorf("Container %s is not up. status is: %s", containerName, out)
	}

	return nil
}
