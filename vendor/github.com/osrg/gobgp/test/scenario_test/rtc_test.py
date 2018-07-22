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

    def assert_adv_count(self, src, dst, rf, count):
        self.assertEqual(count, len(src.get_adj_rib_out(dst, rf=rf)))
        self.assertEqual(count, len(dst.get_adj_rib_in(src, rf=rf)))

    def assert_upd_count(self, src, dst, sent, received):
        messages = src.get_neighbor(dst)['state']['messages']
        self.assertEqual(messages['sent']['update'], sent)
        self.assertEqual(messages['received']['update'], received)

    @classmethod
    def setUpClass(cls):
        # +----+              +----+
        # | g1 |----(iBGP)----| g2 |
        # +----+              +----+
        gobgp_ctn_image_name = parser_option.gobgp_image
        base.TEST_PREFIX = parser_option.test_prefix

        g1 = GoBGPContainer(name='g1', asn=65000, router_id='192.168.0.1',
                            ctn_image_name=gobgp_ctn_image_name,
                            log_level=parser_option.gobgp_log_level)
        g2 = GoBGPContainer(name='g2', asn=65000, router_id='192.168.0.2',
                            ctn_image_name=gobgp_ctn_image_name,
                            log_level=parser_option.gobgp_log_level)

        time.sleep(max(ctn.run() for ctn in [g1, g2]))

        g1.local("gobgp vrf add vrf1 rd 100:100 rt both 100:100")
        g1.local("gobgp vrf add vrf2 rd 200:200 rt both 200:200")
        g2.local("gobgp vrf add vrf1 rd 100:100 rt both 100:100")
        g2.local("gobgp vrf add vrf3 rd 300:300 rt both 300:300")

        g1.local("gobgp vrf vrf1 rib add 10.0.0.0/24")
        g1.local("gobgp vrf vrf2 rib add 10.0.0.0/24")
        g2.local("gobgp vrf vrf1 rib add 20.0.0.0/24")
        g2.local("gobgp vrf vrf3 rib add 20.0.0.0/24")

        cls.g1 = g1
        cls.g2 = g2

    # Test the problem which was reported on #1682
    # https://github.com/osrg/gobgp/issues/1682
    def test_01_neighbor_estabslihed_with_conflict_rtc_config(self):
        self.g1.add_peer(self.g2, vpn=True, passwd='rtc', graceful_restart=True)
        self.g2.add_peer(self.g1, vpn=False, passwd='rtc', graceful_restart=True)
        self.g1.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=self.g2)

    # test each neighbor state is turned establish
    def test_02_neighbor_established(self):
        self.g2.update_peer(self.g1, vpn=True, passwd='rtc', graceful_restart=True)
        self.g1.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=self.g2)

    def test_03_check_adj_rib(self):
        # VRF<#>  g1   g2
        #   1     (*)  (*)
        #   2     (*)
        #   3          (*)
        #
        # ( ): Empty VRF  (*): VRF with a route
        self.assert_adv_count(self.g1, self.g2, 'rtc', 2)
        self.assert_adv_count(self.g1, self.g2, 'ipv4-l3vpn', 1)

        self.assert_adv_count(self.g2, self.g1, 'rtc', 2)
        self.assert_adv_count(self.g2, self.g1, 'ipv4-l3vpn', 1)

    def test_04_add_vrf(self):
        # VRF<#>  g1   g2
        #   1     (*)  (*)
        #   2     (*)
        #   3     ( )  (*)
        self.g1.local("gobgp vrf add vrf3 rd 300:300 rt both 300:300")
        time.sleep(1)

        self.assert_adv_count(self.g1, self.g2, 'rtc', 3)
        self.assert_adv_count(self.g1, self.g2, 'ipv4-l3vpn', 1)

        self.assert_adv_count(self.g2, self.g1, 'rtc', 2)
        self.assert_adv_count(self.g2, self.g1, 'ipv4-l3vpn', 2)

    def test_05_add_route_on_vrf(self):
        # VRF<#>  g1   g2
        #   1     (*)  (*)
        #   2     (*)
        #   3     (*)  (*)
        self.g1.local("gobgp vrf vrf3 rib add 10.0.0.0/24")
        time.sleep(1)

        self.assert_adv_count(self.g1, self.g2, 'rtc', 3)
        self.assert_adv_count(self.g1, self.g2, 'ipv4-l3vpn', 2)

        self.assert_adv_count(self.g2, self.g1, 'rtc', 2)
        self.assert_adv_count(self.g2, self.g1, 'ipv4-l3vpn', 2)

    def test_06_del_route_on_vrf(self):
        # VRF<#>  g1   g2
        #   1     (*)  (*)
        #   2     (*)
        #   3     ( )  (*)
        self.g1.local("gobgp vrf vrf3 rib del 10.0.0.0/24")
        time.sleep(1)

        self.assert_adv_count(self.g1, self.g2, 'rtc', 3)
        self.assert_adv_count(self.g1, self.g2, 'ipv4-l3vpn', 1)

        self.assert_adv_count(self.g2, self.g1, 'rtc', 2)
        self.assert_adv_count(self.g2, self.g1, 'ipv4-l3vpn', 2)

    def test_07_del_vrf(self):
        # VRF<#>  g1   g2
        #   1     (*)  (*)
        #   2     (*)
        #   3          (*)
        self.g1.local("gobgp vrf del vrf3")
        time.sleep(1)

        self.assert_adv_count(self.g1, self.g2, 'rtc', 2)
        self.assert_adv_count(self.g1, self.g2, 'ipv4-l3vpn', 1)

        self.assert_adv_count(self.g2, self.g1, 'rtc', 2)
        self.assert_adv_count(self.g2, self.g1, 'ipv4-l3vpn', 1)

    def test_08_del_vrf_with_route(self):
        # VRF<#>  g1   g2
        #   1          (*)
        #   2     (*)
        #   3          (*)
        self.g1.local("gobgp vrf del vrf1")
        time.sleep(1)

        self.assert_adv_count(self.g1, self.g2, 'rtc', 1)
        self.assert_adv_count(self.g1, self.g2, 'ipv4-l3vpn', 0)

        self.assert_adv_count(self.g2, self.g1, 'rtc', 2)
        self.assert_adv_count(self.g2, self.g1, 'ipv4-l3vpn', 0)

    def test_09_cleanup(self):
        self.g1.local("gobgp vrf del vrf2")
        self.g2.local("gobgp vrf del vrf1")
        self.g2.local("gobgp vrf del vrf3")

    def test_10_rr_setup(self):
        #               +------+
        #               |  g3  |
        #        +------| (RR) |------+
        #        |      +------+      |
        #      (iBGP)              (iBGP)
        #        |                    |
        # +-------------+      +-------------+
        # |     g4      |      |     g5      |
        # | (RR Client) |      | (RR Client) |
        # +-------------+      +-------------+
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

        g3.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=g4)
        g3.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=g5)

        self.__class__.g3 = g3
        self.__class__.g4 = g4
        self.__class__.g5 = g5

    def test_11_rr_check_adj_rib_from_rr(self):
        # VRF<#>  g3   g4   g5
        #   1          ( )  ( )
        #   2
        #   3
        self.g4.local("gobgp vrf add vrf1 rd 100:100 rt both 100:100")
        self.g5.local("gobgp vrf add vrf1 rd 100:100 rt both 100:100")
        time.sleep(1)

        # Check the counts of the sent UPDATE messages in order to detect the
        # infinite RTC UPDATE loop.
        # https://github.com/osrg/gobgp/issues/1630
        self.assert_upd_count(self.g4, self.g3, sent=1, received=2)
        self.assert_upd_count(self.g5, self.g3, sent=1, received=2)

        def check_rtc(client):
            rib = self.g3.get_adj_rib_out(client, rf='rtc')
            self.assertEqual(1, len(rib))
            path = rib[0]
            self.assertEqual(self.g3.peers[client]['local_addr'].split('/')[0], path['nexthop'])
            ids = [attr['value'] for attr in path['attrs'] if attr['type'] == base.BGP_ATTR_TYPE_ORIGINATOR_ID]
            self.assertEqual(1, len(ids))
            self.assertEqual(self.g3.router_id, ids[0])

        check_rtc(self.g4)
        check_rtc(self.g5)

        # VRF<#>  g3   g4   g5
        #   1          (*)  (*)
        #   2
        #   3
        self.g4.local("gobgp vrf vrf1 rib add 40.0.0.0/24")
        self.g5.local("gobgp vrf vrf1 rib add 50.0.0.0/24")
        time.sleep(1)

        def check_ipv4_l3vpn(client):
            rib = self.g3.get_adj_rib_out(client, rf='ipv4-l3vpn')
            self.assertEqual(1, len(rib))
            path = rib[0]
            self.assertNotEqual(self.g3.peers[client]['local_addr'].split('/')[0], path['nexthop'])
            ids = [attr['value'] for attr in path['attrs'] if attr['type'] == base.BGP_ATTR_TYPE_ORIGINATOR_ID]
            self.assertEqual(1, len(ids))
            self.assertNotEqual(client.router_id, ids[0])

        check_ipv4_l3vpn(self.g4)
        check_ipv4_l3vpn(self.g5)

    def test_12_rr_add_vrf(self):
        # VRF<#>  g3   g4   g5
        #   1          (*)  (*)
        #   2          ( )
        #   3
        self.g4.local("gobgp vrf add vrf2 rd 200:200 rt both 200:200")
        time.sleep(1)

        self.assert_adv_count(self.g4, self.g3, 'rtc', 2)
        self.assert_adv_count(self.g4, self.g3, 'ipv4-l3vpn', 1)

        self.assert_adv_count(self.g3, self.g4, 'rtc', 2)
        self.assert_adv_count(self.g3, self.g4, 'ipv4-l3vpn', 1)

        self.assert_adv_count(self.g5, self.g3, 'rtc', 1)
        self.assert_adv_count(self.g5, self.g3, 'ipv4-l3vpn', 1)

        self.assert_adv_count(self.g3, self.g5, 'rtc', 2)
        self.assert_adv_count(self.g3, self.g5, 'ipv4-l3vpn', 1)

    def test_13_rr_add_route_on_vrf(self):
        # VRF<#>  g3   g4   g5
        #   1          (*)  (*)
        #   2          (*)
        #   3
        self.g4.local("gobgp vrf vrf2 rib add 40.0.0.0/24")
        time.sleep(1)

        self.assert_adv_count(self.g4, self.g3, 'rtc', 2)
        self.assert_adv_count(self.g4, self.g3, 'ipv4-l3vpn', 2)

        self.assert_adv_count(self.g3, self.g4, 'rtc', 2)
        self.assert_adv_count(self.g3, self.g4, 'ipv4-l3vpn', 1)

        self.assert_adv_count(self.g5, self.g3, 'rtc', 1)
        self.assert_adv_count(self.g5, self.g3, 'ipv4-l3vpn', 1)

        self.assert_adv_count(self.g3, self.g5, 'rtc', 2)
        self.assert_adv_count(self.g3, self.g5, 'ipv4-l3vpn', 1)

    def test_14_rr_del_vrf_with_route(self):
        # VRF<#>  g3   g4   g5
        #   1               (*)
        #   2          (*)
        #   3
        self.g4.local("gobgp vrf del vrf1")
        time.sleep(1)

        self.assert_adv_count(self.g4, self.g3, 'rtc', 1)
        self.assert_adv_count(self.g4, self.g3, 'ipv4-l3vpn', 1)

        self.assert_adv_count(self.g3, self.g4, 'rtc', 2)
        self.assert_adv_count(self.g3, self.g4, 'ipv4-l3vpn', 0)

        self.assert_adv_count(self.g5, self.g3, 'rtc', 1)
        self.assert_adv_count(self.g5, self.g3, 'ipv4-l3vpn', 1)

        self.assert_adv_count(self.g3, self.g5, 'rtc', 2)
        self.assert_adv_count(self.g3, self.g5, 'ipv4-l3vpn', 0)

    def test_15_rr_cleanup(self):
        self.g4.local("gobgp vrf del vrf2")
        self.g5.local("gobgp vrf del vrf1")

    def test_20_rr_and_non_rr_setup(self):
        # +----------+            +----------+
        # |    g1    |---(iBGP)---|    g2    |
        # | (Non RR  |            | (Non RR  |
        # |  Client) |            |  Client) |
        # +----------+            +----------+
        #      |                        |
        #      +--(iBGP)--+  +--(iBGP)--+
        #                 |  |
        #               +------+
        #               |  g3  |
        #        +------| (RR) |------+
        #        |      +------+      |
        #      (iBGP)              (iBGP)
        #        |                    |
        # +-------------+      +-------------+
        # |     g4      |      |     g5      |
        # | (RR Client) |      | (RR Client) |
        # +-------------+      +-------------+
        self.g3.add_peer(self.g1, vpn=True)
        self.g1.add_peer(self.g3, vpn=True)
        self.g3.add_peer(self.g2, vpn=True)
        self.g2.add_peer(self.g3, vpn=True)

        self.g3.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=self.g1)
        self.g3.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=self.g2)

    def test_21_rr_and_non_rr_add_vrf_on_rr_clients(self):
        # VRF<#>  g1   g2   g3   g4   g5
        #   1                    (*)  (*)
        #   2
        #   3
        self.g4.local("gobgp vrf add vrf1 rd 100:100 rt both 100:100")
        self.g5.local("gobgp vrf add vrf1 rd 100:100 rt both 100:100")
        self.g4.local("gobgp vrf vrf1 rib add 40.0.0.0/24")
        self.g5.local("gobgp vrf vrf1 rib add 50.0.0.0/24")
        time.sleep(1)

        self.assert_adv_count(self.g4, self.g3, 'rtc', 1)
        self.assert_adv_count(self.g4, self.g3, 'ipv4-l3vpn', 1)

        self.assert_adv_count(self.g3, self.g1, 'rtc', 1)
        self.assert_adv_count(self.g3, self.g1, 'ipv4-l3vpn', 0)

        self.assert_adv_count(self.g1, self.g3, 'rtc', 0)
        self.assert_adv_count(self.g1, self.g3, 'ipv4-l3vpn', 0)

        self.assert_adv_count(self.g3, self.g4, 'rtc', 1)
        self.assert_adv_count(self.g3, self.g4, 'ipv4-l3vpn', 1)

    def test_22_rr_and_non_rr_add_vrf_on_non_rr_client(self):
        # VRF<#>  g1   g2   g3   g4   g5
        #   1     (*)            (*)  (*)
        #   2
        #   3
        self.g1.local("gobgp vrf add vrf1 rd 100:100 rt both 100:100")
        self.g1.local("gobgp vrf vrf1 rib add 10.0.0.0/24")
        time.sleep(1)

        self.assert_adv_count(self.g4, self.g3, 'rtc', 1)
        self.assert_adv_count(self.g4, self.g3, 'ipv4-l3vpn', 1)

        self.assert_adv_count(self.g5, self.g3, 'rtc', 1)
        self.assert_adv_count(self.g5, self.g3, 'ipv4-l3vpn', 1)

        self.assert_adv_count(self.g3, self.g1, 'rtc', 1)
        self.assert_adv_count(self.g3, self.g1, 'ipv4-l3vpn', 2)

        self.assert_adv_count(self.g3, self.g2, 'rtc', 1)
        self.assert_adv_count(self.g3, self.g2, 'ipv4-l3vpn', 0)

        self.assert_adv_count(self.g1, self.g3, 'rtc', 1)
        self.assert_adv_count(self.g1, self.g3, 'ipv4-l3vpn', 1)

        self.assert_adv_count(self.g2, self.g3, 'rtc', 0)
        self.assert_adv_count(self.g2, self.g3, 'ipv4-l3vpn', 0)

        self.assert_adv_count(self.g3, self.g4, 'rtc', 1)
        self.assert_adv_count(self.g3, self.g4, 'ipv4-l3vpn', 2)

        self.assert_adv_count(self.g3, self.g5, 'rtc', 1)
        self.assert_adv_count(self.g3, self.g5, 'ipv4-l3vpn', 2)

    def test_23_rr_and_non_rr_add_another_vrf_on_non_rr_client(self):
        # VRF<#>  g1   g2   g3   g4   g5
        #   1     (*)            (*)  (*)
        #   2          (*)
        #   3
        self.g2.local("gobgp vrf add vrf2 rd 200:200 rt both 200:200")
        self.g2.local("gobgp vrf vrf2 rib add 20.0.0.0/24")
        time.sleep(1)

        self.assert_adv_count(self.g4, self.g3, 'rtc', 1)
        self.assert_adv_count(self.g4, self.g3, 'ipv4-l3vpn', 1)

        self.assert_adv_count(self.g5, self.g3, 'rtc', 1)
        self.assert_adv_count(self.g5, self.g3, 'ipv4-l3vpn', 1)

        self.assert_adv_count(self.g3, self.g1, 'rtc', 1)
        self.assert_adv_count(self.g3, self.g1, 'ipv4-l3vpn', 2)

        self.assert_adv_count(self.g3, self.g2, 'rtc', 1)
        self.assert_adv_count(self.g3, self.g2, 'ipv4-l3vpn', 0)

        self.assert_adv_count(self.g1, self.g3, 'rtc', 1)
        self.assert_adv_count(self.g1, self.g3, 'ipv4-l3vpn', 1)

        self.assert_adv_count(self.g2, self.g3, 'rtc', 1)
        self.assert_adv_count(self.g2, self.g3, 'ipv4-l3vpn', 0)

        self.assert_adv_count(self.g3, self.g4, 'rtc', 2)
        self.assert_adv_count(self.g3, self.g4, 'ipv4-l3vpn', 2)

        self.assert_adv_count(self.g3, self.g5, 'rtc', 2)
        self.assert_adv_count(self.g3, self.g5, 'ipv4-l3vpn', 2)

    def test_24_rr_and_non_rr_add_another_vrf_on_rr_client(self):
        # VRF<#>  g1   g2   g3   g4   g5
        #   1     (*)            (*)  (*)
        #   2          (*)       (*)
        #   3
        self.g4.local("gobgp vrf add vrf2 rd 200:200 rt both 200:200")
        self.g4.local("gobgp vrf vrf2 rib add 40.0.0.0/24")
        time.sleep(1)

        self.assert_adv_count(self.g4, self.g3, 'rtc', 2)
        self.assert_adv_count(self.g4, self.g3, 'ipv4-l3vpn', 2)

        self.assert_adv_count(self.g5, self.g3, 'rtc', 1)
        self.assert_adv_count(self.g5, self.g3, 'ipv4-l3vpn', 1)

        self.assert_adv_count(self.g3, self.g1, 'rtc', 2)
        self.assert_adv_count(self.g3, self.g1, 'ipv4-l3vpn', 2)

        self.assert_adv_count(self.g3, self.g2, 'rtc', 2)
        self.assert_adv_count(self.g3, self.g2, 'ipv4-l3vpn', 1)

        self.assert_adv_count(self.g1, self.g3, 'rtc', 1)
        self.assert_adv_count(self.g1, self.g3, 'ipv4-l3vpn', 1)

        self.assert_adv_count(self.g2, self.g3, 'rtc', 1)
        self.assert_adv_count(self.g2, self.g3, 'ipv4-l3vpn', 1)

        self.assert_adv_count(self.g3, self.g4, 'rtc', 2)
        self.assert_adv_count(self.g3, self.g4, 'ipv4-l3vpn', 3)

        self.assert_adv_count(self.g3, self.g5, 'rtc', 2)
        self.assert_adv_count(self.g3, self.g5, 'ipv4-l3vpn', 2)

    def test_25_rr_and_non_rr_clenup(self):
        self.g1.local("gobgp vrf del vrf1")
        self.g2.local("gobgp vrf del vrf2")
        self.g4.local("gobgp vrf del vrf1")
        self.g4.local("gobgp vrf del vrf2")
        self.g5.local("gobgp vrf del vrf1")

    def test_30_ebgp_setup(self):
        # +----+              +----+
        # | g6 |----(eBGP)----| g7 |
        # +----+              +----+
        gobgp_ctn_image_name = parser_option.gobgp_image
        g6 = GoBGPContainer(name='g6', asn=65001, router_id='192.168.0.6',
                            ctn_image_name=gobgp_ctn_image_name,
                            log_level=parser_option.gobgp_log_level)
        g7 = GoBGPContainer(name='g7', asn=65002, router_id='192.168.0.7',
                            ctn_image_name=gobgp_ctn_image_name,
                            log_level=parser_option.gobgp_log_level)

        time.sleep(max(ctn.run() for ctn in [g6, g7]))

        g6.local("gobgp vrf add vrf1 rd 100:100 rt both 100:100")
        g6.local("gobgp vrf add vrf2 rd 200:200 rt both 200:200")
        g7.local("gobgp vrf add vrf1 rd 100:100 rt both 100:100")
        g7.local("gobgp vrf add vrf3 rd 300:300 rt both 300:300")

        g6.local("gobgp vrf vrf1 rib add 60.0.0.0/24")
        g6.local("gobgp vrf vrf2 rib add 60.0.0.0/24")
        g7.local("gobgp vrf vrf1 rib add 70.0.0.0/24")
        g7.local("gobgp vrf vrf3 rib add 70.0.0.0/24")

        for a, b in combinations([g6, g7], 2):
            a.add_peer(b, vpn=True, passwd='rtc', graceful_restart=True)
            b.add_peer(a, vpn=True, passwd='rtc', graceful_restart=True)

        g6.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=g7)

        self.__class__.g6 = g6
        self.__class__.g7 = g7

    def test_31_ebgp_check_adj_rib(self):
        # VRF<#>  g6   g7
        #   1     (*)  (*)
        #   2     (*)
        #   3          (*)
        self.assert_adv_count(self.g6, self.g7, 'rtc', 2)
        self.assert_adv_count(self.g6, self.g7, 'ipv4-l3vpn', 1)

        self.assert_adv_count(self.g7, self.g6, 'rtc', 2)
        self.assert_adv_count(self.g7, self.g6, 'ipv4-l3vpn', 1)

    def test_32_ebgp_add_vrf(self):
        # VRF<#>  g6   g7
        #   1     (*)  (*)
        #   2     (*)
        #   3     ( )  (*)
        self.g6.local("gobgp vrf add vrf3 rd 300:300 rt both 300:300")
        time.sleep(1)

        self.assert_adv_count(self.g6, self.g7, 'rtc', 3)
        self.assert_adv_count(self.g6, self.g7, 'ipv4-l3vpn', 1)

        self.assert_adv_count(self.g7, self.g6, 'rtc', 2)
        self.assert_adv_count(self.g7, self.g6, 'ipv4-l3vpn', 2)

    def test_33_ebgp_add_route_on_vrf(self):
        # VRF<#>  g6   g7
        #   1     (*)  (*)
        #   2     (*)
        #   3     (*)  (*)
        self.g6.local("gobgp vrf vrf3 rib add 60.0.0.0/24")
        time.sleep(1)

        self.assert_adv_count(self.g6, self.g7, 'rtc', 3)
        self.assert_adv_count(self.g6, self.g7, 'ipv4-l3vpn', 2)

        self.assert_adv_count(self.g7, self.g6, 'rtc', 2)
        self.assert_adv_count(self.g7, self.g6, 'ipv4-l3vpn', 2)

    def test_34_ebgp_del_route_on_vrf(self):
        # VRF<#>  g6   g7
        #   1     (*)  (*)
        #   2     (*)
        #   3     ( )  (*)
        self.g6.local("gobgp vrf vrf3 rib del 60.0.0.0/24")
        time.sleep(1)

        self.assert_adv_count(self.g6, self.g7, 'rtc', 3)
        self.assert_adv_count(self.g6, self.g7, 'ipv4-l3vpn', 1)

        self.assert_adv_count(self.g7, self.g6, 'rtc', 2)
        self.assert_adv_count(self.g7, self.g6, 'ipv4-l3vpn', 2)

    def test_35_ebgp_del_vrf(self):
        # VRF<#>  g6   g7
        #   1     (*)  (*)
        #   2     (*)
        #   3          (*)
        self.g6.local("gobgp vrf del vrf3")
        time.sleep(1)

        self.assert_adv_count(self.g6, self.g7, 'rtc', 2)
        self.assert_adv_count(self.g6, self.g7, 'ipv4-l3vpn', 1)

        self.assert_adv_count(self.g7, self.g6, 'rtc', 2)
        self.assert_adv_count(self.g7, self.g6, 'ipv4-l3vpn', 1)

    def test_36_ebgp_del_vrf_with_route(self):
        # VRF<#>  g6   g7
        #   1          (*)
        #   2     (*)
        #   3          (*)
        self.g6.local("gobgp vrf del vrf1")
        time.sleep(1)

        self.assert_adv_count(self.g6, self.g7, 'rtc', 1)
        self.assert_adv_count(self.g6, self.g7, 'ipv4-l3vpn', 0)

        self.assert_adv_count(self.g7, self.g6, 'rtc', 2)
        self.assert_adv_count(self.g7, self.g6, 'ipv4-l3vpn', 0)

    def test_37_ebgp_cleanup(self):
        self.g6.local("gobgp vrf del vrf2")
        self.g7.local("gobgp vrf del vrf1")
        self.g7.local("gobgp vrf del vrf3")

    def test_40_ibgp_ebgp_setup(self):
        # +----------+            +----------+
        # |    g1    |---(iBGP)---|    g2    |
        # | (Non RR  |            | (Non RR  |
        # |  Client) |            |  Client) |
        # +----------+            +----------+
        #      |                        |
        #      +--(iBGP)--+  +--(iBGP)--+
        #                 |  |
        #               +------+                    +----+              +----+
        #               |  g3  |----------(eBGP)----| g6 |----(eBGP)----| g7 |
        #        +------| (RR) |------+             +----+              +----+
        #        |      +------+      |
        #      (iBGP)              (iBGP)
        #        |                    |
        # +-------------+      +-------------+
        # |     g4      |      |     g5      |
        # | (RR Client) |      | (RR Client) |
        # +-------------+      +-------------+
        g1, g2, g3, g4, g5, g6, g7 = (
            self.g1, self.g2, self.g3, self.g4, self.g5, self.g6, self.g7)
        g3.add_peer(g6, vpn=True)
        g6.add_peer(g3, vpn=True)

        g1.local("gobgp vrf add vrf1 rd 100:100 rt both 100:100")
        g2.local("gobgp vrf add vrf2 rd 200:200 rt both 200:200")
        # g3 (RR)
        g4.local("gobgp vrf add vrf1 rd 100:100 rt both 100:100")
        g5.local("gobgp vrf add vrf2 rd 200:200 rt both 200:200")
        g6.local("gobgp vrf add vrf1 rd 100:100 rt both 100:100")
        g7.local("gobgp vrf add vrf3 rd 300:300 rt both 300:300")

        g1.local("gobgp vrf vrf1 rib add 10.0.0.0/24")
        g2.local("gobgp vrf vrf2 rib add 20.0.0.0/24")
        # g3 (RR)
        g4.local("gobgp vrf vrf1 rib add 40.0.0.0/24")
        g5.local("gobgp vrf vrf2 rib add 50.0.0.0/24")
        g6.local("gobgp vrf vrf1 rib add 60.0.0.0/24")
        g7.local("gobgp vrf vrf3 rib add 70.0.0.0/24")

        g3.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=g6)

    def test_41_ibgp_ebgp_check_adj_rib(self):
        # VRF<#>  g1   g2   g3   g4   g5  | g6   g7
        #   1     (*)            (*)      | (*)
        #   2          (*)            (*) |
        #   3                             |      (*)
        self.assert_adv_count(self.g3, self.g6, 'rtc', 2)
        self.assert_adv_count(self.g3, self.g6, 'ipv4-l3vpn', 2)

        self.assert_adv_count(self.g6, self.g3, 'rtc', 2)
        self.assert_adv_count(self.g6, self.g3, 'ipv4-l3vpn', 1)

    def test_42_01_ibgp_ebgp_add_vrf_on_rr_client(self):
        # VRF<#>  g1   g2   g3   g4   g5  | g6   g7
        #   1     (*)            (*)      | (*)
        #   2          (*)            (*) |
        #   3                         ( ) |      (*)
        self.g5.local("gobgp vrf add vrf3 rd 300:300 rt both 300:300")
        time.sleep(1)

        self.assert_adv_count(self.g3, self.g6, 'rtc', 3)
        self.assert_adv_count(self.g3, self.g6, 'ipv4-l3vpn', 2)

        self.assert_adv_count(self.g6, self.g3, 'rtc', 2)
        self.assert_adv_count(self.g6, self.g3, 'ipv4-l3vpn', 2)

    def test_42_02_ibgp_ebgp_add_route_on_vrf_on_rr_client(self):
        # VRF<#>  g1   g2   g3   g4   g5  | g6   g7
        #   1     (*)            (*)      | (*)
        #   2          (*)            (*) |
        #   3                         (*) |      (*)
        self.g5.local("gobgp vrf vrf3 rib add 50.0.0.0/24")
        time.sleep(1)

        self.assert_adv_count(self.g3, self.g6, 'rtc', 3)
        self.assert_adv_count(self.g3, self.g6, 'ipv4-l3vpn', 3)

        self.assert_adv_count(self.g6, self.g3, 'rtc', 2)
        self.assert_adv_count(self.g6, self.g3, 'ipv4-l3vpn', 2)

    def test_42_03_ibgp_ebgp_del_route_on_vrf_on_rr_client(self):
        # VRF<#>  g1   g2   g3   g4   g5  | g6   g7
        #   1     (*)            (*)      | (*)
        #   2          (*)            (*) |
        #   3                         ( ) |      (*)
        self.g5.local("gobgp vrf vrf3 rib del 50.0.0.0/24")
        time.sleep(1)

        self.assert_adv_count(self.g3, self.g6, 'rtc', 3)
        self.assert_adv_count(self.g3, self.g6, 'ipv4-l3vpn', 2)

        self.assert_adv_count(self.g6, self.g3, 'rtc', 2)
        self.assert_adv_count(self.g6, self.g3, 'ipv4-l3vpn', 2)

    def test_42_04_ibgp_ebgp_del_vrf_on_rr_client(self):
        # VRF<#>  g1   g2   g3   g4   g5  | g6   g7
        #   1     (*)            (*)      | (*)
        #   2          (*)            (*) |
        #   3                             |      (*)
        self.g5.local("gobgp vrf del vrf3")
        time.sleep(2)

        self.assert_adv_count(self.g3, self.g6, 'rtc', 2)
        self.assert_adv_count(self.g3, self.g6, 'ipv4-l3vpn', 2)

        self.assert_adv_count(self.g6, self.g3, 'rtc', 2)
        self.assert_adv_count(self.g6, self.g3, 'ipv4-l3vpn', 1)

    def test_43_01_ibgp_ebgp_add_vrf_on_non_rr_client(self):
        # VRF<#>  g1   g2   g3   g4   g5  | g6   g7
        #   1     (*)            (*)      | (*)
        #   2          (*)            (*) |
        #   3     ( )                     |      (*)
        self.g1.local("gobgp vrf add vrf3 rd 300:300 rt both 300:300")
        time.sleep(1)

        self.assert_adv_count(self.g3, self.g6, 'rtc', 3)
        self.assert_adv_count(self.g3, self.g6, 'ipv4-l3vpn', 2)

        self.assert_adv_count(self.g6, self.g3, 'rtc', 2)
        self.assert_adv_count(self.g6, self.g3, 'ipv4-l3vpn', 2)

    def test_43_02_ibgp_ebgp_add_route_on_vrf_on_non_rr_client(self):
        # VRF<#>  g1   g2   g3   g4   g5  | g6   g7
        #   1     (*)            (*)      | (*)
        #   2          (*)            (*) |
        #   3     (*)                     |      (*)
        self.g1.local("gobgp vrf vrf3 rib add 10.0.0.0/24")
        time.sleep(1)

        self.assert_adv_count(self.g3, self.g6, 'rtc', 3)
        self.assert_adv_count(self.g3, self.g6, 'ipv4-l3vpn', 3)

        self.assert_adv_count(self.g6, self.g3, 'rtc', 2)
        self.assert_adv_count(self.g6, self.g3, 'ipv4-l3vpn', 2)

    def test_43_03_ibgp_ebgp_del_route_on_vrf_on_non_rr_client(self):
        # VRF<#>  g1   g2   g3   g4   g5  | g6   g7
        #   1     (*)            (*)      | (*)
        #   2          (*)            (*) |
        #   3     ( )                     |      (*)
        self.g1.local("gobgp vrf vrf3 rib del 10.0.0.0/24")
        time.sleep(1)

        self.assert_adv_count(self.g3, self.g6, 'rtc', 3)
        self.assert_adv_count(self.g3, self.g6, 'ipv4-l3vpn', 2)

        self.assert_adv_count(self.g6, self.g3, 'rtc', 2)
        self.assert_adv_count(self.g6, self.g3, 'ipv4-l3vpn', 2)

    def test_43_04_ibgp_ebgp_del_vrf_on_non_rr_client(self):
        # VRF<#>  g1   g2   g3   g4   g5  | g6   g7
        #   1     (*)            (*)      | (*)
        #   2          (*)            (*) |
        #   3                             |      (*)
        self.g1.local("gobgp vrf del vrf3")
        time.sleep(2)

        self.assert_adv_count(self.g3, self.g6, 'rtc', 2)
        self.assert_adv_count(self.g3, self.g6, 'ipv4-l3vpn', 2)

        self.assert_adv_count(self.g6, self.g3, 'rtc', 2)
        self.assert_adv_count(self.g6, self.g3, 'ipv4-l3vpn', 1)

    def test_44_01_ibgp_ebgp_add_vrf_on_ebgp_peer(self):
        # VRF<#>  g1   g2   g3   g4   g5  | g6   g7
        #   1     (*)            (*)      | (*)
        #   2          (*)            (*) |      ( )
        #   3                             |      (*)
        self.g7.local("gobgp vrf add vrf2 rd 200:200 rt both 200:200")
        time.sleep(1)

        self.assert_adv_count(self.g3, self.g6, 'rtc', 2)
        self.assert_adv_count(self.g3, self.g6, 'ipv4-l3vpn', 4)

        self.assert_adv_count(self.g6, self.g3, 'rtc', 3)
        self.assert_adv_count(self.g6, self.g3, 'ipv4-l3vpn', 1)

    def test_44_02_ibgp_ebgp_add_route_on_ebgp_peer(self):
        # VRF<#>  g1   g2   g3   g4   g5  | g6   g7
        #   1     (*)            (*)      | (*)
        #   2          (*)            (*) |      (*)
        #   3                             |      (*)
        self.g7.local("gobgp vrf vrf2 rib add 70.0.0.0/24")
        time.sleep(1)

        self.assert_adv_count(self.g3, self.g6, 'rtc', 2)
        self.assert_adv_count(self.g3, self.g6, 'ipv4-l3vpn', 4)

        self.assert_adv_count(self.g6, self.g3, 'rtc', 3)
        self.assert_adv_count(self.g6, self.g3, 'ipv4-l3vpn', 2)

    def test_44_03_ibgp_ebgp_del_route_on_vrf_on_ebgp_peer(self):
        # VRF<#>  g1   g2   g3   g4   g5  | g6   g7
        #   1     (*)            (*)      | (*)
        #   2          (*)            (*) |      ( )
        #   3                             |      (*)
        self.g7.local("gobgp vrf vrf2 rib del 70.0.0.0/24")
        time.sleep(1)

        self.assert_adv_count(self.g3, self.g6, 'rtc', 2)
        self.assert_adv_count(self.g3, self.g6, 'ipv4-l3vpn', 4)

        self.assert_adv_count(self.g6, self.g3, 'rtc', 3)
        self.assert_adv_count(self.g6, self.g3, 'ipv4-l3vpn', 1)

    def test_44_04_ibgp_ebgp_del_vrf_on_ebgp_peer(self):
        # VRF<#>  g1   g2   g3   g4   g5  | g6   g7
        #   1     (*)            (*)      | (*)
        #   2          (*)            (*) |
        #   3                             |      (*)
        self.g7.local("gobgp vrf del vrf2")
        time.sleep(2)

        self.assert_adv_count(self.g3, self.g6, 'rtc', 2)
        self.assert_adv_count(self.g3, self.g6, 'ipv4-l3vpn', 2)

        self.assert_adv_count(self.g6, self.g3, 'rtc', 2)
        self.assert_adv_count(self.g6, self.g3, 'ipv4-l3vpn', 1)

    def test_45_ibgp_ebgp_cleanup(self):
        self.g1.local("gobgp vrf del vrf1")
        self.g2.local("gobgp vrf del vrf2")
        self.g4.local("gobgp vrf del vrf1")
        self.g5.local("gobgp vrf del vrf2")
        self.g6.local("gobgp vrf del vrf1")
        self.g7.local("gobgp vrf del vrf3")

    def test_50_rr_addpath_setup(self):
        # Test cases for the infinite RTC UPDATE message loop:
        # https://github.com/osrg/gobgp/issues/1688#issuecomment-391456615
        # https://github.com/osrg/gobgp/pull/1703

        #               +------+
        #               |  g3  |
        #        +------| (RR) |------+
        #        |      +------+      |
        # (iBGP+Add-Paths)     (iBGP+Add-Paths)
        #        |                    |
        # +-------------+      +-------------+
        # |     g4      |      |     g5      |
        # | (RR Client) |      | (RR Client) |
        # +-------------+      +-------------+
        g3, g4, g5 = self.g3, self.g4, self.g5

        g3.update_peer(g4, vpn=True, addpath=True, is_rr_client=True)
        g4.update_peer(g3, vpn=True, addpath=True)

        g3.update_peer(g5, vpn=True, addpath=True, is_rr_client=True)
        g5.update_peer(g3, vpn=True, addpath=True)

        g3.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=g4)
        g3.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=g5)

    def test_51_rr_addpath_check_adj_rib_from_rr(self):
        # VRF<#>  g3   g4   g5
        #   1          ( )  ( )
        #   2
        #   3
        self.g4.local("gobgp vrf add vrf1 rd 100:100 rt both 100:100")
        self.g5.local("gobgp vrf add vrf1 rd 100:100 rt both 100:100")
        time.sleep(1)

        # Check the counts of the sent UPDATE messages in order to detect the
        # infinite RTC UPDATE loop.
        self.assert_upd_count(self.g4, self.g3, sent=1, received=2)
        self.assert_upd_count(self.g5, self.g3, sent=1, received=2)

        def check_rtc(client):
            rib = self.g3.get_adj_rib_out(client, rf='rtc')
            self.assertEqual(1, len(rib))
            path = rib[0]
            self.assertEqual(self.g3.peers[client]['local_addr'].split('/')[0], path['nexthop'])
            ids = [attr['value'] for attr in path['attrs'] if attr['type'] == base.BGP_ATTR_TYPE_ORIGINATOR_ID]
            self.assertEqual(1, len(ids))
            self.assertEqual(self.g3.router_id, ids[0])

        check_rtc(self.g4)
        check_rtc(self.g5)

        # VRF<#>  g3   g4   g5
        #   1          (*)  (*)
        #   2
        #   3
        self.g4.local("gobgp vrf vrf1 rib add 40.0.0.0/24")
        self.g5.local("gobgp vrf vrf1 rib add 50.0.0.0/24")
        time.sleep(1)

        def check_ipv4_l3vpn(client):
            rib = self.g3.get_adj_rib_out(client, rf='ipv4-l3vpn')
            self.assertEqual(1, len(rib))
            path = rib[0]
            self.assertNotEqual(self.g3.peers[client]['local_addr'].split('/')[0], path['nexthop'])
            ids = [attr['value'] for attr in path['attrs'] if attr['type'] == base.BGP_ATTR_TYPE_ORIGINATOR_ID]
            self.assertEqual(1, len(ids))
            self.assertNotEqual(client.router_id, ids[0])

        check_ipv4_l3vpn(self.g4)
        check_ipv4_l3vpn(self.g5)

    def test_52_rr_addpath_add_vrf(self):
        # VRF<#>  g3   g4   g5
        #   1          (*)  (*)
        #   2          ( )
        #   3
        self.g4.local("gobgp vrf add vrf2 rd 200:200 rt both 200:200")
        time.sleep(1)

        self.assert_adv_count(self.g4, self.g3, 'rtc', 2)
        self.assert_adv_count(self.g4, self.g3, 'ipv4-l3vpn', 1)

        self.assert_adv_count(self.g3, self.g4, 'rtc', 2)
        self.assert_adv_count(self.g3, self.g4, 'ipv4-l3vpn', 1)

        self.assert_adv_count(self.g5, self.g3, 'rtc', 1)
        self.assert_adv_count(self.g5, self.g3, 'ipv4-l3vpn', 1)

        self.assert_adv_count(self.g3, self.g5, 'rtc', 2)
        self.assert_adv_count(self.g3, self.g5, 'ipv4-l3vpn', 1)

    def test_53_rr_addpath_add_route_on_vrf(self):
        # VRF<#>  g3   g4   g5
        #   1          (*)  (*)
        #   2          (*)
        #   3
        self.g4.local("gobgp vrf vrf2 rib add 40.0.0.0/24")
        time.sleep(1)

        self.assert_adv_count(self.g4, self.g3, 'rtc', 2)
        self.assert_adv_count(self.g4, self.g3, 'ipv4-l3vpn', 2)

        self.assert_adv_count(self.g3, self.g4, 'rtc', 2)
        self.assert_adv_count(self.g3, self.g4, 'ipv4-l3vpn', 1)

        self.assert_adv_count(self.g5, self.g3, 'rtc', 1)
        self.assert_adv_count(self.g5, self.g3, 'ipv4-l3vpn', 1)

        self.assert_adv_count(self.g3, self.g5, 'rtc', 2)
        self.assert_adv_count(self.g3, self.g5, 'ipv4-l3vpn', 1)

    def test_54_rr_addpath_del_route_on_vrf(self):
        # Related issue:
        # https://github.com/osrg/gobgp/issues/1688

        # VRF<#>  g3   g4   g5
        #   1          ( )  (*)
        #   2          (*)
        #   3
        self.g4.local("gobgp vrf vrf1 rib del 40.0.0.0/24")
        time.sleep(1)

        self.assert_adv_count(self.g4, self.g3, 'rtc', 2)
        self.assert_adv_count(self.g4, self.g3, 'ipv4-l3vpn', 1)

        self.assert_adv_count(self.g3, self.g4, 'rtc', 2)
        self.assert_adv_count(self.g3, self.g4, 'ipv4-l3vpn', 1)

        self.assert_adv_count(self.g5, self.g3, 'rtc', 1)
        self.assert_adv_count(self.g5, self.g3, 'ipv4-l3vpn', 1)

        self.assert_adv_count(self.g3, self.g5, 'rtc', 2)
        self.assert_adv_count(self.g3, self.g5, 'ipv4-l3vpn', 0)

    def test_55_rr_addpath_del_vrf_with_route(self):
        # VRF<#>  g3   g4   g5
        #   1               (*)
        #   2          (*)
        #   3
        self.g4.local("gobgp vrf del vrf1")
        time.sleep(1)

        self.assert_adv_count(self.g4, self.g3, 'rtc', 1)
        self.assert_adv_count(self.g4, self.g3, 'ipv4-l3vpn', 1)

        self.assert_adv_count(self.g3, self.g4, 'rtc', 2)
        self.assert_adv_count(self.g3, self.g4, 'ipv4-l3vpn', 0)

        self.assert_adv_count(self.g5, self.g3, 'rtc', 1)
        self.assert_adv_count(self.g5, self.g3, 'ipv4-l3vpn', 1)

        self.assert_adv_count(self.g3, self.g5, 'rtc', 2)
        self.assert_adv_count(self.g3, self.g5, 'ipv4-l3vpn', 0)

    def test_56_rr_addpath_cleanup(self):
        self.g4.local("gobgp vrf del vrf2")
        self.g5.local("gobgp vrf del vrf1")


if __name__ == '__main__':
    output = local("which docker 2>&1 > /dev/null ; echo $?", capture=True)
    if int(output) is not 0:
        print "docker not found"
        sys.exit(1)

    nose.main(argv=sys.argv, addplugins=[OptionParser()],
              defaultTest=sys.argv[0])
