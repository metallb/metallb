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

import time

from fabric import colors
from fabric.api import local
from fabric.utils import indent

from lib.base import (
    BGPContainer,
    CmdBuffer,
    try_several_times,
)


class BirdContainer(BGPContainer):

    WAIT_FOR_BOOT = 1
    SHARED_VOLUME = '/etc/bird'

    def __init__(self, name, asn, router_id, ctn_image_name='osrg/bird'):
        super(BirdContainer, self).__init__(name, asn, router_id,
                                            ctn_image_name)
        self.shared_volumes.append((self.config_dir, self.SHARED_VOLUME))

    def _start_bird(self):
        c = CmdBuffer()
        c << '#!/bin/bash'
        c << 'bird'
        cmd = 'echo "{0:s}" > {1}/start.sh'.format(c, self.config_dir)
        local(cmd)
        cmd = 'chmod 755 {0}/start.sh'.format(self.config_dir)
        local(cmd)
        self.local('{0}/start.sh'.format(self.SHARED_VOLUME))

    def run(self):
        super(BirdContainer, self).run()
        self.reload_config()
        return self.WAIT_FOR_BOOT

    def create_config(self):
        c = CmdBuffer()
        c << 'router id {0};'.format(self.router_id)
        for peer, info in self.peers.iteritems():
            c << 'protocol bgp {'
            c << '  local as {0};'.format(self.asn)
            n_addr = info['neigh_addr'].split('/')[0]
            c << '  neighbor {0} as {1};'.format(n_addr, peer.asn)
            c << '  multihop;'
            c << '}'

        with open('{0}/bird.conf'.format(self.config_dir), 'w') as f:
            print colors.yellow('[{0}\'s new bird.conf]'.format(self.name))
            print colors.yellow(indent(str(c)))
            f.writelines(str(c))

    def reload_config(self):
        if len(self.peers) == 0:
            return

        def _reload():
            def _is_running():
                ps = self.local('ps', capture=True)
                running = False
                for line in ps.split('\n')[1:]:
                    if 'bird' in line:
                        running = True
                return running
            if _is_running():
                self.local('birdc configure')
            else:
                self._start_bird()
            time.sleep(1)
            if not _is_running():
                raise RuntimeError()
        try_several_times(_reload)


class RawBirdContainer(BirdContainer):
    def __init__(self, name, config, ctn_image_name='osrg/bird'):
        asn = None
        router_id = None
        for line in config.split('\n'):
            line = line.strip()
            if line.startswith('local as'):
                asn = int(line[len('local as'):].strip('; '))
            if line.startswith('router id'):
                router_id = line[len('router id'):].strip('; ')
        if not asn:
            raise Exception('asn not in bird config')
        if not router_id:
            raise Exception('router-id not in bird config')
        self.config = config
        super(RawBirdContainer, self).__init__(name, asn, router_id,
                                               ctn_image_name)

    def create_config(self):
        with open('{0}/bird.conf'.format(self.config_dir), 'w') as f:
            print colors.yellow('[{0}\'s new bird.conf]'.format(self.name))
            print colors.yellow(indent(self.config))
            f.writelines(self.config)
