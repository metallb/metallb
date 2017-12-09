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
from lib.base import BGP_FSM_ESTABLISHED
from lib.gobgp import GoBGPContainer
from lib.exabgp import ExaBGPContainer


class GoBGPTestBase(unittest.TestCase):

    @classmethod
    def setUpClass(cls):
        gobgp_ctn_image_name = parser_option.gobgp_image
        base.TEST_PREFIX = parser_option.test_prefix

        g1 = GoBGPContainer(name='g1', asn=65000, router_id='192.168.0.1',
                            ctn_image_name=gobgp_ctn_image_name,
                            log_level=parser_option.gobgp_log_level)
        e1 = ExaBGPContainer(name='e1', asn=65000, router_id='192.168.0.2')

        ctns = [g1, e1]

        # advertise a route from q1, q2
        matchs = ['destination 10.0.0.0/24', 'source 20.0.0.0/24']
        thens = ['discard']
        e1.add_route(route='flow1', rf='ipv4-flowspec', matchs=matchs, thens=thens)
        matchs2 = ['tcp-flags S', 'protocol ==tcp ==udp', "packet-length '>1000&<2000'", "source-port '!=2&!=22&!=222'"]
        thens2 = ['rate-limit 9600', 'redirect 0.10:100', 'mark 20', 'action sample']
        g1.add_route(route='flow1', rf='ipv4-flowspec', matchs=matchs2, thens=thens2)
        matchs3 = ['destination 2001::/24/10', 'source 2002::/24/15']
        thens3 = ['discard']
        e1.add_route(route='flow2', rf='ipv6-flowspec', matchs=matchs3, thens=thens3)
        matchs4 = ['destination 2001::/24 10', "label '==100'"]
        thens4 = ['discard']
        g1.add_route(route='flow2', rf='ipv6-flowspec', matchs=matchs4, thens=thens4)

        initial_wait_time = max(ctn.run() for ctn in ctns)

        time.sleep(initial_wait_time)

        # ibgp peer. loop topology
        for a, b in combinations(ctns, 2):
            a.add_peer(b, flowspec=True)
            b.add_peer(a, flowspec=True)

        cls.gobgp = g1
        cls.exabgp = e1

    # test each neighbor state is turned establish
    def test_01_neighbor_established(self):
        self.gobgp.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=self.exabgp)

    def test_02_check_gobgp_global_rib(self):
        self.assertTrue(len(self.gobgp.get_global_rib(rf='ipv4-flowspec')) == 2)

    def test_03_check_gobgp_global_rib(self):
        self.assertTrue(len(self.gobgp.get_global_rib(rf='ipv6-flowspec')) == 2)


if __name__ == '__main__':
    output = local("which docker 2>&1 > /dev/null ; echo $?", capture=True)
    if int(output) is not 0:
        print "docker not found"
        sys.exit(1)

    nose.main(argv=sys.argv, addplugins=[OptionParser()],
              defaultTest=sys.argv[0])
