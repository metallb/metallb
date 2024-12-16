// SPDX-License-Identifier:Apache-2.0

package frr

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"reflect"
	"sort"
	"strconv"
	"sync"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	frrv1beta1 "github.com/metallb/frr-k8s/api/v1beta1"
	"go.universe.tf/metallb/internal/bgp"
	"go.universe.tf/metallb/internal/bgp/community"
	"go.universe.tf/metallb/internal/bgp/frr"
	metallbconfig "go.universe.tf/metallb/internal/config"
	"go.universe.tf/metallb/internal/logging"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type sessionManager struct {
	sessions        map[string]*session
	bfdProfiles     []frr.BFDProfile
	targetNamespace string
	nodeToConfigure string
	sync.Mutex
	configChangedCallback func(interface{})
	logger                log.Logger
	logLevel              logging.Level
}

func (sm *sessionManager) SetEventCallback(callback func(interface{})) {
	sm.Lock()
	defer sm.Unlock()
	sm.configChangedCallback = callback
}

type session struct {
	bgp.SessionParameters
	sessionManager *sessionManager
	advertised     []*bgp.Advertisement
	logger         log.Logger
}

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
	err := s.sessionManager.updateConfig()
	if err != nil {
		s.advertised = oldAdvs
		return err
	}

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

	err = s.sessionManager.updateConfig()
	if err != nil {
		return err
	}

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
		logger:            log.With(l, "peer", args.PeerAddress, "localASN", args.MyASN, "peerASN", args.PeerASN),
		advertised:        []*bgp.Advertisement{},
		sessionManager:    sm,
		SessionParameters: args,
	}

	_ = sm.addSession(s)

	err := sm.updateConfig()
	if err != nil {
		_ = sm.deleteSession(s)
		return nil, err
	}

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
	if extraInfo != "" {
		return errors.New("extra info not supported in frr-k8s mode")
	}
	return nil
}

func (sm *sessionManager) SyncBFDProfiles(profiles map[string]*metallbconfig.BFDProfile) error {
	sm.Lock()
	defer sm.Unlock()
	sm.bfdProfiles = make([]frr.BFDProfile, 0)
	for _, p := range profiles {
		frrProfile := frr.ConfigBFDProfileToFRR(p)
		sm.bfdProfiles = append(sm.bfdProfiles, *frrProfile)
	}

	err := sm.updateConfig()
	if err != nil {
		return err
	}
	return nil
}

