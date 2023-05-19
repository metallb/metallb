// SPDX-License-Identifier:Apache-2.0

package frr

import (
	"github.com/pkg/errors"
	"go.universe.tf/metallb/e2etest/pkg/executor"
)

// BFDPeers returns informations for the all the bfd peers in the given
// executor.
func BFDPeers(exec executor.Executor) (map[string]BFDPeer, error) {
	json, err := exec.Exec("vtysh", "-c", "show bfd peers json")
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to query neighbours")
	}
	peers, err := ParseBFDPeers(json)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to parse neighbours %s", json)
	}
	// making a map out of the slice so it can
	// accessed by peer
	res := make(map[string]BFDPeer)
	for _, p := range peers {
		res[p.Peer] = p
	}
	return res, nil
}
