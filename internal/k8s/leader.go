package k8s

import (
	"os"
	"time"

	"go.universe.tf/metallb/internal/arp"

	"github.com/golang/glog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	le "k8s.io/client-go/tools/leaderelection"
	rl "k8s.io/client-go/tools/leaderelection/resourcelock"
)

// NewLeaderElector returns a new LeaderElector used for endpoint leader election using c.
func (c *Client) NewLeaderElector(a *arp.Announce, lockName string) (*le.LeaderElector, error) {
	conf := rl.ResourceLockConfig{Identity: hostname, EventRecorder: &noopEvent{}}
	lock, err := rl.New(rl.EndpointsResourceLock, "metallb-system", lockName, c.client.CoreV1(), conf)
	if err != nil {
		return nil, err
	}

	leader.Set(-1)

	lec := le.LeaderElectionConfig{
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
		Callbacks: le.LeaderCallbacks{
			OnStartedLeading: func(stop <-chan struct{}) {
				glog.Infof("Host %s acquiring leadership", hostname)

				leader.Set(1)
				a.SetLeader(true)

				go a.Acquire()
			},
			OnStoppedLeading: func() {
				glog.Infof("Host %s lost leadership", hostname)

				leader.Set(0)
				a.SetLeader(false)

				go a.Relinquish()
			},
		},
	}

	return le.NewLeaderElector(lec)
}

type noopEvent struct{}

// noopEvents implements the record.EventRecorder interface.
func (f *noopEvent) Event(object runtime.Object, eventtype, reason, message string) {
	/* noop */
}
func (f *noopEvent) Eventf(object runtime.Object, eventtype, reason, messageFmt string, args ...interface{}) {
	/* noop */
}
func (f *noopEvent) PastEventf(object runtime.Object, timestamp metav1.Time, eventtype, reason, messageFmt string, args ...interface{}) {
	/* noop */
}

var hostname = func() string {
	h, _ := os.Hostname()
	return h
}()
