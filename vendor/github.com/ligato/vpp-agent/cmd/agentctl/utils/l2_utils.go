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

package utils

import (
	"errors"
	"fmt"

	"github.com/ligato/cn-infra/db/keyval"
	"github.com/ligato/vpp-agent/plugins/vpp/model/l2"
)

// Bridge domain flag names
const (
	BDName       = "bridge-domain-name"
	IfName       = "interface-name"
	BVI          = "bvi"
	SHZ          = "split-horizon-group"
	IPAddress    = "ip-address"
	PhysAddress  = "physical-address"
	StaticConfig = "static-config"
	IsDrop       = "is-drop"
	IsDelete     = "is-delete"
)

// GetBridgeDomainKeyAndValue returns true if a bridge domain with the specified name
// was found together with the BD key, and data, and data broker.
func GetBridgeDomainKeyAndValue(endpoints []string, label string, bdName string) (bool, string, *l2.BridgeDomains_BridgeDomain, keyval.ProtoBroker) {
	validateBdIdentifiers(label, bdName)

	db, err := GetDbForOneAgent(endpoints, label)
	if err != nil {
		ExitWithError(ExitBadConnection, err)
	}

	key := l2.BridgeDomainKey(bdName)
	bd := &l2.BridgeDomains_BridgeDomain{}

	found, _, err := db.GetValue(key, bd)
	if err != nil {
		ExitWithError(ExitError, errors.New("Error getting existing config - "+err.Error()))
	}

	return found, key, bd, db
}

// GetFibEntry returns the FIB entry if exists.
func GetFibEntry(endpoints []string, label string, bdLabel string, fibMac string) (bool, string, *l2.FibTable_FibEntry) {
	db, err := GetDbForOneAgent(endpoints, label)
	if err != nil {
		ExitWithError(ExitBadConnection, err)
	}

	key := l2.FibKey(bdLabel, fibMac)
	fibEntry := &l2.FibTable_FibEntry{}

	found, _, err := db.GetValue(key, fibEntry)
	if err != nil {
		ExitWithError(ExitError, errors.New("Error getting existing config - "+err.Error()))
	}

	return found, key, fibEntry
}

// WriteBridgeDomainToDb writes bridge domain to the etcd.
func WriteBridgeDomainToDb(db keyval.ProtoBroker, key string, bd *l2.BridgeDomains_BridgeDomain) {
	validateBridgeDomain(bd)
	db.Put(key, bd)
}

// WriteFibDataToDb writes FIB entry to the etcd.
func WriteFibDataToDb(db keyval.ProtoBroker, key string, fib *l2.FibTable_FibEntry) {
	db.Put(key, fib)
}

// DeleteFibDataFromDb removes FIB entry from the etcd.
func DeleteFibDataFromDb(db keyval.ProtoBroker, key string) {
	db.Delete(key)
}

func validateBridgeDomain(bd *l2.BridgeDomains_BridgeDomain) {
	fmt.Printf("Validating bridge domain\n bd: %+v\n", bd)
	// todo implement
}

func validateBdIdentifiers(label string, name string) {
	if label == "" {
		ExitWithError(ExitInvalidInput, errors.New("Missing microservice label"))
	}
	if name == "" {
		ExitWithError(ExitInvalidInput, errors.New("Missing bridge domain name"))
	}
}
