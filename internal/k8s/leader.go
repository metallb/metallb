package k8s

import (
	"time"

	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

// HandleLeadership starts a leader election and notifies handler of
// changes to leadership state.
func (c *Client) HandleLeadership(nodeName string, handler func(bool)) {
	if c.elector != nil {
		panic("HandleLeadership called twice")
	}
	conf := resourcelock.ResourceLockConfig{Identity: nodeName, EventRecorder: c.events}
	lock, err := resourcelock.New(resourcelock.EndpointsResourceLock, "metallb-system", "metallb-speaker", c.client.CoreV1(), conf)
	if err != nil {
		panic(err)
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
		panic(err)
	}
	c.elector = elector
	c.leaderChanged = handler
}
