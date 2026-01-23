package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
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
	socketName          = "frr-reloader.sock"
)

type reloadRequest struct {
	Action string `json:"action"`
	ID     int    `json:"id"`
}

type reloadResponse struct {
	ID     int  `json:"id"`
	Result bool `json:"result"`
}

type reloader struct {
	sharedVolume   string
	pidFile        string
	fileToReload   string
	lockFile       string
	statusFile     string
	socketPath     string
	lockFd         *os.File
	reloadReqChan  chan reloadRequest
	reloadRespChan chan reloadResponse
}

func newReloader() (*reloader, error) {
	sharedVolume := os.Getenv("SHARED_VOLUME")
	if sharedVolume == "" {
		sharedVolume = defaultSharedVolume
	}

	r := &reloader{
		sharedVolume:   sharedVolume,
		pidFile:        filepath.Join(sharedVolume, "reloader.pid"),
		fileToReload:   filepath.Join(sharedVolume, "frr.conf"),
		lockFile:       filepath.Join(sharedVolume, "lock"),
		statusFile:     filepath.Join(sharedVolume, ".status"),
		socketPath:     filepath.Join(sharedVolume, socketName),
		reloadReqChan:  make(chan reloadRequest),
		reloadRespChan: make(chan reloadResponse),
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

func (r *reloader) startHTTPServer(ctx context.Context) error {
	// Remove existing socket file if it exists
	os.Remove(r.socketPath)

	listener, err := net.Listen("unix", r.socketPath)
	if err != nil {
		return fmt.Errorf("failed to create unix socket: %w", err)
	}

	// Set socket permissions
	if err := os.Chmod(r.socketPath, 0600); err != nil {
		listener.Close()
		return fmt.Errorf("failed to set socket permissions: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", r.handleReload)

	server := &http.Server{Handler: mux}

	go func() {
		<-ctx.Done()
		if err := server.Shutdown(context.Background()); err != nil {
			log.Println(err)
		}
		listener.Close()
	}()

	go func() {
		log.Printf("Starting HTTP server on Unix socket: %s", r.socketPath)
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	return nil
}

func (r *reloader) handleReload(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var reloadReq reloadRequest
	if err := json.NewDecoder(req.Body).Decode(&reloadReq); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if reloadReq.Action != "reload" {
		http.Error(w, "Invalid action", http.StatusBadRequest)
		return
	}

	// Send reload request to channel
	r.reloadReqChan <- reloadReq

	// Wait for response
	resp := <-r.reloadRespChan

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Println(err)
	}
}

func (r *reloader) run() {
	// Setup signal handlers
	sigHup := make(chan os.Signal, 1)
	sigTerm := make(chan os.Signal, 1)
	signal.Notify(sigHup, syscall.SIGHUP)
	signal.Notify(sigTerm, syscall.SIGTERM, syscall.SIGINT)

	// Start HTTP server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := r.startHTTPServer(ctx); err != nil {
		log.Fatalf("Failed to start HTTP server: %v", err)
	}

	// Main event loop
	for {
		select {
		case <-sigHup:
			// Handle reload in a goroutine to allow queueing via flock
			go r.reloadFRR()
		case req := <-r.reloadReqChan:
			// Handle reload request from HTTP server
			go func() {
				result := r.reloadFRR()
				r.reloadRespChan <- reloadResponse{
					ID:     req.ID,
					Result: result,
				}
			}()
		case <-sigTerm:
			log.Println("Caught an exit signal..")
			cancel()
			r.cleanup()
			return
		}
	}
}

func (r *reloader) reloadFRR() bool {
	// Acquire exclusive lock (flock 200 in bash)
	if err := syscall.Flock(int(r.lockFd.Fd()), syscall.LOCK_EX); err != nil {
		log.Printf("Failed to acquire lock: %v", err)
		return false
	}
	defer func() {
		if err := syscall.Flock(int(r.lockFd.Fd()), syscall.LOCK_UN); err != nil {
			log.Print(err)
		}
	}()

	log.Println("Caught SIGHUP and acquired lock! Reloading FRR..")
	startTime := time.Now()

	// Check configuration file syntax
	log.Println("Checking the configuration file syntax")
	if !r.runFRRReload("--test", startTime) {
		return false
	}

	// Apply configuration
	log.Println("Applying the configuration file")
	if !r.runFRRReload("--reload --overwrite", startTime) {
		return false
	}

	elapsed := time.Since(startTime)
	log.Printf("FRR reloaded successfully! %d seconds", int(elapsed.Seconds()))
	r.writeStatus("success")
	return true
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
	os.Remove(r.socketPath)
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
