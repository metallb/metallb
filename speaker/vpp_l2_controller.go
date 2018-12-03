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
	"github.com/go-kit/kit/log"
	cniconfig "github.com/ligato/cn-infra/config"
	"github.com/ligato/cn-infra/db/keyval"
	"github.com/ligato/cn-infra/db/keyval/etcd"
	"github.com/ligato/cn-infra/db/keyval/kvproto"
	cnilogging "github.com/ligato/cn-infra/logging"
	"github.com/ligato/vpp-agent/plugins/rest/resturl"
	ifvppcalls "github.com/ligato/vpp-agent/plugins/vpp/ifplugin/vppcalls"
	l3vppcalls "github.com/ligato/vpp-agent/plugins/vpp/l3plugin/vppcalls"
	"github.com/ligato/vpp-agent/plugins/vpp/model/l3"
	"strings"
	"sync"

	"go.universe.tf/metallb/internal/config"
	"io/ioutil"
	"k8s.io/api/core/v1"
	"net"
	"net/http"
	"os"
	"time"
)

var (
	etcdConfig string
	agentPort  string
)

const (
	defaultConfigFile = "./etcdv3.conf"
	defaultAgentPort  = "9999"
	agentPrefix       = "/vnf-agent/"
	mlbPrefix         = "mlb"
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
	inSync   bool
}

