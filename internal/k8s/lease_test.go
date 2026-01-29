// SPDX-License-Identifier:Apache-2.0

package k8s

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/go-kit/log"
	coordinationv1 "k8s.io/api/coordination/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	coordinationv1applyconfigurations "k8s.io/client-go/applyconfigurations/coordination/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	coordinationv1client "k8s.io/client-go/kubernetes/typed/coordination/v1"
	"k8s.io/client-go/rest"
)

// mockK8sClient wraps the fake client to provide better control over responses
type mockK8sClient struct {
	kubernetes.Interface
	leases map[string]*coordinationv1.Lease
	errors map[string]error
}

func newMockK8sClient() *mockK8sClient {
	client := fake.NewSimpleClientset()
	return &mockK8sClient{
		Interface: client,
		leases:    make(map[string]*coordinationv1.Lease),
		errors:    make(map[string]error),
	}
}

func (m *mockK8sClient) CoordinationV1() coordinationv1client.CoordinationV1Interface {
	return &mockCoordinationV1{m}
}

type mockCoordinationV1 struct {
	client *mockK8sClient
}

func (m *mockCoordinationV1) Leases(namespace string) coordinationv1client.LeaseInterface {
	return &mockLeaseInterface{m.client, namespace}
}

func (m *mockCoordinationV1) RESTClient() rest.Interface {
	return nil
}

type mockLeaseInterface struct {
	client    *mockK8sClient
	namespace string
}

func (m *mockLeaseInterface) Create(ctx context.Context, lease *coordinationv1.Lease, opts metav1.CreateOptions) (*coordinationv1.Lease, error) {
	leaseKey := fmt.Sprintf("%s/%s", m.namespace, lease.Name)
	if err, exists := m.client.errors[leaseKey]; exists {
		return nil, err
	}
	m.client.leases[leaseKey] = lease.DeepCopy()
	return lease, nil
}

func (m *mockLeaseInterface) Update(ctx context.Context, lease *coordinationv1.Lease, opts metav1.UpdateOptions) (*coordinationv1.Lease, error) {
	leaseKey := fmt.Sprintf("%s/%s", m.namespace, lease.Name)
	if err, exists := m.client.errors[leaseKey]; exists {
		return nil, err
	}
	m.client.leases[leaseKey] = lease.DeepCopy()
	return lease, nil
}

func (m *mockLeaseInterface) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	leaseKey := fmt.Sprintf("%s/%s", m.namespace, name)
	if err, exists := m.client.errors[leaseKey]; exists {
		return err
	}
	delete(m.client.leases, leaseKey)
	return nil
}

func (m *mockLeaseInterface) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return fmt.Errorf("not implemented")
}

func (m *mockLeaseInterface) Get(ctx context.Context, name string, opts metav1.GetOptions) (*coordinationv1.Lease, error) {
	leaseKey := fmt.Sprintf("%s/%s", m.namespace, name)
	if err, exists := m.client.errors[leaseKey]; exists {
		return nil, err
	}
	if lease, exists := m.client.leases[leaseKey]; exists {
		return lease.DeepCopy(), nil
	}
	return nil, errors.NewNotFound(schema.GroupResource{Resource: "leases"}, name)
}

func (m *mockLeaseInterface) List(ctx context.Context, opts metav1.ListOptions) (*coordinationv1.LeaseList, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockLeaseInterface) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockLeaseInterface) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *coordinationv1.Lease, err error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockLeaseInterface) Apply(ctx context.Context, lease *coordinationv1applyconfigurations.LeaseApplyConfiguration, opts metav1.ApplyOptions) (result *coordinationv1.Lease, err error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockLeaseInterface) ApplyStatus(ctx context.Context, lease *coordinationv1applyconfigurations.LeaseApplyConfiguration, opts metav1.ApplyOptions) (result *coordinationv1.Lease, err error) {
	return nil, fmt.Errorf("not implemented")
}

