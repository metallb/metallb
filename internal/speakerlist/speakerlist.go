// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package speakerlist

import (
	"crypto/sha256"
	"strconv"
	"sync"
	"time"

	"go.universe.tf/metallb/internal/k8s"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/hashicorp/memberlist"
)

// SpeakerList represents a list of healthy speakers.
type SpeakerList struct {
	l         log.Logger
	client    *k8s.Client
	stopCh    chan struct{}
	namespace string
	labels    string

	// The following fields are nil when memberlist is disabled.
	mlEventCh chan memberlist.NodeEvent
	ml        *memberlist.Memberlist
	mlJoinCh  chan struct{}

	mlMux        sync.Mutex // Mutex for mlSpeakerIPs.
	mlSpeakerIPs []string   // Speaker pod IPs.
}

// New creates a new SpeakerList and returns a pointer to it.
func New(logger log.Logger, nodeName, bindAddr, bindPort, secret, namespace, labels string, WANNetwork bool, stopCh chan struct{}) (*SpeakerList, error) {
	sl := SpeakerList{
		l:         logger,
		stopCh:    stopCh,
		namespace: namespace,
		labels:    labels,
	}

	if labels == "" || bindAddr == "" {
		level.Info(logger).Log("op", "startup", "msg", "not starting fast dead node detection (memberlist), need ml-bindaddr / ml-labels config")
		return &sl, nil
	}

	mconfig := memberlist.DefaultLANConfig()
	if WANNetwork {
		mconfig = memberlist.DefaultWANConfig()
	}

	// mconfig.Name MUST be equal to the spec.nodeName field of the speaker pod as we match it
	// against the nodeName field of Endpoint objects inside usableNodes().
	mconfig.Name = nodeName
	mconfig.BindAddr = bindAddr
	if bindPort != "" {
		mlport, err := strconv.Atoi(bindPort)
		if err != nil {
			level.Error(logger).Log("op", "startup", "error", "unable to parse ml-bindport", "msg", err)
			return nil, err
		}
		mconfig.BindPort = mlport
		mconfig.AdvertisePort = mlport
	}
	mconfig.Logger = newMemberlistLogger(sl.l)
	if secret == "" {
		level.Warn(logger).Log("op", "startup", "warning", "no ml-secret-key set, memberlist traffic will not be encrypted")
	} else {
		sha := sha256.New()
		mconfig.SecretKey = sha.Sum([]byte(secret))[:32]
	}

	// This channel is used by the Rejoin() method which runs on k8s node
	// changes. A buffered channel is used to avoid blocking the main goroutine
	// while waiting for the join operation to complete.
	sl.mlJoinCh = make(chan struct{}, 1)

	// ChannelEventDelegate hints that it should not block, so make mlEventCh
	// 'big'.
	// TODO: See https://github.com/metallb/metallb/issues/716
	sl.mlEventCh = make(chan memberlist.NodeEvent, 1024)
	mconfig.Events = &memberlist.ChannelEventDelegate{Ch: sl.mlEventCh}

	ml, err := memberlist.Create(mconfig)
	if err != nil {
		level.Error(logger).Log("op", "startup", "error", err, "msg", "failed to create memberlist")
		return nil, err
	}

	sl.ml = ml

	return &sl, nil
}

// Start initializes the SpeakerList. This functions must be called before using
// other SpeakerList methods.
func (sl *SpeakerList) Start(client *k8s.Client) {
	// TODO: The k8s client parameter should ideally be a parameter to
	// New(). However, that is not possible today because:
	//
	// 1. The newController() function takes the sList as param[1]
	// 2. The newController() function uses the sList param to create the layer 2 controller[2]
	// 3. The new Kubernetes client uses the created controller to set the callbacks[3]
	//
	// Then, we have a dependency cycle if we add the Kubernetes client to speakerlist.New():
	//
	// 1. To create a Kubernetes client we need a controller
	// 2. The newController() function uses the sList param to create the layer 2 controller[2]
	// 3. The new Kubernetes client uses the created controller to set the callbacks[3]
	//
	// We should probably move the speakerlist code to be an implementation
	// detail of the Layer 2 controller and, when doing so, try to fix this
	// problem.
	//
	// [1]: https://github.com/metallb/metallb/pull/662/files#diff-60053ad6fecb5a3cfabb6f3d9e720899R109
	// [2]: https://github.com/metallb/metallb/pull/662/files#diff-60053ad6fecb5a3cfabb6f3d9e720899L232-L251
	// [3]: https://github.com/metallb/metallb/pull/662/files#diff-60053ad6fecb5a3cfabb6f3d9e720899L160-L162

	sl.client = client

	if sl.ml == nil {
		return
	}

	// Initialize sl.mlSpeakerIPs.
	iplist, err := sl.mlSpeakers()
	if err != nil {
		level.Error(sl.l).Log("op", "memberDiscovery", "error", err, "msg", "failed to get pod IPs")
		iplist = nil
	}

	sl.mlMux.Lock()
	sl.mlSpeakerIPs = iplist
	sl.mlMux.Unlock()

	// Update mlSpeakerIPs in the background.
	go sl.updateSpeakerIPs()

	go sl.memberlistWatchEvents()
	go sl.joinMembers()
}

