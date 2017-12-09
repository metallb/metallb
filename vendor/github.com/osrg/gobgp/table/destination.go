// Copyright (C) 2014 Nippon Telegraph and Telephone Corporation.
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

package table

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"sort"

	"github.com/osrg/gobgp/config"
	"github.com/osrg/gobgp/packet/bgp"
	log "github.com/sirupsen/logrus"
)

var SelectionOptions config.RouteSelectionOptionsConfig
var UseMultiplePaths config.UseMultiplePathsConfig

type BestPathReason string

const (
	BPR_UNKNOWN            BestPathReason = "Unknown"
	BPR_ONLY_PATH          BestPathReason = "Only Path"
	BPR_REACHABLE_NEXT_HOP BestPathReason = "Reachable Next Hop"
	BPR_HIGHEST_WEIGHT     BestPathReason = "Highest Weight"
	BPR_LOCAL_PREF         BestPathReason = "Local Pref"
	BPR_LOCAL_ORIGIN       BestPathReason = "Local Origin"
	BPR_ASPATH             BestPathReason = "AS Path"
	BPR_ORIGIN             BestPathReason = "Origin"
	BPR_MED                BestPathReason = "MED"
	BPR_ASN                BestPathReason = "ASN"
	BPR_IGP_COST           BestPathReason = "IGP Cost"
	BPR_ROUTER_ID          BestPathReason = "Router ID"
	BPR_OLDER              BestPathReason = "Older"
	BPR_NON_LLGR_STALE     BestPathReason = "no LLGR Stale"
)

func IpToRadixkey(b []byte, max uint8) string {
	var buffer bytes.Buffer
	for i := 0; i < len(b) && i < int(max); i++ {
		fmt.Fprintf(&buffer, "%08b", b[i])
	}
	return buffer.String()[:max]
}

func CidrToRadixkey(cidr string) string {
	_, n, _ := net.ParseCIDR(cidr)
	ones, _ := n.Mask.Size()
	return IpToRadixkey(n.IP, uint8(ones))
}

func AddrToRadixkey(addr bgp.AddrPrefixInterface) string {
	var (
		ip   net.IP
		size uint8
	)
	switch T := addr.(type) {
	case *bgp.IPAddrPrefix:
		mask := net.CIDRMask(int(T.Length), net.IPv4len*8)
		ip, size = T.Prefix.Mask(mask).To4(), uint8(T.Length)
	case *bgp.IPv6AddrPrefix:
		mask := net.CIDRMask(int(T.Length), net.IPv6len*8)
		ip, size = T.Prefix.Mask(mask).To16(), uint8(T.Length)
	default:
		return CidrToRadixkey(addr.String())
	}
	return IpToRadixkey(ip, size)
}

type PeerInfo struct {
	AS                      uint32
	ID                      net.IP
	LocalAS                 uint32
	LocalID                 net.IP
	Address                 net.IP
	LocalAddress            net.IP
	RouteReflectorClient    bool
	RouteReflectorClusterID net.IP
	MultihopTtl             uint8
	Confederation           bool
}

func (lhs *PeerInfo) Equal(rhs *PeerInfo) bool {
	if lhs == rhs {
		return true
	}

	if rhs == nil {
		return false
	}

	if (lhs.AS == rhs.AS) && lhs.ID.Equal(rhs.ID) && lhs.LocalID.Equal(rhs.LocalID) && lhs.Address.Equal(rhs.Address) {
		return true
	}
	return false
}

func (i *PeerInfo) String() string {
	if i.Address == nil {
		return "local"
	}
	s := bytes.NewBuffer(make([]byte, 0, 64))
	s.WriteString(fmt.Sprintf("{ %s | ", i.Address))
	s.WriteString(fmt.Sprintf("as: %d", i.AS))
	s.WriteString(fmt.Sprintf(", id: %s", i.ID))
	if i.RouteReflectorClient {
		s.WriteString(fmt.Sprintf(", cluster-id: %s", i.RouteReflectorClusterID))
	}
	s.WriteString(" }")
	return s.String()
}

func NewPeerInfo(g *config.Global, p *config.Neighbor) *PeerInfo {
	id := net.ParseIP(string(p.RouteReflector.Config.RouteReflectorClusterId)).To4()
	// exclude zone info
	naddr, _ := net.ResolveIPAddr("ip", p.State.NeighborAddress)
	return &PeerInfo{
		AS:                      p.Config.PeerAs,
		LocalAS:                 g.Config.As,
		LocalID:                 net.ParseIP(g.Config.RouterId).To4(),
		RouteReflectorClient:    p.RouteReflector.Config.RouteReflectorClient,
		Address:                 naddr.IP,
		RouteReflectorClusterID: id,
		MultihopTtl:             p.EbgpMultihop.Config.MultihopTtl,
		Confederation:           p.IsConfederationMember(g),
	}
}

