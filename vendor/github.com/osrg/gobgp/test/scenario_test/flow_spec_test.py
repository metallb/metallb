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
from lib.base import BGP_FSM_ESTABLISHED
from lib.gobgp import GoBGPContainer
from lib.exabgp import ExaBGPContainer
from lib.yabgp import YABGPContainer


class FlowSpecTest(unittest.TestCase):

    """
    Test case for Flow Specification.
    """
    # +------------+            +------------+
    # | G1(GoBGP)  |---(iBGP)---| E1(ExaBGP) |
    # | 172.17.0.2 |            | 172.17.0.3 |
    # +------------+            +------------+
    #      |
    #    (iBGP)
    #      |
    # +------------+
    # | Y1(YABGP)  |
    # | 172.17.0.4 |
    # +------------+

    @classmethod
    def setUpClass(cls):
        gobgp_ctn_image_name = parser_option.gobgp_image
        base.TEST_PREFIX = parser_option.test_prefix

        cls.g1 = GoBGPContainer(name='g1', asn=65000, router_id='192.168.0.1',
                                ctn_image_name=gobgp_ctn_image_name,
                                log_level=parser_option.gobgp_log_level)

        cls.e1 = ExaBGPContainer(name='e1', asn=65000, router_id='192.168.0.2')

        cls.y1 = YABGPContainer(name='y1', asn=65000, router_id='192.168.0.3')

        ctns = [cls.g1, cls.e1, cls.y1]
        initial_wait_time = max(ctn.run() for ctn in ctns)
        time.sleep(initial_wait_time)

        # Add FlowSpec routes into ExaBGP.
        # Note: Currently, ExaBGPContainer only supports to add routes by
        # reloading configuration, so we add routes here.
        cls.e1.add_route(
            route='ipv4/dst/src',
            rf='ipv4-flowspec',
            matchs=[
                'destination 12.1.0.0/24',
                'source 12.2.0.0/24',
            ],
            thens=['discard'])
        cls.e1.add_route(
            route='ipv6/dst/src',
            rf='ipv6-flowspec',
            matchs=[
                'destination 2002:1::/64/10',
                'source 2002:2::/64/15',
            ],
            thens=['discard'])

        # Add FlowSpec routes into GoBGP.
        cls.g1.add_route(
            route='ipv4/all',
            rf='ipv4-flowspec',
            matchs=[
                'destination 11.1.0.0/24',
                'source 11.2.0.0/24',
                "protocol '==tcp &=udp icmp >igmp >=egp <igp <=rsvp !=gre'",
                "port '==80 &=90 8080 >9090 >=8180 <9190 <=8081 !=9091 &!443'",
                'destination-port 80',
                'source-port 8080',
                'icmp-type 1',
                'icmp-code 2',
                "tcp-flags '==S &=SA A !F !=U =!C'",
                'packet-length 100',
                'dscp 12',
                'fragment dont-fragment is-fragment+first-fragment',
            ],
            thens=['discard'])
        cls.g1.add_route(
            route='ipv6/dst/src/label',  # others are tested on IPv4
            rf='ipv6-flowspec',
            matchs=[
                'destination 2001:1::/64 10',
                'source 2001:2::/64 15',
                'label 12',
            ],
            thens=['discard'])

        cls.g1.add_peer(cls.e1, flowspec=True)
        cls.e1.add_peer(cls.g1, flowspec=True)
        cls.g1.add_peer(cls.y1, flowspec=True)
        cls.y1.add_peer(cls.g1, flowspec=True)

        cls.g1.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=cls.e1)
        cls.g1.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=cls.y1)

        # Add FlowSpec routes into YABGP.
        # Note: Currently, YABGPContainer only supports to add routes via
        # REST API after connection established, so we add routes here.
        cls.y1.add_route(
            route='ipv4/all',
            rf='ipv4-flowspec',
            matchs=[
                'destination 13.1.0.0/24',
                'source 13.2.0.0/24',
                "protocol =6|=17",
                # "port",  # not seem to be supported
                'destination-port =80',
                'source-port <8080|>9090',
                'icmp-type >=1',
                'icmp-code <2',
                # "tcp-flags",  # not seem to be supported via REST API
                'packet-length <=100',
                'dscp =12',
                # "fragment",  # not seem to be supported via REST API
            ],
            thens=['traffic-rate:0:0'])  # 'discard'
        # IPv6 FlowSpec: not supported with YABGP v0.4.0
        # cls.y1.add_route(
        #     route='ipv6/dst/src/label',  # others are tested on IPv4
        #     rf='ipv6-flowspec',
        #     matchs=[
        #         'destination 2003:1::/64/10',
        #         'source 2003:2::/64/15',
        #         'label 12',
        #     ],
        #     thens=['traffic-rate:0:0'])  # 'discard'

    def test_01_ipv4_yabgp_adj_rib_in(self):
        rib = self.y1.get_adj_rib_in(peer=self.g1, rf='flowspec')
        self.assertEqual(1, len(rib))
        nlri = list(rib)[0]  # advertised from G1(GoBGP)
        expected = (
            # INPUTS:
            # 'destination 11.1.0.0/24',
            '{"1": "11.1.0.0/24",'
            # 'source 11.2.0.0/24',
            ' "2": "11.2.0.0/24",'
            # "protocol '==tcp &=udp icmp >igmp >=egp <igp <=rsvp !=gre'",
            ' "3": "=6&=17|=1|>2|>=8|<9|<=46|><47",'
            # "port '==80 &=90 8080 >9090 >=8180 <9190 <=8081 !=9091 &!443'",
            ' "4": "=80&=90|=8080|>9090|>=8180|<9190|<=8081|><9091&><443",'
            # 'destination-port 80',
            ' "5": "=80",'
            # 'source-port 8080',
            ' "6": "=8080",'
            # 'icmp-type 1',
            ' "7": "=1",'
            # 'icmp-code 2',
            ' "8": "=2",'
            # "tcp-flags '==S &=SA A !F !=U =!C'",
            ' "9": "=2&=18|16|>1|>=32|>=128",'
            # 'packet-length 100',
            ' "10": "=100",'
            # 'dscp 12',
            ' "11": "=12",'
            # 'fragment dont-fragment is-fragment+first-fragment',
            ' "12": "1|6"}'
        )
        self.assertEqual(expected, nlri)

    def test_02_ipv4_gobgp_global_rib(self):
        rib = self.g1.get_global_rib(rf='ipv4-flowspec')
        self.assertEqual(3, len(rib))
        output_nlri_list = [r['prefix'] for r in rib]
        nlri_e1 = (
            # INPUTS:
            # 'destination 12.1.0.0/24',
            "[destination: 12.1.0.0/24]"
            # 'source 12.2.0.0/24',
            "[source: 12.2.0.0/24]"
        )
        nlri_g1 = (
            # INPUTS:
            # 'destination 11.1.0.0/24',
            "[destination: 11.1.0.0/24]"
            # 'source 11.2.0.0/24',
            "[source: 11.2.0.0/24]"
            # "protocol '==tcp &=udp icmp >igmp >=egp <igp <=rsvp !=gre'",
            "[protocol: ==tcp&==udp ==icmp >igmp >=egp <igp <=rsvp !=gre]"
            # "port '==80 &=90 8080 >9090 >=8180 <9190 <=8081 !=9091 &!443'",
            "[port: ==80&==90 ==8080 >9090 >=8180 <9190 <=8081 !=9091&!=443]"
            # 'destination-port 80',
            "[destination-port: ==80]"
            # 'source-port 8080',
            "[source-port: ==8080]"
            # 'icmp-type 1',
            "[icmp-type: ==1]"
            # 'icmp-code 2',
            "[icmp-code: ==2]"
            # "tcp-flags '==S &=SA A !F !=U =!C'",
            "[tcp-flags: =S&=SA A !F !=U !=C]"
            # 'packet-length 100',
            "[packet-length: ==100]"
            # 'dscp 12',
            "[dscp: ==12]"
            # 'fragment dont-fragment is-fragment+first-fragment',
            "[fragment: dont-fragment is-fragment+first-fragment]"
        )
        nlri_y1 = (
            # INPUTS:
            # 'destination 13.1.0.0/24',
            "[destination: 13.1.0.0/24]"
            # 'source 13.2.0.0/24',
            "[source: 13.2.0.0/24]"
            # "protocol =6|=17",
            "[protocol: ==tcp ==udp]"
            # 'destination-port =80',
            "[destination-port: ==80]"
            # 'source-port <8080|>9090',
            "[source-port: <8080 >9090]"
            # 'icmp-type >=1',
            "[icmp-type: >=1]"
            # 'icmp-code <2',
            "[icmp-code: <2]"
            # 'packet-length <=100',
            "[packet-length: <=100]"
            # 'dscp =12',
            "[dscp: ==12]"
        )
        for nlri in [nlri_e1, nlri_g1, nlri_y1]:
            self.assertIn(nlri, output_nlri_list)

    def test_03_ipv6_gobgp_global_rib(self):
        rib = self.g1.get_global_rib(rf='ipv6-flowspec')
        self.assertEqual(2, len(rib))
        output_nlri_list = [r['prefix'] for r in rib]
        nlri_e1 = (
            # INPUTS:
            # 'destination 2002:1::/64/10',
            "[destination: 2002:1::/64/10]"
            # 'source 2002:2::/64/15',
            "[source: 2002:2::/64/15]"
        )
        nlri_g1 = (
            # INPUTS:
            # 'destination 2001:1::/64 10',
            "[destination: 2001:1::/64/10]"
            # 'source 2001:2::/64 15',
            "[source: 2001:2::/64/15]"
            # 'label 12',
            "[label: ==12]"
        )
        for nlri in [nlri_e1, nlri_g1]:
            self.assertIn(nlri, output_nlri_list)

    def test_04_ipv4_yabgp_delete_route(self):
        # Delete a route on Y1(YABGP)
        self.y1.del_route(route='ipv4/all')
        time.sleep(1)
        # Test if the route is deleted or not
        rib = self.g1.get_adj_rib_in(peer=self.y1, rf='ipv4-flowspec')
        self.assertEqual(0, len(rib))

    def test_05_ipv4_gobgp_delete_route(self):
        # Delete a route on G1(GoBGP)
        self.g1.del_route(route='ipv4/all')
        time.sleep(1)
        # Test if the route is deleted or not
        rib = self.y1.get_adj_rib_in(peer=self.g1, rf='ipv4-flowspec')
        self.assertEqual(0, len(rib))


if __name__ == '__main__':
    output = local("which docker 2>&1 > /dev/null ; echo $?", capture=True)
    if int(output) is not 0:
        print "docker not found"
        sys.exit(1)

    nose.main(argv=sys.argv, addplugins=[OptionParser()],
              defaultTest=sys.argv[0])
