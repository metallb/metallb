// SPDX-License-Identifier:Apache-2.0

package main

import (
	"flag"
	"fmt"
	stdlog "log"
	"net/http"
	"os"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/exporter-toolkit/web"

	"go.universe.tf/metallb/frr-metrics/collector"
	"go.universe.tf/metallb/internal/logging"
	"go.universe.tf/metallb/internal/version"
)

var (
	metricsPort   = flag.Uint("metrics-port", 7473, "Port to listen on for web interface.")
	metricsPath   = flag.String("metrics-path", "/metrics", "Path under which to expose metrics.")
	tlsConfigPath = flag.String("tls-config-path", "", "[EXPERIMENTAL] Path to config yaml file that can enable TLS or authentication.")
)

func metricsHandler(logger log.Logger) http.Handler {
	BGPCollector := collector.NewBGP(logger)
	BFDCollector := collector.NewBFD(logger)

	registry := prometheus.NewRegistry()
	registry.MustRegister(BGPCollector)
	registry.MustRegister(BFDCollector)

	gatherers := prometheus.Gatherers{
		prometheus.DefaultGatherer,
		registry,
	}

	handlerOpts := promhttp.HandlerOpts{
		ErrorLog:      stdlog.New(log.NewStdlibAdapter(level.Error(logger)), "", 0),
		ErrorHandling: promhttp.ContinueOnError,
		Registry:      registry,
	}

	return promhttp.HandlerFor(gatherers, handlerOpts)
}

func main() {
	flag.Parse()

	logger, err := logging.Init("error")
	if err != nil {
		fmt.Printf("failed to initialize logging: %s\n", err)
		os.Exit(1)
	}

	_ = level.Info(logger).Log("version", version.Version(), "commit", version.CommitHash(), "branch", version.Branch(), "goversion", version.GoString(), "msg", "FRR metrics exporter starting "+version.String())

	http.Handle(*metricsPath, metricsHandler(logger))
	srv := &http.Server{Addr: fmt.Sprintf(":%d", *metricsPort)}
	_ = level.Info(logger).Log("msg", "Starting exporter", "metricsPath", metricsPath, "port", metricsPort)

	if err := web.ListenAndServe(srv, *tlsConfigPath, logger); err != nil {
		_ = level.Error(logger).Log("error", err)
		os.Exit(1)
	}
}