type Destination struct {
	routeFamily      bgp.RouteFamily
	nlri             bgp.AddrPrefixInterface
	knownPathList    paths
	withdrawList     paths
	newPathList      paths
	oldKnownPathList paths
	RadixKey         string
	localIdMap       *Bitmap
}

func NewDestination(nlri bgp.AddrPrefixInterface, mapSize int, known ...*Path) *Destination {
	d := &Destination{
		routeFamily:   bgp.AfiSafiToRouteFamily(nlri.AFI(), nlri.SAFI()),
		nlri:          nlri,
		knownPathList: known,
		withdrawList:  make([]*Path, 0),
		newPathList:   make([]*Path, 0),
		localIdMap:    NewBitmap(mapSize),
	}
	// the id zero means id is not allocated yet.
	if mapSize != 0 {
		d.localIdMap.Flag(0)
	}
	switch d.routeFamily {
	case bgp.RF_IPv4_UC, bgp.RF_IPv6_UC, bgp.RF_IPv4_MPLS, bgp.RF_IPv6_MPLS:
		d.RadixKey = AddrToRadixkey(nlri)
	}
	return d
}

func (dd *Destination) Family() bgp.RouteFamily {
	return dd.routeFamily
}

func (dd *Destination) setRouteFamily(routeFamily bgp.RouteFamily) {
	dd.routeFamily = routeFamily
}

func (dd *Destination) GetNlri() bgp.AddrPrefixInterface {
	return dd.nlri
}

func (dd *Destination) setNlri(nlri bgp.AddrPrefixInterface) {
	dd.nlri = nlri
}

func (dd *Destination) GetAllKnownPathList() []*Path {
	return dd.knownPathList
}

func (dd *Destination) GetKnownPathList(id string) []*Path {
	list := make([]*Path, 0, len(dd.knownPathList))
	for _, p := range dd.knownPathList {
		if p.Filtered(id) == POLICY_DIRECTION_NONE {
			list = append(list, p)
		}
	}
	return list
}

func getBestPath(id string, pathList *paths) *Path {
	for _, p := range *pathList {
		if p.Filtered(id) == POLICY_DIRECTION_NONE && !p.IsNexthopInvalid {
			return p
		}
	}
	return nil
}

func (dd *Destination) GetBestPath(id string) *Path {
	return getBestPath(id, &dd.knownPathList)
}

func getMultiBestPath(id string, pathList *paths) []*Path {
	list := make([]*Path, 0, len(*pathList))
	var best *Path
	for _, p := range *pathList {
		if p.Filtered(id) == POLICY_DIRECTION_NONE && !p.IsNexthopInvalid {
			if best == nil {
				best = p
				list = append(list, p)
			} else if best.Compare(p) == 0 {
				list = append(list, p)
			}
		}
	}
	return list
}

func (dd *Destination) GetMultiBestPath(id string) []*Path {
	return getMultiBestPath(id, &dd.knownPathList)
}

func (dd *Destination) GetAddPathChanges(id string) []*Path {
	l := make([]*Path, 0, len(dd.newPathList)+len(dd.withdrawList))
	for _, p := range dd.newPathList {
		l = append(l, p)
	}
	for _, p := range dd.withdrawList {
		l = append(l, p.Clone(true))
	}
	return l
}

func (dd *Destination) GetChanges(id string, peerDown bool) (*Path, *Path, []*Path) {
	best, old := func(id string) (*Path, *Path) {
		old := getBestPath(id, &dd.oldKnownPathList)
		best := dd.GetBestPath(id)
		if best != nil && best.Equal(old) {
			// RFC4684 3.2. Intra-AS VPN Route Distribution
			// When processing RT membership NLRIs received from internal iBGP
			// peers, it is necessary to consider all available iBGP paths for a
			// given RT prefix, for building the outbound route filter, and not just
			// the best path.
			if best.GetRouteFamily() == bgp.RF_RTC_UC {
				return best, old
			}
			// For BGP Nexthop Tracking, checks if the nexthop reachability
			// was changed or not.
			if best.IsNexthopInvalid != old.IsNexthopInvalid {
				return best, old
			}
			return nil, old
		}
		if best == nil {
			if old == nil {
				return nil, nil
			}
			if peerDown {
				// withdraws were generated by peer
				// down so paths are not in knowpath
				// or adjin.
				old.IsWithdraw = true
				return old, old
			}
			return old.Clone(true), old
		}
		return best, old
	}(id)

	var multi []*Path

	if id == GLOBAL_RIB_NAME && UseMultiplePaths.Enabled {
		diff := func(lhs, rhs []*Path) bool {
			if len(lhs) != len(rhs) {
				return true
			}
			for idx, l := range lhs {
				if !l.Equal(rhs[idx]) {
					return true
				}
			}
			return false
		}
		oldM := getMultiBestPath(id, &dd.oldKnownPathList)
		newM := dd.GetMultiBestPath(id)
		if diff(oldM, newM) {
			multi = newM
			if len(newM) == 0 {
				multi = []*Path{best}
			}
		}
	}
	return best, old, multi
}

