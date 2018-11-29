// Copyright (C) 2014, 2015 Nippon Telegraph and Telephone Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package zebra

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"net"
	"strings"
	"syscall"

	log "github.com/sirupsen/logrus"

	"github.com/osrg/gobgp/pkg/packet/bgp"
)

const (
	HEADER_MARKER     = 255
	FRR_HEADER_MARKER = 254
	INTERFACE_NAMSIZ  = 20
)

// Internal Interface Status.
type INTERFACE_STATUS uint8

const (
	INTERFACE_ACTIVE        INTERFACE_STATUS = 0x01
	INTERFACE_SUB           INTERFACE_STATUS = 0x02
	INTERFACE_LINKDETECTION INTERFACE_STATUS = 0x04
	INTERFACE_VRF_LOOPBACK  INTERFACE_STATUS = 0x08
)

// Interface Link Layer Types.
//go:generate stringer -type=LINK_TYPE
type LINK_TYPE uint32

const (
	LINK_TYPE_UNKNOWN LINK_TYPE = iota
	LINK_TYPE_ETHER
	LINK_TYPE_EETHER
	LINK_TYPE_AX25
	LINK_TYPE_PRONET
	LINK_TYPE_IEEE802
	LINK_TYPE_ARCNET
	LINK_TYPE_APPLETLK
	LINK_TYPE_DLCI
	LINK_TYPE_ATM
	LINK_TYPE_METRICOM
	LINK_TYPE_IEEE1394
	LINK_TYPE_EUI64
	LINK_TYPE_INFINIBAND
	LINK_TYPE_SLIP
	LINK_TYPE_CSLIP
	LINK_TYPE_SLIP6
	LINK_TYPE_CSLIP6
	LINK_TYPE_RSRVD
	LINK_TYPE_ADAPT
	LINK_TYPE_ROSE
	LINK_TYPE_X25
	LINK_TYPE_PPP
	LINK_TYPE_CHDLC
	LINK_TYPE_LAPB
	LINK_TYPE_RAWHDLC
	LINK_TYPE_IPIP
	LINK_TYPE_IPIP6
	LINK_TYPE_FRAD
	LINK_TYPE_SKIP
	LINK_TYPE_LOOPBACK
	LINK_TYPE_LOCALTLK
	LINK_TYPE_FDDI
	LINK_TYPE_SIT
	LINK_TYPE_IPDDP
	LINK_TYPE_IPGRE
	LINK_TYPE_IP6GRE
	LINK_TYPE_PIMREG
	LINK_TYPE_HIPPI
	LINK_TYPE_ECONET
	LINK_TYPE_IRDA
	LINK_TYPE_FCPP
	LINK_TYPE_FCAL
	LINK_TYPE_FCPL
	LINK_TYPE_FCFABRIC
	LINK_TYPE_IEEE802_TR
	LINK_TYPE_IEEE80211
	LINK_TYPE_IEEE80211_RADIOTAP
	LINK_TYPE_IEEE802154
	LINK_TYPE_IEEE802154_PHY
)

const VRF_DEFAULT = 0
const MAXPATH_NUM = 64
const MPLS_MAX_LABLE = 16

func HeaderSize(version uint8) uint16 {
	switch version {
	case 3, 4:
		return 8
	case 5, 6:
		return 10
	default:
		return 6
	}
}

func (t INTERFACE_STATUS) String() string {
	ss := make([]string, 0, 3)
	if t&INTERFACE_ACTIVE > 0 {
		ss = append(ss, "ACTIVE")
	}
	if t&INTERFACE_SUB > 0 {
		ss = append(ss, "SUB")
	}
	if t&INTERFACE_LINKDETECTION > 0 {
		ss = append(ss, "LINKDETECTION")
	}
	if t&INTERFACE_VRF_LOOPBACK > 0 {
		ss = append(ss, "VRF_LOOPBACK")
	}
	return strings.Join(ss, "|")
}

// Interface Connected Address Flags
type INTERFACE_ADDRESS_FLAG uint8

const (
	INTERFACE_ADDRESS_SECONDARY  INTERFACE_ADDRESS_FLAG = 0x01
	INTERFACE_ADDRESS_PEER       INTERFACE_ADDRESS_FLAG = 0x02
	INTERFACE_ADDRESS_UNNUMBERED INTERFACE_ADDRESS_FLAG = 0x04
)

func (t INTERFACE_ADDRESS_FLAG) String() string {
	ss := make([]string, 0, 3)
	if t&INTERFACE_ADDRESS_SECONDARY > 0 {
		ss = append(ss, "SECONDARY")
	}
	if t&INTERFACE_ADDRESS_PEER > 0 {
		ss = append(ss, "PEER")
	}
	if t&INTERFACE_ADDRESS_UNNUMBERED > 0 {
		ss = append(ss, "UNNUMBERED")
	}
	return strings.Join(ss, "|")
}

// Address Family Identifier.
//go:generate stringer -type=AFI
type AFI uint8

const (
	AFI_IP    AFI = 1
	AFI_IP6   AFI = 2
	AFI_ETHER AFI = 3
	AFI_MAX   AFI = 4
)

// Subsequent Address Family Identifier.
//go:generate stringer -type=SAFI
type SAFI uint8

const (
	_ SAFI = iota
	SAFI_UNICAST
	SAFI_MULTICAST
	SAFI_RESERVED_3
	SAFI_MPLS_VPN
	SAFI_MAX
)

// API Types.
//go:generate stringer -type=API_TYPE
type API_TYPE uint16

// For FRRouting version 6. (ZAPI version 6)
const (
	FRR_ZAPI6_INTERFACE_ADD API_TYPE = iota
	FRR_ZAPI6_INTERFACE_DELETE
	FRR_ZAPI6_INTERFACE_ADDRESS_ADD
	FRR_ZAPI6_INTERFACE_ADDRESS_DELETE
	FRR_ZAPI6_INTERFACE_UP
	FRR_ZAPI6_INTERFACE_DOWN
	FRR_ZAPI6_INTERFACE_SET_MASTER
	FRR_ZAPI6_ROUTE_ADD
	FRR_ZAPI6_ROUTE_DELETE
	FRR_ZAPI6_ROUTE_NOTIFY_OWNER
	FRR_ZAPI6_REDISTRIBUTE_ADD
	FRR_ZAPI6_REDISTRIBUTE_DELETE
	FRR_ZAPI6_REDISTRIBUTE_DEFAULT_ADD
	FRR_ZAPI6_REDISTRIBUTE_DEFAULT_DELETE
	FRR_ZAPI6_ROUTER_ID_ADD
	FRR_ZAPI6_ROUTER_ID_DELETE
	FRR_ZAPI6_ROUTER_ID_UPDATE
	FRR_ZAPI6_HELLO
	FRR_ZAPI6_CAPABILITIES
	FRR_ZAPI6_NEXTHOP_REGISTER
	FRR_ZAPI6_NEXTHOP_UNREGISTER
	FRR_ZAPI6_NEXTHOP_UPDATE
	FRR_ZAPI6_INTERFACE_NBR_ADDRESS_ADD
	FRR_ZAPI6_INTERFACE_NBR_ADDRESS_DELETE
	FRR_ZAPI6_INTERFACE_BFD_DEST_UPDATE
	FRR_ZAPI6_IMPORT_ROUTE_REGISTER
	FRR_ZAPI6_IMPORT_ROUTE_UNREGISTER
	FRR_ZAPI6_IMPORT_CHECK_UPDATE
	FRR_ZAPI6_IPV4_ROUTE_IPV6_NEXTHOP_ADD
	FRR_ZAPI6_BFD_DEST_REGISTER
	FRR_ZAPI6_BFD_DEST_DEREGISTER
	FRR_ZAPI6_BFD_DEST_UPDATE
	FRR_ZAPI6_BFD_DEST_REPLAY
	FRR_ZAPI6_REDISTRIBUTE_ROUTE_ADD
	FRR_ZAPI6_REDISTRIBUTE_ROUTE_DEL
	FRR_ZAPI6_VRF_UNREGISTER
	FRR_ZAPI6_VRF_ADD
	FRR_ZAPI6_VRF_DELETE
	FRR_ZAPI6_VRF_LABEL
	FRR_ZAPI6_INTERFACE_VRF_UPDATE
	FRR_ZAPI6_BFD_CLIENT_REGISTER
	FRR_ZAPI6_BFD_CLIENT_DEREGISTER
	FRR_ZAPI6_INTERFACE_ENABLE_RADV
	FRR_ZAPI6_INTERFACE_DISABLE_RADV
	FRR_ZAPI6_IPV4_NEXTHOP_LOOKUP_MRIB
	FRR_ZAPI6_INTERFACE_LINK_PARAMS
	FRR_ZAPI6_MPLS_LABELS_ADD
	FRR_ZAPI6_MPLS_LABELS_DELETE
	FRR_ZAPI6_IPMR_ROUTE_STATS
	FRR_ZAPI6_LABEL_MANAGER_CONNECT
	FRR_ZAPI6_GET_LABEL_CHUNK
	FRR_ZAPI6_RELEASE_LABEL_CHUNK
	FRR_ZAPI6_FEC_REGISTER
	FRR_ZAPI6_FEC_UNREGISTER
	FRR_ZAPI6_FEC_UPDATE
	FRR_ZAPI6_ADVERTISE_DEFAULT_GW
	FRR_ZAPI6_ADVERTISE_SUBNET
	FRR_ZAPI6_ADVERTISE_ALL_VNI
	FRR_ZAPI6_LOCAL_ES_ADD
	FRR_ZAPI6_LOCAL_ES_DEL
	FRR_ZAPI6_VNI_ADD
	FRR_ZAPI6_VNI_DEL
	FRR_ZAPI6_L3VNI_ADD
	FRR_ZAPI6_L3VNI_DEL
	FRR_ZAPI6_REMOTE_VTEP_ADD
	FRR_ZAPI6_REMOTE_VTEP_DEL
	FRR_ZAPI6_MACIP_ADD
	FRR_ZAPI6_MACIP_DEL
	FRR_ZAPI6_IP_PREFIX_ROUTE_ADD
	FRR_ZAPI6_IP_PREFIX_ROUTE_DEL
	FRR_ZAPI6_REMOTE_MACIP_ADD
	FRR_ZAPI6_REMOTE_MACIP_DEL
	FRR_ZAPI6_PW_ADD
	FRR_ZAPI6_PW_DELETE
	FRR_ZAPI6_PW_SET
	FRR_ZAPI6_PW_UNSET
	FRR_ZAPI6_PW_STATUS_UPDATE
	FRR_ZAPI6_RULE_ADD
	FRR_ZAPI6_RULE_DELETE
	FRR_ZAPI6_RULE_NOTIFY_OWNER
	FRR_ZAPI6_TABLE_MANAGER_CONNECT
	FRR_ZAPI6_GET_TABLE_CHUNK
	FRR_ZAPI6_RELEASE_TABLE_CHUNK
	FRR_ZAPI6_IPSET_CREATE
	FRR_ZAPI6_IPSET_DESTROY
	FRR_ZAPI6_IPSET_ENTRY_ADD
	FRR_ZAPI6_IPSET_ENTRY_DELETE
	FRR_ZAPI6_IPSET_NOTIFY_OWNER
	FRR_ZAPI6_IPSET_ENTRY_NOTIFY_OWNER
	FRR_ZAPI6_IPTABLE_ADD
	FRR_ZAPI6_IPTABLE_DELETE
	FRR_ZAPI6_IPTABLE_NOTIFY_OWNER
)

// For FRRouting version 4 and 5. (ZAPI version 5)
const (
	FRR_ZAPI5_INTERFACE_ADD API_TYPE = iota
	FRR_ZAPI5_INTERFACE_DELETE
	FRR_ZAPI5_INTERFACE_ADDRESS_ADD
	FRR_ZAPI5_INTERFACE_ADDRESS_DELETE
	FRR_ZAPI5_INTERFACE_UP
	FRR_ZAPI5_INTERFACE_DOWN
	FRR_ZAPI5_INTERFACE_SET_MASTER
	FRR_ZAPI5_ROUTE_ADD
	FRR_ZAPI5_ROUTE_DELETE
	FRR_ZAPI5_ROUTE_NOTIFY_OWNER
	FRR_ZAPI5_IPV4_ROUTE_ADD
	FRR_ZAPI5_IPV4_ROUTE_DELETE
	FRR_ZAPI5_IPV6_ROUTE_ADD
	FRR_ZAPI5_IPV6_ROUTE_DELETE
	FRR_ZAPI5_REDISTRIBUTE_ADD
	FRR_ZAPI5_REDISTRIBUTE_DELETE
	FRR_ZAPI5_REDISTRIBUTE_DEFAULT_ADD
	FRR_ZAPI5_REDISTRIBUTE_DEFAULT_DELETE
	FRR_ZAPI5_ROUTER_ID_ADD
	FRR_ZAPI5_ROUTER_ID_DELETE
	FRR_ZAPI5_ROUTER_ID_UPDATE
	FRR_ZAPI5_HELLO
	FRR_ZAPI5_CAPABILITIES
	FRR_ZAPI5_NEXTHOP_REGISTER
	FRR_ZAPI5_NEXTHOP_UNREGISTER
	FRR_ZAPI5_NEXTHOP_UPDATE
	FRR_ZAPI5_INTERFACE_NBR_ADDRESS_ADD
	FRR_ZAPI5_INTERFACE_NBR_ADDRESS_DELETE
	FRR_ZAPI5_INTERFACE_BFD_DEST_UPDATE
	FRR_ZAPI5_IMPORT_ROUTE_REGISTER
	FRR_ZAPI5_IMPORT_ROUTE_UNREGISTER
	FRR_ZAPI5_IMPORT_CHECK_UPDATE
	FRR_ZAPI5_IPV4_ROUTE_IPV6_NEXTHOP_ADD
	FRR_ZAPI5_BFD_DEST_REGISTER
	FRR_ZAPI5_BFD_DEST_DEREGISTER
	FRR_ZAPI5_BFD_DEST_UPDATE
	FRR_ZAPI5_BFD_DEST_REPLAY
	FRR_ZAPI5_REDISTRIBUTE_ROUTE_ADD
	FRR_ZAPI5_REDISTRIBUTE_ROUTE_DEL
	FRR_ZAPI5_VRF_UNREGISTER
	FRR_ZAPI5_VRF_ADD
	FRR_ZAPI5_VRF_DELETE
	FRR_ZAPI5_VRF_LABEL
	FRR_ZAPI5_INTERFACE_VRF_UPDATE
	FRR_ZAPI5_BFD_CLIENT_REGISTER
	FRR_ZAPI5_INTERFACE_ENABLE_RADV
	FRR_ZAPI5_INTERFACE_DISABLE_RADV
	FRR_ZAPI5_IPV4_NEXTHOP_LOOKUP_MRIB
	FRR_ZAPI5_INTERFACE_LINK_PARAMS
	FRR_ZAPI5_MPLS_LABELS_ADD
	FRR_ZAPI5_MPLS_LABELS_DELETE
	FRR_ZAPI5_IPMR_ROUTE_STATS
	FRR_ZAPI5_LABEL_MANAGER_CONNECT
	FRR_ZAPI5_LABEL_MANAGER_CONNECT_ASYNC
	FRR_ZAPI5_GET_LABEL_CHUNK
	FRR_ZAPI5_RELEASE_LABEL_CHUNK
	FRR_ZAPI5_FEC_REGISTER
	FRR_ZAPI5_FEC_UNREGISTER
	FRR_ZAPI5_FEC_UPDATE
	FRR_ZAPI5_ADVERTISE_DEFAULT_GW
	FRR_ZAPI5_ADVERTISE_SUBNET
	FRR_ZAPI5_ADVERTISE_ALL_VNI
	FRR_ZAPI5_VNI_ADD
	FRR_ZAPI5_VNI_DEL
	FRR_ZAPI5_L3VNI_ADD
	FRR_ZAPI5_L3VNI_DEL
	FRR_ZAPI5_REMOTE_VTEP_ADD
	FRR_ZAPI5_REMOTE_VTEP_DEL
	FRR_ZAPI5_MACIP_ADD
	FRR_ZAPI5_MACIP_DEL
	FRR_ZAPI5_IP_PREFIX_ROUTE_ADD
	FRR_ZAPI5_IP_PREFIX_ROUTE_DEL
	FRR_ZAPI5_REMOTE_MACIP_ADD
	FRR_ZAPI5_REMOTE_MACIP_DEL
	FRR_ZAPI5_PW_ADD
	FRR_ZAPI5_PW_DELETE
	FRR_ZAPI5_PW_SET
	FRR_ZAPI5_PW_UNSET
	FRR_ZAPI5_PW_STATUS_UPDATE
	FRR_ZAPI5_RULE_ADD
	FRR_ZAPI5_RULE_DELETE
	FRR_ZAPI5_RULE_NOTIFY_OWNER
	FRR_ZAPI5_TABLE_MANAGER_CONNECT
	FRR_ZAPI5_GET_TABLE_CHUNK
	FRR_ZAPI5_RELEASE_TABLE_CHUNK
	FRR_ZAPI5_IPSET_CREATE
	FRR_ZAPI5_IPSET_DESTROY
	FRR_ZAPI5_IPSET_ENTRY_ADD
	FRR_ZAPI5_IPSET_ENTRY_DELETE
	FRR_ZAPI5_IPSET_NOTIFY_OWNER
	FRR_ZAPI5_IPSET_ENTRY_NOTIFY_OWNER
	FRR_ZAPI5_IPTABLE_ADD
	FRR_ZAPI5_IPTABLE_DELETE
	FRR_ZAPI5_IPTABLE_NOTIFY_OWNER
)

