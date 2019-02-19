package virtuakube

import (
	"fmt"
	"net"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"

	"go.universe.tf/virtuakube/internal/config"
)

type NetworkConfig struct {
	Name string
}

type Network struct {
	stopped chan bool

	sock string

	mu  sync.Mutex
	cfg *config.Network
	cmd *exec.Cmd

	closed bool
}

func (u *Universe) NewNetwork(cfg *NetworkConfig) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	if u.networks[cfg.Name] != nil {
		return fmt.Errorf("universe already has a network named %q", cfg.Name)
	}

	netID := u.net()
	return u.mkNetwork(&config.Network{
		Name:     cfg.Name,
		NextIPv4: net.ParseIP(fmt.Sprintf("10.248.%d.1", netID)),
		NextIPv6: net.ParseIP(fmt.Sprintf("fd00:%d::1", netID)),
	})
}

func (u *Universe) resumeNetwork(cfg *config.Network) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.mkNetwork(cfg)
}

func (u *Universe) mkNetwork(cfg *config.Network) error {
	cfg.NextIPv4 = cfg.NextIPv4.To4()

	sock := filepath.Join(u.tmpdir, cfg.Name)
	ret := &Network{
		stopped: make(chan bool),
		cfg:     cfg,
		sock:    sock,
		cmd:     exec.Command("vde_switch", "--sock", sock, "-m", "0600"),
	}
	if u.runtimecfg.Interactive {
		ret.cmd.SysProcAttr = &syscall.SysProcAttr{
			Setpgid: true,
		}
	}

	if err := ret.cmd.Start(); err != nil {
		return err
	}
	go func() {
		ret.cmd.Wait()
		close(ret.stopped)
	}()

	u.networks[cfg.Name] = ret
	return nil
}

func (n *Network) Close() error {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.closed {
		return nil
	}
	n.closed = true
	n.cmd.Process.Kill()
	<-n.stopped
	return nil
}

func (n *Network) ip() (net.IP, net.IP) {
	n.mu.Lock()
	defer n.mu.Unlock()
	ret4, ret6 := n.cfg.NextIPv4, n.cfg.NextIPv6
	n.cfg.NextIPv4, n.cfg.NextIPv6 = make(net.IP, 4), make(net.IP, 16)
	copy(n.cfg.NextIPv4, ret4)
	copy(n.cfg.NextIPv6, ret6)
	n.cfg.NextIPv4[3]++
	n.cfg.NextIPv6[15]++
	return ret4, ret6
}
