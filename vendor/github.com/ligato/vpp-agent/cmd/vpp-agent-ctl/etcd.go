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

// package vpp-agent-ctl implements the vpp-agent-ctl test tool for testing
// VPP Agent plugins. In addition to testing, the vpp-agent-ctl tool can
// be used to demonstrate the usage of VPP Agent plugins and their APIs.

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/ligato/cn-infra/config"
	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/db/keyval"
	"github.com/ligato/cn-infra/db/keyval/etcd"
	"github.com/ligato/cn-infra/db/keyval/kvproto"
)

// CreateEtcdClient uses environment variable or ETCD config file to establish connection
func (ctl *VppAgentCtl) createEtcdClient(configFile string) (*etcd.BytesConnectionEtcd, keyval.ProtoBroker, error) {
	var err error

	if configFile == "" {
		configFile = os.Getenv("ETCD_CONFIG")
	}

	cfg := &etcd.Config{}
	if configFile != "" {
		err := config.ParseConfigFromYamlFile(configFile, cfg)
		if err != nil {
			return nil, nil, err
		}
	}
	etcdConfig, err := etcd.ConfigToClient(cfg)
	if err != nil {
		ctl.Log.Fatal(err)
	}

	bDB, err := etcd.NewEtcdConnectionWithBytes(*etcdConfig, ctl.Log)
	if err != nil {
		return nil, nil, err
	}

	return bDB, kvproto.NewProtoWrapperWithSerializer(bDB, &keyval.SerializerJSON{}).
		NewBroker(ctl.serviceLabel.GetAgentPrefix()), nil
}

// ListAllAgentKeys prints all keys stored in the broker
func (ctl *VppAgentCtl) listAllAgentKeys() {
	ctl.Log.Debug("listAllAgentKeys")

	it, err := ctl.broker.ListKeys(ctl.serviceLabel.GetAllAgentsPrefix())
	if err != nil {
		ctl.Log.Error(err)
	}
	for {
		key, _, stop := it.GetNext()
		if stop {
			break
		}
		//ctl.Log.Println("key: ", key)
		fmt.Println("key: ", key)
	}
}

// EtcdGet uses ETCD connection to get value for specific key
func (ctl *VppAgentCtl) etcdGet(key string) {
	ctl.Log.Debug("GET ", key)

	data, found, _, err := ctl.bytesConnection.GetValue(key)
	if err != nil {
		ctl.Log.Error(err)
		return
	}
	if !found {
		ctl.Log.Debug("No value found for the key", key)
	}
	//ctl.Log.Println(string(data))
	fmt.Println(string(data))
}

// EtcdPut stores key/data value
func (ctl *VppAgentCtl) etcdPut(key string, file string) {
	input, err := ctl.readData(file)
	if err != nil {
		ctl.Log.Fatal(err)
	}

	ctl.Log.Println("DB putting ", key, " ", string(input))

	err = ctl.bytesConnection.Put(key, input)
	if err != nil {
		ctl.Log.Panic("error putting the data ", key, " that to DB from ", file, ", err: ", err)
	}
	ctl.Log.Println("DB put successful ", key, " ", file)
}

// EtcdDel removes data under provided key
func (ctl *VppAgentCtl) etcdDel(key string) {
	ctl.Log.Debug("DEL ", key)

	found, err := ctl.bytesConnection.Delete(key, datasync.WithPrefix())
	if err != nil {
		ctl.Log.Error(err)
		return
	}
	if found {
		ctl.Log.Debug("Data deleted:", key)
	} else {
		ctl.Log.Debug("No value found for the key", key)
	}
}

// EtcdDump lists values under key. If no key is provided, all data is read.
func (ctl *VppAgentCtl) etcdDump(key string) {
	ctl.Log.Debug("DUMP ", key)

	data, err := ctl.bytesConnection.ListValues(key)
	if err != nil {
		ctl.Log.Error(err)
		return
	}

	var found bool
	for {
		kv, stop := data.GetNext()
		if stop {
			break
		}
		//ctl.Log.Println(kv.GetKey())
		//ctl.Log.Println(string(kv.GetValue()))
		//ctl.Log.Println()
		fmt.Println(kv.GetKey())
		fmt.Println(string(kv.GetValue()))
		fmt.Println()
		found = true
	}
	if !found {
		ctl.Log.Debug("No value found for the key", key)
	}
}

func (ctl *VppAgentCtl) readData(file string) ([]byte, error) {
	var input []byte
	var err error

	if file == "-" {
		// read JSON from STDIN
		bio := bufio.NewReader(os.Stdin)
		buf := new(bytes.Buffer)
		buf.ReadFrom(bio)
		input = buf.Bytes()
	} else {
		// read JSON from file
		input, err = ioutil.ReadFile(file)
		if err != nil {
			ctl.Log.Panic("error reading the data that needs to be written to DB from ", file, ", err: ", err)
		}
	}

	// validate the JSON
	var js map[string]interface{}
	if json.Unmarshal(input, &js) != nil {
		ctl.Log.Panic("Not a valid JSON: ", string(input))
	}
	return input, err
}
