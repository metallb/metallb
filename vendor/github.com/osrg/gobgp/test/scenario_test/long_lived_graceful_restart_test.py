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

from itertools import chain
import sys
import time
import unittest

from fabric.api import local
import nose

from lib.noseplugin import OptionParser, parser_option

from lib import base
from lib.base import (
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
        g3 = GoBGPContainer(name='g3', asn=65002, router_id='192.168.0.3',
                            ctn_image_name=gobgp_ctn_image_name,
                            log_level=parser_option.gobgp_log_level)
        g4 = GoBGPContainer(name='g4', asn=65003, router_id='192.168.0.4',
                            ctn_image_name=gobgp_ctn_image_name,
                            log_level=parser_option.gobgp_log_level)
        ctns = [g1, g2, g3, g4]

        initial_wait_time = max(ctn.run() for ctn in ctns)

        time.sleep(initial_wait_time)

        g1.add_peer(g2, graceful_restart=True, llgr=True)
        g2.add_peer(g1, graceful_restart=True, llgr=True)
        g1.add_peer(g3, graceful_restart=True, llgr=True)
        g3.add_peer(g1, graceful_restart=True, llgr=True)
        g1.add_peer(g4, graceful_restart=True)
        g4.add_peer(g1, graceful_restart=True)

        cls.bgpds = {'g1': g1, 'g2': g2, 'g3': g3, 'g4': g4}

    # test each neighbor state is turned establish
    def test_01_neighbor_established(self):
        g1 = self.bgpds['g1']
        g2 = self.bgpds['g2']
        g3 = self.bgpds['g3']
        g4 = self.bgpds['g4']
        g1.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=g2)
        g1.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=g3)
        g1.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=g4)

    def test_02_graceful_restart(self):
        g1 = self.bgpds['g1']
        g2 = self.bgpds['g2']
        g3 = self.bgpds['g3']
        g4 = self.bgpds['g4']

        g2.local('gobgp global rib add 10.0.0.0/24')
        g2.local('gobgp global rib add 10.10.0.0/24 community no-llgr')

        time.sleep(1)

        g2.graceful_restart()
        g1.wait_for(expected_state=BGP_FSM_ACTIVE, peer=g2)

        time.sleep(1)

        self.assertTrue(len(g1.get_global_rib('10.0.0.0/24')) == 1)
        # 10.10.0.0/24 is announced with no-llgr community
        # must not exist in the rib
        self.assertTrue(len(g1.get_global_rib('10.10.0.0/24')) == 0)
        for d in g1.get_global_rib():
            for p in d['paths']:
                self.assertTrue(p['stale'])

        self.assertTrue(len(g3.get_global_rib('10.0.0.0/24')) == 1)
        # check llgr-stale community is added to 10.0.0.0/24
        r = g3.get_global_rib('10.0.0.0/24')[0]['paths'][0]
        comms = list(chain.from_iterable([attr['communities'] for attr in r['attrs'] if attr['type'] == 8]))
        self.assertTrue(0xffff0006 in comms)
        # g4 is not llgr capable, llgr-stale route must be
        # withdrawn
        self.assertTrue(len(g4.get_global_rib('10.0.0.0/24')) == 0)

        g2._start_gobgp(graceful_restart=True)
        time.sleep(2)
        g2.local('gobgp global rib add 10.0.0.0/24')
        g2.local('gobgp global rib add 10.10.0.0/24')

    def test_03_neighbor_established(self):
        g1 = self.bgpds['g1']
        g2 = self.bgpds['g2']
        # g3 = self.bgpds['g3']
        # g4 = self.bgpds['g4']
        g1.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=g2)
        time.sleep(1)
        self.assertTrue(len(g1.get_global_rib('10.0.0.0/24')) == 1)
        self.assertTrue(len(g1.get_global_rib('10.10.0.0/24')) == 1)
        for d in g1.get_global_rib():
            for p in d['paths']:
                self.assertFalse(p.get('stale', False))

    def test_04_llgr_stale_route_depreferenced(self):
        g1 = self.bgpds['g1']
        g2 = self.bgpds['g2']
        g3 = self.bgpds['g3']
        g4 = self.bgpds['g4']
        g4.local('gobgp global rib add 10.0.0.0/24 med 100')
        time.sleep(1)
        # check g2's path is chosen as best and advertised
        rib = g3.get_global_rib('10.0.0.0/24')
        self.assertTrue(len(rib) == 1)
        self.assertTrue(g2.asn in rib[0]['paths'][0]['aspath'])

        g2.graceful_restart()
        g1.wait_for(expected_state=BGP_FSM_ACTIVE, peer=g2)

        time.sleep(1)

        # llgr_stale route depreference must happend
        # check g4's path is chosen as best and advertised
        rib = g3.get_global_rib('10.0.0.0/24')
        self.assertTrue(len(rib) == 1)
        self.assertTrue(g4.asn in rib[0]['paths'][0]['aspath'])

        # if no candidate exists, llgr_stale route will be chosen as best
        rib = g3.get_global_rib('10.10.0.0/24')
        self.assertTrue(len(rib) == 1)
        self.assertTrue(g2.asn in rib[0]['paths'][0]['aspath'])

    def test_05_llgr_restart_timer_expire(self):
        time.sleep(35)
        g3 = self.bgpds['g3']
        rib = g3.get_global_rib('10.10.0.0/24')
        self.assertTrue(len(rib) == 0)


if __name__ == '__main__':
    output = local("which docker 2>&1 > /dev/null ; echo $?", capture=True)
    if int(output) is not 0:
        print "docker not found"
        sys.exit(1)

    nose.main(argv=sys.argv, addplugins=[OptionParser()],
              defaultTest=sys.argv[0])
