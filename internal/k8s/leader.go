package k8s

import (
	"time"

	"go.universe.tf/metallb/internal/arp"

	"github.com/golang/glog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	le "k8s.io/client-go/tools/leaderelection"
	rl "k8s.io/client-go/tools/leaderelection/resourcelock"
)

// NewLeaderElector returns a new LeaderElector used for endpoint leader election using c.
func (c *Client) NewLeaderElector(a *arp.Announce, identity string) (*le.LeaderElector, error) {
	conf := rl.ResourceLockConfig{Identity: identity, EventRecorder: &noopEvent{}}
	lock, err := rl.New(rl.EndpointsResourceLock, "metallb-system", identity, c.client.CoreV1(), conf)
	if err != nil {
		return nil, err
	}

	lec := le.LeaderElectionConfig{
		Lock:          lock,
		LeaseDuration: 10 * time.Second,
		RenewDeadline: 5 * time.Second,
		RetryPeriod:   1 * time.Second,
		Callbacks: le.LeaderCallbacks{
			OnStartedLeading: func(stop <-chan struct{}) {
				glog.Infof("Acquiring leadership")
				a.Acquire()
			},
			OnStoppedLeading: func() {
				glog.Infof("Lost leadership")
				a.SetLeader(false)
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