func (dd *Destination) AddWithdraw(withdraw *Path) {
	dd.validatePath(withdraw)
	dd.withdrawList = append(dd.withdrawList, withdraw)
}

func (dd *Destination) AddNewPath(newPath *Path) {
	dd.validatePath(newPath)
	dd.newPathList = append(dd.newPathList, newPath)
}

func (dd *Destination) validatePath(path *Path) {
	if path == nil || path.GetRouteFamily() != dd.routeFamily {

		log.WithFields(log.Fields{
			"Topic":      "Table",
			"Key":        dd.GetNlri().String(),
			"Path":       path,
			"ExpectedRF": dd.routeFamily,
		}).Error("path is nil or invalid route family")
	}
}

// Calculates best-path among known paths for this destination.
//
// Modifies destination's state related to stored paths. Removes withdrawn
// paths from known paths. Also, adds new paths to known paths.
func (dest *Destination) Calculate() *Destination {
	oldKnownPathList := dest.knownPathList
	newPathList := dest.newPathList
	// First remove the withdrawn paths.
	withdrawn := dest.explicitWithdraw()
	// Do implicit withdrawal
	dest.implicitWithdraw()

	for _, path := range withdrawn {
		if id := path.GetNlri().PathLocalIdentifier(); id != 0 {
			dest.localIdMap.Unflag(uint(id))
		}
	}
	// Collect all new paths into known paths.
	dest.knownPathList = append(dest.knownPathList, dest.newPathList...)

	for _, path := range dest.knownPathList {
		if path.GetNlri().PathLocalIdentifier() == 0 {
			id, err := dest.localIdMap.FindandSetZeroBit()
			if err != nil {
				dest.localIdMap.Expand()
				id, _ = dest.localIdMap.FindandSetZeroBit()
			}
			path.GetNlri().SetPathLocalIdentifier(uint32(id))
		}
	}
	// Clear new paths as we copied them.
	dest.newPathList = make([]*Path, 0)
	// Compute new best path
	dest.computeKnownBestPath()

	return &Destination{
		routeFamily:      dest.routeFamily,
		nlri:             dest.nlri,
		knownPathList:    dest.knownPathList,
		oldKnownPathList: oldKnownPathList,
		newPathList:      newPathList,
		withdrawList:     withdrawn,
	}
}

// Removes withdrawn paths.
//
// Note:
// We may have disproportionate number of withdraws compared to know paths
// since not all paths get installed into the table due to bgp policy and
// we can receive withdraws for such paths and withdrawals may not be
// stopped by the same policies.
//
func (dest *Destination) explicitWithdraw() paths {

	// If we have no withdrawals, we have nothing to do.
	if len(dest.withdrawList) == 0 {
		return nil
	}

	log.WithFields(log.Fields{
		"Topic":  "Table",
		"Key":    dest.GetNlri().String(),
		"Length": len(dest.withdrawList),
	}).Debug("Removing withdrawals")

	// If we have some withdrawals and no know-paths, it means it is safe to
	// delete these withdraws.
	if len(dest.knownPathList) == 0 {
		log.WithFields(log.Fields{
			"Topic":  "Table",
			"Key":    dest.GetNlri().String(),
			"Length": len(dest.withdrawList),
		}).Debug("Found withdrawals for path(s) that did not get installed")
		dest.withdrawList = []*Path{}
		return nil
	}

	// If we have some known paths and some withdrawals, we find matches and
	// delete them first.
	matches := make([]*Path, 0, len(dest.withdrawList)/2)
	newKnownPaths := make([]*Path, 0, len(dest.knownPathList)/2)

	// Match all withdrawals from destination paths.
	for _, withdraw := range dest.withdrawList {
		isFound := false
		for _, path := range dest.knownPathList {
			// We have a match if the source and path-id are same.
			if path.GetSource().Equal(withdraw.GetSource()) && path.GetNlri().PathIdentifier() == withdraw.GetNlri().PathIdentifier() {
				isFound = true
				// this path is referenced in peer's adj-rib-in
				// when there was no policy modification applied.
				// we could flag IsWithdraw down after use to avoid
				// a path with IsWithdraw flag exists in adj-rib-in
				path.IsWithdraw = true
				withdraw.GetNlri().SetPathLocalIdentifier(path.GetNlri().PathLocalIdentifier())
				matches = append(matches, withdraw)
			}
		}

		// We do no have any match for this withdraw.
		if !isFound {
			log.WithFields(log.Fields{
				"Topic": "Table",
				"Key":   dest.GetNlri().String(),
				"Path":  withdraw,
			}).Warn("No matching path for withdraw found, may be path was not installed into table")
		}
	}

	for _, path := range dest.knownPathList {
		if !path.IsWithdraw {
			newKnownPaths = append(newKnownPaths, path)
		}
		// here we flag IsWithdraw down
		path.IsWithdraw = false
	}

	dest.knownPathList = newKnownPaths
	dest.withdrawList = make([]*Path, 0)
	return matches
}

