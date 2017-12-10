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
        e1 = ExaBGPContainer(name='e1', asn=65001, router_id='192.168.0.2')

        ctns = [g1, e1]
        initial_wait_time = max(ctn.run() for ctn in ctns)
        time.sleep(initial_wait_time)

        g1.add_peer(e1, treat_as_withdraw=True)
        e1.add_peer(g1)

        cls.g1 = g1
        cls.e1 = e1

    # test each neighbor state is turned establish
    def test_01_neighbor_established(self):
        self.g1.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=self.e1)

    def test_02_attribute_discard(self):
        # Malformed attribute 'AGGREGATOR' should be discard, but the session should not be disconnected.
        self.e1.add_route('10.0.0.0/24', attribute='0x07 0xc0 0x0000006400')

        # Confirm the session is not disconnected
        for _ in range(5):
            state = self.g1.get_neighbor_state(self.e1)
            self.assertTrue(BGP_FSM_ESTABLISHED, state)
            time.sleep(1)

        # Confirm the path is added
        dests = self.g1.get_global_rib()
        self.assertTrue(len(dests) == 1)
        routes = dests[0]['paths']
        self.assertTrue(len(routes) == 1)

        # Confirm the attribute 'AGGREGATOR(type=7)' is discarded
        for d in routes[0]['attrs']:
            self.assertFalse(d['type'] == 7)

        self.e1.del_route('10.0.0.0/24')

    def test_03_treat_as_withdraw(self):
        # Malformed attribute 'MULTI_EXIT_DESC' should be treated as withdraw,
        # but the session should not be disconnected.
        self.e1.add_route('20.0.0.0/24', attribute='0x04 0x80 0x00000064')
        self.e1.add_route('30.0.0.0/24', attribute='0x04 0x80 0x00000064')
        # Malformed
        self.e1.add_route('30.0.0.0/24', attribute='0x04 0x80 0x0000000064')

        # Confirm the session is not disconnected
        for _ in range(5):
            state = self.g1.get_neighbor_state(self.e1)
            self.assertTrue(BGP_FSM_ESTABLISHED, state)
            time.sleep(1)

        # Confirm the number of path in RIB is only one
        dests = self.g1.get_global_rib()
        self.assertTrue(len(dests) == 1)
        self.assertTrue(dests[0]['paths'][0]['nlri']['prefix'] == '20.0.0.0/24')


if __name__ == '__main__':
    output = local("which docker 2>&1 > /dev/null ; echo $?", capture=True)
    if int(output) is not 0:
        print "docker not found"
        sys.exit(1)

    nose.main(argv=sys.argv, addplugins=[OptionParser()],
              defaultTest=sys.argv[0])
