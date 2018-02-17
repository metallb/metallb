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
from itertools import chain
from threading import Thread
import subprocess
import os

from fabric import colors
from fabric.api import local
from fabric.utils import indent
import netaddr
import toml
import yaml
import collections

from lib.base import (
    wait_for_completion,
    BGPContainer,
    CmdBuffer,
    BGP_ATTR_TYPE_AS_PATH,
    BGP_ATTR_TYPE_NEXT_HOP,
    BGP_ATTR_TYPE_MULTI_EXIT_DISC,
    BGP_ATTR_TYPE_LOCAL_PREF,
    BGP_ATTR_TYPE_COMMUNITIES,
    BGP_ATTR_TYPE_MP_REACH_NLRI,
    community_str,
)


def extract_path_attribute(path, typ):
    for a in path['attrs']:
        if a['type'] == typ:
            return a
    return None


class GoBGPContainer(BGPContainer):

    SHARED_VOLUME = '/root/shared_volume'
    QUAGGA_VOLUME = '/etc/quagga'

    def __init__(self, name, asn, router_id, ctn_image_name='osrg/gobgp',
                 log_level='debug', zebra=False, config_format='toml',
                 zapi_version=2, bgp_config=None, ospfd_config=None):
        super(GoBGPContainer, self).__init__(name, asn, router_id,
                                             ctn_image_name)
        self.shared_volumes.append((self.config_dir, self.SHARED_VOLUME))
        self.quagga_config_dir = '{0}/quagga'.format(self.config_dir)
        self.shared_volumes.append((self.quagga_config_dir, self.QUAGGA_VOLUME))

        self.log_level = log_level
        self.prefix_set = None
        self.neighbor_set = None
        self.bgp_set = None
        self.statements = None
        self.default_policy = None
        self.zebra = zebra
        self.zapi_version = zapi_version
        self.config_format = config_format

        # bgp_config is equivalent to config.BgpConfigSet structure
        # Example:
        # bgp_config = {
        #     'global': {
        #         'confederation': {
        #             'config': {
        #                 'identifier': 10,
        #                 'member-as-list': [65001],
        #             }
        #         },
        #     },
        # }
        self.bgp_config = bgp_config or {}

        # To start OSPFd in GoBGP container, specify 'ospfd_config' as a dict
        # type value.
        # Example:
        # ospfd_config = {
        #     'redistributes': [
        #         'connected',
        #     ],
        #     'networks': {
        #         '192.168.1.0/24': '0.0.0.0',  # <network>: <area>
        #     },
        # }
        self.ospfd_config = ospfd_config or {}

    def _start_gobgp(self, graceful_restart=False):
        c = CmdBuffer()
        c << '#!/bin/bash'
        c << '/go/bin/gobgpd -f {0}/gobgpd.conf -l {1} -p {2} -t {3} > ' \
             '{0}/gobgpd.log 2>&1'.format(self.SHARED_VOLUME, self.log_level, '-r' if graceful_restart else '', self.config_format)

        cmd = 'echo "{0:s}" > {1}/start.sh'.format(c, self.config_dir)
        local(cmd, capture=True)
        cmd = "chmod 755 {0}/start.sh".format(self.config_dir)
        local(cmd, capture=True)
        self.local("{0}/start.sh".format(self.SHARED_VOLUME), detach=True)

    def graceful_restart(self):
        self.local("pkill -INT gobgpd")

    def _start_zebra(self):
        if self.zapi_version == 2:
            daemon_bin = '/usr/lib/quagga/zebra'
        else:
            daemon_bin = 'zebra'
        cmd = '{0} -f {1}/zebra.conf'.format(daemon_bin, self.QUAGGA_VOLUME)
        self.local(cmd, detach=True)

    def _start_ospfd(self):
        if self.zapi_version == 2:
            daemon_bin = '/usr/lib/quagga/ospfd'
        else:
            daemon_bin = 'ospfd'
        cmd = '{0} -f {1}/ospfd.conf'.format(daemon_bin, self.QUAGGA_VOLUME)
        self.local(cmd, detach=True)

    def _get_enabled_quagga_daemons(self):
        daemons = []
        if self.zebra:
            daemons.append('zebra')
            if self.ospfd_config:
                daemons.append('ospfd')
        return daemons

    def _is_running(self):
        return self.local('gobgp global'
                          ' > /dev/null 2>&1; echo $?', capture=True) == '0'

    def _wait_for_boot(self):
        for daemon in self._get_enabled_quagga_daemons():
            def _f_quagga():
                ret = self.local("vtysh -d {0} -c 'show run' > /dev/null 2>&1; echo $?".format(daemon), capture=True)
                return ret == '0'

            wait_for_completion(_f_quagga)

        wait_for_completion(self._is_running)

    def run(self):
        super(GoBGPContainer, self).run()
        if self.zebra:
            self._start_zebra()
            if self.ospfd_config:
                self._start_ospfd()
        self._start_gobgp()
        self._wait_for_boot()
        return self.WAIT_FOR_BOOT

    @staticmethod
    def _get_as_path(path):
        asps = (p['as_paths'] for p in path['attrs']
                if p['type'] == BGP_ATTR_TYPE_AS_PATH and 'as_paths' in p and p['as_paths'] is not None)
        asps = chain.from_iterable(asps)
        asns = (asp['asns'] for asp in asps)
        return list(chain.from_iterable(asns))

    @staticmethod
    def _get_nexthop(path):
        for p in path['attrs']:
            if p['type'] == BGP_ATTR_TYPE_NEXT_HOP or p['type'] == BGP_ATTR_TYPE_MP_REACH_NLRI:
                return p['nexthop']

    @staticmethod
    def _get_local_pref(path):
        for p in path['attrs']:
            if p['type'] == BGP_ATTR_TYPE_LOCAL_PREF:
                return p['value']
        return None

    @staticmethod
    def _get_med(path):
        for p in path['attrs']:
            if p['type'] == BGP_ATTR_TYPE_MULTI_EXIT_DISC:
                return p['metric']
        return None

    @staticmethod
    def _get_community(path):
        for p in path['attrs']:
            if p['type'] == BGP_ATTR_TYPE_COMMUNITIES:
                return [community_str(c) for c in p['communities']]
        return None

    def _get_rib(self, dests_dict):
        dests = []
        for k, v in dests_dict.items():
            for p in v:
                p["nexthop"] = self._get_nexthop(p)
                p["aspath"] = self._get_as_path(p)
                p["local-pref"] = self._get_local_pref(p)
                p["community"] = self._get_community(p)
                p["med"] = self._get_med(p)
                p["prefix"] = k
                path_id = p.get("id", None)
                if path_id:
                    p["identifier"] = p["id"]
            dests.append({'paths': v, 'prefix': k})
        return dests

    def _trigger_peer_cmd(self, cmd, peer):
        peer_addr = self.peer_name(peer)
        cmd = 'gobgp neighbor {0} {1}'.format(peer_addr, cmd)
        self.local(cmd)

    def disable_peer(self, peer):
        self._trigger_peer_cmd('disable', peer)

    def enable_peer(self, peer):
        self._trigger_peer_cmd('enable', peer)

    def reset(self, peer):
        self._trigger_peer_cmd('reset', peer)

    def softreset(self, peer, rf='ipv4', type='in'):
        self._trigger_peer_cmd('softreset{0} -a {1}'.format(type, rf), peer)

    def get_local_rib(self, peer, prefix='', rf='ipv4'):
        peer_addr = self.peer_name(peer)
        cmd = 'gobgp -j neighbor {0} local {1} -a {2}'.format(peer_addr, prefix, rf)
        output = self.local(cmd, capture=True)
        return self._get_rib(json.loads(output))

    def get_global_rib(self, prefix='', rf='ipv4'):
        cmd = 'gobgp -j global rib {0} -a {1}'.format(prefix, rf)
        output = self.local(cmd, capture=True)
        return self._get_rib(json.loads(output))

    def monitor_global_rib(self, queue, rf='ipv4'):
        host = self.ip_addrs[0][1].split('/')[0]

        if not os.path.exists('{0}/gobgp'.format(self.config_dir)):
            self.local('cp /go/bin/gobgp {0}/'.format(self.SHARED_VOLUME))

        args = '{0}/gobgp -u {1} -j monitor global rib -a {2}'.format(self.config_dir, host, rf).split(' ')

        def monitor():
            process = subprocess.Popen(args, stdout=subprocess.PIPE)
            for line in iter(process.stdout.readline, ''):
                p = json.loads(line)[0]
                p["nexthop"] = self._get_nexthop(p)
                p["aspath"] = self._get_as_path(p)
                p["local-pref"] = self._get_local_pref(p)
                p["med"] = self._get_med(p)
                queue.put(p)

        t = Thread(target=monitor)
        t.daemon = True
        t.start()

    def _get_adj_rib(self, adj_type, peer, prefix='', rf='ipv4'):
        peer_addr = self.peer_name(peer)
        cmd = 'gobgp neighbor {0} adj-{1} {2} -a {3} -j'.format(peer_addr,
                                                                adj_type,
                                                                prefix, rf)
        output = self.local(cmd, capture=True)
        ret = [p[0] for p in json.loads(output).itervalues()]
        for p in ret:
            p["nexthop"] = self._get_nexthop(p)
            p["aspath"] = self._get_as_path(p)
            p["prefix"] = p['nlri']['prefix']
            p["local-pref"] = self._get_local_pref(p)
            p["med"] = self._get_med(p)
        return ret

    def get_adj_rib_in(self, peer, prefix='', rf='ipv4'):
        return self._get_adj_rib('in', peer, prefix, rf)

    def get_adj_rib_out(self, peer, prefix='', rf='ipv4'):
        return self._get_adj_rib('out', peer, prefix, rf)

    def get_neighbor(self, peer):
        cmd = 'gobgp -j neighbor {0}'.format(self.peer_name(peer))
        return json.loads(self.local(cmd, capture=True))

    def get_neighbor_state(self, peer):
        return self.get_neighbor(peer)['state']['session-state']

    def clear_policy(self):
        self.policies = {}
        for info in self.peers.itervalues():
            info['policies'] = {}
        self.prefix_set = []
        self.neighbor_set = []
        self.statements = []

    def set_prefix_set(self, ps):
        if not isinstance(ps, list):
            ps = [ps]
        self.prefix_set = ps

    def add_prefix_set(self, ps):
        if self.prefix_set is None:
            self.prefix_set = []
        self.prefix_set.append(ps)

    def set_neighbor_set(self, ns):
        if not isinstance(ns, list):
            ns = [ns]
        self.neighbor_set = ns

    def add_neighbor_set(self, ns):
        if self.neighbor_set is None:
            self.neighbor_set = []
        self.neighbor_set.append(ns)

    def set_bgp_defined_set(self, bs):
        self.bgp_set = bs

    def create_config(self):
        self._create_config_bgp()
        if self.zebra:
            local('mkdir -p {0}'.format(self.quagga_config_dir))
            local('chmod 777 {0}'.format(self.quagga_config_dir))
            self._create_config_zebra()
            if self.ospfd_config:
                self._create_config_ospfd()

    def _merge_dict(self, dct, merge_dct):
        for k, v in merge_dct.iteritems():
            if (k in dct and isinstance(dct[k], dict)
                    and isinstance(merge_dct[k], collections.Mapping)):
                self._merge_dict(dct[k], merge_dct[k])
            else:
                dct[k] = merge_dct[k]

    def _create_config_bgp(self):
        config = {
            'global': {
                'config': {
                    'as': self.asn,
                    'router-id': self.router_id,
                },
                'route-selection-options': {
                    'config': {
                        'external-compare-router-id': True,
                    },
                },
            },
            'neighbors': [],
        }

        self._merge_dict(config, self.bgp_config)

        if self.zebra and self.zapi_version == 2:
            config['global']['use-multiple-paths'] = {'config': {'enabled': True}}

        for peer, info in self.peers.iteritems():
            afi_safi_list = []
            if info['interface'] != '':
                afi_safi_list.append({'config': {'afi-safi-name': 'ipv4-unicast'}})
                afi_safi_list.append({'config': {'afi-safi-name': 'ipv6-unicast'}})
            else:
                version = netaddr.IPNetwork(info['neigh_addr']).version
                if version == 4:
                    afi_safi_list.append({'config': {'afi-safi-name': 'ipv4-unicast'}})
                elif version == 6:
                    afi_safi_list.append({'config': {'afi-safi-name': 'ipv6-unicast'}})
                else:
                    Exception('invalid ip address version. {0}'.format(version))

            if info['vpn']:
                afi_safi_list.append({'config': {'afi-safi-name': 'l3vpn-ipv4-unicast'}})
                afi_safi_list.append({'config': {'afi-safi-name': 'l3vpn-ipv6-unicast'}})
                afi_safi_list.append({'config': {'afi-safi-name': 'l2vpn-evpn'}})
                afi_safi_list.append({'config': {'afi-safi-name': 'rtc'}, 'route-target-membership': {'config': {'deferral-time': 10}}})

            if info['flowspec']:
                afi_safi_list.append({'config': {'afi-safi-name': 'ipv4-flowspec'}})
                afi_safi_list.append({'config': {'afi-safi-name': 'l3vpn-ipv4-flowspec'}})
                afi_safi_list.append({'config': {'afi-safi-name': 'ipv6-flowspec'}})
                afi_safi_list.append({'config': {'afi-safi-name': 'l3vpn-ipv6-flowspec'}})

            neigh_addr = None
            interface = None
            peer_as = None
            if info['interface'] == '':
                neigh_addr = info['neigh_addr'].split('/')[0]
                peer_as = info['remote_as']
            else:
                interface = info['interface']
            n = {
                'config': {
                    'neighbor-address': neigh_addr,
                    'neighbor-interface': interface,
                    'peer-as': peer_as,
                    'auth-password': info['passwd'],
                    'vrf': info['vrf'],
                    'remove-private-as': info['remove_private_as'],
                },
                'afi-safis': afi_safi_list,
                'timers': {
                    'config': {
                        'connect-retry': 10,
                    },
                },
                'transport': {
                    'config': {},
                },
            }

            n['as-path-options'] = {'config': {}}
            if info['allow_as_in'] > 0:
                n['as-path-options']['config']['allow-own-as'] = info['allow_as_in']
            if info['replace_peer_as']:
                n['as-path-options']['config']['replace-peer-as'] = info['replace_peer_as']

            if ':' in info['local_addr']:
                n['transport']['config']['local-address'] = info['local_addr'].split('/')[0]

            if info['passive']:
                n['transport']['config']['passive-mode'] = True

            if info['is_rs_client']:
                n['route-server'] = {'config': {'route-server-client': True}}

            if info['local_as']:
                n['config']['local-as'] = info['local_as']

            if info['prefix_limit']:
                for v in afi_safi_list:
                    v['prefix-limit'] = {'config': {'max-prefixes': info['prefix_limit'], 'shutdown-threshold-pct': 80}}

            if info['graceful_restart'] is not None:
                n['graceful-restart'] = {'config': {'enabled': True, 'restart-time': 20}}
                for afi_safi in afi_safi_list:
                    afi_safi['mp-graceful-restart'] = {'config': {'enabled': True}}

                if info['llgr'] is not None:
                    n['graceful-restart']['config']['restart-time'] = 1
                    n['graceful-restart']['config']['long-lived-enabled'] = True
                    for afi_safi in afi_safi_list:
                        afi_safi['long-lived-graceful-restart'] = {'config': {'enabled': True, 'restart-time': 30}}

            if info['is_rr_client']:
                cluster_id = self.router_id
                if 'cluster_id' in info and info['cluster_id'] is not None:
                    cluster_id = info['cluster_id']
                n['route-reflector'] = {'config': {'route-reflector-client': True,
                                                   'route-reflector-cluster-id': cluster_id}}

            if info['addpath']:
                n['add-paths'] = {'config': {'receive': True,
                                             'send-max': 16}}

            if len(info.get('default-policy', [])) + len(info.get('policies', [])) > 0:
                n['apply-policy'] = {'config': {}}

            for typ, p in info.get('policies', {}).iteritems():
                n['apply-policy']['config']['{0}-policy-list'.format(typ)] = [p['name']]

            def _f(v):
                if v == 'reject':
                    return 'reject-route'
                elif v == 'accept':
                    return 'accept-route'
                raise Exception('invalid default policy type {0}'.format(v))

            for typ, d in info.get('default-policy', {}).iteritems():
                n['apply-policy']['config']['default-{0}-policy'.format(typ)] = _f(d)

            if info['treat_as_withdraw']:
                n['error-handling'] = {'config': {'treat-as-withdraw': True}}

            config['neighbors'].append(n)

        config['defined-sets'] = {}
        if self.prefix_set:
            config['defined-sets']['prefix-sets'] = self.prefix_set

        if self.neighbor_set:
            config['defined-sets']['neighbor-sets'] = self.neighbor_set

        if self.bgp_set:
            config['defined-sets']['bgp-defined-sets'] = self.bgp_set

        policy_list = []
        for p in self.policies.itervalues():
            policy = {'name': p['name']}
            if 'statements' in p:
                policy['statements'] = p['statements']
            policy_list.append(policy)

        if len(policy_list) > 0:
            config['policy-definitions'] = policy_list

        if self.zebra:
            config['zebra'] = {'config': {'enabled': True,
                                          'redistribute-route-type-list': ['connect'],
                                          'version': self.zapi_version}}

        with open('{0}/gobgpd.conf'.format(self.config_dir), 'w') as f:
            print colors.yellow('[{0}\'s new gobgpd.conf]'.format(self.name))
            if self.config_format is 'toml':
                raw = toml.dumps(config)
            elif self.config_format is 'yaml':
                raw = yaml.dump(config)
            elif self.config_format is 'json':
                raw = json.dumps(config)
            else:
                raise Exception('invalid config_format {0}'.format(self.config_format))
            print colors.yellow(indent(raw))
            f.write(raw)

    def _create_config_zebra(self):
        c = CmdBuffer()
        c << 'hostname zebra'
        c << 'password zebra'
        c << 'log file {0}/zebra.log'.format(self.QUAGGA_VOLUME)
        c << 'debug zebra packet'
        c << 'debug zebra kernel'
        c << 'debug zebra rib'
        c << 'ipv6 forwarding'
        c << ''

        with open('{0}/zebra.conf'.format(self.quagga_config_dir), 'w') as f:
            print colors.yellow('[{0}\'s new zebra.conf]'.format(self.name))
            print colors.yellow(indent(str(c)))
            f.writelines(str(c))

    def _create_config_ospfd(self):
        c = CmdBuffer()
        c << 'hostname ospfd'
        c << 'password zebra'
        c << 'router ospf'
        for redistribute in self.ospfd_config.get('redistributes', []):
            c << ' redistribute {0}'.format(redistribute)
        for network, area in self.ospfd_config.get('networks', {}).items():
            c << ' network {0} area {1}'.format(network, area)
        c << 'log file {0}/ospfd.log'.format(self.QUAGGA_VOLUME)
        c << ''

        with open('{0}/ospfd.conf'.format(self.quagga_config_dir), 'w') as f:
            print colors.yellow('[{0}\'s new ospfd.conf]'.format(self.name))
            print colors.yellow(indent(str(c)))
            f.writelines(str(c))

    def reload_config(self):
        for daemon in self._get_enabled_quagga_daemons():
            self.local('pkill {0} -SIGHUP'.format(daemon), capture=True)
        self.local('pkill gobgpd -SIGHUP', capture=True)
        self._wait_for_boot()

    def add_route(self, route, rf='ipv4', attribute=None, aspath=None,
                  community=None, med=None, extendedcommunity=None,
                  nexthop=None, matchs=None, thens=None,
                  local_pref=None, identifier=None, reload_config=False):
        if not self._is_running():
            raise RuntimeError('GoBGP is not yet running')

        self.routes.setdefault(route, [])
        path = {
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
        }

        c = CmdBuffer(' ')
        c << 'gobgp global rib -a {0} add'.format(rf)
        if rf in ('ipv4', 'ipv6'):
            c << route
            if path['identifier']:
                c << 'identifier {0}'.format(path['identifier'])
            if path['next-hop']:
                c << 'nexthop {0}'.format(path['next-hop'])
            if path['local-pref']:
                c << 'local-pref {0}'.format(path['local-pref'])
            if path['med']:
                c << 'med {0}'.format(path['med'])
            if path['community']:
                comm = str(path['community'])
                if isinstance(path['community'], (list, tuple)):
                    comm = ','.join(path['community'])
                c << 'community {0}'.format(comm)
        elif rf.endswith('-flowspec'):
            c << 'match {0}'.format(' '.join(path['matchs']))
            c << 'then {0}'.format(' '.join(path['thens']))
        else:
            raise Exception('unsupported address family: {0}'.format(rf))
        self.local(str(c), capture=True)

        self.routes[route].append(path)

    def del_route(self, route, identifier=None, reload_config=True):
        if not self._is_running():
            raise RuntimeError('GoBGP is not yet running')

        if route not in self.routes:
            return

        new_paths = []
        for path in self.routes[route]:
            if path['identifier'] != identifier:
                new_paths.append(path)
            else:
                r = CmdBuffer(' ')
                r << 'gobgp global -a {0}'.format(path['rf'])
                prefix = path['prefix']
                if path['rf'].endswith('-flowspec'):
                    prefix = 'match {0}'.format(' '.join(path['matchs']))
                r << 'rib del {0}'.format(prefix)
                if identifier:
                    r << 'identifier {0}'.format(identifier)
                cmd = str(r)
                self.local(cmd, capture=True)
        self.routes[route] = new_paths
        # no need to reload config


class RawGoBGPContainer(GoBGPContainer):
    def __init__(self, name, config, ctn_image_name='osrg/gobgp',
                 log_level='debug', zebra=False, config_format='yaml'):
        if config_format is 'toml':
            d = toml.loads(config)
        elif config_format is 'yaml':
            d = yaml.load(config)
        elif config_format is 'json':
            d = json.loads(config)
        else:
            raise Exception('invalid config format {0}'.format(config_format))
        asn = d['global']['config']['as']
        router_id = d['global']['config']['router-id']
        self.config = config
        super(RawGoBGPContainer, self).__init__(name, asn, router_id,
                                                ctn_image_name, log_level,
                                                zebra, config_format)

    def create_config(self):
        with open('{0}/gobgpd.conf'.format(self.config_dir), 'w') as f:
            print colors.yellow('[{0}\'s new gobgpd.conf]'.format(self.name))
            print colors.yellow(indent(self.config))
            f.write(self.config)
