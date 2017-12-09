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
import inspect

from fabric.api import local
import nose

from lib.noseplugin import OptionParser, parser_option

from lib import base
from lib.base import BGP_FSM_ESTABLISHED
from lib.gobgp import GoBGPContainer
from lib.exabgp import ExaBGPContainer


counter = 1
_SCENARIOS = {}


def register_scenario(cls):
    global counter
    _SCENARIOS[counter] = cls
    counter += 1


def lookup_scenario(name):
    for value in _SCENARIOS.values():
        if value.__name__ == name:
            return value
    return None


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


@register_scenario
class MalformedMpReachNlri(object):
    """
      No.1 malformaed mp-reach-nlri
    """

    @staticmethod
    def boot(env):
        gobgp_ctn_image_name = env.parser_option.gobgp_image
        log_level = env.parser_option.gobgp_log_level
        g1 = GoBGPContainer(name='g1', asn=65000, router_id='192.168.0.1',
                            ctn_image_name=gobgp_ctn_image_name,
                            log_level=log_level)
        e1 = ExaBGPContainer(name='e1', asn=65001, router_id='192.168.0.2')
        e2 = ExaBGPContainer(name='e2', asn=65001, router_id='192.168.0.2')

        ctns = [g1, e1, e2]
        initial_wait_time = max(ctn.run() for ctn in ctns)
        time.sleep(initial_wait_time)

        for q in [e1, e2]:
            g1.add_peer(q, is_rs_client=True)
            q.add_peer(g1)

        env.g1 = g1
        env.e1 = e1
        env.e2 = e2

    @staticmethod
    def setup(env):
        g1 = env.g1
        e1 = env.e1
        e2 = env.e2
        for c in [e1, e2]:
            g1.wait_for(BGP_FSM_ESTABLISHED, c)

        # advertise malformed MP_REACH_NLRI
        e1.add_route('10.7.0.17/32', attribute='0x0e 0x60 0x11223344')

    @staticmethod
    def check(env):
        g1 = env.g1
        e1 = env.e1
        e2 = env.e2

        def f():
            for line in e1.log().split('\n'):
                if 'UPDATE message error / Attribute Flags Error / 0x600E0411223344' in line:
                    return True
            return False

        wait_for(f)
        # check e2 is still established
        g1.wait_for(BGP_FSM_ESTABLISHED, e2)

    @staticmethod
    def executor(env):
        lookup_scenario("MalformedMpReachNlri").boot(env)
        lookup_scenario("MalformedMpReachNlri").setup(env)
        lookup_scenario("MalformedMpReachNlri").check(env)


@register_scenario
class MalformedMpUnReachNlri(object):
    """
      No.2 malformaed mp-unreach-nlri
    """

    @staticmethod
    def boot(env):
        lookup_scenario("MalformedMpReachNlri").boot(env)

    @staticmethod
    def setup(env):
        g1 = env.g1
        e1 = env.e1
        e2 = env.e2
        for c in [e1, e2]:
            g1.wait_for(BGP_FSM_ESTABLISHED, c)

        # advertise malformed MP_UNREACH_NLRI
        e1.add_route('10.7.0.17/32', attribute='0x0f 0x60 0x11223344')

    @staticmethod
    def check(env):
        g1 = env.g1
        e1 = env.e1
        e2 = env.e2

        def f():
            for line in e1.log().split('\n'):
                if 'UPDATE message error / Attribute Flags Error / 0x600F0411223344' in line:
                    return True
            return False

        wait_for(f)
        # check e2 is still established
        g1.wait_for(BGP_FSM_ESTABLISHED, e2)

    @staticmethod
    def executor(env):
        lookup_scenario("MalformedMpUnReachNlri").boot(env)
        lookup_scenario("MalformedMpUnReachNlri").setup(env)
        lookup_scenario("MalformedMpUnReachNlri").check(env)