// Identifies which of known paths are old and removes them.
//
// Known paths will no longer have paths whose new version is present in
// new paths.
func (dest *Destination) implicitWithdraw() paths {
	newKnownPaths := make([]*Path, 0, len(dest.knownPathList))
	implicitWithdrawn := make([]*Path, 0, len(dest.knownPathList))
	for _, path := range dest.knownPathList {
		found := false
		for _, newPath := range dest.newPathList {
			if newPath.NoImplicitWithdraw() {
				continue
			}
			// Here we just check if source is same and not check if path
			// version num. as newPaths are implicit withdrawal of old
			// paths and when doing RouteRefresh (not EnhancedRouteRefresh)
			// we get same paths again.
			if newPath.GetSource().Equal(path.GetSource()) && newPath.GetNlri().PathIdentifier() == path.GetNlri().PathIdentifier() {
				log.WithFields(log.Fields{
					"Topic": "Table",
					"Key":   dest.GetNlri().String(),
					"Path":  path,
				}).Debug("Implicit withdrawal of old path, since we have learned new path from the same peer")

				found = true
				newPath.GetNlri().SetPathLocalIdentifier(path.GetNlri().PathLocalIdentifier())
				break
			}
		}
		if found {
			implicitWithdrawn = append(implicitWithdrawn, path)
		} else {
			newKnownPaths = append(newKnownPaths, path)
		}
	}
	dest.knownPathList = newKnownPaths
	return implicitWithdrawn
}

func (dest *Destination) computeKnownBestPath() (*Path, BestPathReason, error) {

	// If we do not have any paths to this destination, then we do not have
	// new best path.
	if len(dest.knownPathList) == 0 {
		return nil, BPR_UNKNOWN, nil
	}

	log.WithFields(log.Fields{
		"Topic": "Table",
	}).Debugf("computeKnownBestPath knownPathList: %d", len(dest.knownPathList))

	// We pick the first path as current best path. This helps in breaking
	// tie between two new paths learned in one cycle for which best-path
	// calculation steps lead to tie.
	if len(dest.knownPathList) == 1 {
		// If the first path has the invalidated next-hop, which evaluated by
		// IGP, returns no path with the reason of the next-hop reachability.
		if dest.knownPathList[0].IsNexthopInvalid {
			return nil, BPR_REACHABLE_NEXT_HOP, nil
		}
		return dest.knownPathList[0], BPR_ONLY_PATH, nil
	}
	sort.Sort(dest.knownPathList)
	newBest := dest.knownPathList[0]
	// If the first path has the invalidated next-hop, which evaluated by IGP,
	// returns no path with the reason of the next-hop reachability.
	if dest.knownPathList[0].IsNexthopInvalid {
		return nil, BPR_REACHABLE_NEXT_HOP, nil
	}
	return newBest, newBest.reason, nil
}

type paths []*Path

func (p paths) Len() int {
	return len(p)
}

func (p paths) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

