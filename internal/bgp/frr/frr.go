// SPDX-License-Identifier:Apache-2.0

package frr

import (
	"fmt"
	"net"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/go-kit/kit/log"
	"go.universe.tf/metallb/internal/bgp"
	metallbconfig "go.universe.tf/metallb/internal/config"
	"go.universe.tf/metallb/internal/ipfamily"
)

// As the MetalLB controller should handle messages synchronously, there should
// no need to lock this data structure. TODO: confirm this.

type sessionManager struct {
	sessions     map[string]*session
	bfdProfiles  []BFDProfile
	reloadConfig chan *frrConfig
}

type session struct {
	myASN          uint32
	routerID       net.IP // May be nil, meaning "derive from context"
	myNode         string
	addr           string
	srcAddr        net.IP
	asn            uint32
	holdTime       time.Duration
	keepaliveTime  time.Duration
	logger         log.Logger
	password       string
	advertised     []*bgp.Advertisement
	bfdProfile     string
	ebgpMultiHop   bool
	sessionManager *sessionManager
}

// Create a variable for os.Hostname() in order to make it easy to mock out
// in unit tests.
var osHostname = os.Hostname

// sessionName() defines the format of the key of the 'sessions' map in
// the 'frrState' struct.
func sessionName(myAddr string, myAsn uint32, addr string, asn uint32) string {
	return fmt.Sprintf("%d@%s-%d@%s", asn, addr, myAsn, myAddr)
}

func validate(adv *bgp.Advertisement) error {
	if adv.Prefix.IP.To4() != nil {
		if adv.NextHop != nil && adv.NextHop.To4() == nil {
			return fmt.Errorf("next-hop must be IPv4, got %q", adv.NextHop)
		}
	} else if adv.Prefix.IP.To16() != nil {
		if adv.NextHop != nil && adv.NextHop.To16() == nil {
			return fmt.Errorf("next-hop must be IPv6, got %q", adv.NextHop)
		}
	} else {
		return fmt.Errorf("unable to validate IP address")
	}

	if len(adv.Communities) > 63 {
		return fmt.Errorf("max supported communities is 63, got %d", len(adv.Communities))
	}
	return nil
}

func (s *session) Set(advs ...*bgp.Advertisement) error {
	sessionName := sessionName(s.srcAddr.String(), s.myASN, s.addr, s.asn)
	if _, found := s.sessionManager.sessions[sessionName]; !found {
		return fmt.Errorf("session not established before advertisement")
	}

	newAdvs := []*bgp.Advertisement{}
	for _, adv := range advs {
		err := validate(adv)
		if err != nil {
			return err
		}
		newAdvs = append(newAdvs, adv)
	}
	oldAdvs := s.advertised
	s.advertised = newAdvs

	// Attempt to create a config
	config, err := s.sessionManager.createConfig()
	if err != nil {
		s.advertised = oldAdvs
		return err
	}

	s.sessionManager.reloadConfig <- config
	return nil
}

// Close() shuts down the BGP session.
func (s *session) Close() error {
	err := s.sessionManager.deleteSession(s)
	if err != nil {
		return err
	}

	frrConfig, err := s.sessionManager.createConfig()
	if err != nil {
		return err
	}

	s.sessionManager.reloadConfig <- frrConfig
	return nil
}

// NewSession() creates a BGP session using the given session parameters.
//
// The session will immediately try to connect and synchronize its
// local state with the peer.
func (sm *sessionManager) NewSession(l log.Logger, addr string, srcAddr net.IP, myASN uint32, routerID net.IP, asn uint32, holdTime, keepaliveTime time.Duration, password, myNode, bfdProfile string, ebgpMultiHop bool) (bgp.Session, error) {
	s := &session{
		myASN:          myASN,
		routerID:       routerID,
		myNode:         myNode,
		addr:           addr,
		srcAddr:        srcAddr,
		asn:            asn,
		holdTime:       holdTime,
		keepaliveTime:  keepaliveTime,
		logger:         log.With(l, "peer", addr, "localASN", myASN, "peerASN", asn),
		password:       password,
		advertised:     []*bgp.Advertisement{},
		sessionManager: sm,
		bfdProfile:     bfdProfile,
		ebgpMultiHop:   ebgpMultiHop,
	}

	_ = sm.addSession(s)

	frrConfig, err := sm.createConfig()
	if err != nil {
		_ = sm.deleteSession(s)
		return nil, err
	}

	sm.reloadConfig <- frrConfig
	return s, nil
}

