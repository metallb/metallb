# Copyright (C) 2016 Nippon Telegraph and Telephone Corporation.
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
from lib.base import (
    BGP_FSM_IDLE,
    BGP_FSM_ACTIVE,
    BGP_FSM_ESTABLISHED,
)
from lib.gobgp import GoBGPContainer


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

        time.sleep(initial_wait_time)

        g1.add_route('10.10.10.0/24')
        g1.add_route('10.10.20.0/24')

        g1.add_peer(g2, graceful_restart=True)
        g2.add_peer(g1, graceful_restart=True)

        cls.bgpds = {'g1': g1, 'g2': g2}

    # test each neighbor state is turned establish
    def test_01_neighbor_established(self):
        g1 = self.bgpds['g1']
        g2 = self.bgpds['g2']
        g1.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=g2)

    def test_02_graceful_restart(self):
        g1 = self.bgpds['g1']
        g2 = self.bgpds['g2']
        g1.graceful_restart()
        g2.wait_for(expected_state=BGP_FSM_ACTIVE, peer=g1)
        self.assertTrue(len(g2.get_global_rib('10.10.20.0/24')) == 1)
        self.assertTrue(len(g2.get_global_rib('10.10.10.0/24')) == 1)
        for d in g2.get_global_rib():
            for p in d['paths']:
                self.assertTrue(p['stale'])

        g1.routes = {}
        g1._start_gobgp(graceful_restart=True)
        time.sleep(3)
        g1.add_route('10.10.20.0/24')

    def test_03_neighbor_established(self):
        g1 = self.bgpds['g1']
        g2 = self.bgpds['g2']
        g1.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=g2)
        time.sleep(1)
        self.assertTrue(len(g2.get_global_rib('10.10.20.0/24')) == 1)
        self.assertTrue(len(g2.get_global_rib('10.10.10.0/24')) == 0)
        for d in g2.get_global_rib():
            for p in d['paths']:
                self.assertFalse(p.get('stale', False))

    def test_04_add_non_graceful_restart_enabled_peer(self):
        g1 = self.bgpds['g1']
        # g2 = self.bgpds['g2']
        gobgp_ctn_image_name = parser_option.gobgp_image
        g3 = GoBGPContainer(name='g3', asn=65002, router_id='192.168.0.3',
                            ctn_image_name=gobgp_ctn_image_name,
                            log_level=parser_option.gobgp_log_level)
        self.bgpds['g3'] = g3
        time.sleep(g3.run())
        g3.add_route('10.10.30.0/24')
        g1.add_peer(g3)
        g3.add_peer(g1)
        g1.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=g3)
        time.sleep(1)
        self.assertTrue(len(g3.get_global_rib('10.10.20.0/24')) == 1)

    def test_05_graceful_restart(self):
        g1 = self.bgpds['g1']
        g2 = self.bgpds['g2']
        g3 = self.bgpds['g3']
        g1.graceful_restart()
        g2.wait_for(expected_state=BGP_FSM_ACTIVE, peer=g1)
        self.assertTrue(len(g2.get_global_rib('10.10.20.0/24')) == 1)
        self.assertTrue(len(g2.get_global_rib('10.10.30.0/24')) == 1)
        for d in g2.get_global_rib():
            for p in d['paths']:
                self.assertTrue(p['stale'])

        self.assertTrue(len(g3.get_global_rib('10.10.20.0/24')) == 0)
        self.assertTrue(len(g3.get_global_rib('10.10.30.0/24')) == 1)

    def test_06_test_restart_timer_expire(self):
        time.sleep(25)
        g2 = self.bgpds['g2']
        self.assertTrue(len(g2.get_global_rib()) == 0)

    def test_07_multineighbor_established(self):
        g1 = self.bgpds['g1']
        g2 = self.bgpds['g2']
        g3 = self.bgpds['g3']

        g1._start_gobgp()

        g1.del_peer(g2)
        g1.del_peer(g3)
        g2.del_peer(g1)
        g3.del_peer(g1)
        g1.add_peer(g2, graceful_restart=True, llgr=True)
        g1.add_peer(g3, graceful_restart=True, llgr=True)
        g2.add_peer(g1, graceful_restart=True, llgr=True)
        g3.add_peer(g1, graceful_restart=True, llgr=True)

        g2.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=g1)
        g3.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=g1)

    def test_08_multineighbor_graceful_restart(self):
        g1 = self.bgpds['g1']
        g2 = self.bgpds['g2']
        g3 = self.bgpds['g3']

        g1.graceful_restart()
        g2.wait_for(expected_state=BGP_FSM_ACTIVE, peer=g1)
        g3.wait_for(expected_state=BGP_FSM_ACTIVE, peer=g1)

        g1._start_gobgp(graceful_restart=True)

        count = 0
        while ((g1.get_neighbor_state(g2) != BGP_FSM_ESTABLISHED)
                or (g1.get_neighbor_state(g3) != BGP_FSM_ESTABLISHED)):
            count += 1
            # assert connections are not refused
            self.assertTrue(g1.get_neighbor_state(g2) != BGP_FSM_IDLE)
            self.assertTrue(g1.get_neighbor_state(g3) != BGP_FSM_IDLE)
            if count > 120:
                raise Exception('timeout')
            time.sleep(1)


if __name__ == '__main__':
    output = local("which docker 2>&1 > /dev/null ; echo $?", capture=True)
    if int(output) is not 0:
        print "docker not found"
        sys.exit(1)

    nose.main(argv=sys.argv, addplugins=[OptionParser()],
              defaultTest=sys.argv[0])