@register_scenario
class MalformedAsPath(object):
    """
      No.3 malformaed as-path
    """

    @staticmethod
    def boot(env):
        lookup_scenario("MalformedMpReachNlri").boot(env)

    @staticmethod
    def setup(env):
        g1 = env.g1
        e1 = env.e1
        e2 = env.e2
        for c in [e1, e2]:
            g1.wait_for(BGP_FSM_ESTABLISHED, c)

        # advertise malformed AS_PATH
        # Send the attribute to the length and number of aspath is inconsistent
        # Attribute Type  0x02 (AS_PATH)
        # Attribute Flag  0x40 (well-known transitive)
        # Attribute Value 0x02020000ffdc (
        #  segment type    = 02
        #  segment length  = 02 -> # correct value = 01
        #  as number       = 65500   )
        e1.add_route('10.7.0.17/32', attribute='0x02 0x60 0x11223344')

    @staticmethod
    def check(env):
        g1 = env.g1
        e1 = env.e1
        e2 = env.e2

        def f():
            for line in e1.log().split('\n'):
                if 'UPDATE message error / Attribute Flags Error / 0x60020411223344' in line:
                    return True
            return False

        wait_for(f)
        # check e2 is still established
        g1.wait_for(BGP_FSM_ESTABLISHED, e2)

    @staticmethod
    def executor(env):
        lookup_scenario("MalformedAsPath").boot(env)
        lookup_scenario("MalformedAsPath").setup(env)
        lookup_scenario("MalformedAsPath").check(env)


@register_scenario
class MalformedAs4Path(object):
    """
      No.4 malformaed as4-path
    """

    @staticmethod
    def boot(env):
        lookup_scenario("MalformedMpReachNlri").boot(env)

    @staticmethod
    def setup(env):
        g1 = env.g1
        e1 = env.e1
        e2 = env.e2
        for c in [e1, e2]:
            g1.wait_for(BGP_FSM_ESTABLISHED, c)

        # advertise malformed AS4_PATH
        e1.add_route('10.7.0.17/32', attribute='0x11 0x60 0x11223344')

    @staticmethod
    def check(env):
        g1 = env.g1
        e1 = env.e1
        e2 = env.e2

        def f():
            for line in e1.log().split('\n'):
                if 'UPDATE message error / Attribute Flags Error / 0x60110411223344' in line:
                    return True
            return False

        wait_for(f)
        # check e2 is still established
        g1.wait_for(BGP_FSM_ESTABLISHED, e2)

    @staticmethod
    def executor(env):
        lookup_scenario("MalformedAs4Path").boot(env)
        lookup_scenario("MalformedAs4Path").setup(env)
        lookup_scenario("MalformedAs4Path").check(env)


@register_scenario
class MalformedNexthop(object):
    """
      No.5 malformaed nexthop
    """

    @staticmethod
    def boot(env):
        lookup_scenario("MalformedMpReachNlri").boot(env)

    @staticmethod
    def setup(env):
        g1 = env.g1
        e1 = env.e1
        e2 = env.e2
        for c in [e1, e2]:
            g1.wait_for(BGP_FSM_ESTABLISHED, c)

        # advertise malformed NEXT_HOP
        # 0x0e: MP_REACH_NLRI
        # 0x60: Optional, Transitive
        # 0x01: AFI(IPv4)
        # 0x01: SAFI(unicast)
        # 0x10: Length of Next Hop Address
        # 0xffffff00: Network address of Next Hop
        # 0x00: Reserved
        e1.add_route('10.7.0.17/32', attribute='0x0e 0x60 0x010110ffffff0000')

    @staticmethod
    def check(env):
        g1 = env.g1
        e1 = env.e1
        e2 = env.e2

        def f():
            for line in e1.log().split('\n'):
                if 'UPDATE message error / Attribute Flags Error / 0x600E08010110FFFFFF0000' in line:
                    return True
            return False

        wait_for(f)
        # check e2 is still established
        g1.wait_for(BGP_FSM_ESTABLISHED, e2)

    @staticmethod
    def executor(env):
        lookup_scenario("MalformedNexthop").boot(env)
        lookup_scenario("MalformedNexthop").setup(env)
        lookup_scenario("MalformedNexthop").check(env)


