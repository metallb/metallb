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
from lib.base import BGP_FSM_ESTABLISHED
from lib.gobgp import GoBGPContainer


class GoBGPTestBase(unittest.TestCase):

    @classmethod
    def setUpClass(cls):
        gobgp_ctn_image_name = parser_option.gobgp_image
        base.TEST_PREFIX = parser_option.test_prefix

        g1 = GoBGPContainer(name='g1', asn=65001, router_id='192.168.0.1',
                            ctn_image_name=gobgp_ctn_image_name,
                            log_level=parser_option.gobgp_log_level,
                            config_format='yaml')
        g2 = GoBGPContainer(name='g2', asn=65002, router_id='192.168.0.2',
                            ctn_image_name=gobgp_ctn_image_name,
                            log_level=parser_option.gobgp_log_level,
                            config_format='yaml')
        g3 = GoBGPContainer(name='g3', asn=65003, router_id='192.168.0.3',
                            ctn_image_name=gobgp_ctn_image_name,
                            log_level=parser_option.gobgp_log_level,
                            config_format='yaml')
        g4 = GoBGPContainer(name='g4', asn=65004, router_id='192.168.0.4',
                            ctn_image_name=gobgp_ctn_image_name,
                            log_level=parser_option.gobgp_log_level,
                            config_format='yaml')
        g5 = GoBGPContainer(name='g5', asn=65005, router_id='192.168.0.5',
                            ctn_image_name=gobgp_ctn_image_name,
                            log_level=parser_option.gobgp_log_level,
                            config_format='yaml')
        g6 = GoBGPContainer(name='g6', asn=65006, router_id='192.168.0.6',
                            ctn_image_name=gobgp_ctn_image_name,
                            log_level=parser_option.gobgp_log_level,
                            config_format='yaml')
        g7 = GoBGPContainer(name='g7', asn=65007, router_id='192.168.0.7',
                            ctn_image_name=gobgp_ctn_image_name,
                            log_level=parser_option.gobgp_log_level,
                            config_format='yaml')

        ctns = [g1, g2, g3, g4, g5, g6, g7]

        initial_wait_time = max(ctn.run() for ctn in ctns)

        time.sleep(initial_wait_time)

        g4.local("gobgp vrf add red rd 10:10 rt both 10:10")
        g4.local("gobgp vrf add blue rd 20:20 rt both 20:20")

        g5.local("gobgp vrf add red rd 10:10 rt both 10:10")
        g5.local("gobgp vrf add blue rd 20:20 rt both 20:20")

        g1.add_peer(g4)
        g4.add_peer(g1, vrf='red')

        g2.add_peer(g4)
        g4.add_peer(g2, vrf='red')

        g3.add_peer(g4)
        g4.add_peer(g3, vrf='blue')

        g4.add_peer(g5, vpn=True)
        g5.add_peer(g4, vpn=True)

        g5.add_peer(g6, vrf='red')
        g6.add_peer(g5)

        g5.add_peer(g7, vrf='blue')
        g7.add_peer(g5)

        cls.g1 = g1
        cls.g2 = g2
        cls.g3 = g3
        cls.g4 = g4
        cls.g5 = g5
        cls.g6 = g6
        cls.g7 = g7

    # test each neighbor state is turned establish
    def test_01_neighbor_established(self):
        self.g4.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=self.g1)
        self.g4.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=self.g2)
        self.g4.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=self.g3)
        self.g4.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=self.g5)
        self.g5.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=self.g6)
        self.g5.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=self.g7)

    def test_02_inject_from_vrf_red(self):
        self.g1.local('gobgp global rib add 10.0.0.0/24')

        time.sleep(1)

        dst = self.g2.get_global_rib('10.0.0.0/24')
        self.assertTrue(len(dst) == 1)
        self.assertTrue(len(dst[0]['paths']) == 1)
        path = dst[0]['paths'][0]
        self.assertTrue([self.g4.asn, self.g1.asn] == path['aspath'])

        dst = self.g3.get_global_rib('10.0.0.0/24')
        self.assertTrue(len(dst) == 0)

        dst = self.g4.get_global_rib(rf='vpnv4')
        self.assertTrue(len(dst) == 1)
        self.assertTrue(len(dst[0]['paths']) == 1)
        path = dst[0]['paths'][0]
        self.assertTrue([self.g1.asn] == path['aspath'])

        dst = self.g6.get_global_rib('10.0.0.0/24')
        self.assertTrue(len(dst) == 1)
        self.assertTrue(len(dst[0]['paths']) == 1)
        path = dst[0]['paths'][0]
        self.assertTrue([self.g5.asn, self.g4.asn, self.g1.asn] == path['aspath'])

        dst = self.g7.get_global_rib('10.0.0.0/24')
        self.assertTrue(len(dst) == 0)

    def test_03_inject_from_vrf_blue(self):
        self.g3.local('gobgp global rib add 10.0.0.0/24')

        time.sleep(1)

        dst = self.g2.get_global_rib('10.0.0.0/24')
        self.assertTrue(len(dst) == 1)
        self.assertTrue(len(dst[0]['paths']) == 1)
        path = dst[0]['paths'][0]
        self.assertTrue([self.g4.asn, self.g1.asn] == path['aspath'])

        dst = self.g4.get_global_rib(rf='vpnv4')
        self.assertTrue(len(dst) == 2)

        dst = self.g6.get_global_rib('10.0.0.0/24')
        self.assertTrue(len(dst) == 1)
        self.assertTrue(len(dst[0]['paths']) == 1)
        path = dst[0]['paths'][0]
        self.assertTrue([self.g5.asn, self.g4.asn, self.g1.asn] == path['aspath'])

        dst = self.g7.get_global_rib('10.0.0.0/24')
        self.assertTrue(len(dst) == 1)
        self.assertTrue(len(dst[0]['paths']) == 1)
        path = dst[0]['paths'][0]
        self.assertTrue([self.g5.asn, self.g4.asn, self.g3.asn] == path['aspath'])


if __name__ == '__main__':
    output = local("which docker 2>&1 > /dev/null ; echo $?", capture=True)
    if int(output) is not 0:
        print "docker not found"
        sys.exit(1)

    nose.main(argv=sys.argv, addplugins=[OptionParser()],
              defaultTest=sys.argv[0])