// For FRRouting.
const (
	FRR_INTERFACE_ADD API_TYPE = iota
	FRR_INTERFACE_DELETE
	FRR_INTERFACE_ADDRESS_ADD
	FRR_INTERFACE_ADDRESS_DELETE
	FRR_INTERFACE_UP
	FRR_INTERFACE_DOWN
	FRR_IPV4_ROUTE_ADD
	FRR_IPV4_ROUTE_DELETE
	FRR_IPV6_ROUTE_ADD
	FRR_IPV6_ROUTE_DELETE
	FRR_REDISTRIBUTE_ADD
	FRR_REDISTRIBUTE_DELETE
	FRR_REDISTRIBUTE_DEFAULT_ADD
	FRR_REDISTRIBUTE_DEFAULT_DELETE
	FRR_ROUTER_ID_ADD
	FRR_ROUTER_ID_DELETE
	FRR_ROUTER_ID_UPDATE
	FRR_HELLO
	FRR_NEXTHOP_REGISTER
	FRR_NEXTHOP_UNREGISTER
	FRR_NEXTHOP_UPDATE
	FRR_INTERFACE_NBR_ADDRESS_ADD
	FRR_INTERFACE_NBR_ADDRESS_DELETE
	FRR_INTERFACE_BFD_DEST_UPDATE
	FRR_IMPORT_ROUTE_REGISTER
	FRR_IMPORT_ROUTE_UNREGISTER
	FRR_IMPORT_CHECK_UPDATE
	FRR_IPV4_ROUTE_IPV6_NEXTHOP_ADD
	FRR_BFD_DEST_REGISTER
	FRR_BFD_DEST_DEREGISTER
	FRR_BFD_DEST_UPDATE
	FRR_BFD_DEST_REPLAY
	FRR_REDISTRIBUTE_IPV4_ADD
	FRR_REDISTRIBUTE_IPV4_DEL
	FRR_REDISTRIBUTE_IPV6_ADD
	FRR_REDISTRIBUTE_IPV6_DEL
	FRR_VRF_UNREGISTER
	FRR_VRF_ADD
	FRR_VRF_DELETE
	FRR_INTERFACE_VRF_UPDATE
	FRR_BFD_CLIENT_REGISTER
	FRR_INTERFACE_ENABLE_RADV
	FRR_INTERFACE_DISABLE_RADV
	FRR_IPV4_NEXTHOP_LOOKUP_MRIB
	FRR_INTERFACE_LINK_PARAMS
	FRR_MPLS_LABELS_ADD
	FRR_MPLS_LABELS_DELETE
	FRR_IPV4_NEXTHOP_ADD
	FRR_IPV4_NEXTHOP_DELETE
	FRR_IPV6_NEXTHOP_ADD
	FRR_IPV6_NEXTHOP_DELETE
	FRR_IPMR_ROUTE_STATS
	FRR_LABEL_MANAGER_CONNECT
	FRR_GET_LABEL_CHUNK
	FRR_RELEASE_LABEL_CHUNK
	FRR_PW_ADD
	FRR_PW_DELETE
	FRR_PW_SET
	FRR_PW_UNSET
	FRR_PW_STATUS_UPDATE
)

// For Quagga.
const (
	_ API_TYPE = iota
	INTERFACE_ADD
	INTERFACE_DELETE
	INTERFACE_ADDRESS_ADD
	INTERFACE_ADDRESS_DELETE
	INTERFACE_UP
	INTERFACE_DOWN
	IPV4_ROUTE_ADD
	IPV4_ROUTE_DELETE
	IPV6_ROUTE_ADD
	IPV6_ROUTE_DELETE
	REDISTRIBUTE_ADD
	REDISTRIBUTE_DELETE
	REDISTRIBUTE_DEFAULT_ADD
	REDISTRIBUTE_DEFAULT_DELETE
	IPV4_NEXTHOP_LOOKUP
	IPV6_NEXTHOP_LOOKUP
	IPV4_IMPORT_LOOKUP
	IPV6_IMPORT_LOOKUP
	INTERFACE_RENAME
	ROUTER_ID_ADD
	ROUTER_ID_DELETE
	ROUTER_ID_UPDATE
	HELLO
	IPV4_NEXTHOP_LOOKUP_MRIB
	VRF_UNREGISTER
	INTERFACE_LINK_PARAMS
	NEXTHOP_REGISTER
	NEXTHOP_UNREGISTER
	NEXTHOP_UPDATE
	MESSAGE_MAX
)

// Route Types.
//go:generate stringer -type=ROUTE_TYPE
type ROUTE_TYPE uint8

// For FRRouting version 6 (ZAPI version 6).
const (
	FRR_ZAPI6_ROUTE_SYSTEM ROUTE_TYPE = iota
	FRR_ZAPI6_ROUTE_KERNEL
	FRR_ZAPI6_ROUTE_CONNECT
	FRR_ZAPI6_ROUTE_STATIC
	FRR_ZAPI6_ROUTE_RIP
	FRR_ZAPI6_ROUTE_RIPNG
	FRR_ZAPI6_ROUTE_OSPF
	FRR_ZAPI6_ROUTE_OSPF6
	FRR_ZAPI6_ROUTE_ISIS
	FRR_ZAPI6_ROUTE_BGP
	FRR_ZAPI6_ROUTE_PIM
	FRR_ZAPI6_ROUTE_EIGRP
	FRR_ZAPI6_ROUTE_NHRP
	FRR_ZAPI6_ROUTE_HSLS
	FRR_ZAPI6_ROUTE_OLSR
	FRR_ZAPI6_ROUTE_TABLE
	FRR_ZAPI6_ROUTE_LDP
	FRR_ZAPI6_ROUTE_VNC
	FRR_ZAPI6_ROUTE_VNC_DIRECT
	FRR_ZAPI6_ROUTE_VNC_DIRECT_RH
	FRR_ZAPI6_ROUTE_BGP_DIRECT
	FRR_ZAPI6_ROUTE_BGP_DIRECT_EXT
	FRR_ZAPI6_ROUTE_BABEL
	FRR_ZAPI6_ROUTE_SHARP
	FRR_ZAPI6_ROUTE_PBR
	FRR_ZAPI6_ROUTE_BFD
	FRR_ZAPI6_ROUTE_ALL
	FRR_ZAPI6_ROUTE_MAX
)

// For FRRouting version 4 and 5 (ZAPI version 5).
const (
	FRR_ZAPI5_ROUTE_SYSTEM ROUTE_TYPE = iota
	FRR_ZAPI5_ROUTE_KERNEL
	FRR_ZAPI5_ROUTE_CONNECT
	FRR_ZAPI5_ROUTE_STATIC
	FRR_ZAPI5_ROUTE_RIP
	FRR_ZAPI5_ROUTE_RIPNG
	FRR_ZAPI5_ROUTE_OSPF
	FRR_ZAPI5_ROUTE_OSPF6
	FRR_ZAPI5_ROUTE_ISIS
	FRR_ZAPI5_ROUTE_BGP
	FRR_ZAPI5_ROUTE_PIM
	FRR_ZAPI5_ROUTE_EIGRP
	FRR_ZAPI5_ROUTE_NHRP
	FRR_ZAPI5_ROUTE_HSLS
	FRR_ZAPI5_ROUTE_OLSR
	FRR_ZAPI5_ROUTE_TABLE
	FRR_ZAPI5_ROUTE_LDP
	FRR_ZAPI5_ROUTE_VNC
	FRR_ZAPI5_ROUTE_VNC_DIRECT
	FRR_ZAPI5_ROUTE_VNC_DIRECT_RH
	FRR_ZAPI5_ROUTE_BGP_DIRECT
	FRR_ZAPI5_ROUTE_BGP_DIRECT_EXT
	FRR_ZAPI5_ROUTE_BABEL
	FRR_ZAPI5_ROUTE_SHARP
	FRR_ZAPI5_ROUTE_PBR
	FRR_ZAPI5_ROUTE_ALL
	FRR_ZAPI5_ROUTE_MAX
)

// For FRRouting.
const (
	FRR_ROUTE_SYSTEM ROUTE_TYPE = iota
	FRR_ROUTE_KERNEL
	FRR_ROUTE_CONNECT
	FRR_ROUTE_STATIC
	FRR_ROUTE_RIP
	FRR_ROUTE_RIPNG
	FRR_ROUTE_OSPF
	FRR_ROUTE_OSPF6
	FRR_ROUTE_ISIS
	FRR_ROUTE_BGP
	FRR_ROUTE_PIM
	FRR_ROUTE_HSLS
	FRR_ROUTE_OLSR
	FRR_ROUTE_TABLE
	FRR_ROUTE_LDP
	FRR_ROUTE_VNC
	FRR_ROUTE_VNC_DIRECT
	FRR_ROUTE_VNC_DIRECT_RH
	FRR_ROUTE_BGP_DIRECT
	FRR_ROUTE_BGP_DIRECT_EXT
	FRR_ROUTE_ALL
	FRR_ROUTE_MAX
)

// For Quagga.
const (
	ROUTE_SYSTEM ROUTE_TYPE = iota
	ROUTE_KERNEL
	ROUTE_CONNECT
	ROUTE_STATIC
	ROUTE_RIP
	ROUTE_RIPNG
	ROUTE_OSPF
	ROUTE_OSPF6
	ROUTE_ISIS
	ROUTE_BGP
	ROUTE_PIM
	ROUTE_HSLS
	ROUTE_OLSR
	ROUTE_BABEL
	ROUTE_MAX
)

var routeTypeValueMapFrrZapi6 = map[string]ROUTE_TYPE{
	"system":             FRR_ZAPI6_ROUTE_SYSTEM,
	"kernel":             FRR_ZAPI6_ROUTE_KERNEL,
	"connect":            FRR_ZAPI6_ROUTE_CONNECT, // hack for backward compatibility
	"directly-connected": FRR_ZAPI6_ROUTE_CONNECT,
	"static":             FRR_ZAPI6_ROUTE_STATIC,
	"rip":                FRR_ZAPI6_ROUTE_RIP,
	"ripng":              FRR_ZAPI6_ROUTE_RIPNG,
	"ospf":               FRR_ZAPI6_ROUTE_OSPF,
	"ospf3":              FRR_ZAPI6_ROUTE_OSPF6,
	"isis":               FRR_ZAPI6_ROUTE_ISIS,
	"bgp":                FRR_ZAPI6_ROUTE_BGP,
	"pim":                FRR_ZAPI6_ROUTE_PIM,
	"eigrp":              FRR_ZAPI6_ROUTE_EIGRP,
	"nhrp":               FRR_ZAPI6_ROUTE_EIGRP,
	"hsls":               FRR_ZAPI6_ROUTE_HSLS,
	"olsr":               FRR_ZAPI6_ROUTE_OLSR,
	"table":              FRR_ZAPI6_ROUTE_TABLE,
	"ldp":                FRR_ZAPI6_ROUTE_LDP,
	"vnc":                FRR_ZAPI6_ROUTE_VNC,
	"vnc-direct":         FRR_ZAPI6_ROUTE_VNC_DIRECT,
	"vnc-direct-rh":      FRR_ZAPI6_ROUTE_VNC_DIRECT_RH,
	"bgp-direct":         FRR_ZAPI6_ROUTE_BGP_DIRECT,
	"bgp-direct-ext":     FRR_ZAPI6_ROUTE_BGP_DIRECT_EXT,
	"babel":              FRR_ZAPI6_ROUTE_BABEL,
	"sharp":              FRR_ZAPI6_ROUTE_SHARP,
	"pbr":                FRR_ZAPI6_ROUTE_PBR,
	"bfd":                FRR_ZAPI6_ROUTE_BFD,
	"all":                FRR_ZAPI6_ROUTE_ALL,
}

