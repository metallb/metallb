package k8s // import "go.universe.tf/metallb/internal/k8s"

import (
	"errors"
	"fmt"
	"net/http"

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
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

// A Controller takes action when watched Kubernetes objects change.
type Controller interface {
	// SetBalancer is called whenever a Service or its related
	// Endpoints change. When the Service is deleted, SetBalancer is
	// called with svc=nil and eps=nil.
	SetBalancer(name string, svc *v1.Service, eps *v1.Endpoints) error
	// SetConfig is called whenever the MetalLB configuration for the
	// cluster changes. If the config is deleted from the cluster,
	// SetConfig is called with cfg=nil.
	SetConfig(cfg *config.Config) error
	// MarkSynced is called when SetBalancer has been called at least
	// once for every Service in the cluster, and SetConfig has been
	// called.
	MarkSynced()
}

// Client watches a Kubernetes cluster and translates events into
// Controller method calls.
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

// NewClient connects to masterAddr, using kubeconfig to authenticate,
// and calls methods on ctrls as the cluster state changes.
//
// The client uses the given name to identify itself to the cluster
// (e.g. when logging events). watchEps defines whether ctrl cares
// about the endpoints associated with a service. If watchEps is
// false, the controller will not watch endpoint objects, and the
// endpoints in SetBalancer calls will always be nil.
func NewClient(name, masterAddr, kubeconfig string, watchEps bool, ctrls ...Controller) (*Client, error) {
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
	cmWatcher := cache.NewListWatchFromClient(clientset.CoreV1().RESTClient(), "configmaps", "metallb-system", fields.OneTermEqualSelector("metadata.name", "config"))
	cmIndexer, cmInformer := cache.NewIndexerInformer(cmWatcher, &v1.ConfigMap{}, 0, cmHandlers, cache.Indexers{})

	ret := &Client{
		controller:  multiController(ctrls),
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

// Run watches for events on the Kubernetes cluster, and dispatches
// calls to the Controller.
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

type multiController []Controller

func (m multiController) SetBalancer(name string, svc *v1.Service, eps *v1.Endpoints) error {
	var errs []error
	for _, ctrl := range m {
		errs = append(errs, ctrl.SetBalancer(name, svc, eps))
	}
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func (m multiController) SetConfig(cfg *config.Config) error {
	var errs []error
	for _, ctrl := range m {
		errs = append(errs, ctrl.SetConfig(cfg))
	}
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func (m multiController) MarkSynced() {
	for _, ctrl := range m {
		ctrl.MarkSynced()
	}
}
