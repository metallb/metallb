// SPDX-License-Identifier:Apache-2.0

package main

import (
	"fmt"

	"github.com/pkg/errors"
	"go.universe.tf/metallb/internal/config"
)

func validateFRROnlyConfiguration(cfg *config.Config) error {
	for _, p := range cfg.Peers {
		if p.BFDProfile != "" {
			return fmt.Errorf("peer %s has bfd profile set", p.Addr)
		}
	}
	if len(cfg.BFDProfiles) > 0 {
		return errors.New("bfd profiles section set")
	}
	return nil
}