// updateSpeakerIPs runs forever updating the sl.mlSpeakerIPs slice with the
// IPs of all the speaker pods. As the function queries the API server, it waits at
// least 5 minutes between queries to avoid self-induced API server DoS (should
// be treated with care).
func (sl *SpeakerList) updateSpeakerIPs() {
	for {
		// This either stops the loop or makes sure to wait at least 5
		// minutes before making another call to sl.mlSpeakers(), which
		// queries the API server.
		select {
		case <-sl.stopCh:
			return
		case <-time.After(5 * time.Minute):
			// This blocks until the API server responds.
			iplist, err := sl.mlSpeakers()
			if err != nil {
				level.Error(sl.l).Log("op", "memberDiscovery", "error", err, "msg", "failed to get pod IPs")
				continue
			}

			sl.mlMux.Lock()
			sl.mlSpeakerIPs = iplist
			sl.mlMux.Unlock()
		}
	}
}

func (sl *SpeakerList) mlSpeakers() ([]string, error) {
	// This call blocks until we get a response from the API server.
	// In the client-go version we are using, there is no way to use a
	// context. Newer versions of client-go support using a context
	// in the call sl.client.PodIPs() is using.
	//
	// TODO: When updating client-go, this code can be simplified by using a
	// context in the following way:
	// If we use a context, we can specify a timeout and then there is no need to
	// run mlUpdateSpeaker() in the background. We could then call
	// this function from mlJoin() with a context (timeout) and use
	// sl.mlSpeakerIPs on failure. On success, we could update
	// sl.mlSpeakerIPs with the response and continue to do the join.
	iplist, err := sl.client.PodIPs(sl.namespace, sl.labels)
	if err != nil {
		return nil, err
	}

	return iplist, nil
}

func (sl *SpeakerList) joinMembers() {
	// Every one minute, try to rejoin.
	// This joins nodes that leave the cluster for just a few seconds.
	// Discovering new IPs (updated by sl.updateSpeakerIPs()) might take a while
	// longer.
	ticker := time.NewTicker(1 * time.Minute)

	for {
		select {
		case <-sl.stopCh:
			return
		case <-ticker.C:
			sl.mlJoin()
		case <-sl.mlJoinCh:
			sl.mlJoin()
		}
	}
}

func (sl *SpeakerList) members() map[string]struct{} {
	members := make(map[string]struct{})

	for _, node := range sl.ml.Members() {
		ip := node.Addr.String()
		members[ip] = struct{}{}
	}

	return members
}

// mlJoin joins speaker pods that are not members of this cluster
// to the cluster. It performs a memberlist.Join() with the IPs in
// mlSpeakerIPs that are not members of the cluster.
func (sl *SpeakerList) mlJoin() {
	// IPs to be joined to the memberlist cluster.
	var joinIPs []string

	members := sl.members()

	sl.mlMux.Lock()
	defer sl.mlMux.Unlock()

	for _, ip := range sl.mlSpeakerIPs {
		// If an IP is not a member of the cluster, add it to joinIPs.
		if _, isMember := members[ip]; !isMember {
			joinIPs = append(joinIPs, ip)
		}
	}

	if len(joinIPs) == 0 {
		// Not logging here to avoid spamming the logs.
		return
	}

	nr, err := sl.ml.Join(joinIPs)
	if err != nil || nr != len(joinIPs) {
		level.Error(sl.l).Log("op", "memberDiscovery", "msg", "partial join", "joined", nr, "expected", len(joinIPs), "error", err)
	} else {
		level.Info(sl.l).Log("op", "Member detection", "msg", "memberlist join successfully", "number of other nodes", nr)
	}
}

// Rejoin initiates a discovery and joins all the speakers to the memberlist
// cluster.
func (sl *SpeakerList) Rejoin() {
	if sl.ml == nil {
		return
	}

	select {
	case sl.mlJoinCh <- struct{}{}:
		level.Info(sl.l).Log("op", "memberDiscovery", "msg", "triggering discovery")
	default:
		level.Debug(sl.l).Log("op", "memberDiscovery", "msg", "previous discovery in progress - doing nothing")
	}
}

// UsableSpeakers returns a map of usable speaker nodes.
func (sl *SpeakerList) UsableSpeakers() map[string]bool {
	if sl.ml == nil {
		return nil
	}
	activeNodes := map[string]bool{}
	for _, n := range sl.ml.Members() {
		activeNodes[n.Name] = true
	}
	return activeNodes
}

// Stop stops the SpeakerList.
func (sl *SpeakerList) Stop() {
	if sl.ml == nil {
		return
	}

	level.Info(sl.l).Log("op", "shutdown", "msg", "leaving memberlist cluster")
	err := sl.ml.Leave(time.Second)
	level.Info(sl.l).Log("op", "shutdown", "msg", "left memberlist cluster", "error", err)
	err = sl.ml.Shutdown()
	level.Info(sl.l).Log("op", "shutdown", "msg", "memberlist shutdown", "error", err)
}

func event2String(e memberlist.NodeEventType) string {
	return []string{"NodeJoin", "NodeLeave", "NodeUpdate"}[e]
}

func (sl *SpeakerList) memberlistWatchEvents() {
	for {
		select {
		case e := <-sl.mlEventCh:
			level.Info(sl.l).Log("msg", "node event - forcing sync", "node addr", e.Node.Addr, "node name", e.Node.Name, "node event", event2String(e.Event))
			sl.client.ForceSync()
		case <-sl.stopCh:
			return
		}
	}
}
