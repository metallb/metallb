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
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"io/ioutil"
	"log"

	"github.com/ligato/cn-infra/agent"
	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/db/cryptodata"
	"github.com/ligato/cn-infra/db/keyval"
	"github.com/ligato/cn-infra/db/keyval/etcd"
	"github.com/ligato/cn-infra/examples/cryptodata-proto-plugin/ipsec"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/servicelabel"
	"github.com/pkg/errors"
)

// PluginName represents name of plugin.
const PluginName = "example"

func main() {
	// Start Agent with ExamplePlugin using ETCDPlugin, CryptoDataPlugin, logger and service label.
	p := &ExamplePlugin{
		Deps: Deps{
			Log:          logging.ForPlugin(PluginName),
			ServiceLabel: &servicelabel.DefaultPlugin,
			KvProto:      &etcd.DefaultPlugin,
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
	KvProto      keyval.KvProtoPlugin
	CryptoData   cryptodata.ClientAPI
}

// ExamplePlugin demonstrates the usage of cryptodata API.
type ExamplePlugin struct {
	Deps
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

	// Prepare data
	key1, err := plugin.encryptData("cryptoKey1", publicKey)
	if err != nil {
		return err
	}
	key2, err := plugin.encryptData("cryptoKey2", publicKey)
	if err != nil {
		return err
	}
	encryptedData := &ipsec.TunnelInterfaces{
		Tunnels: []*ipsec.TunnelInterfaces_Tunnel{
			{
				Name:           "tunnel1",
				LocalCryptoKey: key1,
				IpAddresses: []string{
					"192.168.0.1",
					"192.168.0.2",
				},
			},
			{
				Name:            "tunnel2",
				RemoteCryptoKey: key2,
				IpAddresses: []string{
					"192.168.0.5",
					"192.168.0.8",
				},
			},
		},
	}
	plugin.Log.Infof("Putting value %v", encryptedData)

	// Prepare path for storing the data
	key := plugin.etcdKey(ipsec.KeyPrefix)

	// Prepare mapping
	decrypter := cryptodata.NewDecrypterProto()
	decrypter.RegisterMapping(
		&ipsec.TunnelInterfaces{},
		[]string{"Tunnels", "LocalCryptoKey"},
		[]string{"Tunnels", "RemoteCryptoKey"},
	)

	// Prepare broker and watcher with crypto layer
	crypto := plugin.CryptoData.WrapProto(plugin.KvProto, decrypter)
	broker := crypto.NewBroker(keyval.Root)
	watcher := crypto.NewWatcher(keyval.Root)

	// Start watching
	watcher.Watch(plugin.watchChanges, nil, key)

	// Put proto data to ETCD
	err = broker.Put(key, encryptedData)
	if err != nil {
		return err
	}

	// Get proto data from ETCD and decrypt them with crypto layer
	decryptedData := &ipsec.TunnelInterfaces{}
	_, _, err = broker.GetValue(key, decryptedData)
	if err != nil {
		return err
	}
	plugin.Log.Infof("Got value %v", decryptedData)

	// List all values from ETCD under key and decrypt them with crypto layer
	iter, err := broker.ListValues(key)
	if err != nil {
		return err
	}

	kv, stop := iter.GetNext()
	if !stop {
		decryptedDataList := &ipsec.TunnelInterfaces{}
		kv.GetValue(decryptedDataList)
		plugin.Log.Infof("Listed value %v", decryptedDataList)
	}

	// Close agent and example
	close(plugin.exampleFinished)

	return nil
}

// Close closes ExamplePlugin
func (plugin *ExamplePlugin) Close() error {
	return nil
}

// watchChanges is watching for changes in DB
func (plugin *ExamplePlugin) watchChanges(x datasync.ProtoWatchResp) {
	message := &ipsec.TunnelInterfaces{}
	err := x.GetValue(message)
	if err == nil {
		plugin.Log.Infof("Got watch message %v", message)
	}
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
