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

	"go.universe.tf/metallb/internal/arp"
	"go.universe.tf/metallb/internal/config"
	"k8s.io/api/core/v1"
)

type arpController struct {
	announcer *arp.Announce
}

func (c *arpController) SetConfig(*config.Config) error {
	return nil
}

func (c *arpController) SetBalancer(name string, lbIP net.IP, pool *config.Pool) error {
	c.announcer.SetBalancer(name, lbIP)
	return nil
}

func (c *arpController) DeleteBalancer(name, reason string) error {
	if !c.announcer.AnnounceName(name) {
		return nil
	}
	c.announcer.DeleteBalancer(name)
	return nil
}

func (c *arpController) SetLeader(isLeader bool) {
	c.announcer.SetLeader(isLeader)
}

func (c *arpController) SetNode(*v1.Node) error {
	return nil
}