func (p paths) Less(i, j int) bool {

	//Compares given paths and returns best path.
	//
	//Parameters:
	//	-`path1`: first path to compare
	//	-`path2`: second path to compare
	//
	//	Best path processing will involve following steps:
	//	1.  Select a path with a reachable next hop.
	//	2.  Select the path with the highest weight.
	//	3.  If path weights are the same, select the path with the highest
	//	local preference value.
	//	4.  Prefer locally originated routes (network routes, redistributed
	//	routes, or aggregated routes) over received routes.
	//	5.  Select the route with the shortest AS-path length.
	//	6.  If all paths have the same AS-path length, select the path based
	//	on origin: IGP is preferred over EGP; EGP is preferred over
	//	Incomplete.
	//	7.  If the origins are the same, select the path with lowest MED
	//	value.
	//	8.  If the paths have the same MED values, select the path learned
	//	via EBGP over one learned via IBGP.
	//	9.  Select the route with the lowest IGP cost to the next hop.
	//	10. Select the route received from the peer with the lowest BGP
	//	router ID.
	//
	//	Returns None if best-path among given paths cannot be computed else best
	//	path.
	//	Assumes paths from NC has source equal to None.
	//

	path1 := p[i]
	path2 := p[j]

	var better *Path
	reason := BPR_UNKNOWN

	// draft-uttaro-idr-bgp-persistence-02
	if better == nil {
		better = compareByLLGRStaleCommunity(path1, path2)
		reason = BPR_NON_LLGR_STALE
	}
	// Follow best path calculation algorithm steps.
	// compare by reachability
	if better == nil {
		better = compareByReachableNexthop(path1, path2)
		reason = BPR_REACHABLE_NEXT_HOP
	}
	if better == nil {
		better = compareByHighestWeight(path1, path2)
		reason = BPR_HIGHEST_WEIGHT
	}
	if better == nil {
		better = compareByLocalPref(path1, path2)
		reason = BPR_LOCAL_PREF
	}
	if better == nil {
		better = compareByLocalOrigin(path1, path2)
		reason = BPR_LOCAL_ORIGIN
	}
	if better == nil {
		better = compareByASPath(path1, path2)
		reason = BPR_ASPATH
	}
	if better == nil {
		better = compareByOrigin(path1, path2)
		reason = BPR_ORIGIN
	}
	if better == nil {
		better = compareByMED(path1, path2)
		reason = BPR_MED
	}
	if better == nil {
		better = compareByASNumber(path1, path2)
		reason = BPR_ASN
	}
	if better == nil {
		better = compareByIGPCost(path1, path2)
		reason = BPR_IGP_COST
	}
	if better == nil {
		better = compareByAge(path1, path2)
		reason = BPR_OLDER
	}
	if better == nil {
		var e error = nil
		better, e = compareByRouterID(path1, path2)
		if e != nil {
			log.WithFields(log.Fields{
				"Topic": "Table",
				"Error": e,
			}).Error("Could not get best path by comparing router ID")
		}
		reason = BPR_ROUTER_ID
	}
	if better == nil {
		reason = BPR_UNKNOWN
		better = path1
	}

	better.reason = reason

	if better == path1 {
		return true
	}
	return false
}

func compareByLLGRStaleCommunity(path1, path2 *Path) *Path {
	p1 := path1.IsLLGRStale()
	p2 := path2.IsLLGRStale()
	if p1 == p2 {
		return nil
	} else if p1 {
		return path2
	}
	return path1
}

func compareByReachableNexthop(path1, path2 *Path) *Path {
	//	Compares given paths and selects best path based on reachable next-hop.
	//
	//	If no path matches this criteria, return nil.
	//	For BGP Nexthop Tracking, evaluates next-hop is validated by IGP.
	log.WithFields(log.Fields{
		"Topic": "Table",
	}).Debugf("enter compareByReachableNexthop -- path1: %s, path2: %s", path1, path2)

	if path1.IsNexthopInvalid && !path2.IsNexthopInvalid {
		return path2
	} else if !path1.IsNexthopInvalid && path2.IsNexthopInvalid {
		return path1
	}

	return nil
}

func compareByHighestWeight(path1, path2 *Path) *Path {
	//	Selects a path with highest weight.
	//
	//	Weight is BGPS specific parameter. It is local to the router on which it
	//	is configured.
	//	Return:
	//	nil if best path among given paths cannot be decided, else best path.
	log.WithFields(log.Fields{
		"Topic": "Table",
	}).Debugf("enter compareByHighestWeight -- path1: %s, path2: %s", path1, path2)
	return nil
}

func compareByLocalPref(path1, path2 *Path) *Path {
	//	Selects a path with highest local-preference.
	//
	//	Unlike the weight attribute, which is only relevant to the local
	//	router, local preference is an attribute that routers exchange in the
	//	same AS. Highest local-pref is preferred. If we cannot decide,
	//	we return None.
	//
	//	# Default local-pref values is 100
	log.WithFields(log.Fields{
		"Topic": "Table",
	}).Debug("enter compareByLocalPref")
	localPref1, _ := path1.GetLocalPref()
	localPref2, _ := path2.GetLocalPref()
	// Highest local-preference value is preferred.
	if localPref1 > localPref2 {
		return path1
	} else if localPref1 < localPref2 {
		return path2
	} else {
		return nil
	}
}

