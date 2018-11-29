package virtuakube

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

var incrVMID = make(chan int)

func init() {
	id := 1
	go func() {
		for {
			incrVMID <- id
			id++
		}
	}()
}

// VMConfig is the configuration for a virtual machine.
type VMConfig struct {
	// BackingImagePath is the base disk image for the VM.
	BackingImagePath string
	// Hostname to set on the VM
	Hostname string
	// Amount of RAM.
	MemoryMiB int
	// If true, create a window for the VM's display.
	Display bool
	// Ports to forward from localhost to the VM
	PortForwards map[int]bool
	// BootScriptPath is the path to a boot script that the VM should
	// execute during boot. Alternatively, BootScript is a literal
	// boot script to execute.
	BootScriptPath string
	BootScript     []byte
}

// Copy returns a deep copy of the VM config.
func (v *VMConfig) Copy() *VMConfig {
	ret := &VMConfig{
		BackingImagePath: v.BackingImagePath,
		Hostname:         v.Hostname,
		MemoryMiB:        v.MemoryMiB,
		Display:          v.Display,
		PortForwards:     make(map[int]bool),
		BootScriptPath:   v.BootScriptPath,
		BootScript:       v.BootScript,
	}
	for fwd, v := range v.PortForwards {
		ret.PortForwards[fwd] = v
	}
	return ret
}

// VM is a virtual machine.
type VM struct {
	cfg      *VMConfig
	u        *Universe
	hostPath string
	diskPath string
	mac      string
	forwards map[int]int
	ipv4     string
	ipv6     string

	cmd *exec.Cmd

	mu      sync.Mutex
	started bool
}

func randomMAC() string {
	mac := make(net.HardwareAddr, 6)
	if _, err := rand.Read(mac); err != nil {
		panic("system ran out of randomness")
	}
	// Sets the MAC to be one of the "private" range. Private MACs
	// have the second-least significant bit of the most significant
	// byte set.
	mac[0] = 0x52
	return mac.String()
}

func validateVMConfig(cfg *VMConfig) (*VMConfig, error) {
	if cfg == nil || cfg.BackingImagePath == "" {
		return nil, errors.New("VMConfig with at least BackingImagePath is required")
	}

	cfg = cfg.Copy()

	bp, err := filepath.Abs(cfg.BackingImagePath)
	if err != nil {
		return nil, err
	}
	if _, err = os.Stat(bp); err != nil {
		return nil, err
	}
	cfg.BackingImagePath = bp

	if cfg.BootScriptPath != "" && cfg.BootScript != nil {
		return nil, errors.New("cannot specify both BootScriptPath and BootScript")
	}
	if cfg.BootScriptPath != "" {
		bp, err = filepath.Abs(cfg.BootScriptPath)
		if err != nil {
			return nil, err
		}
		if _, err = os.Stat(bp); err != nil {
			return nil, err
		}
		cfg.BootScriptPath = bp
	}

	if cfg.Hostname == "" {
		cfg.Hostname = "vm" + strconv.Itoa(<-incrVMID)
	}
	if cfg.MemoryMiB == 0 {
		cfg.MemoryMiB = 1024
	}

	return cfg, nil
}

func makeForwards(fwds map[int]int) string {
	var ret []string
	for dst, src := range fwds {
		ret = append(ret, fmt.Sprintf("hostfwd=tcp:127.0.0.1:%d-:%d", src, dst))
	}
	return strings.Join(ret, ",")
}

