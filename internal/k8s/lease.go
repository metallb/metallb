package k8s

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	coordinationv1 "k8s.io/api/coordination/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	// DefaultLeaseDuration is the default duration for Layer2 leader election leases.
	DefaultLeaseDuration = 15 * time.Second
	// DefaultRenewDeadline is the default renew deadline for Layer2 leader election leases.
	DefaultRenewDeadline = 10 * time.Second
	// DefaultRetryPeriod is the default retry period for Layer2 leader election leases.
	DefaultRetryPeriod = 2 * time.Second
)

// Layer2LeaseManager manages Kubernetes Leases for Layer2 announcements to prevent split-brain.
type Layer2LeaseManager struct {
	client        kubernetes.Interface
	namespace     string
	leaseName     string
	identity      string
	logger        log.Logger
	leaseDuration time.Duration
	renewDeadline time.Duration
	retryPeriod   time.Duration
}

// NewLayer2LeaseManager creates a new Layer2 lease manager.
func NewLayer2LeaseManager(client kubernetes.Interface, namespace, leaseName, identity string, logger log.Logger) *Layer2LeaseManager {
	return &Layer2LeaseManager{
		client:        client,
		namespace:     namespace,
		leaseName:     leaseName,
		identity:      identity,
		logger:        logger,
		leaseDuration: DefaultLeaseDuration,
		renewDeadline: DefaultRenewDeadline,
		retryPeriod:   DefaultRetryPeriod,
	}
}

// SetLeaseTimings allows customization of lease timing parameters.
func (lm *Layer2LeaseManager) SetLeaseTimings(leaseDuration, renewDeadline, retryPeriod time.Duration) {
	lm.leaseDuration = leaseDuration
	lm.renewDeadline = renewDeadline
	lm.retryPeriod = retryPeriod
}

// TryAcquireLease attempts to acquire a lease for the given IP address
// Returns true if the lease was acquired, false otherwise.
func (lm *Layer2LeaseManager) TryAcquireLease(ctx context.Context, ip string) (bool, error) {
	leaseName := fmt.Sprintf("%s-%s", lm.leaseName, ip)

	// Check if lease already exists and who holds it
	lease, err := lm.client.CoordinationV1().Leases(lm.namespace).Get(ctx, leaseName, metav1.GetOptions{})
	if err != nil {
		// Lease doesn't exist, try to create it
		leaseDurationSeconds := int32(lm.leaseDuration.Seconds())
		newLease := &coordinationv1.Lease{
			ObjectMeta: metav1.ObjectMeta{
				Name:      leaseName,
				Namespace: lm.namespace,
			},
			Spec: coordinationv1.LeaseSpec{
				HolderIdentity:       &lm.identity,
				LeaseDurationSeconds: &leaseDurationSeconds,
				RenewTime:            &metav1.MicroTime{Time: time.Now()},
			},
		}

		_, err = lm.client.CoordinationV1().Leases(lm.namespace).Create(ctx, newLease, metav1.CreateOptions{})
		if err != nil {
			level.Debug(lm.logger).Log("op", "TryAcquireLease", "ip", ip, "msg", "failed to create lease", "error", err)
			return false, nil
		}

		level.Info(lm.logger).Log("op", "TryAcquireLease", "ip", ip, "msg", "successfully acquired lease")
		return true, nil
	}

	// Lease exists, check if it's expired or if we can take it over
	now := time.Now()
	if lease.Spec.RenewTime != nil && lease.Spec.LeaseDurationSeconds != nil {
		expiryTime := lease.Spec.RenewTime.Add(time.Duration(*lease.Spec.LeaseDurationSeconds) * time.Second)
		if now.After(expiryTime) {
			// Lease is expired, try to take it over
			lease.Spec.HolderIdentity = &lm.identity
			lease.Spec.RenewTime = &metav1.MicroTime{Time: now}

			_, err = lm.client.CoordinationV1().Leases(lm.namespace).Update(ctx, lease, metav1.UpdateOptions{})
			if err != nil {
				level.Debug(lm.logger).Log("op", "TryAcquireLease", "ip", ip, "msg", "failed to take over expired lease", "error", err)
				return false, nil
			}

			level.Info(lm.logger).Log("op", "TryAcquireLease", "ip", ip, "msg", "successfully took over expired lease")
			return true, nil
		}
	}

	// Lease is held by someone else and not expired
	if lease.Spec.HolderIdentity != nil && *lease.Spec.HolderIdentity != lm.identity {
		level.Debug(lm.logger).Log("op", "TryAcquireLease", "ip", ip, "holder", *lease.Spec.HolderIdentity, "msg", "lease held by another speaker")
		return false, nil
	}

	// We already hold this lease, renew it
	lease.Spec.RenewTime = &metav1.MicroTime{Time: now}
	_, err = lm.client.CoordinationV1().Leases(lm.namespace).Update(ctx, lease, metav1.UpdateOptions{})
	if err != nil {
		level.Warn(lm.logger).Log("op", "TryAcquireLease", "ip", ip, "msg", "failed to renew lease", "error", err)
		return false, err
	}

	return true, nil
}

// RenewLease renews the lease for the given IP address.
func (lm *Layer2LeaseManager) RenewLease(ctx context.Context, ip string) error {
	leaseName := fmt.Sprintf("%s-%s", lm.leaseName, ip)

	lease, err := lm.client.CoordinationV1().Leases(lm.namespace).Get(ctx, leaseName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get lease %s: %w", leaseName, err)
	}

	if lease.Spec.HolderIdentity == nil || *lease.Spec.HolderIdentity != lm.identity {
		return fmt.Errorf("lease %s is not held by this speaker", leaseName)
	}

	lease.Spec.RenewTime = &metav1.MicroTime{Time: time.Now()}
	_, err = lm.client.CoordinationV1().Leases(lm.namespace).Update(ctx, lease, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to renew lease %s: %w", leaseName, err)
	}

	return nil
}

// ReleaseLease releases the lease for the given IP address.
func (lm *Layer2LeaseManager) ReleaseLease(ctx context.Context, ip string) error {
	leaseName := fmt.Sprintf("%s-%s", lm.leaseName, ip)

	lease, err := lm.client.CoordinationV1().Leases(lm.namespace).Get(ctx, leaseName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get lease %s: %w", leaseName, err)
	}

	if lease.Spec.HolderIdentity == nil || *lease.Spec.HolderIdentity != lm.identity {
		level.Debug(lm.logger).Log("op", "ReleaseLease", "ip", ip, "msg", "lease not held by this speaker, nothing to release")
		return nil
	}

	// Delete the lease to release it
	err = lm.client.CoordinationV1().Leases(lm.namespace).Delete(ctx, leaseName, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete lease %s: %w", leaseName, err)
	}

	level.Info(lm.logger).Log("op", "ReleaseLease", "ip", ip, "msg", "successfully released lease")
	return nil
}

// StartLeaseRenewal starts a background goroutine to renew the lease for the given IP.
func (lm *Layer2LeaseManager) StartLeaseRenewal(ctx context.Context, ip string) {
	go func() {
		ticker := time.NewTicker(lm.retryPeriod)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				level.Debug(lm.logger).Log("op", "StartLeaseRenewal", "ip", ip, "msg", "lease renewal stopped")
				return
			case <-ticker.C:
				err := lm.RenewLease(ctx, ip)
				if err != nil {
					level.Warn(lm.logger).Log("op", "StartLeaseRenewal", "ip", ip, "msg", "failed to renew lease", "error", err)
				}
			}
		}
	}()
}
