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
    BGP_FSM_ACTIVE,
    BGP_FSM_ESTABLISHED,
)
from lib.gobgp import GoBGPContainer
from lib.quagga import QuaggaBGPContainer


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

        rs_clients = [
            QuaggaBGPContainer(name='q{0}'.format(i + 1), asn=(65001 + i),
                               router_id='192.168.0.{0}'.format(i + 2))
            for i in range(3)]
        ctns = [g1] + rs_clients
        q1 = rs_clients[0]
        q2 = rs_clients[1]
        q3 = rs_clients[2]

        # advertise a route from route-server-clients
        routes = []
        for idx, rs_client in enumerate(rs_clients):
            route = '10.0.{0}.0/24'.format(idx + 1)
            rs_client.add_route(route)
            routes.append(route)

        initial_wait_time = max(ctn.run() for ctn in ctns)

        time.sleep(initial_wait_time)

        for rs_client in rs_clients:
            g1.add_peer(rs_client, is_rs_client=True, passwd='passwd', passive=True, prefix_limit=10)
            rs_client.add_peer(g1, passwd='passwd')

        cls.gobgp = g1
        cls.quaggas = {'q1': q1, 'q2': q2, 'q3': q3}

    def check_gobgp_local_rib(self):
        for rs_client in self.quaggas.itervalues():
            done = False
            for _ in range(self.retry_limit):
                if done:
                    break
                local_rib = self.gobgp.get_local_rib(rs_client)
                local_rib = [p['prefix'] for p in local_rib]

                state = self.gobgp.get_neighbor_state(rs_client)
                self.assertEqual(state, BGP_FSM_ESTABLISHED)
                if len(local_rib) < (len(self.quaggas) - 1):
                    time.sleep(self.wait_per_retry)
                    continue

                self.assertTrue(len(local_rib) == (len(self.quaggas) - 1))

                for c in self.quaggas.itervalues():
                    if rs_client != c:
                        for r in c.routes:
                            self.assertTrue(r in local_rib)

                done = True
            if done:
                continue
            # should not reach here
            raise AssertionError

    def check_rs_client_rib(self):
        for rs_client in self.quaggas.itervalues():
            done = False
            for _ in range(self.retry_limit):
                if done:
                    break
                global_rib = rs_client.get_global_rib()
                global_rib = [p['prefix'] for p in global_rib]
                if len(global_rib) < len(self.quaggas):
                    time.sleep(self.wait_per_retry)
                    continue

                self.assertTrue(len(global_rib) == len(self.quaggas))

                for c in self.quaggas.itervalues():
                    for r in c.routes:
                        self.assertTrue(r in global_rib)

                done = True
            if done:
                continue
            # should not reach here
            raise AssertionError

    # test each neighbor state is turned establish
    def test_01_neighbor_established(self):
        for q in self.quaggas.itervalues():
            self.gobgp.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=q)

    # check advertised routes are stored in route-server's local-rib
    def test_02_check_gobgp_local_rib(self):
        self.check_gobgp_local_rib()

    # check gobgp's global rib. when configured as route-server, global rib
    # must be empty
    def test_03_check_gobgp_global_rib(self):
        self.assertTrue(len(self.gobgp.get_global_rib()) == 0)

    # check routes are properly advertised to route-server-client
    def test_04_check_rs_clients_rib(self):
        self.check_rs_client_rib()

    # check if quagga that is appended can establish connection with gobgp
    def test_05_add_rs_client(self):
        q4 = QuaggaBGPContainer(name='q4', asn=65004, router_id='192.168.0.5')
        self.quaggas['q4'] = q4

        route = '10.0.4.0/24'
        q4.add_route(route)

        initial_wait_time = q4.run()
        time.sleep(initial_wait_time)
        self.gobgp.add_peer(q4, is_rs_client=True)
        q4.add_peer(self.gobgp)

        self.gobgp.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=q4)

    # check advertised routes are stored in gobgp's local-rib
    def test_05_check_gobgp_local_rib(self):
        self.check_gobgp_local_rib()

    # check routes are properly advertised to quagga
    def test_06_check_rs_clients_rib(self):
        self.check_rs_client_rib()

    def test_07_stop_one_rs_client(self):
        q4 = self.quaggas['q4']
        q4.stop()
        self.gobgp.wait_for(expected_state=BGP_FSM_ACTIVE, peer=q4)

        del self.quaggas['q4']

    # check a route advertised from q4 is deleted from gobgp's local-rib
    def test_08_check_gobgp_local_rib(self):
        self.check_gobgp_local_rib()

    # check whether gobgp properly sent withdrawal message with q4's route
    def test_09_check_rs_clients_rib(self):
        self.check_rs_client_rib()

    @unittest.skip("med shouldn't work with different AS peers by default")
    def test_10_add_distant_relative(self):
        q1 = self.quaggas['q1']
        q2 = self.quaggas['q2']
        q3 = self.quaggas['q3']
        q5 = QuaggaBGPContainer(name='q5', asn=65005, router_id='192.168.0.6')

        initial_wait_time = q5.run()
        time.sleep(initial_wait_time)

        for q in [q2, q3]:
            q5.add_peer(q)
            q.add_peer(q5)

        med200 = {'name': 'med200',
                  'type': 'permit',
                  'match': '0.0.0.0/0',
                  'med': 200,
                  'priority': 10}
        q2.add_policy(med200, self.gobgp, 'out')
        med100 = {'name': 'med100',
                  'type': 'permit',
                  'match': '0.0.0.0/0',
                  'med': 100,
                  'priority': 10}
        q3.add_policy(med100, self.gobgp, 'out')

        q5.add_route('10.0.6.0/24')

        q2.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=q5)
        q3.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=q5)
        self.gobgp.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=q2)
        self.gobgp.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=q3)

        def check_nexthop(target_prefix, expected_nexthop):
            is_done = False
            for _ in range(self.retry_limit):
                if is_done:
                    break
                time.sleep(self.wait_per_retry)
                for path in q1.get_global_rib():
                    if path['prefix'] == target_prefix:
                        print "{0}'s nexthop is {1}".format(path['prefix'],
                                                            path['nexthop'])
                        n_addrs = [i[1].split('/')[0] for i in
                                   expected_nexthop.ip_addrs]
                        if path['nexthop'] in n_addrs:
                            is_done = True
                            break
            return is_done

        done = check_nexthop('10.0.6.0/24', q3)
        self.assertTrue(done)

        med300 = {'name': 'med300',
                  'type': 'permit',
                  'match': '0.0.0.0/0',
                  'med': 300,
                  'priority': 5}
        q3.add_policy(med300, self.gobgp, 'out')

        time.sleep(self.wait_per_retry)

        done = check_nexthop('10.0.6.0/24', q2)
        self.assertTrue(done)


if __name__ == '__main__':
    output = local("which docker 2>&1 > /dev/null ; echo $?", capture=True)
    if int(output) is not 0:
        print "docker not found"
        sys.exit(1)

    nose.main(argv=sys.argv, addplugins=[OptionParser()],
              defaultTest=sys.argv[0])
