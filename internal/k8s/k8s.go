package k8s // import "go.universe.tf/metallb/internal/k8s"

import (
	"errors"
	"fmt"
	"io/ioutil"
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
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

// Client watches a Kubernetes cluster and translates events into
// Controller method calls.
type Client struct {
	config Config

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
}

// Config specifies the configuration of the Kubernetes
// client/watcher.
type Config struct {
	ProcessName   string
	ConfigMapName string
	NodeName      string

	MetricsPort   int
	ReadEndpoints bool

	ServiceChanged func(string, *v1.Service, *v1.Endpoints) error
	ConfigChanged  func(*config.Config) error
	NodeChanged    func(*v1.Node) error
	LeaderChanged  func(bool)
	Synced         func()
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
func New(cfg *Config) (*Client, error) {
	k8sConfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("building client config: %s", err)
	}
	clientset, err := kubernetes.NewForConfig(k8sConfig)
	if err != nil {
		return nil, fmt.Errorf("creating Kubernetes client: %s", err)
	}

	bs, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		return nil, fmt.Errorf("getting namespace from pod service account data: %s", err)
	}
	namespace := string(bs)

	broadcaster := record.NewBroadcaster()
	broadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: v1core.New(clientset.CoreV1().RESTClient()).Events("")})
	recorder := broadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: cfg.ProcessName})

	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	c := &Client{
		config: *cfg,
		client: clientset,
		events: recorder,
		queue:  queue,
	}

	if cfg.ServiceChanged != nil {
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

		c.syncFuncs = append(c.syncFuncs, c.svcInformer.HasSynced)

		if cfg.ReadEndpoints {
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

			c.syncFuncs = append(c.syncFuncs, c.epInformer.HasSynced)
		}
	}

	if cfg.ConfigChanged != nil {
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
		cmWatcher := cache.NewListWatchFromClient(c.client.CoreV1().RESTClient(), "configmaps", namespace, fields.OneTermEqualSelector("metadata.name", cfg.ConfigMapName))
		c.cmIndexer, c.cmInformer = cache.NewIndexerInformer(cmWatcher, &v1.ConfigMap{}, 0, cmHandlers, cache.Indexers{})

		c.syncFuncs = append(c.syncFuncs, c.cmInformer.HasSynced)
	}

	if cfg.NodeChanged != nil {
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
		watcher := cache.NewListWatchFromClient(c.client.CoreV1().RESTClient(), "nodes", v1.NamespaceAll, fields.OneTermEqualSelector("metadata.name", cfg.NodeName))
		c.nodeIndexer, c.nodeInformer = cache.NewIndexerInformer(watcher, &v1.Node{}, 0, handlers, cache.Indexers{})

		c.syncFuncs = append(c.syncFuncs, c.nodeInformer.HasSynced)
	}

	if cfg.LeaderChanged != nil {
		conf := resourcelock.ResourceLockConfig{Identity: cfg.NodeName, EventRecorder: c.events}
		lock, err := resourcelock.New(resourcelock.EndpointsResourceLock, namespace, "metallb-speaker", c.client.CoreV1(), conf)
		if err != nil {
			return nil, fmt.Errorf("creating resource lock for leader election: %s", err)
		}

		leader.Set(-1)

		lec := leaderelection.LeaderElectionConfig{
			Lock: lock,
			// Time before the lock expires and other replicas can try to
			// become leader.
			LeaseDuration: 10 * time.Second,
			// How long we should keep trying to hold the lock before
			// giving up and deciding we've lost it.
			RenewDeadline: 9 * time.Second,
			// Time to wait between refreshing the lock when we are
			// leader.
			RetryPeriod: 5 * time.Second,
			Callbacks: leaderelection.LeaderCallbacks{
				OnStartedLeading: func(stop <-chan struct{}) {
					c.queue.Add(electionKey(true))
				},
				OnStoppedLeading: func() {
					c.queue.Add(electionKey(false))
				},
			},
		}

		elector, err := leaderelection.NewLeaderElector(lec)
		if err != nil {
			return nil, fmt.Errorf("creating leader elector: %s", err)
		}
		c.elector = elector
	}
	return c, nil
}

// Run watches for events on the Kubernetes cluster, and dispatches
// calls to the Controller.
func (c *Client) Run() error {
	http.Handle("/metrics", promhttp.Handler())
	go func() {
		http.ListenAndServe(fmt.Sprintf(":%d", c.config.MetricsPort), nil)
	}()

	if c.svcInformer != nil {
		go c.svcInformer.Run(nil)
	}
	if c.epInformer != nil {
		go c.epInformer.Run(nil)
	}
	if c.cmInformer != nil {
		go c.cmInformer.Run(nil)
	}
	if c.nodeInformer != nil {
		go c.nodeInformer.Run(nil)
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

	if !cache.WaitForCacheSync(nil, c.syncFuncs...) {
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
			return c.config.ServiceChanged(string(k), nil, nil)
		}

		var eps *v1.Endpoints
		if c.epIndexer != nil {
			epsIntf, exists, err := c.epIndexer.GetByKey(string(k))
			if err != nil {
				return fmt.Errorf("get endpoints %q: %s", k, err)
			}
			if !exists {
				return c.config.ServiceChanged(string(k), nil, nil)
			}
			eps = epsIntf.(*v1.Endpoints)
		}

		return c.config.ServiceChanged(string(k), svc.(*v1.Service), eps)

	case cmKey:
		cmi, exists, err := c.cmIndexer.GetByKey(string(k))
		if err != nil {
			return fmt.Errorf("get configmap %q: %s", k, err)
		}
		if !exists {
			configStale.Set(1)
			return c.config.ConfigChanged(nil)
		}
		cm := cmi.(*v1.ConfigMap)
		cfg, err := config.Parse([]byte(cm.Data["config"]))
		if err != nil {
			configStale.Set(1)
			c.events.Eventf(cm, v1.EventTypeWarning, "InvalidConfig", "%s", err)
			return nil
		}

		if err := c.config.ConfigChanged(cfg); err != nil {
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
		return c.config.NodeChanged(node)

	case electionKey:
		if k {
			leader.Set(1)
			c.config.LeaderChanged(true)
		} else {
			leader.Set(0)
			c.config.LeaderChanged(false)
		}
		return nil

	case synced:
		if c.config.Synced != nil {
			c.config.Synced()
		}
		return nil

	default:
		panic(fmt.Errorf("unknown key type for %#v (%T)", key, key))
	}
}
