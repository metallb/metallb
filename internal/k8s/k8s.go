// SPDX-License-Identifier:Apache-2.0

package k8s // import "go.universe.tf/metallb/internal/k8s"

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	metallbv1beta2 "go.universe.tf/metallb/api/v1beta2"

	"go.universe.tf/metallb/internal/config"
	"go.universe.tf/metallb/internal/k8s/controllers"
	"go.universe.tf/metallb/internal/k8s/epslices"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	"errors"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	frrv1beta1 "github.com/metallb/frr-k8s/api/v1beta1"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	discovery "k8s.io/api/discovery/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"

	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	caName          = "cert"
	caOrganization  = "metallb"
	MLSecretKeyName = "secretkey"
)

var (
	scheme                      = runtime.NewScheme()
	setupLog                    = ctrl.Log.WithName("setup")
	validatingWebhookName       = "metallb-webhook-configuration"
	bgppeerConvertingWebhookCRD = "bgppeers.metallb.io"
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(metallbv1beta1.AddToScheme(scheme))
	utilruntime.Must(metallbv1beta2.AddToScheme(scheme))

	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(appsv1.AddToScheme(scheme))
	utilruntime.Must(policyv1beta1.AddToScheme(scheme))
	utilruntime.Must(rbacv1.AddToScheme(scheme))
	utilruntime.Must(apiext.AddToScheme(scheme))
	utilruntime.Must(discovery.AddToScheme(scheme))
	utilruntime.Must(frrv1beta1.AddToScheme(scheme))

	// +kubebuilder:scaffold:scheme
}

// Client watches a Kubernetes cluster and translates events into
// Controller method calls.
type Client struct {
	logger log.Logger

	client           *kubernetes.Clientset
	events           record.EventRecorder
	mgr              manager.Manager
	validateConfig   config.Validate
	ForceSync        func()
	BGPEventCallback func(interface{})
}

// Config specifies the configuration of the Kubernetes
// client/watcher.
type Config struct {
	ProcessName         string
	NodeName            string
	PodName             string
	MetricsHost         string
	MetricsPort         int
	EnablePprof         bool
	ReadEndpoints       bool
	Logger              log.Logger
	Namespace           string
	ValidateConfig      config.Validate
	EnableWebhook       bool
	WebHookMinVersion   uint16
	WebHookCipherSuites []uint16
	DisableCertRotation bool
	WebhookSecretName   string
	CertDir             string
	CertServiceName     string
	LoadBalancerClass   string
	WebhookWithHTTP2    bool
	WithFRRK8s          bool
	FRRK8sNamespace     string
	Listener
	Layer2StatusChan    <-chan event.GenericEvent
	Layer2StatusFetcher controllers.L2StatusFetcher
	BGPStatusChan       <-chan event.GenericEvent
	BGPPeersFetcher     controllers.PeersForService
	PoolStatusChan      <-chan event.GenericEvent
	PoolCountersFetcher controllers.PoolCountersFetcher
}

