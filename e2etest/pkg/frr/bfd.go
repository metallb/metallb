// SPDX-License-Identifier:Apache-2.0

package frr

import (
	"github.com/pkg/errors"
	"go.universe.tf/metallb/e2etest/pkg/executor"
	bgpfrr "go.universe.tf/metallb/internal/bgp/frr"
)

// BFDPeers returns informations for the all the bfd peers in the given
// executor.
func BFDPeers(exec executor.Executor) (map[string]bgpfrr.BFDPeer, error) {
	res, err := exec.Exec("vtysh", "-c", "show bfd peers json")
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to query neighbours")
	}
	peers, err := bgpfrr.ParseBFDPeers(res)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to parse neighbours %s", res)
	}
	return peers, nil
}
