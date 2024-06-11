// SPDX-License-Identifier:Apache-2.0

package frr

import (
	"strings"

	"errors"

	"go.universe.tf/e2etest/pkg/executor"
)

// Daemons returns informations for the all the neighbors in the given
// executor.
func Daemons(exec executor.Executor) ([]string, error) {
	res, err := exec.Exec("vtysh", "-c", "show daemons")
	if err != nil {
		return nil, errors.Join(err, errors.New("Failed to query neighbours"))
	}
	res = strings.TrimSuffix(res, "\n")
	daemons := strings.Split(res, " ")
	for i := range daemons {
		daemons[i] = strings.TrimPrefix(daemons[i], " ")
		daemons[i] = strings.TrimSuffix(daemons[i], " ")
	}
	return daemons, nil
}