func compareByLocalOrigin(path1, path2 *Path) *Path {

	// Select locally originating path as best path.
	// Locally originating routes are network routes, redistributed routes,
	// or aggregated routes.
	// Returns None if given paths have same source.
	//
	// If both paths are from same sources we cannot compare them here.
	log.WithFields(log.Fields{
		"Topic": "Table",
	}).Debug("enter compareByLocalOrigin")
	if path1.GetSource().Equal(path2.GetSource()) {
		return nil
	}

	// Here we consider prefix from NC as locally originating static route.
	// Hence it is preferred.
	if path1.IsLocal() {
		return path1
	}

	if path2.IsLocal() {
		return path2
	}
	return nil
}

func compareByASPath(path1, path2 *Path) *Path {
	// Calculated the best-paths by comparing as-path lengths.
	//
	// Shortest as-path length is preferred. If both path have same lengths,
	// we return None.
	if SelectionOptions.IgnoreAsPathLength {
		log.WithFields(log.Fields{
			"Topic": "Table",
		}).Debug("compareByASPath -- skip")
		return nil
	}
	log.WithFields(log.Fields{
		"Topic": "Table",
	}).Debug("enter compareByASPath")
	attribute1 := path1.getPathAttr(bgp.BGP_ATTR_TYPE_AS_PATH)
	attribute2 := path2.getPathAttr(bgp.BGP_ATTR_TYPE_AS_PATH)

	// With addpath support, we could compare paths from API don't
	// AS_PATH. No need to warn here.
	if !path1.IsLocal() && !path2.IsLocal() && (attribute1 == nil || attribute2 == nil) {
		log.WithFields(log.Fields{
			"Topic":   "Table",
			"Key":     "compareByASPath",
			"ASPath1": attribute1,
			"ASPath2": attribute2,
		}).Warn("can't compare ASPath because it's not present")
	}

	l1 := path1.GetAsPathLen()
	l2 := path2.GetAsPathLen()

	log.WithFields(log.Fields{
		"Topic": "Table",
	}).Debugf("compareByASPath -- l1: %d, l2: %d", l1, l2)
	if l1 > l2 {
		return path2
	} else if l1 < l2 {
		return path1
	} else {
		return nil
	}
}

func compareByOrigin(path1, path2 *Path) *Path {
	//	Select the best path based on origin attribute.
	//
	//	IGP is preferred over EGP; EGP is preferred over Incomplete.
	//	If both paths have same origin, we return None.
	log.WithFields(log.Fields{
		"Topic": "Table",
	}).Debug("enter compareByOrigin")
	attribute1 := path1.getPathAttr(bgp.BGP_ATTR_TYPE_ORIGIN)
	attribute2 := path2.getPathAttr(bgp.BGP_ATTR_TYPE_ORIGIN)

	if attribute1 == nil || attribute2 == nil {
		log.WithFields(log.Fields{
			"Topic":   "Table",
			"Key":     "compareByOrigin",
			"Origin1": attribute1,
			"Origin2": attribute2,
		}).Error("can't compare origin because it's not present")
		return nil
	}

	origin1, n1 := binary.Uvarint(attribute1.(*bgp.PathAttributeOrigin).Value)
	origin2, n2 := binary.Uvarint(attribute2.(*bgp.PathAttributeOrigin).Value)
	log.WithFields(log.Fields{
		"Topic": "Table",
	}).Debugf("compareByOrigin -- origin1: %d(%d), origin2: %d(%d)", origin1, n1, origin2, n2)

	// If both paths have same origins
	if origin1 == origin2 {
		return nil
	} else if origin1 < origin2 {
		return path1
	} else {
		return path2
	}
}

