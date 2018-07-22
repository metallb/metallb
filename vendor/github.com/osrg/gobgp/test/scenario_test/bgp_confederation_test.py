# Copyright (C) 2017 Nippon Telegraph and Telephone Corporation.
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
from __future__ import print_function

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


class GoBGPTestBase(unittest.TestCase):

    def _check_global_rib_first(self, q, prefix, aspath):
        route = q.get_global_rib(prefix)[0]
        self.assertListEqual(aspath, route['aspath'])

    @classmethod
    def setUpClass(cls):
        #                  +-----Confederation(AS30)-----+
        #  AS21     AS20   | +-AS65002-+     +-AS65001-+ |   AS10
        # +----+   +----+  | | +-----+ |     | +-----+ | |  +----+
        # | q3 |---| q2 |--+-+-| g1  |-+-----+-| q11 |-+-+--| q1 |
        # +----+   +----+  | | +-----+ |     | +-----+ | |  +----+
        #                  | |    |    |     |    |    | |
        #                  | |    |    |     |    |    | |
        #                  | |    |    |     |    |    | |
        #                  | | +-----+ |     | +-----+ | |
        #                  | | | q22 | |     | | q12 | | |
        #                  | | +-----+ |     | +-----+ | |
        #                  | +---------+     +---------+ |
        #                  +-----------------------------+

        gobgp_ctn_image_name = parser_option.gobgp_image
        base.TEST_PREFIX = parser_option.test_prefix

        bgp_conf_1 = {'global': {'confederation': {'config': {
            'enabled': True, 'identifier': 30, 'member-as-list': [65002]}}}}
        bgp_conf_2 = {'global': {'confederation': {'config': {
            'enabled': True, 'identifier': 30, 'member-as-list': [65001]}}}}

        g1 = GoBGPContainer(name='g1', asn=65002, router_id='192.168.2.1',
                            ctn_image_name=gobgp_ctn_image_name,
                            log_level=parser_option.gobgp_log_level,
                            bgp_config=bgp_conf_2)

        q1 = QuaggaBGPContainer(name='q1', asn=10, router_id='1.1.1.1')
        q2 = QuaggaBGPContainer(name='q2', asn=20, router_id='2.2.2.2')
        q3 = QuaggaBGPContainer(name='q3', asn=21, router_id='3.3.3.3')
        q11 = QuaggaBGPContainer(name='q11', asn=65001, router_id='192.168.1.1', bgpd_config=bgp_conf_1)
        q12 = QuaggaBGPContainer(name='q12', asn=65001, router_id='192.168.1.2', bgpd_config=bgp_conf_1)
        q22 = QuaggaBGPContainer(name='q22', asn=65002, router_id='192.168.2.2', bgpd_config=bgp_conf_2)

        ctns = [g1, q1, q2, q3, q11, q12, q22]

        cls.initial_wait_time = max(ctn.run() for ctn in ctns)

        time.sleep(cls.initial_wait_time)

        q1.add_peer(q11, remote_as=30)
        q11.add_peer(q1)
        q11.add_peer(q12)
        q12.add_peer(q11)
        g1.add_peer(q11)
        q11.add_peer(g1)
        g1.add_peer(q22)
        q22.add_peer(g1)
        g1.add_peer(q2)
        q2.add_peer(g1, remote_as=30)
        q3.add_peer(q2)
        q2.add_peer(q3)
        cls.gobgp = g1
        cls.quaggas = {'q1': q1, 'q2': q2, 'q3': q3, 'q11': q11, 'q12': q12, 'q22': q22}

    # test each neighbor state is turned establish
    def test_01_neighbor_established(self):
        self.gobgp.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=self.quaggas['q11'])
        self.gobgp.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=self.quaggas['q22'])
        self.gobgp.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=self.quaggas['q2'])
        self.quaggas['q11'].wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=self.quaggas['q1'])
        self.quaggas['q11'].wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=self.quaggas['q12'])
        self.quaggas['q2'].wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=self.quaggas['q3'])

    def test_02_route_advertise(self):
        self.quaggas['q3'].add_route('10.0.0.0/24')
        time.sleep(self.initial_wait_time)

        routes = []
        for _ in range(60):
            routes = self.quaggas['q1'].get_global_rib('10.0.0.0/24')
            if routes:
                break
            time.sleep(1)
        self.failIf(len(routes) == 0)

        # Confirm AS_PATH in confederation is removed
        self._check_global_rib_first(self.quaggas['q1'], '10.0.0.0/24', [30, 20, 21])

        # Confirm AS_PATH in confederation is not removed
        self._check_global_rib_first(self.quaggas['q11'], '10.0.0.0/24', [65002, 20, 21])

        self._check_global_rib_first(self.quaggas['q22'], '10.0.0.0/24', [20, 21])

    def test_03_best_path(self):
        self.quaggas['q1'].add_route('10.0.0.0/24')

        routes = []
        for _ in range(60):
            routes = self.gobgp.get_global_rib('10.0.0.0/24')
            if len(routes) == 1:
                if len(routes[0]['paths']) == 2:
                    break
            time.sleep(1)
        self.failIf(len(routes) != 1)
        self.failIf(len(routes[0]['paths']) != 2)

        # In g1, there are two routes to 10.0.0.0/24
        # confirm the route from q1 is selected as the best path
        # because it has shorter AS_PATH.
        # (AS_CONFED_* segments in AS_PATH is not counted)
        paths = routes[0]['paths']
        self.assertTrue(paths[0]['aspath'], [65001, 10])

        # confirm the new best path is advertised
        self._check_global_rib_first(self.quaggas['q22'], '10.0.0.0/24', [65001, 10])


if __name__ == '__main__':
    output = local("which docker 2>&1 > /dev/null ; echo $?", capture=True)
    if int(output) is not 0:
        print("docker not found")
        sys.exit(1)

    nose.main(argv=sys.argv, addplugins=[OptionParser()],
              defaultTest=sys.argv[0])