// NewVM creates an unstarted VM with the given configuration.
func (u *Universe) NewVM(cfg *VMConfig) (*VM, error) {
	cfg, err := validateVMConfig(cfg)
	if err != nil {
		return nil, err
	}

	tmp, err := u.Tmpdir("vm")
	if err != nil {
		return nil, err
	}

	hostPath := filepath.Join(tmp, "hostfs")
	if err = os.Mkdir(hostPath, 0700); err != nil {
		return nil, err
	}
	diskPath := filepath.Join(tmp, "disk.qcow2")
	disk := exec.Command(
		"qemu-img",
		"create",
		"-f", "qcow2",
		"-b", cfg.BackingImagePath,
		"-f", "qcow2",
		diskPath,
	)
	out, err := disk.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("creating VM disk: %v\n%s", err, string(out))
	}
	fwds := map[int]int{}
	for fwd := range cfg.PortForwards {
		fwds[fwd] = <-u.ports
	}

	ret := &VM{
		cfg:      cfg,
		u:        u,
		hostPath: hostPath,
		diskPath: diskPath,
		mac:      randomMAC(),
		forwards: fwds,
		ipv4:     <-u.ipv4s,
		ipv6:     <-u.ipv6s,
	}
	ret.cmd = exec.CommandContext(
		u.Context(),
		"qemu-system-x86_64",
		"-enable-kvm",
		"-m", strconv.Itoa(ret.cfg.MemoryMiB),
		"-device", "virtio-net,netdev=net0,mac=52:54:00:12:34:56",
		"-device", fmt.Sprintf("virtio-net,netdev=net1,mac=%s", ret.mac),
		"-device", "virtio-rng-pci,rng=rng0",
		"-device", "virtio-serial",
		"-object", "rng-random,filename=/dev/urandom,id=rng0",
		"-netdev", fmt.Sprintf("user,id=net0,hostname=%s,%s", ret.cfg.Hostname, makeForwards(ret.forwards)),
		"-netdev", fmt.Sprintf("vde,id=net1,sock=%s", u.switchSock()),
		"-drive", fmt.Sprintf("if=virtio,file=%s,media=disk", ret.diskPath),
		"-virtfs", fmt.Sprintf("local,path=%s,mount_tag=host0,security_model=none,id=host0", ret.hostPath),
	)
	if !cfg.Display {
		ret.cmd.Args = append(ret.cmd.Args,
			"-nographic",
			"-serial", "null",
			"-monitor", "none",
		)
	}

	return ret, nil
}

// Dir returns the path to the directory that is shared with the
// running VM.
func (v *VM) Dir() string {
	return v.hostPath
}

// Start boots the virtual machine. The universe is destroyed if the
// VM ever shuts down.
func (v *VM) Start() error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.started {
		return errors.New("already started")
	}
	v.started = true

	ips := []string{v.ipv4, v.ipv6}
	if err := ioutil.WriteFile(filepath.Join(v.Dir(), "ip"), []byte(strings.Join(ips, "\n")), 0644); err != nil {
		return err
	}

	if v.cfg.BootScriptPath != "" {
		bs, err := ioutil.ReadFile(v.cfg.BootScriptPath)
		if err != nil {
			return err
		}
		if err := ioutil.WriteFile(filepath.Join(v.Dir(), "bootscript.sh"), bs, 0755); err != nil {
			return err
		}
	} else if v.cfg.BootScript != nil {
		if err := ioutil.WriteFile(filepath.Join(v.Dir(), "bootscript.sh"), v.cfg.BootScript, 0755); err != nil {
			return err
		}
	}

	if err := v.cmd.Start(); err != nil {
		v.u.Close()
		return err
	}

	// Destroy the universe if the VM exits.
	go func() {
		v.cmd.Wait()
		// TODO: better logging and stuff
		v.u.Close()
	}()

	return nil
}

// WaitReady waits until the VM's bootscript creates the boot-done
// file in the shared host directory.
func (v *VM) WaitReady(ctx context.Context) error {
	stop := ctx.Done()
	for {
		select {
		case <-stop:
			return errors.New("timeout")
		default:
		}

		_, err := os.Stat(filepath.Join(v.Dir(), "boot-done"))
		if err != nil {
			if os.IsNotExist(err) {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			return err
		}
		return nil
	}
}

// ForwardedPort returns the port on localhost that maps to the given
// port on the VM.
func (v *VM) ForwardedPort(dst int) int {
	return v.forwards[dst]
}

func (v *VM) IPv4() string { return v.ipv4 }
func (v *VM) IPv6() string { return v.ipv6 }
