// Copyright 2018 Cisco Systems Inc.
//
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

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"k8s.io/api/core/v1"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-kit/kit/log"
	"go.universe.tf/metallb/internal/config"

	cniconfig "github.com/ligato/cn-infra/config"
	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/db/keyval"
	"github.com/ligato/cn-infra/db/keyval/etcd"
	"github.com/ligato/cn-infra/db/keyval/kvproto"
	cnilogging "github.com/ligato/cn-infra/logging"

	"github.com/ligato/vpp-agent/plugins/restv2/resturl"
	ifvppcalls "github.com/ligato/vpp-agent/plugins/vppv2/ifplugin/vppcalls"
	"github.com/ligato/vpp-agent/plugins/vppv2/model/l3"
)

var (
	etcdConfig string
	agentPort  string
)

const (
	proxyArpTTL            = 2 * time.Minute + 30 * time.Second
	proxyArpUpdateInterval = 1 * time.Minute

	defaultConfigFile = "./etcdv3.conf"
	defaultAgentPort  = "9999"
	agentPrefix       = "/vnf-agent/"
)

func init() {
	flag.StringVar(&etcdConfig, "etcd-config", defaultConfigFile,
		fmt.Sprintf("etcd config file name (default '%s'", defaultConfigFile))
	flag.StringVar(&agentPort, "agent-port", defaultAgentPort,
		fmt.Sprintf("port to access agent's REST API (default %s)", defaultAgentPort))
}

type vppL2Controller struct {
	myNode    string
	bytesConn *etcd.BytesConnectionEtcd
	broker    keyval.ProtoBroker
	ipAddr    string
	ethIfcs   []string

	sync.RWMutex
	ips      map[string]net.IP // svcName -> IP
	ipRefcnt map[string]int    // ip.String() -> number of uses
}

func NewVppL2Controller(l log.Logger, myNode string) *vppL2Controller {
	var err error
	ctl := &vppL2Controller{
		myNode:   myNode,
		ipAddr:   "",
		ips:      make(map[string]net.IP, 0),
		ipRefcnt: make(map[string]int, 0),
	}

	if ctl.bytesConn, ctl.broker, err = ctl.createEtcdClient(etcdConfig); err != nil {
		l.Log("op", "startup", "error", "etcd client init failed", "msg", err)
		os.Exit(1)
	}

	go ctl.periodicLeaseRenewal(l)
	return ctl
}

func (c *vppL2Controller) SetConfig(l log.Logger, cfg *config.Config) error {
	l.Log("config", *cfg)
	return nil
}

func (c *vppL2Controller) ShouldAnnounce(l log.Logger, name string, svc *v1.Service, eps *v1.Endpoints) string {
	l.Log("name", name)
	l2c := &layer2Controller{myNode: c.myNode}
	return l2c.ShouldAnnounce(l, name, svc, eps)
}

func (c *vppL2Controller) SetBalancer(l log.Logger, name string, lbIP net.IP, pool *config.Pool) error {
	l.Log("name", name, "lbIP", lbIP.String())
	c.Lock()
	defer c.Unlock()

	name = strings.Replace(name, "/", "-", -1)

	// Kubernetes may inform us that we should advertise this address multiple
	// times, so just no-op any subsequent requests.
	if _, ok := c.ips[name]; ok {
		return nil
	}

	c.ipRefcnt[lbIP.String()]++
	if c.ipRefcnt[lbIP.String()] > 1 {
		// Multiple services are using this IP, so there's nothing
		// else to do right now.
		return nil
	}

	c.ips[name] = lbIP

	if err := c.updateProxyArp(); err != nil {
		return fmt.Errorf("failed to set proxyarp range %s, error '%s'", name, err)
	}

	return nil
}

func (c *vppL2Controller) DeleteBalancer(l log.Logger, name, reason string) error {
	l.Log("name", name, "reason", reason)
	c.Lock()
	defer c.Unlock()

	name = strings.Replace(name, "/", "-", -1)

	ip, ok := c.ips[name]
	if !ok {
		return nil
	}
	delete(c.ips, name)

	c.ipRefcnt[ip.String()]--
	if c.ipRefcnt[ip.String()] > 0 {
		// Another service is still using this IP, don't touch any
		// more things.
		return nil
	}

	delete(c.ipRefcnt, ip.String())

	if err := c.updateProxyArp(); err != nil {
		l.Log("error", fmt.Sprintf("failed to delete proxyArp range for %s, error %s", name, err))
	}

	return nil
}

