// Copyright (c) 2018 Cisco and/or its affiliates.
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

package cryptodata

import (
	"crypto/x509"
	"encoding/pem"
	"io/ioutil"

	"github.com/ligato/cn-infra/infra"
)

// Config is used to read private key from file
type Config struct {
	// Private key file is used to create rsa.PrivateKey from this PEM path
	PrivateKeyFiles []string `json:"private-key-files"`
}

// Deps lists dependencies of the cryptodata plugin.
type Deps struct {
	infra.PluginDeps
}

// Plugin implements cryptodata as plugin.
type Plugin struct {
	Deps
	ClientAPI
	// Plugin is disabled if there is no config file available
	disabled bool
}

// Init initializes cryptodata plugin.
func (p *Plugin) Init() (err error) {
	var config Config
	found, err := p.Cfg.LoadValue(&config)
	if err != nil {
		return err
	}

	if !found {
		p.Log.Info("cryptodata config not found, skip loading this plugin")
		p.disabled = true
		return nil
	}

	// Read client config and create it
	clientConfig := ClientConfig{}
	for _, file := range config.PrivateKeyFiles {
		bytes, err := ioutil.ReadFile(file)
		if err != nil {
			p.Log.Infof("%v", err)
			return err
		}

		for {
			block, rest := pem.Decode(bytes)
			if block == nil {
				break
			}

			privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
			if err != nil {
				p.Log.Infof("%v", err)
				return err
			}

			err = privateKey.Validate()
			if err != nil {
				p.Log.Infof("%v", err)
				return err
			}

			privateKey.Precompute()
			clientConfig.PrivateKeys = append(clientConfig.PrivateKeys, privateKey)

			if rest == nil {
				break
			}

			bytes = rest
		}
	}

	p.ClientAPI = NewClient(clientConfig)
	return
}

// Close closes cryptodata plugin.
func (p *Plugin) Close() error {
	return nil
}

// Disabled returns *true* if the plugin is not in use due to missing configuration.
func (p *Plugin) Disabled() bool {
	return p.disabled
}
