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

from itertools import combinations
import sys
import time
import unittest

from fabric.api import local
import nose

from lib.noseplugin import OptionParser, parser_option

from lib import base
from lib.base import (
    BGP_FSM_IDLE,
    BGP_FSM_ESTABLISHED,
)
from lib.base import wait_for_completion
from lib.gobgp import GoBGPContainer
from lib.quagga import QuaggaBGPContainer


class GoBGPTestBase(unittest.TestCase):

    @classmethod
    def setUpClass(cls):
        gobgp_ctn_image_name = parser_option.gobgp_image
        base.TEST_PREFIX = parser_option.test_prefix

        g1 = GoBGPContainer(name='g1', asn=65000, router_id='192.168.0.1',
                            ctn_image_name=gobgp_ctn_image_name,
                            log_level=parser_option.gobgp_log_level)
        q1 = QuaggaBGPContainer(name='q1', asn=65000, router_id='192.168.0.2')
        q2 = QuaggaBGPContainer(name='q2', asn=65000, router_id='192.168.0.3')

        qs = [q1, q2]
        ctns = [g1, q1, q2]

        # advertise a route from q1, q2
        for idx, c in enumerate(qs):
            route = '10.0.{0}.0/24'.format(idx + 1)
            c.add_route(route)

        initial_wait_time = max(ctn.run() for ctn in ctns)

        time.sleep(initial_wait_time)

        # ibgp peer. loop topology
        for a, b in combinations(ctns, 2):
            a.add_peer(b)
            b.add_peer(a)

        cls.gobgp = g1
        cls.quaggas = {'q1': q1, 'q2': q2}

    # test each neighbor state is turned establish
    def test_01_neighbor_established(self):
        for q in self.quaggas.itervalues():
            self.gobgp.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=q)

    def test_02_check_gobgp_global_rib(self):
        for q in self.quaggas.itervalues():
            # paths expected to exist in gobgp's global rib
            routes = q.routes.keys()
            timeout = 120
            interval = 1
            count = 0
            while True:
                # gobgp's global rib
                state = self.gobgp.get_neighbor_state(q)
                self.assertEqual(state, BGP_FSM_ESTABLISHED)
                global_rib = [p['prefix'] for p in self.gobgp.get_global_rib()]

                for p in global_rib:
                    if p in routes:
                        routes.remove(p)

                if len(routes) == 0:
                    break

                time.sleep(interval)
                count += interval
                if count >= timeout:
                    raise Exception('timeout')

    def test_03_check_gobgp_adj_rib_out(self):
        for q in self.quaggas.itervalues():
            paths = self.gobgp.get_adj_rib_out(q)
            # bgp speaker mustn't forward iBGP routes to iBGP peers
            self.assertTrue(len(paths) == 0)

    def test_04_originate_path(self):
        self.gobgp.add_route('10.10.0.0/24')
        dst = self.gobgp.get_global_rib('10.10.0.0/24')
        self.assertTrue(len(dst) == 1)
        self.assertTrue(len(dst[0]['paths']) == 1)
        path = dst[0]['paths'][0]
        self.assertTrue(path['nexthop'] == '0.0.0.0')
        self.assertTrue(len(path['aspath']) == 0)

    def test_05_check_gobgp_adj_rib_out(self):
        for q in self.quaggas.itervalues():
            paths = self.gobgp.get_adj_rib_out(q)
            self.assertTrue(len(paths) == len(self.gobgp.routes))
            path = paths[0]
            self.assertTrue(path['nlri']['prefix'] == '10.10.0.0/24')
            peer_info = self.gobgp.peers[q]
            local_addr = peer_info['local_addr'].split('/')[0]
            self.assertTrue(path['nexthop'] == local_addr)
            self.assertTrue(len(path['aspath']) == 0)

    # check routes are properly advertised to all BGP speaker
    def test_06_check_quagga_global_rib(self):
        interval = 1
        timeout = int(120 / interval)
        for q in self.quaggas.itervalues():
            done = False
            for _ in range(timeout):
                if done:
                    break
                global_rib = q.get_global_rib()
                # quagga's global_rib must have two routes at least,
                # a self-generated route and a gobgp-generated route
                if len(global_rib) < len(q.routes) + len(self.gobgp.routes):
                    time.sleep(interval)
                    continue

                peer_info = self.gobgp.peers[q]
                local_addr = peer_info['local_addr'].split('/')[0]
                for r in self.gobgp.routes:
                    self.assertTrue(r in (p['prefix'] for p in global_rib))
                    for rr in global_rib:
                        if rr['prefix'] == r:
                            self.assertTrue(rr['nexthop'] == local_addr)

                for r in q.routes.keys():
                    self.assertTrue(r in (p['prefix'] for p in global_rib))
                    for rr in global_rib:
                        if rr['prefix'] == r:
                            self.assertTrue(rr['nexthop'] == '0.0.0.0')

                done = True
            if done:
                continue
            # should not reach here
            raise AssertionError

    def test_07_add_ebgp_peer(self):
        q3 = QuaggaBGPContainer(name='q3', asn=65001, router_id='192.168.0.4')
        self.quaggas['q3'] = q3

        q3.add_route('10.0.3.0/24')

        initial_wait_time = q3.run()
        time.sleep(initial_wait_time)
        self.gobgp.add_peer(q3)
        q3.add_peer(self.gobgp)

        self.gobgp.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=q3)

    def test_08_check_global_rib(self):
        self.test_02_check_gobgp_global_rib()

    def test_09_check_gobgp_ebgp_adj_rib_out(self):
        q1 = self.quaggas['q1']
        q2 = self.quaggas['q2']
        q3 = self.quaggas['q3']
        paths = self.gobgp.get_adj_rib_out(q3)
        total_len = len(q1.routes) + len(q2.routes) + len(self.gobgp.routes)
        assert len(paths) == total_len
        for path in paths:
            peer_info = self.gobgp.peers[q3]
            local_addr = peer_info['local_addr'].split('/')[0]
            self.assertTrue(path['nexthop'] == local_addr)
            self.assertTrue(path['aspath'] == [self.gobgp.asn])

    def test_10_check_gobgp_ibgp_adj_rib_out(self):
        q1 = self.quaggas['q1']
        q3 = self.quaggas['q3']
        peer_info = self.gobgp.peers[q3]
        neigh_addr = peer_info['neigh_addr'].split('/')[0]

        for prefix in q3.routes.iterkeys():
            paths = self.gobgp.get_adj_rib_out(q1, prefix)
            self.assertTrue(len(paths) == 1)
            path = paths[0]
            # bgp router mustn't change nexthop of routes from eBGP peers
            # which are sent to iBGP peers
            self.assertTrue(path['nexthop'] == neigh_addr)
            # bgp router mustn't change aspath of routes from eBGP peers
            # which are sent to iBGP peers
            self.assertTrue(path['aspath'] == [q3.asn])

    # disable ebgp peer, check ebgp routes are removed
    def test_11_disable_ebgp_peer(self):
        q3 = self.quaggas['q3']
        self.gobgp.disable_peer(q3)
        del self.quaggas['q3']
        self.gobgp.wait_for(expected_state=BGP_FSM_IDLE, peer=q3)

        for route in q3.routes.iterkeys():
            dst = self.gobgp.get_global_rib(route)
            self.assertTrue(len(dst) == 0)

        for q in self.quaggas.itervalues():
            paths = self.gobgp.get_adj_rib_out(q)
            # only gobgp's locally generated routes must exists
            print paths
            self.assertTrue(len(paths) == len(self.gobgp.routes))

    def test_12_disable_ibgp_peer(self):
        q1 = self.quaggas['q1']
        self.gobgp.disable_peer(q1)
        self.gobgp.wait_for(expected_state=BGP_FSM_IDLE, peer=q1)

        for route in q1.routes.iterkeys():
            dst = self.gobgp.get_global_rib(route)
            self.assertTrue(len(dst) == 0)

    def test_13_enable_ibgp_peer(self):
        q1 = self.quaggas['q1']
        self.gobgp.enable_peer(q1)
        self.gobgp.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=q1)

    def test_14_check_gobgp_adj_rib_out(self):
        for q in self.quaggas.itervalues():
            paths = self.gobgp.get_adj_rib_out(q)
            # only gobgp's locally generated routes must exists
            self.assertTrue(len(paths) == len(self.gobgp.routes))

    def test_15_add_ebgp_peer(self):
        q4 = QuaggaBGPContainer(name='q4', asn=65001, router_id='192.168.0.5')
        self.quaggas['q4'] = q4

        prefix = '10.0.4.0/24'
        q4.add_route(prefix)

        initial_wait_time = q4.run()
        time.sleep(initial_wait_time)
        self.gobgp.add_peer(q4)
        q4.add_peer(self.gobgp)

        self.gobgp.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=q4)

        q1 = self.quaggas['q1']
        q2 = self.quaggas['q2']
        for q in [q1, q2]:
            def _f():
                return prefix in [p['nlri']['prefix'] for p in self.gobgp.get_adj_rib_out(q)]
            wait_for_completion(_f)

        def f():
            return len(q2.get_global_rib(prefix)) == 1
        wait_for_completion(f)

    def test_16_add_best_path_from_ibgp(self):
        q1 = self.quaggas['q1']
        q2 = self.quaggas['q2']

        prefix = '10.0.4.0/24'
        q1.add_route(prefix)

        def f1():
            l = self.gobgp.get_global_rib(prefix)
            return len(l) == 1 and len(l[0]['paths']) == 2
        wait_for_completion(f1)

        def f2():
            return prefix not in [p['nlri']['prefix'] for p in self.gobgp.get_adj_rib_out(q2)]
        wait_for_completion(f2)

        def f3():
            l = q2.get_global_rib(prefix)
            # route from ibgp so aspath should be empty
            return len(l) == 1 and len(l[0]['aspath']) == 0
        wait_for_completion(f3)


if __name__ == '__main__':
    output = local("which docker 2>&1 > /dev/null ; echo $?", capture=True)
    if int(output) is not 0:
        print "docker not found"
        sys.exit(1)

    nose.main(argv=sys.argv, addplugins=[OptionParser()],
              defaultTest=sys.argv[0])
