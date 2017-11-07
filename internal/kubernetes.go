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

package internal

import (
	"errors"
	"fmt"

	"github.com/golang/glog"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

const (
	AnnotationAssignedIP    = "metallb.go.universe.tf/assigned-ip"
	AnnotationAutoAllocated = "metallb.go.universe.tf/auto-allocated"
)

func Client(masterAddr, kubeconfig, componentName string) (*kubernetes.Clientset, record.EventRecorder, error) {
	config, err := clientcmd.BuildConfigFromFlags(masterAddr, kubeconfig)
	if err != nil {
		return nil, nil, fmt.Errorf("building client config: %s", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, fmt.Errorf("creating kubernetes client: %s", err)
	}

	broadcaster := record.NewBroadcaster()
	broadcaster.StartLogging(glog.Infof)
	broadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: v1core.New(clientset.CoreV1().RESTClient()).Events("")})
	recorder := broadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: componentName})

	return clientset, recorder, nil
}

type Balancer interface {
	UpdateBalancer(name string, svc *v1.Service, eps *v1.Endpoints) error
	DeleteBalancer(name string) error
}

type watcher struct {
	balancer Balancer

	svcIndexer cache.Indexer
	epIndexer  cache.Indexer

	svcInformer cache.Controller
	epInformer  cache.Controller

	queue workqueue.RateLimitingInterface
}

func WatchServices(client *kubernetes.Clientset, balancer Balancer) error {
	w, err := newWatcher(client, balancer)
	if err != nil {
		return err
	}
	return w.run()
}

func newWatcher(client *kubernetes.Clientset, balancer Balancer) (*watcher, error) {
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	handlers := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				queue.Add(key)
			}
		},
		UpdateFunc: func(old interface{}, new interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(new)
			if err == nil {
				queue.Add(key)
			}
		},
		DeleteFunc: func(obj interface{}) {
			// IndexerInformer uses a delta queue, therefore for deletes we have to use this
			// key function.
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err == nil {
				queue.Add(key)
			}
		},
	}

	svcWatcher := cache.NewListWatchFromClient(client.CoreV1().RESTClient(), "services", v1.NamespaceAll, fields.Everything())
	svcIndexer, svcInformer := cache.NewIndexerInformer(svcWatcher, &v1.Service{}, 0, handlers, cache.Indexers{})

	epWatcher := cache.NewListWatchFromClient(client.CoreV1().RESTClient(), "endpoints", v1.NamespaceAll, fields.Everything())
	epIndexer, epInformer := cache.NewIndexerInformer(epWatcher, &v1.Endpoints{}, 0, handlers, cache.Indexers{})

	return &watcher{balancer, svcIndexer, epIndexer, svcInformer, epInformer, queue}, nil
}

func (w *watcher) run() error {
	// Start informers, which will start priming the caches
	go w.svcInformer.Run(nil)
	go w.epInformer.Run(nil)

	// Wait for caches to converge
	if !cache.WaitForCacheSync(nil, w.svcInformer.HasSynced, w.epInformer.HasSynced) {
		return errors.New("timed out waiting for cache sync")
	}

	// Run control loop
	for {
		key, quit := w.queue.Get()
		if quit {
			return nil
		}
		err := w.sync(key.(string))
		if err != nil {
			glog.Infof("Error syncing service %q: %s", key, err)
			w.queue.AddRateLimited(key)
		} else {
			w.queue.Forget(key)
		}
	}
}

func (w *watcher) sync(key string) error {
	defer w.queue.Done(key)

	// Does the service exist? If not, we're done.
	svc, exists, err := w.svcIndexer.GetByKey(key)
	if err != nil {
		return fmt.Errorf("get service: %s", err)
	}
	if !exists {
		return w.balancer.DeleteBalancer(key)
	}

	ep, exists, err := w.epIndexer.GetByKey(key)
	if err != nil {
		return fmt.Errorf("get endpoints: %s", err)
	}
	if !exists {
		return w.balancer.DeleteBalancer(key)
	}

	return w.balancer.UpdateBalancer(key, svc.(*v1.Service), ep.(*v1.Endpoints))
}
