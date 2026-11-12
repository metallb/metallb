// SPDX-License-Identifier:Apache-2.0

package frr

import (
	"errors"
	"fmt"
	"net/netip"

	"go.universe.tf/e2etest/pkg/executor"
)

// BFDPeers returns informations for the all the bfd peers in the given
// executor.
func BFDPeers(exec executor.Executor) (map[netip.Addr]BFDPeer, error) {
	json, err := exec.Exec("vtysh", "-c", "show bfd peers json")
	if err != nil {
		return nil, errors.Join(err, errors.New("Failed to query neighbours"))
	}
	peers, err := ParseBFDPeers(json)
	if err != nil {
		return nil, errors.Join(err, fmt.Errorf("Failed to parse neighbours %s", json))
	}
	// making a map out of the slice so it can
	// accessed by peer
	res := make(map[netip.Addr]BFDPeer)
	for _, p := range peers {
		ip, err := netip.ParseAddr(p.Peer)
		if err != nil {
			return nil, fmt.Errorf("invalid BFD peer address %q: %w", p.Peer, err)
		}
		res[ip.Unmap()] = p
	}
	return res, nil
}
