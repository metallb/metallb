// SPDX-License-Identifier:Apache-2.0

package frr

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
	"go.universe.tf/metallb/e2etest/pkg/executor"
	config "go.universe.tf/metallb/e2etest/pkg/frr/config"
	"go.universe.tf/metallb/e2etest/pkg/frr/consts"
)

const (
	// BGP configuration directory.
	frrConfigDir = "config/frr"
	// FRR routing image.
	frrImage = "frrouting/frr:v7.5.1"
	// FRR container mount destination path.
	frrMountPath = "/etc/frr"
)

// Run a BGP router in a container.
func StartContainer(containerName string, testDirName string) error {
	srcFiles := fmt.Sprintf("%s/.", frrConfigDir)
	res, err := exec.Command("cp", "-r", srcFiles, testDirName).CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, "Failed to copy FRR config directory. %s", string(res))
	}

	volume := fmt.Sprintf("%s:%s", testDirName, frrMountPath)
	out, err := exec.Command("docker", "run", "-d", "--privileged", "--network", "kind", "--rm", "--name", containerName,
		"--volume", volume, frrImage).CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, "Failed to start %s container. %s", containerName, out)
	}

	return nil
}

// Returns the IPv4 and IPv6 addresses of the container.
func GetContainerIPs(containerName string) (ipv4 string, ipv6 string, err error) {
	containerIP, err := exec.Command("docker", "inspect", "-f", "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}",
		containerName).CombinedOutput()
	if err != nil {
		return "", "", errors.Wrapf(err, "Failed to get FRR IP address")
	}

	containerIPv4 := strings.TrimSuffix(string(containerIP), "\n")

	containerIP, err = exec.Command("docker", "inspect", "-f", "{{range .NetworkSettings.Networks}}{{.GlobalIPv6Address}}{{end}}",
		containerName).CombinedOutput()
	if err != nil {
		return "", "", errors.Wrapf(err, "Failed to get FRR IP address")
	}

	containerIPv6 := strings.TrimSuffix(string(containerIP), "\n")

	return containerIPv4, containerIPv6, nil
}

// Updating the BGP config file.
func UpdateBGPConfigFile(testDirName string, bgpConfig string, exec executor.Executor) error {
	err := config.SetBGPConfig(testDirName, bgpConfig)
	if err != nil {
		return errors.Wrapf(err, "Failed to update BGP config file")
	}

	err = reloadFRRConfig(consts.BGPConfigFile, exec)
	if err != nil {
		return errors.Wrapf(err, "Failed to reload BGP config file")
	}

	return nil
}

// Delete the BGP router container configuration.
func StopContainer(containerName string, testDirName string) error {
	// Kill the BGP router container.
	out, err := exec.Command("docker", "kill", containerName).CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, "Failed to kill %s container. %s", containerName, out)
	}

	err = os.RemoveAll(testDirName)
	if err != nil {
		return errors.Wrapf(err, "Failed to delete FRR config directory.")
	}

	return nil
}

// Change volume permissions.
// Allows deleting the test directory or updating the files in the volume.
func UpdateContainerVolumePermissions(containerName string) error {
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
	out, err := exec.Command("docker", "exec", containerName, "sh", "-c", cmd).CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, "Failed to change %s container volume permissions. %s", containerName, string(out))
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
