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


class GoBGPTestBase(unittest.TestCase):

    @classmethod
    def setUpClass(cls):
        gobgp_ctn_image_name = parser_option.gobgp_image
        base.TEST_PREFIX = parser_option.test_prefix

        g1 = GoBGPContainer(name='g1', asn=65000, router_id='192.168.0.1',
                            ctn_image_name=gobgp_ctn_image_name,
                            log_level=parser_option.gobgp_log_level)
        g2 = GoBGPContainer(name='g2', asn=65000, router_id='192.168.0.2',
                            ctn_image_name=gobgp_ctn_image_name,
                            log_level=parser_option.gobgp_log_level)

        ctns = {ctn.name: ctn for ctn in [g1, g2]}

        initial_wait_time = max(ctn.run() for ctn in ctns.values())

        time.sleep(initial_wait_time)

        g1.local("gobgp vrf add vrf1 rd 100:100 rt both 100:100")
        g1.local("gobgp vrf add vrf2 rd 200:200 rt both 200:200")
        g2.local("gobgp vrf add vrf1 rd 100:100 rt both 100:100")
        g2.local("gobgp vrf add vrf3 rd 300:300 rt both 300:300")

        g1.local("gobgp vrf vrf1 rib add 10.0.0.0/24")
        g1.local("gobgp vrf vrf2 rib add 10.0.0.0/24")
        g2.local("gobgp vrf vrf1 rib add 20.0.0.0/24")
        g2.local("gobgp vrf vrf3 rib add 20.0.0.0/24")

        for a, b in combinations(ctns.values(), 2):
            a.add_peer(b, vpn=True, passwd='rtc', graceful_restart=True)
            b.add_peer(a, vpn=True, passwd='rtc', graceful_restart=True)

        cls.g1 = g1
        cls.g2 = g2
        cls.ctns = ctns

    # test each neighbor state is turned establish
    def test_01_neighbor_established(self):
        self.g1.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=self.g2)

    def test_02_check_gobgp_adj_rib_out(self):
        time.sleep(2)
        self.assertTrue(len(self.g1.get_adj_rib_out(self.g2, rf='ipv4-l3vpn')) == 1)
        self.assertTrue(len(self.g2.get_adj_rib_out(self.g1, rf='ipv4-l3vpn')) == 1)

    def test_03_add_vrf(self):
        self.g1.local("gobgp vrf add vrf3 rd 300:300 rt both 300:300")
        time.sleep(2)
        self.assertTrue(len(self.g1.get_adj_rib_out(self.g2, rf='rtc')) == 3)
        self.assertTrue(len(self.g1.get_adj_rib_in(self.g2, rf='ipv4-l3vpn')) == 2)

    def test_04_del_vrf(self):
        self.g1.local("gobgp vrf del vrf1")
        time.sleep(2)
        self.assertTrue(len(self.g1.get_adj_rib_in(self.g2, rf='ipv4-l3vpn')) == 1)
        self.assertTrue(len(self.g1.get_adj_rib_out(self.g2, rf='rtc')) == 2)

    def test_05_rr_setup(self):
        gobgp_ctn_image_name = parser_option.gobgp_image
        g3 = GoBGPContainer(name='g3', asn=65000, router_id='192.168.0.3',
                            ctn_image_name=gobgp_ctn_image_name,
                            log_level=parser_option.gobgp_log_level)
        g4 = GoBGPContainer(name='g4', asn=65000, router_id='192.168.0.4',
                            ctn_image_name=gobgp_ctn_image_name,
                            log_level=parser_option.gobgp_log_level)
        g5 = GoBGPContainer(name='g5', asn=65000, router_id='192.168.0.5',
                            ctn_image_name=gobgp_ctn_image_name,
                            log_level=parser_option.gobgp_log_level)

        time.sleep(max(ctn.run() for ctn in [g3, g4, g5]))

        g3.add_peer(g4, vpn=True, is_rr_client=True)
        g4.add_peer(g3, vpn=True)

        g3.add_peer(g5, vpn=True, is_rr_client=True)
        g5.add_peer(g3, vpn=True)

        for v in [g3, g4, g5]:
            self.ctns[v.name] = v

    def test_06_neighbor_established(self):
        g3 = self.ctns['g3']
        g4 = self.ctns['g4']
        g5 = self.ctns['g5']
        g3.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=g4)
        g3.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=g5)

    def test_07_rr_test(self):
        g3 = self.ctns['g3']
        g4 = self.ctns['g4']
        g5 = self.ctns['g5']
        g4.local("gobgp vrf add vrf1 rd 100:100 rt both 100:100")
        time.sleep(1)
        g5.local("gobgp vrf add vrf1 rd 100:100 rt both 100:100")

        time.sleep(1)

        def check_rtc(client):
            rib = g3.get_adj_rib_out(client, rf='rtc')
            self.assertTrue(len(rib) == 1)
            path = rib[0]
            self.assertTrue(path['nexthop'] == g3.peers[client]['local_addr'].split('/')[0])
            ids = [attr['value'] for attr in path['attrs'] if attr['type'] == base.BGP_ATTR_TYPE_ORIGINATOR_ID]
            self.assertTrue(len(ids) == 1)
            self.assertTrue(ids[0] == g3.router_id)

        check_rtc(g4)
        check_rtc(g5)

        g4.local("gobgp vrf vrf1 rib add 40.0.0.0/24")
        g5.local("gobgp vrf vrf1 rib add 50.0.0.0/24")
        time.sleep(1)

        def check_ipv4_l3vpn(client):
            rib = g3.get_adj_rib_out(client, rf='ipv4-l3vpn')
            self.assertTrue(len(rib) == 1)
            path = rib[0]
            self.assertTrue(path['nexthop'] != g3.peers[client]['local_addr'].split('/')[0])
            ids = [attr['value'] for attr in path['attrs'] if attr['type'] == base.BGP_ATTR_TYPE_ORIGINATOR_ID]
            self.assertTrue(len(ids) == 1)
            self.assertTrue(ids[0] != client.router_id)

        check_ipv4_l3vpn(g4)
        check_ipv4_l3vpn(g5)

    def test_08_rr_setup2(self):
        g1 = self.ctns['g1']
        g2 = self.ctns['g2']
        g3 = self.ctns['g3']

        g1.local("gobgp vrf del vrf2")
        g1.local("gobgp vrf del vrf3")
        g2.local("gobgp vrf del vrf1")
        g2.local("gobgp vrf del vrf3")

        g3.add_peer(g1, vpn=True)
        g1.add_peer(g3, vpn=True)
        g3.add_peer(g2, vpn=True)
        g2.add_peer(g3, vpn=True)

        g3.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=g1)
        g3.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=g2)

    def test_09_rr_test2(self):
        g1 = self.ctns['g1']
        g2 = self.ctns['g2']
        g3 = self.ctns['g3']
        g4 = self.ctns['g4']
        g5 = self.ctns['g5']

        self.assertTrue(len(g1.get_adj_rib_in(g3, rf='ipv4-l3vpn')) == 0)
        self.assertTrue(len(g2.get_adj_rib_in(g3, rf='ipv4-l3vpn')) == 0)

        g1.local("gobgp vrf add vrf1 rd 100:100 rt both 100:100")
        g1.local("gobgp vrf vrf1 rib add 10.0.0.0/24")

        time.sleep(1)

        self.assertTrue(len(g1.get_adj_rib_in(g3, rf='ipv4-l3vpn')) == 2)
        self.assertTrue(len(g2.get_adj_rib_in(g3, rf='ipv4-l3vpn')) == 0)
        self.assertTrue(len(g3.get_adj_rib_in(g1, rf='ipv4-l3vpn')) == 1)
        self.assertTrue(len(g3.get_adj_rib_in(g2, rf='ipv4-l3vpn')) == 0)
        self.assertTrue(len(g4.get_adj_rib_in(g3, rf='ipv4-l3vpn')) == 2)
        self.assertTrue(len(g5.get_adj_rib_in(g3, rf='ipv4-l3vpn')) == 2)

        g2.local("gobgp vrf add vrf2 rd 200:200 rt both 200:200")
        g2.local("gobgp vrf vrf2 rib add 20.0.0.0/24")

        time.sleep(1)

        self.assertTrue(len(g1.get_adj_rib_in(g3, rf='ipv4-l3vpn')) == 2)
        self.assertTrue(len(g2.get_adj_rib_in(g3, rf='ipv4-l3vpn')) == 0)
        self.assertTrue(len(g3.get_adj_rib_in(g1, rf='ipv4-l3vpn')) == 1)
        self.assertTrue(len(g3.get_adj_rib_in(g2, rf='ipv4-l3vpn')) == 0)
        self.assertTrue(len(g4.get_adj_rib_in(g3, rf='ipv4-l3vpn')) == 2)
        self.assertTrue(len(g5.get_adj_rib_in(g3, rf='ipv4-l3vpn')) == 2)

        g4.local("gobgp vrf add vrf2 rd 200:200 rt both 200:200")

        time.sleep(1)

        self.assertTrue(len(g1.get_adj_rib_in(g3, rf='ipv4-l3vpn')) == 2)
        self.assertTrue(len(g2.get_adj_rib_in(g3, rf='ipv4-l3vpn')) == 0)
        self.assertTrue(len(g3.get_adj_rib_in(g1, rf='ipv4-l3vpn')) == 1)
        self.assertTrue(len(g3.get_adj_rib_in(g2, rf='ipv4-l3vpn')) == 1)
        self.assertTrue(len(g4.get_adj_rib_in(g3, rf='ipv4-l3vpn')) == 3)
        self.assertTrue(len(g5.get_adj_rib_in(g3, rf='ipv4-l3vpn')) == 2)


if __name__ == '__main__':
    output = local("which docker 2>&1 > /dev/null ; echo $?", capture=True)
    if int(output) is not 0:
        print "docker not found"
        sys.exit(1)

    nose.main(argv=sys.argv, addplugins=[OptionParser()],
              defaultTest=sys.argv[0])
