package virtuakube

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.universe.tf/virtuakube/internal/config"
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

// UniverseConfig contains the ephemeral runtime settings for the
// universe. A nil UniverseConfig is equivalent to the zero value.
type UniverseConfig struct {
	// If non-nil, all commands executed on VMs and during image
	// building are logged to this writer, along with their
	// stdout/stderr.
	CommandLog io.Writer
	// Whether VMs should have a GUI. Useful for debugging Virtuakube
	// itself.
	VMGraphics bool
	// Make subprocesses immune to ^C, to enable interactive control.
	Interactive bool
}

// A Universe is a virtual sandbox and its associated resources.
type Universe struct {
	// Root containing all the stuff in the universe.
	dir string

	tmpdir string

	// Channel that gets closed right at the end of Close, Destroy, or
	// Save. Wait waits for this channel to get closed.
	closedCh chan bool

	// Runtime configuration for this particular run of the
	// universe. Not persisted after Close.
	runtimecfg *UniverseConfig

	// Must hold this mutex to touch any of the following.
	mu sync.Mutex

	// Configuration for the entire universe, loaded from
	// file. Contains data about all snapshots, but isn't updated in
	// realtime while a snapshot is running.
	cfg *config.Universe

	// Current network parameters as they mutate while the universe is
	// running.
	nextPort int
	nextNet  int

	// Name of the currently running snapshot.
	activeSnapshot string

	// Time at which the currently active snapshot was started. We use
	// this, in combination with the snapshotted universe time, to
	// figure out what time it is for the running snapshot.
	startTime time.Time

	// Resources in the universe: a virtual switch, some VMs, some k8s
	// clusters.
	networks map[string]*Network
	images   map[string]string // name -> diskpath
	vms      map[string]*VM
	clusters map[string]*Cluster

	// Records any close errors, so we can do concurrent-safe
	// shutdown.
	closed   bool
	closeErr error
}

// Create creates a new empty Universe in dir. The directory must not
// already exist.
func Create(dir string, runtimecfg *UniverseConfig) (*Universe, error) {
	cfg := &config.Universe{
		Snapshots: map[string]*config.Snapshot{
			"": {
				Name:     "",
				ID:       randomSnapshotID(),
				NextPort: 50000,
				Clock:    time.Now(),
			},
		},
	}

	dir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}

	if err := os.Mkdir(dir, 0700); err != nil {
		return nil, err
	}

	cfgPath := filepath.Join(dir, "config.json")

	if err := config.Write(cfgPath, cfg); err != nil {
		return nil, err
	}

	return Open(dir, "", runtimecfg)
}

// Open opens the existing Universe in dir, and resumes from snapshot.
func Open(dir string, snapshot string, runtimecfg *UniverseConfig) (*Universe, error) {
	if err := checkTools(universeTools); err != nil {
		return nil, err
	}

	dir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}

	if runtimecfg == nil {
		runtimecfg = &UniverseConfig{}
	}

	cfgPath := filepath.Join(dir, "config.json")
	cfg, err := config.Read(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("reading universe config: %v", err)
	}

	snap := cfg.Snapshots[snapshot]
	if snap == nil {
		return nil, fmt.Errorf("no snapshot %q in universe", snapshot)
	}

	tmpdir, err := ioutil.TempDir(dir, "tmp")
	if err != nil {
		return nil, fmt.Errorf("creating temporary directory: %v", err)
	}

	ret := &Universe{
		dir:            dir,
		tmpdir:         tmpdir,
		closedCh:       make(chan bool),
		cfg:            cfg,
		runtimecfg:     runtimecfg,
		nextPort:       snap.NextPort,
		nextNet:        snap.NextNet,
		activeSnapshot: snapshot,
		startTime:      time.Now(),
		networks:       map[string]*Network{},
		images:         map[string]string{},
		vms:            map[string]*VM{},
		clusters:       map[string]*Cluster{},
	}

	for _, img := range snap.Images {
		ret.images[img.Name] = img.File
	}

	// thaw networks first, because VMs need them.
	for _, net := range snap.Networks {
		if err := ret.resumeNetwork(net); err != nil {
			return nil, err
		}
	}

	// thaw all VMs concurrently. This is the expensive step where
	// they struggle to load their huge memory snapshots. At the end
	// of thaw, they're fully loaded, but with their CPUs stopped.
	//
	// TODO: this isn't actually parallel because we lock the universe
	// in each resumeVM call *headdesk*
	vms := snap.VMs
	res := make(chan error, len(vms))
	for _, vmcfg := range vms {
		go func(vmcfg *config.VM) {
			_, err := ret.resumeVM(vmcfg)
			if err != nil {
				res <- err
				return
			}

			res <- nil
		}(vmcfg)
	}
	for range vms {
		if err := <-res; err != nil {
			return nil, err
		}
	}

	// Now that the expensive load is done, blow through all VMs and
	// restart their CPUs in rapid succession, to keep the clock skew
	// between VMs minimal.
	for _, vm := range ret.vms {
		if err := vm.boot(); err != nil {
			return nil, err
		}
	}

	// Thaw all cluster objects, now that the cluster VMs are running.
	for _, clusterCfg := range snap.Clusters {
		if err := ret.resumeCluster(clusterCfg); err != nil {
			return nil, err
		}
	}

	return ret, nil
}

// Close closes the universe, discarding all changes since the last
// call to Save.
func (u *Universe) Close() error {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.closed {
		return u.closeErr
	}

	u.closeWithLock()
	close(u.closedCh)
	return u.closeErr
}

