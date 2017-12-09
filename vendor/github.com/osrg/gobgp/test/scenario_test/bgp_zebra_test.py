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

import sys
import time
import unittest

from fabric.api import local
import nose

from lib.noseplugin import OptionParser, parser_option

from lib import base
from lib.base import (
    Bridge,
    BGP_FSM_ESTABLISHED,
)
from lib.gobgp import GoBGPContainer
from lib.quagga import QuaggaBGPContainer


class GoBGPTestBase(unittest.TestCase):

    @classmethod
    def setUpClass(cls):
        gobgp_ctn_image_name = parser_option.gobgp_image
        base.TEST_PREFIX = parser_option.test_prefix

        # preparing the container for ipv4
        g1_v4 = GoBGPContainer(name='g1_v4', asn=65000, router_id='192.168.0.1',
                               ctn_image_name=gobgp_ctn_image_name,
                               log_level=parser_option.gobgp_log_level,
                               zebra=True)
        q1_v4 = QuaggaBGPContainer(name='q1_v4', asn=65001, router_id='192.168.0.2', zebra=True)
        o1_v4 = QuaggaBGPContainer(name='o1_v4', asn=65002, router_id='192.168.0.3')
        o2_v4 = QuaggaBGPContainer(name='o2_v4', asn=65002, router_id='192.168.0.4')

        # preparing the container for ipv6
        g1_v6 = GoBGPContainer(name='g1_v6', asn=65000, router_id='192.168.0.1',
                               ctn_image_name=gobgp_ctn_image_name,
                               log_level=parser_option.gobgp_log_level,
                               zebra=True)
        q1_v6 = QuaggaBGPContainer(name='q1_v6', asn=65001, router_id='192.168.0.2', zebra=True)
        o1_v6 = QuaggaBGPContainer(name='o1_v6', asn=65002, router_id='192.168.0.3')
        o2_v6 = QuaggaBGPContainer(name='o2_v6', asn=65002, router_id='192.168.0.4')

        # preparing the bridge for ipv4
        br01_v4 = Bridge(name='br01_v4', subnet='192.168.10.0/24')
        br02_v4 = Bridge(name='br02_v4', subnet='192.168.20.0/24')
        br03_v4 = Bridge(name='br03_v4', subnet='192.168.30.0/24')

        # preparing the bridge for ipv6
        br01_v6 = Bridge(name='br01_v6', subnet='2001:10::/32')
        br02_v6 = Bridge(name='br02_v6', subnet='2001:20::/32')
        br03_v6 = Bridge(name='br03_v6', subnet='2001:30::/32')

        cls.ctns = {'ipv4': [g1_v4, q1_v4, o1_v4, o2_v4],
                    'ipv6': [g1_v6, q1_v6, o1_v6, o2_v6]}
        cls.gobgps = {'ipv4': g1_v4, 'ipv6': g1_v6}
        cls.quaggas = {'ipv4': q1_v4, 'ipv6': q1_v6}
        cls.others = {'ipv4': [o1_v4, o2_v4], 'ipv6': [o1_v6, o2_v6]}
        cls.bridges = {
            'br01_v4': br01_v4,
            'br02_v4': br02_v4,
            'br03_v4': br03_v4,
            'br01_v6': br01_v6,
            'br02_v6': br02_v6,
            'br03_v6': br03_v6,
        }

    """
      No.1 start up ipv4 containers and check state
           each neighbor is established in ipv4 environment
    """

    def test_01_check_neighbor_established(self):
        g1 = self.gobgps['ipv4']
        q1 = self.quaggas['ipv4']
        o1 = self.others['ipv4'][0]
        o2 = self.others['ipv4'][1]
        # start up containers of ipv4 environment
        initial_wait_time = max(ctn.run() for ctn in self.ctns['ipv4'])
        time.sleep(initial_wait_time)

        # make ipv4 bridge and set ip to each container
        [self.bridges['br01_v4'].addif(ctn) for ctn in [o1, g1]]
        [self.bridges['br02_v4'].addif(ctn) for ctn in [g1, q1]]
        [self.bridges['br03_v4'].addif(ctn) for ctn in [q1, o2]]

        g1.add_peer(q1, bridge=self.bridges['br02_v4'].name)
        q1.add_peer(g1, bridge=self.bridges['br02_v4'].name)

        g1.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=q1)

    """
      No.2 check whether the ping is reachable in container
           that have previously beyond the gobpg in ipv4 environment
    """

    def test_02_check_reachablily_beyond_gobgp_from_quagga(self):
        g1 = self.gobgps['ipv4']
        q1 = self.quaggas['ipv4']
        o1 = self.others['ipv4'][0]

        next_hop = None
        for info in g1.ip_addrs:
            if 'br01_v4' in info[2]:
                next_hop = info[1].split('/')[0]
        self.assertFalse(next_hop is None)
        o1.add_static_route(self.bridges['br02_v4'].subnet, next_hop)
        addr = [e[1] for e in o1.ip_addrs if 'br01_v4' in e[2]]
        self.assertTrue(len(addr) == 1)
        q1.get_reachablily(addr[0])

    """
      No.3 check whether the ping is reachable in container
           that have previously beyond the quagga in ipv4 environment
    """

    def test_03_check_reachablily_beyond_quagga_from_gobgp(self):
        g1 = self.gobgps['ipv4']
        q1 = self.quaggas['ipv4']
        o2 = self.others['ipv4'][1]

        next_hop = q1.ip_addrs[2][1].split('/')[0]
        o2.add_static_route(self.bridges['br02_v4'].subnet, next_hop)
        addr = [e[1] for e in o2.ip_addrs if 'br03_v4' in e[2]]
        self.assertTrue(len(addr) == 1)
        g1.get_reachablily(addr[0])

    """
      No.4 start up ipv4 containers and check state
           each neighbor is established in ipv6 environment
    """

    def test_04_check_neighbor_established_v6(self):
        g1 = self.gobgps['ipv6']
        q1 = self.quaggas['ipv6']
        o1 = self.others['ipv6'][0]
        o2 = self.others['ipv6'][1]
        # start up containers of ipv6 environment
        initial_wait_time = max(ctn.run() for ctn in self.ctns['ipv6'])
        time.sleep(initial_wait_time)

        # make ipv6 bridge and set ip to each container
        [self.bridges['br01_v6'].addif(ctn) for ctn in [o1, g1]]
        [self.bridges['br02_v6'].addif(ctn) for ctn in [g1, q1]]
        [self.bridges['br03_v6'].addif(ctn) for ctn in [q1, o2]]

        g1.add_peer(q1, bridge=self.bridges['br02_v6'].name)
        q1.add_peer(g1, bridge=self.bridges['br02_v6'].name)

        g1.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=q1)

    """
      No.5 check whether the ping is reachable in container
           that have previously beyond the gobpg in ipv6 environment
    """

    def test_05_check_reachablily_beyond_gobgp_from_quagga(self):
        g1 = self.gobgps['ipv6']
        q1 = self.quaggas['ipv6']
        o1 = self.others['ipv6'][0]

        next_hop = g1.ip_addrs[1][1].split('/')[0]
        g1.set_ipv6_forward()
        o1.add_static_route(self.bridges['br02_v6'].subnet, next_hop)
        addr = [e[1] for e in o1.ip_addrs if 'br01_v6' in e[2]]
        self.assertTrue(len(addr) == 1)
        q1.get_reachablily(addr[0])

    """
      No.6 check whether the ping is reachable in container
           that have previously beyond the quagga in ipv6 environment
    """

    def test_06_check_reachablily_beyond_quagga_from_gobgp(self):
        g1 = self.gobgps['ipv6']
        q1 = self.quaggas['ipv6']
        o2 = self.others['ipv6'][1]

        next_hop = q1.ip_addrs[2][1].split('/')[0]
        q1.set_ipv6_forward()
        o2.add_static_route(self.bridges['br02_v6'].subnet, next_hop)
        addr = [e[1] for e in o2.ip_addrs if 'br03_v6' in e[2]]
        self.assertTrue(len(addr) == 1)
        g1.get_reachablily(addr[0])

    def test_07_mpath_test_setup(self):
        g1 = GoBGPContainer(name='g1', asn=65000, router_id='192.168.0.1',
                            ctn_image_name=parser_option.gobgp_image,
                            log_level=parser_option.gobgp_log_level,
                            config_format=parser_option.config_format,
                            zebra=True)
        g2 = GoBGPContainer(name='g2', asn=65001, router_id='192.168.0.2',
                            ctn_image_name=parser_option.gobgp_image)
        g3 = GoBGPContainer(name='g3', asn=65001, router_id='192.168.0.3',
                            ctn_image_name=parser_option.gobgp_image)
        g4 = GoBGPContainer(name='g4', asn=65000, router_id='192.168.0.4',
                            ctn_image_name=parser_option.gobgp_image)
        g5 = GoBGPContainer(name='g5', asn=65000, router_id='192.168.0.5',
                            ctn_image_name=parser_option.gobgp_image)

        ctns = [g1, g2, g3, g4, g5]
        for ctn in ctns:
            self.ctns[ctn.name] = ctn

        initial_wait_time = max(ctn.run() for ctn in ctns)

        time.sleep(initial_wait_time)

        # advertise same prefix
        g2.add_route('10.0.10.0/24')
        g3.add_route('10.0.10.0/24')
        g4.add_route('10.0.10.0/24')
        g5.add_route('10.0.10.0/24')

        for g in [g2, g3, g4, g5]:
            g1.add_peer(g)
            g.add_peer(g1)

    def test_08_mpath_test_check_neighbor_established(self):
        g1 = self.ctns['g1']
        g2 = self.ctns['g2']
        g3 = self.ctns['g3']
        g4 = self.ctns['g4']
        g5 = self.ctns['g5']
        for g in [g2, g3, g4, g5]:
            g1.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=g)

    def test_09_mpath_test_check_mpath_injected(self):
        g1 = self.ctns['g1']
        g2 = self.ctns['g2']
        g3 = self.ctns['g3']
        g4 = self.ctns['g4']
        g5 = self.ctns['g5']

        def nexthops():
            n = []
            for line in g1.local('ip route show 10.0.10.0/24', capture=True).split('\n'):
                line = line.strip()
                if 'via' in line:
                    n.append(line.split(' ')[2].strip())
            return n

        def validate_nexthops(peers):
            interval = 1
            count = 0
            timeout = 30
            while True:
                valid = False
                nhs = nexthops()
                if len(nhs) == len(peers):
                    valid = True
                    for peer in peers:
                        if g1.peers[peer]['neigh_addr'].split('/')[0] not in nhs:
                            valid = False
                            break
                if valid:
                    return

                time.sleep(interval)
                count += interval
                if count >= timeout:
                    raise Exception(nhs)

        validate_nexthops([g4, g5])

        g4.local('gobgp g ri del 10.0.10.0/24')
        validate_nexthops([g5])

        g4.local('gobgp g ri add 10.0.10.0/24 local-pref 200')
        validate_nexthops([g4])

        g4.local('gobgp g ri del 10.0.10.0/24')
        g5.local('gobgp g ri del 10.0.10.0/24')
        validate_nexthops([g2, g3])

        g3.local('gobgp g ri del 10.0.10.0/24')
        validate_nexthops([g2])

        g3.local('gobgp g ri add 10.0.10.0/24 med 10')
        validate_nexthops([g2])

        g2.local('gobgp g ri add 10.0.10.0/24 med 20')
        validate_nexthops([g3])


if __name__ == '__main__':
    output = local("which docker 2>&1 > /dev/null ; echo $?", capture=True)
    if int(output) is not 0:
        print "docker not found"
        sys.exit(1)

    nose.main(argv=sys.argv, addplugins=[OptionParser()],
              defaultTest=sys.argv[0])