var routeTypeValueMapFrrZapi5 = map[string]ROUTE_TYPE{
	"system":             FRR_ZAPI5_ROUTE_SYSTEM,
	"kernel":             FRR_ZAPI5_ROUTE_KERNEL,
	"connect":            FRR_ZAPI5_ROUTE_CONNECT, // hack for backward compatibility
	"directly-connected": FRR_ZAPI5_ROUTE_CONNECT,
	"static":             FRR_ZAPI5_ROUTE_STATIC,
	"rip":                FRR_ZAPI5_ROUTE_RIP,
	"ripng":              FRR_ZAPI5_ROUTE_RIPNG,
	"ospf":               FRR_ZAPI5_ROUTE_OSPF,
	"ospf3":              FRR_ZAPI5_ROUTE_OSPF6,
	"isis":               FRR_ZAPI5_ROUTE_ISIS,
	"bgp":                FRR_ZAPI5_ROUTE_BGP,
	"pim":                FRR_ZAPI5_ROUTE_PIM,
	"eigrp":              FRR_ZAPI5_ROUTE_EIGRP,
	"nhrp":               FRR_ZAPI5_ROUTE_EIGRP,
	"hsls":               FRR_ZAPI5_ROUTE_HSLS,
	"olsr":               FRR_ZAPI5_ROUTE_OLSR,
	"table":              FRR_ZAPI5_ROUTE_TABLE,
	"ldp":                FRR_ZAPI5_ROUTE_LDP,
	"vnc":                FRR_ZAPI5_ROUTE_VNC,
	"vnc-direct":         FRR_ZAPI5_ROUTE_VNC_DIRECT,
	"vnc-direct-rh":      FRR_ZAPI5_ROUTE_VNC_DIRECT_RH,
	"bgp-direct":         FRR_ZAPI5_ROUTE_BGP_DIRECT,
	"bgp-direct-ext":     FRR_ZAPI5_ROUTE_BGP_DIRECT_EXT,
	"babel":              FRR_ZAPI5_ROUTE_BABEL,
	"sharp":              FRR_ZAPI5_ROUTE_SHARP,
	"pbr":                FRR_ZAPI5_ROUTE_PBR,
	"all":                FRR_ZAPI5_ROUTE_ALL,
}

var routeTypeValueMapFrr = map[string]ROUTE_TYPE{
	"system":             FRR_ROUTE_SYSTEM,
	"kernel":             FRR_ROUTE_KERNEL,
	"connect":            FRR_ROUTE_CONNECT, // hack for backward compatibility
	"directly-connected": FRR_ROUTE_CONNECT,
	"static":             FRR_ROUTE_STATIC,
	"rip":                FRR_ROUTE_RIP,
	"ripng":              FRR_ROUTE_RIPNG,
	"ospf":               FRR_ROUTE_OSPF,
	"ospf3":              FRR_ROUTE_OSPF6,
	"isis":               FRR_ROUTE_ISIS,
	"bgp":                FRR_ROUTE_BGP,
	"pim":                FRR_ROUTE_PIM,
	"hsls":               FRR_ROUTE_HSLS,
	"olsr":               FRR_ROUTE_OLSR,
	"table":              FRR_ROUTE_TABLE,
	"ldp":                FRR_ROUTE_LDP,
	"vnc":                FRR_ROUTE_VNC,
	"vnc-direct":         FRR_ROUTE_VNC_DIRECT,
	"vnc-direct-rh":      FRR_ROUTE_VNC_DIRECT_RH,
	"bgp-direct":         FRR_ROUTE_BGP_DIRECT,
	"bgp-direct-ext":     FRR_ROUTE_BGP_DIRECT_EXT,
	"all":                FRR_ROUTE_ALL,
}

var routeTypeValueMap = map[string]ROUTE_TYPE{
	"system":             ROUTE_SYSTEM,
	"kernel":             ROUTE_KERNEL,
	"connect":            ROUTE_CONNECT, // hack for backward compatibility
	"directly-connected": ROUTE_CONNECT,
	"static":             ROUTE_STATIC,
	"rip":                ROUTE_RIP,
	"ripng":              ROUTE_RIPNG,
	"ospf":               ROUTE_OSPF,
	"ospf3":              ROUTE_OSPF6,
	"isis":               ROUTE_ISIS,
	"bgp":                ROUTE_BGP,
	"pim":                ROUTE_PIM,
	"hsls":               ROUTE_HSLS,
	"olsr":               ROUTE_OLSR,
	"babel":              ROUTE_BABEL,
}

func RouteTypeFromString(typ string, version uint8) (ROUTE_TYPE, error) {
	delegateRouteTypeValueMap := routeTypeValueMap
	if version == 4 {
		delegateRouteTypeValueMap = routeTypeValueMapFrr
	} else if version == 5 {
		delegateRouteTypeValueMap = routeTypeValueMapFrrZapi5
	} else if version >= 6 {
		delegateRouteTypeValueMap = routeTypeValueMapFrrZapi6
	}
	t, ok := delegateRouteTypeValueMap[typ]
	if ok {
		return t, nil
	}
	return t, fmt.Errorf("unknown route type: %s", typ)
}

func addressFamilyFromApi(Api API_TYPE, version uint8) uint8 {
	if version <= 3 {
		switch Api {
		case IPV4_ROUTE_ADD, IPV4_ROUTE_DELETE, IPV4_NEXTHOP_LOOKUP, IPV4_IMPORT_LOOKUP:
			return syscall.AF_INET
		case IPV6_ROUTE_ADD, IPV6_ROUTE_DELETE, IPV6_NEXTHOP_LOOKUP, IPV6_IMPORT_LOOKUP:
			return syscall.AF_INET6
		}
	} else if version == 4 {
		switch Api {
		case FRR_REDISTRIBUTE_IPV4_ADD, FRR_REDISTRIBUTE_IPV4_DEL, FRR_IPV4_ROUTE_ADD, FRR_IPV4_ROUTE_DELETE, FRR_IPV4_NEXTHOP_LOOKUP_MRIB:
			return syscall.AF_INET
		case FRR_REDISTRIBUTE_IPV6_ADD, FRR_REDISTRIBUTE_IPV6_DEL, FRR_IPV6_ROUTE_ADD, FRR_IPV6_ROUTE_DELETE:
			return syscall.AF_INET6
		}
	} else if version == 5 {
		switch Api {
		case FRR_ZAPI5_IPV4_ROUTE_ADD, FRR_ZAPI5_IPV4_ROUTE_DELETE, FRR_ZAPI5_IPV4_NEXTHOP_LOOKUP_MRIB:
			return syscall.AF_INET
		case FRR_ZAPI5_IPV6_ROUTE_ADD, FRR_ZAPI5_IPV6_ROUTE_DELETE:
			return syscall.AF_INET6
		}
	}
	return syscall.AF_UNSPEC
}

func addressByteLength(family uint8) (int, error) {
	switch family {
	case syscall.AF_INET:
		return net.IPv4len, nil
	case syscall.AF_INET6:
		return net.IPv6len, nil
	}
	return 0, fmt.Errorf("unknown address family: %d", family)
}

func ipFromFamily(family uint8, buf []byte) net.IP {
	switch family {
	case syscall.AF_INET:
		return net.IP(buf).To4()
	case syscall.AF_INET6:
		return net.IP(buf).To16()
	}
	return nil
}

// API Message Flags.
type MESSAGE_FLAG uint8

// For FRRouting version 4, 5 and 6 (ZAPI version 5 and 6).
const (
	FRR_ZAPI5_MESSAGE_NEXTHOP  MESSAGE_FLAG = 0x01
	FRR_ZAPI5_MESSAGE_DISTANCE MESSAGE_FLAG = 0x02
	FRR_ZAPI5_MESSAGE_METRIC   MESSAGE_FLAG = 0x04
	FRR_ZAPI5_MESSAGE_TAG      MESSAGE_FLAG = 0x08
	FRR_ZAPI5_MESSAGE_MTU      MESSAGE_FLAG = 0x10
	FRR_ZAPI5_MESSAGE_SRCPFX   MESSAGE_FLAG = 0x20
	FRR_ZAPI5_MESSAGE_LABEL    MESSAGE_FLAG = 0x40
	FRR_ZAPI5_MESSAGE_TABLEID  MESSAGE_FLAG = 0x80
)

// For FRRouting.
const (
	FRR_MESSAGE_NEXTHOP  MESSAGE_FLAG = 0x01
	FRR_MESSAGE_IFINDEX  MESSAGE_FLAG = 0x02
	FRR_MESSAGE_DISTANCE MESSAGE_FLAG = 0x04
	FRR_MESSAGE_METRIC   MESSAGE_FLAG = 0x08
	FRR_MESSAGE_TAG      MESSAGE_FLAG = 0x10
	FRR_MESSAGE_MTU      MESSAGE_FLAG = 0x20
	FRR_MESSAGE_SRCPFX   MESSAGE_FLAG = 0x40
)

// For Quagga.
const (
	MESSAGE_NEXTHOP  MESSAGE_FLAG = 0x01
	MESSAGE_IFINDEX  MESSAGE_FLAG = 0x02
	MESSAGE_DISTANCE MESSAGE_FLAG = 0x04
	MESSAGE_METRIC   MESSAGE_FLAG = 0x08
	MESSAGE_MTU      MESSAGE_FLAG = 0x10
	MESSAGE_TAG      MESSAGE_FLAG = 0x20
)

func (t MESSAGE_FLAG) String(version uint8) string {
	var ss []string
	if (version <= 3 && t&MESSAGE_NEXTHOP > 0) ||
		(version == 4 && t&FRR_MESSAGE_NEXTHOP > 0) ||
		(version >= 5 && t&FRR_ZAPI5_MESSAGE_NEXTHOP > 0) {
		ss = append(ss, "NEXTHOP")
	}
	if (version <= 3 && t&MESSAGE_IFINDEX > 0) || (version == 4 && t&FRR_MESSAGE_IFINDEX > 0) {
		ss = append(ss, "IFINDEX")
	}
	if (version <= 3 && t&MESSAGE_DISTANCE > 0) ||
		(version == 4 && t&FRR_MESSAGE_DISTANCE > 0) ||
		(version >= 5 && t&FRR_ZAPI5_MESSAGE_DISTANCE > 0) {
		ss = append(ss, "DISTANCE")
	}
	if (version <= 3 && t&MESSAGE_METRIC > 0) ||
		(version == 4 && t&FRR_MESSAGE_METRIC > 0) ||
		(version >= 5 && t&FRR_ZAPI5_MESSAGE_METRIC > 0) {
		ss = append(ss, "METRIC")
	}
	if (version <= 3 && t&MESSAGE_MTU > 0) || (version == 4 && t&FRR_MESSAGE_MTU > 0) ||
		(version >= 5 && t&FRR_ZAPI5_MESSAGE_MTU > 0) {
		ss = append(ss, "MTU")
	}
	if (version <= 3 && t&MESSAGE_TAG > 0) || (version == 4 && t&FRR_MESSAGE_TAG > 0) ||
		(version >= 5 && t&FRR_ZAPI5_MESSAGE_TAG > 0) {
		ss = append(ss, "TAG")
	}
	if (version == 4 && t&FRR_MESSAGE_SRCPFX > 0) ||
		(version >= 5 && t&FRR_ZAPI5_MESSAGE_SRCPFX > 0) {
		ss = append(ss, "SRCPFX")
	}
	if version >= 5 && t&FRR_ZAPI5_MESSAGE_LABEL > 0 {
		ss = append(ss, "LABLE")
	}

	return strings.Join(ss, "|")
}

// Message Flags
type FLAG uint64

const (
	FLAG_INTERNAL        FLAG = 0x01
	FLAG_SELFROUTE       FLAG = 0x02
	FLAG_BLACKHOLE       FLAG = 0x04
	FLAG_IBGP            FLAG = 0x08
	FLAG_SELECTED        FLAG = 0x10
	FLAG_CHANGED         FLAG = 0x20
	FLAG_STATIC          FLAG = 0x40
	FLAG_REJECT          FLAG = 0x80
	FLAG_SCOPE_LINK      FLAG = 0x100
	FLAG_FIB_OVERRIDE    FLAG = 0x200
	FLAG_EVPN_ROUTE      FLAG = 0x400
	FLAG_RR_USE_DISTANCE FLAG = 0x800
)

func (t FLAG) String() string {
	var ss []string
	if t&FLAG_INTERNAL > 0 {
		ss = append(ss, "FLAG_INTERNAL")
	}
	if t&FLAG_SELFROUTE > 0 {
		ss = append(ss, "FLAG_SELFROUTE")
	}
	if t&FLAG_BLACKHOLE > 0 {
		ss = append(ss, "FLAG_BLACKHOLE")
	}
	if t&FLAG_IBGP > 0 {
		ss = append(ss, "FLAG_IBGP")
	}
	if t&FLAG_SELECTED > 0 {
		ss = append(ss, "FLAG_SELECTED")
	}
	if t&FLAG_CHANGED > 0 {
		ss = append(ss, "FLAG_CHANGED")
	}
	if t&FLAG_STATIC > 0 {
		ss = append(ss, "FLAG_STATIC")
	}
	if t&FLAG_REJECT > 0 {
		ss = append(ss, "FLAG_REJECT")
	}
	if t&FLAG_SCOPE_LINK > 0 {
		ss = append(ss, "FLAG_SCOPE_LINK")
	}
	if t&FLAG_FIB_OVERRIDE > 0 {
		ss = append(ss, "FLAG_FIB_OVERRIDE")
	}
	if t&FLAG_EVPN_ROUTE > 0 {
		ss = append(ss, "FLAG_EVPN_ROUTE")
	}
	if t&FLAG_RR_USE_DISTANCE > 0 {
		ss = append(ss, "FLAG_RR_USE_DISTANCE")
	}

	return strings.Join(ss, "|")
}

// Nexthop Types.
//go:generate stringer -type=NEXTHOP_TYPE
type NEXTHOP_TYPE uint8

// For FRRouting.
const (
	_ NEXTHOP_TYPE = iota
	FRR_NEXTHOP_TYPE_IFINDEX
	FRR_NEXTHOP_TYPE_IPV4
	FRR_NEXTHOP_TYPE_IPV4_IFINDEX
	FRR_NEXTHOP_TYPE_IPV6
	FRR_NEXTHOP_TYPE_IPV6_IFINDEX
	FRR_NEXTHOP_TYPE_BLACKHOLE
)

// For Quagga.
const (
	_ NEXTHOP_TYPE = iota
	NEXTHOP_TYPE_IFINDEX
	NEXTHOP_TYPE_IFNAME
	NEXTHOP_TYPE_IPV4
	NEXTHOP_TYPE_IPV4_IFINDEX
	NEXTHOP_TYPE_IPV4_IFNAME
	NEXTHOP_TYPE_IPV6
	NEXTHOP_TYPE_IPV6_IFINDEX
	NEXTHOP_TYPE_IPV6_IFNAME
	NEXTHOP_TYPE_BLACKHOLE
)

// Nexthop Flags.
//go:generate stringer -type=NEXTHOP_FLAG
type NEXTHOP_FLAG uint8

