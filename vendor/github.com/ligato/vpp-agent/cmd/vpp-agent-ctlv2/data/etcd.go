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

package data

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io/ioutil"
	"os"

	"github.com/ligato/cn-infra/config"
	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/db/keyval"
	"github.com/ligato/cn-infra/db/keyval/etcd"
	"github.com/ligato/cn-infra/db/keyval/kvproto"
)

// EtcdCtl provides ETCD crud methods for vpp-agent-ctl
type EtcdCtl interface {
	// CreateEtcdClient creates a new connection to etcd
	CreateEtcdClient(configFile string) (*etcd.BytesConnectionEtcd, keyval.ProtoBroker, error)
	// ListAllAgentKeys prints all agent keys
	ListAllAgentKeys()
	// Put adds new data to etcd
	Put(key string, file string)
	// Del removes data from etcd
	Del(key string)
	// Get key value from the ETCD
	Get(key string)
	// Dump all values for given key prefix
	Dump(key string)
}

// ListAllAgentKeys prints all keys stored in the broker
func (ctl *VppAgentCtlImpl) ListAllAgentKeys() {
	ctl.Log.Debug("listAllAgentKeys")

	it, err := ctl.bytesConnection.ListKeys(ctl.serviceLabel.GetAllAgentsPrefix())
	if err != nil {
		ctl.Log.Error(err)
	}
	for {
		key, _, stop := it.GetNext()
		if stop {
			break
		}
		ctl.Log.Infof("key: %s", key)
	}
}

// CreateEtcdClient uses environment variable or ETCD config file to establish connection
func (ctl *VppAgentCtlImpl) CreateEtcdClient(configFile string) (*etcd.BytesConnectionEtcd, keyval.ProtoBroker, error) {
	var err error

	if configFile == "" {
		configFile = os.Getenv("ETCD_CONFIG")
	}

	cfg := &etcd.Config{}
	if configFile != "" {
		err = config.ParseConfigFromYamlFile(configFile, cfg)
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

// Get uses ETCD connection to get value for specific key
func (ctl *VppAgentCtlImpl) Get(key string) {
	ctl.Log.Debug("GET ", key)

	data, found, _, err := ctl.bytesConnection.GetValue(key)
	if err != nil {
		ctl.Log.Error(err)
		return
	}
	if !found {
		ctl.Log.Debug("No value found for the key", key)
	}
	ctl.Log.Println(string(data))
}

// Put stores key/data value
func (ctl *VppAgentCtlImpl) Put(key string, file string) {
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

// Del removes data under provided key
func (ctl *VppAgentCtlImpl) Del(key string) {
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

// Dump lists values under key. If no key is provided, all data is read.
func (ctl *VppAgentCtlImpl) Dump(key string) {
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
		ctl.Log.Println(kv.GetKey())
		ctl.Log.Println(string(kv.GetValue()))
		ctl.Log.Println()
		found = true
	}
	if !found {
		ctl.Log.Debug("No value found for the key", key)
	}
}

func (ctl *VppAgentCtlImpl) readData(file string) ([]byte, error) {
	var input []byte
	var err error

	if file == "-" {
		// read JSON from STDIN
		bio := bufio.NewReader(os.Stdin)
		buf := new(bytes.Buffer)
		if _, err = buf.ReadFrom(bio); err != nil {
			ctl.Log.Errorf("error reading json: %v", err)
		}
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
