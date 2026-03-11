// SPDX-License-Identifier:Apache-2.0

package main

import (
	"flag"
	"fmt"
	stdlog "log"
	"net/http"
	"os"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"go.universe.tf/metallb/frr-tools/metrics/collector"
	"go.universe.tf/metallb/frr-tools/metrics/liveness"
	"go.universe.tf/metallb/frr-tools/metrics/vtysh"
	"go.universe.tf/metallb/internal/logging"
	"go.universe.tf/metallb/internal/tlsconfig"
	"go.universe.tf/metallb/internal/version"
)

var (
	metricsPort         = flag.Uint("metrics-port", 9121, "Port to listen on for web interface.")
	metricsBindAddress  = flag.String("metrics-bind-address", "0.0.0.0", "The address the metric endpoint binds to.")
	metricsPath         = flag.String("metrics-path", "/metrics", "Path under which to expose metrics.")
	tlsCertFile         = flag.String("tls-cert-file", "", "x509 certificate for HTTPS. If empty, a self-signed cert is auto-generated.")
	tlsKeyFile          = flag.String("tls-private-key-file", "", "x509 private key matching --tls-cert-file.")
	tlsCipherSuites     = flag.String("tls-cipher-suites", "", "Comma-separated list of TLS cipher suites. Only applies to TLS 1.2. If empty, uses Go defaults.")
	tlsCurvePreferences = flag.String("tls-curve-preferences", "", "Comma-separated list of numeric CurveID values (see https://pkg.go.dev/crypto/tls#CurveID). If empty, uses Go defaults.")
	tlsMinVersionFlag   = flag.String("tls-min-version", "", "Minimum TLS version (VersionTLS12 or VersionTLS13). If empty, defaults to VersionTLS13.")
)

func main() {
	flag.Parse()

	logger, err := logging.Init("error")
	if err != nil {
		fmt.Printf("failed to initialize logging: %s\n", err)
		os.Exit(1)
	}

	level.Info(logger).Log("version", version.Version(), "commit", version.CommitHash(), "branch", version.Branch(), "goversion", version.GoString(), "msg", "FRR metrics exporter starting "+version.String())

	mux := http.NewServeMux()
	metricsHandler, err := newMetricsHandler(logger)
	if err != nil {
		level.Error(logger).Log("msg", "failed to create metrics handler", "error", err)
		os.Exit(1)
	}
	mux.Handle(*metricsPath, metricsHandler)
	mux.Handle("/livez", liveness.Handler(vtysh.Run, logger))

	tlsOpt, err := tlsconfig.OptFor(*tlsCipherSuites, *tlsCurvePreferences, *tlsMinVersionFlag)
	if err != nil {
		level.Error(logger).Log("msg", "failed to parse TLS flags", "error", err)
		os.Exit(1)
	}
	tlsCfg, err := tlsconfig.For(tlsOpt, *tlsCertFile, *tlsKeyFile)
	if err != nil {
		level.Error(logger).Log("msg", "failed to configure TLS", "error", err)
		os.Exit(1)
	}

	srv := &http.Server{
		Addr:        fmt.Sprintf("%s:%d", *metricsBindAddress, *metricsPort),
		ReadTimeout: 3 * time.Second,
		Handler:     mux,
		TLSConfig:   tlsCfg,
	}

	level.Info(logger).Log("msg", "Starting exporter", "metricsPath", *metricsPath, "port", *metricsPort)

	if err := srv.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
		level.Error(logger).Log("error", err)
		os.Exit(1)
	}
}

func newMetricsHandler(logger log.Logger) (http.Handler, error) {
	handler := promHandler(logger)
	filter, err := rbacFilter()
	if err != nil {
		return nil, err
	}
	return filter(ctrl.Log.WithName("metrics-auth"), handler)
}

func promHandler(logger log.Logger) http.Handler {
	BGPCollector := collector.NewBGP(logger)
	BFDCollector := collector.NewBFD(logger)

	registry := prometheus.NewRegistry()
	registry.MustRegister(BGPCollector)
	registry.MustRegister(BFDCollector)

	return promhttp.HandlerFor(
		prometheus.Gatherers{prometheus.DefaultGatherer, registry},
		promhttp.HandlerOpts{
			ErrorLog:      stdlog.New(log.NewStdlibAdapter(level.Error(logger)), "", 0),
			ErrorHandling: promhttp.ContinueOnError,
			Registry:      registry,
		},
	)
}

func rbacFilter() (metricsserver.Filter, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("getting in-cluster config: %w", err)
	}
	httpClient, err := rest.HTTPClientFor(config)
	if err != nil {
		return nil, fmt.Errorf("creating HTTP client: %w", err)
	}
	return filters.WithAuthenticationAndAuthorization(config, httpClient)
}
