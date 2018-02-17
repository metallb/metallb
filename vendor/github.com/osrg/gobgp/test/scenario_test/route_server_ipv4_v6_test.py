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


class GoBGPIPv6Test(unittest.TestCase):

    wait_per_retry = 5
    retry_limit = 15

    @classmethod
    def setUpClass(cls):
        gobgp_ctn_image_name = parser_option.gobgp_image
        base.TEST_PREFIX = parser_option.test_prefix

        g1 = GoBGPContainer(name='g1', asn=65002, router_id='192.168.0.2',
                            ctn_image_name=gobgp_ctn_image_name,
                            log_level=parser_option.gobgp_log_level)
        q1 = QuaggaBGPContainer(name='q1', asn=65003, router_id='192.168.0.3')
        q2 = QuaggaBGPContainer(name='q2', asn=65004, router_id='192.168.0.4')
        q3 = QuaggaBGPContainer(name='q3', asn=65005, router_id='192.168.0.5')
        q4 = QuaggaBGPContainer(name='q4', asn=65006, router_id='192.168.0.6')

        ctns = [g1, q1, q2, q3, q4]
        v4 = [q1, q2]
        v6 = [q3, q4]

        for idx, q in enumerate(v4):
            route = '10.0.{0}.0/24'.format(idx + 1)
            q.add_route(route)

        for idx, q in enumerate(v6):
            route = '2001:{0}::/96'.format(idx + 1)
            q.add_route(route, rf='ipv6')

        initial_wait_time = max(ctn.run() for ctn in ctns)

        time.sleep(initial_wait_time)

        for ctn in v4:
            g1.add_peer(ctn, is_rs_client=True)
            ctn.add_peer(g1)

        for ctn in v6:
            g1.add_peer(ctn, is_rs_client=True, v6=True)
            ctn.add_peer(g1, v6=True)

        cls.gobgp = g1
        cls.quaggas = {'q1': q1, 'q2': q2, 'q3': q3, 'q4': q4}
        cls.ipv4s = {'q1': q1, 'q2': q2}
        cls.ipv6s = {'q3': q3, 'q4': q4}

    def check_gobgp_local_rib(self, ctns, rf):
        for rs_client in ctns.itervalues():
            done = False
            for _ in range(self.retry_limit):
                if done:
                    break

                state = self.gobgp.get_neighbor_state(rs_client)
                self.assertEqual(state, BGP_FSM_ESTABLISHED)
                local_rib = self.gobgp.get_local_rib(rs_client, rf=rf)
                local_rib = [p['prefix'] for p in local_rib]
                if len(local_rib) < (len(ctns) - 1):
                    time.sleep(self.wait_per_retry)
                    continue

                self.assertTrue(len(local_rib) == (len(ctns) - 1))

                for c in ctns.itervalues():
                    if rs_client != c:
                        for r in c.routes:
                            self.assertTrue(r in local_rib)

                done = True
            if done:
                continue
            # should not reach here
            raise AssertionError

    def check_rs_client_rib(self, ctns, rf):
        for rs_client in ctns.itervalues():
            done = False
            for _ in range(self.retry_limit):
                if done:
                    break
                global_rib = rs_client.get_global_rib(rf=rf)
                global_rib = [p['prefix'] for p in global_rib]
                if len(global_rib) < len(ctns):
                    time.sleep(self.wait_per_retry)
                    continue

                self.assertTrue(len(global_rib) == len(ctns))

                for c in ctns.itervalues():
                    for r in c.routes:
                        self.assertTrue(r in global_rib)

                done = True
            if done:
                continue
            # should not reach here
            raise AssertionError

    # test each neighbor state is turned establish
    def test_01_neighbor_established(self):
        for q in self.quaggas.itervalues():
            self.gobgp.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=q)

    def test_02_check_ipv4_peer_rib(self):
        self.check_gobgp_local_rib(self.ipv4s, 'ipv4')
        self.check_rs_client_rib(self.ipv4s, 'ipv4')

    def test_03_check_ipv6_peer_rib(self):
        self.check_gobgp_local_rib(self.ipv6s, 'ipv6')
        self.check_rs_client_rib(self.ipv6s, 'ipv6')

    def test_04_add_in_policy_to_reject_all(self):
        for q in self.gobgp.peers.itervalues():
            self.gobgp.local('gobgp neighbor {0} policy in set default reject'.format(q['neigh_addr'].split('/')[0]))

    def test_05_check_ipv4_peer_rib(self):
        self.check_gobgp_local_rib(self.ipv4s, 'ipv4')
        self.check_rs_client_rib(self.ipv4s, 'ipv4')

    def test_06_check_ipv6_peer_rib(self):
        self.check_gobgp_local_rib(self.ipv6s, 'ipv6')
        self.check_rs_client_rib(self.ipv6s, 'ipv6')

    def test_07_add_in_policy_to_reject_all(self):
        self.gobgp.local('gobgp neighbor all softresetin')
        time.sleep(1)

    def test_08_check_rib(self):
        for q in self.ipv4s.itervalues():
            self.assertTrue(all(p['filtered'] for p in self.gobgp.get_adj_rib_in(q)))
            self.assertTrue(len(self.gobgp.get_adj_rib_out(q)) == 0)
            self.assertTrue(len(q.get_global_rib()) == len(q.routes))

        for q in self.ipv6s.itervalues():
            self.assertTrue(all(p['filtered'] for p in self.gobgp.get_adj_rib_in(q, rf='ipv6')))
            self.assertTrue(len(self.gobgp.get_adj_rib_out(q, rf='ipv6')) == 0)
            self.assertTrue(len(q.get_global_rib(rf='ipv6')) == len(q.routes))


if __name__ == '__main__':
    output = local("which docker 2>&1 > /dev/null ; echo $?", capture=True)
    if int(output) is not 0:
        print "docker not found"
        sys.exit(1)

    nose.main(argv=sys.argv, addplugins=[OptionParser()],
              defaultTest=sys.argv[0])
