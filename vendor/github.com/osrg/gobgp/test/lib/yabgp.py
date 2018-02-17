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

from __future__ import absolute_import
from __future__ import print_function

import json
import os

from fabric import colors
from fabric.api import local
from fabric.utils import indent

from lib.base import (
    FLOWSPEC_NAME_TO_TYPE,
    BGPContainer,
    CmdBuffer,
    try_several_times,
    wait_for_completion,
)


class YABGPContainer(BGPContainer):

    WAIT_FOR_BOOT = 1
    SHARED_VOLUME = '/etc/yabgp'

    def __init__(self, name, asn, router_id,
                 ctn_image_name='osrg/yabgp:v0.4.0'):
        super(YABGPContainer, self).__init__(name, asn, router_id,
                                             ctn_image_name)
        self.shared_volumes.append((self.config_dir, self.SHARED_VOLUME))

    def _copy_helper_app(self):
        import lib
        mod_dir = os.path.dirname(lib.__file__)
        local('docker cp {0}/yabgp_helper.py'
              ' {1}:/root/'.format(mod_dir, self.name))

    def _start_yabgp(self):
        self.local(
            'python /root/yabgp_helper.py'
            ' --config-file {0}/yabgp.ini'.format(self.SHARED_VOLUME),
            detach=True)

    def _wait_for_boot(self):
        return wait_for_completion(self._curl_is_running)

    def run(self):
        super(YABGPContainer, self).run()
        # self.create_config() is called in super class
        self._copy_helper_app()
        # To start YABGP, it is required to configure neighbor settings, so
        # here does not start YABGP yet.
        # self._start_yabgp()
        # self._wait_for_boot()
        return self.WAIT_FOR_BOOT

    def create_config(self):
        # Currently, supports only single peer
        c = CmdBuffer('\n')
        c << '[DEFAULT]'
        c << 'log_dir = {0}'.format(self.SHARED_VOLUME)
        c << 'use_stderr = False'
        c << '[message]'
        c << 'write_disk = True'
        c << 'write_dir = {0}/data/bgp/'.format(self.SHARED_VOLUME)
        c << 'format = json'

        if self.peers:
            info = next(iter(self.peers.values()))
            remote_as = info['remote_as']
            neigh_addr = info['neigh_addr'].split('/')[0]
            local_as = info['local_as'] or self.asn
            local_addr = info['local_addr'].split('/')[0]
            c << '[bgp]'
            c << 'afi_safi = ipv4, ipv6, vpnv4, vpnv6, flowspec, evpn'
            c << 'remote_as = {0}'.format(remote_as)
            c << 'remote_addr = {0}'.format(neigh_addr)
            c << 'local_as = {0}'.format(local_as)
            c << 'local_addr = {0}'.format(local_addr)

        with open('{0}/yabgp.ini'.format(self.config_dir), 'w') as f:
            print(colors.yellow('[{0}\'s new yabgp.ini]'.format(self.name)))
            print(colors.yellow(indent(str(c))))
            f.writelines(str(c))

    def reload_config(self):
        if self.peers == 0:
            return

        def _reload():
            def _is_running():
                ps = self.local('ps -ef', capture=True)
                running = False
                for line in ps.split('\n'):
                    if 'yabgp_helper' in line:
                        running = True
                return running

            if _is_running():
                self.local('/usr/bin/pkill -9 python')

            self._start_yabgp()
            self._wait_for_boot()
            if not _is_running():
                raise RuntimeError()

        try_several_times(_reload)

    def _curl_is_running(self):
        c = CmdBuffer(' ')
        c << "curl -X GET"
        c << "-u admin:admin"
        c << "-H 'Content-Type: application/json'"
        c << "http://localhost:8801/v1/"
        c << "> /dev/null 2>&1; echo $?"
        return self.local(str(c), capture=True) == '0'

    def _curl_send_update(self, path, peer):
        c = CmdBuffer(' ')
        c << "curl -X POST"
        c << "-u admin:admin"
        c << "-H 'Content-Type: application/json'"
        c << "http://localhost:8801/v1/peer/{0}/send/update".format(peer)
        c << "-d '{0}'".format(json.dumps(path))
        return json.loads(self.local(str(c), capture=True))

    def _construct_ip_unicast_update(self, rf, prefix, nexthop):
        # YABGP v0.4.0
        #
        # IPv4 Unicast:
        # curl -X POST \
        #      -u admin:admin \
        #      -H 'Content-Type: application/json' \
        #      http://localhost:8801/v1/peer/172.17.0.2/send/update -d '{
        #     "attr": {
        #         "1": 0,
        #         "2": [
        #             [
        #                 2,
        #                 [
        #                     1,
        #                     2,
        #                     3
        #                 ]
        #             ]
        #         ],
        #         "3": "192.0.2.1",
        #         "5": 500
        #     },
        #     "nlri": [
        #         "172.20.1.0/24",
        #         "172.20.2.0/24"
        #     ]
        # }'
        #
        # IPv6 Unicast:
        # curl -X POST \
        #      -u admin:admin \
        #      -H 'Content-Type: application/json' \
        #      http://localhost:8801/v1/peer/172.17.0.2/send/update -d '{
        #     "attr": {
        #         "1": 0,
        #         "2": [
        #             [
        #                 2,
        #                 [
        #                     65502
        #                 ]
        #             ]
        #         ],
        #         "4": 0,
        #         "14": {
        #             "afi_safi": [
        #                 2,
        #                 1
        #             ],
        #             "linklocal_nexthop": "fe80::c002:bff:fe7e:0",
        #             "nexthop": "2001:db8::2",
        #             "nlri": [
        #                 "::2001:db8:2:2/64",
        #                 "::2001:db8:2:1/64",
        #                 "::2001:db8:2:0/64"
        #             ]
        #         }
        #     }
        # }'
        if rf == 'ipv4':
            return {
                "attr": {
                    "3": nexthop,
                },
                "nlri": [prefix],
            }
        elif rf == 'ipv6':
            return {
                "attr": {
                    "14": {  # MP_REACH_NLRI
                        "afi_safi": [2, 1],
                        "nexthop": nexthop,
                        "nlri": [prefix],
                    },
                },
            }
        else:
            raise ValueError(
                'invalid address family for ipv4/ipv6 unicast: %s' % rf)

    def _construct_ip_unicast_withdraw(self, rf, prefix):
        # YABGP v0.4.0
        #
        # IPv4 Unicast:
        # curl -X POST \
        #      -u admin:admin \
        #      -H 'Content-Type: application/json' \
        #      http://localhost:8801/v1/peer/172.17.0.2/send/update -d '{
        #     "withdraw": [
        #         "172.20.1.0/24",
        #         "172.20.2.0/24"
        #     ]
        # }'
        #
        # IPv6 Unicast:
        # curl -X POST \
        #      -u admin:admin \
        #      -H 'Content-Type: application/json' \
        #      http://localhost:8801/v1/peer/172.17.0.2/send/update -d '{
        #     "attr": {
        #         "15": {
        #             "afi_safi": [
        #                 2,
        #                 1
        #             ],
        #             "withdraw": [
        #                 "::2001:db8:2:2/64",
        #                 "::2001:db8:2:1/64",
        #                 "::2001:db8:2:0/64"
        #             ]
        #         }
        #     }
        # }'
        if rf == 'ipv4':
            return {
                "withdraw": [prefix],
            }
        elif rf == 'ipv6':
            return {
                "attr": {
                    "15": {  # MP_UNREACH_NLRI
                        "afi_safi": [2, 1],
                        "withdraw": [prefix],
                    },
                },
            }
        else:
            raise ValueError(
                'invalid address family for ipv4/ipv6 unicast: %s' % rf)

    def _construct_flowspec_match(self, matchs):
        assert isinstance(matchs, (tuple, list))
        ret = {}
        for m in matchs:
            # m = "source-port '!=2 !=22&!=222'"
            # typ = "source-port"
            # args = "'!=2 !=22&!=222'"
            typ, args = m.split(' ', 1)
            # t = 6
            t = FLOWSPEC_NAME_TO_TYPE.get(typ, None)
            if t is None:
                raise ValueError('invalid flowspec match type: %s' % typ)
            # args = "!=2|!=22&!=222"
            args = args.strip("'").strip('"').replace(' ', '|')
            ret[t] = args
        return ret

    def _construct_flowspec_update(self, rf, matchs, thens):
        # YABGP v0.4.0
        #
        # curl -X POST \
        #      -u admin:admin \
        #      -H 'Content-Type: application/json' \
        #      http://localhost:8801/v1/peer/172.17.0.2/send/update -d '{
        #     "attr": {
        #         "1": 0,
        #         "14": {
        #             "afi_safi": [
        #                 1,
        #                 133
        #             ],
        #             "nexthop": "",
        #             "nlri": [
        #                 {
        #                     "1": "10.0.0.0/24"
        #                 }
        #             ]
        #         },
        #         "16": [
        #             "traffic-rate:0:0"
        #         ],
        #         "2": [],
        #         "5": 100
        #     }
        # }'
        #
        # Format of "thens":
        #   "traffic-rate:<AS>:<rate>"
        #   "traffic-marking-dscp:<int value>"
        #   "redirect-nexthop:<int value>"
        #   "redirect-vrf:<RT>"
        thens = thens or []
        if rf == 'ipv4-flowspec':
            afi_safi = [1, 133]
        else:
            raise ValueError('invalid address family for flowspec: %s' % rf)

        return {
            "attr": {
                "14": {  # MP_REACH_NLRI
                    "afi_safi": afi_safi,
                    "nexthop": "",
                    "nlri": [self._construct_flowspec_match(matchs)]
                },
                "16": thens,  # EXTENDED COMMUNITIES
            },
        }

    def _construct_flowspec_withdraw(self, rf, matchs):
        # curl -X POST \
        #      -u admin:admin \
        #      -H 'Content-Type: application/json' \
        #      http://localhost:8801/v1/peer/172.17.0.2/send/update -d '{
        #     "attr": {
        #         "15": {
        #             "afi_safi": [
        #                 1,
        #                 133
        #             ],
        #             "withdraw": [
        #                 {
        #                     "1": "192.88.2.3/24",
        #                     "2": "192.89.1.3/24"
        #                 },
        #                 {
        #                     "1": "192.88.4.3/24",
        #                     "2": "192.89.2.3/24"
        #                 }
        #             ]
        #         }
        #     }
        # }'
        if rf == 'ipv4-flowspec':
            afi_safi = [1, 133]
        else:
            raise ValueError('invalid address family for flowspec: %s' % rf)

        return {
            "attr": {
                "15": {  # MP_UNREACH_NLRI
                    "afi_safi": afi_safi,
                    "withdraw": [self._construct_flowspec_match(matchs)],
                },
            },
        }

    def _update_path_attributes(self, path, aspath=None, med=None,
                                local_pref=None):
        # ORIGIN: Currently support only IGP(0)
        path['attr']['1'] = 0
        # AS_PATH: Currently support only AS_SEQUENCE(2)
        if aspath is None:
            path['attr']['2'] = []
        else:
            path['attr']['2'] = [[2, aspath]]
        # MED
        if med is not None:
            path['attr']['4'] = med
        # LOCAL_PREF
        if local_pref is not None:
            path['attr']['5'] = local_pref
        # TODO:
        # Support COMMUNITY and EXTENDED COMMUNITIES

        return path

    def add_route(self, route, rf='ipv4', attribute=None, aspath=None,
                  community=None, med=None, extendedcommunity=None,
                  nexthop=None, matchs=None, thens=None,
                  local_pref=None, identifier=None, reload_config=True):
        self.routes.setdefault(route, [])

        for info in self.peers.values():
            peer = info['neigh_addr'].split('/')[0]

            if rf in ['ipv4', 'ipv6']:
                nexthop = nexthop or info['local_addr'].split('/')[0]
                path = self._construct_ip_unicast_update(
                    rf, route, nexthop)
            # TODO:
            # Support "evpn" address family
            elif rf in ['ipv4-flowspec', 'ipv6-flowspec']:
                path = self._construct_flowspec_update(
                    rf, matchs, thens)
            else:
                raise ValueError('unsupported address family: %s' % rf)

            self._update_path_attributes(
                path, aspath=aspath, med=med, local_pref=local_pref)

            self._curl_send_update(path, peer)

        self.routes[route].append({
            'prefix': route,
            'rf': rf,
            'attr': attribute,
            'next-hop': nexthop,
            'as-path': aspath,
            'community': community,
            'med': med,
            'local-pref': local_pref,
            'extended-community': extendedcommunity,
            'identifier': identifier,
            'matchs': matchs,
            'thens': thens,
        })

    def del_route(self, route, identifier=None, reload_config=True):
        new_paths = []
        withdraw = None
        for p in self.routes.get(route, []):
            if p['identifier'] != identifier:
                new_paths.append(p)
            else:
                withdraw = p

        if not withdraw:
            return
        rf = withdraw['rf']

        for info in self.peers.values():
            peer = info['neigh_addr'].split('/')[0]

            if rf in ['ipv4', 'ipv6']:
                r = self._construct_ip_unicast_withdraw(rf, route)
            elif rf == 'ipv4-flowspec':
                # NOTE: "ipv6-flowspec" does not seem to be supported with
                # YABGP v0.4.0
                matchs = withdraw['matchs']
                r = self._construct_flowspec_withdraw(rf, matchs)
            else:
                raise ValueError('unsupported address family: %s' % rf)

            self._curl_send_update(r, peer)

        self.routes[route] = new_paths

    def _get_adj_rib(self, peer, in_out='in'):
        peer_addr = self.peer_name(peer)
        c = CmdBuffer(' ')
        c << "curl -X GET"
        c << "-u admin:admin"
        c << "-H 'Content-Type: application/json'"
        c << "http://localhost:8801/v1-ext/peer/{0}/adj-rib-{1}".format(
            peer_addr, in_out)
        return json.loads(self.local(str(c), capture=True))

    def get_adj_rib_in(self, peer, rf='ipv4'):
        # "rf" should be either of;
        #   ipv4, ipv6, vpnv4, vpnv6, flowspec, evpn
        # The same as supported "afi_safi" in yabgp.ini
        ribs = self._get_adj_rib(peer, 'in')
        return ribs.get(rf, {})

    def get_adj_rib_out(self, peer, rf='ipv4'):
        # "rf" should be either of;
        #   ipv4, ipv6, vpnv4, vpnv6, flowspec, evpn
        # The same as supported "afi_safi" in yabgp.ini
        ribs = self._get_adj_rib(peer, 'out')
        return ribs.get(rf, {})
