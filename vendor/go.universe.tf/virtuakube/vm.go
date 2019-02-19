package virtuakube

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/crypto/ssh"

	"go.universe.tf/virtuakube/internal/config"
)

// VMConfig is the configuration for a virtual machine.
type VMConfig struct {
	Name         string
	Image        string
	MemoryMiB    int
	Networks     []string
	PortForwards map[int]bool

	// Only available to image builder.
	*kernelConfig
}

// kernelConfig is the configuration for booting qemu with an external
// kernel.
type kernelConfig struct {
	diskPath   string
	kernelPath string
	initrdPath string
	cmdline    string
}

// VM is a virtual machine.
type VM struct {
	cfg *config.VM

	// Closed when the VM has exited.
	stopped chan bool

	universeStartTime time.Time
	universeOpenTime  time.Time

	commandLog io.Writer

	mu sync.Mutex

	// Qemu subprocess that runs the VM.
	cmd *exec.Cmd

	// Link to the qemu monitor. Used to handle freezing.
	monIn  io.WriteCloser
	monOut io.ReadCloser

	// SSH connection to the VM.
	ssh *ssh.Client

	// Tracking the state of the VM to enable/disable parts of the
	// API.
	started bool
	closed  bool
}

func (u *Universe) mkVM(cfg *config.VM, kernel *kernelConfig, resume bool) (*VM, error) {
	ret := &VM{
		cfg:               cfg,
		stopped:           make(chan bool),
		universeStartTime: u.cfg.Snapshots[u.activeSnapshot].Clock,
		universeOpenTime:  u.startTime,
		commandLog:        u.runtimecfg.CommandLog,
	}

	ret.cmd = exec.Command(
		"qemu-system-x86_64",
		"-machine", "q35",
		"-m", strconv.Itoa(cfg.MemoryMiB),
		"-device", "virtio-net,netdev=net0,mac=52:54:00:12:34:56",
		"-device", "virtio-rng-pci,rng=rng0",
		"-device", "virtio-serial",
		"-object", "rng-random,filename=/dev/urandom,id=rng0",
		"-netdev", fmt.Sprintf("user,id=net0,%s", makeForwards(cfg.PortForwards)),
		"-drive", fmt.Sprintf("if=virtio,file=%s,media=disk", cfg.DiskFile),
		"-rtc", "clock=vm",
		"-serial", "null",
		"-monitor", "stdio",
		"-S",
		"-enable-kvm",
	)

	if !u.runtimecfg.VMGraphics {
		ret.cmd.Args = append(ret.cmd.Args, "-nographic")
	}

	for i, net := range cfg.Networks {
		ret.cmd.Args = append(ret.cmd.Args,
			"-device", fmt.Sprintf("virtio-net,netdev=net%d,addr=%d,mac=%s", i+1, i+5, cfg.MAC[net]),
			"-netdev", fmt.Sprintf("vde,id=net%d,sock=%s", i+1, u.networks[net].sock),
		)
	}

	if kernel != nil {
		ret.cmd.Args = append(ret.cmd.Args,
			"-kernel", kernel.kernelPath,
			"-initrd", kernel.initrdPath,
			"-append", kernel.cmdline,
		)
	}
	if resume {
		ret.cmd.Args = append(ret.cmd.Args, "-loadvm", u.cfg.Snapshots[u.activeSnapshot].ID)
	}
	ret.cmd.Dir = u.dir
	if u.runtimecfg.Interactive {
		ret.cmd.SysProcAttr = &syscall.SysProcAttr{
			Setpgid: true,
		}
	}
	ret.cmd.Stderr = os.Stderr

	monIn, err := ret.cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdin pipe: %v", err)
	}
	ret.monIn = monIn
	monOut, err := ret.cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdout pipe: %v", err)
	}
	ret.monOut = monOut

	if err := ret.cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting VM: %v", err)
	}
	go func() {
		ret.cmd.Wait()
		close(ret.stopped)
	}()

	if _, err := readToPrompt(ret.monOut); err != nil {
		ret.Close()
		return nil, fmt.Errorf("reading qemu monitor prompt: %v", err)
	}

	u.vms[cfg.Name] = ret
	return ret, nil
}

