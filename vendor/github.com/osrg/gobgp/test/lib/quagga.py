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

import re

from fabric import colors
from fabric.utils import indent
import netaddr

from lib.base import (
    wait_for_completion,
    BGPContainer,
    OSPFContainer,
    CmdBuffer,
    BGP_FSM_IDLE,
    BGP_FSM_ACTIVE,
    BGP_FSM_ESTABLISHED,
    BGP_ATTR_TYPE_MULTI_EXIT_DISC,
    BGP_ATTR_TYPE_LOCAL_PREF,
)


class QuaggaBGPContainer(BGPContainer):

    WAIT_FOR_BOOT = 1
    SHARED_VOLUME = '/etc/quagga'

    def __init__(self, name, asn, router_id, ctn_image_name='osrg/quagga', bgpd_config=None, zebra=False):
        super(QuaggaBGPContainer, self).__init__(name, asn, router_id,
                                                 ctn_image_name)
        self.shared_volumes.append((self.config_dir, self.SHARED_VOLUME))
        self.zebra = zebra

        # bgp_config is equivalent to config.BgpConfigSet structure
        # Example:
        # bgpd_config = {
        #     'global': {
        #         'confederation': {
        #             'identifier': 10,
        #             'peers': [65001],
        #         },
        #     },
        # }
        self.bgpd_config = bgpd_config or {}

    def _get_enabled_daemons(self):
        daemons = ['bgpd']
        if self.zebra:
            daemons.append('zebra')
        return daemons

    def _is_running(self):
        def f(d):
            return self.local(
                'vtysh -d {0} -c "show version"'
                ' > /dev/null 2>&1; echo $?'.format(d), capture=True) == '0'

        return all([f(d) for d in self._get_enabled_daemons()])

    def _wait_for_boot(self):
        wait_for_completion(self._is_running)

    def run(self):
        super(QuaggaBGPContainer, self).run()
        self._wait_for_boot()
        return self.WAIT_FOR_BOOT

    def get_global_rib(self, prefix='', rf='ipv4'):
        rib = []
        if prefix != '':
            return self.get_global_rib_with_prefix(prefix, rf)

        out = self.vtysh('show bgp {0} unicast'.format(rf), config=False)
        if out.startswith('No BGP network exists'):
            return rib

        for line in out.split('\n')[6:-2]:
            line = line[3:]

            p = line.split()[0]
            if '/' not in p:
                continue

            rib.extend(self.get_global_rib_with_prefix(p, rf))

        return rib

    def get_global_rib_with_prefix(self, prefix, rf):
        rib = []

        lines = [line.strip() for line in self.vtysh('show bgp {0} unicast {1}'.format(rf, prefix), config=False).split('\n')]

        if lines[0] == '% Network not in table':
            return rib

        lines = lines[2:]

        if lines[0].startswith('Not advertised'):
            lines.pop(0)  # another useless line
        elif lines[0].startswith('Advertised to non peer-group peers:'):
            lines = lines[2:]  # other useless lines
        else:
            raise Exception('unknown output format {0}'.format(lines))

        while len(lines) > 0:
            if lines[0] == 'Local':
                aspath = []
            else:
                aspath = [int(re.sub('\D', '', asn)) for asn in lines[0].split()]

            nexthop = lines[1].split()[0].strip()
            info = [s.strip(',') for s in lines[2].split()]
            attrs = []
            ibgp = False
            best = False
            if 'metric' in info:
                med = info[info.index('metric') + 1]
                attrs.append({'type': BGP_ATTR_TYPE_MULTI_EXIT_DISC, 'metric': int(med)})
            if 'localpref' in info:
                localpref = info[info.index('localpref') + 1]
                attrs.append({'type': BGP_ATTR_TYPE_LOCAL_PREF, 'value': int(localpref)})
            if 'internal' in info:
                ibgp = True
            if 'best' in info:
                best = True

            rib.append({'prefix': prefix, 'nexthop': nexthop,
                        'aspath': aspath, 'attrs': attrs, 'ibgp': ibgp, 'best': best})

            lines = lines[5:]

        return rib

    def get_neighbor_state(self, peer):
        if peer not in self.peers:
            raise Exception('not found peer {0}'.format(peer.router_id))

        neigh_addr = self.peers[peer]['neigh_addr'].split('/')[0]

        info = [l.strip() for l in self.vtysh('show bgp neighbors {0}'.format(neigh_addr), config=False).split('\n')]

        if not info[0].startswith('BGP neighbor is'):
            raise Exception('unknown format')

        idx1 = info[0].index('BGP neighbor is ')
        idx2 = info[0].index(',')
        n_addr = info[0][idx1 + len('BGP neighbor is '):idx2]
        if n_addr == neigh_addr:
            idx1 = info[2].index('= ')
            state = info[2][idx1 + len('= '):]
            if state.startswith('Idle'):
                return BGP_FSM_IDLE
            elif state.startswith('Active'):
                return BGP_FSM_ACTIVE
            elif state.startswith('Established'):
                return BGP_FSM_ESTABLISHED
            else:
                return state

        raise Exception('not found peer {0}'.format(peer.router_id))

    def send_route_refresh(self):
        self.vtysh('clear ip bgp * soft', config=False)

    def create_config(self):
        self._create_config_bgp()
        if self.zebra:
            self._create_config_zebra()

    def _create_config_bgp(self):

        c = CmdBuffer()
        c << 'hostname bgpd'
        c << 'password zebra'
        c << 'router bgp {0}'.format(self.asn)
        c << 'bgp router-id {0}'.format(self.router_id)
        if any(info['graceful_restart'] for info in self.peers.itervalues()):
            c << 'bgp graceful-restart'

        if 'global' in self.bgpd_config:
            if 'confederation' in self.bgpd_config['global']:
                conf = self.bgpd_config['global']['confederation']['config']
                c << 'bgp confederation identifier {0}'.format(conf['identifier'])
                c << 'bgp confederation peers {0}'.format(' '.join([str(i) for i in conf['member-as-list']]))

        version = 4
        for peer, info in self.peers.iteritems():
            version = netaddr.IPNetwork(info['neigh_addr']).version
            n_addr = info['neigh_addr'].split('/')[0]
            if version == 6:
                c << 'no bgp default ipv4-unicast'
            c << 'neighbor {0} remote-as {1}'.format(n_addr, info['remote_as'])
            # For rapid convergence
            c << 'neighbor {0} advertisement-interval 1'.format(n_addr)
            if info['is_rs_client']:
                c << 'neighbor {0} route-server-client'.format(n_addr)
            for typ, p in info['policies'].iteritems():
                c << 'neighbor {0} route-map {1} {2}'.format(n_addr, p['name'],
                                                             typ)
            if info['passwd']:
                c << 'neighbor {0} password {1}'.format(n_addr, info['passwd'])
            if info['passive']:
                c << 'neighbor {0} passive'.format(n_addr)
            if version == 6:
                c << 'address-family ipv6 unicast'
                c << 'neighbor {0} activate'.format(n_addr)
                c << 'exit-address-family'

        if self.zebra:
            if version == 6:
                c << 'address-family ipv6 unicast'
                c << 'redistribute connected'
                c << 'exit-address-family'
            else:
                c << 'redistribute connected'

        for name, policy in self.policies.iteritems():
            c << 'access-list {0} {1} {2}'.format(name, policy['type'],
                                                  policy['match'])
            c << 'route-map {0} permit 10'.format(name)
            c << 'match ip address {0}'.format(name)
            c << 'set metric {0}'.format(policy['med'])

        c << 'debug bgp as4'
        c << 'debug bgp fsm'
        c << 'debug bgp updates'
        c << 'debug bgp events'
        c << 'log file {0}/bgpd.log'.format(self.SHARED_VOLUME)

        with open('{0}/bgpd.conf'.format(self.config_dir), 'w') as f:
            print colors.yellow('[{0}\'s new bgpd.conf]'.format(self.name))
            print colors.yellow(indent(str(c)))
            f.writelines(str(c))

    def _create_config_zebra(self):
        c = CmdBuffer()
        c << 'hostname zebra'
        c << 'password zebra'
        c << 'log file {0}/zebra.log'.format(self.SHARED_VOLUME)
        c << 'debug zebra packet'
        c << 'debug zebra kernel'
        c << 'debug zebra rib'
        c << 'ipv6 forwarding'
        c << ''

        with open('{0}/zebra.conf'.format(self.config_dir), 'w') as f:
            print colors.yellow('[{0}\'s new zebra.conf]'.format(self.name))
            print colors.yellow(indent(str(c)))
            f.writelines(str(c))

    def vtysh(self, cmd, config=True):
        if not isinstance(cmd, list):
            cmd = [cmd]
        cmd = ' '.join("-c '{0}'".format(c) for c in cmd)
        if config:
            return self.local("vtysh -d bgpd -c 'enable' -c 'conf t' -c 'router bgp {0}' {1}".format(self.asn, cmd), capture=True)
        else:
            return self.local("vtysh -d bgpd {0}".format(cmd), capture=True)

    def reload_config(self):
        for daemon in self._get_enabled_daemons():
            self.local('pkill {0} -SIGHUP'.format(daemon), capture=True)
        self._wait_for_boot()

    def _vtysh_add_route_map(self, path):
        supported_attributes = (
            'next-hop',
            'as-path',
            'community',
            'med',
            'local-pref',
            'extended-community',
        )
        if not any([path[k] for k in supported_attributes]):
            return ''

        c = CmdBuffer(' ')
        route_map_name = 'RM-{0}'.format(path['prefix'])
        c << "vtysh -c 'configure terminal'"
        c << "-c 'route-map {0} permit 10'".format(route_map_name)
        if path['next-hop']:
            if path['rf'] == 'ipv4':
                c << "-c 'set ip next-hop {0}'".format(path['next-hop'])
            elif path['rf'] == 'ipv6':
                c << "-c 'set ipv6 next-hop {0}'".format(path['next-hop'])
            else:
                raise ValueError('Unsupported address family: {0}'.format(path['rf']))
        if path['as-path']:
            as_path = ' '.join([str(n) for n in path['as-path']])
            c << "-c 'set as-path prepend {0}'".format(as_path)
        if path['community']:
            comm = ' '.join(path['community'])
            c << "-c 'set community {0}'".format(comm)
        if path['med']:
            c << "-c 'set metric {0}'".format(path['med'])
        if path['local-pref']:
            c << "-c 'set local-preference {0}'".format(path['local-pref'])
        if path['extended-community']:
            # Note: Currently only RT is supported.
            extcomm = ' '.join(path['extended-community'])
            c << "-c 'set extcommunity rt {0}'".format(extcomm)
        self.local(str(c), capture=True)

        return route_map_name

    def add_route(self, route, rf='ipv4', attribute=None, aspath=None,
                  community=None, med=None, extendedcommunity=None,
                  nexthop=None, matchs=None, thens=None,
                  local_pref=None, identifier=None, reload_config=False):
        if not self._is_running():
            raise RuntimeError('Quagga/Zebra is not yet running')

        if rf not in ('ipv4', 'ipv6'):
            raise ValueError('Unsupported address family: {0}'.format(rf))

        self.routes.setdefault(route, [])
        path = {
            'prefix': route,
            'rf': rf,
            'next-hop': nexthop,
            'as-path': aspath,
            'community': community,
            'med': med,
            'local-pref': local_pref,
            'extended-community': extendedcommunity,
            # Note: The following settings are not yet supported on this
            # implementation.
            'attr': None,
            'identifier': None,
            'matchs': None,
            'thens': None,
        }

        # Prepare route-map before adding prefix
        route_map_name = self._vtysh_add_route_map(path)
        path['route_map'] = route_map_name

        c = CmdBuffer(' ')
        c << "vtysh -c 'configure terminal'"
        c << "-c 'router bgp {0}'".format(self.asn)
        if rf == 'ipv6':
            c << "-c 'address-family ipv6'"
        if route_map_name:
            c << "-c 'network {0} route-map {1}'".format(route, route_map_name)
        else:
            c << "-c 'network {0}'".format(route)
        self.local(str(c), capture=True)

        self.routes[route].append(path)

    def _vtysh_del_route_map(self, path):
        route_map_name = path.get('route_map', '')
        if not route_map_name:
            return

        c = CmdBuffer(' ')
        c << "vtysh -c 'configure terminal'"
        c << "-c 'no route-map {0}'".format(route_map_name)
        self.local(str(c), capture=True)

    def del_route(self, route, identifier=None, reload_config=False):
        if not self._is_running():
            raise RuntimeError('Quagga/Zebra is not yet running')

        path = None
        new_paths = []
        for p in self.routes.get(route, []):
            if p['identifier'] != identifier:
                new_paths.append(p)
            else:
                path = p
        if not path:
            return

        rf = path['rf']
        c = CmdBuffer(' ')
        c << "vtysh -c 'configure terminal'"
        c << "-c 'router bgp {0}'".format(self.asn)
        c << "-c 'address-family {0} unicast'".format(rf)
        c << "-c 'no network {0}'".format(route)
        self.local(str(c), capture=True)

        # Delete route-map after deleting prefix
        self._vtysh_del_route_map(path)

        self.routes[route] = new_paths


