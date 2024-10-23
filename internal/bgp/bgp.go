// SPDX-License-Identifier:Apache-2.0

package bgp // import "go.universe.tf/metallb/internal/bgp"

import (
	"io"
	"net"
	"reflect"
	"time"

	"github.com/go-kit/log"
	"go.universe.tf/metallb/internal/bgp/community"
	"go.universe.tf/metallb/internal/config"
	v1 "k8s.io/api/core/v1"
)

// Advertisement represents one network path and its BGP attributes.
type Advertisement struct {
	// The prefix being advertised to the peer.
	Prefix *net.IPNet
	// The local preference of this route. Only propagated to IBGP
	// peers (i.e. where the peer ASN matches the local ASN).
	LocalPref uint32
	// BGP communities to attach to the path.
	Communities []community.BGPCommunity
	// Used to declare the intent of announcing IPs
	// only to the BGPPeers in this list.
	Peers []string
}

// Equal returns true if a and b are equivalent advertisements.
func (a *Advertisement) Equal(b *Advertisement) bool {
	if a.Prefix.String() != b.Prefix.String() {
		return false
	}
	if a.LocalPref != b.LocalPref {
		return false
	}

	if !reflect.DeepEqual(a.Peers, b.Peers) {
		return false
	}

	return reflect.DeepEqual(a.Communities, b.Communities)
}

func (a *Advertisement) MatchesPeer(peerName string) bool {
	if len(a.Peers) == 0 {
		return true
	}

	for _, peer := range a.Peers {
		if peer == peerName {
			return true
		}
	}
	return false
}

type Session interface {
	io.Closer
	Set(advs ...*Advertisement) error
}

type SessionParameters struct {
	PeerAddress     string
	SourceAddress   net.IP
	MyASN           uint32
	RouterID        net.IP
	PeerASN         uint32
	DynamicASN      string
	HoldTime        *time.Duration
	KeepAliveTime   *time.Duration
	ConnectTime     *time.Duration
	Password        string
	PasswordRef     v1.SecretReference
	CurrentNode     string
	BFDProfile      string
	GracefulRestart bool
	EBGPMultiHop    bool
	VRFName         string
	SessionName     string
	DisableMP       bool
}
type SessionManager interface {
	NewSession(logger log.Logger, args SessionParameters) (Session, error)
	SyncBFDProfiles(profiles map[string]*config.BFDProfile) error
	SyncExtraInfo(extras string) error
	SetEventCallback(func(interface{}))
}
