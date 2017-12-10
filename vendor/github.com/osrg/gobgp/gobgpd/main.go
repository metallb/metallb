//
// Copyright (C) 2014-2017 Nippon Telegraph and Telephone Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/jessevdk/go-flags"
	"github.com/kr/pretty"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	api "github.com/osrg/gobgp/api"
	"github.com/osrg/gobgp/config"
	"github.com/osrg/gobgp/packet/bgp"
	"github.com/osrg/gobgp/server"
	"github.com/osrg/gobgp/table"
)

var version = "master"

func main() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM)

	var opts struct {
		ConfigFile      string `short:"f" long:"config-file" description:"specifying a config file"`
		ConfigType      string `short:"t" long:"config-type" description:"specifying config type (toml, yaml, json)" default:"toml"`
		LogLevel        string `short:"l" long:"log-level" description:"specifying log level"`
		LogPlain        bool   `short:"p" long:"log-plain" description:"use plain format for logging (json by default)"`
		UseSyslog       string `short:"s" long:"syslog" description:"use syslogd"`
		Facility        string `long:"syslog-facility" description:"specify syslog facility"`
		DisableStdlog   bool   `long:"disable-stdlog" description:"disable standard logging"`
		CPUs            int    `long:"cpus" description:"specify the number of CPUs to be used"`
		GrpcHosts       string `long:"api-hosts" description:"specify the hosts that gobgpd listens on" default:":50051"`
		GracefulRestart bool   `short:"r" long:"graceful-restart" description:"flag restart-state in graceful-restart capability"`
		Dry             bool   `short:"d" long:"dry-run" description:"check configuration"`
		PProfHost       string `long:"pprof-host" description:"specify the host that gobgpd listens on for pprof" default:"localhost:6060"`
		PProfDisable    bool   `long:"pprof-disable" description:"disable pprof profiling"`
		TLS             bool   `long:"tls" description:"enable TLS authentication for gRPC API"`
		TLSCertFile     string `long:"tls-cert-file" description:"The TLS cert file"`
		TLSKeyFile      string `long:"tls-key-file" description:"The TLS key file"`
		Version         bool   `long:"version" description:"show version number"`
	}
	_, err := flags.Parse(&opts)
	if err != nil {
		os.Exit(1)
	}

	if opts.Version {
		fmt.Println("gobgpd version", version)
		os.Exit(0)
	}

	if opts.CPUs == 0 {
		runtime.GOMAXPROCS(runtime.NumCPU())
	} else {
		if runtime.NumCPU() < opts.CPUs {
			log.Errorf("Only %d CPUs are available but %d is specified", runtime.NumCPU(), opts.CPUs)
			os.Exit(1)
		}
		runtime.GOMAXPROCS(opts.CPUs)
	}

	if !opts.PProfDisable {
		go func() {
			log.Println(http.ListenAndServe(opts.PProfHost, nil))
		}()
	}

	switch opts.LogLevel {
	case "debug":
		log.SetLevel(log.DebugLevel)
	case "info":
		log.SetLevel(log.InfoLevel)
	default:
		log.SetLevel(log.InfoLevel)
	}

	if opts.DisableStdlog == true {
		log.SetOutput(ioutil.Discard)
	} else {
		log.SetOutput(os.Stdout)
	}

	if opts.UseSyslog != "" {
		if err := addSyslogHook(opts.UseSyslog, opts.Facility); err != nil {
			log.Error("Unable to connect to syslog daemon, ", opts.UseSyslog)
		}
	}

	if opts.LogPlain {
		if opts.DisableStdlog {
			log.SetFormatter(&log.TextFormatter{
				DisableColors: true,
			})
		}
	} else {
		log.SetFormatter(&log.JSONFormatter{})
	}

	configCh := make(chan *config.BgpConfigSet)
	if opts.Dry {
		go config.ReadConfigfileServe(opts.ConfigFile, opts.ConfigType, configCh)
		c := <-configCh
		if opts.LogLevel == "debug" {
			pretty.Println(c)
		}
		os.Exit(0)
	}

	log.Info("gobgpd started")
	bgpServer := server.NewBgpServer()
	go bgpServer.Serve()

	var grpcOpts []grpc.ServerOption
	if opts.TLS {
		creds, err := credentials.NewServerTLSFromFile(opts.TLSCertFile, opts.TLSKeyFile)
		if err != nil {
			log.Fatalf("Failed to generate credentials: %v", err)
		}
		grpcOpts = []grpc.ServerOption{grpc.Creds(creds)}
	}
	// start grpc Server
	apiServer := api.NewServer(bgpServer, grpc.NewServer(grpcOpts...), opts.GrpcHosts)
	go func() {
		if err := apiServer.Serve(); err != nil {
			log.Fatalf("failed to listen grpc port: %s", err)
		}
	}()

	if opts.ConfigFile != "" {
		go config.ReadConfigfileServe(opts.ConfigFile, opts.ConfigType, configCh)
	}

	var c *config.BgpConfigSet = nil
	for {
		select {
		case newConfig := <-configCh:
			var added, deleted, updated []config.Neighbor
			var addedPg, deletedPg, updatedPg []config.PeerGroup
			var updatePolicy bool

			if c == nil {
				c = newConfig
				if err := bgpServer.Start(&newConfig.Global); err != nil {
					log.Fatalf("failed to set global config: %s", err)
				}
				if newConfig.Zebra.Config.Enabled {
					if err := bgpServer.StartZebraClient(&newConfig.Zebra.Config); err != nil {
						log.Fatalf("failed to set zebra config: %s", err)
					}
				}
				if len(newConfig.Collector.Config.Url) > 0 {
					if err := bgpServer.StartCollector(&newConfig.Collector.Config); err != nil {
						log.Fatalf("failed to set collector config: %s", err)
					}
				}
				for i, _ := range newConfig.RpkiServers {
					if err := bgpServer.AddRpki(&newConfig.RpkiServers[i].Config); err != nil {
						log.Fatalf("failed to set rpki config: %s", err)
					}
				}
				for i, _ := range newConfig.BmpServers {
					if err := bgpServer.AddBmp(&newConfig.BmpServers[i].Config); err != nil {
						log.Fatalf("failed to set bmp config: %s", err)
					}
				}
				for i, _ := range newConfig.MrtDump {
					if len(newConfig.MrtDump[i].Config.FileName) == 0 {
						continue
					}
					if err := bgpServer.EnableMrt(&newConfig.MrtDump[i].Config); err != nil {
						log.Fatalf("failed to set mrt config: %s", err)
					}
				}
				p := config.ConfigSetToRoutingPolicy(newConfig)
				if err := bgpServer.UpdatePolicy(*p); err != nil {
					log.Fatalf("failed to set routing policy: %s", err)
				}

				added = newConfig.Neighbors
				addedPg = newConfig.PeerGroups
				if opts.GracefulRestart {
					for i, n := range added {
						if n.GracefulRestart.Config.Enabled {
							added[i].GracefulRestart.State.LocalRestarting = true
						}
					}
				}

			} else {
				addedPg, deletedPg, updatedPg = config.UpdatePeerGroupConfig(c, newConfig)
				added, deleted, updated = config.UpdateNeighborConfig(c, newConfig)
				updatePolicy = config.CheckPolicyDifference(config.ConfigSetToRoutingPolicy(c), config.ConfigSetToRoutingPolicy(newConfig))

				if updatePolicy {
					log.Info("Policy config is updated")
					p := config.ConfigSetToRoutingPolicy(newConfig)
					bgpServer.UpdatePolicy(*p)
				}
				// global policy update
				if !newConfig.Global.ApplyPolicy.Config.Equal(&c.Global.ApplyPolicy.Config) {
					a := newConfig.Global.ApplyPolicy.Config
					toDefaultTable := func(r config.DefaultPolicyType) table.RouteType {
						var def table.RouteType
						switch r {
						case config.DEFAULT_POLICY_TYPE_ACCEPT_ROUTE:
							def = table.ROUTE_TYPE_ACCEPT
						case config.DEFAULT_POLICY_TYPE_REJECT_ROUTE:
							def = table.ROUTE_TYPE_REJECT
						}
						return def
					}
					toPolicyDefinitions := func(r []string) []*config.PolicyDefinition {
						p := make([]*config.PolicyDefinition, 0, len(r))
						for _, n := range r {
							p = append(p, &config.PolicyDefinition{
								Name: n,
							})
						}
						return p
					}

					def := toDefaultTable(a.DefaultImportPolicy)
					ps := toPolicyDefinitions(a.ImportPolicyList)
					bgpServer.ReplacePolicyAssignment("", table.POLICY_DIRECTION_IMPORT, ps, def)

					def = toDefaultTable(a.DefaultExportPolicy)
					ps = toPolicyDefinitions(a.ExportPolicyList)
					bgpServer.ReplacePolicyAssignment("", table.POLICY_DIRECTION_EXPORT, ps, def)

					updatePolicy = true

				}
				c = newConfig
			}
			for i, pg := range addedPg {
				log.Infof("PeerGroup %s is added", pg.Config.PeerGroupName)
				if err := bgpServer.AddPeerGroup(&addedPg[i]); err != nil {
					log.Warn(err)
				}
			}
			for i, pg := range deletedPg {
				log.Infof("PeerGroup %s is deleted", pg.Config.PeerGroupName)
				if err := bgpServer.DeletePeerGroup(&deletedPg[i]); err != nil {
					log.Warn(err)
				}
			}
			for i, pg := range updatedPg {
				log.Infof("PeerGroup %s is updated", pg.Config.PeerGroupName)
				u, err := bgpServer.UpdatePeerGroup(&updatedPg[i])
				if err != nil {
					log.Warn(err)
				}
				updatePolicy = updatePolicy || u
			}
			for i, dn := range newConfig.DynamicNeighbors {
				log.Infof("Dynamic Neighbor %s is added to PeerGroup %s", dn.Config.Prefix, dn.Config.PeerGroup)
				if err := bgpServer.AddDynamicNeighbor(&newConfig.DynamicNeighbors[i]); err != nil {
					log.Warn(err)
				}
			}
			for i, p := range added {
				log.Infof("Peer %v is added", p.State.NeighborAddress)
				if err := bgpServer.AddNeighbor(&added[i]); err != nil {
					log.Warn(err)
				}
			}
			for i, p := range deleted {
				log.Infof("Peer %v is deleted", p.State.NeighborAddress)
				if err := bgpServer.DeleteNeighbor(&deleted[i]); err != nil {
					log.Warn(err)
				}
			}
			for i, p := range updated {
				log.Infof("Peer %v is updated", p.State.NeighborAddress)
				u, err := bgpServer.UpdateNeighbor(&updated[i])
				if err != nil {
					log.Warn(err)
				}
				updatePolicy = updatePolicy || u
			}

			if updatePolicy {
				bgpServer.SoftResetIn("", bgp.RouteFamily(0))
			}
		case <-sigCh:
			bgpServer.Shutdown()
		}
	}
}