// New connects to masterAddr, using kubeconfig to authenticate.
//
// The client uses processName to identify itself to the cluster
// (e.g. when logging events).
func New(cfg *Config) (*Client, error) {
	namespaceSelector := cache.ByObject{
		Field: fields.ParseSelectorOrDie(fmt.Sprintf("metadata.namespace=%s", cfg.Namespace)),
	}

	objectsPerNamespace := map[client.Object]cache.ByObject{
		&metallbv1beta1.BFDProfile{}:       namespaceSelector,
		&metallbv1beta1.BGPAdvertisement{}: namespaceSelector,
		&metallbv1beta1.BGPPeer{}:          namespaceSelector,
		&metallbv1beta1.IPAddressPool{}:    namespaceSelector,
		&metallbv1beta1.L2Advertisement{}:  namespaceSelector,
		&metallbv1beta2.BGPPeer{}:          namespaceSelector,
		&metallbv1beta1.Community{}:        namespaceSelector,
		&metallbv1beta1.ServiceBGPStatus{}: namespaceSelector,
		&corev1.Secret{}:                   namespaceSelector,
		&corev1.ConfigMap{}:                namespaceSelector,
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:         scheme,
		LeaderElection: false,
		Cache: cache.Options{
			ByObject: objectsPerNamespace,
		},
		WebhookServer: webhookServer(cfg),
		Metrics: metricsserver.Options{
			BindAddress: "0", // Disable metrics endpoint of controller manager
		},
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}
	clientset, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		return nil, fmt.Errorf("creating Kubernetes client: %s", err)
	}

	recorder := mgr.GetEventRecorderFor(cfg.ProcessName)

	reloadChan := make(chan event.GenericEvent)
	reload := func() {
		reloadChan <- controllers.NewReloadEvent()
	}

	c := &Client{
		logger:         cfg.Logger,
		client:         clientset,
		events:         recorder,
		mgr:            mgr,
		validateConfig: cfg.ValidateConfig,
		ForceSync:      reload,
	}

	if cfg.ConfigChanged != nil {
		if err = (&controllers.ConfigReconciler{
			Client:         mgr.GetClient(),
			Logger:         cfg.Logger,
			Scheme:         mgr.GetScheme(),
			Namespace:      cfg.Namespace,
			ValidateConfig: cfg.ValidateConfig,
			Handler:        cfg.ConfigHandler,
			ForceReload:    reload,
		}).SetupWithManager(mgr); err != nil {
			level.Error(c.logger).Log("error", err, "unable to create controller", "config")
			return nil, errors.Join(err, errors.New("unable to create controller for config"))
		}
	}

	if cfg.PoolChanged != nil {
		if err = (&controllers.PoolReconciler{
			Client:         mgr.GetClient(),
			Logger:         cfg.Logger,
			Scheme:         mgr.GetScheme(),
			Namespace:      cfg.Namespace,
			ValidateConfig: cfg.ValidateConfig,
			Handler:        cfg.PoolHandler,
			ForceReload:    reload,
		}).SetupWithManager(mgr); err != nil {
			level.Error(c.logger).Log("error", err, "unable to create controller", "config")
			return nil, errors.Join(err, errors.New("failed to create config reconciler"))
		}

		if err = (&controllers.PoolStatusReconciler{
			Client:          mgr.GetClient(),
			Logger:          cfg.Logger,
			CountersFetcher: cfg.PoolCountersFetcher,
			ReconcileChan:   cfg.PoolStatusChan,
		}).SetupWithManager(mgr); err != nil {
			level.Error(c.logger).Log("error", err, "unable to create controller", "config")
			return nil, errors.Join(err, errors.New("failed to create pool status reconciler"))
		}
	}

	if cfg.NodeChanged != nil {
		if err = (&controllers.NodeReconciler{
			Client:      mgr.GetClient(),
			Logger:      cfg.Logger,
			Scheme:      mgr.GetScheme(),
			Handler:     cfg.NodeHandler,
			NodeName:    cfg.NodeName,
			ForceReload: reload,
		}).SetupWithManager(mgr); err != nil {
			level.Error(c.logger).Log("error", err, "unable to create controller", "node")
			return nil, errors.Join(err, errors.New("failed to create node reconciler"))
		}
	}

	if cfg.WithFRRK8s {
		frrk8sController := controllers.FRRK8sReconciler{
			Client:          mgr.GetClient(),
			Logger:          cfg.Logger,
			Scheme:          mgr.GetScheme(),
			FRRK8sNamespace: cfg.FRRK8sNamespace,
			NodeName:        cfg.NodeName,
		}
		if err := frrk8sController.SetupWithManager(mgr); err != nil {
			level.Error(c.logger).Log("error", err, "unable to create controller", "frrk8s")
			return nil, errors.Join(err, errors.New("failed to create frrk8s reconciler"))
		}
		c.BGPEventCallback = frrk8sController.UpdateConfig
	}

	if cfg.ReadEndpoints {
		// Set a field indexer so we can retrieve all the endpoints for a given service.
		if err := mgr.GetFieldIndexer().IndexField(context.Background(), &discovery.EndpointSlice{}, epslices.SlicesServiceIndexName, func(rawObj client.Object) []string {
			epSlice, ok := rawObj.(*discovery.EndpointSlice)
			if epSlice == nil {
				level.Error(c.logger).Log("controller", "fieldindexer", "error", "received nil epslice")
				return nil
			}
			if !ok {
				level.Error(c.logger).Log("controller", "fieldindexer", "error", "received object that is not epslice", "object", rawObj.GetObjectKind().GroupVersionKind().Kind)
				return nil
			}
			serviceKey, err := epslices.ServiceKeyForSlice(epSlice)
			if err != nil {
				level.Error(c.logger).Log("controller", "ServiceReconciler", "error", "failed to get service from epslices", "epslice", epSlice.Name, "error", err)
			}
			return []string{serviceKey.String()}
		}); err != nil {
			return nil, err
		}
	}

	if cfg.ServiceChanged != nil {
		if err = (&controllers.ServiceReconciler{
			Client:            mgr.GetClient(),
			Logger:            cfg.Logger,
			Scheme:            mgr.GetScheme(),
			Handler:           cfg.ServiceHandler,
			Endpoints:         cfg.ReadEndpoints,
			Reload:            reloadChan,
			LoadBalancerClass: cfg.LoadBalancerClass,
		}).SetupWithManager(mgr); err != nil {
			level.Error(c.logger).Log("error", err, "unable to create controller", "service")
			return nil, errors.Join(err, errors.New("failed to create service reconciler"))
		}
	}

	// metallb controller doesn't need this reconciler
	if cfg.Layer2StatusChan != nil {
		selfPod, err := clientset.CoreV1().Pods(cfg.Namespace).Get(context.TODO(), cfg.PodName, metav1.GetOptions{})
		if err != nil {
			level.Error(c.logger).Log("unable to get speaker pod itself", err)
			return nil, err
		}
		if err = (&controllers.Layer2StatusReconciler{
			Client:        mgr.GetClient(),
			Logger:        cfg.Logger,
			NodeName:      cfg.NodeName,
			Namespace:     cfg.Namespace,
			SpeakerPod:    selfPod.DeepCopy(),
			ReconcileChan: cfg.Layer2StatusChan,
			StatusFetcher: cfg.Layer2StatusFetcher,
		}).SetupWithManager(mgr); err != nil {
			level.Error(c.logger).Log("error", err, "unable to create controller", "layer2Status")
		}
	}

	if cfg.BGPStatusChan != nil {
		selfPod, err := clientset.CoreV1().Pods(cfg.Namespace).Get(context.TODO(), cfg.PodName, metav1.GetOptions{})
		if err != nil {
			level.Error(c.logger).Log("unable to get speaker pod itself", err)
			return nil, err
		}
		if err = (&controllers.ServiceBGPStatusReconciler{
			Client:        mgr.GetClient(),
			Logger:        cfg.Logger,
			NodeName:      cfg.NodeName,
			Namespace:     cfg.Namespace,
			SpeakerPod:    selfPod.DeepCopy(),
			ReconcileChan: cfg.BGPStatusChan,
			PeersFetcher:  cfg.BGPPeersFetcher,
		}).SetupWithManager(mgr); err != nil {
			level.Error(c.logger).Log("error", err, "unable to create controller", "layer2Status")
		}
	}

	startListeners := make(chan struct{})
	go func(l log.Logger) {
		// We start the webhooks and the metric at the same time so the readiness probe will
		// return success only when we are able to serve webhook requests.
		<-startListeners
		if cfg.EnableWebhook {
			err := enableWebhook(c.mgr, cfg.ValidateConfig, cfg.Namespace, cfg.Logger)
			if err != nil {
				level.Error(l).Log("error", err, "unable to create", "webhooks")
			}
		}

		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())

		if cfg.EnablePprof {
			mux.HandleFunc("/debug/pprof/", pprof.Index)
			mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
			mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
			mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
			mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
		}

		server := &http.Server{
			Addr:              net.JoinHostPort(cfg.MetricsHost, fmt.Sprint(cfg.MetricsPort)),
			Handler:           mux,
			ReadHeaderTimeout: 3 * time.Second,
		}

		err := server.ListenAndServe()
		if err != nil {
			level.Error(l).Log("op", "listenAndServe", "err", err, "msg", "cannot listen and serve", "host", cfg.MetricsHost, "port", cfg.MetricsPort)
		}
	}(c.logger)

	// The cert rotator will notify when we can start the webhook
	// and the metric endpoint
	if cfg.EnableWebhook && !cfg.DisableCertRotation {
		err = enableCertRotation(startListeners, cfg, mgr)
		if err != nil {
			return nil, errors.Join(err, errors.New("failed to enable cert rotation"))
		}
	} else {
		// otherwise we can go on and start them
		close(startListeners)
	}

	return c, nil
}

