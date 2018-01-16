package k8s // import "go.universe.tf/metallb/internal/k8s"

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"go.universe.tf/metallb/internal/config"

	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

// Client watches a Kubernetes cluster and translates events into
// Controller method calls.
type Client struct {
	client *kubernetes.Clientset
	events record.EventRecorder

	queue workqueue.RateLimitingInterface

	svcIndexer   cache.Indexer
	svcInformer  cache.Controller
	epIndexer    cache.Indexer
	epInformer   cache.Controller
	cmIndexer    cache.Indexer
	cmInformer   cache.Controller
	nodeIndexer  cache.Indexer
	nodeInformer cache.Controller

	elector *leaderelection.LeaderElector

	syncFuncs []cache.InformerSynced

	serviceChanged            func(string, *v1.Service) error
	serviceOrEndpointsChanged func(string, *v1.Service, *v1.Endpoints) error
	configChanged             func(*config.Config) error
	nodeChanged               func(*v1.Node) error
	leaderChanged             func(bool)
	synced                    func()
}

type svcKey string
type cmKey string
type nodeKey string
type electionKey bool
type synced string

// New connects to masterAddr, using kubeconfig to authenticate.
//
// The client uses processName to identify itself to the cluster
// (e.g. when logging events).
func New(processName, masterAddr, kubeconfig string) (*Client, error) {
	k8sConfig, err := clientcmd.BuildConfigFromFlags(masterAddr, kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("building client config: %s", err)
	}
	clientset, err := kubernetes.NewForConfig(k8sConfig)
	if err != nil {
		return nil, fmt.Errorf("creating kubernetes client: %s", err)
	}

	broadcaster := record.NewBroadcaster()
	broadcaster.StartLogging(glog.Infof)
	broadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: v1core.New(clientset.CoreV1().RESTClient()).Events("")})
	recorder := broadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: processName})

	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	return &Client{
		client: clientset,
		events: recorder,
		queue:  queue,
	}, nil
}

// HandleService registers a handler for changes to Service objects.
func (c *Client) HandleService(handler func(string, *v1.Service) error) {
	// Note this also gets called internally by
	// HandleServiceAndEndpoints with a nil handler. Make sure the
	// code can handle that correctly. When called with a nil handler
	// it should just set up the indexer/informer and nothing else.
	if c.serviceChanged != nil {
		panic("HandleService called twice")
	}

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

	c.serviceChanged = handler
	c.syncFuncs = append(c.syncFuncs, c.svcInformer.HasSynced)
}

// HandleServiceAndEndpoints registers a handler for changes to Service objects
// and their associated Endpoints.
func (c *Client) HandleServiceAndEndpoints(handler func(string, *v1.Service, *v1.Endpoints) error) {
	c.HandleService(nil)

	epHandlers := cache.ResourceEventHandlerFuncs{
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
	epWatcher := cache.NewListWatchFromClient(c.client.CoreV1().RESTClient(), "endpoints", v1.NamespaceAll, fields.Everything())
	c.epIndexer, c.epInformer = cache.NewIndexerInformer(epWatcher, &v1.Endpoints{}, 0, epHandlers, cache.Indexers{})

	c.serviceOrEndpointsChanged = handler
	c.syncFuncs = append(c.syncFuncs, c.epInformer.HasSynced)
}

// HandleConfig registers a handler for changes to MetalLB's configuration.
func (c *Client) HandleConfig(handler func(*config.Config) error) {
	if c.configChanged != nil {
		panic("HandleConfig called twice")
	}

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
	cmWatcher := cache.NewListWatchFromClient(c.client.CoreV1().RESTClient(), "configmaps", "metallb-system", fields.OneTermEqualSelector("metadata.name", "config"))
	c.cmIndexer, c.cmInformer = cache.NewIndexerInformer(cmWatcher, &v1.ConfigMap{}, 0, cmHandlers, cache.Indexers{})

	c.configChanged = handler
	c.syncFuncs = append(c.syncFuncs, c.cmInformer.HasSynced)
}

// HandleNode registers a handler for changes to the given Node.
func (c *Client) HandleNode(nodeName string, handler func(*v1.Node) error) {
	if c.nodeChanged != nil {
		panic("HandleMyNode called twice")
	}

	handlers := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				c.queue.Add(nodeKey(key))
			}
		},
		UpdateFunc: func(old interface{}, new interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(new)
			if err == nil {
				c.queue.Add(nodeKey(key))
			}
		},
		DeleteFunc: func(obj interface{}) {
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err == nil {
				c.queue.Add(nodeKey(key))
			}
		},
	}
	watcher := cache.NewListWatchFromClient(c.client.CoreV1().RESTClient(), "nodes", v1.NamespaceAll, fields.OneTermEqualSelector("metadata.name", nodeName))
	c.nodeIndexer, c.nodeInformer = cache.NewIndexerInformer(watcher, &v1.Node{}, 0, handlers, cache.Indexers{})

	c.nodeChanged = handler
	c.syncFuncs = append(c.syncFuncs, c.nodeInformer.HasSynced)
}

// HandleSynced registers a handler for the "local cache synced" signal.
func (c *Client) HandleSynced(handler func()) {
	if c.synced != nil {
		panic("HandleSynced called twice")
	}
	c.synced = handler
}

