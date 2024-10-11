// SPDX-License-Identifier:Apache-2.0

package frr

import (
	"fmt"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"go.universe.tf/metallb/internal/bgp"
	"go.universe.tf/metallb/internal/bgp/community"
	metallbconfig "go.universe.tf/metallb/internal/config"
	"go.universe.tf/metallb/internal/ipfamily"
	"go.universe.tf/metallb/internal/logging"
	"k8s.io/apimachinery/pkg/util/sets"
)

// As the MetalLB controller should handle messages synchronously, there should
// no need to lock this data structure. TODO: confirm this.

type sessionManager struct {
	sessions     map[string]*session
	bfdProfiles  []BFDProfile
	extraConfig  string
	reloadConfig chan reloadEvent
	logLevel     string
	sync.Mutex
}

type session struct {
	bgp.SessionParameters
	sessionManager *sessionManager
	advertised     []*bgp.Advertisement
}

// Create a variable for os.Hostname() in order to make it easy to mock out
// in unit tests.
var osHostname = os.Hostname

// sessionName() defines the format of the key of the 'sessions' map in
// the 'frrState' struct.
func sessionName(s session) string {
	baseName := fmt.Sprintf("%d@%s-%d@%s", s.PeerASN, s.PeerAddress, s.MyASN, s.SourceAddress)
	if s.VRFName == "" {
		return baseName
	}
	return baseName + "/" + s.VRFName
}

func validate(adv *bgp.Advertisement) error {
	if len(adv.Communities) > 63 {
		return fmt.Errorf("max supported communities is 63, got %d", len(adv.Communities))
	}
	return nil
}

