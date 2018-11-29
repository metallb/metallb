package virtuakube

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"golang.org/x/crypto/ssh"
)

var buildTools = []string{
	"qemu-system-x86_64",
	"qemu-img",
	"docker",
	"virt-make-fs",
}

const (
	dockerfile = `
FROM debian:buster
RUN apt-get -y update
RUN DEBIAN_FRONTEND=noninteractive apt-get -y install --no-install-recommends \
  dbus              \
  linux-image-amd64 \
  openssh-server    \
  systemd-sysv
RUN echo "root:root" | chpasswd
RUN echo -e "[Match]\nMACAddress=52:54:00:12:34:56\n[Network]\nDHCP=ipv4" >/etc/systemd/network/10-internet.network
RUN echo "PermitRootLogin yes" >>/etc/ssh/sshd_config
RUN systemctl enable systemd-networkd
`
)

// BuildConfig is the configuration for a disk image build.
type BuildConfig struct {
	// InstallScript is the script to run on the initial VM image to
	// turn it into the final image.
	InstallScript []byte

	// OutputPath is the destination path for the built VM image.
	OutputPath string
	// TempDir is a temporary directory to use as scratch space. It
	// should have enough free space to store ~2 copies of the final
	// image.
	TempDir string

	// If true, create a window for the VM's display, and don't halt
	// the VM if the install script fails.
	Debug bool
}

func BuildImage(ctx context.Context, cfg *BuildConfig) error {
	var err error

	tmp, err := ioutil.TempDir(cfg.TempDir, "virtuakube-build")
	if err != nil {
		return fmt.Errorf("creating tempdir %q: %v", tmp, err)
	}
	defer os.RemoveAll(tmp)

	if err := ioutil.WriteFile(filepath.Join(tmp, "Dockerfile"), []byte(dockerfile), 0644); err != nil {
		return fmt.Errorf("writing Dockerfile: %v", err)
	}

	iidPath := filepath.Join(tmp, "iid")
	err = buildCmd(
		ctx, cfg.Debug,
		"docker", "build", "--iidfile", iidPath, tmp,
	)
	if err != nil {
		return fmt.Errorf("running docker build: %v", err)
	}

	iid, err := ioutil.ReadFile(iidPath)
	if err != nil {
		return fmt.Errorf("reading image ID: %v", err)
	}

	cidPath := filepath.Join(tmp, "cid")
	err = buildCmd(
		ctx, cfg.Debug,
		"docker", "run",
		"--cidfile", cidPath,
		fmt.Sprintf("--mount=type=bind,source=%s,destination=/tmp/ctx", tmp),
		string(iid),
		"cp", "/vmlinuz", "/initrd.img", "/tmp/ctx",
	)
	if err != nil {
		return fmt.Errorf("running docker run: %v", err)
	}

	cid, err := ioutil.ReadFile(cidPath)
	if err != nil {
		return fmt.Errorf("reading image ID: %v", err)
	}

	tarPath := filepath.Join(tmp, "fs.tar")
	err = buildCmd(
		ctx, cfg.Debug,
		"docker", "export",
		"-o", tarPath,
		string(cid),
	)
	if err != nil {
		return fmt.Errorf("exporting image tarball: %v", err)
	}

	imgPath := filepath.Join(tmp, "fs.img")
	err = buildCmd(
		ctx, cfg.Debug,
		"virt-make-fs",
		"--partition", "--format=qcow2",
		"--type=ext4", "--size=10G",
		tarPath, imgPath,
	)
	if err != nil {
		return fmt.Errorf("creating image file: %v", err)
	}

	if err := os.Remove(tarPath); err != nil {
		return fmt.Errorf("removing image tarball: %v", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	cmd := exec.CommandContext(
		ctx,
		"qemu-system-x86_64",
		"-enable-kvm",
		"-m", "2048",
		"-device", "virtio-net,netdev=net0,mac=52:54:00:12:34:56",
		"-device", "virtio-rng-pci,rng=rng0",
		"-object", "rng-random,filename=/dev/urandom,id=rng0",
		"-netdev", "user,id=net0,hostfwd=tcp:127.0.0.1:50000-:22",
		"-drive", fmt.Sprintf("if=virtio,file=%s,media=disk", imgPath),
		"-kernel", filepath.Join(tmp, "vmlinuz"),
		"-initrd", filepath.Join(tmp, "initrd.img"),
		"-append", "root=/dev/vda1 rw",
	)
	if cfg.Debug {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting build VM: %v", err)
	}
	go func() {
		cmd.Wait()
		cancel()
	}()

	client, err := dialSSH(ctx, "127.0.0.1:50000")
	if err != nil {
		return fmt.Errorf("connecting to VM with SSH: %v", err)
	}
	defer client.Close()

	sess, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("creating SSH session: %v", err)
	}
	defer sess.Close()
	if cfg.Debug {
		sess.Stdout = os.Stdout
		sess.Stderr = os.Stderr
	}
	sess.Stdin = bytes.NewBuffer(cfg.InstallScript)
	if err := sess.Run("cat >/tmp/install.sh"); err != nil {
		return fmt.Errorf("copying install script: %v", err)
	}

	sess, err = client.NewSession()
	if err != nil {
		return fmt.Errorf("creating SSH session: %v", err)
	}
	defer sess.Close()
	if cfg.Debug {
		sess.Stdout = os.Stdout
		sess.Stderr = os.Stderr
	}
	if err := sess.Run("bash /tmp/install.sh"); err != nil {
		return fmt.Errorf("running install script: %v", err)
	}

	sess, err = client.NewSession()
	if err != nil {
		return fmt.Errorf("creating SSH session: %v", err)
	}
	defer sess.Close()
	if cfg.Debug {
		sess.Stdout = os.Stdout
		sess.Stderr = os.Stderr
	}
	if err := sess.Run("poweroff"); err != nil {
		return fmt.Errorf("powering off VM: %v", err)
	}

	<-ctx.Done()

	return nil
}

func buildCmd(ctx context.Context, debug bool, command string, args ...string) error {
	cmd := exec.CommandContext(ctx, command, args...)
	if debug {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	return cmd.Run()
}

func dialSSH(ctx context.Context, target string) (*ssh.Client, error) {
	sshCfg := &ssh.ClientConfig{
		User:            "root",
		Auth:            []ssh.AuthMethod{ssh.Password("root")},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         time.Second,
	}
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	for ctx.Err() == nil {
		client, err := ssh.Dial("tcp", target, sshCfg)
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		return client, nil
	}

	return nil, ctx.Err()
}