func compareByMED(path1, path2 *Path) *Path {
	//	Select the path based with lowest MED value.
	//
	//	If both paths have same MED, return None.
	//	By default, a route that arrives with no MED value is treated as if it
	//	had a MED of 0, the most preferred value.
	//	RFC says lower MED is preferred over higher MED value.
	//  compare MED among not only same AS path but also all path,
	//  like bgp always-compare-med

	isInternal := func() bool { return path1.GetAsPathLen() == 0 && path2.GetAsPathLen() == 0 }()

	isSameAS := func() bool {
		firstAS := func(path *Path) uint32 {
			if aspath := path.GetAsPath(); aspath != nil {
				asPathParam := aspath.Value
				for i := 0; i < len(asPathParam); i++ {
					asPath := asPathParam[i].(*bgp.As4PathParam)
					if asPath.Num == 0 {
						continue
					}
					if asPath.Type == bgp.BGP_ASPATH_ATTR_TYPE_CONFED_SET || asPath.Type == bgp.BGP_ASPATH_ATTR_TYPE_CONFED_SEQ {
						continue
					}
					return asPath.AS[0]
				}
			}
			return 0
		}
		return firstAS(path1) != 0 && firstAS(path1) == firstAS(path2)
	}()

	if SelectionOptions.AlwaysCompareMed || isInternal || isSameAS {
		log.WithFields(log.Fields{
			"Topic": "Table",
		}).Debug("enter compareByMED")
		getMed := func(path *Path) uint32 {
			attribute := path.getPathAttr(bgp.BGP_ATTR_TYPE_MULTI_EXIT_DISC)
			if attribute == nil {
				return 0
			}
			med := attribute.(*bgp.PathAttributeMultiExitDisc).Value
			return med
		}

		med1 := getMed(path1)
		med2 := getMed(path2)
		log.WithFields(log.Fields{
			"Topic": "Table",
		}).Debugf("compareByMED -- med1: %d, med2: %d", med1, med2)
		if med1 == med2 {
			return nil
		} else if med1 < med2 {
			return path1
		}
		return path2
	} else {
		log.WithFields(log.Fields{
			"Topic": "Table",
		}).Debugf("skip compareByMED %v %v %v", SelectionOptions.AlwaysCompareMed, isInternal, isSameAS)
		return nil
	}
}

func compareByASNumber(path1, path2 *Path) *Path {

	//Select the path based on source (iBGP/eBGP) peer.
	//
	//eBGP path is preferred over iBGP. If both paths are from same kind of
	//peers, return None.
	log.WithFields(log.Fields{
		"Topic": "Table",
	}).Debug("enter compareByASNumber")

	log.WithFields(log.Fields{
		"Topic": "Table",
	}).Debugf("compareByASNumber -- p1Asn: %d, p2Asn: %d", path1.GetSource().AS, path2.GetSource().AS)
	// Path from confederation member should be treated as internal (IBGP learned) path.
	isIBGP1 := path1.GetSource().Confederation || path1.IsIBGP()
	isIBGP2 := path2.GetSource().Confederation || path2.IsIBGP()
	// If one path is from ibgp peer and another is from ebgp peer, take the ebgp path.
	if isIBGP1 != isIBGP2 {
		if isIBGP1 {
			return path2
		}
		return path1
	}

	// If both paths are from ebgp or ibpg peers, we cannot decide.
	return nil
}

func compareByIGPCost(path1, path2 *Path) *Path {
	//	Select the route with the lowest IGP cost to the next hop.
	//
	//	Return None if igp cost is same.
	// Currently BGPS has no concept of IGP and IGP cost.
	log.WithFields(log.Fields{
		"Topic": "Table",
	}).Debugf("enter compareByIGPCost -- path1: %v, path2: %v", path1, path2)
	return nil
}

func compareByRouterID(path1, path2 *Path) (*Path, error) {
	//	Select the route received from the peer with the lowest BGP router ID.
	//
	//	If both paths are eBGP paths, then we do not do any tie breaking, i.e we do
	//	not pick best-path based on this criteria.
	//	RFC: http://tools.ietf.org/html/rfc5004
	//	We pick best path between two iBGP paths as usual.
	log.WithFields(log.Fields{
		"Topic": "Table",
	}).Debug("enter compareByRouterID")

	// If both paths are from NC we have same router Id, hence cannot compare.
	if path1.IsLocal() && path2.IsLocal() {
		return nil, nil
	}

	// If both paths are from eBGP peers, then according to RFC we need
	// not tie break using router id.
	if !SelectionOptions.ExternalCompareRouterId && !path1.IsIBGP() && !path2.IsIBGP() {
		return nil, nil
	}

	if !SelectionOptions.ExternalCompareRouterId && path1.IsIBGP() != path2.IsIBGP() {
		return nil, fmt.Errorf("This method does not support comparing ebgp with ibgp path")
	}

	// At least one path is not coming from NC, so we get local bgp id.
	id1 := binary.BigEndian.Uint32(path1.GetSource().ID)
	id2 := binary.BigEndian.Uint32(path2.GetSource().ID)

	// If both router ids are same/equal we cannot decide.
	// This case is possible since router ids are arbitrary.
	if id1 == id2 {
		return nil, nil
	} else if id1 < id2 {
		return path1, nil
	} else {
		return path2, nil
	}
}

