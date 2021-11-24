// SPDX-License-Identifier:Apache-2.0

package container

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
	"go.universe.tf/metallb/e2etest/pkg/executor"
	"go.universe.tf/metallb/e2etest/pkg/frr/config"
	"go.universe.tf/metallb/e2etest/pkg/frr/consts"
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
}

// Start creates a new FRR container on the host and returns the corresponding *FRR.
// A situation where a non-nil container and an error are returned is possible.
func Start(name string, nc config.NeighborConfig, rc config.RouterConfig, network string, hostipv4 string, hostipv6 string) (*FRR, error) {
	configDir, err := ioutil.TempDir("", "frr-conf")
	if err != nil {
		return nil, err
	}

	err = startContainer(name, configDir, rc, network)
	if err != nil {
		return nil, err
	}

	exc := executor.ForContainer(name)

	frr := &FRR{
		Executor:       exc,
		Name:           name,
		configDir:      configDir,
		NeighborConfig: nc,
		RouterConfig:   rc,
	}

	if network == hostNetwork {
		frr.Ipv4 = hostipv4
		frr.Ipv6 = hostipv6
	} else {
		err = frr.updateIPS()
		if err != nil {
			return frr, err
		}
	}

	err = frr.updateVolumePermissions()
	if err != nil {
		return frr, err
	}

	return frr, nil
}

// Run a BGP router in a container.
func startContainer(containerName string, testDirName string, rc config.RouterConfig, network string) error {
	srcFiles := fmt.Sprintf("%s/.", frrConfigDir)
	res, err := exec.Command("cp", "-r", srcFiles, testDirName).CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, "Failed to copy FRR config directory. %s", string(res))
	}

	err = config.SetDaemonsConfig(testDirName, rc)
	if err != nil {
		return err
	}

	volume := fmt.Sprintf("%s:%s", testDirName, frrMountPath)
	out, err := exec.Command(executor.ContainerRuntime, "run", "-d", "--privileged", "--network", network, "--rm", "--name", containerName,
		"--volume", volume, frrImage).CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, "Failed to start %s container. %s", containerName, out)
	}

	return nil
}

// Sets the IPv4 and IPv6 addresses of the *FRR.
// todo: improve error handling, especially check that containerIPv4 and containerIPv6 are not empty
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
func (c *FRR) Stop() error {
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

func (c *FRR) AddressesForFamily(ipFamily string) []string {
	addresses := []string{c.Ipv4}
	switch ipFamily {
	case "ipv6":
		addresses = []string{c.Ipv6}
	case "dual":
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
