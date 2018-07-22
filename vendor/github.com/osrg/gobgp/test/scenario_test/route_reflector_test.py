# Copyright (C) 2015 Nippon Telegraph and Telephone Corporation.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
# implied.
# See the License for the specific language governing permissions and
# limitations under the License.

from __future__ import absolute_import

import sys
import time
import unittest

from fabric.api import local
import nose

from lib.noseplugin import OptionParser, parser_option

from lib import base
from lib.base import BGP_FSM_ESTABLISHED
from lib.gobgp import GoBGPContainer
from lib.quagga import QuaggaBGPContainer


def wait_for(f, timeout=120):
    interval = 1
    count = 0
    while True:
        if f():
            return

        time.sleep(interval)
        count += interval
        if count >= timeout:
            raise Exception('timeout')


class GoBGPTestBase(unittest.TestCase):
    def assert_adv_count(self, src, dst, rf, count):
        self.assertEqual(count, len(src.get_adj_rib_out(dst, rf=rf)))
        self.assertEqual(count, len(dst.get_adj_rib_in(src, rf=rf)))

    def assert_upd_count(self, src, dst, sent, received):
        messages = src.get_neighbor(dst)['state']['messages']
        self.assertEqual(messages['sent'].get('update', 0), sent)
        self.assertEqual(messages['received'].get('update', 0), received)

    @classmethod
    def setUpClass(cls):
        gobgp_ctn_image_name = parser_option.gobgp_image
        base.TEST_PREFIX = parser_option.test_prefix

        g1 = GoBGPContainer(name='g1', asn=65000, router_id='192.168.0.1',
                            ctn_image_name=gobgp_ctn_image_name,
                            log_level=parser_option.gobgp_log_level)
        q1 = QuaggaBGPContainer(name='q1', asn=65000, router_id='192.168.0.2')
        q2 = QuaggaBGPContainer(name='q2', asn=65000, router_id='192.168.0.3')
        q3 = QuaggaBGPContainer(name='q3', asn=65000, router_id='192.168.0.4')
        q4 = QuaggaBGPContainer(name='q4', asn=65000, router_id='192.168.0.5')

        qs = [q1, q2, q3, q4]
        ctns = [g1, q1, q2, q3, q4]

        initial_wait_time = max(ctn.run() for ctn in ctns)
        time.sleep(initial_wait_time)

        # g1 as a route reflector
        g1.add_peer(q1, is_rr_client=True)
        q1.add_peer(g1)
        g1.add_peer(q2, is_rr_client=True)
        q2.add_peer(g1)
        g1.add_peer(q3)
        q3.add_peer(g1)
        g1.add_peer(q4)
        q4.add_peer(g1)

        # advertise a route from q1, q2
        for idx, c in enumerate(qs):
            route = '10.0.{0}.0/24'.format(idx + 1)
            c.add_route(route)

        cls.gobgp = g1
        cls.quaggas = {'q1': q1, 'q2': q2, 'q3': q3, 'q4': q4}

    # test each neighbor state is turned establish
    def test_01_neighbor_established(self):
        for q in self.quaggas.itervalues():
            self.gobgp.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=q)

    def test_02_check_gobgp_global_rib(self):
        for q in self.quaggas.itervalues():
            # paths expected to exist in gobgp's global rib
            def f():
                state = self.gobgp.get_neighbor_state(q)
                self.assertEqual(state, BGP_FSM_ESTABLISHED)

                routes = q.routes.keys()
                global_rib = [p['prefix'] for p in self.gobgp.get_global_rib()]
                for p in global_rib:
                    if p in routes:
                        routes.remove(p)

                return len(routes) == 0
            wait_for(f)

    def test_03_check_gobgp_adj_rib_out(self):
        for q in self.quaggas.itervalues():
            paths = [p['nlri']['prefix'] for p in self.gobgp.get_adj_rib_out(q)]
            for qq in self.quaggas.itervalues():
                if q == qq:
                    continue
                if self.gobgp.peers[q]['is_rr_client']:
                    for p in qq.routes.keys():
                        self.assertTrue(p in paths)
                else:
                    for p in qq.routes.keys():
                        if self.gobgp.peers[qq]['is_rr_client']:
                            self.assertTrue(p in paths)
                        else:
                            self.assertFalse(p in paths)

    def test_10_setup_rr_rtc_isolation_policy(self):
        #                              +-------+
        #                              |  rr   |
        #        +----------------+----| (RR)  |---+----------------+
        #        |                |    +-------+   |                |
        #        |                |                |                |
        #      (iBGP)           (iBGP)           (iBGP)          (iBGP)
        #        |                |                |                |
        # +-------------+  +-------------+  +-------------+  +-------------+
        # |     acme1   |  |    acme2    |  |   tyrell1   |  |   tyrell2   |
        # | (RR Client) |  | (RR Client) |  | (RR Client) |  | (RR Client) |
        # +-------------+  +-------------+  +-------------+  +-------------+


        gobgp_ctn_image_name = parser_option.gobgp_image
        rr = GoBGPContainer(name='rr', asn=65000, router_id='192.168.1.1',
                            ctn_image_name=gobgp_ctn_image_name,
                            log_level=parser_option.gobgp_log_level)
        acme1 = GoBGPContainer(name='acme1', asn=65000, router_id='192.168.1.101',
                               ctn_image_name=gobgp_ctn_image_name,
                               log_level=parser_option.gobgp_log_level)
        acme2 = GoBGPContainer(name='acme2', asn=65000, router_id='192.168.1.102',
                               ctn_image_name=gobgp_ctn_image_name,
                               log_level=parser_option.gobgp_log_level)

        tyrell1 = GoBGPContainer(name='tyrell1', asn=65000, router_id='192.168.1.201',
                                 ctn_image_name=gobgp_ctn_image_name,
                                 log_level=parser_option.gobgp_log_level)

        tyrell2 = GoBGPContainer(name='tyrell2', asn=65000, router_id='192.168.1.202',
                                 ctn_image_name=gobgp_ctn_image_name,
                                 log_level=parser_option.gobgp_log_level)

        time.sleep(max(ctn.run() for ctn in [rr, acme1, acme2, tyrell1, tyrell2]))

        rr.add_peer(acme1, vpn=True, addpath=True, graceful_restart=True, llgr=True, is_rr_client=True)
        acme1.add_peer(rr, vpn=True, addpath=True, graceful_restart=True, llgr=True)

        rr.add_peer(acme2, vpn=True, addpath=True, graceful_restart=True, llgr=True, is_rr_client=True)
        acme2.add_peer(rr, vpn=True, addpath=True, graceful_restart=True, llgr=True)

        rr.add_peer(tyrell1, vpn=True, addpath=True, graceful_restart=True, llgr=True, is_rr_client=True)
        tyrell1.add_peer(rr, vpn=True, addpath=True, graceful_restart=True, llgr=True)

        rr.add_peer(tyrell2, vpn=True, addpath=True, graceful_restart=True, llgr=True, is_rr_client=True)
        tyrell2.add_peer(rr, vpn=True, addpath=True, graceful_restart=True, llgr=True)

        self.__class__.rr = rr
        self.__class__.acme1 = acme1
        self.__class__.acme2 = acme2
        self.__class__.tyrell1 = tyrell1
        self.__class__.tyrell2 = tyrell2

        # add import/export policy to allow peers exchange routes within specific RTs
        # later tests should not break due to RTC Updates being filtered-out

        rr.local("gobgp policy neighbor add clients-acme {} {}".format(
            rr.peer_name(acme1),
            rr.peer_name(acme2)))

        rr.local("gobgp policy neighbor add clients-tyrell {} {}".format(
            rr.peer_name(tyrell1),
            rr.peer_name(tyrell2)))

        rr.local("gobgp policy ext-community add rts-acme   rt:^100:.*$")
        rr.local("gobgp policy ext-community add rts-tyrell rt:^200:.*$")

        rr.local('gobgp policy statement add allow-rtc')
        rr.local('gobgp policy statement allow-rtc add condition afi-safi-in rtc')
        rr.local('gobgp policy statement allow-rtc add action accept')

        rr.local('gobgp policy statement add allow-acme')
        rr.local('gobgp policy statement allow-acme add condition neighbor clients-acme')
        rr.local('gobgp policy statement allow-acme add condition ext-community rts-acme')
        rr.local('gobgp policy statement allow-acme add action accept')

        rr.local('gobgp policy statement add allow-tyrell')
        rr.local('gobgp policy statement allow-tyrell add condition neighbor clients-tyrell')
        rr.local('gobgp policy statement allow-tyrell add condition ext-community rts-tyrell')
        rr.local('gobgp policy statement allow-tyrell add action accept')
        rr.local('gobgp policy add tenancy allow-rtc allow-acme allow-tyrell')

        rr.local('gobgp global policy import add tenancy default reject')
        rr.local('gobgp global policy export add tenancy default reject')

        acme1.local("gobgp vrf add a1 rd 100:100 rt both 100:100")
        acme2.local("gobgp vrf add a1 rd 100:100 rt both 100:100")

        tyrell1.local("gobgp vrf add t1 rd 200:100 rt both 200:100")
        tyrell2.local("gobgp vrf add t1 rd 200:100 rt both 200:100")

        rr.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=acme1)
        rr.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=acme2)
        rr.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=tyrell1)
        rr.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=tyrell2)

    def test_11_routes_in_allowed_acme_rts_are_exchanged(self):
        self.acme1.local("gobgp vrf a1 rib add 10.10.0.0/16 local-pref 100")
        self.acme2.local("gobgp vrf a1 rib add 10.20.0.0/16")
        self.tyrell1.local("gobgp vrf t1 rib add 20.10.0.0/16")
        self.tyrell2.local("gobgp vrf t1 rib add 20.20.0.0/16")
        time.sleep(1)

        self.assert_adv_count(self.rr, self.acme1, 'rtc', 2)
        self.assert_adv_count(self.rr, self.acme1, 'ipv4-l3vpn', 1)
        self.assert_adv_count(self.rr, self.acme2, 'rtc', 2)
        self.assert_adv_count(self.rr, self.acme2, 'ipv4-l3vpn', 1)
        self.assert_adv_count(self.rr, self.tyrell1, 'rtc', 2)
        self.assert_adv_count(self.rr, self.tyrell1, 'ipv4-l3vpn', 1)
        self.assert_adv_count(self.rr, self.tyrell2, 'rtc', 2)
        self.assert_adv_count(self.rr, self.tyrell2, 'ipv4-l3vpn', 1)

    def test_12_routes_from_separate_rts_peers_are_isolated_by_rr(self):
        self.tyrell1.local("gobgp vrf add a1 rd 100:100 rt both 100:100")
        self.tyrell1.local("gobgp vrf a1 rib add 10.10.0.0/16 local-pref 200")
        self.tyrell1.local("gobgp vrf a1 rib add 10.30.0.0/16")
        time.sleep(1)

        rr_t2_in = self.rr.get_adj_rib_in(self.tyrell1, rf='ipv4-l3vpn')
        self.assertEqual(3, len(rr_t2_in))

        rr_a2_out = self.rr.get_adj_rib_out(self.acme2, rf='ipv4-l3vpn')
        self.assertEqual(1, len(rr_a2_out))

        a2_routes = self.acme2.get_adj_rib_in(self.rr, rf='ipv4-l3vpn')
        self.assertEqual(1, len(a2_routes))
        ar0 = a2_routes[0]
        self.assertEqual('10.10.0.0/16', ar0['prefix'])
        self.assertEqual(self.rr.peer_name(self.acme1), ar0['nexthop'])
        self.assertEqual(100, ar0['local-pref'])


if __name__ == '__main__':
    output = local("which docker 2>&1 > /dev/null ; echo $?", capture=True)
    if int(output) is not 0:
        print "docker not found"
        sys.exit(1)

    nose.main(argv=sys.argv, addplugins=[OptionParser()],
              defaultTest=sys.argv[0])
