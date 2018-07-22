# Copyright (C) 2017 Nippon Telegraph and Telephone Corporation.
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

from fabric.api import local
import nose

from lib.noseplugin import OptionParser, parser_option

from lib import base
from lib.base import (
    assert_several_times,
    Bridge,
    BGP_FSM_ESTABLISHED,
)
from lib.gobgp import GoBGPContainer
from lib.quagga import QuaggaOSPFContainer


def get_ifname_with_prefix(prefix, f=local):
    command = (
        "ip addr show to %s"
        " | head -n1 | cut -d'@' -f1 | cut -d' ' -f2") % prefix

    return f(command, capture=True)


class ZebraNHTTest(unittest.TestCase):
    """
    Test case for Next-Hop Tracking with Zebra integration.
    """

    def _assert_med_equal(self, rt, prefix, med):
        rib = rt.get_global_rib(prefix=prefix)
        self.assertEqual(len(rib), 1)
        self.assertEqual(len(rib[0]['paths']), 1)
        self.assertEqual(rib[0]['paths'][0]['med'], med)

    # R1: GoBGP
    # R2: GoBGP + Zebra + OSPFd
    # R3: Zebra + OSPFd
    # R4: Zebra + OSPFd
    #
    #       +----+
    #       | R3 |... has loopback 10.3.1.1/32
    #       +----+
    #        /  |
    #       /   |
    #      /   +----+
    #     /    | R4 |
    #    /     +----+
    # +----+      |
    # | R2 |------+
    # +----+
    #   | 192.168.0.2/24
    #   |
    #   | 192.168.0.0/24
    #   |
    #   | 192.168.0.1/24
    # +----+
    # | R1 |
    # +----+

    @classmethod
    def setUpClass(cls):
        gobgp_ctn_image_name = parser_option.gobgp_image
        base.TEST_PREFIX = parser_option.test_prefix
        cls.r1 = GoBGPContainer(
            name='r1', asn=65000, router_id='192.168.0.1',
            ctn_image_name=gobgp_ctn_image_name,
            log_level=parser_option.gobgp_log_level,
            zebra=False)

        cls.r2 = GoBGPContainer(
            name='r2', asn=65000, router_id='192.168.0.2',
            ctn_image_name=gobgp_ctn_image_name,
            log_level=parser_option.gobgp_log_level,
            zebra=True,
            zapi_version=3,
            ospfd_config={
                'networks': {
                    '192.168.23.0/24': '0.0.0.0',
                    '192.168.24.0/24': '0.0.0.0',
                },
            })

        cls.r3 = QuaggaOSPFContainer(
            name='r3',
            zebra_config={
                'interfaces': {
                    'lo': [
                        'ip address 10.3.1.1/32',
                    ],
                },
            },
            ospfd_config={
                'networks': {
                    '10.3.1.1/32': '0.0.0.0',
                    '192.168.23.0/24': '0.0.0.0',
                    '192.168.34.0/24': '0.0.0.0',
                },
            })

        cls.r4 = QuaggaOSPFContainer(
            name='r4',
            ospfd_config={
                'networks': {
                    '192.168.34.0/24': '0.0.0.0',
                    '192.168.24.0/24': '0.0.0.0',
                },
            })

        wait_time = max(ctn.run() for ctn in [cls.r1, cls.r2, cls.r3, cls.r4])
        time.sleep(wait_time)

        cls.br_r1_r2 = Bridge(name='br_r1_r2', subnet='192.168.12.0/24')
        for ctn in (cls.r1, cls.r2):
            cls.br_r1_r2.addif(ctn)

        cls.br_r2_r3 = Bridge(name='br_r2_r3', subnet='192.168.23.0/24')
        for ctn in (cls.r2, cls.r3):
            cls.br_r2_r3.addif(ctn)

        cls.br_r2_r4 = Bridge(name='br_r2_r4', subnet='192.168.24.0/24')
        for ctn in (cls.r2, cls.r4):
            cls.br_r2_r4.addif(ctn)

        cls.br_r3_r4 = Bridge(name='br_r3_r4', subnet='192.168.34.0/24')
        for ctn in (cls.r3, cls.r4):
            cls.br_r3_r4.addif(ctn)

    def test_01_BGP_neighbor_established(self):
        # Test to start BGP connection up between r1-r2.

        self.r1.add_peer(self.r2, bridge=self.br_r1_r2.name)
        self.r2.add_peer(self.r1, bridge=self.br_r1_r2.name)

        self.r1.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=self.r2)

    def test_02_OSPF_established(self):
        # Test to start OSPF connection up between r2-r3 and receive the route
        # to r3's loopback '10.3.1.1'.
        def _f():
            self.assertEqual(self.r2.local(
                "vtysh -c 'show ip ospf route'"
                " | grep '10.3.1.1/32' > /dev/null"
                " && echo OK || echo NG",
                capture=True), 'OK')

        assert_several_times(f=_f, t=120)

    def test_03_add_ipv4_route(self):
        # Test to add IPv4 route to '10.3.1.0/24' whose nexthop is r3's
        # loopback '10.3.1.1'. Also, test to receive the initial MED/Metric.

        # MED/Metric = 10(r2 to r3) + 10(r3-ethX to r3-lo)
        med = 20

        self.r2.local(
            'gobgp global rib add -a ipv4 10.3.1.0/24 nexthop 10.3.1.1')

        assert_several_times(
            f=lambda: self._assert_med_equal(self.r2, '10.3.1.0/24', med))
        assert_several_times(
            f=lambda: self._assert_med_equal(self.r1, '10.3.1.0/24', med))

        # Test if the path, which came after the NEXTHOP_UPDATE message was
        # received from Zebra, is updated by reflecting the nexthop cache.
        self.r2.local(
            'gobgp global rib add -a ipv4 10.3.2.0/24 nexthop 10.3.1.1')

        assert_several_times(
            f=lambda: self._assert_med_equal(self.r2, '10.3.2.0/24', med))
        assert_several_times(
            f=lambda: self._assert_med_equal(self.r1, '10.3.2.0/24', med))

        self.r2.local(
            'gobgp global rib del -a ipv4 10.3.2.0/24')

    def test_04_link_r2_r3_down(self):
        # Test to update MED to the nexthop if the Metric to that nexthop is
        # changed by the link down. If the link r2-r3 goes down, MED/Metric
        # should be increased.

        # MED/Metric = 10(r2 to r4) + 10(r4 to r3) + 10(r3-ethX to r3-lo)
        med = 30

        ifname = get_ifname_with_prefix('192.168.23.3/24', f=self.r3.local)
        self.r3.local('ip link set %s down' % ifname)

        assert_several_times(
            f=lambda: self._assert_med_equal(self.r2, '10.3.1.0/24', med))
        assert_several_times(
            f=lambda: self._assert_med_equal(self.r1, '10.3.1.0/24', med))

    def test_05_nexthop_unreachable(self):
        # Test to update the nexthop state if nexthop become unreachable by
        # link down. If the link r2-r3 and r2-r4 goes down, there is no route
        # to r3.

        def _f_r2(prefix):
            self.assertEqual(self.r2.local(
                "gobgp global rib -a ipv4 %s"
                " | grep '^* ' > /dev/null"  # not best "*>"
                " && echo OK || echo NG" % prefix,
                capture=True), 'OK')

        def _f_r1(prefix):
            self.assertEqual(self.r1.local(
                "gobgp global rib -a ipv4 %s"
                "| grep 'Network not in table' > /dev/null"
                " && echo OK || echo NG" % prefix,
                capture=True), 'OK')

        ifname = get_ifname_with_prefix('192.168.24.4/24', f=self.r4.local)
        self.r4.local('ip link set %s down' % ifname)

        assert_several_times(f=lambda: _f_r2("10.3.1.0/24"), t=120)
        assert_several_times(f=lambda: _f_r1("10.3.1.0/24"), t=120)

        # Test if the path, which came after the NEXTHOP_UPDATE message was
        # received from Zebra, is updated by reflecting the nexthop cache.
        self.r2.local(
            'gobgp global rib add -a ipv4 10.3.2.0/24 nexthop 10.3.1.1')

        assert_several_times(f=lambda: _f_r2("10.3.2.0/24"), t=120)
        assert_several_times(f=lambda: _f_r1("10.3.2.0/24"), t=120)

        # Confirm the stability of the nexthop state
        for _ in range(10):
            time.sleep(1)
            _f_r2("10.3.1.0/24")
            _f_r1("10.3.1.0/24")
            _f_r2("10.3.2.0/24")
            _f_r1("10.3.2.0/24")

    def test_06_link_r2_r4_restore(self):
        # Test to update the nexthop state if nexthop become reachable again.
        # If the link r2-r4 goes up again, MED/Metric should be the value of
        # the path going through r4.

        # MED/Metric = 10(r2 to r4) + 10(r4 to r3) + 10(r3-ethX to r3-lo)
        med = 30

        ifname = get_ifname_with_prefix('192.168.24.4/24', f=self.r4.local)
        self.r4.local('ip link set %s up' % ifname)

        assert_several_times(
            f=lambda: self._assert_med_equal(self.r2, '10.3.1.0/24', med))
        assert_several_times(
            f=lambda: self._assert_med_equal(self.r1, '10.3.1.0/24', med))

    def test_07_nexthop_restore(self):
        # Test to update the nexthop state if the Metric to that nexthop is
        # changed. If the link r2-r3 goes up again, MED/Metric should be update
        # with the initial value.

        # MED/Metric = 10(r2 to r3) + 10(r3-ethX to r3-lo)
        med = 20

        ifname = get_ifname_with_prefix('192.168.23.3/24', f=self.r3.local)
        self.r3.local('ip link set %s up' % ifname)

        assert_several_times(
            f=lambda: self._assert_med_equal(self.r2, '10.3.1.0/24', med))
        assert_several_times(
            f=lambda: self._assert_med_equal(self.r1, '10.3.1.0/24', med))


if __name__ == '__main__':
    output = local("which docker 2>&1 > /dev/null ; echo $?", capture=True)
    if int(output) is not 0:
        print "docker not found"
        sys.exit(1)

    nose.main(argv=sys.argv, addplugins=[OptionParser()],
              defaultTest=sys.argv[0])
