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

import unittest
from fabric.api import local
from lib import base
from lib.base import BGP_FSM_ESTABLISHED
from lib.gobgp import GoBGPContainer
import sys
import os
import time
import nose
from lib.noseplugin import OptionParser, parser_option
from itertools import combinations


class GoBGPTestBase(unittest.TestCase):

    @classmethod
    def setUpClass(cls):
        gobgp_ctn_image_name = parser_option.gobgp_image
        base.TEST_PREFIX = parser_option.test_prefix

        g1 = GoBGPContainer(name='g1', asn=65000, router_id='192.168.0.1',
                            ctn_image_name=gobgp_ctn_image_name,
                            log_level=parser_option.gobgp_log_level)
        g2 = GoBGPContainer(name='g2', asn=65001, router_id='192.168.0.2',
                            ctn_image_name=gobgp_ctn_image_name,
                            log_level=parser_option.gobgp_log_level)
        ctns = [g1, g2]

        initial_wait_time = max(ctn.run() for ctn in ctns)

        time.sleep(initial_wait_time + 2)

        done = False
        def f(ifname, ctn):
            out = ctn.local('ip -6 n', capture=True)
            l = [line for line in out.split('\n') if ifname in line]
            if len(l) == 0:
                return False
            elif len(l) > 1:
                raise Exception('not p2p link')
            return 'REACHABLE' in l[0]

        for i in range(20):
            g1.local('ping6 -c 1 ff02::1%eth0')
            g2.local('ping6 -c 1 ff02::1%eth0')
            if f('eth0', g1) and f('eth0', g2):
                done = True
                break
            time.sleep(1)

        if not done:
            raise Exception('timeout')

        for a, b in combinations(ctns, 2):
            a.add_peer(b, interface='eth0')
            b.add_peer(a, interface='eth0')

        cls.g1 = g1
        cls.g2 = g2

    # test each neighbor state is turned establish
    def test_01_neighbor_established(self):
        self.g1.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=self.g2)

    def test_02_add_ipv4_route(self):

        self.g1.add_route('10.0.0.0/24')

        time.sleep(1)

        rib = self.g2.get_global_rib(rf='ipv4')
        self.assertTrue(len(rib) == 1)


if __name__ == '__main__':
    output = local("which docker 2>&1 > /dev/null ; echo $?", capture=True)
    if int(output) is not 0:
        print "docker not found"
        sys.exit(1)

    nose.main(argv=sys.argv, addplugins=[OptionParser()],
              defaultTest=sys.argv[0])
