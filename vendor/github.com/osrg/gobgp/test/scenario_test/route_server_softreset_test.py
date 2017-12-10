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

    wait_per_retry = 5
    retry_limit = 15

    @classmethod
    def setUpClass(cls):
        gobgp_ctn_image_name = parser_option.gobgp_image
        base.TEST_PREFIX = parser_option.test_prefix

        g1 = GoBGPContainer(name='g1', asn=65000, router_id='192.168.0.1',
                            ctn_image_name=gobgp_ctn_image_name,
                            log_level=parser_option.gobgp_log_level)
        g2 = GoBGPContainer(name='g2', asn=65001, router_id='192.168.0.2',
                            ctn_image_name=gobgp_ctn_image_name,
                            log_level=parser_option.gobgp_log_level)
        g3 = GoBGPContainer(name='g3', asn=65002, router_id='192.168.0.3',
                            ctn_image_name=gobgp_ctn_image_name,
                            log_level=parser_option.gobgp_log_level)
        g4 = GoBGPContainer(name='g4', asn=65003, router_id='192.168.0.4',
                            ctn_image_name=gobgp_ctn_image_name,
                            log_level=parser_option.gobgp_log_level)
        ctns = [g1, g2, g3, g4]

        # advertise a route from route-server-clients
        cls.clients = {}
        for cli in [g2, g3, g4]:
            cls.clients[cli.name] = cli
        initial_wait_time = max(ctn.run() for ctn in ctns)

        time.sleep(initial_wait_time)

        for cli in cls.clients.itervalues():
            g1.add_peer(cli, is_rs_client=True, passwd='passwd', passive=True, prefix_limit=10)
            cli.add_peer(g1, passwd='passwd')

        cls.gobgp = g1

    # test each neighbor state is turned establish
    def test_01_neighbor_established(self):
        for cli in self.clients.itervalues():
            self.gobgp.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=cli)

    def test_02_softresetin_test1(self):
        g1 = self.gobgp
        g2 = self.clients['g2']
        g3 = self.clients['g3']

        p1 = {'ip-prefix': '10.0.10.0/24'}
        p2 = {'ip-prefix': '10.0.20.0/24'}

        ps0 = {'prefix-set-name': 'ps0', 'prefix-list': [p1, p2]}
        g1.set_prefix_set(ps0)

        st0 = {'conditions': {'match-prefix-set': {'prefix-set': 'ps0'}},
               'actions': {'route-disposition': 'accept-route'}}

        pol0 = {'name': 'pol0', 'statements': [st0]}

        _filename = g1.add_policy(pol0, g3, 'in', 'reject')

        g3.add_route('10.0.10.0/24')
        g3.add_route('10.0.20.0/24')

        time.sleep(1)

        num = g2.get_neighbor(g1)['state']['messages']['received']['update']

        ps0 = {'prefix-set-name': 'ps0', 'prefix-list': [p1]}
        g1.set_prefix_set(ps0)
        g1.create_config()
        # this will cause g1 to do softresetin for all neighbors (policy is changed)
        g1.reload_config()

        time.sleep(1)

        num2 = g2.get_neighbor(g1)['state']['messages']['received']['update']
        self.assertTrue((num + 1) == num2)

        g3.softreset(g1, type='out')

        time.sleep(1)

        num3 = g2.get_neighbor(g1)['state']['messages']['received']['update']
        self.assertTrue(num2 == num3)

    def test_03_softresetin_test2(self):
        g1 = self.gobgp
        g2 = self.clients['g2']

        g2.add_route('10.0.10.0/24')
        time.sleep(1)

        num = g2.get_neighbor(g1)['state']['messages']['received']['update']
        time.sleep(3)

        g1.local('gobgp n all softresetin')
        time.sleep(3)

        num1 = g2.get_neighbor(g1)['state']['messages']['received']['update']

        self.assertTrue(num == num1)


if __name__ == '__main__':
    output = local("which docker 2>&1 > /dev/null ; echo $?", capture=True)
    if int(output) is not 0:
        print "docker not found"
        sys.exit(1)

    nose.main(argv=sys.argv, addplugins=[OptionParser()],
              defaultTest=sys.argv[0])
