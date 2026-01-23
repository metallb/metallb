package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"syscall"
	"time"
)

const (
	defaultSharedVolume = "/etc/frr_reloader"
	frrReloadScript     = "/usr/lib/frr/frr-reload.py"
)

type reloader struct {
	sharedVolume string
	pidFile      string
	fileToReload string
	lockFile     string
	statusFile   string
	lockFd       *os.File
}

func newReloader() (*reloader, error) {
	sharedVolume := os.Getenv("SHARED_VOLUME")
	if sharedVolume == "" {
		sharedVolume = defaultSharedVolume
	}

	r := &reloader{
		sharedVolume: sharedVolume,
		pidFile:      filepath.Join(sharedVolume, "reloader.pid"),
		fileToReload: filepath.Join(sharedVolume, "frr.conf"),
		lockFile:     filepath.Join(sharedVolume, "lock"),
		statusFile:   filepath.Join(sharedVolume, ".status"),
	}

	// Clean up any existing files
	r.cleanFiles()

	// Write PID file
	pid := os.Getpid()
	log.Printf("PID is: %d, writing to %s", pid, r.pidFile)
	if err := os.WriteFile(r.pidFile, []byte(fmt.Sprintf("%d", pid)), 0644); err != nil {
		return nil, fmt.Errorf("failed to write PID file: %w", err)
	}

	// Create lock file
	lockFd, err := os.OpenFile(r.lockFile, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to create lock file: %w", err)
	}
	r.lockFd = lockFd

	return r, nil
}

func (r *reloader) run() {
	// Setup signal handlers
	sigHup := make(chan os.Signal, 1)
	sigTerm := make(chan os.Signal, 1)
	signal.Notify(sigHup, syscall.SIGHUP)
	signal.Notify(sigTerm, syscall.SIGTERM, syscall.SIGINT)

	// Main event loop
	for {
		select {
		case <-sigHup:
			// Handle reload in a goroutine to allow queueing via flock
			go r.reloadFRR()
		case <-sigTerm:
			log.Println("Caught an exit signal..")
			r.cleanup()
			return
		}
	}
}

func (r *reloader) reloadFRR() {
	// Acquire exclusive lock (flock 200 in bash)
	if err := syscall.Flock(int(r.lockFd.Fd()), syscall.LOCK_EX); err != nil {
		log.Printf("Failed to acquire lock: %v", err)
		return
	}
	defer syscall.Flock(int(r.lockFd.Fd()), syscall.LOCK_UN)

	log.Println("Caught SIGHUP and acquired lock! Reloading FRR..")
	startTime := time.Now()

	// Check configuration file syntax
	log.Println("Checking the configuration file syntax")
	if !r.runFRRReload("--test", startTime) {
		return
	}

	// Apply configuration
	log.Println("Applying the configuration file")
	if !r.runFRRReload("--reload --overwrite", startTime) {
		return
	}

	elapsed := time.Since(startTime)
	log.Printf("FRR reloaded successfully! %d seconds", int(elapsed.Seconds()))
	r.writeStatus("success")
}

func (r *reloader) runFRRReload(args string, startTime time.Time) bool {
	cmdStr := fmt.Sprintf("python3 %s %s --stdout %s", frrReloadScript, args, r.fileToReload)
	cmd := exec.Command("sh", "-c", cmdStr)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Printf("Failed to create stdout pipe: %v", err)
		r.writeStatus("failure")
		return false
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Printf("Failed to create stderr pipe: %v", err)
		r.writeStatus("failure")
		return false
	}

	if err := cmd.Start(); err != nil {
		elapsed := time.Since(startTime)
		log.Printf("Failed to start command: %v %d seconds", err, int(elapsed.Seconds()))
		r.writeStatus("failure")
		return false
	}

	// Read and filter output (redact passwords)
	go filterAndPrintOutput(stdout)
	go filterAndPrintOutput(stderr)

	if err := cmd.Wait(); err != nil {
		elapsed := time.Since(startTime)
		if args == "--test" {
			log.Printf("Syntax error spotted: aborting.. %d seconds", int(elapsed.Seconds()))
		} else {
			log.Printf("Failed to fully apply configuration file %d seconds", int(elapsed.Seconds()))
		}
		r.writeStatus("failure")
		return false
	}

	return true
}

func filterAndPrintOutput(r io.Reader) {
	// Regex to match and redact passwords
	passwordRegex := regexp.MustCompile(`password.*`)

	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			output := string(buf[:n])
			output = passwordRegex.ReplaceAllString(output, "password <retracted>")
			fmt.Print(output)
		}
		if err != nil {
			break
		}
	}
}

func (r *reloader) writeStatus(status string) {
	timestamp := time.Now().Unix()
	content := fmt.Sprintf("%d %s", timestamp, status)
	if err := os.WriteFile(r.statusFile, []byte(content), 0644); err != nil {
		log.Printf("Failed to write status file: %v", err)
	}
}

func (r *reloader) cleanFiles() {
	os.Remove(r.pidFile)
	os.Remove(r.lockFile)
}

func (r *reloader) cleanup() {
	r.cleanFiles()
	if r.lockFd != nil {
		r.lockFd.Close()
	}
}

func main() {
	log.SetFlags(0)

	r, err := newReloader()
	if err != nil {
		log.Fatalf("Failed to initialize reloader: %v", err)
	}

	r.run()
}