// Run watches for events on the Kubernetes cluster, and dispatches
// calls to the Controller.
func (c *Client) Run(httpPort int) error {
	http.Handle("/metrics", promhttp.Handler())
	go func() {
		glog.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", httpPort), nil))
	}()

	stop := make(chan struct{})
	defer close(stop)

	if c.svcInformer != nil {
		go c.svcInformer.Run(stop)
	}
	if c.epInformer != nil {
		go c.epInformer.Run(stop)
	}
	if c.cmInformer != nil {
		go c.cmInformer.Run(stop)
	}
	if c.nodeInformer != nil {
		go c.nodeInformer.Run(stop)
	}
	if c.elector != nil {
		go func() {
			for {
				c.elector.Run()
				glog.Info("Restarting leader election loop in 10s")
				time.Sleep(10 * time.Second)
			}
		}()
	}

	if !cache.WaitForCacheSync(stop, c.syncFuncs...) {
		return errors.New("timed out waiting for cache sync")
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

// Update writes svc back into the Kubernetes cluster. If successful,
// the updated Service is returned. Note that changes to svc.Status
// are not propagated, for that you need to call UpdateStatus.
func (c *Client) Update(svc *v1.Service) (*v1.Service, error) {
	return c.client.CoreV1().Services(svc.Namespace).Update(svc)
}

// UpdateStatus writes the protected "status" field of svc back into
// the Kubernetes cluster.
func (c *Client) UpdateStatus(svc *v1.Service) error {
	_, err := c.client.CoreV1().Services(svc.Namespace).UpdateStatus(svc)
	return err
}

// Infof logs an informational event about svc to the Kubernetes cluster.
func (c *Client) Infof(svc *v1.Service, kind, msg string, args ...interface{}) {
	c.events.Eventf(svc, v1.EventTypeNormal, kind, msg, args...)
}

// Errorf logs an error event about svc to the Kubernetes cluster.
func (c *Client) Errorf(svc *v1.Service, kind, msg string, args ...interface{}) {
	c.events.Eventf(svc, v1.EventTypeWarning, kind, msg, args...)
}

// NodeHasHealthyEndpoint return true if this node has at least one healthy endpoint.
func NodeHasHealthyEndpoint(eps *v1.Endpoints, node string) bool {
	ready := map[string]bool{}
	for _, subset := range eps.Subsets {
		for _, ep := range subset.Addresses {
			if ep.NodeName == nil || *ep.NodeName != node {
				continue
			}
			if _, ok := ready[ep.IP]; !ok {
				// Only set true if nothing else has expressed an
				// opinion. This means that false will take precedence
				// if there's any unready ports for a given endpoint.
				ready[ep.IP] = true
			}
		}
		for _, ep := range subset.NotReadyAddresses {
			ready[ep.IP] = false
		}
	}

	for _, r := range ready {
		if r {
			// At least one fully healthy endpoint on this machine.
			return true
		}
	}
	return false
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
			if c.serviceChanged != nil {
				return c.serviceChanged(string(k), nil)
			}
			return c.serviceOrEndpointsChanged(string(k), nil, nil)
		}

		var eps *v1.Endpoints
		if c.epIndexer != nil {
			epsIntf, exists, err := c.epIndexer.GetByKey(string(k))
			if err != nil {
				return fmt.Errorf("get endpoints %q: %s", k, err)
			}
			if !exists {
				return c.serviceOrEndpointsChanged(string(k), nil, nil)
			}
			eps = epsIntf.(*v1.Endpoints)
		}

		if c.serviceChanged != nil {
			return c.serviceChanged(string(k), svc.(*v1.Service))
		}
		return c.serviceOrEndpointsChanged(string(k), svc.(*v1.Service), eps)

	case cmKey:
		cmi, exists, err := c.cmIndexer.GetByKey(string(k))
		if err != nil {
			return fmt.Errorf("get configmap %q: %s", k, err)
		}
		if !exists {
			configStale.Set(1)
			return c.configChanged(nil)
		}
		cm := cmi.(*v1.ConfigMap)
		cfg, err := config.Parse([]byte(cm.Data["config"]))
		if err != nil {
			configStale.Set(1)
			c.events.Eventf(cm, v1.EventTypeWarning, "InvalidConfig", "%s", err)
			return nil
		}

		if err := c.configChanged(cfg); err != nil {
			configStale.Set(1)
			c.events.Eventf(cm, v1.EventTypeWarning, "InvalidConfig", "%s", err)
			return nil
		}

		configLoaded.Set(1)
		configStale.Set(0)

		if c.svcIndexer != nil {
			glog.Infof("config changed, reconverging all services")
			for _, k := range c.svcIndexer.ListKeys() {
				c.queue.AddRateLimited(svcKey(k))
			}
		}

		return nil

	case nodeKey:
		n, exists, err := c.nodeIndexer.GetByKey(string(k))
		if err != nil {
			return fmt.Errorf("get node %q: %s", k, err)
		}
		if !exists {
			return fmt.Errorf("node %q doesn't exist", k)
		}
		node := n.(*v1.Node)
		return c.nodeChanged(node)

	case electionKey:
		if k {
			leader.Set(1)
			c.leaderChanged(true)
		} else {
			leader.Set(0)
			c.leaderChanged(false)
		}
		return nil

	case synced:
		if c.synced != nil {
			c.synced()
		}
		return nil

	default:
		panic(fmt.Errorf("unknown key type for %#v (%T)", key, key))
	}
}
