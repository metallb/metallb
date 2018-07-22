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

        # Add FlowSpec routes into GoBGP.
        cls.g1.add_route(
            route='ipv4/all',
            rf='ipv4-flowspec',
            matchs=[
                'destination 11.1.0.0/24',
                'source 11.2.0.0/24',
                "protocol '==tcp &=udp icmp >igmp >=egp <ipip <=rsvp !=gre'",
                "port '==80 &=90 8080 >9090 >=8180 <9190 <=8081 !=9091 &!443'",
                'destination-port 80',
                'source-port 8080',
                'icmp-type 0',
                'icmp-code 2',
                "tcp-flags '==S &=SA A !F !=U =!R'",
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

        # Add FlowSpec routes into ExaBGP.
        cls.e1.add_route(
            route='ipv4/all',
            rf='ipv4-flowspec',
            matchs=[
                'destination 12.1.0.0/24',
                'source 12.2.0.0/24',
                'protocol =tcp',
                'port >=80',
                'destination-port >5000',
                'source-port 8080',
                'icmp-type <1',
                'icmp-code <=2',
                "tcp-flags FIN",
                'packet-length >100&<200',
                'dscp 12',
                'fragment dont-fragment',
            ],
            thens=['discard'])
        cls.e1.add_route(
            route='ipv6/dst/src/protocol/label',  # others are tested on IPv4
            rf='ipv6-flowspec',
            matchs=[
                'destination 2002:1::/64/10',
                'source 2002:2::/64/15',
                'next-header udp',
                'flow-label >100',
            ],
            thens=['discard'])

        # Add FlowSpec routes into YABGP.
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

    def test_01_ipv4_exabgp_adj_rib_in(self):
        rib = self.e1.get_adj_rib_in(self.g1, rf='ipv4-flowspec')
        self.assertEqual(1, len(rib))
        nlri = list(rib)[0]  # advertised from G1(GoBGP)
        _exp_fmt = (
            # INPUTS:
            # 'destination 11.1.0.0/24',
            "destination-ipv4 11.1.0.0/24 "
            # 'source 11.2.0.0/24',
            "source-ipv4 11.2.0.0/24 "
            # "protocol '==tcp &=udp icmp >igmp >=egp <ipip <=rsvp !=gre'",
            "protocol [ =tcp&=udp =icmp >igmp >=egp <ipip <=rsvp !=gre ] "
            # "port '==80 &=90 8080 >9090 >=8180 <9190 <=8081 !=9091 &!443'",
            "port [ =80&=90 =8080 >9090 >=8180 <9190 <=8081 !=9091&!=443 ] "
            # 'destination-port 80',
            "destination-port =80 "
            # 'source-port 8080',
            "source-port =8080 "
            # 'icmp-type 0',
            "icmp-type =echo-reply "
            # 'icmp-code 2',
            "icmp-code =2 "
            # "tcp-flags '==S &=SA A !F !=U =!R'",
            "tcp-flags [ =syn&=%s ack !fin !=urgent !=rst ] "
            # 'packet-length 100',
            "packet-length =100 "
            # 'dscp 12',
            "dscp =12 "
            # 'fragment dont-fragment is-fragment+first-fragment',
            "fragment [ dont-fragment is-fragment+first-fragment ]"
        )
        # Note: Considers variants of SYN + ACK
        expected_list = (_exp_fmt % 'syn+ack', _exp_fmt % 'ack+syn')
        self.assertIn(nlri, expected_list)

    def test_02_ipv6_exabgp_adj_rib_in(self):
        rib = self.e1.get_adj_rib_in(self.g1, rf='ipv6-flowspec')
        self.assertEqual(1, len(rib))
        nlri = list(rib)[0]  # advertised from G1(GoBGP)
        expected = (
            # INPUTS:
            # 'destination 2001:1::/64 10',
            "destination-ipv6 2001:1::/64/10 "
            # 'source 2001:2::/64 15',
            "source-ipv6 2001:2::/64/15 "
            # 'label 12',
            "flow-label =12"
        )
        self.assertEqual(expected, nlri)

    def test_03_ipv4_yabgp_adj_rib_in(self):
        rib = self.y1.get_adj_rib_in(peer=self.g1, rf='flowspec')
        self.assertEqual(1, len(rib))
        nlri = list(rib)[0]  # advertised from G1(GoBGP)
        expected = (
            # INPUTS:
            # 'destination 11.1.0.0/24',
            '{"1": "11.1.0.0/24",'
            # 'source 11.2.0.0/24',
            ' "2": "11.2.0.0/24",'
            # "protocol '==tcp &=udp icmp >igmp >=egp <ipip <=rsvp !=gre'",
            ' "3": "=6&=17|=1|>2|>=8|<94|<=46|><47",'
            # "port '==80 &=90 8080 >9090 >=8180 <9190 <=8081 !=9091 &!443'",
            ' "4": "=80&=90|=8080|>9090|>=8180|<9190|<=8081|><9091&><443",'
            # 'destination-port 80',
            ' "5": "=80",'
            # 'source-port 8080',
            ' "6": "=8080",'
            # 'icmp-type 0',
            ' "7": "=0",'
            # 'icmp-code 2',
            ' "8": "=2",'
            # "tcp-flags '==S &=SA A !F !=U =!R'",
            ' "9": "=2&=18|16|>1|>=32|>=4",'
            # 'packet-length 100',
            ' "10": "=100",'
            # 'dscp 12',
            ' "11": "=12",'
            # 'fragment dont-fragment is-fragment+first-fragment',
            ' "12": "1|6"}'
        )
        self.assertEqual(expected, nlri)

    def test_04_ipv6_yabgp_adj_rib_in(self):
        # IPv6 FlowSpec: not supported with YABGP v0.4.0
        pass

    def test_05_ipv4_gobgp_global_rib(self):
        rib = self.g1.get_global_rib(rf='ipv4-flowspec')
        self.assertEqual(3, len(rib))
        output_nlri_list = [r['prefix'] for r in rib]
        nlri_g1 = (
            # INPUTS:
            # 'destination 11.1.0.0/24',
            "[destination: 11.1.0.0/24]"
            # 'source 11.2.0.0/24',
            "[source: 11.2.0.0/24]"
            # "protocol '==tcp &=udp icmp >igmp >=egp <ipip <=rsvp !=gre'",
            "[protocol: ==tcp&==udp ==icmp >igmp >=egp <ipip <=rsvp !=gre]"
            # "port '==80 &=90 8080 >9090 >=8180 <9190 <=8081 !=9091 &!443'",
            "[port: ==80&==90 ==8080 >9090 >=8180 <9190 <=8081 !=9091&!=443]"
            # 'destination-port 80',
            "[destination-port: ==80]"
            # 'source-port 8080',
            "[source-port: ==8080]"
            # 'icmp-type 0',
            "[icmp-type: ==0]"
            # 'icmp-code 2',
            "[icmp-code: ==2]"
            # "tcp-flags '==S &=SA A !F !=U =!R'",
            "[tcp-flags: =S&=SA A !F !=U !=R]"
            # 'packet-length 100',
            "[packet-length: ==100]"
            # 'dscp 12',
            "[dscp: ==12]"
            # 'fragment dont-fragment is-fragment+first-fragment',
            "[fragment: dont-fragment is-fragment+first-fragment]"
        )
        nlri_e1 = (
            # INPUTS:
            # 'destination 12.1.0.0/24',
            '[destination: 12.1.0.0/24]'
            # 'source 12.2.0.0/24',
            '[source: 12.2.0.0/24]'
            # 'protocol =tcp',
            '[protocol: ==tcp]'
            # 'port >=80',
            '[port: >=80]'
            # 'destination-port >5000',
            '[destination-port: >5000]'
            # 'source-port 8080',
            '[source-port: ==8080]'
            # 'icmp-type <1',
            '[icmp-type: <1]'
            # 'icmp-code <=2',
            '[icmp-code: <=2]'
            # "tcp-flags FIN",
            '[tcp-flags: F]'
            # 'packet-length >100&<200',
            '[packet-length: >100&<200]'
            # 'dscp 12',
            '[dscp: ==12]'
            # 'fragment dont-fragment',
            '[fragment: dont-fragment]'
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
        for nlri in [nlri_g1, nlri_e1, nlri_y1]:
            self.assertIn(nlri, output_nlri_list)

    def test_06_ipv6_gobgp_global_rib(self):
        rib = self.g1.get_global_rib(rf='ipv6-flowspec')
        import json
        self.assertEqual(2, len(rib), json.dumps(rib))
        output_nlri_list = [r['prefix'] for r in rib]
        nlri_g1 = (
            # INPUTS:
            # 'destination 2001:1::/64 10',
            "[destination: 2001:1::/64/10]"
            # 'source 2001:2::/64 15',
            "[source: 2001:2::/64/15]"
            # 'label 12',
            "[label: ==12]"
        )
        nlri_e1 = (
            # INPUTS:
            # 'destination 2002:1::/64/10',
            '[destination: 2002:1::/64/10]'
            # 'source 2002:2::/64/15',
            '[source: 2002:2::/64/15]'
            # 'next-header udp',
            '[protocol: ==udp]'
            # 'flow-label >100',
            '[label: >100]'
        )
        for nlri in [nlri_g1, nlri_e1]:
            self.assertIn(nlri, output_nlri_list)

    def test_07_ipv4_exabgp_delete_route(self):
        # Delete a route on E1(ExaBGP)
        self.e1.del_route(route='ipv4/all')
        time.sleep(1)
        # Test if the route is deleted or not
        rib = self.g1.get_adj_rib_in(peer=self.e1, rf='ipv4-flowspec')
        self.assertEqual(0, len(rib))

    def test_08_ipv6_exabgp_delete_route(self):
        # Delete a route on E1(ExaBGP)
        self.e1.del_route(route='ipv6/dst/src/protocol/label')
        time.sleep(1)
        # Test if the route is deleted or not
        rib = self.g1.get_adj_rib_in(peer=self.e1, rf='ipv6-flowspec')
        self.assertEqual(0, len(rib))

    def test_09_ipv4_yabgp_delete_route(self):
        # Delete a route on Y1(YABGP)
        self.y1.del_route(route='ipv4/all')
        time.sleep(1)
        # Test if the route is deleted or not
        rib = self.g1.get_adj_rib_in(peer=self.y1, rf='ipv4-flowspec')
        self.assertEqual(0, len(rib))

    def test_10_ipv6_yabgp_delete_route(self):
        # IPv6 FlowSpec: not supported with YABGP v0.4.0
        pass

    def test_11_ipv4_gobgp_delete_route(self):
        # Delete a route on G1(GoBGP)
        self.g1.del_route(route='ipv4/all')
        time.sleep(1)
        # Test if the route is deleted or not
        rib_e1 = self.e1.get_adj_rib_in(peer=self.g1, rf='ipv4-flowspec')
        self.assertEqual(0, len(rib_e1))
        rib_y1 = self.y1.get_adj_rib_in(peer=self.g1, rf='ipv4-flowspec')
        self.assertEqual(0, len(rib_y1))

    def test_12_ipv6_gobgp_delete_route(self):
        # Delete a route on G1(GoBGP)
        self.g1.del_route(route='ipv6/dst/src/label')
        time.sleep(1)
        # Test if the route is deleted or not
        rib_e1 = self.e1.get_adj_rib_in(peer=self.g1, rf='ipv6-flowspec')
        self.assertEqual(0, len(rib_e1))
        rib_y1 = self.y1.get_adj_rib_in(peer=self.g1, rf='ipv6-flowspec')
        self.assertEqual(0, len(rib_y1))


if __name__ == '__main__':
    output = local("which docker 2>&1 > /dev/null ; echo $?", capture=True)
    if int(output) is not 0:
        print "docker not found"
        sys.exit(1)

    nose.main(argv=sys.argv, addplugins=[OptionParser()],
              defaultTest=sys.argv[0])