const (
	NEXTHOP_FLAG_ACTIVE     NEXTHOP_FLAG = 0x01 // This nexthop is alive.
	NEXTHOP_FLAG_FIB        NEXTHOP_FLAG = 0x02 // FIB nexthop.
	NEXTHOP_FLAG_RECURSIVE  NEXTHOP_FLAG = 0x04 // Recursive nexthop.
	NEXTHOP_FLAG_ONLINK     NEXTHOP_FLAG = 0x08 // Nexthop should be installed onlink.
	NEXTHOP_FLAG_MATCHED    NEXTHOP_FLAG = 0x10 // Already matched vs a nexthop
	NEXTHOP_FLAG_FILTERED   NEXTHOP_FLAG = 0x20 // rmap filtered (version >= 4)
	NEXTHOP_FLAG_DUPLICATE  NEXTHOP_FLAG = 0x40 // nexthop duplicates (version >= 5)
	NEXTHOP_FLAG_EVPN_RVTEP NEXTHOP_FLAG = 0x80 // EVPN remote vtep nexthop (version >= 5)
)

// Interface PTM Enable Configuration.
//go:generate stringer -type=PTM_ENABLE
type PTM_ENABLE uint8

const (
	PTM_ENABLE_OFF    PTM_ENABLE = 0
	PTM_ENABLE_ON     PTM_ENABLE = 1
	PTM_ENABLE_UNSPEC PTM_ENABLE = 2
)

// PTM Status.
//go:generate stringer -type=PTM_STATUS
type PTM_STATUS uint8

const (
	PTM_STATUS_DOWN    PTM_STATUS = 0
	PTM_STATUS_UP      PTM_STATUS = 1
	PTM_STATUS_UNKNOWN PTM_STATUS = 2
)

type Client struct {
	outgoing      chan *Message
	incoming      chan *Message
	redistDefault ROUTE_TYPE
	conn          net.Conn
	Version       uint8
}

func NewClient(network, address string, typ ROUTE_TYPE, version uint8) (*Client, error) {
	conn, err := net.Dial(network, address)
	if err != nil {
		return nil, err
	}
	outgoing := make(chan *Message)
	incoming := make(chan *Message, 64)
	if version < 2 {
		version = 2
	} else if version > 6 {
		version = 6
	}

	c := &Client{
		outgoing:      outgoing,
		incoming:      incoming,
		redistDefault: typ,
		conn:          conn,
		Version:       version,
	}

	go func() {
		for {
			m, more := <-outgoing
			if more {
				b, err := m.Serialize()
				if err != nil {
					log.WithFields(log.Fields{
						"Topic": "Zebra",
					}).Warnf("failed to serialize: %v", m)
					continue
				}

				_, err = conn.Write(b)
				if err != nil {
					log.WithFields(log.Fields{
						"Topic": "Zebra",
					}).Errorf("failed to write: %s", err)
					close(outgoing)
				}
			} else {
				log.Debug("finish outgoing loop")
				return
			}
		}
	}()

	// Send HELLO/ROUTER_ID_ADD messages to negotiate the Zebra message version.
	c.SendHello()
	c.SendRouterIDAdd()

	receiveSingleMsg := func() (*Message, error) {
		headerBuf, err := readAll(conn, int(HeaderSize(version)))
		if err != nil {
			log.WithFields(log.Fields{
				"Topic": "Zebra",
				"Error": err,
			}).Error("failed to read header")
			return nil, err
		}

		hd := &Header{}
		err = hd.DecodeFromBytes(headerBuf)
		if err != nil {
			log.WithFields(log.Fields{
				"Topic": "Zebra",
				"Data":  headerBuf,
				"Error": err,
			}).Error("failed to decode header")
			return nil, err
		}

		bodyBuf, err := readAll(conn, int(hd.Len-HeaderSize(version)))
		if err != nil {
			log.WithFields(log.Fields{
				"Topic":  "Zebra",
				"Header": hd,
				"Error":  err,
			}).Error("failed to read body")
			return nil, err
		}

		m, err := ParseMessage(hd, bodyBuf)
		if err != nil {
			// Just outputting warnings (not error message) and ignore this
			// error considering the case that body parser is not implemented
			// yet.
			log.WithFields(log.Fields{
				"Topic":  "Zebra",
				"Header": hd,
				"Data":   bodyBuf,
				"Error":  err,
			}).Warn("failed to decode body")
			return nil, nil
		}
		log.WithFields(log.Fields{
			"Topic":   "Zebra",
			"Message": m,
		}).Debug("read message from zebra")

		return m, nil
	}

	// Try to receive the first message from Zebra.
	if m, err := receiveSingleMsg(); err != nil {
		c.Close()
		// Return error explicitly in order to retry connection.
		return nil, err
	} else if m != nil {
		incoming <- m
	}

	// Start receive loop only when the first message successfully received.
	go func() {
		defer close(incoming)
		for {
			if m, err := receiveSingleMsg(); err != nil {
				return
			} else if m != nil {
				incoming <- m
			}
		}
	}()

	return c, nil
}

func readAll(conn net.Conn, length int) ([]byte, error) {
	buf := make([]byte, length)
	_, err := io.ReadFull(conn, buf)
	return buf, err
}

func (c *Client) Receive() chan *Message {
	return c.incoming
}

func (c *Client) Send(m *Message) {
	defer func() {
		if err := recover(); err != nil {
			log.WithFields(log.Fields{
				"Topic": "Zebra",
			}).Debugf("recovered: %s", err)
		}
	}()
	log.WithFields(log.Fields{
		"Topic":  "Zebra",
		"Header": m.Header,
		"Body":   m.Body,
	}).Debug("send command to zebra")
	c.outgoing <- m
}

func (c *Client) SendCommand(command API_TYPE, vrfId uint32, body Body) error {
	var marker uint8 = HEADER_MARKER
	if c.Version >= 4 {
		marker = FRR_HEADER_MARKER
	}
	m := &Message{
		Header: Header{
			Len:     HeaderSize(c.Version),
			Marker:  marker,
			Version: c.Version,
			VrfId:   vrfId,
			Command: command,
		},
		Body: body,
	}
	c.Send(m)
	return nil
}

func (c *Client) SendHello() error {
	if c.redistDefault > 0 {
		command := HELLO
		if c.Version == 4 {
			command = FRR_HELLO
		} else if c.Version == 5 {
			command = FRR_ZAPI5_HELLO
		} else if c.Version >= 6 {
			command = FRR_ZAPI6_HELLO
		}
		body := &HelloBody{
			RedistDefault: c.redistDefault,
			Instance:      0,
		}
		return c.SendCommand(command, VRF_DEFAULT, body)
	}
	return nil
}

func (c *Client) SendRouterIDAdd() error {
	command := ROUTER_ID_ADD
	if c.Version == 4 {
		command = FRR_ROUTER_ID_ADD
	} else if c.Version == 5 {
		command = FRR_ZAPI5_ROUTER_ID_ADD
	} else if c.Version >= 6 {
		command = FRR_ZAPI6_ROUTER_ID_ADD
	}
	return c.SendCommand(command, VRF_DEFAULT, nil)
}

func (c *Client) SendInterfaceAdd() error {
	command := INTERFACE_ADD
	if c.Version == 4 {
		command = FRR_INTERFACE_ADD
	} else if c.Version >= 5 {
		command = FRR_ZAPI5_INTERFACE_ADD
	}
	return c.SendCommand(command, VRF_DEFAULT, nil)
}

func (c *Client) SendRedistribute(t ROUTE_TYPE, vrfId uint32) error {
	command := REDISTRIBUTE_ADD
	if c.redistDefault != t {
		bodies := make([]*RedistributeBody, 0)
		if c.Version <= 3 {
			bodies = append(bodies, &RedistributeBody{
				Redist: t,
			})
		} else { // version >= 4
			command = FRR_REDISTRIBUTE_ADD
			if c.Version == 5 {
				command = FRR_ZAPI5_REDISTRIBUTE_ADD
			} else if c.Version >= 6 {
				command = FRR_ZAPI6_REDISTRIBUTE_ADD
			}
			for _, afi := range []AFI{AFI_IP, AFI_IP6} {
				bodies = append(bodies, &RedistributeBody{
					Afi:      afi,
					Redist:   t,
					Instance: 0,
				})
			}
		}

		for _, body := range bodies {
			return c.SendCommand(command, vrfId, body)
		}
	}

	return nil
}

func (c *Client) SendRedistributeDelete(t ROUTE_TYPE) error {
	if t < ROUTE_MAX {
		command := REDISTRIBUTE_DELETE
		if c.Version == 4 {
			command = FRR_REDISTRIBUTE_DELETE
		} else if c.Version == 5 {
			command = FRR_ZAPI5_REDISTRIBUTE_DELETE
		} else if c.Version >= 6 {
			command = FRR_ZAPI6_REDISTRIBUTE_DELETE
		}
		body := &RedistributeBody{
			Redist: t,
		}
		return c.SendCommand(command, VRF_DEFAULT, body)
	} else {
		return fmt.Errorf("unknown route type: %d", t)
	}
}

func (c *Client) SendIPRoute(vrfId uint32, body *IPRouteBody, isWithdraw bool) error {
	command := IPV4_ROUTE_ADD
	if c.Version <= 3 {
		if body.Prefix.Prefix.To4() != nil {
			if isWithdraw {
				command = IPV4_ROUTE_DELETE
			}
		} else {
			if isWithdraw {
				command = IPV6_ROUTE_DELETE
			} else {
				command = IPV6_ROUTE_ADD
			}
		}
	} else if c.Version == 4 { // version >= 4
		if body.Prefix.Prefix.To4() != nil {
			if isWithdraw {
				command = FRR_IPV4_ROUTE_DELETE
			} else {
				command = FRR_IPV4_ROUTE_ADD
			}
		} else {
			if isWithdraw {
				command = FRR_IPV6_ROUTE_DELETE
			} else {
				command = FRR_IPV6_ROUTE_ADD
			}
		}
	} else { // version >= 5 (version 6 uses the same value as version 5)
		if isWithdraw {
			command = FRR_ZAPI5_ROUTE_DELETE
		} else {
			command = FRR_ZAPI5_ROUTE_ADD
		}
	}
	return c.SendCommand(command, vrfId, body)
}

func (c *Client) SendNexthopRegister(vrfId uint32, body *NexthopRegisterBody, isWithdraw bool) error {
	// Note: NEXTHOP_REGISTER and NEXTHOP_UNREGISTER messages are not
	// supported in Zebra protocol version<3.
	if c.Version < 3 {
		return fmt.Errorf("NEXTHOP_REGISTER/NEXTHOP_UNREGISTER are not supported in version: %d", c.Version)
	}
	command := NEXTHOP_REGISTER
	if c.Version == 3 {
		if isWithdraw {
			command = NEXTHOP_UNREGISTER
		}
	} else if c.Version == 4 { // version == 4
		if isWithdraw {
			command = FRR_NEXTHOP_UNREGISTER
		} else {
			command = FRR_NEXTHOP_REGISTER
		}
	} else if c.Version == 5 { // version == 5
		if isWithdraw {
			command = FRR_ZAPI5_NEXTHOP_UNREGISTER
		} else {
			command = FRR_ZAPI5_NEXTHOP_REGISTER
		}
	} else { // version >= 6
		if isWithdraw {
			command = FRR_ZAPI6_NEXTHOP_UNREGISTER
		} else {
			command = FRR_ZAPI6_NEXTHOP_REGISTER
		}
	}
	return c.SendCommand(command, vrfId, body)
}

func (c *Client) Close() error {
	close(c.outgoing)
	return c.conn.Close()
}

type Header struct {
	Len     uint16
	Marker  uint8
	Version uint8
	VrfId   uint32 // ZAPI v4: 16bits, v5: 32bits
	Command API_TYPE
}

func (h *Header) Serialize() ([]byte, error) {
	buf := make([]byte, HeaderSize(h.Version))
	binary.BigEndian.PutUint16(buf[0:2], h.Len)
	buf[2] = h.Marker
	buf[3] = h.Version
	switch h.Version {
	case 2:
		binary.BigEndian.PutUint16(buf[4:6], uint16(h.Command))
	case 3, 4:
		binary.BigEndian.PutUint16(buf[4:6], uint16(h.VrfId))
		binary.BigEndian.PutUint16(buf[6:8], uint16(h.Command))
	case 5, 6:
		binary.BigEndian.PutUint32(buf[4:8], uint32(h.VrfId))
		binary.BigEndian.PutUint16(buf[8:10], uint16(h.Command))
	default:
		return nil, fmt.Errorf("Unsupported ZAPI version: %d", h.Version)
	}
	return buf, nil
}

func (h *Header) DecodeFromBytes(data []byte) error {
	if uint16(len(data)) < 4 {
		return fmt.Errorf("Not all ZAPI message header")
	}
	h.Len = binary.BigEndian.Uint16(data[0:2])
	h.Marker = data[2]
	h.Version = data[3]
	if uint16(len(data)) < HeaderSize(h.Version) {
		return fmt.Errorf("Not all ZAPI message header")
	}
	switch h.Version {
	case 2:
		h.Command = API_TYPE(binary.BigEndian.Uint16(data[4:6]))
	case 3, 4:
		h.VrfId = uint32(binary.BigEndian.Uint16(data[4:6]))
		h.Command = API_TYPE(binary.BigEndian.Uint16(data[6:8]))
	case 5, 6:
		h.VrfId = binary.BigEndian.Uint32(data[4:8])
		h.Command = API_TYPE(binary.BigEndian.Uint16(data[8:10]))
	default:
		return fmt.Errorf("Unsupported ZAPI version: %d", h.Version)
	}
	return nil
}

type Body interface {
	DecodeFromBytes([]byte, uint8) error
	Serialize(uint8) ([]byte, error)
	String() string
}

type UnknownBody struct {
	Data []byte
}

func (b *UnknownBody) DecodeFromBytes(data []byte, version uint8) error {
	b.Data = data
	return nil
}

func (b *UnknownBody) Serialize(version uint8) ([]byte, error) {
	return b.Data, nil
}

func (b *UnknownBody) String() string {
	return fmt.Sprintf("data: %v", b.Data)
}

type HelloBody struct {
	RedistDefault ROUTE_TYPE
	Instance      uint16
	ReceiveNotify uint8
}

// Reference: zread_hello function in zebra/zserv.c of Quagga1.2.x (ZAPI3)
// Reference: zread_hello function in zebra/zserv.c of FRR3.x (ZAPI4)
// Reference: zread_hello function in zebra/zapi_msg.c of FRR5.x (ZAPI5)
func (b *HelloBody) DecodeFromBytes(data []byte, version uint8) error {
	b.RedistDefault = ROUTE_TYPE(data[0])
	if version >= 4 {
		b.Instance = binary.BigEndian.Uint16(data[1:3])
		if version >= 5 {
			b.ReceiveNotify = data[3]
		}
	}
	return nil
}

// Reference: zebra_hello_send function in lib/zclient.c of Quagga1.2.x (ZAPI3)
// Reference: zebra_hello_send function in lib/zclient.c of FRR3.x (ZAPI4)
// Reference: zebra_hello_send function in lib/zclient.c of FRR5.x (ZAPI5)
func (b *HelloBody) Serialize(version uint8) ([]byte, error) {
	if version <= 3 {
		return []byte{uint8(b.RedistDefault)}, nil
	} else { // version >= 4
		var buf []byte
		if version == 4 {
			buf = make([]byte, 3)
		} else if version >= 5 {
			buf = make([]byte, 4)
		}
		buf[0] = uint8(b.RedistDefault)
		binary.BigEndian.PutUint16(buf[1:3], b.Instance)
		if version >= 5 {
			buf[3] = b.ReceiveNotify
		}
		return buf, nil
	}
}