func (sm *sessionManager) updateConfig() error {
	newConfig := frrv1beta1.FRRConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ConfigName(sm.nodeToConfigure),
			Namespace: sm.targetNamespace,
		},
		Spec: frrv1beta1.FRRConfigurationSpec{
			NodeSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"kubernetes.io/hostname": sm.nodeToConfigure,
				},
			},
			BGP: frrv1beta1.BGPConfig{
				Routers:     make([]frrv1beta1.Router, 0),
				BFDProfiles: make([]frrv1beta1.BFDProfile, 0),
			},
		},
	}

	type router struct {
		myASN     uint32
		routerID  string
		neighbors map[string]frrv1beta1.Neighbor
		vrf       string
		prefixes  map[string]string
	}

	routers := make(map[string]*router)

	for _, s := range sm.sessions {
		var neighbor frrv1beta1.Neighbor
		var exist bool
		var rout *router

		routerName := frr.RouterName(s.RouterID.String(), s.MyASN, s.VRFName)
		if rout, exist = routers[routerName]; !exist {
			rout = &router{
				myASN:     s.MyASN,
				neighbors: make(map[string]frrv1beta1.Neighbor),
				prefixes:  make(map[string]string),
				vrf:       s.VRFName,
			}
			if s.RouterID != nil {
				rout.routerID = s.RouterID.String()
			}
			routers[routerName] = rout
		}

		neighborName := frr.NeighborName(s.PeerAddress, s.PeerASN, s.DynamicASN, s.VRFName)
		if neighbor, exist = rout.neighbors[neighborName]; !exist {
			host, port, err := net.SplitHostPort(s.PeerAddress)
			if err != nil {
				return err
			}

			portUint, err := strconv.ParseUint(port, 10, 16)
			if err != nil {
				return err
			}
			portUint16 := uint16(portUint)

			if !reflect.DeepEqual(s.PasswordRef, corev1.SecretReference{}) && s.Password != "" {
				return fmt.Errorf("invalid session with password and secret set: %s", sessionName(*s))
			}

			var connectTime *metav1.Duration
			if s.ConnectTime != nil {
				connectTime = &metav1.Duration{Duration: *s.ConnectTime}
			}

			var holdTime *metav1.Duration
			var keepaliveTime *metav1.Duration
			if s.HoldTime != nil {
				holdTime = &metav1.Duration{Duration: *s.HoldTime}
			}
			if s.KeepAliveTime != nil {
				keepaliveTime = &metav1.Duration{Duration: *s.KeepAliveTime}
			}

			neighbor = frrv1beta1.Neighbor{
				ASN:                   s.PeerASN,
				DynamicASN:            frrv1beta1.DynamicASNMode(s.DynamicASN),
				Address:               host,
				Port:                  &portUint16,
				HoldTime:              holdTime,
				KeepaliveTime:         keepaliveTime,
				ConnectTime:           connectTime,
				BFDProfile:            s.BFDProfile,
				EnableGracefulRestart: s.GracefulRestart,
				EBGPMultiHop:          s.EBGPMultiHop,
				ToAdvertise: frrv1beta1.Advertise{
					Allowed: frrv1beta1.AllowedOutPrefixes{
						Prefixes: make([]string, 0),
					},
					PrefixesWithLocalPref: make([]frrv1beta1.LocalPrefPrefixes, 0),
					PrefixesWithCommunity: make([]frrv1beta1.CommunityPrefixes, 0),
				},
				Password: s.Password,
				PasswordSecret: frrv1beta1.SecretReference{
					Name:      s.PasswordRef.Name,
					Namespace: s.PasswordRef.Namespace,
				},
				DisableMP: s.DisableMP,
			}
		}

		/* As 'session.advertised' is a map, we can be sure there are no
		   duplicate prefixes and can, therefore, just add them to the
		   'neighbor.Advertisements' list. */
		prefixesForCommunity := map[string][]string{}
		prefixesForLocalPref := map[uint32][]string{}
		for _, adv := range s.advertised {
			prefix := adv.Prefix.String()
			neighbor.ToAdvertise.Allowed.Prefixes = append(neighbor.ToAdvertise.Allowed.Prefixes, prefix)
			rout.prefixes[prefix] = prefix

			for _, c := range adv.Communities {
				comm := c.String()
				if community.IsLarge(c) {
					comm = fmt.Sprintf("large:%s", c.String())
				}
				prefixesForCommunity[comm] = append(prefixesForCommunity[comm], prefix)
			}
			if adv.LocalPref != 0 {
				prefixesForLocalPref[adv.LocalPref] = append(prefixesForLocalPref[adv.LocalPref], prefix)
			}
		}
		neighbor.ToAdvertise.PrefixesWithCommunity = toAdvertiseWithCommunity(prefixesForCommunity)
		neighbor.ToAdvertise.PrefixesWithLocalPref = toAdvertiseWithLocalPref(prefixesForLocalPref)
		neighbor.ToAdvertise.Allowed.Prefixes = removeDuplicates(neighbor.ToAdvertise.Allowed.Prefixes)
		sort.Strings(neighbor.ToAdvertise.Allowed.Prefixes)

		rout.neighbors[neighborName] = neighbor
	}

	for _, r := range sortMap(routers) {
		toAdd := frrv1beta1.Router{
			ASN:       r.myASN,
			ID:        r.routerID,
			VRF:       r.vrf,
			Neighbors: sortMap(r.neighbors),
			Prefixes:  sortMap(r.prefixes),
		}
		newConfig.Spec.BGP.Routers = append(newConfig.Spec.BGP.Routers, toAdd)
	}

	for _, bfd := range sm.bfdProfiles {
		bfd := bfd
		toAdd := frrv1beta1.BFDProfile{
			Name:             bfd.Name,
			ReceiveInterval:  bfd.ReceiveInterval,
			TransmitInterval: bfd.TransmitInterval,
			DetectMultiplier: bfd.DetectMultiplier,
			EchoInterval:     bfd.EchoInterval,
			MinimumTTL:       bfd.MinimumTTL,
			EchoMode:         &bfd.EchoMode,
			PassiveMode:      &bfd.PassiveMode,
		}

		newConfig.Spec.BGP.BFDProfiles = append(newConfig.Spec.BGP.BFDProfiles, toAdd)
	}
	sort.Slice(newConfig.Spec.BGP.BFDProfiles, func(i, j int) bool {
		return newConfig.Spec.BGP.BFDProfiles[i].Name < newConfig.Spec.BGP.BFDProfiles[j].Name
	})

	sm.configChangedCallback(newConfig)
	if sm.logLevel == logging.LevelDebug {
		sm.dumpConfig(newConfig)
	}

	return nil
}

