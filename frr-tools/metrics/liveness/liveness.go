// SPDX-License-Identifier:Apache-2.0

package liveness

import (
	"net/http"
	"strings"

	"github.com/go-kit/log"

	"go.universe.tf/metallb/frr-tools/metrics/vtysh"
)

func Handler(frrCli vtysh.Cli, logger log.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res, err := frrCli("show daemons")
		if err != nil {
			http.Error(w, "failed to call show daemons", http.StatusInternalServerError)
			logger.Log("failed to call show daemons", err)
			return
		}
		expected := map[string]struct{}{
			"bfdd":     {},
			"bgpd":     {},
			"staticd":  {},
			"watchfrr": {},
			"zebra":    {},
		}

		runningDaemons := strings.Split(strings.TrimSuffix(res, "\n"), " ")
		for _, d := range runningDaemons {
			delete(expected, d)
		}
		if len(expected) > 0 {
			logger.Log("daemons not running. got: ", res, "missing: ", expected)
			http.NotFound(w, r)
		}
	})
}