func (sm *sessionManager) addSession(s *session) error {
	if s == nil {
		return fmt.Errorf("invalid session")
	}
	sessionName := sessionName(s.srcAddr.String(), s.myASN, s.addr, s.asn)
	sm.sessions[sessionName] = s

	return nil
}

func (sm *sessionManager) deleteSession(s *session) error {
	if s == nil {
		return fmt.Errorf("invalid session")
	}
	sessionName := sessionName(s.srcAddr.String(), s.myASN, s.addr, s.asn)
	delete(sm.sessions, sessionName)

	return nil
}

func (sm *sessionManager) SyncBFDProfiles(profiles map[string]*metallbconfig.BFDProfile) error {
	sm.bfdProfiles = make([]BFDProfile, 0)
	for _, p := range profiles {
		frrProfile := configBFDProfileToFRR(p)
		sm.bfdProfiles = append(sm.bfdProfiles, *frrProfile)
	}
	sort.Slice(sm.bfdProfiles, func(i, j int) bool {
		return sm.bfdProfiles[i].Name < sm.bfdProfiles[j].Name
	})

	frrConfig, err := sm.createConfig()
	if err != nil {
		return err
	}

	sm.reloadConfig <- frrConfig
	return nil
}

func (sm *sessionManager) createConfig() (*frrConfig, error) {
	hostname, err := osHostname()
	if err != nil {
		return nil, err
	}

	config := &frrConfig{
		Hostname:               hostname,
		Loglevel:               "informational",
		Routers:                make(map[string]*routerConfig),
		BFDProfiles:            sm.bfdProfiles,
		PrefixesV4ForCommunity: make([]communityPrefixes, 0),
		PrefixesV6ForCommunity: make([]communityPrefixes, 0),
		PrefixesV4ForLocalPref: make([]localPrefPrefixes, 0),
		PrefixesV6ForLocalPref: make([]localPrefPrefixes, 0),
	}
	frrLogLevel, found := os.LookupEnv("FRR_LOGGING_LEVEL")
	if found {
		config.Loglevel = frrLogLevel
	}

	prefixesV4ForCommunity := map[string]stringSet{}
	prefixesV6ForCommunity := map[string]stringSet{}
	prefixesV4ForLocalPref := map[uint32]stringSet{}
	prefixesV6ForLocalPref := map[uint32]stringSet{}

	for _, s := range sm.sessions {
		var router *routerConfig
		var neighbor *neighborConfig
		var exist bool

		routerName := routerName(s.routerID.String(), s.myASN)
		if router, exist = config.Routers[routerName]; !exist {
			router = &routerConfig{
				MyASN:     s.myASN,
				Neighbors: make(map[string]*neighborConfig),
			}
			if s.routerID != nil {
				router.RouterId = s.routerID.String()
			}
			config.Routers[routerName] = router
		}

		neighborName := neighborName(s.addr, s.asn)
		if neighbor, exist = router.Neighbors[neighborName]; !exist {
			host, port, err := net.SplitHostPort(s.addr)
			if err != nil {
				return nil, err
			}

			portUint, err := strconv.ParseUint(port, 10, 16)
			if err != nil {
				return nil, err
			}

			neighbor = &neighborConfig{
				ASN:            s.asn,
				Addr:           host,
				Port:           uint16(portUint),
				HoldTime:       uint64(s.holdTime / time.Second),
				KeepaliveTime:  uint64(s.keepaliveTime / time.Second),
				Password:       s.password,
				Advertisements: make([]*advertisementConfig, 0),
				BFDProfile:     s.bfdProfile,
				EBGPMultiHop:   s.ebgpMultiHop,
			}
			if s.srcAddr != nil {
				neighbor.SrcAddr = s.srcAddr.String()
			}
			router.Neighbors[neighborName] = neighbor
		}

		/* As 'session.advertised' is a map, we can be sure there are no
		   duplicate prefixes and can, therefore, just add them to the
		   'neighbor.Advertisements' list. */
		for _, adv := range s.advertised {
			family := ipfamily.ForAddress(adv.Prefix.IP)
			communities := make([]string, 0)

			// Convert community 32bits value to : format
			for _, c := range adv.Communities {
				community := metallbconfig.CommunityToString(c)
				communities = append(communities, community)
				addPrefixForCommunity(prefixesV4ForCommunity, prefixesV6ForCommunity, adv.Prefix.String(), family, community)
			}
			advConfig := advertisementConfig{
				IPFamily:    family,
				Prefix:      adv.Prefix.String(),
				Communities: communities,
				LocalPref:   adv.LocalPref,
			}
			if adv.LocalPref != 0 {
				addPrefixForLocalPref(prefixesV4ForLocalPref, prefixesV6ForLocalPref, adv.Prefix.String(), family, adv.LocalPref)
			}

			neighbor.Advertisements = append(neighbor.Advertisements, &advConfig)
		}
	}

	// The maps must be converted in sorted slice to make the rendering stable
	config.PrefixesV4ForCommunity = mapToCommunityPrefixes(prefixesV4ForCommunity)
	config.PrefixesV6ForCommunity = mapToCommunityPrefixes(prefixesV6ForCommunity)
	config.PrefixesV4ForLocalPref = mapToLocalPrefPrefixes(prefixesV4ForLocalPref)
	config.PrefixesV6ForLocalPref = mapToLocalPrefPrefixes(prefixesV6ForLocalPref)

	return config, nil
}