func (b *HelloBody) String() string {
	return fmt.Sprintf(
		"route_type: %s, instance :%d",
		b.RedistDefault.String(), b.Instance)
}

type RedistributeBody struct {
	Afi      AFI
	Redist   ROUTE_TYPE
	Instance uint16
}

//  Reference: zebra_redistribute_add function in zebra/redistribute.c of Quagga1.2.x (ZAPI3)
//  Reference: zebra_redistribute_add function in zebra/redistribute.c of FRR3.x (ZAPI4)
//  Reference: zebra_redistribute_add function in zebra/redistribute.c of FRR5.x (ZAPI5)
func (b *RedistributeBody) DecodeFromBytes(data []byte, version uint8) error {
	if version <= 3 {
		b.Redist = ROUTE_TYPE(data[0])
	} else { // version >= 4
		b.Afi = AFI(data[0])
		b.Redist = ROUTE_TYPE(data[1])
		b.Instance = binary.BigEndian.Uint16(data[2:4])
	}
	return nil
}

//  Reference: zebra_redistribute_send function in lib/zclient.c of Quagga1.2.x (ZAPI3)
//  Reference: zebra_redistribute_send function in lib/zclient.c of FRR3.x (ZAPI4)
//  Reference: zebra_redistribute_send function in lib/zclient.c of FRR5.x (ZAPI5)
func (b *RedistributeBody) Serialize(version uint8) ([]byte, error) {
	if version <= 3 {
		return []byte{uint8(b.Redist)}, nil
	} else { // version >= 4
		buf := make([]byte, 4)
		buf[0] = uint8(b.Afi)
		buf[1] = uint8(b.Redist)
		binary.BigEndian.PutUint16(buf[2:4], b.Instance)
		return buf, nil
	}
}

func (b *RedistributeBody) String() string {
	return fmt.Sprintf(
		"afi: %s, route_type: %s, instance :%d",
		b.Afi.String(), b.Redist.String(), b.Instance)
}

type LinkParam struct {
	Status      uint32
	TeMetric    uint32
	MaxBw       float32
	MaxRsvBw    float32
	UnrsvBw     [8]float32
	BwClassNum  uint32
	AdminGroup  uint32
	RemoteAS    uint32
	RemoteIP    net.IP
	AveDelay    uint32
	MinDelay    uint32
	MaxDelay    uint32
	DelayVar    uint32
	PktLoss     float32
	ResidualBw  float32
	AvailableBw float32
	UseBw       float32
}

type InterfaceUpdateBody struct {
	Name         string
	Index        uint32
	Status       INTERFACE_STATUS
	Flags        uint64
	PTMEnable    PTM_ENABLE
	PTMStatus    PTM_STATUS
	Metric       uint32
	Speed        uint32
	MTU          uint32
	MTU6         uint32
	Bandwidth    uint32
	Linktype     LINK_TYPE
	HardwareAddr net.HardwareAddr
	LinkParam    LinkParam
}

//  Reference: zebra_interface_if_set_value function in lib/zclient.c of Quagga1.2.x (ZAPI4)
//  Reference: zebra_interface_if_set_value function in lib/zclient.c of FRR3.x (ZAPI4)
//  Reference: zebra_interface_if_set_value function in lib/zclient.c of FRR5.x (ZAPI5)
func (b *InterfaceUpdateBody) DecodeFromBytes(data []byte, version uint8) error {
	if len(data) < INTERFACE_NAMSIZ+29 {
		return fmt.Errorf("lack of bytes. need %d but %d", INTERFACE_NAMSIZ+29, len(data))
	}

	b.Name = strings.Trim(string(data[:INTERFACE_NAMSIZ]), "\u0000")
	data = data[INTERFACE_NAMSIZ:]
	b.Index = binary.BigEndian.Uint32(data[0:4])
	b.Status = INTERFACE_STATUS(data[4])
	b.Flags = binary.BigEndian.Uint64(data[5:13])
	if version >= 4 {
		b.PTMEnable = PTM_ENABLE(data[13])
		b.PTMStatus = PTM_STATUS(data[14])
		b.Metric = binary.BigEndian.Uint32(data[15:19])
		b.Speed = binary.BigEndian.Uint32(data[19:23])
		data = data[23:]
	} else {
		b.Metric = binary.BigEndian.Uint32(data[13:17])
		data = data[17:]
	}
	b.MTU = binary.BigEndian.Uint32(data[0:4])
	b.MTU6 = binary.BigEndian.Uint32(data[4:8])
	b.Bandwidth = binary.BigEndian.Uint32(data[8:12])
	if version >= 3 {
		b.Linktype = LINK_TYPE(binary.BigEndian.Uint32(data[12:16]))
		data = data[16:]
	} else {
		data = data[12:]
	}
	l := binary.BigEndian.Uint32(data[:4])
	if l > 0 {
		if len(data) < 4+int(l) {
			return fmt.Errorf("lack of bytes. need %d but %d", 4+l, len(data))
		}
		b.HardwareAddr = data[4 : 4+l]
	}
	if version >= 5 {
		LinkParam := data[4+l]
		if LinkParam > 0 {
			data = data[5+l:]
			b.LinkParam.Status = binary.BigEndian.Uint32(data[0:4])
			b.LinkParam.TeMetric = binary.BigEndian.Uint32(data[4:8])
			b.LinkParam.MaxBw = math.Float32frombits(binary.BigEndian.Uint32(data[8:12]))
			b.LinkParam.MaxRsvBw = math.Float32frombits(binary.BigEndian.Uint32(data[12:16]))
			b.LinkParam.BwClassNum = binary.BigEndian.Uint32(data[16:20])
			for i := uint32(0); i < b.LinkParam.BwClassNum; i++ {
				b.LinkParam.UnrsvBw[i] = math.Float32frombits(binary.BigEndian.Uint32(data[20+i*4 : 24+i*4]))
			}
			data = data[20+b.LinkParam.BwClassNum*4:]
			b.LinkParam.AdminGroup = binary.BigEndian.Uint32(data[0:4])
			b.LinkParam.RemoteAS = binary.BigEndian.Uint32(data[4:8])
			b.LinkParam.RemoteIP = data[8:12]
			b.LinkParam.AveDelay = binary.BigEndian.Uint32(data[12:16])
			b.LinkParam.MinDelay = binary.BigEndian.Uint32(data[16:20])
			b.LinkParam.MaxDelay = binary.BigEndian.Uint32(data[20:24])
			b.LinkParam.DelayVar = binary.BigEndian.Uint32(data[24:28])
			b.LinkParam.PktLoss = math.Float32frombits(binary.BigEndian.Uint32(data[28:32]))
			b.LinkParam.ResidualBw = math.Float32frombits(binary.BigEndian.Uint32(data[32:36]))
			b.LinkParam.AvailableBw = math.Float32frombits(binary.BigEndian.Uint32(data[36:40]))
			b.LinkParam.UseBw = math.Float32frombits(binary.BigEndian.Uint32(data[40:44]))
		}
	}
	return nil
}

func (b *InterfaceUpdateBody) Serialize(version uint8) ([]byte, error) {
	return []byte{}, nil
}

func (b *InterfaceUpdateBody) String() string {
	s := fmt.Sprintf(
		"name: %s, idx: %d, status: %s, flags: %s, ptm_enable: %s, ptm_status: %s, metric: %d, speed: %d, mtu: %d, mtu6: %d, bandwidth: %d, linktype: %s",
		b.Name, b.Index, b.Status.String(), intfflag2string(b.Flags), b.PTMEnable.String(), b.PTMStatus.String(), b.Metric, b.Speed, b.MTU, b.MTU6, b.Bandwidth, b.Linktype.String())
	if len(b.HardwareAddr) > 0 {
		return s + fmt.Sprintf(", mac: %s", b.HardwareAddr.String())
	}
	return s
}

type InterfaceAddressUpdateBody struct {
	Index       uint32
	Flags       INTERFACE_ADDRESS_FLAG
	Prefix      net.IP
	Length      uint8
	Destination net.IP
}

//  Reference: zebra_interface_address_read function in lib/zclient.c of Quagga1.2.x (ZAPI4)
//  Reference: zebra_interface_address_read function in lib/zclient.c of FRR3.x (ZAPI4)
//  Reference: zebra_interface_address_read function in lib/zclient.c of FRR5.x (ZAPI5)
func (b *InterfaceAddressUpdateBody) DecodeFromBytes(data []byte, version uint8) error {
	b.Index = binary.BigEndian.Uint32(data[:4])
	b.Flags = INTERFACE_ADDRESS_FLAG(data[4])
	family := data[5]
	addrlen, err := addressByteLength(family)
	if err != nil {
		return err
	}
	b.Prefix = data[6 : 6+addrlen]
	b.Length = data[6+addrlen]
	b.Destination = data[7+addrlen : 7+addrlen*2]
	return nil
}

func (b *InterfaceAddressUpdateBody) Serialize(version uint8) ([]byte, error) {
	return []byte{}, nil
}

func (b *InterfaceAddressUpdateBody) String() string {
	return fmt.Sprintf(
		"idx: %d, flags: %s, addr: %s/%d",
		b.Index, b.Flags.String(), b.Prefix.String(), b.Length)
}

type RouterIDUpdateBody struct {
	Length uint8
	Prefix net.IP
}

//  Reference: zebra_router_id_update_read function in lib/zclient.c of Quagga1.2.x (ZAPI4)
//  Reference: zebra_router_id_update_read function in lib/zclient.c of FRR3.x (ZAPI4)
//  Reference: zebra_router_id_update_read function in lib/zclient.c of FRR5.x (ZAPI5)
func (b *RouterIDUpdateBody) DecodeFromBytes(data []byte, version uint8) error {
	family := data[0]

	addrlen, err := addressByteLength(family)
	if err != nil {
		return err
	}
	b.Prefix = data[1 : 1+addrlen]
	b.Length = data[1+addrlen]
	return nil
}

func (b *RouterIDUpdateBody) Serialize(version uint8) ([]byte, error) {
	return []byte{}, nil
}

func (b *RouterIDUpdateBody) String() string {
	return fmt.Sprintf("id: %s/%d", b.Prefix.String(), b.Length)
}

/*
 Reference: struct zapi_nexthop in lib/zclient.h of FRR5.x (ZAPI5)
*/
type Nexthop struct {
	Type          NEXTHOP_TYPE
	VrfId         uint32
	Ifindex       uint32
	Gate          net.IP
	BlackholeType uint8
	LabelNum      uint8
	MplsLabels    []uint32
}

func (n *Nexthop) String() string {
	s := fmt.Sprintf(
		"type: %s, gate: %s, ifindex: %d",
		n.Type.String(), n.Gate.String(), n.Ifindex)
	return s
}

type Prefix struct {
	Family    uint8
	PrefixLen uint8
	Prefix    net.IP
}

type IPRouteBody struct {
	Type      ROUTE_TYPE
	Instance  uint16
	Flags     FLAG
	Message   MESSAGE_FLAG
	SAFI      SAFI
	Prefix    Prefix
	SrcPrefix Prefix
	Nexthops  []Nexthop
	Distance  uint8
	Metric    uint32
	Mtu       uint32
	Tag       uint32
	Rmac      [6]byte
	Api       API_TYPE
}

func (b *IPRouteBody) RouteFamily(version uint8) bgp.RouteFamily {
	if b == nil {
		return bgp.RF_OPAQUE
	}
	family := addressFamilyFromApi(b.Api, version)
	if family == syscall.AF_UNSPEC {
		if b.Prefix.Prefix.To4() != nil {
			family = syscall.AF_INET
		} else if b.Prefix.Prefix.To16() != nil {
			family = syscall.AF_INET6
		}
	}
	switch family {
	case syscall.AF_INET:
		return bgp.RF_IPv4_UC
	case syscall.AF_INET6:
		return bgp.RF_IPv6_UC
	}
	return bgp.RF_OPAQUE
}

func (b *IPRouteBody) IsWithdraw(version uint8) bool {
	if version <= 3 {
		switch b.Api {
		case IPV4_ROUTE_DELETE, IPV6_ROUTE_DELETE:
			return true
		}
	} else if version == 4 {
		switch b.Api {
		case FRR_IPV4_ROUTE_DELETE, FRR_IPV6_ROUTE_DELETE, FRR_REDISTRIBUTE_IPV4_DEL, FRR_REDISTRIBUTE_IPV6_DEL:
			return true
		}
	} else if version == 5 {
		switch b.Api {
		case FRR_ZAPI5_ROUTE_DELETE, FRR_ZAPI5_IPV4_ROUTE_DELETE, FRR_ZAPI5_IPV6_ROUTE_DELETE, FRR_ZAPI5_REDISTRIBUTE_ROUTE_DEL:
			return true
		}
	} else if version >= 6 {
		switch b.Api {
		case FRR_ZAPI6_ROUTE_DELETE, FRR_ZAPI6_REDISTRIBUTE_ROUTE_DEL:
			return true
		}
	}
	return false
}

