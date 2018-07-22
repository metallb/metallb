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

from fabric import colors

from lib.base import (
    BGPContainer,
    CmdBuffer,
    try_several_times,
    wait_for_completion,
)


class ExaBGPContainer(BGPContainer):

    SHARED_VOLUME = '/shared_volume'
    PID_FILE = '/var/run/exabgp.pid'

    def __init__(self, name, asn, router_id, ctn_image_name='osrg/exabgp:4.0.5'):
        super(ExaBGPContainer, self).__init__(name, asn, router_id, ctn_image_name)
        self.shared_volumes.append((self.config_dir, self.SHARED_VOLUME))

    def _pre_start_exabgp(self):
        # Create named pipes for "exabgpcli"
        named_pipes = '/run/exabgp.in /run/exabgp.out'
        self.local('mkfifo {0}'.format(named_pipes), capture=True)
        self.local('chmod 777 {0}'.format(named_pipes), capture=True)

    def _start_exabgp(self):
        cmd = CmdBuffer(' ')
        cmd << 'env exabgp.log.destination={0}/exabgpd.log'.format(self.SHARED_VOLUME)
        cmd << 'exabgp.daemon.user=root'
        cmd << 'exabgp.daemon.pid={0}'.format(self.PID_FILE)
        cmd << 'exabgp.tcp.bind="0.0.0.0" exabgp.tcp.port=179'
        cmd << 'exabgp {0}/exabgpd.conf'.format(self.SHARED_VOLUME)
        self.local(str(cmd), detach=True)

    def _wait_for_boot(self):
        def _f():
            ret = self.local('exabgpcli version > /dev/null 2>&1; echo $?', capture=True)
            return ret == '0'

        return wait_for_completion(_f)

    def run(self):
        super(ExaBGPContainer, self).run()
        self._pre_start_exabgp()
        # To start ExaBGP, it is required to configure neighbor settings, so
        # here does not start ExaBGP yet.
        # self._start_exabgp()
        return self.WAIT_FOR_BOOT

    def create_config(self):
        # Manpage of exabgp.conf(5):
        # https://github.com/Exa-Networks/exabgp/blob/master/doc/man/exabgp.conf.5
        cmd = CmdBuffer('\n')
        for peer, info in self.peers.iteritems():
            cmd << 'neighbor {0} {{'.format(info['neigh_addr'].split('/')[0])
            cmd << '    router-id {0};'.format(self.router_id)
            cmd << '    local-address {0};'.format(info['local_addr'].split('/')[0])
            cmd << '    local-as {0};'.format(self.asn)
            cmd << '    peer-as {0};'.format(peer.asn)

            caps = []
            if info['as2']:
                caps.append('        asn4 disable;')
            if info['addpath']:
                caps.append('        add-path send/receive;')
            if caps:
                cmd << '    capability {'
                for cap in caps:
                    cmd << cap
                cmd << '    }'

            if info['passwd']:
                cmd << '    md5-password "{0}";'.format(info['passwd'])

            if info['passive']:
                cmd << '    passive;'
            cmd << '}'

        with open('{0}/exabgpd.conf'.format(self.config_dir), 'w') as f:
            print colors.yellow('[{0}\'s new exabgpd.conf]'.format(self.name))
            print colors.yellow(str(cmd))
            f.write(str(cmd))

    def _is_running(self):
        ret = self.local("test -f {0}; echo $?".format(self.PID_FILE), capture=True)
        return ret == '0'

    def reload_config(self):
        if not self.peers:
            return

        def _reload():
            if self._is_running():
                self.local('/usr/bin/pkill --pidfile {0} && rm -f {0}'.format(self.PID_FILE), capture=True)
            else:
                self._start_exabgp()
                self._wait_for_boot()

            if not self._is_running():
                raise RuntimeError('Could not start ExaBGP')

        try_several_times(_reload)

    def _construct_ip_unicast(self, path):
        cmd = CmdBuffer(' ')
        cmd << str(path['prefix'])
        if path['next-hop']:
            cmd << 'next-hop {0}'.format(path['next-hop'])
        else:
            cmd << 'next-hop self'
        return str(cmd)

    def _construct_flowspec(self, path):
        cmd = CmdBuffer(' ')
        cmd << '{ match {'
        for match in path['matchs']:
            cmd << '{0};'.format(match)
        cmd << '} then {'
        for then in path['thens']:
            cmd << '{0};'.format(then)
        cmd << '} }'
        return str(cmd)

    def _construct_path_attributes(self, path):
        cmd = CmdBuffer(' ')
        if path['as-path']:
            cmd << 'as-path [{0}]'.format(' '.join(str(i) for i in path['as-path']))
        if path['med']:
            cmd << 'med {0}'.format(path['med'])
        if path['local-pref']:
            cmd << 'local-preference {0}'.format(path['local-pref'])
        if path['community']:
            cmd << 'community [{0}]'.format(' '.join(c for c in path['community']))
        if path['extended-community']:
            cmd << 'extended-community [{0}]'.format(path['extended-community'])
        if path['attr']:
            cmd << 'attribute [ {0} ]'.format(path['attr'])
        return str(cmd)

    def _construct_path(self, path, rf='ipv4', is_withdraw=False):
        cmd = CmdBuffer(' ')

        if rf in ['ipv4', 'ipv6']:
            cmd << 'route'
            cmd << self._construct_ip_unicast(path)
        elif rf in ['ipv4-flowspec', 'ipv6-flowspec']:
            cmd << 'flow route'
            cmd << self._construct_flowspec(path)
        else:
            raise ValueError('unsupported address family: %s' % rf)

        if path['identifier']:
            cmd << 'path-information {0}'.format(path['identifier'])

        if not is_withdraw:
            # Withdrawal should not require path attributes
            cmd << self._construct_path_attributes(path)

        return str(cmd)

    def add_route(self, route, rf='ipv4', attribute=None, aspath=None,
                  community=None, med=None, extendedcommunity=None,
                  nexthop=None, matchs=None, thens=None,
                  local_pref=None, identifier=None, reload_config=False):
        if not self._is_running():
            raise RuntimeError('ExaBGP is not yet running')

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

        cmd = CmdBuffer(' ')
        cmd << "exabgpcli 'announce"
        cmd << self._construct_path(path, rf=rf)
        cmd << "'"
        self.local(str(cmd), capture=True)

        self.routes[route].append(path)

    def del_route(self, route, identifier=None, reload_config=False):
        if not self._is_running():
            raise RuntimeError('ExaBGP is not yet running')

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
        cmd = CmdBuffer(' ')
        cmd << "exabgpcli 'withdraw"
        cmd << self._construct_path(path, rf=rf, is_withdraw=True)
        cmd << "'"
        self.local(str(cmd), capture=True)

        self.routes[route] = new_paths

    def _get_adj_rib(self, peer, rf, in_out='in'):
        # IPv4 Unicast:
        # neighbor 172.17.0.2 ipv4 unicast 192.168.100.0/24 path-information 0.0.0.20 next-hop self
        # IPv6 FlowSpec:
        # neighbor 172.17.0.2 ipv6 flow flow destination-ipv6 2002:1::/64/0 source-ipv6 2002:2::/64/0 next-header =udp flow-label >100
        rf_map = {
            'ipv4': ['ipv4', 'unicast'],
            'ipv6': ['ipv6', 'unicast'],
            'ipv4-flowspec': ['ipv4', 'flow'],
            'ipv6-flowspec': ['ipv6', 'flow'],
        }
        assert rf in rf_map
        assert in_out in ('in', 'out')
        peer_addr = self.peer_name(peer)
        lines = self.local('exabgpcli show adj-rib {0}'.format(in_out), capture=True).split('\n')
        # rib = {
        #     <nlri>: [
        #         {
        #             'nlri': <nlri>,
        #             'next-hop': <next-hop>,
        #             ...
        #         },
        #         ...
        #     ],
        # }
        rib = {}
        for line in lines:
            if not line:
                continue
            values = line.split()
            if peer_addr != values[1]:
                continue
            elif rf is not None and rf_map[rf] != values[2:4]:
                continue
            if rf in ('ipv4', 'ipv6'):
                nlri = values[4]
                rib.setdefault(nlri, [])
                path = {k: v for k, v in zip(*[iter(values[5:])] * 2)}
                path['nlri'] = nlri
                rib[nlri].append(path)
            elif rf in ('ipv4-flowspec', 'ipv6-flowspec'):
                # XXX: Missing path attributes?
                nlri = ' '.join(values[5:])
                rib.setdefault(nlri, [])
                path = {'nlri': nlri}
                rib[nlri].append(path)
        return rib

    def get_adj_rib_in(self, peer, rf='ipv4'):
        return self._get_adj_rib(peer, rf, 'in')

    def get_adj_rib_out(self, peer, rf='ipv4'):
        return self._get_adj_rib(peer, rf, 'out')


class RawExaBGPContainer(ExaBGPContainer):
    def __init__(self, name, config, ctn_image_name='osrg/exabgp',
                 exabgp_path=''):
        asn = None
        router_id = None
        for line in config.split('\n'):
            line = line.strip()
            if line.startswith('local-as'):
                asn = int(line[len('local-as'):].strip('; '))
            if line.startswith('router-id'):
                router_id = line[len('router-id'):].strip('; ')
        if not asn:
            raise Exception('asn not in exabgp config')
        if not router_id:
            raise Exception('router-id not in exabgp config')
        self.config = config

        super(RawExaBGPContainer, self).__init__(name, asn, router_id,
                                                 ctn_image_name, exabgp_path)

    def create_config(self):
        with open('{0}/exabgpd.conf'.format(self.config_dir), 'w') as f:
            print colors.yellow('[{0}\'s new exabgpd.conf]'.format(self.name))
            print colors.yellow(self.config)
            f.write(self.config)
