package k8s

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.universe.tf/metallb/internal/config"
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

type Controller interface {
	SetBalancer(name string, svc *v1.Service, eps *v1.Endpoints) error
	SetConfig(cfg *config.Config) error
	MarkSynced()
}

type Client struct {
	controller Controller

	client *kubernetes.Clientset
	events record.EventRecorder

	queue workqueue.RateLimitingInterface

	svcIndexer  cache.Indexer
	svcInformer cache.Controller
	epIndexer   cache.Indexer
	epInformer  cache.Controller
	cmIndexer   cache.Indexer
	cmInformer  cache.Controller
}

type svcKey string
type cmKey string
type synced string

func NewClient(name, masterAddr, kubeconfig string, ctrl Controller, watchEps bool) (*Client, error) {
	config, err := clientcmd.BuildConfigFromFlags(masterAddr, kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("building client config: %s", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("creating kubernetes client: %s", err)
	}

	broadcaster := record.NewBroadcaster()
	broadcaster.StartLogging(glog.Infof)
	broadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: v1core.New(clientset.CoreV1().RESTClient()).Events("")})
	recorder := broadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: name})

	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	svcHandlers := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				queue.Add(svcKey(key))
			}
		},
		UpdateFunc: func(old interface{}, new interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(new)
			if err == nil {
				queue.Add(svcKey(key))
			}
		},
		DeleteFunc: func(obj interface{}) {
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err == nil {
				queue.Add(svcKey(key))
			}
		},
	}
	svcWatcher := cache.NewListWatchFromClient(clientset.CoreV1().RESTClient(), "services", v1.NamespaceAll, fields.Everything())
	svcIndexer, svcInformer := cache.NewIndexerInformer(svcWatcher, &v1.Service{}, 0, svcHandlers, cache.Indexers{})

	cmHandlers := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				queue.Add(cmKey(key))
			}
		},
		UpdateFunc: func(old interface{}, new interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(new)
			if err == nil {
				queue.Add(cmKey(key))
			}
		},
		DeleteFunc: func(obj interface{}) {
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err == nil {
				queue.Add(cmKey(key))
			}
		},
	}
	cmWatcher := cache.NewListWatchFromClient(clientset.CoreV1().RESTClient(), "configmaps", "kube-system", fields.OneTermEqualSelector("metadata.name", "metallb-config"))
	cmIndexer, cmInformer := cache.NewIndexerInformer(cmWatcher, &v1.ConfigMap{}, 0, cmHandlers, cache.Indexers{})

	ret := &Client{
		controller:  ctrl,
		client:      clientset,
		events:      recorder,
		queue:       queue,
		svcIndexer:  svcIndexer,
		svcInformer: svcInformer,
		cmIndexer:   cmIndexer,
		cmInformer:  cmInformer,
	}

	if watchEps {
		epWatcher := cache.NewListWatchFromClient(clientset.CoreV1().RESTClient(), "endpoints", v1.NamespaceAll, fields.Everything())
		ret.epIndexer, ret.epInformer = cache.NewIndexerInformer(epWatcher, &v1.Endpoints{}, 0, svcHandlers, cache.Indexers{})
	}

	return ret, nil
}

func (c *Client) Run(httpPort int) error {
	http.Handle("/metrics", promhttp.Handler())
	go func() {
		glog.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", httpPort), nil))
	}()

	stop := make(chan struct{})
	defer close(stop)

	if c.epInformer != nil {
		go c.svcInformer.Run(stop)
		go c.cmInformer.Run(stop)
		go c.epInformer.Run(stop)
		if !cache.WaitForCacheSync(stop, c.svcInformer.HasSynced, c.cmInformer.HasSynced, c.epInformer.HasSynced) {
			return errors.New("timed out waiting for cache sync")
		}
	} else {
		go c.svcInformer.Run(stop)
		go c.cmInformer.Run(stop)
		if !cache.WaitForCacheSync(stop, c.svcInformer.HasSynced, c.cmInformer.HasSynced) {
			return errors.New("timed out waiting for cache sync")
		}
	}

	c.queue.Add(synced(""))

	for {
		key, quit := c.queue.Get()
		if quit {
			return nil
		}
		updates.Inc()
		if err := c.sync(key); err != nil {
			updateErrors.Inc()
			glog.Infof("error syncing %q: %s", key, err)
			c.queue.AddRateLimited(key)
		} else {
			c.queue.Forget(key)
		}
	}
}

func (c *Client) Update(svc *v1.Service) (*v1.Service, error) {
	return c.client.CoreV1().Services(svc.Namespace).Update(svc)
}

func (c *Client) UpdateStatus(svc *v1.Service) error {
	_, err := c.client.CoreV1().Services(svc.Namespace).UpdateStatus(svc)
	return err
}

func (c *Client) Infof(svc *v1.Service, kind, msg string, args ...interface{}) {
	c.events.Eventf(svc, v1.EventTypeNormal, kind, msg, args...)
}

func (c *Client) Errorf(svc *v1.Service, kind, msg string, args ...interface{}) {
	c.events.Eventf(svc, v1.EventTypeWarning, kind, msg, args...)
}

func (c *Client) sync(key interface{}) error {
	defer c.queue.Done(key)

	switch k := key.(type) {
	case svcKey:
		svc, exists, err := c.svcIndexer.GetByKey(string(k))
		if err != nil {
			return fmt.Errorf("get service %q: %s", k, err)
		}
		if !exists {
			return c.controller.SetBalancer(string(k), nil, nil)
		}

		var eps *v1.Endpoints
		if c.epIndexer != nil {
			epsIntf, exists, err := c.epIndexer.GetByKey(string(k))
			if err != nil {
				return fmt.Errorf("get endpoints %q: %s", k, err)
			}
			if !exists {
				return c.controller.SetBalancer(string(k), nil, nil)
			}
			eps = epsIntf.(*v1.Endpoints)
		}

		return c.controller.SetBalancer(string(k), svc.(*v1.Service), eps)

	case cmKey:
		cmi, exists, err := c.cmIndexer.GetByKey(string(k))
		if err != nil {
			return fmt.Errorf("get configmap %q: %s", k, err)
		}
		if !exists {
			configStale.Set(1)
			return c.controller.SetConfig(nil)
		}
		cm := cmi.(*v1.ConfigMap)
		cfg, err := config.Parse([]byte(cm.Data["config"]))
		if err != nil {
			configStale.Set(1)
			c.events.Eventf(cm, v1.EventTypeWarning, "InvalidConfig", "%s", err)
			return nil
		}

		if err := c.controller.SetConfig(cfg); err != nil {
			configStale.Set(1)
			c.events.Eventf(cm, v1.EventTypeWarning, "InvalidConfig", "%s", err)
			return nil
		}

		configLoaded.Set(1)
		configStale.Set(0)

		glog.Infof("config changed, reconverging all services")
		for _, k := range c.svcIndexer.ListKeys() {
			c.queue.AddRateLimited(svcKey(k))
		}

		return nil

	case synced:
		c.controller.MarkSynced()
		return nil

	default:
		panic(fmt.Errorf("unknown key type for %#v (%T)", key, key))
	}
}
