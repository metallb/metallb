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

import sys
import time
import unittest

import nose
from fabric.api import local

from lib import base
from lib.base import BGP_FSM_ESTABLISHED
from lib.gobgp import GoBGPContainer
from lib.exabgp import ExaBGPContainer
from lib.noseplugin import OptionParser, parser_option


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
        g3 = GoBGPContainer(name='g3', asn=65000, router_id='192.168.0.3',
                            ctn_image_name=gobgp_ctn_image_name,
                            log_level=parser_option.gobgp_log_level)
        e1 = ExaBGPContainer(name='e1', asn=65000, router_id='192.168.0.4')

        ctns = [g1, g2, g3, e1]
        initial_wait_time = max(ctn.run() for ctn in ctns)

        time.sleep(initial_wait_time)

        g1.add_peer(e1, addpath=True)
        e1.add_peer(g1, addpath=True)

        g1.add_peer(g2, addpath=False, is_rr_client=True)
        g2.add_peer(g1, addpath=False)

        g1.add_peer(g3, addpath=True, is_rr_client=True)
        g3.add_peer(g1, addpath=True)

        cls.g1 = g1
        cls.g2 = g2
        cls.g3 = g3
        cls.e1 = e1

    # test each neighbor state is turned establish
    def test_00_neighbor_established(self):
        self.g1.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=self.g2)
        self.g1.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=self.e1)

    # prepare routes with path_id (no error check)
    def test_01_prepare_add_paths_routes(self):
        self.e1.add_route(route='192.168.100.0/24', identifier=10, aspath=[100, 200, 300])
        self.e1.add_route(route='192.168.100.0/24', identifier=20, aspath=[100, 200])
        self.e1.add_route(route='192.168.100.0/24', identifier=30, aspath=[100])
        time.sleep(1)  # XXX: wait for routes re-calculated and advertised

    # test three routes are installed to the rib due to add-path feature
    def test_02_check_g1_global_rib(self):
        rib = self.g1.get_global_rib()
        self.assertEqual(len(rib), 1)
        self.assertEqual(len(rib[0]['paths']), 3)

    # test only the best path is advertised to g2
    def test_03_check_g2_global_rib(self):
        rib = self.g2.get_global_rib()
        self.assertEqual(len(rib), 1)
        self.assertEqual(len(rib[0]['paths']), 1)
        self.assertEqual(rib[0]['paths'][0]['aspath'], [100])

    # test three routes are advertised to g3
    def test_04_check_g3_global_rib(self):
        rib = self.g3.get_global_rib()
        self.assertEqual(len(rib), 1)
        self.assertEqual(len(rib[0]['paths']), 3)

    # withdraw a route with path_id (no error check)
    def test_05_withdraw_route_with_path_id(self):
        self.e1.del_route(route='192.168.100.0/24', identifier=30)
        time.sleep(1)  # XXX: wait for routes re-calculated and advertised

    # test the withdrawn route is removed from the rib
    def test_06_check_g1_global_rib(self):
        rib = self.g1.get_global_rib()
        self.assertEqual(len(rib), 1)
        self.assertEqual(len(rib[0]['paths']), 2)
        for path in rib[0]['paths']:
            self.assertTrue(path['aspath'] == [100, 200, 300] or
                            path['aspath'] == [100, 200])

    # test the best path is replaced due to the removal from g1 rib
    def test_07_check_g2_global_rib(self):
        rib = self.g2.get_global_rib()
        self.assertEqual(len(rib), 1)
        self.assertEqual(len(rib[0]['paths']), 1)
        self.assertEqual(rib[0]['paths'][0]['aspath'], [100, 200])

    # test the withdrawn route is removed from the rib of g3
    def test_08_check_g3_global_rib(self):
        rib = self.g3.get_global_rib()
        self.assertEqual(len(rib), 1)
        self.assertEqual(len(rib[0]['paths']), 2)
        for path in rib[0]['paths']:
            self.assertTrue(path['aspath'] == [100, 200, 300] or
                            path['aspath'] == [100, 200])

    # install a route with path_id via GoBGP CLI (no error check)
    def test_09_install_add_paths_route_via_cli(self):
        # identifier is duplicated with the identifier of the route from e1
        self.g1.add_route(route='192.168.100.0/24', identifier=10, local_pref=500)
        time.sleep(1)  # XXX: wait for routes re-calculated and advertised

    # test the route from CLI is installed to the rib
    def test_10_check_g1_global_rib(self):
        rib = self.g1.get_global_rib()
        self.assertEqual(len(rib), 1)
        self.assertEqual(len(rib[0]['paths']), 3)
        for path in rib[0]['paths']:
            self.assertTrue(path['aspath'] == [100, 200, 300] or
                            path['aspath'] == [100, 200] or
                            path['aspath'] == [])
            if len(path['aspath']) == 0:
                self.assertEqual(path['local-pref'], 500)

    # test the best path is replaced due to the CLI route from g1 rib
    def test_11_check_g2_global_rib(self):
        rib = self.g2.get_global_rib()
        self.assertEqual(len(rib), 1)
        self.assertEqual(len(rib[0]['paths']), 1)
        self.assertEqual(rib[0]['paths'][0]['aspath'], [])

    # test the route from CLI is advertised from g1
    def test_12_check_g3_global_rib(self):
        rib = self.g3.get_global_rib()
        self.assertEqual(len(rib), 1)
        self.assertEqual(len(rib[0]['paths']), 3)
        for path in rib[0]['paths']:
            self.assertTrue(path['aspath'] == [100, 200, 300] or
                            path['aspath'] == [100, 200] or
                            path['aspath'] == [])
            if len(path['aspath']) == 0:
                self.assertEqual(path['local-pref'], 500)

    # remove non-existing route with path_id via GoBGP CLI (no error check)
    def test_13_remove_non_existing_add_paths_route_via_cli(self):
        # specify locally non-existing identifier which has the same value
        # with the identifier of the route from e1
        self.g1.del_route(route='192.168.100.0/24', identifier=20)
        time.sleep(1)  # XXX: wait for routes re-calculated and advertised

    # test none of route is removed by non-existing path_id via CLI
    def test_14_check_g1_global_rib(self):
        rib = self.g1.get_global_rib()
        self.assertEqual(len(rib), 1)
        self.assertEqual(len(rib[0]['paths']), 3)
        for path in rib[0]['paths']:
            self.assertTrue(path['aspath'] == [100, 200, 300] or
                            path['aspath'] == [100, 200] or
                            path['aspath'] == [])
            if len(path['aspath']) == 0:
                self.assertEqual(path['local-pref'], 500)

    # remove route with path_id via GoBGP CLI (no error check)
    def test_15_remove_add_paths_route_via_cli(self):
        self.g1.del_route(route='192.168.100.0/24', identifier=10)
        time.sleep(1)  # XXX: wait for routes re-calculated and advertised

    # test the route is removed from the rib via CLI
    def test_16_check_g1_global_rib(self):
        rib = self.g1.get_global_rib()
        self.assertEqual(len(rib), 1)
        self.assertEqual(len(rib[0]['paths']), 2)
        for path in rib[0]['paths']:
            self.assertTrue(path['aspath'] == [100, 200, 300] or
                            path['aspath'] == [100, 200])

    # test the best path is replaced the removal from g1 rib
    def test_17_check_g2_global_rib(self):
        rib = self.g2.get_global_rib()
        self.assertEqual(len(rib), 1)
        self.assertEqual(len(rib[0]['paths']), 1)
        self.assertEqual(rib[0]['paths'][0]['aspath'], [100, 200])

    # test the removed route from CLI is withdrawn by g1
    def test_18_check_g3_global_rib(self):
        rib = self.g3.get_global_rib()
        self.assertEqual(len(rib), 1)
        self.assertEqual(len(rib[0]['paths']), 2)
        for path in rib[0]['paths']:
            self.assertTrue(path['aspath'] == [100, 200, 300] or
                            path['aspath'] == [100, 200])


if __name__ == '__main__':
    output = local("which docker 2>&1 > /dev/null ; echo $?", capture=True)
    if int(output) is not 0:
        print "docker not found"
        sys.exit(1)

    nose.main(argv=sys.argv, addplugins=[OptionParser()],
              defaultTest=sys.argv[0])