func (sm *sessionManager) dumpConfig(config frrv1beta1.FRRConfiguration) {
	toDump, err := ConfigToDump(config)
	if err != nil {
		level.Error(sm.logger).Log("component", "frrk8s", "event", "failed to dump config", "error", err)
	}
	level.Debug(sm.logger).Log("component", "frrk8s", "event", "sent new config to the controller", "config", toDump)
}

func toAdvertiseWithCommunity(prefixesForCommunity map[string][]string) []frrv1beta1.CommunityPrefixes {
	res := []frrv1beta1.CommunityPrefixes{}
	for c, prefixes := range prefixesForCommunity {
		sort.Strings(prefixes)
		res = append(res, frrv1beta1.CommunityPrefixes{Community: c, Prefixes: removeDuplicates(prefixes)})
	}
	sort.Slice(res, func(i, j int) bool {
		return res[i].Community < res[j].Community
	})
	return res
}

func removeDuplicates(input []string) []string {
	seen := make(map[string]struct{})
	result := []string{}

	for _, value := range input {
		if _, ok := seen[value]; !ok {
			seen[value] = struct{}{}
			result = append(result, value)
		}
	}
	return result
}

func toAdvertiseWithLocalPref(prefixesForLocalPref map[uint32][]string) []frrv1beta1.LocalPrefPrefixes {
	res := []frrv1beta1.LocalPrefPrefixes{}
	for p, prefixes := range prefixesForLocalPref {
		sort.Strings(prefixes)
		res = append(res, frrv1beta1.LocalPrefPrefixes{LocalPref: p, Prefixes: removeDuplicates(prefixes)})
	}
	sort.Slice(res, func(i, j int) bool {
		return res[i].LocalPref < res[j].LocalPref
	})
	return res
}

func NewSessionManager(l log.Logger, logLevel logging.Level, node, namespace string) bgp.SessionManager {
	res := &sessionManager{
		sessions:        map[string]*session{},
		nodeToConfigure: node,
		targetNamespace: namespace,
		logger:          l,
		logLevel:        logLevel,
	}

	return res
}

func ConfigName(node string) string {
	return "metallb-" + node
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

func ConfigToDump(config frrv1beta1.FRRConfiguration) (string, error) {
	toDump := config.DeepCopy()
	for _, r := range toDump.Spec.BGP.Routers {
		for i := range r.Neighbors {
			r.Neighbors[i].Password = "<retracted>"
		}
	}

	res, err := json.Marshal(toDump)
	if err != nil {
		return "", err
	}
	return string(res), nil
}