func (u *Universe) closeWithLock() {
	// Assumes u hasn't been closed already. Caller's responsibility
	// to check that.
	u.closed = true

	for _, vm := range u.vms {
		if err := vm.Close(); err != nil {
			u.closeErr = err
		}
	}

	for _, net := range u.networks {
		if err := net.Close(); err != nil {
			u.closeErr = err
		}
	}

	snap := u.cfg.Snapshots[u.activeSnapshot]

	for name, path := range u.images {
		if snap.Images[name] == nil {
			if err := os.Remove(filepath.Join(u.dir, path)); err != nil {
				u.closeErr = err
			}
		}
	}
	for name, vm := range u.vms {
		if snap.VMs[name] == nil {
			if err := os.Remove(filepath.Join(u.dir, vm.cfg.DiskFile)); err != nil {
				u.closeErr = err
			}
		}
	}

	if err := os.RemoveAll(u.tmpdir); err != nil {
		u.closeErr = err
	}
}

// Destroy closes the universe and recursively deletes the universe
// directory.
func (u *Universe) Destroy() error {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.closed {
		return u.closeErr
	}

	u.closeWithLock()

	if err := os.RemoveAll(u.dir); err != nil {
		u.closeErr = err
	}

	close(u.closedCh)
	return u.closeErr
}

// Save snapshots the current state of VMs and clusters, then closes
// the universe.
func (u *Universe) Save(snapshotName string) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.closed {
		return u.closeErr
	}

	snap := &config.Snapshot{
		Name:     snapshotName,
		NextPort: u.nextPort,
		NextNet:  u.nextNet,
		Clock:    u.cfg.Snapshots[u.activeSnapshot].Clock.Add(time.Since(u.startTime)),
		Networks: map[string]*config.Network{},
		Images:   map[string]*config.Image{},
		VMs:      map[string]*config.VM{},
		Clusters: map[string]*config.Cluster{},
	}
	if oldSnap := u.cfg.Snapshots[snapshotName]; oldSnap != nil {
		snap.ID = oldSnap.ID
	} else {
		snap.ID = randomSnapshotID()
	}

	// VM saving is slow, so parallelize it.
	errs := make(chan error, len(u.vms))
	for name, vm := range u.vms {
		go func(name string, vm *VM) {
			if err := vm.freeze(snap.ID); err != nil {
				errs <- fmt.Errorf("freezing %q: %v", name, err)
				return
			}
			errs <- nil
		}(name, vm)
	}
	for range u.vms {
		if err := <-errs; err != nil {
			u.closeErr = err
			return u.closeErr
		}
	}

	for _, network := range u.networks {
		snap.Networks[network.cfg.Name] = network.cfg
	}
	for name, disk := range u.images {
		snap.Images[name] = &config.Image{
			Name: name,
			File: disk,
		}
	}
	for _, vm := range u.vms {
		snap.VMs[vm.cfg.Name] = vm.cfg
	}
	for _, cluster := range u.clusters {
		snap.Clusters[cluster.cfg.Name] = cluster.cfg
	}

	// By now all VMs should have shutdown during their freeze. Kill
	// remaining things. But clear all the new* maps so that
	// closeWithLock doesn't delete stuff we just saved.
	u.networks = nil
	u.images = nil
	u.vms = nil
	u.clusters = nil
	u.closeWithLock()

	u.cfg.Snapshots[snapshotName] = snap

	bs, err := json.MarshalIndent(u.cfg, "", "  ")
	if err != nil {
		u.closeErr = err
		return u.closeErr
	}
	if err := ioutil.WriteFile(filepath.Join(u.dir, "config.json"), bs, 0600); err != nil {
		u.closeErr = err
		return u.closeErr
	}

	close(u.closedCh)
	return nil
}

// Wait waits for the universe to be Closed, Saved or Destroyed.
func (u *Universe) Wait(ctx context.Context) error {
	select {
	case <-u.closedCh:
		return nil
	case <-ctx.Done():
		return errors.New("timeout")
	}
}

func (u *Universe) Snapshots() []string {
	u.mu.Lock()
	defer u.mu.Unlock()
	var ret []string
	for name := range u.cfg.Snapshots {
		ret = append(ret, name)
	}
	return ret
}

func (u *Universe) image(name string) string {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.images[name]
}

func (u *Universe) Command(command string, args ...string) *exec.Cmd {
	cmd := exec.Command(command, args...)
	if u.runtimecfg.CommandLog != nil {
		cmd.Stdout = u.runtimecfg.CommandLog
		cmd.Stderr = u.runtimecfg.CommandLog
	}
	return cmd
}

// VM returns the VM with the given name, or nil if no such VM
// exists in the universe.
func (u *Universe) VM(name string) *VM {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.vms[name]
}

// VM returns a list of all VMs in the universe.
func (u *Universe) VMs() []*VM {
	u.mu.Lock()
	defer u.mu.Unlock()
	ret := make([]*VM, 0, len(u.vms))

	for _, vm := range u.vms {
		ret = append(ret, vm)
	}

	return ret
}

// Cluster returns the Cluster with the given name, or nil if no such
// Cluster exists in the universe.
func (u *Universe) Cluster(name string) *Cluster {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.clusters[name]
}

// Clusters returns a list of all Clusters in the universe.
func (u *Universe) Clusters() []*Cluster {
	u.mu.Lock()
	defer u.mu.Unlock()
	ret := make([]*Cluster, 0, len(u.clusters))

	for _, cluster := range u.clusters {
		ret = append(ret, cluster)
	}

	return ret
}

func (u *Universe) port() int {
	ret := u.nextPort
	u.nextPort++
	return ret
}

func (u *Universe) net() int {
	ret := u.nextNet
	u.nextNet++
	return ret
}

func randomSnapshotID() string {
	rnd := make([]byte, 32)
	if _, err := rand.Read(rnd); err != nil {
		panic("system ran out of randomness")
	}
	return fmt.Sprintf("%x", rnd)
}
