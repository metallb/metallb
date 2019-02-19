package virtuakube

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"go.universe.tf/virtuakube/internal/assets"
)

var buildTools = []string{
	"qemu-system-x86_64",
	"qemu-img",
	"docker",
	"virt-make-fs",
}

const (
	dockerfile = `
FROM debian:stretch
RUN apt-get -y update
RUN DEBIAN_FRONTEND=noninteractive apt-get -y install --no-install-recommends \
  ca-certificates \
  dbus \
  grub2 \
  ifupdown \
  isc-dhcp-client \
  isc-dhcp-common \
  linux-image-amd64 \
  openssh-server \
  systemd-sysv
RUN DEBIAN_FRONTEND=noninteractive apt-get -y upgrade --no-install-recommends
RUN echo "root:root" | chpasswd
RUN echo "PermitRootLogin yes" >>/etc/ssh/sshd_config
RUN echo "auto enp0s2\niface enp0s2 inet dhcp" >/etc/network/interfaces
RUN echo "supersede domain-name-servers 8.8.8.8;" >>/etc/dhcp/dhclient.conf
`

	hosts = `
127.0.0.1 localhost
::1 localhost
`

	fstab = `
/dev/vda1 / ext4 rw,relatime 0 1
bpffs /sys/fs/bpf bpf rw,relatime 0 0
`
)

// ImageCustomizeFunc is a function that applies customizations to a
// VM that's being built by NewImage.
type ImageCustomizeFunc func(*VM) error

// ImageConfig is the build configuration for an Image.
type ImageConfig struct {
	Name           string
	CustomizeFuncs []ImageCustomizeFunc
	NoKVM          bool
}

// Image is a VM disk base image.
type Image struct {
	path string
}

func (u *Universe) ImportImage(name, path string) error {
	disk := randomDiskName()
	src, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("opening %q: %v", path, err)
	}

	dst, err := os.Create(filepath.Join(u.dir, disk))
	if err != nil {
		return fmt.Errorf("creating %q: %v", disk, err)
	}

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("writing disk: %v", err)
	}

	u.mu.Lock()
	defer u.mu.Unlock()
	if u.images[name] != "" {
		panic("what")
	}
	u.images[name] = disk

	return nil
}

// NewImage builds a VM disk image using the given config.
func (u *Universe) NewImage(cfg *ImageConfig) error {
	if err := checkTools(buildTools); err != nil {
		return err
	}

	tmp, err := ioutil.TempDir(u.tmpdir, "b")
	if err != nil {
		return fmt.Errorf("creating tempdir in %q: %v", u.dir, err)
	}
	defer os.RemoveAll(tmp)

	if err := ioutil.WriteFile(filepath.Join(tmp, "Dockerfile"), []byte(dockerfile), 0644); err != nil {
		return fmt.Errorf("writing dockerfile: %v", err)
	}

	iidPath := filepath.Join(tmp, "iid")
	cmd := exec.Command("docker", "build", "--iidfile", iidPath, tmp)
	cmd.Stdout = u.runtimecfg.CommandLog
	cmd.Stderr = u.runtimecfg.CommandLog
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("running docker build: %v", err)
	}

	iid, err := ioutil.ReadFile(iidPath)
	if err != nil {
		return fmt.Errorf("reading image ID: %v", err)
	}

	cidPath := filepath.Join(tmp, "cid")
	cmd = exec.Command(
		"docker", "run",
		"--cidfile", cidPath,
		fmt.Sprintf("--mount=type=bind,source=%s,destination=/tmp/ctx", tmp),
		string(iid),
		"cp", "/vmlinuz", "/initrd.img", "/tmp/ctx",
	)
	cmd.Stdout = u.runtimecfg.CommandLog
	cmd.Stderr = u.runtimecfg.CommandLog
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("extracting kernel from container: %v", err)
	}

	cid, err := ioutil.ReadFile(cidPath)
	if err != nil {
		return fmt.Errorf("reading image ID: %v", err)
	}

	tarPath := filepath.Join(tmp, "fs.tar")
	cmd = exec.Command(
		"docker", "export",
		"-o", tarPath,
		string(cid),
	)
	cmd.Stdout = u.runtimecfg.CommandLog
	cmd.Stderr = u.runtimecfg.CommandLog
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("exporting image tarball: %v", err)
	}

	imgPath := filepath.Join(tmp, "fs.img")
	cmd = exec.Command(
		"virt-make-fs",
		"--partition", "--format=qcow2",
		"--type=ext4", "--size=10G",
		tarPath, imgPath,
	)
	cmd.Stdout = u.runtimecfg.CommandLog
	cmd.Stderr = u.runtimecfg.CommandLog
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("creating image file: %v", err)
	}

	if err := os.Remove(tarPath); err != nil {
		return fmt.Errorf("removing image tarball: %v", err)
	}

	tmpu, err := Create(filepath.Join(tmp, "u"), u.runtimecfg)
	if err != nil {
		return fmt.Errorf("creating virtuakube instance: %v", err)
	}
	defer tmpu.Destroy()

	if err := tmpu.ImportImage("build", imgPath); err != nil {
		return fmt.Errorf("importing half-built image: %v", err)
	}

	v, err := tmpu.NewVM(&VMConfig{
		Image:     "build",
		MemoryMiB: 2048,
		kernelConfig: &kernelConfig{
			kernelPath: filepath.Join(tmp, "vmlinuz"),
			initrdPath: filepath.Join(tmp, "initrd.img"),
			cmdline:    "root=/dev/vda1 rw",
		},
	})
	if err != nil {
		return fmt.Errorf("creating image VM: %v", err)
	}
	if err := v.Start(); err != nil {
		return fmt.Errorf("starting image VM: %v", err)
	}

	if err := v.WriteFile("/etc/hosts", []byte(hosts)); err != nil {
		return fmt.Errorf("install /etc/hosts: %v", err)
	}

	if err := v.WriteFile("/etc/fstab", []byte(fstab)); err != nil {
		return fmt.Errorf("install /etc/fstab: %v", err)
	}

	err = v.RunMultiple(
		"update-initramfs -u",

		"grub-install /dev/vda",
		"perl -pi -e 's/GRUB_TIMEOUT=.*/GRUB_TIMEOUT=0/' /etc/default/grub",
		"update-grub2",

		"rm /etc/machine-id /var/lib/dbus/machine-id",
		"touch /etc/machine-id",
		"chattr +i /etc/machine-id",
	)
	if err != nil {
		return fmt.Errorf("finalize base image configuration: %v", err)
	}

	for _, f := range cfg.CustomizeFuncs {
		if err = f(v); err != nil {
			return fmt.Errorf("applying customize func: %v", err)
		}
	}

	if _, err := v.Run("sync"); err != nil {
		return fmt.Errorf("syncing image disk: %v", err)
	}

	v.Run("poweroff")
	if err := v.Wait(context.Background()); err != nil {
		return fmt.Errorf("waiting for VM shutdown: %v", err)
	}

	ret := randomDiskName()

	cmd = exec.Command(
		"qemu-img", "convert",
		"-O", "qcow2",
		filepath.Join(tmp, "u", tmpu.image("build")),
		filepath.Join(u.dir, ret),
	)
	cmd.Stdout = u.runtimecfg.CommandLog
	cmd.Stderr = u.runtimecfg.CommandLog
	if err := cmd.Run(); err != nil {
		os.Remove(ret)
		return fmt.Errorf("running qemu-img convert: %v", err)
	}

	u.mu.Lock()
	defer u.mu.Unlock()
	if u.images[cfg.Name] != "" {
		panic("I don't know what to do about this yet")
	}
	u.images[cfg.Name] = ret

	return nil
}