func (s *session) Set(advs ...*bgp.Advertisement) error {
	s.sessionManager.Lock()
	defer s.sessionManager.Unlock()
	sessionName := sessionName(*s)
	if _, found := s.sessionManager.sessions[sessionName]; !found {
		return fmt.Errorf("session %s not established before advertisement", sessionName)
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

	s.sessionManager.reloadConfig <- reloadEvent{config: config}
	return nil
}

// Close() shuts down the BGP session.
func (s *session) Close() error {
	s.sessionManager.Lock()
	defer s.sessionManager.Unlock()
	err := s.sessionManager.deleteSession(s)
	if err != nil {
		return err
	}

	frrConfig, err := s.sessionManager.createConfig()
	if err != nil {
		return err
	}

	s.sessionManager.reloadConfig <- reloadEvent{config: frrConfig}
	return nil
}

// NewSession() creates a BGP session using the given session parameters.
//
// The session will immediately try to connect and synchronize its
// local state with the peer.
func (sm *sessionManager) NewSession(l log.Logger, args bgp.SessionParameters) (bgp.Session, error) {
	sm.Lock()
	defer sm.Unlock()
	s := &session{
		advertised:        []*bgp.Advertisement{},
		sessionManager:    sm,
		SessionParameters: args,
	}

	_ = sm.addSession(s)

	frrConfig, err := sm.createConfig()
	if err != nil {
		_ = sm.deleteSession(s)
		return nil, err
	}

	sm.reloadConfig <- reloadEvent{config: frrConfig}
	return s, nil
}

func (sm *sessionManager) addSession(s *session) error {
	if s == nil {
		return fmt.Errorf("invalid session")
	}
	sessionName := sessionName(*s)
	sm.sessions[sessionName] = s

	return nil
}

func (sm *sessionManager) deleteSession(s *session) error {
	if s == nil {
		return fmt.Errorf("invalid session")
	}
	sessionName := sessionName(*s)
	delete(sm.sessions, sessionName)

	return nil
}

func (sm *sessionManager) SyncExtraInfo(extraInfo string) error {
	sm.Lock()
	defer sm.Unlock()
	sm.extraConfig = extraInfo
	frrConfig, err := sm.createConfig()
	if err != nil {
		return err
	}

	sm.reloadConfig <- reloadEvent{config: frrConfig}
	return nil
}

func (sm *sessionManager) SyncBFDProfiles(profiles map[string]*metallbconfig.BFDProfile) error {
	sm.Lock()
	defer sm.Unlock()
	sm.bfdProfiles = make([]BFDProfile, 0)
	for _, p := range profiles {
		frrProfile := ConfigBFDProfileToFRR(p)
		sm.bfdProfiles = append(sm.bfdProfiles, *frrProfile)
	}
	sort.Slice(sm.bfdProfiles, func(i, j int) bool {
		return sm.bfdProfiles[i].Name < sm.bfdProfiles[j].Name
	})

	frrConfig, err := sm.createConfig()
	if err != nil {
		return err
	}

	sm.reloadConfig <- reloadEvent{config: frrConfig}
	return nil
}

func (sm *sessionManager) createConfig() (*frrConfig, error) {
	hostname, err := osHostname()
	if err != nil {
		return nil, err
	}

	config := &frrConfig{
		Hostname:    hostname,
		Loglevel:    sm.logLevel,
		BFDProfiles: sm.bfdProfiles,
		ExtraConfig: sm.extraConfig,
	}

	type neighborProperties struct {
		// It has at least one advertisement with these communities
		CommunitiesV4 sets.Set[string]
		CommunitiesV6 sets.Set[string]
		// It has at least one advertisement with these large communities
		LargeCommunitiesV4 sets.Set[string]
		LargeCommunitiesV6 sets.Set[string]
		// It has at least one advertisement with these local preferences
		LocalPrefsV4 sets.Set[uint32]
		LocalPrefsV6 sets.Set[uint32]
	}

	type router struct {
		myASN               uint32
		routerID            string
		neighbors           map[string]*neighborConfig
		neighborsProperties map[string]*neighborProperties
		vrf                 string
		ipV4Prefixes        map[string]string
		ipV6Prefixes        map[string]string
	}

	routers := make(map[string]*router)

	// leave it for backward compatibility
	frrLogLevel, found := os.LookupEnv("FRR_LOGGING_LEVEL")
	if found {
		config.Loglevel = frrLogLevel
	}

	for _, s := range sm.sessions {
		var neighbor *neighborConfig
		var exist bool
		var rout *router

		routerName := RouterName(s.RouterID.String(), s.MyASN, s.VRFName)
		if rout, exist = routers[routerName]; !exist {
			rout = &router{
				myASN:               s.MyASN,
				neighbors:           make(map[string]*neighborConfig),
				neighborsProperties: make(map[string]*neighborProperties),
				ipV4Prefixes:        make(map[string]string),
				ipV6Prefixes:        make(map[string]string),
				vrf:                 s.VRFName,
			}
			if s.RouterID != nil {
				rout.routerID = s.RouterID.String()
			}
			routers[routerName] = rout
		}

		neighborName := NeighborName(s.PeerAddress, s.PeerASN, s.DynamicASN, s.VRFName)
		if neighbor, exist = rout.neighbors[neighborName]; !exist {
			host, port, err := net.SplitHostPort(s.PeerAddress)
			if err != nil {
				return nil, err
			}

			portUint, err := strconv.ParseUint(port, 10, 16)
			if err != nil {
				return nil, err
			}

			family := ipfamily.ForAddress(net.ParseIP(host))

			var connectTime int64
			if s.ConnectTime != nil {
				connectTime = int64(*s.ConnectTime / time.Second)
			}

			var holdTime *int64
			var keepaliveTime *int64
			if s.HoldTime != nil {
				time := int64(*s.HoldTime / time.Second)
				holdTime = &time
			}
			if s.KeepAliveTime != nil {
				time := int64(*s.KeepAliveTime / time.Second)
				keepaliveTime = &time
			}

			neighbor = &neighborConfig{
				Name:            neighborName,
				IPFamily:        family,
				ASN:             asnFor(s.PeerASN, s.DynamicASN),
				Addr:            host,
				Port:            uint16(portUint),
				HoldTime:        holdTime,
				KeepaliveTime:   keepaliveTime,
				ConnectTime:     connectTime,
				Password:        s.Password,
				Advertisements:  make([]*advertisementConfig, 0),
				BFDProfile:      s.BFDProfile,
				GracefulRestart: s.GracefulRestart,
				EBGPMultiHop:    s.EBGPMultiHop,
				VRFName:         s.VRFName,
				DisableMP:       s.DisableMP,
			}
			if s.SourceAddress != nil {
				neighbor.SrcAddr = s.SourceAddress.String()
			}
			rout.neighbors[neighborName] = neighbor
			rout.neighborsProperties[neighborName] = &neighborProperties{
				CommunitiesV4:      sets.New[string](),
				CommunitiesV6:      sets.New[string](),
				LargeCommunitiesV4: sets.New[string](),
				LargeCommunitiesV6: sets.New[string](),
				LocalPrefsV4:       sets.New[uint32](),
				LocalPrefsV6:       sets.New[uint32](),
			}
		}

		properties := rout.neighborsProperties[neighborName]

		for _, adv := range s.advertised {
			family := ipfamily.ForAddress(adv.Prefix.IP)

			communities := make([]string, 0)
			largeCommunities := make([]string, 0)

			// Convert community 32bits value to : format
			for _, c := range adv.Communities {
				if community.IsLarge(c) {
					largeCommunities = append(largeCommunities, c.String())
					if family == ipfamily.IPv4 {
						properties.LargeCommunitiesV4.Insert(c.String())
						continue
					}
					properties.LargeCommunitiesV6.Insert(c.String())
					continue
				}
				communities = append(communities, c.String())
				if family == ipfamily.IPv4 {
					properties.CommunitiesV4.Insert(c.String())
					continue
				}
				properties.CommunitiesV6.Insert(c.String())
			}

			prefix := adv.Prefix.String()
			advConfig := advertisementConfig{
				IPFamily:         family,
				Prefix:           prefix,
				Communities:      sort.StringSlice(communities),
				LargeCommunities: sort.StringSlice(largeCommunities),
				LocalPref:        adv.LocalPref,
			}
			if adv.LocalPref != 0 {
				if family == ipfamily.IPv4 {
					properties.LocalPrefsV4.Insert(adv.LocalPref)
				} else {
					properties.LocalPrefsV6.Insert(adv.LocalPref)
				}
			}

			neighbor.Advertisements, err = addToAdvertisements(neighbor.Advertisements, &advConfig)
			if err != nil {
				return nil, err
			}

			switch family {
			case ipfamily.IPv4:
				rout.ipV4Prefixes[prefix] = prefix
				neighbor.HasV4Advertisements = true
			case ipfamily.IPv6:
				rout.ipV6Prefixes[prefix] = prefix
				neighbor.HasV6Advertisements = true
			}
		}
	}

	for _, r := range sortMap(routers) {
		toAdd := &routerConfig{
			MyASN:        r.myASN,
			RouterID:     r.routerID,
			VRF:          r.vrf,
			Neighbors:    sortMap(r.neighbors),
			IPV4Prefixes: sortMap(r.ipV4Prefixes),
			IPV6Prefixes: sortMap(r.ipV6Prefixes),
		}
		// Filling in the neighbor properties
		for _, n := range toAdd.Neighbors {
			properties := r.neighborsProperties[n.Name]
			n.CommunitiesV4 = sets.List(properties.CommunitiesV4)
			n.CommunitiesV6 = sets.List(properties.CommunitiesV6)
			n.LargeCommunitiesV4 = sets.List(properties.LargeCommunitiesV4)
			n.LargeCommunitiesV6 = sets.List(properties.LargeCommunitiesV6)
			n.LocalPrefsV4 = sets.List(properties.LocalPrefsV4)
			n.LocalPrefsV6 = sets.List(properties.LocalPrefsV6)
		}

		config.Routers = append(config.Routers, toAdd)
	}
	return config, nil
}

func (sm *sessionManager) SetEventCallback(func(interface{})) {}

var debounceTimeout = 3 * time.Second
var failureTimeout = time.Second * 5

func NewSessionManager(l log.Logger, logLevel logging.Level) bgp.SessionManager {
	res := &sessionManager{
		sessions:     map[string]*session{},
		bfdProfiles:  []BFDProfile{},
		reloadConfig: make(chan reloadEvent),
		logLevel:     logLevelToFRR(logLevel),
	}
	reload := func(config *frrConfig) error {
		return generateAndReloadConfigFile(config, l)
	}

	debouncer(reload, res.reloadConfig, debounceTimeout, failureTimeout, l)

	reloadValidator(l, res.reloadConfig)

	return res
}

func mockNewSessionManager(l log.Logger, logLevel logging.Level) *sessionManager {
	res := &sessionManager{
		sessions:     map[string]*session{},
		bfdProfiles:  []BFDProfile{},
		reloadConfig: make(chan reloadEvent),
		logLevel:     logLevelToFRR(logLevel),
	}
	reload := func(config *frrConfig) error {
		return generateAndReloadConfigFile(config, l)
	}

	debouncer(reload, res.reloadConfig, debounceTimeout, failureTimeout, l)

	reloadValidator(l, res.reloadConfig)

	return res
}

func reloadValidator(l log.Logger, reload chan<- reloadEvent) {
	var tickerIntervals = 30 * time.Second
	var prevReloadTimeStamp string

	ticker := time.NewTicker(tickerIntervals)
	go func() {
		for range ticker.C {
			validateReload(l, &prevReloadTimeStamp, reload)
		}
	}()
}

const statusFileName = "/etc/frr_reloader/.status"

func validateReload(l log.Logger, prevReloadTimeStamp *string, reload chan<- reloadEvent) {
	bytes, err := os.ReadFile(statusFileName)
	if err != nil {
		if !os.IsNotExist(err) {
			level.Error(l).Log("op", "reload-validate", "error", err, "cause", "readFile", "fileName", statusFileName)
		}
		return
	}

	lastReloadStatus := strings.Fields(string(bytes))
	if len(lastReloadStatus) != 2 {
		level.Error(l).Log("op", "reload-validate", "error", err, "cause", "Fields", "bytes", string(bytes))
		return
	}

	timeStamp, status := lastReloadStatus[0], lastReloadStatus[1]
	if timeStamp == *prevReloadTimeStamp {
		return
	}

	*prevReloadTimeStamp = timeStamp

	if strings.Compare(status, "failure") == 0 {
		level.Error(l).Log("op", "reload-validate", "error", fmt.Errorf("reload failure"),
			"cause", "frr reload failed", "status", status)
		reload <- reloadEvent{useOld: true}
		return
	}

	level.Info(l).Log("op", "reload-validate", "success", "reloaded config")
}

func ConfigBFDProfileToFRR(p *metallbconfig.BFDProfile) *BFDProfile {
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

func logLevelToFRR(level logging.Level) string {
	// Allowed frr log levels are: emergencies, alerts, critical,
	// 		errors, warnings, notifications, informational, or debugging
	switch level {
	case logging.LevelAll, logging.LevelDebug:
		return "debugging"
	case logging.LevelInfo:
		return "informational"
	case logging.LevelWarn:
		return "warnings"
	case logging.LevelError:
		return "error"
	case logging.LevelNone:
		return "emergencies"
	}

	return "informational"
}

func sortMap[T any](toSort map[string]T) []T {
	keys := make([]string, 0)
	for k := range toSort {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	res := make([]T, 0)
	for _, k := range keys {
		res = append(res, toSort[k])
	}
	return res
}

func addToAdvertisements(current []*advertisementConfig, toAdd *advertisementConfig) ([]*advertisementConfig, error) {
	i := sort.Search(len(current), func(i int) bool { return current[i].Prefix >= toAdd.Prefix })
	if i == len(current) {
		return append(current, toAdd), nil
	}

	if current[i].Prefix == toAdd.Prefix {
		var err error
		current[i], err = mergeAdvertisements(current[i], toAdd)
		if err != nil {
			return nil, err
		}
		return current, nil
	}
	res := make([]*advertisementConfig, len(current)+1)
	copy(res[:i], current[:i])
	copy(res[i+1:], current[i:])
	res[i] = toAdd
	return res, nil
}

func mergeAdvertisements(adv1, adv2 *advertisementConfig) (*advertisementConfig, error) {
	res := &advertisementConfig{}
	if adv1.Prefix != adv2.Prefix {
		return nil, fmt.Errorf("cannot merge advertisements with different prefixes: %s != %s", adv1.Prefix, adv2.Prefix)
	}
	if adv1.IPFamily != adv2.IPFamily {
		return nil, fmt.Errorf("cannot merge advertisements with different ipfamilies: %s != %s", adv1.IPFamily, adv2.IPFamily)
	}
	if adv1.LocalPref != adv2.LocalPref {
		return nil, fmt.Errorf("cannot merge advertisements with different local preferences: %d != %d", adv1.LocalPref, adv2.LocalPref)
	}

	res.Prefix = adv1.Prefix
	res.IPFamily = adv1.IPFamily
	res.LocalPref = adv1.LocalPref
	res.Communities = mergeCommunities(adv1.Communities, adv2.Communities)
	res.LargeCommunities = mergeCommunities(adv1.LargeCommunities, adv2.LargeCommunities)
	return res, nil
}

func mergeCommunities(c1, c2 []string) []string {
	communities := sets.New[string]()
	communities.Insert(c1...)
	communities.Insert(c2...)
	return sets.List(communities)
}
