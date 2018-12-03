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
	"bytes"
	"os"
	"strconv"

	"github.com/ligato/cn-infra/logging/logrus"
	"golang.org/x/crypto/bcrypt"
)

// A simple utility to help with password hashing. Hashed password can be stored as
// user password in config file. Always provide two parameters; password and cost.

func main() {
	// Read args
	args := os.Args
	if len(args) != 3 {
		usage()
		return
	}

	pass := args[1]
	cost, err := strconv.Atoi(args[2])
	if err != nil {
		logrus.DefaultLogger().Errorf("invalid cost format: %v", err)
		os.Exit(1)
	}
	if cost < 4 || cost > 31 {
		logrus.DefaultLogger().Errorf("invalid cost value %d, set it in interval 4-31", cost)
		os.Exit(1)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(pass), cost)
	if err != nil {
		logrus.DefaultLogger().Errorf("failed to hash password: %v", err)
		os.Exit(1)
	}

	logrus.DefaultLogger().Print(string(hash))
}

// Show info
func usage() {
	var buffer bytes.Buffer
	// Crud operation
	buffer.WriteString(` 

	Simple password hasher. Since the user credentials 
	can be stored in the config file, this utility helps 
	to hash the password and store it as such. 

	./password-hasher <password> <cost>

	The cost value has to match the one in the config file.
	Allowed interval is 4-31. Note that the high number
	takes a lot of time to process.

	Please do not use your bank account password. 
	We didn't spend much time securing it.
	`)

	logrus.DefaultLogger().Print(buffer.String())
}
