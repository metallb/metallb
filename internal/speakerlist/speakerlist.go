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

// SpeakerList gives you the list of healthy speaker
type SpeakerList struct {
	l         gokitlog.Logger
	client    *k8s.Client
	stopCh    chan struct{}
	namespace string
	labels    string
	// following fields are nil when MemberList is disabled
	mlEventCh chan memberlist.NodeEvent
	mList     *memberlist.Memberlist
}

// New creates a new SpeakerList
func New(logger gokitlog.Logger, nodeName, bindAddr, bindPort, secret, namespace, labels string, stopCh chan struct{}) (*SpeakerList, error) {
	sl := SpeakerList{
		l:         logger,
		stopCh:    stopCh,
		namespace: namespace,
		labels:    labels,
	}

	if namespace == "" || labels == "" || bindAddr == "" {
		logger.Log("op", "startup", "msg", "Not starting fast dead node detection (MemberList), need ml-bindaddr / ml-labels / ml-namespace config")
		return &sl, nil
	}

	mconfig := memberlist.DefaultLANConfig()
	// mconfig.Name MUST be spec.nodeName, as we will match it against Enpoints nodeName in usableNodes()
	mconfig.Name = nodeName
	mconfig.BindAddr = bindAddr
	if bindPort != "" {
		mlport, err := strconv.Atoi(bindPort)
		if err != nil {
			sl.l.Log("op", "startup", "error", "unable to parse ml-bindport", "msg", err)
			return nil, err
		}
		mconfig.BindPort = mlport
		mconfig.AdvertisePort = mlport
	}
	loggerout := gokitlog.NewStdlibAdapter(gokitlog.With(sl.l, "component", "MemberList"))
	mconfig.Logger = golog.New(loggerout, "", golog.Lshortfile)
	if secret == "" {
		sl.l.Log("op", "startup", "warning", "no ml-secret-key set, MemberList traffic will not be encrypted")
	} else {
		sha := sha256.New()
		mconfig.SecretKey = sha.Sum([]byte(secret))[:16]
	}
	// ChannelEventDelegate hint that it should not block, so make mlEventCh 'big'
	sl.mlEventCh = make(chan memberlist.NodeEvent, 1024)
	mconfig.Events = &memberlist.ChannelEventDelegate{Ch: sl.mlEventCh}
	var err error
	sl.mList, err = memberlist.Create(mconfig)
	if err != nil {
		sl.l.Log("op", "startup", "error", err, "msg", "failed to create memberlist")
		return nil, err
	}
	return &sl, nil
}

// Start starts the needed goroutines
func (sl *SpeakerList) Start(client *k8s.Client) error {
	sl.client = client

	if sl.mList == nil {
		return nil
	}

	go sl.memberlistWatchEvents()

	iplist, err := sl.client.GetPodsIPs(sl.namespace, sl.labels)
	if err != nil {
		sl.l.Log("op", "startup", "error", err, "msg", "failed to get PodsIPs")
		return err
	}
	n, err := sl.mList.Join(iplist)
	sl.l.Log("op", "startup", "msg", "Memberlist join", "nb joigned", n, "error", err)

	return nil
}

// UsableSpeakers return a map of usable nodes
func (sl *SpeakerList) UsableSpeakers() map[string]bool {
	if sl.mList == nil {
		return nil
	}
	activeNodes := map[string]bool{}
	for _, n := range sl.mList.Members() {
		activeNodes[n.Name] = true
	}
	return activeNodes
}

// Stop properly Leave / Shutdown MemberList cluster
func (sl *SpeakerList) Stop() {
	if sl.mList == nil {
		return
	}
	sl.l.Log("op", "shutdown", "msg", "leaving MemberList cluster")
	err := sl.mList.Leave(time.Second)
	sl.l.Log("op", "shutdown", "msg", "left MemberList cluster", "error", err)
	err = sl.mList.Shutdown()
	sl.l.Log("op", "shutdown", "msg", "MemberList shutdown", "error", err)
}

func event2String(e memberlist.NodeEventType) string {
	return []string{"NodeJoin", "NodeLeave", "NodeUpdate"}[e]
}

func (sl *SpeakerList) memberlistWatchEvents() {
	for {
		select {
		case e := <-sl.mlEventCh:
			sl.l.Log("msg", "Node event", "node addr", e.Node.Addr, "node name", e.Node.Name, "node event", event2String(e.Event))
			sl.l.Log("msg", "Call Force Sync")
			sl.client.ForceSync()
		case <-sl.stopCh:
			return
		}
	}
}
