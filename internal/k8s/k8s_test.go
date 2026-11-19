// SPDX-License-Identifier:Apache-2.0

package k8s

import (
	"context"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

func TestDefaultHealthProbePort(t *testing.T) {
	if DefaultHealthProbePort <= 0 {
		t.Errorf("DefaultHealthProbePort must be positive, got %d", DefaultHealthProbePort)
	}
}

func TestMetricsBindAddress(t *testing.T) {
	tests := []struct {
		name string
		port int
		want string
	}{
		{name: "disabled", port: 0, want: "0"},
		{name: "enabled", port: 9120, want: "0.0.0.0:9120"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := metricsBindAddress(test.port); got != test.want {
				t.Fatalf("metricsBindAddress(%d) = %q, want %q", test.port, got, test.want)
			}
		})
	}
}

// TestCacheConfigurationStateScope needs a real API server, as field selectors
// are evaluated server-side.
func TestCacheConfigurationStateScope(t *testing.T) {
	const namespace = "metallb-system"

	testEnv := &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
		Scheme:                scheme,
	}
	restConfig, err := testEnv.Start()
	if err != nil {
		t.Fatalf("failed to start test environment: %v", err)
	}
	defer func() {
		if err := testEnv.Stop(); err != nil {
			t.Errorf("failed to stop test environment: %v", err)
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cl, err := client.New(restConfig, client.Options{Scheme: scheme})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	configState := func(namespace, name string) *metallbv1beta1.ConfigurationState {
		return &metallbv1beta1.ConfigurationState{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}}
	}
	for _, ns := range []string{namespace, "other"} {
		if err := cl.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}); err != nil {
			t.Fatalf("failed to create namespace %s: %v", ns, err)
		}
	}
	for _, obj := range []*metallbv1beta1.ConfigurationState{
		configState(namespace, "speaker-a"),
		configState(namespace, "speaker-b"),
		configState(namespace, "controller"),
		configState("other", "speaker-a"),
	} {
		if err := cl.Create(ctx, obj); err != nil {
			t.Fatalf("failed to create ConfigurationState %s/%s: %v", obj.Namespace, obj.Name, err)
		}
	}

	c, err := cache.New(restConfig, cache.Options{Scheme: scheme, ByObject: cacheByObject(namespace, "speaker-a")})
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	informer, err := c.GetInformer(ctx, &metallbv1beta1.ConfigurationState{})
	if err != nil {
		t.Fatalf("failed to get informer: %v", err)
	}
	var lock sync.Mutex
	var events []string
	record := func(event string, obj any) {
		if tombstone, ok := obj.(toolscache.DeletedFinalStateUnknown); ok {
			obj = tombstone.Obj
		}
		o := obj.(client.Object)
		lock.Lock()
		defer lock.Unlock()
		events = append(events, event+" "+o.GetNamespace()+"/"+o.GetName())
	}
	_, err = informer.AddEventHandler(toolscache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj any) { record("ADD", obj) },
		UpdateFunc: func(_, obj any) { record("UPDATE", obj) },
		DeleteFunc: func(obj any) { record("DELETE", obj) },
	})
	if err != nil {
		t.Fatalf("failed to add event handler: %v", err)
	}
	cacheStopped := make(chan error, 1)
	go func() { cacheStopped <- c.Start(ctx) }()
	defer func() {
		cancel()
		if err := <-cacheStopped; err != nil {
			t.Errorf("failed to run cache: %v", err)
		}
	}()
	if !c.WaitForCacheSync(ctx) {
		t.Fatalf("failed to sync cache")
	}

	var list metallbv1beta1.ConfigurationStateList
	if err := c.List(ctx, &list); err != nil {
		t.Fatalf("failed to list ConfigurationStates: %v", err)
	}
	if len(list.Items) != 1 || list.Items[0].Namespace != namespace || list.Items[0].Name != "speaker-a" {
		names := []string{}
		for _, item := range list.Items {
			names = append(names, item.Namespace+"/"+item.Name)
		}
		t.Fatalf("expected only %s/speaker-a in the cache, got %v", namespace, names)
	}
	err = c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "speaker-b"}, &metallbv1beta1.ConfigurationState{})
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected NotFound getting another ConfigurationState from the cache, got %v", err)
	}

	// Writes to the other resources are not delivered, while the deletion and
	// creation of our own are, as the condition reporters rely on the latter.
	// Events arrive in the order of the writes, so had the writes to the other
	// resources been delivered, they would come before the DELETE.
	for _, name := range []string{"speaker-b", "controller"} {
		obj := configState(namespace, name)
		obj.Labels = map[string]string{"updated": "true"}
		if err := cl.Patch(ctx, obj, client.Merge); err != nil {
			t.Fatalf("failed to update ConfigurationState %s: %v", name, err)
		}
	}
	if err := cl.Delete(ctx, configState(namespace, "speaker-a")); err != nil {
		t.Fatalf("failed to delete ConfigurationState: %v", err)
	}
	if err := cl.Create(ctx, configState(namespace, "speaker-a")); err != nil {
		t.Fatalf("failed to create ConfigurationState: %v", err)
	}

	want := []string{"ADD metallb-system/speaker-a", "DELETE metallb-system/speaker-a", "ADD metallb-system/speaker-a"}
	var got []string
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		lock.Lock()
		got = slices.Clone(events)
		lock.Unlock()
		if len(got) >= len(want) {
			break
		}
	}
	if !slices.Equal(got, want) {
		t.Fatalf("expected events %v, got %v", want, got)
	}
}