// NewVM creates an unstarted virtual machine with the given configuration.
func (u *Universe) NewVM(cfg *VMConfig) (*VM, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.newVMWithLock(cfg)
}

func (u *Universe) newVMWithLock(cfg *VMConfig) (*VM, error) {
	if cfg == nil {
		return nil, errors.New("no VMConfig specified")
	}

	if u.vms[cfg.Name] != nil {
		return nil, fmt.Errorf("universe already has a VM named %q", cfg.Name)
	}

	vmcfg := &config.VM{
		Name:         cfg.Name,
		DiskFile:     randomDiskName(),
		MemoryMiB:    cfg.MemoryMiB,
		PortForwards: map[int]int{},
		Networks:     cfg.Networks,
		MAC:          map[string]string{},
		IPv4:         map[string]net.IP{},
		IPv6:         map[string]net.IP{},
	}
	if vmcfg.Name == "" {
		vmcfg.Name = randomHostname()
	}
	if vmcfg.MemoryMiB == 0 {
		vmcfg.MemoryMiB = 1024
	}
	for _, net := range vmcfg.Networks {
		nw := u.networks[net]
		if nw == nil {
			return nil, fmt.Errorf("universe doesn't have a network named %q", net)
		}
		ip4, ip6 := nw.ip()
		vmcfg.MAC[net] = randomMAC()
		vmcfg.IPv4[net] = ip4
		vmcfg.IPv6[net] = ip6
	}
	wantPorts := []int{}
	if cfg.PortForwards == nil {
		cfg.PortForwards = map[int]bool{}
	}
	cfg.PortForwards[22] = true
	for fwd := range cfg.PortForwards {
		wantPorts = append(wantPorts, fwd)
	}
	sort.Ints(wantPorts)
	for _, fwd := range wantPorts {
		vmcfg.PortForwards[fwd] = u.port()
	}

	img := u.images[cfg.Image]
	if img == "" {
		return nil, fmt.Errorf("universe doesn't have an image named %q", cfg.Image)
	}

	if cfg.kernelConfig == nil {
		disk := exec.Command(
			"qemu-img",
			"create",
			"-f", "qcow2",
			"-b", filepath.Join(u.dir, img),
			"-f", "qcow2",
			filepath.Join(u.dir, vmcfg.DiskFile),
		)
		out, err := disk.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("creating VM disk: %v\n%s", err, string(out))
		}
	} else {
		vmcfg.DiskFile = img
	}

	vm, err := u.mkVM(vmcfg, cfg.kernelConfig, false)
	if err != nil {
		return nil, fmt.Errorf("creating VM: %v", err)
	}

	return vm, nil
}

func (u *Universe) resumeVM(cfg *config.VM) (*VM, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.mkVM(cfg, nil, true)
}

// Start starts the virtual machine and waits for it to finish
// booting.
func (v *VM) Start() error {
	if err := v.boot(); err != nil {
		return err
	}

	if _, err := v.Run("hostnamectl set-hostname " + v.cfg.Name); err != nil {
		v.Close()
		return err
	}

	for i, net := range v.cfg.Networks {
		interfaceID := i + 5 // the PCI slot layout on these VMs means the NICs start at ens4.
		err := v.RunMultiple(
			fmt.Sprintf("ip addr add %s/24 dev enp0s%d", v.cfg.IPv4[net], interfaceID),
			fmt.Sprintf("ip addr add %s/24 dev enp0s%d", v.cfg.IPv6[net], interfaceID),
			fmt.Sprintf("ip link set dev enp0s%d up", interfaceID),
		)
		if err != nil {
			v.Close()
			return err
		}
	}

	return nil
}