@register_scenario
class MalformedRouteFamily(object):
    """
      No.6 malformaed route family
    """

    @staticmethod
    def boot(env):
        lookup_scenario("MalformedMpReachNlri").boot(env)

    @staticmethod
    def setup(env):
        g1 = env.g1
        e1 = env.e1
        e2 = env.e2
        for c in [e1, e2]:
            g1.wait_for(BGP_FSM_ESTABLISHED, c)

        # advertise malformed ROUTE_FAMILY
        # 0x0e: MP_REACH_NLRI
        # 0x60: Optional, Transitive
        # 0x01: AFI(IPv4)
        # 0x01: SAFI(unicast)
        # 0x10: Length of Next Hop Address
        # 0xffffff00: Network address of Next Hop
        # 0x00: Reserved
        e1.add_route('10.7.0.17/32', attribute='0x0e 0x60 0x0002011020010db800000000000000000000000100')

    @staticmethod
    def check(env):
        g1 = env.g1
        e1 = env.e1
        e2 = env.e2

        def f():
            for line in e1.log().split('\n'):
                if 'UPDATE message error / Attribute Flags Error / 0x600E150002011020010DB800000000000000000000000100' in line:
                    return True
            return False

        wait_for(f)
        # check e2 is still established
        g1.wait_for(BGP_FSM_ESTABLISHED, e2)

    @staticmethod
    def executor(env):
        lookup_scenario("MalformedRouteFamily").boot(env)
        lookup_scenario("MalformedRouteFamily").setup(env)
        lookup_scenario("MalformedRouteFamily").check(env)


@register_scenario
class MalformedAsPathSegmentLengthInvalid(object):
    """
      No.7 malformaed aspath segment length invalid
    """

    @staticmethod
    def boot(env):
        lookup_scenario("MalformedMpReachNlri").boot(env)

    @staticmethod
    def setup(env):
        g1 = env.g1
        e1 = env.e1
        e2 = env.e2
        for c in [e1, e2]:
            g1.wait_for(BGP_FSM_ESTABLISHED, c)

        # advertise malformed AS_PATH SEGMENT LENGTH
        # Send the attribute to the length and number of aspath is inconsistent
        # Attribute Type  0x02 (AS_PATH)
        # Attribute Flag  0x40 (well-known transitive)
        # Attribute Value 0x02020000ffdc (
        #  segment type    = 02
        #  segment length  = 02 -> # correct value = 01
        #  as number       = 65500   )
        e1.add_route('10.7.0.17/32', attribute='0x02 0x40 0x0202ffdc')

    @staticmethod
    def check(env):
        g1 = env.g1
        e1 = env.e1
        e2 = env.e2

        def f():
            for line in e1.log().split('\n'):
                if 'UPDATE message error / Malformed AS_PATH / 0x4002040202FFDC' in line:
                    return True
            return False

        wait_for(f)
        # check e2 is still established
        g1.wait_for(BGP_FSM_ESTABLISHED, e2)

    @staticmethod
    def executor(env):
        lookup_scenario("MalformedAsPathSegmentLengthInvalid").boot(env)
        lookup_scenario("MalformedAsPathSegmentLengthInvalid").setup(env)
        lookup_scenario("MalformedAsPathSegmentLengthInvalid").check(env)