func TestNewLayer2LeaseManager(t *testing.T) {
	logger := log.NewNopLogger()
	client := newMockK8sClient()
	
	lm := NewLayer2LeaseManager(client, "test-namespace", "metallb-layer2", "test-node", logger)
	
	if lm.client != client {
		t.Error("client not set correctly")
	}
	if lm.namespace != "test-namespace" {
		t.Error("namespace not set correctly")
	}
	if lm.leaseName != "metallb-layer2" {
		t.Error("leaseName not set correctly")
	}
	if lm.identity != "test-node" {
		t.Error("identity not set correctly")
	}
	if lm.logger != logger {
		t.Error("logger not set correctly")
	}
	if lm.leaseDuration != DefaultLeaseDuration {
		t.Error("leaseDuration not set to default")
	}
	if lm.renewDeadline != DefaultRenewDeadline {
		t.Error("renewDeadline not set to default")
	}
	if lm.retryPeriod != DefaultRetryPeriod {
		t.Error("retryPeriod not set to default")
	}
}

func TestSetLeaseTimings(t *testing.T) {
	logger := log.NewNopLogger()
	client := newMockK8sClient()
	lm := NewLayer2LeaseManager(client, "test-namespace", "test-lease", "test-identity", logger)
	
	customDuration := 30 * time.Second
	customRenewDeadline := 20 * time.Second
	customRetryPeriod := 5 * time.Second
	
	lm.SetLeaseTimings(customDuration, customRenewDeadline, customRetryPeriod)
	
	if lm.leaseDuration != customDuration {
		t.Error("leaseDuration not updated correctly")
	}
	if lm.renewDeadline != customRenewDeadline {
		t.Error("renewDeadline not updated correctly")
	}
	if lm.retryPeriod != customRetryPeriod {
		t.Error("retryPeriod not updated correctly")
	}
}

func TestTryAcquireLease_NewLease(t *testing.T) {
	logger := log.NewNopLogger()
	client := newMockK8sClient()
	lm := NewLayer2LeaseManager(client, "test-namespace", "metallb-layer2", "test-node", logger)
	
	ctx := context.Background()
	ip := "192.168.1.1"
	
	acquired, err := lm.TryAcquireLease(ctx, ip)
	
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !acquired {
		t.Error("expected lease to be acquired")
	}
	
	// Verify lease was created
	leaseKey := fmt.Sprintf("%s/metallb-layer2-%s", "test-namespace", ip)
	if _, exists := client.leases[leaseKey]; !exists {
		t.Error("lease was not created")
	}
}

func TestTryAcquireLease_IPv6Address(t *testing.T) {
	logger := log.NewNopLogger()
	client := newMockK8sClient()
	lm := NewLayer2LeaseManager(client, "test-namespace", "metallb-layer2", "test-node", logger)
	
	ctx := context.Background()
	ip := "2001:db8::1"
	
	acquired, err := lm.TryAcquireLease(ctx, ip)
	
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !acquired {
		t.Error("expected lease to be acquired for IPv6 address")
	}
	
	// Verify lease was created
	leaseKey := fmt.Sprintf("%s/metallb-layer2-%s", "test-namespace", ip)
	if _, exists := client.leases[leaseKey]; !exists {
		t.Error("IPv6 lease was not created")
	}
}

func TestTryAcquireLease_ExistingLease(t *testing.T) {
	logger := log.NewNopLogger()
	client := newMockK8sClient()
	lm := NewLayer2LeaseManager(client, "test-namespace", "metallb-layer2", "test-node", logger)
	
	ctx := context.Background()
	ip := "192.168.1.1"
	
	// First acquisition should succeed
	acquired, err := lm.TryAcquireLease(ctx, ip)
	if err != nil {
		t.Errorf("unexpected error on first acquisition: %v", err)
	}
	if !acquired {
		t.Error("expected first lease acquisition to succeed")
	}
	
	// Second acquisition should succeed (we already hold the lease, so it gets renewed)
	acquired, err = lm.TryAcquireLease(ctx, ip)
	if err != nil {
		t.Errorf("unexpected error on second acquisition: %v", err)
	}
	if !acquired {
		t.Error("expected second lease acquisition to succeed (renewal)")
	}
}