// CreateMlSecret create the memberlist secret.
func (c *Client) CreateMlSecret(namespace, controllerDeploymentName, secretName string) error {
	// Use List instead of Get to differentiate between API errors and non existing secret.
	// Matching error text is prone to future breakage.
	l, err := c.client.CoreV1().Secrets(namespace).List(context.TODO(), metav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("metadata.name", secretName).String(),
	})
	if err != nil {
		return err
	}
	if len(l.Items) > 0 {
		level.Debug(c.logger).Log("op", "CreateMlSecret", "msg", "secret already exists, nothing to do")
		return nil
	}

	// Get the controller Deployment info to set secret ownerReference.
	d, err := c.client.AppsV1().Deployments(namespace).Get(context.TODO(), controllerDeploymentName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	// Create the secret key (128 bits).
	secret := make([]byte, 16)
	_, err = rand.Read(secret)
	if err != nil {
		return err
	}
	// base64 encode the secret key as it'll be passed a env variable.
	secretB64 := make([]byte, base64.RawStdEncoding.EncodedLen(len(secret)))
	base64.RawStdEncoding.Encode(secretB64, secret)

	// Create the K8S Secret object.
	_, err = c.client.CoreV1().Secrets(namespace).Create(
		context.TODO(),
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: secretName,
				OwnerReferences: []metav1.OwnerReference{{
					// d.APIVersion is empty.
					APIVersion: "apps/v1",
					// d.Kind is empty.
					Kind: "Deployment",
					Name: d.Name,
					UID:  d.UID,
				}},
			},
			Data: map[string][]byte{MLSecretKeyName: secretB64},
		},
		metav1.CreateOptions{})
	if err == nil {
		level.Info(c.logger).Log("op", "CreateMlSecret", "msg", "secret successfully created")
	}
	return err
}