func compareByAge(path1, path2 *Path) *Path {
	if !path1.IsIBGP() && !path2.IsIBGP() && !SelectionOptions.ExternalCompareRouterId {
		age1 := path1.GetTimestamp().UnixNano()
		age2 := path2.GetTimestamp().UnixNano()
		if age1 == age2 {
			return nil
		} else if age1 < age2 {
			return path1
		}
		return path2
	}
	return nil
}

func (dest *Destination) String() string {
	return fmt.Sprintf("Destination NLRI: %s", dest.nlri.String())
}

type DestinationSelectOption struct {
	ID        string
	VRF       *Vrf
	adj       bool
	Best      bool
	MultiPath bool
}

func (d *Destination) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.GetAllKnownPathList())
}

func (old *Destination) Select(option ...DestinationSelectOption) *Destination {
	id := GLOBAL_RIB_NAME
	var vrf *Vrf
	adj := false
	best := false
	mp := false
	for _, o := range option {
		if o.ID != "" {
			id = o.ID
		}
		if o.VRF != nil {
			vrf = o.VRF
		}
		adj = o.adj
		best = o.Best
		mp = o.MultiPath
	}
	var paths []*Path
	if adj {
		paths = old.knownPathList
	} else {
		paths = old.GetKnownPathList(id)
		if vrf != nil {
			ps := make([]*Path, 0, len(paths))
			for _, p := range paths {
				if CanImportToVrf(vrf, p) {
					ps = append(ps, p.ToLocal())
				}
			}
			paths = ps
		}
		if len(paths) == 0 {
			return nil
		}
		if best {
			if !mp {
				paths = []*Path{paths[0]}
			} else {
				ps := make([]*Path, 0, len(paths))
				var best *Path
				for _, p := range paths {
					if best == nil {
						best = p
						ps = append(ps, p)
					} else if best.Compare(p) == 0 {
						ps = append(ps, p)
					}
				}
				paths = ps
			}
		}
	}
	new := NewDestination(old.nlri, 0)
	for _, path := range paths {
		p := path.Clone(path.IsWithdraw)
		p.Filter("", path.Filtered(id))
		new.knownPathList = append(new.knownPathList, p)
	}
	return new
}

type destinations []*Destination

func (d destinations) Len() int {
	return len(d)
}

func (d destinations) Swap(i, j int) {
	d[i], d[j] = d[j], d[i]
}

func (d destinations) Less(i, j int) bool {
	switch d[i].routeFamily {
	case bgp.RF_FS_IPv4_UC, bgp.RF_FS_IPv6_UC, bgp.RF_FS_IPv4_VPN, bgp.RF_FS_IPv6_VPN, bgp.RF_FS_L2_VPN:
		var s, t *bgp.FlowSpecNLRI
		switch d[i].routeFamily {
		case bgp.RF_FS_IPv4_UC:
			s = &d[i].nlri.(*bgp.FlowSpecIPv4Unicast).FlowSpecNLRI
			t = &d[j].nlri.(*bgp.FlowSpecIPv4Unicast).FlowSpecNLRI
		case bgp.RF_FS_IPv6_UC:
			s = &d[i].nlri.(*bgp.FlowSpecIPv6Unicast).FlowSpecNLRI
			t = &d[j].nlri.(*bgp.FlowSpecIPv6Unicast).FlowSpecNLRI
		case bgp.RF_FS_IPv4_VPN:
			s = &d[i].nlri.(*bgp.FlowSpecIPv4VPN).FlowSpecNLRI
			t = &d[j].nlri.(*bgp.FlowSpecIPv4VPN).FlowSpecNLRI
		case bgp.RF_FS_IPv6_VPN:
			s = &d[i].nlri.(*bgp.FlowSpecIPv6VPN).FlowSpecNLRI
			t = &d[j].nlri.(*bgp.FlowSpecIPv6VPN).FlowSpecNLRI
		case bgp.RF_FS_L2_VPN:
			s = &d[i].nlri.(*bgp.FlowSpecL2VPN).FlowSpecNLRI
			t = &d[j].nlri.(*bgp.FlowSpecL2VPN).FlowSpecNLRI
		}
		if r, _ := bgp.CompareFlowSpecNLRI(s, t); r >= 0 {
			return true
		} else {
			return false
		}
	default:
		strings := sort.StringSlice{d[i].nlri.String(), d[j].nlri.String()}
		return strings.Less(0, 1)
	}
}