// boot starts the VM process and waits for SSH to establish.
func (v *VM) boot() error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.started {
		return errors.New("already started")
	}

	if _, err := fmt.Fprintf(v.monIn, "cont\n"); err != nil {
		v.closeWithLock()
		return err
	}
	if _, err := readToPrompt(v.monOut); err != nil {
		v.closeWithLock()
		return err
	}

	// Try dialing SSH
	for {
		select {
		case <-v.stopped:
			return errors.New("timeout")
		default:
		}

		sshCfg := &ssh.ClientConfig{
			User:            "root",
			Auth:            []ssh.AuthMethod{ssh.Password("root")},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			Timeout:         time.Second,
		}

		client, err := ssh.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", v.ForwardedPort(22)), sshCfg)
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		v.ssh = client
		break
	}

	sess, err := v.ssh.NewSession()
	if err != nil {
		return err
	}
	if _, err := v.runWithSession(sess, "timedatectl set-ntp false", nil); err != nil {
		return err
	}
	sess, err = v.ssh.NewSession()
	if err != nil {
		return err
	}
	if _, err := v.runWithSession(sess, fmt.Sprintf("timedatectl set-time %q", v.universeStartTime.Add(time.Since(v.universeOpenTime)).Format("2006-01-02 15:04:05")), nil); err != nil {
		return err
	}

	return nil
}

// Wait waits for the VM to shut down.
func (v *VM) Wait(ctx context.Context) error {
	select {
	case <-v.stopped:
		return nil
	case <-ctx.Done():
		return errors.New("timeout")
	}
}

// Run runs the given shell command as root on the VM, and returns its
// output.
func (v *VM) Run(command string) ([]byte, error) {
	v.mu.Lock()
	sess, err := v.ssh.NewSession()
	v.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return v.runWithSession(sess, command, nil)
}

func (v *VM) RunWithInput(command string, stdin io.Reader) ([]byte, error) {
	v.mu.Lock()
	sess, err := v.ssh.NewSession()
	v.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return v.runWithSession(sess, command, stdin)
}

// does not hold v.mu, you can't access any protected members!
func (v *VM) runWithSession(sess *ssh.Session, command string, stdin io.Reader) ([]byte, error) {
	defer sess.Close()
	var out bytes.Buffer
	sess.Stdin = stdin
	sess.Stdout = &out
	sess.Stderr = &out
	if v.commandLog != nil {
		sess.Stdout = io.MultiWriter(&out, v.commandLog)
		sess.Stderr = sess.Stdout
		fmt.Fprintf(v.commandLog, "[%s] %s\n", v.cfg.Name, command)
	}

	if err := sess.Run(command); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// RunMultiple runs all given commands sequentially. It stops at the
// first unsuccessful command and returns its error.
func (v *VM) RunMultiple(commands ...string) error {
	for _, cmd := range commands {
		if _, err := v.Run(cmd); err != nil {
			return err
		}
	}
	return nil
}

// WriteFile writes bs to the given path on the VM.
func (v *VM) WriteFile(path string, bs []byte) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	sess, err := v.ssh.NewSession()
	if err != nil {
		return err
	}
	defer sess.Close()
	sess.Stdin = bytes.NewBuffer(bs)
	if v.commandLog != nil {
		fmt.Fprintf(v.commandLog, "[%s] (write file %s)\n", v.cfg.Name, path)
	}

	return sess.Run("cat >" + path)
}

// ReadFile reads path from the VM and returns its contents.
func (v *VM) ReadFile(path string) ([]byte, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	sess, err := v.ssh.NewSession()
	if err != nil {
		return nil, err
	}
	defer sess.Close()
	if v.commandLog != nil {
		fmt.Fprintf(v.commandLog, "[%s] (read file %s)\n", v.cfg.Name, path)
	}
	return sess.Output("cat " + path)
}

// Dial connects to the given destination, through the VM.
func (v *VM) Dial(network, addr string) (net.Conn, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	return v.ssh.Dial(network, addr)
}

