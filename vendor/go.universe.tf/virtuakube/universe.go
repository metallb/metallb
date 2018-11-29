package virtuakube

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

var universeTools = []string{
	"vde_switch",
	"qemu-system-x86_64",
	"qemu-img",
}

// checkTools returns an error if a command required by virtuakube is
// not available on the system.
func checkTools(tools []string) error {
	missing := []string{}
	for _, tool := range tools {
		_, err := exec.LookPath(tool)
		if err != nil {
			if e, ok := err.(*exec.Error); ok && e.Err == exec.ErrNotFound {
				missing = append(missing, tool)
				continue
			}
			return err
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("required tools missing: %s", strings.Join(missing, ", "))
	}
	return nil
}

// A Universe is a virtual test network and its associated resources.
type Universe struct {
	VMBaseImage string

	tmpdir   string
	ctx      context.Context
	shutdown context.CancelFunc
	ports    chan int
	ipv4s    chan string
	ipv6s    chan string

	swtch *exec.Cmd
	sock  string

	closeMu  sync.Mutex
	closed   bool
	closeErr error
}

// New creates a new virtual universe. The ctx controls the overall
// lifetime of the universe, i.e. if the context is canceled or times
// out, the universe will be destroyed.
func New(ctx context.Context) (*Universe, error) {
	if err := checkTools(universeTools); err != nil {
		return nil, err
	}

	p, err := ioutil.TempDir("", "virtuakube")
	if err != nil {
		return nil, err
	}

	ctx, shutdown := context.WithCancel(ctx)

	sock := filepath.Join(p, "switch")

	ret := &Universe{
		tmpdir:   p,
		ctx:      ctx,
		shutdown: shutdown,
		ports:    make(chan int),
		ipv4s:    make(chan string),
		ipv6s:    make(chan string),
		swtch: exec.CommandContext(
			ctx,
			"vde_switch",
			"--sock", sock,
			"-m", "0600",
		),
		sock: sock,
	}

	if err := ret.swtch.Start(); err != nil {
		ret.Close()
		return nil, err
	}
	// Destroy the universe if the virtual switch exits
	go func() {
		ret.swtch.Wait()
		// TODO: logging and stuff
		ret.Close()
	}()
	// Destroy the universe if the parent context cancels
	go func() {
		<-ctx.Done()
		ret.Close()
	}()
	go func() {
		port := 50000
		for {
			select {
			case ret.ports <- port:
				port++
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() {
		ip := net.IPv4(192, 168, 50, 1).To4()
		for {
			select {
			case ret.ipv4s <- ip.String():
				ip[3]++
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() {
		ip := net.ParseIP("fd00::1")
		for {
			select {
			case ret.ipv6s <- ip.String():
				ip[15]++
			case <-ctx.Done():
				return
			}
		}
	}()

	return ret, nil
}

// Tmpdir creates a temporary directory and returns its absolute
// path. The directory will be cleaned up when the universe is
// destroyed.
func (u *Universe) Tmpdir(prefix string) (string, error) {
	p, err := ioutil.TempDir(u.tmpdir, prefix)
	if err != nil {
		return "", err
	}
	return p, nil
}

// Context returns a context that gets canceled when the universe is
// destroyed.
func (u *Universe) Context() context.Context {
	return u.ctx
}

// Close destroys the universe, freeing up processes and temporary
// files.
func (u *Universe) Close() error {
	u.closeMu.Lock()
	defer u.closeMu.Unlock()
	if u.closed {
		return u.closeErr
	}
	u.closed = true

	u.shutdown()

	u.closeErr = os.RemoveAll(u.tmpdir)
	return u.closeErr
}

// Wait waits for the universe to end.
func (u *Universe) Wait(ctx context.Context) error {
	select {
	case <-u.ctx.Done():
		return nil
	case <-ctx.Done():
		return errors.New("timeout")
	}
}

func (u *Universe) switchSock() string {
	return u.sock
}
