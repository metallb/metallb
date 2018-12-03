//  Copyright (c) 2018 Cisco and/or its affiliates.
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at:
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package main

import (
	"log"
	"github.com/ligato/cn-infra/agent"
	"github.com/ligato/cn-infra/db/keyval/etcd"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/servicelabel"
	"github.com/ligato/cn-infra/db/cryptodata"
	"github.com/ligato/cn-infra/config"
	"io/ioutil"
	"encoding/pem"
	"crypto/x509"
	"crypto/rsa"
	"github.com/pkg/errors"
	"encoding/base64"
	"fmt"
	"github.com/ligato/cn-infra/db/keyval"
)

// PluginName represents name of plugin.
const PluginName = "example"

// JSONData are example data sent to db
const JSONData = `{
  "encrypted":true,
  "value": {
	 "payload": "$crypto$%v"
  }
}`

func main() {
	// Start Agent with ExamplePlugin using ETCDPlugin CryptoDataPlugin, logger and service label.
	p := &ExamplePlugin{
		Deps: Deps{
			Log:          logging.ForPlugin(PluginName),
			ServiceLabel: &servicelabel.DefaultPlugin,
			CryptoData:   &cryptodata.DefaultPlugin,
		},
		exampleFinished: make(chan struct{}),
	}

	if err := agent.NewAgent(
		agent.AllPlugins(p),
		agent.QuitOnClose(p.exampleFinished),
	).Run(); err != nil {
		log.Fatal(err)
	}
}

// Deps lists dependencies of ExamplePlugin.
type Deps struct {
	Log          logging.PluginLogger
	ServiceLabel servicelabel.ReaderAPI
	CryptoData   cryptodata.ClientAPI
}

// ExamplePlugin demonstrates the usage of cryptodata API.
type ExamplePlugin struct {
	Deps
	db              *etcd.BytesConnectionEtcd
	exampleFinished chan struct{}
}

// String returns plugin name
func (plugin *ExamplePlugin) String() string {
	return PluginName
}

// Init starts the consumer.
func (plugin *ExamplePlugin) Init() error {
	// Read public key
	publicKey, err := readPublicKey("../cryptodata-lib/key-pub.pem")
	if err != nil {
		return err
	}

	// Create ETCD connection
	plugin.db, err = plugin.newEtcdConnection("etcd.conf")
	if err != nil {
		return err
	}

	// Prepare data
	data, err := plugin.encryptData("hello-world", publicKey)
	if err != nil {
		return err
	}
	encryptedJSON := fmt.Sprintf(JSONData, data)
	plugin.Log.Infof("Putting value %v", encryptedJSON)

	// Prepare path for storing the data
	key := plugin.etcdKey("value")

	// Put JSON data to ETCD
	err = plugin.db.Put(key, []byte(encryptedJSON))
	if err != nil {
		return err
	}

	// WrapBytes ETCD connection with crypto layer
	dbWrapped := plugin.CryptoData.WrapBytes(plugin.db, cryptodata.NewDecrypterJSON())
	broker := dbWrapped.NewBroker(keyval.Root)

	// Get JSON data from ETCD and decrypt them with crypto layer
	decryptedJSON, _, _, err := broker.GetValue(key)
	if err != nil {
		return err
	}
	plugin.Log.Infof("Got value %v", string(decryptedJSON))

	// Close agent and example
	close(plugin.exampleFinished)

	return nil
}

// Close closes ExamplePlugin
func (plugin *ExamplePlugin) Close() error {
	if plugin.db != nil {
		return plugin.db.Close()
	}
	return nil
}

// The ETCD key prefix used for this example
func (plugin *ExamplePlugin) etcdKey(label string) string {
	return "/vnf-agent/" + plugin.ServiceLabel.GetAgentLabel() + "/api/v1/example/db/simple/" + label
}

// encryptData first encrypts the provided value using crypto layer and then encodes
// the data with base64 for JSON compatibility
func (plugin *ExamplePlugin) encryptData(value string, publicKey *rsa.PublicKey) (string, error) {
	encryptedValue, err := plugin.CryptoData.EncryptData([]byte(value), publicKey)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(encryptedValue), nil
}

// newEtcdConnection creates new ETCD bytes connection from provided etcd config path
func (plugin *ExamplePlugin) newEtcdConnection(configPath string) (*etcd.BytesConnectionEtcd, error) {
	etcdFileConfig := &etcd.Config{}
	err := config.ParseConfigFromYamlFile(configPath, etcdFileConfig)
	if err != nil {
		return nil, err
	}

	etcdConfig, err := etcd.ConfigToClient(etcdFileConfig)
	if err != nil {
		return nil, err
	}

	return etcd.NewEtcdConnectionWithBytes(*etcdConfig, plugin.Log)
}

// readPublicKey reads rsa public key from PEM file on provided path
func readPublicKey(path string) (*rsa.PublicKey, error) {
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(bytes)
	if block == nil {
		return nil, errors.New("failed to decode PEM for key " + path)
	}
	pubInterface, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	publicKey, ok := pubInterface.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("failed to convert public key to rsa.PublicKey")
	}

	return publicKey, nil
}