// Close shuts down the VM, reverting all changes since the universe
// was last saved.
func (v *VM) Close() error {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.closeWithLock()
}

func (v *VM) closeWithLock() error {
	if v.closed {
		return nil
	}
	v.closed = true
	v.cmd.Process.Kill()
	<-v.stopped
	return nil
}

func (v *VM) freeze(snapshot string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.closed {
		return errors.New("cannot freeze closed VM")
	}
	v.closed = true

	// Stop the VM CPUs, so we're not competing with the VM while
	// snapshotting.
	if _, err := fmt.Fprintf(v.monIn, "stop\n"); err != nil {
		return err
	}
	if _, err := readToPrompt(v.monOut); err != nil {
		return err
	}

	// Write the snapshot.
	if _, err := fmt.Fprintf(v.monIn, "savevm %s\n", snapshot); err != nil {
		return err
	}
	if _, err := readToPrompt(v.monOut); err != nil {
		return err
	}

	// Shut down qemu. We don't expect a monitor response here,
	// instead we wait for the context to get canceled, which will get
	// triggered by the goroutine that's waiting for the qemu process
	// to exit.
	if _, err := fmt.Fprintf(v.monIn, "quit\n"); err != nil {
		return err
	}
	<-v.stopped

	return nil
}

// Hostname returns the configured hostname of the VM. It might be
// different from the VM's actual hostname if its hostname was changed
// after boot by something other than virtuakube.
func (v *VM) Hostname() string {
	return v.cfg.Name
}

// Networks returns the networks to which the VM is connected.
func (v *VM) Networks() []string {
	ret := []string{}
	for _, net := range v.cfg.Networks {
		ret = append(ret, net)
	}
	return ret
}

// ForwardedPort returns the port on localhost that maps to the given
// port on the VM.
func (v *VM) ForwardedPort(dst int) int {
	return v.cfg.PortForwards[dst]
}

// IPv4 returns the LAN IPv4 address of the VM.
func (v *VM) IPv4(network string) net.IP { return v.cfg.IPv4[network] }

// IPv6 returns the LAN IPv6 address of the VM.
func (v *VM) IPv6(network string) net.IP { return v.cfg.IPv6[network] }

var (
	qemuPrompt = []byte("\r\n(qemu) ")
	ansiCSI_K  = []byte("\x1b[K")
)

// Read to the next (qemu) prompt, and return whatever was before
// that.
func readToPrompt(r io.Reader) (string, error) {
	var buf bytes.Buffer
	b := make([]byte, 100)
	for {
		n, err := r.Read(b)
		if err != nil {
			return "", err
		}
		buf.Write(b[:n])
		have := buf.Bytes()
		if bytes.HasSuffix(have, qemuPrompt) {
			buf.Reset()
			ret := bytes.TrimSuffix(have, qemuPrompt)
			if i := bytes.LastIndex(ret, ansiCSI_K); i != -1 {
				ret = ret[i+len(ansiCSI_K):]
			}
			return strings.TrimSpace(strings.Replace(string(ret), "\r\n", "\n", -1)), nil
		}
	}
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

func randomHostname() string {
	rnd := make([]byte, 6)
	if _, err := rand.Read(rnd); err != nil {
		panic("system ran out of randomness")
	}
	return fmt.Sprintf("vm%x", rnd)
}

func randomDiskName() string {
	rnd := make([]byte, 16)
	if _, err := rand.Read(rnd); err != nil {
		panic("system ran out of randomness")
	}
	return fmt.Sprintf("disk-%x", rnd)
}

// Make a series of "hostfwd" statements for the qemu commandline.
func makeForwards(fwds map[int]int) string {
	var ret []string
	for dst, src := range fwds {
		ret = append(ret, fmt.Sprintf("hostfwd=tcp:127.0.0.1:%d-:%d", src, dst))
	}
	return strings.Join(ret, ",")
}
