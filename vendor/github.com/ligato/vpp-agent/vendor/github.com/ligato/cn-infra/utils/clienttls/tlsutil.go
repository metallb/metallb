// Copyright (c) 2017 Cisco and/or its affiliates.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package clienttls provides tls utilities.
package clienttls

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"github.com/coreos/etcd/pkg/tlsutil"
)

// TLS stores the client side TLS settings
type TLS struct {
	Enabled    bool   `json:"enabled"`     // enable/disable TLS
	SkipVerify bool   `json:"skip-verify"` // whether to skip verification of server name & certificate
	Certfile   string `json:"cert-file"`   // client certificate
	Keyfile    string `json:"key-file"`    // client private key
	CAfile     string `json:"ca-file"`     // certificate authority
}

// CreateTLSConfig used to generate the crypto/tls Config
func CreateTLSConfig(config TLS) (*tls.Config, error) {
	var (
		cert *tls.Certificate
		cp   *x509.CertPool
		err  error
	)
	if config.Certfile != "" && config.Keyfile != "" {
		cert, err = tlsutil.NewCert(config.Certfile, config.Keyfile, nil)
		if err != nil {
			return nil, fmt.Errorf("tlsutil.NewCert() failed: %s", err)
		}
	}

	if config.CAfile != "" {
		cp, err = tlsutil.NewCertPool([]string{config.CAfile})
		if err != nil {
			return nil, fmt.Errorf("tlsutil.NewCertPool() failed: %s", err)
		}
	}

	tlsConfig := &tls.Config{
		MinVersion:         tls.VersionTLS10,
		InsecureSkipVerify: config.SkipVerify,
		RootCAs:            cp,
	}
	if cert != nil {
		tlsConfig.Certificates = []tls.Certificate{*cert}
	}

	return tlsConfig, nil
}
