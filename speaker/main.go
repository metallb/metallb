// Copyright 2017 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"go.universe.tf/metallb/pkg/k8s"
	"go.universe.tf/metallb/pkg/logging"
	"go.universe.tf/metallb/pkg/speaker"
	"go.universe.tf/metallb/pkg/speakerlist"
	"go.universe.tf/metallb/pkg/version"
)

func main() {
	logger, err := logging.Init()
	if err != nil {
		fmt.Printf("failed to initialize logging: %s\n", err)
		os.Exit(1)
	}

	var (
		config      = flag.String("config", "config", "Kubernetes ConfigMap containing MetalLB's configuration")
		configNS    = flag.String("config-ns", "", "config file namespace (only needed when running outside of k8s)")
		kubeconfig  = flag.String("kubeconfig", "", "absolute path to the kubeconfig file (only needed when running outside of k8s)")
		host        = flag.String("host", os.Getenv("METALLB_HOST"), "HTTP host address")
		mlBindAddr  = flag.String("ml-bindaddr", os.Getenv("METALLB_ML_BIND_ADDR"), "Bind addr for MemberList (fast dead node detection)")
		mlBindPort  = flag.String("ml-bindport", os.Getenv("METALLB_ML_BIND_PORT"), "Bind port for MemberList (fast dead node detection)")
		mlLabels    = flag.String("ml-labels", os.Getenv("METALLB_ML_LABELS"), "Labels to match the speakers (for MemberList / fast dead node detection)")
		mlNamespace = flag.String("ml-namespace", os.Getenv("METALLB_ML_NAMESPACE"), "Namespace of the speakers (for MemberList / fast dead node detection)")
		mlSecret    = flag.String("ml-secret-key", os.Getenv("METALLB_ML_SECRET_KEY"), "Secret key for MemberList (fast dead node detection)")
		myNode      = flag.String("node-name", os.Getenv("METALLB_NODE_NAME"), "name of this Kubernetes node (spec.nodeName)")
		port        = flag.Int("port", 7472, "HTTP listening port")
	)
	flag.Parse()

	logger.Log("version", version.Version(), "commit", version.CommitHash(), "branch", version.Branch(), "msg", "MetalLB speaker starting "+version.String())

	if *myNode == "" {
		logger.Log("op", "startup", "error", "must specify --node-name or METALLB_NODE_NAME", "msg", "missing configuration")
		os.Exit(1)
	}

	stopCh := make(chan struct{})
	go func() {
		c1 := make(chan os.Signal)
		signal.Notify(c1, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)
		<-c1
		logger.Log("op", "shutdown", "msg", "starting shutdown")
		signal.Stop(c1)
		close(stopCh)
	}()
	defer logger.Log("op", "shutdown", "msg", "done")

	sList, err := speakerlist.New(logger, *myNode, *mlBindAddr, *mlBindPort, *mlSecret, *mlNamespace, *mlLabels, stopCh)
	if err != nil {
		os.Exit(1)
	}

	// Setup all clients and speakers, config decides what is being done runtime.
	ctrl, err := speaker.NewController(speaker.ControllerConfig{
		MyNode: *myNode,
		Logger: logger,
		SList:  sList,
	})
	if err != nil {
		logger.Log("op", "startup", "error", err, "msg", "failed to create MetalLB controller")
		os.Exit(1)
	}

	client, err := k8s.New(&k8s.Config{
		ProcessName:   "metallb-speaker",
		ConfigMapName: *config,
		ConfigMapNS:   *configNS,
		NodeName:      *myNode,
		Logger:        logger,
		Kubeconfig:    *kubeconfig,

		MetricsHost:   *host,
		MetricsPort:   *port,
		ReadEndpoints: true,

		ServiceChanged: ctrl.SetBalancer,
		ConfigChanged:  ctrl.SetConfig,
		NodeChanged:    ctrl.SetNode,
	})
	if err != nil {
		logger.Log("op", "startup", "error", err, "msg", "failed to create k8s client")
		os.Exit(1)
	}
	ctrl.Client = client

	sList.Start(client)
	defer sList.Stop()

	if err := client.Run(stopCh); err != nil {
		logger.Log("op", "startup", "error", err, "msg", "failed to run k8s client")
	}
}
