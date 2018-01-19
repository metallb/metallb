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
	"io"
	"net"
	"sort"
	"time"

	"go.universe.tf/metallb/internal/bgp"
	"go.universe.tf/metallb/internal/config"

	"github.com/golang/glog"
)

type peer struct {
	cfg *config.Peer
	bgp session
}

func (c *controller) SetBalancerBGP(name string, lbIP net.IP, pool *config.Pool) error {
	c.bgpSvcAds[name] = nil
	for _, adCfg := range pool.BGPAdvertisements {
		m := net.CIDRMask(adCfg.AggregationLength, 32)
		ad := &bgp.Advertisement{
			Prefix: &net.IPNet{
				IP:   lbIP.Mask(m),
				Mask: m,
			},
			NextHop:   c.myIP,
			LocalPref: adCfg.LocalPref,
		}
		for comm := range adCfg.Communities {
			ad.Communities = append(ad.Communities, comm)
		}
		sort.Slice(ad.Communities, func(i, j int) bool { return ad.Communities[i] < ad.Communities[j] })
		c.bgpSvcAds[name] = append(c.bgpSvcAds[name], ad)
	}

	if err := c.updateAdsBGP(); err != nil {
		return err
	}

	glog.Infof("%s: making %d advertisements using BGP", name, len(c.bgpSvcAds[name]))

	return nil
}

func (c *controller) updateAdsBGP() error {
	var allAds []*bgp.Advertisement
	for _, ads := range c.bgpSvcAds {
		// This list might contain duplicates, but that's fine,
		// they'll get compacted by the session code when it's
		// calculating advertisements.
		//
		// TODO: be more intelligent about compacting advertisements
		// and detecting conflicting advertisements.
		allAds = append(allAds, ads...)
	}
	for _, peer := range c.bgpPeers {
		if err := peer.bgp.Set(allAds...); err != nil {
			return err
		}
	}
	return nil
}

func (c *controller) deleteBalancerBGP(name, reason string) error {
	if _, ok := c.bgpSvcAds[name]; !ok {
		return nil
	}
	glog.Infof("%s: stopping announcements, %s", name, reason)
	delete(c.bgpSvcAds, name)
	return c.updateAdsBGP()
}

type session interface {
	io.Closer
	Set(advs ...*bgp.Advertisement) error
}

var newBGP = func(addr string, myASN uint32, routerID net.IP, asn uint32, hold time.Duration) (session, error) {
	return bgp.New(addr, myASN, routerID, asn, hold)
}
