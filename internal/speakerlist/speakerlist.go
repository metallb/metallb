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
	golog "log"
	"strconv"
	"time"

	"go.universe.tf/metallb/internal/k8s"

	gokitlog "github.com/go-kit/kit/log"
	"github.com/hashicorp/memberlist"
)

// SpeakerList represents a list of healthy speakers.
type SpeakerList struct {
	l         gokitlog.Logger
	client    *k8s.Client
	stopCh    chan struct{}
	namespace string
	labels    string

	// The following fields are nil when memberlist is disabled.
	mlEventCh chan memberlist.NodeEvent
	ml        *memberlist.Memberlist
}

// New creates a new SpeakerList and returns a pointer to it.
func New(logger gokitlog.Logger, nodeName, bindAddr, bindPort, secret, namespace, labels string, stopCh chan struct{}) (*SpeakerList, error) {
	sl := SpeakerList{
		l:         logger,
		stopCh:    stopCh,
		namespace: namespace,
		labels:    labels,
	}

	if namespace == "" || labels == "" || bindAddr == "" {
		logger.Log("op", "startup", "msg", "not starting fast dead node detection (memberlist), need ml-bindaddr / ml-labels / ml-namespace config")
		return &sl, nil
	}

	mconfig := memberlist.DefaultLANConfig()

	// mconfig.Name MUST be equal to the spec.nodeName field of the speaker pod as we match it
	// against the nodeName field of Endpoint objects inside usableNodes().
	mconfig.Name = nodeName
	mconfig.BindAddr = bindAddr
	if bindPort != "" {
		mlport, err := strconv.Atoi(bindPort)
		if err != nil {
			logger.Log("op", "startup", "error", "unable to parse ml-bindport", "msg", err)
			return nil, err
		}
		mconfig.BindPort = mlport
		mconfig.AdvertisePort = mlport
	}
	loggerout := gokitlog.NewStdlibAdapter(gokitlog.With(sl.l, "component", "Memberlist"))
	mconfig.Logger = golog.New(loggerout, "", golog.Lshortfile)
	if secret == "" {
		logger.Log("op", "startup", "warning", "no ml-secret-key set, memberlist traffic will not be encrypted")
	} else {
		sha := sha256.New()
		mconfig.SecretKey = sha.Sum([]byte(secret))[:16]
	}

	// ChannelEventDelegate hint that it should not block, so make mlEventCh
	// 'big'.
	sl.mlEventCh = make(chan memberlist.NodeEvent, 1024)
	mconfig.Events = &memberlist.ChannelEventDelegate{Ch: sl.mlEventCh}

	ml, err := memberlist.Create(mconfig)
	if err != nil {
		logger.Log("op", "startup", "error", err, "msg", "failed to create memberlist")
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
	// 2. The newController() function uses it to create the layer 2 controller [2]
	// 3. The new Kubernetes client uses the created controller to set the callbacks [3]
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

	go sl.memberlistWatchEvents()

	iplist, err := sl.client.GetPodsIPs(sl.namespace, sl.labels)
	if err != nil {
		sl.l.Log("op", "startup", "error", err, "msg", "failed to get pod IPs")
		return err
	}
	n, err := sl.ml.Join(iplist)
	sl.l.Log("op", "startup", "msg", "Memberlist join", "nb joigned", n, "error", err)

	return nil
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

	sl.l.Log("op", "shutdown", "msg", "leaving memberlist cluster")
	err := sl.ml.Leave(time.Second)
	sl.l.Log("op", "shutdown", "msg", "left memberlist cluster", "error", err)
	err = sl.ml.Shutdown()
	sl.l.Log("op", "shutdown", "msg", "memberlist shutdown", "error", err)
}

func event2String(e memberlist.NodeEventType) string {
	return []string{"NodeJoin", "NodeLeave", "NodeUpdate"}[e]
}

func (sl *SpeakerList) memberlistWatchEvents() {
	for {
		select {
		case e := <-sl.mlEventCh:
			sl.l.Log("msg", "node event - forcing sync", "node addr", e.Node.Addr, "node name", e.Node.Name, "node event", event2String(e.Event))
			sl.client.ForceSync()
		case <-sl.stopCh:
			return
		}
	}
}
