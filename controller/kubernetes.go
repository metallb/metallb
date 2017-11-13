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
	"errors"
	"fmt"

	"github.com/golang/glog"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type svcKey string
type cmKey string

func (c *controller) watch() error {
	c.queue = workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	svcHandlers := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				c.queue.Add(svcKey(key))
			}
		},
		UpdateFunc: func(old interface{}, new interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(new)
			if err == nil {
				c.queue.Add(svcKey(key))
			}
		},
		DeleteFunc: func(obj interface{}) {
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err == nil {
				c.queue.Add(svcKey(key))
			}
		},
	}
	svcWatcher := cache.NewListWatchFromClient(c.client.CoreV1().RESTClient(), "services", v1.NamespaceAll, fields.Everything())
	c.svcIndexer, c.svcInformer = cache.NewIndexerInformer(svcWatcher, &v1.Service{}, 0, svcHandlers, cache.Indexers{})

	cmHandlers := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				c.queue.Add(cmKey(key))
			}
		},
		UpdateFunc: func(old interface{}, new interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(new)
			if err == nil {
				c.queue.Add(cmKey(key))
			}
		},
		DeleteFunc: func(obj interface{}) {
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err == nil {
				c.queue.Add(cmKey(key))
			}
		},
	}
	cmWatcher := cache.NewListWatchFromClient(c.client.CoreV1().RESTClient(), "configmaps", "kube-system", fields.OneTermEqualSelector("metadata.name", "metallb-config"))
	c.cmIndexer, c.cmInformer = cache.NewIndexerInformer(cmWatcher, &v1.ConfigMap{}, 0, cmHandlers, cache.Indexers{})

	stop := make(chan struct{})
	defer close(stop)

	go c.svcInformer.Run(stop)
	go c.cmInformer.Run(stop)

	if !cache.WaitForCacheSync(stop, c.svcInformer.HasSynced, c.cmInformer.HasSynced) {
		return errors.New("timed out waiting for cache sync")
	}

	for {
		key, quit := c.queue.Get()
		if quit {
			return nil
		}

		if err := c.sync(key); err != nil {
			glog.Infof("error syncing %q: %s", key, err)
			c.queue.AddRateLimited(key)
		} else {
			c.queue.Forget(key)
		}
	}
}

func (c *controller) sync(key interface{}) error {
	defer c.queue.Done(key)

	switch k := key.(type) {
	case svcKey:
		svc, exists, err := c.svcIndexer.GetByKey(string(k))
		if err != nil {
			return fmt.Errorf("get service %q: %s", k, err)
		}
		if !exists {
			return c.DeleteBalancer(string(k))
		}
		return c.UpdateBalancer(string(k), svc.(*v1.Service))
	case cmKey:
		cm, exists, err := c.cmIndexer.GetByKey(string(k))
		if err != nil {
			return fmt.Errorf("get configmap %q: %s", k, err)
		}
		if !exists {
			return c.UpdateConfig(nil)
		}
		return c.UpdateConfig(cm.(*v1.ConfigMap))
	default:
		panic(fmt.Errorf("unknown key type for %#v (%T)", key, key))
	}
}
