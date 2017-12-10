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
from fabric.api import local

from lib.base import (
    BGPContainer,
    CmdBuffer,
)


class BagpipeContainer(BGPContainer):

    SHARED_VOLUME = '/root/shared_volume'

    def __init__(self, name, asn, router_id,
                 ctn_image_name='yoshima/bagpipe-bgp'):
        super(BagpipeContainer, self).__init__(name, asn, router_id,
                                               ctn_image_name)
        self.shared_volumes.append((self.config_dir, self.SHARED_VOLUME))

    def run(self):
        super(BagpipeContainer, self).run()
        cmd = CmdBuffer(' ')
        cmd << 'docker exec'
        cmd << '{0} cp {1}/bgp.conf'.format(self.name, self.SHARED_VOLUME)
        cmd << '/etc/bagpipe-bgp/'
        local(str(cmd), capture=True)
        cmd = 'docker exec {0} service bagpipe-bgp start'.format(self.name)
        local(cmd, capture=True)

    def create_config(self):
        c = CmdBuffer()
        c << '[BGP]'
        if len(self.ip_addrs) > 0:
            c << 'local_address={0}'.format(self.ip_addrs[0][1].split('/')[0])
        for info in self.peers.values():
            c << 'peers={0}'.format(info['neigh_addr'].split('/')[0])
        c << 'my_as={0}'.format(self.asn)
        c << 'enable_rtc=True'
        c << '[API]'
        c << 'api_host=localhost'
        c << 'api_port=8082'
        c << '[DATAPLANE_DRIVER_IPVPN]'
        c << 'dataplane_driver = DummyDataplaneDriver'
        c << '[DATAPLANE_DRIVER_EVPN]'
        c << 'dataplane_driver = DummyDataplaneDriver'

        with open('{0}/bgp.conf'.format(self.config_dir), 'w') as f:
            print colors.yellow(str(c))
            f.writelines(str(c))

    def reload_config(self):
        cmd = CmdBuffer(' ')
        cmd << 'docker exec'
        cmd << '{0} cp {1}/bgp.conf'.format(self.name, self.SHARED_VOLUME)
        cmd << '/etc/bagpipe-bgp/'
        local(str(cmd), capture=True)
        cmd = 'docker exec {0} service bagpipe-bgp restart'.format(self.name)
        local(cmd, capture=True)

    def pipework(self, bridge, ip_addr, intf_name=""):
        super(BagpipeContainer, self).pipework(bridge, ip_addr, intf_name)
        self.create_config()
        if self.is_running:
            self.reload_config()
