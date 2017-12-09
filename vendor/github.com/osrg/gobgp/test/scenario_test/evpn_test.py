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
from lib.base import (
    BGP_FSM_ESTABLISHED,
    BGP_ATTR_TYPE_EXTENDED_COMMUNITIES,
)
from lib.gobgp import GoBGPContainer


def get_mac_mobility_sequence(pattr):
    for ecs in [
            p['value'] for p in pattr
            if 'type' in p and p['type'] == BGP_ATTR_TYPE_EXTENDED_COMMUNITIES]:
        for ec in [e for e in ecs if 'type' in e and e['type'] == 6]:
            if ec['subtype'] == 0:
                if 'sequence' not in ec:
                    return 0
                else:
                    return ec['sequence']
    return -1


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
        ctns = [g1, g2]

        initial_wait_time = max(ctn.run() for ctn in ctns)

        time.sleep(initial_wait_time)
        g1.local("gobgp vrf add vrf1 rd 10:10 rt both 10:10")
        g2.local("gobgp vrf add vrf1 rd 10:10 rt both 10:10")

        for a, b in combinations(ctns, 2):
            a.add_peer(b, vpn=True, passwd='evpn')
            b.add_peer(a, vpn=True, passwd='evpn')

        cls.g1 = g1
        cls.g2 = g2

    # test each neighbor state is turned establish
    def test_01_neighbor_established(self):
        self.g1.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=self.g2)

    def test_02_add_evpn_route(self):
        self.g1.local('gobgp global rib add '
                      '-a evpn macadv 11:22:33:44:55:66 10.0.0.1 1000 1000 '
                      'rd 10:10 rt 10:10')
        grib = self.g1.get_global_rib(rf='evpn')
        self.assertTrue(len(grib) == 1)
        dst = grib[0]
        self.assertTrue(len(dst['paths']) == 1)
        path = dst['paths'][0]
        self.assertTrue(path['nexthop'] == '0.0.0.0')

        interval = 1
        timeout = int(30 / interval)
        done = False
        for _ in range(timeout):
            if done:
                break
            grib = self.g2.get_global_rib(rf='evpn')

            if len(grib) < 1:
                time.sleep(interval)
                continue

            self.assertTrue(len(grib) == 1)
            dst = grib[0]
            self.assertTrue(len(dst['paths']) == 1)
            path = dst['paths'][0]
            n_addrs = [i[1].split('/')[0] for i in self.g1.ip_addrs]
            self.assertTrue(path['nexthop'] in n_addrs)
            done = True

    def test_03_check_mac_mobility(self):
        self.g2.local('gobgp global rib add '
                      '-a evpn macadv 11:22:33:44:55:66 10.0.0.1 1000 1000 '
                      'rd 10:20 rt 10:10')

        time.sleep(3)

        grib = self.g1.get_global_rib(rf='evpn')
        self.assertTrue(len(grib) == 1)
        dst = grib[0]
        self.assertTrue(len(dst['paths']) == 1)
        path = dst['paths'][0]
        n_addrs = [i[1].split('/')[0] for i in self.g2.ip_addrs]
        self.assertTrue(path['nexthop'] in n_addrs)
        self.assertTrue(get_mac_mobility_sequence(path['attrs']) == 0)

    def test_04_check_mac_mobility_again(self):
        self.g1.local('gobgp global rib add '
                      '-a evpn macadv 11:22:33:44:55:66 10.0.0.1 1000 1000 '
                      'rd 10:20 rt 10:10')

        time.sleep(3)

        grib = self.g2.get_global_rib(rf='evpn')
        self.assertTrue(len(grib) == 1)
        dst = grib[0]
        self.assertTrue(len(dst['paths']) == 1)
        path = dst['paths'][0]
        n_addrs = [i[1].split('/')[0] for i in self.g1.ip_addrs]
        self.assertTrue(path['nexthop'] in n_addrs)
        self.assertTrue(get_mac_mobility_sequence(path['attrs']) == 1)


if __name__ == '__main__':
    output = local("which docker 2>&1 > /dev/null ; echo $?", capture=True)
    if int(output) is not 0:
        print "docker not found"
        sys.exit(1)

    nose.main(argv=sys.argv, addplugins=[OptionParser()],
              defaultTest=sys.argv[0])