class RawQuaggaBGPContainer(QuaggaBGPContainer):
    def __init__(self, name, config, ctn_image_name='osrg/quagga', zebra=False):
        asn = None
        router_id = None
        for line in config.split('\n'):
            line = line.strip()
            if line.startswith('router bgp'):
                asn = int(line[len('router bgp'):].strip())
            if line.startswith('bgp router-id'):
                router_id = line[len('bgp router-id'):].strip()
        if not asn:
            raise Exception('asn not in quagga config')
        if not router_id:
            raise Exception('router-id not in quagga config')
        self.config = config
        super(RawQuaggaBGPContainer, self).__init__(name, asn, router_id,
                                                    ctn_image_name, zebra)

    def create_config(self):
        with open('{0}/bgpd.conf'.format(self.config_dir), 'w') as f:
            print colors.yellow('[{0}\'s new bgpd.conf]'.format(self.name))
            print colors.yellow(indent(self.config))
            f.writelines(self.config)


class QuaggaOSPFContainer(OSPFContainer):
    SHARED_VOLUME = '/etc/quagga'
    ZAPI_V2_IMAGE = 'osrg/quagga'
    ZAPI_V3_IMAGE = 'osrg/quagga:v1.0'

    def __init__(self, name, image=ZAPI_V2_IMAGE, zapi_verion=2,
                 zebra_config=None, ospfd_config=None):
        if zapi_verion != 2:
            image = self.ZAPI_V3_IMAGE
        super(QuaggaOSPFContainer, self).__init__(name, image)
        self.shared_volumes.append((self.config_dir, self.SHARED_VOLUME))

        self.zapi_vserion = zapi_verion

        # Example:
        # zebra_config = {
        #     'interfaces': {  # interface settings
        #         'eth0': [
        #             'ip address 192.168.0.1/24',
        #         ],
        #     },
        #     'routes': [  # static route settings
        #         'ip route 172.16.0.0/16 172.16.0.1',
        #     ],
        # }
        self.zebra_config = zebra_config or {}

        # Example:
        # ospfd_config = {
        #     'redistribute_types': [
        #         'connected',
        #     ],
        #     'networks': {
        #         '192.168.1.0/24': '0.0.0.0',  # <network>: <area>
        #     },
        # }
        self.ospfd_config = ospfd_config or {}

    def run(self):
        super(QuaggaOSPFContainer, self).run()
        # self.create_config() is called in super(...).run()
        self._start_zebra()
        self._start_ospfd()
        return self.WAIT_FOR_BOOT

    def create_config(self):
        self._create_config_zebra()
        self._create_config_ospfd()

    def _create_config_zebra(self):
        c = CmdBuffer()
        c << 'hostname zebra'
        c << 'password zebra'
        for name, settings in self.zebra_config.get('interfaces', {}).items():
            c << 'interface {0}'.format(name)
            for setting in settings:
                c << str(setting)
        for route in self.zebra_config.get('routes', []):
            c << str(route)
        c << 'log file {0}/zebra.log'.format(self.SHARED_VOLUME)
        c << 'debug zebra packet'
        c << 'debug zebra kernel'
        c << 'debug zebra rib'
        c << 'ipv6 forwarding'
        c << ''

        with open('{0}/zebra.conf'.format(self.config_dir), 'w') as f:
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
            self.networks[network] = area  # for superclass
            c << ' network {0} area {1}'.format(network, area)
        c << 'log file {0}/ospfd.log'.format(self.SHARED_VOLUME)
        c << ''

        with open('{0}/ospfd.conf'.format(self.config_dir), 'w') as f:
            print colors.yellow('[{0}\'s new ospfd.conf]'.format(self.name))
            print colors.yellow(indent(str(c)))
            f.writelines(str(c))

    def _start_zebra(self):
        # Do nothing. supervisord will automatically start Zebra daemon.
        return

    def _start_ospfd(self):
        if self.zapi_vserion == 2:
            ospfd_cmd = '/usr/lib/quagga/ospfd'
        else:
            ospfd_cmd = 'ospfd'
        self.local(
            '{0} -f {1}/ospfd.conf'.format(ospfd_cmd, self.SHARED_VOLUME),
            detach=True)
