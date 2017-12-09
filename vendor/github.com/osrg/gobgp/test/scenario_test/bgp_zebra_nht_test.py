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
from fabric.state import env
import nose

from lib.noseplugin import OptionParser, parser_option

from lib import base
from lib.base import (
    Bridge,
    BGP_FSM_ESTABLISHED,
)
from lib.gobgp import GoBGPContainer
from lib.quagga import QuaggaOSPFContainer


def try_local(command, f=local, ok_ret_codes=None, **kwargs):
    ok_ret_codes = ok_ret_codes or []
    orig_ok_ret_codes = list(env.ok_ret_codes)
    try:
        env.ok_ret_codes.extend(ok_ret_codes)
        return f(command, **kwargs)
    finally:
        env.ok_ret_codes = orig_ok_ret_codes


def wait_for(f, timeout=120):
    interval = 1
    count = 0
    while True:
        if f():
            return

        time.sleep(interval)
        count += interval
        if count >= timeout:
            raise Exception('timeout')


def get_ifname_with_prefix(prefix, f=local):
    command = (
        "ip addr show to %s"
        " | head -n1 | cut -d'@' -f1 | cut -d' ' -f2") % prefix

    return f(command, capture=True)


class ZebraNHTTest(unittest.TestCase):
    """
    Test case for Next-Hop Tracking with Zebra integration.
    """
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

        local("echo 'start %s'" % cls.__name__, capture=True)

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
        [cls.br_r1_r2.addif(ctn) for ctn in (cls.r1, cls.r2)]

        cls.br_r2_r3 = Bridge(name='br_r2_r3', subnet='192.168.23.0/24')
        [cls.br_r2_r3.addif(ctn) for ctn in (cls.r2, cls.r3)]

        cls.br_r2_r4 = Bridge(name='br_r2_r4', subnet='192.168.24.0/24')
        [cls.br_r2_r4.addif(ctn) for ctn in (cls.r2, cls.r4)]

        cls.br_r3_r4 = Bridge(name='br_r3_r4', subnet='192.168.34.0/24')
        [cls.br_r3_r4.addif(ctn) for ctn in (cls.r3, cls.r4)]

    def test_01_BGP_neighbor_established(self):
        """
        Test to start BGP connection up between r1-r2.
        """

        self.r1.add_peer(self.r2, bridge=self.br_r1_r2.name)
        self.r2.add_peer(self.r1, bridge=self.br_r1_r2.name)

        self.r1.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=self.r2)

    def test_02_OSPF_established(self):
        """
        Test to start OSPF connection up between r2-r3 and receive the route
        to r3's loopback '10.3.1.1'.
        """
        def _f():
            return try_local(
                "vtysh -c 'show ip ospf route'"
                " | grep '10.3.1.1/32'",
                f=self.r2.local,
                ok_ret_codes=[1],  # for the empty case with "grep" command
                capture=True)

        wait_for(f=_f)

    def test_03_add_ipv4_route(self):
        """
        Test to add IPv4 route to '10.3.1.0/24' whose nexthop is r3's
        loopback '10.3.1.1'.

        Also, test to receive the initial MED/Metric.
        """
        # MED/Metric = 10(r2 to r3) + 10(r3-ethX to r3-lo)
        med = 20

        def _f_r2():
            return try_local(
                "gobgp global rib -a ipv4 10.3.1.0/24"
                " | grep 'Med: %d'" % med,
                f=self.r2.local,
                ok_ret_codes=[1],  # for the empty case with "grep" command
                capture=True)

        def _f_r1():
            return try_local(
                "gobgp global rib -a ipv4 10.3.1.0/24"
                " | grep 'Med: %d'" % med,
                f=self.r1.local,
                ok_ret_codes=[1],  # for the empty case with "grep" command
                capture=True)

        self.r2.local(
            'gobgp global rib add -a ipv4 10.3.1.0/24 nexthop 10.3.1.1')

        wait_for(f=_f_r2)
        wait_for(f=_f_r1)

    def test_04_link_r2_r3_down(self):
        """
        Test to update MED to the nexthop if the Metric to that nexthop is
        changed by the link down.

        If the link r2-r3 goes down, MED/Metric should be increased.
        """
        # MED/Metric = 10(r2 to r4) + 10(r4 to r3) + 10(r3-ethX to r3-lo)
        med = 30

        def _f_r2():
            return try_local(
                "gobgp global rib -a ipv4 10.3.1.0/24"
                " | grep 'Med: %d'" % med,
                f=self.r2.local,
                ok_ret_codes=[1],  # for the empty case with "grep" command
                capture=True)

        def _f_r1():
            return try_local(
                "gobgp global rib -a ipv4 10.3.1.0/24"
                " | grep 'Med: %d'" % med,
                f=self.r1.local,
                ok_ret_codes=[1],  # for the empty case with "grep" command
                capture=True)

        ifname = get_ifname_with_prefix('192.168.23.3/24', f=self.r3.local)
        self.r3.local('ip link set %s down' % ifname)

        wait_for(f=_f_r2)
        wait_for(f=_f_r1)

    def test_05_link_r2_r3_restore(self):
        """
        Test to update MED to the nexthop if the Metric to that nexthop is
        changed by the link up again.

        If the link r2-r3 goes up again, MED/Metric should be update with
        the initial value.
        """
        # MED/Metric = 10(r2 to r3) + 10(r3-ethX to r3-lo)
        med = 20

        def _f_r2():
            return try_local(
                "gobgp global rib -a ipv4 10.3.1.0/24"
                " | grep 'Med: %d'" % med,
                f=self.r2.local,
                ok_ret_codes=[1],  # for the empty case with "grep" command
                capture=True)

        def _f_r1():
            return try_local(
                "gobgp global rib -a ipv4 10.3.1.0/24"
                " | grep 'Med: %d'" % med,
                f=self.r1.local,
                ok_ret_codes=[1],  # for the empty case with "grep" command
                capture=True)

        ifname = get_ifname_with_prefix('192.168.23.3/24', f=self.r3.local)
        self.r3.local('ip link set %s up' % ifname)

        wait_for(f=_f_r2)
        wait_for(f=_f_r1)


if __name__ == '__main__':
    output = local("which docker 2>&1 > /dev/null ; echo $?", capture=True)
    if int(output) is not 0:
        print "docker not found"
        sys.exit(1)

    nose.main(argv=sys.argv, addplugins=[OptionParser()],
              defaultTest=sys.argv[0])
