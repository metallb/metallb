package main

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"

	gokitlog "github.com/go-kit/kit/log"
	"go.universe.tf/metallb/internal/config"
)

// serveStatus initializes a status endpoint for each protocol in the protocols
// map at the specified host and port and returns a pointer to an http.Server.
func serveStatus(l gokitlog.Logger, protocols map[config.Proto]Protocol, host string, port int, wg *sync.WaitGroup) *http.Server {
	for p := range protocols {
		http.HandleFunc(fmt.Sprintf("/status/%s", p), protocols[p].StatusHandler())
	}

	addr := net.JoinHostPort(host, strconv.Itoa(port))
	server := &http.Server{Addr: addr}

	go func() {
		defer wg.Done()
		l.Log("op", "startup", "msg", fmt.Sprintf("starting status endpoint at %s", addr))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			l.Log("op", "startup", "error", err, "msg", "listening for status requests")
			os.Exit(1)
		}
		l.Log("op", "shutdown", "msg", "status endpoint shutdown complete")
	}()

	return server
}
