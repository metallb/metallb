package main

import (
	"time"

	"go.universe.tf/metallb/internal/arp"

	"github.com/golang/glog"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	le "k8s.io/client-go/tools/leaderelection"
	rl "k8s.io/client-go/tools/leaderelection/resourcelock"
)

// NewLeaderElector returns a new LeaderElector used for endpoint leader election.
func NewLeaderElector(a *arp.Announce, ns string, client corev1.CoreV1Interface) (*le.LeaderElector, error) {
	lock, err := rl.New(rl.EndpointsResourceLock, ns, identity, client, rl.ResourceLockConfig{Identity: identity, EventRecorder: nil})
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

const identity = "metallb-arp-speaker"