func (c *vppL2Controller) SetNode(l log.Logger, n *v1.Node) error {
	// l.Log("node", n.Name)
	if len(n.Status.Addresses) == 0 {
		return nil
	}
	ipAddr := ""
	for _, ip := range n.Status.Addresses {
		if ip.Type == "InternalIP" {
			ipAddr = ip.Address
			break
		}
	}

	if ipAddr == "" {
		return nil
	}

	if (ipAddr == c.ipAddr) && (c.ethIfcs != nil) {
		return nil
	}

	c.ipAddr = ipAddr
	err := c.getEthernetInterfaces()
	l.Log("ipAddr", c.ipAddr, "ethInterfaces", c.ethIfcs)
	return err
}

func (c *vppL2Controller) createEtcdClient(configFile string) (*etcd.BytesConnectionEtcd, keyval.ProtoBroker, error) {
	var err error

	// Override a default config file value if not already overridden by CLI option
	if envCfgFile := os.Getenv("ETCD_CONFIG"); envCfgFile != "" && configFile == defaultConfigFile {
		configFile = envCfgFile
	}

	if configFile == "" {
		return nil, nil,
			fmt.Errorf("missing etcd config file spec: use 'ETCD_CONFIG' flag or '--etcd-config' option")
	}

	cfg := &etcd.Config{}
	if err := cniconfig.ParseConfigFromYamlFile(configFile, cfg); err != nil {
		return nil, nil, err
	}

	etcdConfig, err := etcd.ConfigToClient(cfg)
	if err != nil {
		return nil, nil, err
	}

	bDB, err := etcd.NewEtcdConnectionWithBytes(*etcdConfig, cnilogging.DefaultLogger)
	if err != nil {
		return nil, nil, err
	}

	return bDB, kvproto.NewProtoWrapperWithSerializer(bDB, &keyval.SerializerJSON{}).NewBroker(agentPrefix), nil
}

func (c *vppL2Controller) getEthernetInterfaces() error {
	ifcs := make(map[int]ifvppcalls.InterfaceDetails, 0)
	if err := c.httpRead(resturl.Ethernet, &ifcs); err != nil {
		return err
	}

	ethIfcs := make([]string, 0)
	for _, ifc := range ifcs {
		ethIfcs = append(ethIfcs, ifc.Interface.Name)
	}

	c.ethIfcs = ethIfcs
	return nil
}

// updateProxyArp updates the VPP proxy ARP configuration stored in ETCD.
func (c *vppL2Controller) updateProxyArp() error {
	if len(c.ips) == 0 {
		_, err := c.broker.Delete(c.getProxyArpKey())
		return err
	}

	var ifList []*l3.ProxyARP_Interface
	for _, ifc := range c.ethIfcs {
		ifList = append(ifList, &l3.ProxyARP_Interface{Name: ifc})
	}
	var rangeList []*l3.ProxyARP_Range
	for _, ip := range c.ips {
		rangeList = append(rangeList, &l3.ProxyARP_Range{
			FirstIpAddr: ip.String(),
			LastIpAddr:  ip.String(),
		})
	}
	rl := &l3.ProxyARP{
		Interfaces: ifList,
		Ranges:     rangeList,
	}

	return c.broker.Put(c.getProxyArpKey(), rl, datasync.WithTTL(proxyArpTTL))
}

// periodicLeaseRenewal resets TTL timer for the proxy ARP configuration.
// Until the proper lease renewal is supported by cn-infra, we simply re-Put
// the latest configuration.
func (c *vppL2Controller) periodicLeaseRenewal(l log.Logger) {
	for {
		select {
		case <-time.After(proxyArpUpdateInterval):
			c.Lock()
			if err := c.updateProxyArp(); err != nil {
				l.Log("error", fmt.Sprintf("failed to update proxyArp configuration, error %s", err))
			}
			c.Unlock()
		}
	}
}

func (c *vppL2Controller) httpRead(id string, data interface{}) error {
	client := http.Client{
		Transport:     nil,
		CheckRedirect: nil,
		Jar:           nil,
		Timeout:       10 * time.Second,
	}

	url := fmt.Sprintf("http://%s:%s/%s", c.ipAddr, agentPort, id)
	res, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("failed to get data from '%s', error %s", url, err.Error())
	}

	if res.StatusCode < 200 || res.StatusCode > 299 {
		return fmt.Errorf("failed to get data from '%s', HTTP status %s", url, res.Status)
	}

	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	b = []byte(b)
	err = json.Unmarshal(b, data)
	if err != nil {
		return fmt.Errorf("failed to unmarshal interface data for '%s', error %s", id, err)
	}

	return nil
}

func (c *vppL2Controller) getProxyArpKey() string {
	return fmt.Sprintf("%s/%s", c.myNode, l3.ProxyARPKey)
}
