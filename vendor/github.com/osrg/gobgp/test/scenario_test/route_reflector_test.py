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

        # advertise a route from q1, q2
        for idx, c in enumerate(qs):
            route = '10.0.{0}.0/24'.format(idx + 1)
            c.add_route(route)

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


if __name__ == '__main__':
    output = local("which docker 2>&1 > /dev/null ; echo $?", capture=True)
    if int(output) is not 0:
        print "docker not found"
        sys.exit(1)

    nose.main(argv=sys.argv, addplugins=[OptionParser()],
              defaultTest=sys.argv[0])
