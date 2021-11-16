// SPDX-License-Identifier:Apache-2.0

package main

import (
	"net"
	"testing"

	"go.universe.tf/metallb/internal/config"
	"k8s.io/apimachinery/pkg/labels"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		desc     string
		config   *config.Config
		mustFail bool
	}{
		{
			desc: "peer with bfd profile",
			config: &config.Config{
				Peers: []*config.Peer{
					{
						Addr:          net.ParseIP("1.2.3.4"),
						NodeSelectors: []labels.Selector{labels.Everything()},
						BFDProfile:    "foo",
					},
				},
			},
			mustFail: true,
		},
		{
			desc: "bfd profile set",
			config: &config.Config{
				Peers: []*config.Peer{
					{
						Addr:          net.ParseIP("1.2.3.4"),
						NodeSelectors: []labels.Selector{labels.Everything()},
					},
				},
				BFDProfiles: map[string]*config.BFDProfile{
					"default": {
						Name: "default",
					},
				},
			},
			mustFail: true,
		},
		{
			desc: "should pass",
			config: &config.Config{
				Peers: []*config.Peer{
					{
						Addr:          net.ParseIP("1.2.3.4"),
						NodeSelectors: []labels.Selector{labels.Everything()},
					},
				},
			},
			mustFail: false,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			err := validateFRROnlyConfiguration(test.config)
			if test.mustFail && err == nil {
				t.Fatalf("Expected error for %s", test.desc)
			}
			if !test.mustFail && err != nil {
				t.Fatalf("Not expected error %s for %s", err, test.desc)
			}
		})
	}
}