func NewVppL2Controller(l log.Logger, myNode string) *vppL2Controller {
	var err error
	ctl := &vppL2Controller{
		myNode:   myNode,
		ipAddr:   "",
		ips:      make(map[string]net.IP, 0),
		ipRefcnt: make(map[string]int, 0),
		inSync:   true,
	}

	if ctl.bytesConn, ctl.broker, err = ctl.createEtcdClient(etcdConfig); err != nil {
		l.Log("op", "startup", "error", "etcd client init failed", "msg", err)
		os.Exit(1)
	}
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

	if err := c.putProxyArpRange(name, lbIP.String()); err != nil {
		c.inSync = false
		return fmt.Errorf("failed to set proxyarp range %s, error '%s'", name, err)
	}

	if err := c.putProxyArpInterfaces(name); err != nil {
		c.inSync = false
		return fmt.Errorf("failed to set proxyarp interfaces for range %s, error '%s'", name, err)
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

	rngKey := c.getRngKeyForName(name)
	existed, err := c.broker.Delete(rngKey)
	if err != nil {
		l.Log("error", fmt.Sprintf("failed to delete proxyArp range for %s, error %s", name, err))
		c.inSync = false
	}
	if existed {
		l.Log("msg", fmt.Sprintf("proxyArp range not found for name %s", name))
	}

	ifcKey := c.getIfcKeyForName(name)
	existed, err = c.broker.Delete(ifcKey)
	if err != nil {
		l.Log("error", fmt.Sprintf("failed to delete proxyArp interface for %s, error %s", name, err))
		c.inSync = false
	}
	if existed {
		l.Log("msg", fmt.Sprintf("proxyArp interface not found for name %s", name))
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

// putProxyArpRange stores the VPP proxy ARP range configuration to ETCD
func (c *vppL2Controller) putProxyArpRange(name string, ip string) error {
	rl := &l3.ProxyArpRanges_RangeList{
		Label: name,
		Ranges: []*l3.ProxyArpRanges_RangeList_Range{
			{
				FirstIp: ip,
				LastIp:  ip,
			},
		},
	}

	key := c.getRngKeyForName(rl.Label)
	return c.broker.Put(key, rl)
}

func (c *vppL2Controller) putProxyArpInterfaces(name string) error {
	ifList := make([]*l3.ProxyArpInterfaces_InterfaceList_Interface, 0)
	for _, ifc := range c.ethIfcs {
		paIfc := &l3.ProxyArpInterfaces_InterfaceList_Interface{Name: ifc}
		ifList = append(ifList, paIfc)
	}

	ifl := &l3.ProxyArpInterfaces_InterfaceList{
		Label:      name,
		Interfaces: ifList,
	}

	key := c.getIfcKeyForName(name)
	return c.broker.Put(key, ifl)
}

func (c *vppL2Controller) etcdMarkAndSweep(l log.Logger) {
	c.Lock()
	defer c.Unlock()

	c.inSync = true

	c.resyncProxyArpRanges(l)
	c.resyncProxyArpInterfaces(l)

	paRngs := make([]*l3vppcalls.ProxyArpRangesDetails, 0)
	if err := c.httpRead(resturl.PArpRngs, &paRngs); err != nil {
		l.Log("failed to  get proxyarp range data, err", err)
		return
	}

	paIfcs := make([]*l3vppcalls.ProxyArpInterfaceDetails, 0)
	if err := c.httpRead(resturl.PArpRngs, &paIfcs); err != nil {
		l.Log("failed to  get proxyarp interface data, err", err)
		return
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

func (c *vppL2Controller) resyncProxyArpRanges(l log.Logger) {
	rngKey := fmt.Sprintf("%s/%s", c.myNode, l3.ProxyARPRangeKey)
	rngIter, err := c.broker.ListValues(rngKey)
	if err != nil {
		l.Log("failed to get etcd data", l3.ProxyARPRangeKey)
		return
	}

	rngVals := make(map[string]*l3.ProxyArpRanges_RangeList)
	for {
		kv, stop := rngIter.GetNext()
		if stop {
			break
		}

		rl := &l3.ProxyArpRanges_RangeList{}
		kv.GetValue(rl)
		rngVals[kv.GetKey()] = rl
	}

	for name, ip := range c.ips {
		rngKey := c.getRngKeyForName(name)
		if rng, ok := rngVals[rngKey]; ok {
			delete(rngVals, rngKey)

			if (len(rng.Ranges) == 1) &&
				(ip.String() == rng.Ranges[0].FirstIp) &&
				(ip.String() == rng.Ranges[0].LastIp) {
				continue
			}
		}

		// Try to fix
		if err := c.putProxyArpRange(name, ip.String()); err != nil {
			l.Log("re-sync putProxyArpRange failed", err, "name", name, "ip", ip.String())
			c.inSync = false
		}
	}
	// delete leftover rngVals from etcd
	for rngKey := range rngVals {
		if _, err := c.broker.Delete(rngKey); err != nil {
			l.Log("re-sync delete %s failed", err, "key", rngKey)
			c.inSync = false
		}
	}
}

func (c *vppL2Controller) resyncProxyArpInterfaces(l log.Logger) {
	ifcKey := fmt.Sprintf("%s/%s", c.myNode, l3.ProxyARPInterfaceKey)
	ifcIter, err := c.broker.ListValues(ifcKey)
	if err != nil {
		l.Log("failed to get etcd data", l3.ProxyARPInterfaceKey)
		return
	}

	ifcVals := make(map[string]*l3.ProxyArpInterfaces_InterfaceList)
	for {
		kv, stop := ifcIter.GetNext()
		if stop {
			break
		}

		ifl := &l3.ProxyArpInterfaces_InterfaceList{}
		kv.GetValue(ifl)
		ifcVals[kv.GetKey()] = ifl
	}

	for name := range c.ips {
		rngKey := c.getIfcKeyForName(name)
		if _, ok := ifcVals[rngKey]; ok {
			delete(ifcVals, rngKey)

			// validate interfaces
		}
		// Try to fix
	}
	// delete leftover ifcVals from etcd
	for rngKey := range ifcVals {
		if _, err := c.broker.Delete(rngKey); err != nil {
			l.Log("re-sync delete %s failed", err, "key", rngKey)
			c.inSync = false
		}
	}
}

func (c *vppL2Controller) getRngKeyForName(name string) string {
	return fmt.Sprintf("%s/%s", c.myNode,
		l3.ProxyArpRangeKey(fmt.Sprintf("%s-%s", mlbPrefix, name)))
}

func (c *vppL2Controller) getIfcKeyForName(name string) string {
	return fmt.Sprintf("%s/%s", c.myNode,
		l3.ProxyArpInterfaceKey(fmt.Sprintf("%s-%s", mlbPrefix, name)))
}
