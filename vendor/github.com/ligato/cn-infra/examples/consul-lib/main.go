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
	"fmt"
	"log"

	"github.com/hashicorp/consul/api"
	"github.com/ligato/cn-infra/db/keyval"
	"github.com/ligato/cn-infra/db/keyval/consul"
	"github.com/ligato/cn-infra/db/keyval/kvproto"
	"github.com/ligato/cn-infra/examples/etcd-lib/model/phonebook"
)

func main() {
	// create new consul client with default config
	db, err := consul.NewClient(api.DefaultConfig())
	if err != nil {
		log.Fatal(err)
	}

	protoDb := kvproto.NewProtoWrapper(db)
	defer protoDb.Close()

	// put some data into store and get it
	put(protoDb, []string{"Me", "TheCompany", "123456"})
	get(protoDb, "Me")

	// put some data into store and list values and keys
	put(protoDb, []string{"You", "TheCompany", "666"})
	list(protoDb)
	listKeys(protoDb)

	// delete some data from store and list values
	del(protoDb, "Me")
	list(protoDb)
}

// listKeys lists keys
func listKeys(db keyval.ProtoBroker) {
	resp, err := db.ListKeys(phonebook.EtcdPath())
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("list keys:")
	for {
		key, _, stop := resp.GetNext()
		if stop {
			break
		}

		fmt.Printf("- %s\n", key)

	}
}

// list lists keys with values
func list(db keyval.ProtoBroker) {
	resp, err := db.ListValues(phonebook.EtcdPath())
	if err != nil {
		log.Fatal(err)
	}

	var revision int64
	fmt.Println("list values:")
	for {
		c := &phonebook.Contact{}
		kv, stop := resp.GetNext()
		if stop {
			break
		}
		if kv.GetRevision() > revision {
			revision = kv.GetRevision()
		}
		err = kv.GetValue(c)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("\t%s\n\t\t%s\n\t\t%s\n", c.Name, c.Company, c.Phonenumber)

	}
	fmt.Println("Revision", revision)
}

// put saves single entry
func put(db keyval.ProtoBroker, data []string) {
	c := &phonebook.Contact{Name: data[0], Company: data[1], Phonenumber: data[2]}

	key := phonebook.EtcdContactPath(c)

	err := db.Put(key, c)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Saved:", key)
}

// get retrieves single entry
func get(db keyval.ProtoBroker, data string) {
	c := &phonebook.Contact{Name: data}

	key := phonebook.EtcdContactPath(c)

	found, _, err := db.GetValue(key, c)
	if err != nil {
		log.Fatal(err)
	} else if !found {
		fmt.Println("Not found")
		return
	}

	fmt.Println("Loaded:", key, c)
}

// del deletes single entry
func del(db keyval.ProtoBroker, data string) {
	c := &phonebook.Contact{Name: data}

	key := phonebook.EtcdContactPath(c)

	existed, err := db.Delete(key)
	if err != nil {
		log.Fatal(err)
	} else if !existed {
		fmt.Println("Not existed")
		return
	}

	fmt.Println("Deleted:", key)
}