func TestTryAcquireLease_ClientError(t *testing.T) {
	logger := log.NewNopLogger()
	client := newMockK8sClient()
	lm := NewLayer2LeaseManager(client, "test-namespace", "metallb-layer2", "test-node", logger)
	
	ctx := context.Background()
	ip := "192.168.1.1"
	
	// Set up an error for the lease creation
	leaseKey := fmt.Sprintf("%s/metallb-layer2-%s", "test-namespace", ip)
	client.errors[leaseKey] = fmt.Errorf("simulated client error")
	
	acquired, err := lm.TryAcquireLease(ctx, ip)
	
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if acquired {
		t.Error("expected lease acquisition to fail due to client error")
	}
}

func TestTryAcquireLease_LeaseHeldByOther(t *testing.T) {
	logger := log.NewNopLogger()
	client := newMockK8sClient()
	lm := NewLayer2LeaseManager(client, "test-namespace", "metallb-layer2", "test-node", logger)
	
	ctx := context.Background()
	ip := "192.168.1.1"
	
	// Create a lease held by a different node
	leaseName := fmt.Sprintf("metallb-layer2-%s", ip)
	leaseDurationSeconds := int32(15)
	otherNode := "other-node"
	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      leaseName,
			Namespace: "test-namespace",
		},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity:       &otherNode,
			LeaseDurationSeconds: &leaseDurationSeconds,
			RenewTime:            &metav1.MicroTime{Time: time.Now()},
		},
	}
	
	leaseKey := fmt.Sprintf("%s/%s", "test-namespace", leaseName)
	client.leases[leaseKey] = lease
	
	// Try to acquire the lease
	acquired, err := lm.TryAcquireLease(ctx, ip)
	
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if acquired {
		t.Error("expected lease acquisition to fail when held by another node")
	}
}

func TestRenewLease_Success(t *testing.T) {
	logger := log.NewNopLogger()
	client := newMockK8sClient()
	lm := NewLayer2LeaseManager(client, "test-namespace", "metallb-layer2", "test-node", logger)
	
	ctx := context.Background()
	ip := "192.168.1.1"
	
	// First acquire the lease
	acquired, err := lm.TryAcquireLease(ctx, ip)
	if err != nil || !acquired {
		t.Fatal("failed to acquire initial lease")
	}
	
	// Now renew it
	err = lm.RenewLease(ctx, ip)
	if err != nil {
		t.Errorf("unexpected error renewing lease: %v", err)
	}
}

func TestRenewLease_NoExistingLease(t *testing.T) {
	logger := log.NewNopLogger()
	client := newMockK8sClient()
	lm := NewLayer2LeaseManager(client, "test-namespace", "metallb-layer2", "test-node", logger)
	
	ctx := context.Background()
	ip := "192.168.1.1"
	
	// Try to renew a lease that doesn't exist
	err := lm.RenewLease(ctx, ip)
	if err == nil {
		t.Error("expected error when renewing non-existent lease")
	}
}

func TestReleaseLease_Success(t *testing.T) {
	logger := log.NewNopLogger()
	client := newMockK8sClient()
	lm := NewLayer2LeaseManager(client, "test-namespace", "metallb-layer2", "test-node", logger)
	
	ctx := context.Background()
	ip := "192.168.1.1"
	
	// First acquire the lease
	acquired, err := lm.TryAcquireLease(ctx, ip)
	if err != nil || !acquired {
		t.Fatal("failed to acquire initial lease")
	}
	
	// Now release it
	err = lm.ReleaseLease(ctx, ip)
	if err != nil {
		t.Errorf("unexpected error releasing lease: %v", err)
	}
	
	// Verify lease was deleted
	leaseKey := fmt.Sprintf("%s/metallb-layer2-%s", "test-namespace", ip)
	if _, exists := client.leases[leaseKey]; exists {
		t.Error("lease should have been deleted")
	}
}