// CustomizeInstallK8s is a build customization function that installs
// Docker and Kubernetes prerequisites, as required for NewCluster to
// function.
func CustomizeInstallK8s(v *VM) error {
	repos := []byte(`
deb [arch=amd64] https://download.docker.com/linux/debian stretch stable
deb http://apt.kubernetes.io/ kubernetes-xenial main
`)
	if err := v.WriteFile("/etc/apt/sources.list.d/k8s.list", repos); err != nil {
		return err
	}

	pkgs := []string{
		"apt-transport-https",
		"curl",
		"ebtables",
		"ethtool",
		"gpg",
		"gpg-agent",
	}
	k8sPkgs := []string{
		"docker-ce=18.06.*",
		"kubelet",
		"kubeadm",
		"kubectl",
	}
	err := v.RunMultiple(
		"DEBIAN_FRONTEND=noninteractive apt-get -y install --no-install-recommends "+strings.Join(pkgs, " "),
		"curl -fsSL https://download.docker.com/linux/debian/gpg | apt-key add -",
		"curl -fsSL https://packages.cloud.google.com/apt/doc/apt-key.gpg | apt-key add -",
		"apt-get -y update",
		"DEBIAN_FRONTEND=noninteractive apt-get -y install --no-install-recommends "+strings.Join(k8sPkgs, " "),
		"echo br_netfilter >>/etc/modules",
		"echo export KUBECONFIG=/etc/kubernetes/admin.conf >>/etc/profile.d/k8s.sh",
	)
	if err != nil {
		return err
	}

	return nil
}

// CustomizePreloadK8sImages is a build customization function that
// pre-pulls all the Docker images needed to fully initialize a
// Kubernetes cluster.
func CustomizePreloadK8sImages(v *VM) error {
	err := v.RunMultiple(
		"systemctl start docker",
		"kubeadm config images pull",
	)
	if err != nil {
		return err
	}

	imgs := strings.Split(string(assets.MustAsset("addon-images")), " ")
	for _, img := range imgs {
		if img == "" {
			continue
		}
		if _, err := v.Run("docker pull " + img); err != nil {
			return err
		}
	}

	return nil
}

// CustomizeScript is a build customization function that executes the
// script at path on a VM running the disk image being built.
func CustomizeScript(path string) func(*VM) error {
	return func(v *VM) error {
		bs, err := ioutil.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading script %q: %v", path, err)
		}

		if err := v.WriteFile("/tmp/script", bs); err != nil {
			return fmt.Errorf("writing script to VM: %v", err)
		}

		return v.RunMultiple(
			"chmod +x /tmp/script",
			"/tmp/script",
			"rm /tmp/script",
		)
	}
}
