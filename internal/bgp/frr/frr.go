// SPDX-License-Identifier:Apache-2.0

package frr

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/go-kit/kit/log"
	"go.universe.tf/metallb/internal/bgp"
)

// As the MetalLB controller should handle messages synchronously, there should
// no need to lock this data structure. TODO: confirm this.
type frrState struct {
	sessions map[string]*Session
}

var state *frrState

func init() {
	state = &frrState{
		sessions: map[string]*Session{},
	}
}

type Session struct {
	myASN      uint32
	routerID   net.IP // May be nil, meaning "derive from context"
	myNode     string
	addr       string
	srcAddr    net.IP
	asn        uint32
	holdTime   time.Duration
	logger     log.Logger
	password   string
	advertised []*bgp.Advertisement
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

func (s *Session) Set(advs ...*bgp.Advertisement) error {
	sessionName := sessionName(s.srcAddr.String(), s.myASN, s.addr, s.asn)
	if _, found := state.sessions[sessionName]; !found {
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
	config, err := state.createConfig()
	if err != nil {
		s.advertised = oldAdvs
		return err
	}

	return generateAndReloadConfigFile(config)
}

// Close() shuts down the BGP session.
func (s *Session) Close() error {
	err := state.deleteSession(s)
	if err != nil {
		return err
	}

	frrConfig, err := state.createConfig()
	if err != nil {
		return err
	}

	return generateAndReloadConfigFile(frrConfig)
}

// New() creates a BGP session using the given session parameters.
//
// The session will immediately try to connect and synchronize its
// local state with the peer.
func New(l log.Logger, addr string, srcAddr net.IP, myASN uint32, routerID net.IP, asn uint32, holdTime time.Duration, password string, myNode string) (*Session, error) {
	session := &Session{
		myASN:      myASN,
		routerID:   routerID,
		myNode:     myNode,
		addr:       addr,
		srcAddr:    srcAddr,
		asn:        asn,
		holdTime:   holdTime,
		logger:     log.With(l, "peer", addr, "localASN", myASN, "peerASN", asn),
		password:   password,
		advertised: []*bgp.Advertisement{},
	}

	_ = state.addSession(session)

	frrConfig, err := state.createConfig()
	if err != nil {
		_ = state.deleteSession(session)
		return nil, err
	}

	err = generateAndReloadConfigFile(frrConfig)
	if err != nil {
		_ = state.deleteSession(session)
		return nil, err
	}

	return session, err
}

func (state *frrState) addSession(session *Session) error {
	if session == nil {
		return fmt.Errorf("invalid session")
	}
	sessionName := sessionName(session.srcAddr.String(), session.myASN, session.addr, session.asn)
	state.sessions[sessionName] = session

	return nil
}

func (state *frrState) deleteSession(session *Session) error {
	if session == nil {
		return fmt.Errorf("invalid session")
	}
	sessionName := sessionName(session.srcAddr.String(), session.myASN, session.addr, session.asn)
	delete(state.sessions, sessionName)

	return nil
}

func (s *frrState) createConfig() (*frrConfig, error) {
	hostname, err := osHostname()
	if err != nil {
		return nil, err
	}

	config := &frrConfig{
		Hostname: hostname,
		Loglevel: "", // TODO.
		Routers:  make(map[string]*routerConfig),
	}

	for _, session := range s.sessions {
		var router *routerConfig
		var neighbor *neighborConfig
		var exist bool

		routerName := routerName(session.routerID.String(), session.myASN)
		if router, exist = config.Routers[routerName]; !exist {
			router = &routerConfig{
				MyASN:     session.myASN,
				Neighbors: make(map[string]*neighborConfig),
			}
			if session.routerID != nil {
				router.RouterId = session.routerID.String()
			}
			config.Routers[routerName] = router
		}

		neighborName := neighborName(session.addr, session.asn)
		if neighbor, exist = router.Neighbors[neighborName]; !exist {
			host, port, err := net.SplitHostPort(session.addr)
			if err != nil {
				return nil, err
			}

			portUint, err := strconv.ParseUint(port, 10, 16)
			if err != nil {
				return nil, err
			}

			neighbor = &neighborConfig{
				ASN:            session.asn,
				Addr:           host,
				Port:           uint16(portUint),
				Advertisements: make([]*advertisementConfig, 0),
			}
			router.Neighbors[neighborName] = neighbor
		}

		/* As 'session.advertised' is a map, we can be sure there are no
		   duplicate prefixes and can, therefore, just add them to the
		   'neighbor.Advertisements' list. */
		for _, adv := range session.advertised {
			var version string
			if adv.Prefix.IP.To4() != nil {
				version = "ipv4"
			} else if adv.Prefix.IP.To16() != nil {
				version = "ipv6"
			}

			advConfig := advertisementConfig{
				Version: version,
				Prefix:  adv.Prefix.String(),
			}

			neighbor.Advertisements = append(neighbor.Advertisements, &advConfig)
		}
	}

	return config, nil
}
