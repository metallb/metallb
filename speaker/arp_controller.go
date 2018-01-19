// Copyright 2017 Google Inc.
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
	"net"

	"go.universe.tf/metallb/internal/config"

	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
)

func (c *controller) SetBalancerARP(name string, lbIP net.IP, pool *config.Pool) error {
	glog.Infof("%s: making 1 advertisement using ARP", name)
	c.arpAnn.SetBalancer(name, lbIP)
	return nil
}

func (c *controller) deleteBalancerARP(name, reason string) error {
	if !c.arpAnn.AnnounceName(name) {
		return nil
	}

	glog.Infof("%s: stopping announcements, %s", name, reason)
	announcing.Delete(prometheus.Labels{
		"protocol": string(config.ARP),
		"service":  name,
		"node":     c.myNode,
		"ip":       c.ips.IP(name).String(),
	})
	c.ips.Unassign(name)
	c.arpAnn.DeleteBalancer(name)
	return nil
}