// Reference: zapi_ipv4_route function in lib/zclient.c  of Quagga1.2.x (ZAPI3)
// Reference: zapi_ipv4_route function in lib/zclient.c  of FRR3.x (ZAPI4)
// Reference: zapi_route_encode function in lib/zclient.c of FRR5.x (ZAPI5)
func (b *IPRouteBody) Serialize(version uint8) ([]byte, error) {
	var buf []byte
	if version <= 3 {
		buf = make([]byte, 5)
	} else if version == 4 {
		buf = make([]byte, 10)
	} else { // version >= 5
		buf = make([]byte, 9)
	}
	buf[0] = uint8(b.Type)
	if version <= 3 {
		buf[1] = uint8(b.Flags)
		buf[2] = uint8(b.Message)
		binary.BigEndian.PutUint16(buf[3:5], uint16(b.SAFI))
	} else { // version >= 4
		binary.BigEndian.PutUint16(buf[1:3], uint16(b.Instance))
		binary.BigEndian.PutUint32(buf[3:7], uint32(b.Flags))
		buf[7] = uint8(b.Message)
		if version == 4 {
			binary.BigEndian.PutUint16(buf[8:10], uint16(b.SAFI))
		} else { // version >= 5
			buf[8] = uint8(b.SAFI)
			if b.Flags&FLAG_EVPN_ROUTE > 0 {
				// size of struct ethaddr is 6 octets defined by ETH_ALEN
				buf = append(buf, b.Rmac[:6]...)
			}
			if b.Prefix.Family == syscall.AF_UNSPEC {
				if b.Prefix.Prefix.To4() != nil {
					b.Prefix.Family = syscall.AF_INET
				} else if b.Prefix.Prefix.To16() != nil {
					b.Prefix.Family = syscall.AF_INET6
				}
			}
			buf = append(buf, b.Prefix.Family)
		}
	}
	byteLen := (int(b.Prefix.PrefixLen) + 7) / 8
	buf = append(buf, b.Prefix.PrefixLen)
	buf = append(buf, b.Prefix.Prefix[:byteLen]...)

	if (version == 4 && b.Message&FRR_MESSAGE_SRCPFX > 0) ||
		(version >= 5 && b.Message&FRR_ZAPI5_MESSAGE_SRCPFX > 0) {
		byteLen = (int(b.SrcPrefix.PrefixLen) + 7) / 8
		buf = append(buf, b.SrcPrefix.PrefixLen)
		buf = append(buf, b.SrcPrefix.Prefix[:byteLen]...)
	}
	if (version <= 3 && b.Message&MESSAGE_NEXTHOP > 0) ||
		(version == 4 && b.Message&FRR_MESSAGE_NEXTHOP > 0) ||
		(version >= 5 && b.Message&FRR_ZAPI5_MESSAGE_NEXTHOP > 0) {
		if version < 5 {
			if b.Flags&FLAG_BLACKHOLE > 0 {
				buf = append(buf, []byte{1, uint8(NEXTHOP_TYPE_BLACKHOLE)}...)
			} else {
				buf = append(buf, uint8(len(b.Nexthops)))
			}
		} else { // version >= 5
			bbuf := make([]byte, 2)
			binary.BigEndian.PutUint16(bbuf, uint16(len(b.Nexthops)))
			buf = append(buf, bbuf...)
		}
		for _, nexthop := range b.Nexthops {
			if version >= 5 {
				bbuf := make([]byte, 4)
				binary.BigEndian.PutUint32(bbuf, nexthop.VrfId)
				buf = append(buf, bbuf...)
			}

			if nexthop.Type == NEXTHOP_TYPE(0) {
				if nexthop.Gate.To4() != nil {
					if version <= 3 {
						nexthop.Type = NEXTHOP_TYPE_IPV4
					} else {
						nexthop.Type = FRR_NEXTHOP_TYPE_IPV4
					}
					if version >= 5 && nexthop.Ifindex > 0 {
						nexthop.Type = FRR_NEXTHOP_TYPE_IPV4_IFINDEX
					}
				} else if nexthop.Gate.To16() != nil {
					if version <= 3 {
						nexthop.Type = NEXTHOP_TYPE_IPV6
					} else {
						nexthop.Type = FRR_NEXTHOP_TYPE_IPV6
					}
					if version >= 5 && nexthop.Ifindex > 0 {
						nexthop.Type = FRR_NEXTHOP_TYPE_IPV6_IFINDEX
					}
				} else if nexthop.Ifindex > 0 {
					if version <= 3 {
						nexthop.Type = NEXTHOP_TYPE_IFINDEX
					} else {
						nexthop.Type = FRR_NEXTHOP_TYPE_IFINDEX
					}
				} else if version >= 5 {
					nexthop.Type = FRR_NEXTHOP_TYPE_BLACKHOLE
				}
			}

			buf = append(buf, uint8(nexthop.Type))

			if (version <= 3 && nexthop.Type == NEXTHOP_TYPE_IPV4) ||
				(version >= 4 && nexthop.Type == FRR_NEXTHOP_TYPE_IPV4) {
				buf = append(buf, nexthop.Gate.To4()...)
			} else if (version <= 3 && nexthop.Type == NEXTHOP_TYPE_IPV6) ||
				(version >= 4 && nexthop.Type == FRR_NEXTHOP_TYPE_IPV6) {
				buf = append(buf, nexthop.Gate.To16()...)
			} else if (version <= 3 && nexthop.Type == NEXTHOP_TYPE_IFINDEX) ||
				(version >= 4 && nexthop.Type == FRR_NEXTHOP_TYPE_IFINDEX) {
				bbuf := make([]byte, 4)
				binary.BigEndian.PutUint32(bbuf, nexthop.Ifindex)
				buf = append(buf, bbuf...)
			} else if version >= 5 && nexthop.Type == FRR_NEXTHOP_TYPE_IPV4_IFINDEX {
				buf = append(buf, nexthop.Gate.To4()...)
				bbuf := make([]byte, 4)
				binary.BigEndian.PutUint32(bbuf, nexthop.Ifindex)
				buf = append(buf, bbuf...)
			} else if version >= 5 && nexthop.Type == FRR_NEXTHOP_TYPE_IPV6_IFINDEX {
				buf = append(buf, nexthop.Gate.To16()...)
				bbuf := make([]byte, 4)
				binary.BigEndian.PutUint32(bbuf, nexthop.Ifindex)
				buf = append(buf, bbuf...)
			} else if version >= 5 && nexthop.Type == FRR_NEXTHOP_TYPE_BLACKHOLE {
				buf = append(buf, uint8(nexthop.BlackholeType))
			}
			if version >= 5 && b.Message&FRR_ZAPI5_MESSAGE_LABEL > 0 {
				buf = append(buf, nexthop.LabelNum)
				bbuf := make([]byte, 4)
				binary.BigEndian.PutUint32(bbuf, nexthop.MplsLabels[0])
				buf = append(buf, bbuf...)
			}
		}
		if (version <= 3 && b.Message&MESSAGE_DISTANCE > 0) ||
			(version == 4 && b.Message&FRR_MESSAGE_DISTANCE > 0) ||
			(version >= 5 && b.Message&FRR_ZAPI5_MESSAGE_DISTANCE > 0) {
			buf = append(buf, b.Distance)
		}
		if (version <= 3 && b.Message&MESSAGE_METRIC > 0) ||
			(version == 4 && b.Message&FRR_MESSAGE_METRIC > 0) ||
			(version >= 5 && b.Message&FRR_ZAPI5_MESSAGE_METRIC > 0) {
			bbuf := make([]byte, 4)
			binary.BigEndian.PutUint32(bbuf, b.Metric)
			buf = append(buf, bbuf...)
		}
		if (version <= 3 && b.Message&MESSAGE_MTU > 0) ||
			(version == 4 && b.Message&FRR_MESSAGE_MTU > 0) ||
			(version >= 5 && b.Message&FRR_ZAPI5_MESSAGE_MTU > 0) {
			bbuf := make([]byte, 4)
			binary.BigEndian.PutUint32(bbuf, b.Mtu)
			buf = append(buf, bbuf...)
		}
		if (version <= 3 && b.Message&MESSAGE_TAG > 0) ||
			(version == 4 && b.Message&FRR_MESSAGE_TAG > 0) ||
			(version >= 5 && b.Message&FRR_ZAPI5_MESSAGE_TAG > 0) {
			bbuf := make([]byte, 4)
			binary.BigEndian.PutUint32(bbuf, b.Tag)
			buf = append(buf, bbuf...)
		}
	}
	return buf, nil
}

// Reference: zebra_read_ipv4 function in bgpd/bgp_zebra.c of Quagga1.2.x (ZAPI3)
// Reference: zebra_read_ipv4 function in bgpd/bgp_zebra.c of FRR4.x (ZAPI4)
// Reference: zapi_route_decode function in lib/zclient.c of FRR5.x (ZAPI5)
func (b *IPRouteBody) DecodeFromBytes(data []byte, version uint8) error {
	if b == nil {
		return fmt.Errorf("[IPRouteBody DecodeFromBytes] IPRouteBody is nil")
	}
	b.Prefix.Family = addressFamilyFromApi(b.Api, version)
	/* REDSTRIBUTE_IPV4_ADD|DEL and REDSITRBUTE_IPV6_ADD|DEL have merged to
	   REDISTRIBUTE_ROUTE_ADD|DEL in ZAPI version 5.
	   Therefore it can not judge the protocol famiiy from API. */

	b.Type = ROUTE_TYPE(data[0])
	if version <= 3 {
		b.Flags = FLAG(data[1])
		data = data[2:]
	} else { // version >= 4
		b.Instance = binary.BigEndian.Uint16(data[1:3])
		b.Flags = FLAG(binary.BigEndian.Uint32(data[3:7]))
		data = data[7:]
	}

	b.Message = MESSAGE_FLAG(data[0])
	b.SAFI = SAFI(SAFI_UNICAST)
	if version >= 5 {
		b.SAFI = SAFI(data[1])
		data = data[2:]
		if b.Flags&FLAG_EVPN_ROUTE > 0 {
			// size of struct ethaddr is 6 octets defined by ETH_ALEN
			copy(b.Rmac[0:6], data[0:6])
			data = data[6:]
		}
		b.Prefix.Family = data[0]
	}

	addrByteLen, err := addressByteLength(b.Prefix.Family)
	if err != nil {
		return err
	}

	addrBitLen := uint8(addrByteLen * 8)

	b.Prefix.PrefixLen = data[1]
	if b.Prefix.PrefixLen > addrBitLen {
		return fmt.Errorf("prefix length is greater than %d", addrByteLen*8)
	}
	pos := 2
	rest := len(data[pos:]) + 2

	buf := make([]byte, addrByteLen)
	byteLen := int((b.Prefix.PrefixLen + 7) / 8)
	if pos+byteLen > rest {
		return fmt.Errorf("message length invalid pos:%d rest:%d", pos, rest)
	}
	copy(buf, data[pos:pos+byteLen])
	b.Prefix.Prefix = ipFromFamily(b.Prefix.Family, buf)
	pos += byteLen

	if (version == 4 && b.Message&FRR_MESSAGE_SRCPFX > 0) ||
		(version >= 5 && b.Message&FRR_ZAPI5_MESSAGE_SRCPFX > 0) {
		if pos+1 > rest {
			return fmt.Errorf("MESSAGE_SRCPFX message length invalid pos:%d rest:%d", pos, rest)
		}
		b.SrcPrefix.PrefixLen = data[pos]
		if b.SrcPrefix.PrefixLen > addrBitLen {
			return fmt.Errorf("prefix length is greater than %d", addrByteLen*8)
		}
		pos += 1
		buf = make([]byte, addrByteLen)
		byteLen = int((b.SrcPrefix.PrefixLen + 7) / 8)
		copy(buf, data[pos:pos+byteLen])
		if pos+byteLen > rest {
			return fmt.Errorf("MESSAGE_SRCPFX message length invalid pos:%d rest:%d", pos, rest)
		}
		b.SrcPrefix.Prefix = ipFromFamily(b.Prefix.Family, buf)
		pos += byteLen
	}

	b.Nexthops = []Nexthop{}
	if (version <= 3 && b.Message&MESSAGE_NEXTHOP > 0) ||
		(version == 4 && b.Message&FRR_MESSAGE_NEXTHOP > 0) ||
		(version >= 5 && b.Message&FRR_ZAPI5_MESSAGE_NEXTHOP > 0) {
		var numNexthop uint16
		if version <= 4 {
			if pos+1 > rest {
				return fmt.Errorf("MESSAGE_NEXTHOP message length invalid pos:%d rest:%d", pos, rest)
			}
			numNexthop = uint16(data[pos])
			pos += 1
		} else { // version >= 5
			if pos+2 > rest {
				return fmt.Errorf("MESSAGE_NEXTHOP message length invalid pos:%d rest:%d", pos, rest)
			}
			numNexthop = binary.BigEndian.Uint16(data[pos : pos+2])
			pos += 2
		}
		for i := 0; i < int(numNexthop); i++ {
			var nexthop Nexthop
			if version <= 3 {
				if b.Prefix.Family == syscall.AF_INET {
					nexthop.Type = NEXTHOP_TYPE_IPV4
				} else if b.Prefix.Family == syscall.AF_INET6 {
					nexthop.Type = NEXTHOP_TYPE_IPV6
				}
			} else if version == 4 {
				if b.Prefix.Family == syscall.AF_INET {
					nexthop.Type = FRR_NEXTHOP_TYPE_IPV4
				} else if b.Prefix.Family == syscall.AF_INET6 {
					nexthop.Type = FRR_NEXTHOP_TYPE_IPV6
				}
			} else { // version >= 5
				if pos+5 > rest {
					return fmt.Errorf("MESSAGE_NEXTHOP message length invalid pos:%d rest:%d", pos, rest)
				}
				nexthop.VrfId = binary.BigEndian.Uint32(data[pos : pos+4])
				nexthop.Type = NEXTHOP_TYPE(data[pos+4])
				pos += 5
			}

			if (version <= 3 && nexthop.Type == NEXTHOP_TYPE_IPV4) ||
				(version >= 4 && nexthop.Type == FRR_NEXTHOP_TYPE_IPV4) {
				if pos+4 > rest {
					return fmt.Errorf("MESSAGE_NEXTHOP NEXTHOP_TYPE_IPV4 message length invalid pos:%d rest:%d", pos, rest)
				}
				addr := data[pos : pos+4]
				nexthop.Gate = net.IP(addr).To4()
				pos += 4
			} else if (version <= 3 && nexthop.Type == NEXTHOP_TYPE_IPV6) ||
				(version >= 4 && nexthop.Type == FRR_NEXTHOP_TYPE_IPV6) {
				if pos+16 > rest {
					return fmt.Errorf("MESSAGE_NEXTHOP NEXTHOP_TYPE_IPV6 message length invalid pos:%d rest:%d", pos, rest)
				}
				addr := data[pos : pos+16]
				nexthop.Gate = net.IP(addr).To16()
				pos += 16
			} else if version >= 5 && nexthop.Type == FRR_NEXTHOP_TYPE_IFINDEX {
				if pos+4 > rest {
					return fmt.Errorf("MESSAGE_NEXTHOP NEXTHOP_TYPE_IFINDEX message length invalid pos:%d rest:%d", pos, rest)
				}
				nexthop.Ifindex = binary.BigEndian.Uint32(data[pos : pos+4])
				pos += 4
				// barkward compatibility
				if b.Prefix.Family == syscall.AF_INET {
					nexthop.Gate = net.ParseIP("0.0.0.0")
				} else if b.Prefix.Family == syscall.AF_INET6 {
					nexthop.Gate = net.ParseIP("::")
				}
			} else if version >= 5 && nexthop.Type == FRR_NEXTHOP_TYPE_IPV4_IFINDEX {
				if pos+8 > rest {
					return fmt.Errorf("MESSAGE_NEXTHOP NEXTHOP_TYPE_IPV4_IFINDEX message length invalid pos:%d rest:%d", pos, rest)
				}
				addr := data[pos : pos+4]
				nexthop.Gate = net.IP(addr).To4()
				nexthop.Ifindex = binary.BigEndian.Uint32(data[pos+4 : pos+8])
				pos += 8
			} else if version >= 5 && nexthop.Type == FRR_NEXTHOP_TYPE_IPV6_IFINDEX {
				if pos+20 > rest {
					return fmt.Errorf("MESSAGE_NEXTHOP NEXTHOP_TYPE_IPV6_IFINDEX message length invalid pos:%d rest:%d", pos, rest)
				}
				addr := data[pos : pos+16]
				nexthop.Gate = net.IP(addr).To16()
				nexthop.Ifindex = binary.BigEndian.Uint32(data[pos+16 : pos+20])
				pos += 20
			} else if version >= 5 && nexthop.Type == FRR_NEXTHOP_TYPE_BLACKHOLE {
				if pos+1 > rest {
					return fmt.Errorf("MESSAGE_NEXTHOP NEXTHOP_TYPE_BLACKHOLE message length invalid pos:%d rest:%d", pos, rest)
				}
				nexthop.BlackholeType = data[pos]
				pos += 1
			}
			b.Nexthops = append(b.Nexthops, nexthop)
		}
	}

	if (version <= 3 && b.Message&MESSAGE_IFINDEX > 0) ||
		(version == 4 && b.Message&FRR_MESSAGE_IFINDEX > 0) {
		if pos+1 > rest {
			return fmt.Errorf("MESSAGE_IFINDEX message length invalid pos:%d rest:%d", pos, rest)
		}
		numIfIndex := uint8(data[pos])
		pos += 1
		for i := 0; i < int(numIfIndex); i++ {
			if pos+4 > rest {
				return fmt.Errorf("MESSAGE_IFINDEX message length invalid pos:%d rest:%d", pos, rest)
			}
			var nexthop Nexthop
			nexthop.Ifindex = binary.BigEndian.Uint32(data[pos : pos+4])
			if version <= 3 {
				nexthop.Type = NEXTHOP_TYPE_IFINDEX
			} else if version == 4 {
				nexthop.Type = FRR_NEXTHOP_TYPE_IFINDEX
			}
			b.Nexthops = append(b.Nexthops, nexthop)
			pos += 4
		}
	}

	if (version <= 3 && b.Message&MESSAGE_DISTANCE > 0) ||
		(version == 4 && b.Message&FRR_MESSAGE_DISTANCE > 0) ||
		(version >= 5 && b.Message&FRR_ZAPI5_MESSAGE_DISTANCE > 0) {
		if pos+1 > rest {
			return fmt.Errorf("MESSAGE_DISTANCE message length invalid pos:%d rest:%d", pos, rest)
		}
		b.Distance = data[pos]
		pos += 1
	}
	if (version <= 3 && b.Message&MESSAGE_METRIC > 0) ||
		(version == 4 && b.Message&FRR_MESSAGE_METRIC > 0) ||
		(version >= 5 && b.Message&FRR_ZAPI5_MESSAGE_METRIC > 0) {
		if pos+4 > rest {
			return fmt.Errorf("MESSAGE_METRIC message length invalid pos:%d rest:%d", pos, rest)
		}
		b.Metric = binary.BigEndian.Uint32(data[pos : pos+4])
		pos += 4
	}
	if (version <= 3 && b.Message&MESSAGE_MTU > 0) ||
		(version == 4 && b.Message&FRR_MESSAGE_MTU > 0) ||
		(version >= 5 && b.Message&FRR_ZAPI5_MESSAGE_MTU > 0) {
		if pos+4 > rest {
			return fmt.Errorf("MESSAGE_MTU message length invalid pos:%d rest:%d", pos, rest)
		}
		b.Mtu = binary.BigEndian.Uint32(data[pos : pos+4])
		pos += 4
	}
	if (version <= 3 && b.Message&MESSAGE_TAG > 0) ||
		(version == 4 && b.Message&FRR_MESSAGE_TAG > 0) ||
		(version >= 5 && b.Message&FRR_ZAPI5_MESSAGE_TAG > 0) {
		if pos+4 > rest {
			return fmt.Errorf("MESSAGE_TAG message length invalid pos:%d rest:%d", pos, rest)
		}
		b.Tag = binary.BigEndian.Uint32(data[pos : pos+4])
		pos += 4
	}
	if pos != rest {
		return fmt.Errorf("message length invalid")
	}

	return nil
}