@register_scenario
class MalformedNexthopLoopbackAddr(object):
    """
      No.8 malformaed nexthop loopback addr
    """

    @staticmethod
    def boot(env):
        lookup_scenario("MalformedMpReachNlri").boot(env)

    @staticmethod
    def setup(env):
        g1 = env.g1
        e1 = env.e1
        e2 = env.e2
        for c in [e1, e2]:
            g1.wait_for(BGP_FSM_ESTABLISHED, c)

        # Malformed Invalid NEXT_HOP Attribute
        # Send the attribute of invalid nexthop
        # next-hop 127.0.0.1 -> # correct value = other than loopback and 0.0.0.0 address
        e1.add_route('10.7.0.17/32', nexthop='127.0.0.1')

    @staticmethod
    def check(env):
        g1 = env.g1
        e1 = env.e1
        e2 = env.e2

        def f():
            for line in e1.log().split('\n'):
                if 'UPDATE message error / Invalid NEXT_HOP Attribute / 0x4003047F000001' in line:
                    return True
            return False

        wait_for(f)
        # check e2 is still established
        g1.wait_for(BGP_FSM_ESTABLISHED, e2)

    @staticmethod
    def executor(env):
        lookup_scenario("MalformedNexthopLoopbackAddr").boot(env)
        lookup_scenario("MalformedNexthopLoopbackAddr").setup(env)
        lookup_scenario("MalformedNexthopLoopbackAddr").check(env)


@register_scenario
class MalformedOriginType(object):
    """
      No.9 malformaed origin type
    """

    @staticmethod
    def boot(env):
        lookup_scenario("MalformedMpReachNlri").boot(env)

    @staticmethod
    def setup(env):
        g1 = env.g1
        e1 = env.e1
        e2 = env.e2
        for c in [e1, e2]:
            g1.wait_for(BGP_FSM_ESTABLISHED, c)

        # Invalid ORIGIN Attribute
        # Send the attribute of origin type 4
        # Attribute Type  0x01 (Origin)
        # Attribute Flag  0x40 (well-known transitive)
        # Attribute Value 0x04 (
        #  origin type    = 04 -> # correct value = 01 or 02 or 03 )
        e1.add_route('10.7.0.17/32', attribute='0x1 0x40 0x04')

    @staticmethod
    def check(env):
        g1 = env.g1
        e1 = env.e1
        e2 = env.e2

        def f():
            for line in e1.log().split('\n'):
                if 'UPDATE message error / Invalid ORIGIN Attribute / 0x40010104' in line:
                    return True
            return False

        wait_for(f)
        # check e2 is still established
        g1.wait_for(BGP_FSM_ESTABLISHED, e2)

    @staticmethod
    def executor(env):
        lookup_scenario("MalformedOriginType").boot(env)
        lookup_scenario("MalformedOriginType").setup(env)
        lookup_scenario("MalformedOriginType").check(env)


class TestGoBGPBase(unittest.TestCase):

    wait_per_retry = 5
    retry_limit = 10

    @classmethod
    def setUpClass(cls):
        idx = parser_option.test_index
        base.TEST_PREFIX = parser_option.test_prefix
        cls.parser_option = parser_option
        cls.executors = []
        if idx == 0:
            print 'unset test-index. run all test sequential'
            for _, v in _SCENARIOS.items():
                for k, m in inspect.getmembers(v, inspect.isfunction):
                    if k == 'executor':
                        cls.executor = m
                cls.executors.append(cls.executor)
        elif idx not in _SCENARIOS:
            print 'invalid test-index. # of scenarios: {0}'.format(len(_SCENARIOS))
            sys.exit(1)
        else:
            for k, m in inspect.getmembers(_SCENARIOS[idx], inspect.isfunction):
                if k == 'executor':
                    cls.executor = m
            cls.executors.append(cls.executor)

    def test(self):
        for e in self.executors:
            yield e


if __name__ == '__main__':
    output = local("which docker 2>&1 > /dev/null ; echo $?", capture=True)
    if int(output) is not 0:
        print "docker not found"
        sys.exit(1)

    nose.main(argv=sys.argv, addplugins=[OptionParser()],
              defaultTest=sys.argv[0])