func addPrefixForLocalPref(prefixesV4, prefixesV6 map[uint32]stringSet, prefix string, family ipfamily.Family, localPref uint32) {
	if family == ipfamily.IPv4 {
		_, ok := prefixesV4[localPref]
		if !ok {
			prefixesV4[localPref] = newStringSet()
		}
		prefixesV4[localPref].Add(prefix)
		return
	}
	_, ok := prefixesV6[localPref]
	if !ok {
		prefixesV6[localPref] = newStringSet()
	}
	prefixesV6[localPref].Add(prefix)
}

func addPrefixForCommunity(prefixesV4, prefixesV6 map[string]stringSet, prefix string, family ipfamily.Family, community string) {
	if family == ipfamily.IPv4 {
		_, ok := prefixesV4[community]
		if !ok {
			prefixesV4[community] = newStringSet()
		}
		prefixesV4[community].Add(prefix)
		return

	}
	_, ok := prefixesV6[community]
	if !ok {
		prefixesV6[community] = newStringSet()
	}
	prefixesV6[community].Add(prefix)
}

func mapToLocalPrefPrefixes(m map[uint32]stringSet) []localPrefPrefixes {
	res := make([]localPrefPrefixes, 0)

	for k, v := range m {
		res = append(res, localPrefPrefixes{LocalPreference: k, Prefixes: v.Elements()})
	}
	sort.Slice(res, func(i int, j int) bool {
		return res[i].LocalPreference < res[j].LocalPreference
	})
	return res
}

func mapToCommunityPrefixes(m map[string]stringSet) []communityPrefixes {
	res := make([]communityPrefixes, 0)
	for k, v := range m {
		res = append(res, communityPrefixes{Community: k, Prefixes: v.Elements()})
	}
	sort.Slice(res, func(i int, j int) bool {
		return res[i].Community < res[j].Community
	})
	return res
}

var debounceTimeout = 500 * time.Millisecond

func NewSessionManager(l log.Logger) *sessionManager {
	res := &sessionManager{
		sessions:     map[string]*session{},
		bfdProfiles:  []BFDProfile{},
		reloadConfig: make(chan *frrConfig),
	}
	reload := func(config *frrConfig) {
		generateAndReloadConfigFile(config, l)
	}
	debouncer(reload, res.reloadConfig, debounceTimeout)

	return res
}

func configBFDProfileToFRR(p *metallbconfig.BFDProfile) *BFDProfile {
	res := &BFDProfile{}
	res.Name = p.Name
	res.ReceiveInterval = p.ReceiveInterval
	res.TransmitInterval = p.TransmitInterval
	res.DetectMultiplier = p.DetectMultiplier
	res.EchoInterval = p.EchoInterval
	res.EchoMode = p.EchoMode
	res.PassiveMode = p.PassiveMode
	res.MinimumTTL = p.MinimumTTL
	return res
}
