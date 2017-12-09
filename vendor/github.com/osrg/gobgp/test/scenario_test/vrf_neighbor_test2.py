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
    BGP_FSM_ACTIVE,
    BGP_FSM_ESTABLISHED,
    wait_for_completion,
)
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
        g2 = GoBGPContainer(name='g2', asn=65001, router_id='192.168.0.2',
                            ctn_image_name=gobgp_ctn_image_name,
                            log_level=parser_option.gobgp_log_level,
                            config_format='yaml')
        g3 = GoBGPContainer(name='g3', asn=65001, router_id='192.168.0.3',
                            ctn_image_name=gobgp_ctn_image_name,
                            log_level=parser_option.gobgp_log_level,
                            config_format='yaml')
        g4 = GoBGPContainer(name='g4', asn=65001, router_id='192.168.0.4',
                            ctn_image_name=gobgp_ctn_image_name,
                            log_level=parser_option.gobgp_log_level,
                            config_format='yaml')
        g5 = GoBGPContainer(name='g5', asn=65001, router_id='192.168.0.5',
                            ctn_image_name=gobgp_ctn_image_name,
                            log_level=parser_option.gobgp_log_level,
                            config_format='yaml')

        ctns = [g1, g2, g3, g4, g5]

        initial_wait_time = max(ctn.run() for ctn in ctns)

        time.sleep(initial_wait_time)

        g3.local("gobgp vrf add red rd 10:10 rt both 10:10")
        g3.local("gobgp vrf add blue rd 20:20 rt both 20:20")

        g1.add_peer(g3, graceful_restart=True, llgr=True)
        g3.add_peer(g1, vrf='red', is_rr_client=True, graceful_restart=True, llgr=True)

        g2.add_peer(g3, graceful_restart=True, llgr=True)
        g3.add_peer(g2, vrf='red', is_rr_client=True, graceful_restart=True, llgr=True)

        g4.add_peer(g3, graceful_restart=True, llgr=True)
        g3.add_peer(g4, vrf='blue', is_rr_client=True, graceful_restart=True, llgr=True)

        g5.add_peer(g3, graceful_restart=True, llgr=True)
        g3.add_peer(g5, vrf='blue', is_rr_client=True, graceful_restart=True, llgr=True)

        cls.g1 = g1
        cls.g2 = g2
        cls.g3 = g3
        cls.g4 = g4
        cls.g5 = g5

    # test each neighbor state is turned establish
    def test_01_neighbor_established(self):
        self.g3.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=self.g1)
        self.g3.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=self.g2)
        self.g3.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=self.g4)
        self.g3.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=self.g5)

    def test_02_add_routes(self):
        self.g1.local("gobgp global rib add 10.0.0.0/24")
        self.g4.local("gobgp global rib add 10.0.0.0/24")
        wait_for_completion(lambda: len(self.g3.get_global_rib(rf="vpnv4")) == 2)
        wait_for_completion(lambda: len(self.g2.get_global_rib()) == 1)
        wait_for_completion(lambda: len(self.g5.get_global_rib()) == 1)

    def test_03_disable(self):
        self.g3.disable_peer(self.g1)
        wait_for_completion(lambda: len(self.g3.get_global_rib(rf="vpnv4")) == 1)
        wait_for_completion(lambda: len(self.g2.get_global_rib()) == 0)
        wait_for_completion(lambda: len(self.g5.get_global_rib()) == 1)
        self.g3.enable_peer(self.g1)
        self.g3.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=self.g1)

    def test_04_softreset_in(self):
        self.g3.softreset(self.g1)
        wait_for_completion(lambda: len(self.g3.get_global_rib()) == 0)
        wait_for_completion(lambda: len(self.g3.get_global_rib(rf="vpnv4")) == 2)
        wait_for_completion(lambda: len(self.g2.get_global_rib()) == 1)
        wait_for_completion(lambda: len(self.g5.get_global_rib()) == 1)

    def test_05_softreset_out(self):
        self.g3.softreset(self.g2, type='out')
        wait_for_completion(lambda: len(self.g3.get_global_rib()) == 0)
        wait_for_completion(lambda: len(self.g3.get_global_rib(rf="vpnv4")) == 2)
        wait_for_completion(lambda: len(self.g2.get_global_rib()) == 1)
        wait_for_completion(lambda: len(self.g5.get_global_rib()) == 1)

    def test_06_graceful_restart(self):
        self.g1.graceful_restart()
        self.g3.wait_for(expected_state=BGP_FSM_ACTIVE, peer=self.g1)

        wait_for_completion(lambda: len(self.g3.get_global_rib(rf="vpnv4")) == 2)
        wait_for_completion(lambda: len(self.g2.get_global_rib()) == 1)

        wait_for_completion(lambda: len(self.g3.get_global_rib(rf="vpnv4")) == 1)
        wait_for_completion(lambda: len(self.g2.get_global_rib()) == 0)


if __name__ == '__main__':
    output = local("which docker 2>&1 > /dev/null ; echo $?", capture=True)
    if int(output) is not 0:
        print "docker not found"
        sys.exit(1)

    nose.main(argv=sys.argv, addplugins=[OptionParser()],
              defaultTest=sys.argv[0])
