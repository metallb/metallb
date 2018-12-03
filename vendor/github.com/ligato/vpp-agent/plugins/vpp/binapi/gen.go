//  Copyright (c) 2018 Cisco and/or its affiliates.
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at:
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

//go:generate binapi-generator --input-file=/usr/share/vpp/api/acl.api.json --output-dir=.
//go:generate binapi-generator --input-file=/usr/share/vpp/api/af_packet.api.json --output-dir=.
//go:generate binapi-generator --input-file=/usr/share/vpp/api/bfd.api.json --output-dir=.
//go:generate binapi-generator --input-file=/usr/share/vpp/api/dhcp.api.json --output-dir=.
//go:generate binapi-generator --input-file=/usr/share/vpp/api/interface.api.json --output-dir=.
//go:generate binapi-generator --input-file=/usr/share/vpp/api/ip.api.json --output-dir=.
//go:generate binapi-generator --input-file=/usr/share/vpp/api/ipsec.api.json --output-dir=.
//go:generate binapi-generator --input-file=/usr/share/vpp/api/l2.api.json --output-dir=.
//go:generate binapi-generator --input-file=/usr/share/vpp/api/memif.api.json --output-dir=.
//go:generate binapi-generator --input-file=/usr/share/vpp/api/nat.api.json --output-dir=.
//go:generate binapi-generator --input-file=/usr/share/vpp/api/session.api.json --output-dir=.
//go:generate binapi-generator --input-file=/usr/share/vpp/api/sr.api.json --output-dir=.
//go:generate binapi-generator --input-file=/usr/share/vpp/api/stats.api.json --output-dir=.
//go:generate binapi-generator --input-file=/usr/share/vpp/api/stn.api.json --output-dir=.
//go:generate binapi-generator --input-file=/usr/share/vpp/api/tap.api.json --output-dir=.
//go:generate binapi-generator --input-file=/usr/share/vpp/api/tapv2.api.json --output-dir=.
//go:generate binapi-generator --input-file=/usr/share/vpp/api/vpe.api.json --output-dir=.
//go:generate binapi-generator --input-file=/usr/share/vpp/api/vxlan.api.json --output-dir=.

package binapi