func (b *IPRouteBody) String() string {
	s := fmt.Sprintf(
		"type: %s, instance: %d, flags: %s, message: %d, safi: %s, prefix: %s/%d, src_prefix: %s/%d",
		b.Type.String(), b.Instance, b.Flags.String(), b.Message, b.SAFI.String(), b.Prefix.Prefix.String(), b.Prefix.PrefixLen, b.SrcPrefix.Prefix.String(), b.SrcPrefix.PrefixLen)
	for i, nh := range b.Nexthops {
		s += fmt.Sprintf(", nexthops[%d]: %s", i, nh.String())
		/*
			s += fmt.Sprintf(", nexthops[%d]: %s", i, nh.Gate.String())
			s += fmt.Sprintf(", ifindex[%d]: %d", i, nh.Ifindex)
		*/
	}
	return s + fmt.Sprintf(
		", distance: %d, metric: %d, mtu: %d, tag: %d",
		b.Distance, b.Metric, b.Mtu, b.Tag)
}

func decodeNexthopsFromBytes(nexthops *[]Nexthop, data []byte, family uint8, version uint8) (int, error) {
	addrByteLen, err := addressByteLength(family)
	if err != nil {
		return 0, err
	}

	numNexthop := int(data[0])
	offset := 1

	for i := 0; i < numNexthop; i++ {
		nexthop := Nexthop{}
		nexthop.Type = NEXTHOP_TYPE(data[offset])
		offset += 1

		// On Quagga, NEXTHOP_TYPE_IFNAME is same as NEXTHOP_TYPE_IFINDEX,
		// NEXTHOP_TYPE_IPV4_IFNAME is same as NEXTHOP_TYPE_IPV4_IFINDEX,
		// NEXTHOP_TYPE_IPV6_IFNAME is same as NEXTHOP_TYPE_IPV6_IFINDEX

		// On FRRouting version 3.0 or later, NEXTHOP_TYPE_IPV4 and NEXTHOP_TYPE_IPV6 have
		// the same structure with NEXTHOP_TYPE_IPV4_IFINDEX and NEXTHOP_TYPE_IPV6_IFINDEX.

		if (version <= 3 && (nexthop.Type == NEXTHOP_TYPE_IFINDEX || nexthop.Type == NEXTHOP_TYPE_IFNAME)) ||
			(version >= 4 && nexthop.Type == FRR_NEXTHOP_TYPE_IFINDEX) {
			nexthop.Ifindex = binary.BigEndian.Uint32(data[offset : offset+4])
			offset += 4
		} else if version <= 3 && nexthop.Type == NEXTHOP_TYPE_IPV4 {
			nexthop.Gate = net.IP(data[offset : offset+addrByteLen]).To4()
			offset += addrByteLen
		} else if version <= 3 && nexthop.Type == NEXTHOP_TYPE_IPV6 {
			nexthop.Gate = net.IP(data[offset : offset+addrByteLen]).To16()
			offset += addrByteLen
		} else if (version <= 3 && (nexthop.Type == NEXTHOP_TYPE_IPV4_IFINDEX || nexthop.Type == NEXTHOP_TYPE_IPV4_IFNAME)) ||
			(version >= 4 && (nexthop.Type == FRR_NEXTHOP_TYPE_IPV4 || nexthop.Type == FRR_NEXTHOP_TYPE_IPV4_IFINDEX)) {
			nexthop.Gate = net.IP(data[offset : offset+addrByteLen]).To4()
			offset += addrByteLen
			nexthop.Ifindex = binary.BigEndian.Uint32(data[offset : offset+4])
			offset += 4
		} else if (version <= 3 && (nexthop.Type == NEXTHOP_TYPE_IPV6_IFINDEX || nexthop.Type == NEXTHOP_TYPE_IPV6_IFNAME)) ||
			(version >= 4 && (nexthop.Type == FRR_NEXTHOP_TYPE_IPV6 || nexthop.Type == FRR_NEXTHOP_TYPE_IPV6_IFINDEX)) {
			nexthop.Gate = net.IP(data[offset : offset+addrByteLen]).To16()
			offset += addrByteLen
			nexthop.Ifindex = binary.BigEndian.Uint32(data[offset : offset+4])
			offset += 4
		}
		if version >= 5 {
			nexthop.LabelNum = data[offset]
			offset += 1
			if nexthop.LabelNum > MPLS_MAX_LABLE {
				nexthop.LabelNum = MPLS_MAX_LABLE
			}
			var n uint8
			for ; n < nexthop.LabelNum; n++ {
				nexthop.MplsLabels[n] = binary.BigEndian.Uint32(data[offset : offset+4])
				offset += 4
			}
		}
		*nexthops = append(*nexthops, nexthop)
	}

	return offset, nil
}

type NexthopLookupBody struct {
	Api      API_TYPE
	Addr     net.IP
	Distance uint8
	Metric   uint32
	Nexthops []Nexthop
}

// Quagga only. Reference: zread_ipv[4|6]_nexthop_lookup in zebra/zserv.c of Quagga1.2.x (ZAPI3)
func (b *NexthopLookupBody) Serialize(version uint8) ([]byte, error) {
	family := addressFamilyFromApi(b.Api, version)
	buf := make([]byte, 0)

	if family == syscall.AF_INET {
		buf = append(buf, b.Addr.To4()...)
	} else if family == syscall.AF_INET6 {
		buf = append(buf, b.Addr.To16()...)
	}
	return buf, nil
}

// Quagga only. Reference: zsend_ipv[4|6]_nexthop_lookup in zebra/zserv.c of Quagga1.2.x (ZAPI3)
func (b *NexthopLookupBody) DecodeFromBytes(data []byte, version uint8) error {
	family := addressFamilyFromApi(b.Api, version)
	addrByteLen, err := addressByteLength(family)
	if err != nil {
		return err
	}

	if len(data) < addrByteLen {
		return fmt.Errorf("message length invalid")
	}

	buf := make([]byte, addrByteLen)
	copy(buf, data[0:addrByteLen])
	pos := addrByteLen
	b.Addr = ipFromFamily(family, buf)

	if version >= 4 {
		b.Distance = data[pos]
		pos++
	}

	if len(data[pos:]) > int(1+addrByteLen) {
		b.Metric = binary.BigEndian.Uint32(data[pos : pos+4])
		pos += 4
		b.Nexthops = []Nexthop{}
		if nexthopsByteLen, err := decodeNexthopsFromBytes(&b.Nexthops, data[pos:], family, version); err != nil {
			return err
		} else {
			pos += nexthopsByteLen
		}
	}

	return nil
}

func (b *NexthopLookupBody) String() string {
	s := fmt.Sprintf(
		"addr: %s, distance:%d, metric: %d",
		b.Addr.String(), b.Distance, b.Metric)
	if len(b.Nexthops) > 0 {
		for _, nh := range b.Nexthops {
			s = s + fmt.Sprintf(", nexthop:{%s}", nh.String())
		}
	}
	return s
}

type ImportLookupBody struct {
	Api          API_TYPE
	PrefixLength uint8
	Prefix       net.IP
	Addr         net.IP
	Metric       uint32
	Nexthops     []Nexthop
}

// Quagga only. Reference: zread_ipv4_import_lookup in zebra/zserv.c of Quagga1.2.x (ZAPI3)
func (b *ImportLookupBody) Serialize(version uint8) ([]byte, error) {
	buf := make([]byte, 1)
	buf[0] = b.PrefixLength
	buf = append(buf, b.Addr.To4()...)
	return buf, nil
}

// Quagga only. Reference: zsend_ipv4_import_lookup in zebra/zserv.c of Quagga1.2.x (ZAPI3)
func (b *ImportLookupBody) DecodeFromBytes(data []byte, version uint8) error {
	family := addressFamilyFromApi(b.Api, version)
	addrByteLen, err := addressByteLength(family)
	if err != nil {
		return err
	}

	if len(data) < addrByteLen {
		return fmt.Errorf("message length invalid")
	}

	buf := make([]byte, addrByteLen)
	copy(buf, data[0:addrByteLen])
	pos := addrByteLen

	b.Addr = net.IP(buf).To4()

	if len(data[pos:]) > int(1+addrByteLen) {
		b.Metric = binary.BigEndian.Uint32(data[pos : pos+4])
		pos += 4
		b.Nexthops = []Nexthop{}
		if nexthopsByteLen, err := decodeNexthopsFromBytes(&b.Nexthops, data[pos:], family, version); err != nil {
			return err
		} else {
			pos += nexthopsByteLen
		}
	}

	return nil
}

func (b *ImportLookupBody) String() string {
	s := fmt.Sprintf(
		"prefix: %s/%d, addr: %s, metric: %d",
		b.Prefix.String(), b.PrefixLength, b.Addr.String(), b.Metric)
	if len(b.Nexthops) > 0 {
		for _, nh := range b.Nexthops {
			s = s + fmt.Sprintf(", nexthop:{%s}", nh.String())
		}
	}
	return s
}

type RegisteredNexthop struct {
	Connected uint8
	Family    uint16
	// Note: Ignores PrefixLength (uint8),
	// because this field should be always:
	// - 32 if Address Family is AF_INET
	// - 128 if Address Family is AF_INET6
	Prefix net.IP
}

func (n *RegisteredNexthop) Len() int {
	// Connected (1 byte) + Address Family (2 bytes) + Prefix Length (1 byte) + Prefix (variable)
	if n.Family == uint16(syscall.AF_INET) {
		return 4 + net.IPv4len
	} else {
		return 4 + net.IPv6len
	}
}

// Reference: sendmsg_nexthop in bgpd/bgp_nht.c of Quagga1.2.x (ZAPI3)
// Reference: sendmsg_zebra_rnh in bgpd/bgp_nht.c of FRR3.x (ZAPI4)
// Reference: zclient_send_rnh function in lib/zclient.c of FRR5.x (ZAPI5)
func (n *RegisteredNexthop) Serialize() ([]byte, error) {
	// Connected (1 byte)
	buf := make([]byte, 4)
	buf[0] = byte(n.Connected)

	// Address Family (2 bytes)
	binary.BigEndian.PutUint16(buf[1:3], n.Family)
	// Prefix Length (1 byte)
	addrByteLen, err := addressByteLength(uint8(n.Family))
	if err != nil {
		return nil, err
	}

	buf[3] = byte(addrByteLen * 8)
	// Prefix (variable)
	switch n.Family {
	case uint16(syscall.AF_INET):
		buf = append(buf, n.Prefix.To4()...)
	case uint16(syscall.AF_INET6):
		buf = append(buf, n.Prefix.To16()...)
	default:
		return nil, fmt.Errorf("invalid address family: %d", n.Family)
	}

	return buf, nil
}

// Reference: zserv_nexthop_register in zebra/zserv.c of Quagga1.2.x (ZAPI3)
// Reference: zserv_rnh_register in zebra/zserv.c of FRR3.x (ZAPI4)
// Reference: zread_rnh_register in zebra/zapi_msg.c of FRR5.x (ZAPI5)
func (n *RegisteredNexthop) DecodeFromBytes(data []byte) error {
	// Connected (1 byte)
	n.Connected = uint8(data[0])
	// Address Family (2 bytes)
	n.Family = binary.BigEndian.Uint16(data[1:3])
	// Note: Ignores Prefix Length (1 byte)
	addrByteLen := (int(data[3]) + 7) / 8
	// Prefix (variable)
	n.Prefix = ipFromFamily(uint8(n.Family), data[4:4+addrByteLen])

	return nil
}

