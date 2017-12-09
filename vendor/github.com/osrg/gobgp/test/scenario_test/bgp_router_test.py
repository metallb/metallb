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

import json
import sys
import time
import unittest

from fabric.api import local
import nose

from lib.noseplugin import OptionParser, parser_option

from lib import base
from lib.base import (
    BGP_FSM_IDLE,
    BGP_FSM_ACTIVE,
    BGP_FSM_ESTABLISHED,
    BGP_ATTR_TYPE_MULTI_EXIT_DISC,
    BGP_ATTR_TYPE_LOCAL_PREF,
    wait_for_completion,
)
from lib.gobgp import (
    GoBGPContainer,
    extract_path_attribute,
)
from lib.quagga import QuaggaBGPContainer
from lib.exabgp import ExaBGPContainer


class GoBGPTestBase(unittest.TestCase):

    @classmethod
    def setUpClass(cls):
        gobgp_ctn_image_name = parser_option.gobgp_image
        base.TEST_PREFIX = parser_option.test_prefix

        g1 = GoBGPContainer(name='g1', asn=65000, router_id='192.168.0.1',
                            ctn_image_name=gobgp_ctn_image_name,
                            log_level=parser_option.gobgp_log_level)
        q1 = QuaggaBGPContainer(name='q1', asn=65001, router_id='192.168.0.2')
        q2 = QuaggaBGPContainer(name='q2', asn=65002, router_id='192.168.0.3')
        q3 = QuaggaBGPContainer(name='q3', asn=65003, router_id='192.168.0.4')

        qs = [q1, q2, q3]
        ctns = [g1, q1, q2, q3]

        # advertise a route from q1, q2, q3
        for idx, q in enumerate(qs):
            route = '10.0.{0}.0/24'.format(idx + 1)
            q.add_route(route)

        initial_wait_time = max(ctn.run() for ctn in ctns)

        time.sleep(initial_wait_time)

        for q in qs:
            g1.add_peer(q, reload_config=False, passwd='passwd')
            q.add_peer(g1, passwd='passwd', passive=True)

        g1.create_config()
        g1.reload_config()

        cls.gobgp = g1
        cls.quaggas = {'q1': q1, 'q2': q2, 'q3': q3}

    # test each neighbor state is turned establish
    def test_01_neighbor_established(self):
        for q in self.quaggas.itervalues():
            self.gobgp.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=q)

    def test_02_check_gobgp_global_rib(self):
        for q in self.quaggas.itervalues():
            # paths expected to exist in gobgp's global rib
            routes = q.routes.keys()
            timeout = 120
            interval = 1
            count = 0

            while True:
                # gobgp's global rib
                state = self.gobgp.get_neighbor_state(q)
                self.assertEqual(state, BGP_FSM_ESTABLISHED)
                global_rib = [p['prefix'] for p in self.gobgp.get_global_rib()]

                for p in global_rib:
                    if p in routes:
                        routes.remove(p)

                if len(routes) == 0:
                    break

                time.sleep(interval)
                count += interval
                if count >= timeout:
                    raise Exception('timeout')

    # check gobgp properly add it's own asn to aspath
    def test_03_check_gobgp_adj_out_rib(self):
        for q in self.quaggas.itervalues():
            for path in self.gobgp.get_adj_rib_out(q):
                asns = path['aspath']
                self.assertTrue(self.gobgp.asn in asns)

    # check routes are properly advertised to all BGP speaker
    def test_04_check_quagga_global_rib(self):
        interval = 1
        timeout = int(120 / interval)
        for q in self.quaggas.itervalues():
            done = False
            for _ in range(timeout):
                if done:
                    break
                global_rib = q.get_global_rib()
                global_rib = [p['prefix'] for p in global_rib]
                if len(global_rib) < len(self.quaggas):
                    time.sleep(interval)
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

    def test_05_add_quagga(self):
        q4 = QuaggaBGPContainer(name='q4', asn=65004, router_id='192.168.0.5')
        self.quaggas['q4'] = q4

        q4.add_route('10.0.4.0/24')

        initial_wait_time = q4.run()
        time.sleep(initial_wait_time)
        self.gobgp.add_peer(q4)
        q4.add_peer(self.gobgp)

        self.gobgp.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=q4)

    def test_06_check_global_rib(self):
        self.test_02_check_gobgp_global_rib()
        self.test_04_check_quagga_global_rib()

    def test_07_stop_one_quagga(self):
        g1 = self.gobgp
        q4 = self.quaggas['q4']
        q4.stop()
        self.gobgp.wait_for(expected_state=BGP_FSM_ACTIVE, peer=q4)

        g1.del_peer(q4)
        del self.quaggas['q4']

    # check gobgp properly send withdrawal message with q4's route
    def test_08_check_global_rib(self):
        self.test_02_check_gobgp_global_rib()
        self.test_04_check_quagga_global_rib()

    def test_09_add_distant_relative(self):
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
                  'med': 200}
        q2.add_policy(med200, self.gobgp, 'out')
        med100 = {'name': 'med100',
                  'type': 'permit',
                  'match': '0.0.0.0/0',
                  'med': 100}
        q3.add_policy(med100, self.gobgp, 'out')

        q5.add_route('10.0.6.0/24')

        self.gobgp.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=q2)
        self.gobgp.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=q3)
        q2.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=q5)
        q3.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=q5)

        timeout = 120
        interval = 1
        count = 0
        while True:
            paths = self.gobgp.get_adj_rib_out(q1, '10.0.6.0/24')
            if len(paths) > 0:
                path = paths[0]
                print "{0}'s nexthop is {1}".format(path['nlri']['prefix'],
                                                    path['nexthop'])
                n_addrs = [i[1].split('/')[0] for i in self.gobgp.ip_addrs]
                if path['nexthop'] in n_addrs:
                    break

            time.sleep(interval)
            count += interval
            if count >= timeout:
                raise Exception('timeout')

    def test_10_originate_path(self):
        self.gobgp.add_route('10.10.0.0/24')
        dst = self.gobgp.get_global_rib('10.10.0.0/24')
        self.assertTrue(len(dst) == 1)
        self.assertTrue(len(dst[0]['paths']) == 1)
        path = dst[0]['paths'][0]
        self.assertTrue(path['nexthop'] == '0.0.0.0')
        self.assertTrue(len(path['aspath']) == 0)

    def test_11_check_adj_rib_out(self):
        for q in self.quaggas.itervalues():
            paths = self.gobgp.get_adj_rib_out(q, '10.10.0.0/24')
            self.assertTrue(len(paths) == 1)
            path = paths[0]
            peer_info = self.gobgp.peers[q]
            local_addr = peer_info['local_addr'].split('/')[0]
            self.assertTrue(path['nexthop'] == local_addr)
            self.assertTrue(path['aspath'] == [self.gobgp.asn])

    def test_12_disable_peer(self):
        q1 = self.quaggas['q1']
        self.gobgp.disable_peer(q1)
        self.gobgp.wait_for(expected_state=BGP_FSM_IDLE, peer=q1)

        time.sleep(3)

        for route in q1.routes.iterkeys():
            dst = self.gobgp.get_global_rib(route)
            self.assertTrue(len(dst) == 0)

            for q in self.quaggas.itervalues():
                if q is q1:
                    continue
                paths = self.gobgp.get_adj_rib_out(q, route)
                self.assertTrue(len(paths) == 0)

    def test_13_enable_peer(self):
        q1 = self.quaggas['q1']
        self.gobgp.enable_peer(q1)
        self.gobgp.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=q1)

    def test_14_check_adj_rib_out(self):
        self.test_11_check_adj_rib_out()

    def test_15_check_active_connection(self):
        g1 = self.gobgp
        g2 = GoBGPContainer(name='g2', asn=65000, router_id='192.168.0.7',
                            ctn_image_name=self.gobgp.image,
                            log_level=parser_option.gobgp_log_level)
        time.sleep(g2.run())
        self.quaggas['g2'] = g2
        g2.add_peer(g1, passive=True)
        g1.add_peer(g2)
        g1.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=g2)

    def test_16_check_local_pref_and_med_handling(self):
        g1 = self.gobgp
        g1.add_route('10.20.0.0/24', local_pref=1000, med=2000)
        # iBGP peer
        g2 = self.quaggas['g2']
        paths = g2.get_global_rib('10.20.0.0/24')
        self.assertTrue(len(paths) == 1)
        self.assertTrue(len(paths[0]['paths']) == 1)
        path = paths[0]['paths'][0]
        local_pref = extract_path_attribute(path, BGP_ATTR_TYPE_LOCAL_PREF)
        self.assertTrue(local_pref['value'] == 1000)
        med = extract_path_attribute(path, BGP_ATTR_TYPE_MULTI_EXIT_DISC)
        self.assertTrue(med['metric'] == 2000)

        # eBGP peer
        q1 = self.quaggas['q1']
        paths = q1.get_global_rib('10.20.0.0/24')
        self.assertTrue(len(paths) == 1)
        path = paths[0]
        local_pref = extract_path_attribute(path, BGP_ATTR_TYPE_LOCAL_PREF)
        # local_pref's default value is 100
        self.assertTrue(local_pref['value'] == 100)
        med = extract_path_attribute(path, BGP_ATTR_TYPE_MULTI_EXIT_DISC)
        self.assertTrue(med['metric'] == 2000)

    def test_17_check_shutdown(self):
        g1 = self.gobgp
        q1 = self.quaggas['q1']
        q2 = self.quaggas['q2']
        q3 = self.quaggas['q3']

        q2.add_route('20.0.0.0/24')
        q3.add_route('20.0.0.0/24')

        self.test_01_neighbor_established()

        self.test_02_check_gobgp_global_rib()

        paths = q1.get_global_rib('20.0.0.0/24')
        self.assertTrue(len(paths) == 1)
        n_addrs = [i[1].split('/')[0] for i in self.gobgp.ip_addrs]
        self.assertTrue(paths[0]['nexthop'] in n_addrs)

        q3.stop()

        time.sleep(3)

        paths = q1.get_global_rib('20.0.0.0/24')
        self.assertTrue(len(paths) == 1)
        self.assertTrue(paths[0]['nexthop'] in n_addrs)

        g1.del_peer(q3)
        del self.quaggas['q3']

    def test_18_check_withdrawal(self):
        g1 = self.gobgp
        q1 = self.quaggas['q1']
        q2 = self.quaggas['q2']

        g1.add_route('30.0.0.0/24')
        q1.add_route('30.0.0.0/24')

        self.test_01_neighbor_established()

        self.test_02_check_gobgp_global_rib()

        paths = g1.get_adj_rib_out(q1, '30.0.0.0/24')
        self.assertTrue(len(paths) == 1)
        self.assertTrue('source-id' not in paths[0])
        paths = g1.get_adj_rib_out(q2, '30.0.0.0/24')
        self.assertTrue(len(paths) == 1)
        self.assertTrue('source-id' not in paths[0])

        g1.local('gobgp global rib del 30.0.0.0/24')

        paths = g1.get_adj_rib_out(q1, '30.0.0.0/24')
        self.assertTrue(len(paths) == 0)
        paths = g1.get_adj_rib_out(q2, '30.0.0.0/24')
        self.assertTrue(len(paths) == 1)
        self.assertTrue(paths[0]['source-id'] == '192.168.0.2')

    def test_19_check_grpc_add_neighbor(self):
        g1 = self.gobgp
        e1 = ExaBGPContainer(name='e1', asn=65000, router_id='192.168.0.7')
        time.sleep(e1.run())
        e1.add_peer(g1)
        self.quaggas['e1'] = e1
        n = e1.peers[g1]['local_addr'].split('/')[0]
        g1.local('gobgp n add {0} as 65000'.format(n))
        g1.add_peer(e1, reload_config=False)

        g1.wait_for(expected_state=BGP_FSM_ESTABLISHED, peer=e1)

    def test_20_check_grpc_del_neighbor(self):
        g1 = self.gobgp
        e1 = self.quaggas['e1']
        n = e1.peers[g1]['local_addr'].split('/')[0]
        g1.local('gobgp n del {0}'.format(n))
        g1.del_peer(e1, reload_config=False)

    def test_21_check_withdrawal_2(self):
        g1 = self.gobgp
        g2 = self.quaggas['g2']

        prefix = '40.10.0.0/24'
        g1.add_route(prefix)
        wait_for_completion(lambda: len(g1.get_global_rib(prefix)) == 1)
        wait_for_completion(lambda: len(g2.get_global_rib(prefix)) == 1)

        r = g2.local('gobgp monitor global rib -j', stream=True, tty=False)

        g1.local('gobgp global rib del 40.10.0.0/24')
        del g1.routes[prefix]

        wait_for_completion(lambda: len(g1.get_global_rib(prefix)) == 0)
        wait_for_completion(lambda: len(g2.get_global_rib(prefix)) == 0)

        ret = json.loads(r.next())
        self.assertTrue(ret[0]['nlri']['prefix'] == prefix)
        self.assertTrue('withdrawal' in ret[0])

    def test_22_check_cli_sorted(self):
        g1 = self.gobgp
        cnt = 0

        def next_prefix():
            for i in range(100, 105):
                for j in range(100, 105):
                    yield '{0}.{1}.0.0/24'.format(i, j)

        for p in next_prefix():
            g1.local('gobgp global rib add {0}'.format(p))
            cnt += 1

        cnt2 = 0
        g = next_prefix()
        n = g.next()
        for path in g1.local("gobgp global rib", capture=True).split('\n')[1:]:
            if [elem for elem in path.split(' ') if elem != ''][1] == n:
                try:
                    cnt2 += 1
                    n = g.next()
                except StopIteration:
                    break

        self.assertTrue(cnt == cnt2)

    def test_23_check_withdrawal3(self):
        gobgp_ctn_image_name = parser_option.gobgp_image
        g1 = self.gobgp
        g3 = GoBGPContainer(name='g3', asn=65006, router_id='192.168.0.8',
                            ctn_image_name=gobgp_ctn_image_name,
                            log_level=parser_option.gobgp_log_level)
        g4 = GoBGPContainer(name='g4', asn=65007, router_id='192.168.0.9',
                            ctn_image_name=gobgp_ctn_image_name,
                            log_level=parser_option.gobgp_log_level)

        initial_wait_time = max(ctn.run() for ctn in [g3, g4])
        time.sleep(initial_wait_time)

        self.quaggas = {'g3': g3, 'g4': g4}

        g3.local('gobgp global rib add 50.0.0.0/24')

        g1.add_peer(g3, passive=True)
        g3.add_peer(g1)
        g1.add_peer(g4, passive=True)
        g4.add_peer(g1)

        self.test_01_neighbor_established()

        self.test_02_check_gobgp_global_rib()

        g4.local('gobgp global rib add 50.0.0.0/24 med 10')

        paths = g1.get_adj_rib_out(g3, '50.0.0.0/24')
        self.assertTrue(len(paths) == 0)
        paths = g1.get_adj_rib_out(g4, '50.0.0.0/24')
        self.assertTrue(len(paths) == 1)
        self.assertTrue(paths[0]['source-id'] == '192.168.0.8')

        g3.local('gobgp global rib del 50.0.0.0/24')

        paths = g1.get_adj_rib_out(g3, '50.0.0.0/24')
        self.assertTrue(len(paths) == 1)
        self.assertTrue(paths[0]['source-id'] == '192.168.0.9')
        paths = g1.get_adj_rib_out(g4, '50.0.0.0/24')
        self.assertTrue(len(paths) == 0)


if __name__ == '__main__':
    output = local("which docker 2>&1 > /dev/null ; echo $?", capture=True)
    if int(output) is not 0:
        print "docker not found"
        sys.exit(1)

    nose.main(argv=sys.argv, addplugins=[OptionParser()],
              defaultTest=sys.argv[0])