// PodIPs returns the IPs of all the pods matched by the labels string.
func (c *Client) PodIPs(namespace, labels string) ([]string, error) {
	pl, err := c.client.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: labels})
	if err != nil {
		return nil, err
	}
	iplist := []string{}
	for _, pod := range pl.Items {
		iplist = append(iplist, pod.Status.PodIP)
	}
	return iplist, nil
}

// Run watches for events on the Kubernetes cluster, and dispatches
// calls to the Controller.
func (c *Client) Run(stopCh <-chan struct{}) error {
	ctx := ctrl.SetupSignalHandler()

	level.Info(c.logger).Log("op", "Run", "msg", "Starting Manager")
	if err := c.mgr.Start(ctx); err != nil {
		return err
	}

	return nil
}

// UpdateStatus writes the protected "status" field of svc back into
// the Kubernetes cluster.
func (c *Client) UpdateStatus(svc *corev1.Service) error {
	_, err := c.client.CoreV1().Services(svc.Namespace).UpdateStatus(context.TODO(), svc, metav1.UpdateOptions{})
	return err
}

// Infof logs an informational event about svc to the Kubernetes cluster.
func (c *Client) Infof(svc *corev1.Service, kind, msg string, args ...interface{}) {
	c.events.Eventf(svc, corev1.EventTypeNormal, kind, msg, args...)
}

// Errorf logs an error event about svc to the Kubernetes cluster.
func (c *Client) Errorf(svc *corev1.Service, kind, msg string, args ...interface{}) {
	c.events.Eventf(svc, corev1.EventTypeWarning, kind, msg, args...)
}

func webhookServer(cfg *Config) webhook.Server {
	disableHTTP2 := func(c *tls.Config) {
		if cfg.WebhookWithHTTP2 {
			return
		}
		c.NextProtos = []string{"http/1.1"}
	}

	tlsSecurity := func(tlsConfig *tls.Config) {
		tlsConfig.MinVersion = cfg.WebHookMinVersion
		tlsConfig.CipherSuites = cfg.WebHookCipherSuites
	}

	webhookServerOptions := webhook.Options{
		TLSOpts: []func(config *tls.Config){disableHTTP2, tlsSecurity},
		Port:    9443,
	}

	res := webhook.NewServer(webhookServerOptions)
	return res
}