func (n *RegisteredNexthop) String() string {
	return fmt.Sprintf(
		"connected: %d, family: %d, prefix: %s",
		n.Connected, n.Family, n.Prefix.String())
}

type NexthopRegisterBody struct {
	Api      API_TYPE
	Nexthops []*RegisteredNexthop
}

// Reference: sendmsg_nexthop in bgpd/bgp_nht.c of Quagga1.2.x (ZAPI3)
// Reference: sendmsg_zebra_rnh in bgpd/bgp_nht.c of FRR3.x (ZAPI4)
// Reference: zclient_send_rnh function in lib/zclient.c of FRR5.x (ZAPI5)
func (b *NexthopRegisterBody) Serialize(version uint8) ([]byte, error) {
	buf := make([]byte, 0)

	// List of Registered Nexthops
	for _, nh := range b.Nexthops {
		nhBuf, err := nh.Serialize()
		if err != nil {
			return nil, err
		}
		buf = append(buf, nhBuf...)
	}

	return buf, nil
}

// Reference: zserv_nexthop_register in zebra/zserv.c of Quagga1.2.x (ZAPI3)
// Reference: zserv_rnh_register in zebra/zserv.c of FRR3.x (ZAPI4)
// Reference: zread_rnh_register in zebra/zapi_msg.c of FRR5.x (ZAPI5)
func (b *NexthopRegisterBody) DecodeFromBytes(data []byte, version uint8) error {
	offset := 0

	// List of Registered Nexthops
	b.Nexthops = []*RegisteredNexthop{}
	for len(data[offset:]) > 0 {
		nh := new(RegisteredNexthop)
		err := nh.DecodeFromBytes(data[offset:])
		if err != nil {
			return err
		}
		b.Nexthops = append(b.Nexthops, nh)

		offset += nh.Len()
		if len(data) < offset {
			break
		}
	}

	return nil
}

func (b *NexthopRegisterBody) String() string {
	s := make([]string, 0)
	for _, nh := range b.Nexthops {
		s = append(s, fmt.Sprintf("nexthop:{%s}", nh.String()))
	}
	return strings.Join(s, ", ")
}

/* NEXTHOP_UPDATE message uses same data structure as IPRoute (zapi_route)
   in FRR version 4, 5 (ZApi version 5) */
type NexthopUpdateBody IPRouteBody

// Reference: send_client function in zebra/zebra_rnh.c of Quagga1.2.x (ZAPI3)
// Reference: send_client function in zebra/zebra_rnh.c of FRR3.x (ZAPI4)
// Reference: send_client function in zebra/zebra_rnh.c of FRR5.x (ZAPI5)
func (b *NexthopUpdateBody) Serialize(version uint8) ([]byte, error) {
	// Address Family (2 bytes)
	buf := make([]byte, 3)
	binary.BigEndian.PutUint16(buf, uint16(b.Prefix.Family))
	addrByteLen, err := addressByteLength(b.Prefix.Family)
	if err != nil {
		return nil, err
	}

	buf[2] = byte(addrByteLen * 8)
	// Prefix Length (1 byte) + Prefix (variable)
	switch b.Prefix.Family {
	case syscall.AF_INET:
		buf = append(buf, b.Prefix.Prefix.To4()...)
	case syscall.AF_INET6:
		buf = append(buf, b.Prefix.Prefix.To16()...)
	default:
		return nil, fmt.Errorf("invalid address family: %d", b.Prefix.Family)
	}
	if version >= 5 {
		// Type (1 byte) (if version>=5)
		// Instance (2 bytes) (if version>=5)
		buf = append(buf, byte(b.Type))
		bbuf := make([]byte, 2)
		binary.BigEndian.PutUint16(bbuf, b.Instance)
		buf = append(buf, bbuf...)
	}
	if version >= 4 {
		// Distance (1 byte) (if version>=4)
		buf = append(buf, b.Distance)
	}
	// Metric (4 bytes)
	bbuf := make([]byte, 4)
	binary.BigEndian.PutUint32(bbuf, b.Metric)
	buf = append(buf, bbuf...)
	// Number of Nexthops (1 byte)
	buf = append(buf, uint8(0)) // Temporary code
	// ToDo Processing Route Entry

	return buf, nil
}

// Reference: bgp_parse_nexthop_update function in bgpd/bgp_nht.c of Quagga1.2.x (ZAPI3)
// Reference: bgp_parse_nexthop_update function in bgpd/bgp_nht.c of FRR3.x (ZAPI4)
// Reference: zapi_nexthop_update_decode function in lib/zclient.c of FRR5.x (ZAPI5)
func (b *NexthopUpdateBody) DecodeFromBytes(data []byte, version uint8) error {
	// Address Family (2 bytes)
	prefixFamily := binary.BigEndian.Uint16(data[0:2])
	b.Prefix.Family = uint8(prefixFamily)
	b.Prefix.PrefixLen = data[2]
	offset := 3

	addrByteLen, err := addressByteLength(b.Prefix.Family)
	if err != nil {
		return err
	}

	b.Prefix.Prefix = ipFromFamily(b.Prefix.Family, data[offset:offset+addrByteLen])
	offset += addrByteLen

	if version >= 5 {
		b.Type = ROUTE_TYPE(data[offset])
		b.Instance = binary.BigEndian.Uint16(data[offset+1 : offset+3])
		offset += 3
	}
	// Distance (1 byte) (if version>=4)
	if version >= 4 {
		b.Distance = data[offset]
		offset += 1
	}
	// Metric (4 bytes)
	// Number of Nexthops (1 byte)
	if len(data[offset:]) < 5 {
		return fmt.Errorf("invalid message length: missing metric(4 bytes) or nexthops(1 byte): %d<5", len(data[offset:]))
	}
	b.Metric = binary.BigEndian.Uint32(data[offset : offset+4])
	offset += 4

	// List of Nexthops
	b.Nexthops = []Nexthop{}
	if nexthopsByteLen, err := decodeNexthopsFromBytes(&b.Nexthops, data[offset:], b.Prefix.Family, version); err != nil {
		return err
	} else {
		offset += nexthopsByteLen
	}
	return nil
}

func (b *NexthopUpdateBody) String() string {
	s := fmt.Sprintf(
		"family: %d, prefix: %s, distance: %d, metric: %d",
		b.Prefix.Family, b.Prefix.Prefix.String(), b.Distance, b.Metric)
	for _, nh := range b.Nexthops {
		s = s + fmt.Sprintf(", nexthop:{%s}", nh.String())
	}
	return s
}

type Message struct {
	Header Header
	Body   Body
}

func (m *Message) Serialize() ([]byte, error) {
	var body []byte
	if m.Body != nil {
		var err error
		body, err = m.Body.Serialize(m.Header.Version)
		if err != nil {
			return nil, err
		}
	}
	m.Header.Len = uint16(len(body)) + HeaderSize(m.Header.Version)
	hdr, err := m.Header.Serialize()
	if err != nil {
		return nil, err
	}
	return append(hdr, body...), nil
}

func (m *Message) parseMessage(data []byte) error {
	switch m.Header.Command {
	case INTERFACE_ADD, INTERFACE_DELETE, INTERFACE_UP, INTERFACE_DOWN:
		m.Body = &InterfaceUpdateBody{}
	case INTERFACE_ADDRESS_ADD, INTERFACE_ADDRESS_DELETE:
		m.Body = &InterfaceAddressUpdateBody{}
	case ROUTER_ID_UPDATE:
		m.Body = &RouterIDUpdateBody{}
	case IPV4_ROUTE_ADD, IPV6_ROUTE_ADD, IPV4_ROUTE_DELETE, IPV6_ROUTE_DELETE:
		m.Body = &IPRouteBody{Api: m.Header.Command}
	case IPV4_NEXTHOP_LOOKUP, IPV6_NEXTHOP_LOOKUP:
		m.Body = &NexthopLookupBody{Api: m.Header.Command}
	case IPV4_IMPORT_LOOKUP:
		m.Body = &ImportLookupBody{Api: m.Header.Command}
	case NEXTHOP_UPDATE:
		m.Body = &NexthopUpdateBody{Api: m.Header.Command}
	default:
		m.Body = &UnknownBody{}
	}
	return m.Body.DecodeFromBytes(data, m.Header.Version)
}

func (m *Message) parseFrrMessage(data []byte) error {
	switch m.Header.Command {
	case FRR_INTERFACE_ADD, FRR_INTERFACE_DELETE, FRR_INTERFACE_UP, FRR_INTERFACE_DOWN:
		m.Body = &InterfaceUpdateBody{}
	case FRR_INTERFACE_ADDRESS_ADD, FRR_INTERFACE_ADDRESS_DELETE:
		m.Body = &InterfaceAddressUpdateBody{}
	case FRR_ROUTER_ID_UPDATE:
		m.Body = &RouterIDUpdateBody{}
	case FRR_NEXTHOP_UPDATE:
		m.Body = &NexthopUpdateBody{}
	case FRR_INTERFACE_NBR_ADDRESS_ADD, FRR_INTERFACE_NBR_ADDRESS_DELETE:
		// TODO
		m.Body = &UnknownBody{}
	case FRR_INTERFACE_BFD_DEST_UPDATE:
		// TODO
		m.Body = &UnknownBody{}
	case FRR_IMPORT_CHECK_UPDATE:
		// TODO
		m.Body = &UnknownBody{}
	case FRR_BFD_DEST_REPLAY:
		// TODO
		m.Body = &UnknownBody{}
	case FRR_REDISTRIBUTE_IPV4_ADD, FRR_REDISTRIBUTE_IPV4_DEL, FRR_REDISTRIBUTE_IPV6_ADD, FRR_REDISTRIBUTE_IPV6_DEL:
		m.Body = &IPRouteBody{Api: m.Header.Command}
	case FRR_INTERFACE_VRF_UPDATE:
		// TODO
		m.Body = &UnknownBody{}
	case FRR_INTERFACE_LINK_PARAMS:
		// TODO
		m.Body = &UnknownBody{}
	case FRR_PW_STATUS_UPDATE:
		// TODO
		m.Body = &UnknownBody{}
	default:
		m.Body = &UnknownBody{}
	}
	return m.Body.DecodeFromBytes(data, m.Header.Version)
}

func (m *Message) parseFrrZapi5Message(data []byte) error {
	switch m.Header.Command {
	case FRR_ZAPI5_INTERFACE_ADD, FRR_ZAPI5_INTERFACE_DELETE, FRR_ZAPI5_INTERFACE_UP, FRR_ZAPI5_INTERFACE_DOWN:
		m.Body = &InterfaceUpdateBody{}
	case FRR_ZAPI5_INTERFACE_ADDRESS_ADD, FRR_ZAPI5_INTERFACE_ADDRESS_DELETE:
		m.Body = &InterfaceAddressUpdateBody{}
	case FRR_ZAPI5_ROUTER_ID_UPDATE:
		m.Body = &RouterIDUpdateBody{}
	case FRR_ZAPI5_NEXTHOP_UPDATE:
		m.Body = &NexthopUpdateBody{}
	case FRR_ZAPI5_INTERFACE_NBR_ADDRESS_ADD, FRR_ZAPI5_INTERFACE_NBR_ADDRESS_DELETE:
		// TODO
		m.Body = &UnknownBody{}
	case FRR_ZAPI5_INTERFACE_BFD_DEST_UPDATE:
		// TODO
		m.Body = &UnknownBody{}
	case FRR_ZAPI5_IMPORT_CHECK_UPDATE:
		// TODO
		m.Body = &UnknownBody{}
	case FRR_ZAPI5_BFD_DEST_REPLAY:
		// TODO
		m.Body = &UnknownBody{}
	case FRR_ZAPI5_REDISTRIBUTE_ROUTE_ADD, FRR_ZAPI5_REDISTRIBUTE_ROUTE_DEL:
		m.Body = &IPRouteBody{Api: m.Header.Command}
	case FRR_ZAPI5_INTERFACE_VRF_UPDATE:
		// TODO
		m.Body = &UnknownBody{}
	case FRR_ZAPI5_INTERFACE_LINK_PARAMS:
		// TODO
		m.Body = &UnknownBody{}
	case FRR_ZAPI5_PW_STATUS_UPDATE:
		// TODO
		m.Body = &UnknownBody{}
	default:
		m.Body = &UnknownBody{}
	}
	return m.Body.DecodeFromBytes(data, m.Header.Version)
}

func (m *Message) parseFrrZapi6Message(data []byte) error {
	switch m.Header.Command {
	case FRR_ZAPI6_INTERFACE_ADD, FRR_ZAPI6_INTERFACE_DELETE, FRR_ZAPI6_INTERFACE_UP, FRR_ZAPI6_INTERFACE_DOWN:
		m.Body = &InterfaceUpdateBody{}
	case FRR_ZAPI6_INTERFACE_ADDRESS_ADD, FRR_ZAPI6_INTERFACE_ADDRESS_DELETE:
		m.Body = &InterfaceAddressUpdateBody{}
	case FRR_ZAPI6_ROUTER_ID_UPDATE:
		m.Body = &RouterIDUpdateBody{}
	case FRR_ZAPI6_NEXTHOP_UPDATE:
		m.Body = &NexthopUpdateBody{}
	case FRR_ZAPI6_INTERFACE_NBR_ADDRESS_ADD, FRR_ZAPI6_INTERFACE_NBR_ADDRESS_DELETE:
		// TODO
		m.Body = &UnknownBody{}
	case FRR_ZAPI6_INTERFACE_BFD_DEST_UPDATE:
		// TODO
		m.Body = &UnknownBody{}
	case FRR_ZAPI6_IMPORT_CHECK_UPDATE:
		// TODO
		m.Body = &UnknownBody{}
	case FRR_ZAPI6_BFD_DEST_REPLAY:
		// TODO
		m.Body = &UnknownBody{}
	case FRR_ZAPI6_REDISTRIBUTE_ROUTE_ADD, FRR_ZAPI6_REDISTRIBUTE_ROUTE_DEL:
		m.Body = &IPRouteBody{Api: m.Header.Command}
	case FRR_ZAPI6_INTERFACE_VRF_UPDATE:
		// TODO
		m.Body = &UnknownBody{}
	case FRR_ZAPI6_INTERFACE_LINK_PARAMS:
		// TODO
		m.Body = &UnknownBody{}
	case FRR_ZAPI6_PW_STATUS_UPDATE:
		// TODO
		m.Body = &UnknownBody{}
	default:
		m.Body = &UnknownBody{}
	}
	return m.Body.DecodeFromBytes(data, m.Header.Version)
}

func ParseMessage(hdr *Header, data []byte) (m *Message, err error) {
	m = &Message{Header: *hdr}
	if m.Header.Version == 4 {
		err = m.parseFrrMessage(data)
	} else if m.Header.Version == 5 {
		err = m.parseFrrZapi5Message(data)
	} else if m.Header.Version == 6 {
		err = m.parseFrrZapi6Message(data)
	} else {
		err = m.parseMessage(data)
	}
	if err != nil {
		return nil, err
	}
	return m, nil
}
