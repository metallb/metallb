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
	"github.com/ligato/cn-infra/db/cryptodata"
	"os"
	"io/ioutil"
	"encoding/pem"
	"crypto/x509"
	"crypto/rsa"
	"encoding/base64"
)

// JSONData are example data to be decrypted
const JSONData = `{
  "encrypted":true,
  "value": {
	 "payload": "$crypto$%v"
  }
}`

func main() {
	// Read private key
	bytes, err := ioutil.ReadFile("key.pem")
	if err != nil {
		panic(err)
	}
	block, _ := pem.Decode(bytes)
	if block == nil {
		panic("failed to decode PEM for key key.pem")
	}
	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		panic(err)
	}

	// Read public key
	bytes, err = ioutil.ReadFile("key-pub.pem")
	if err != nil {
		panic(err)
	}
	block, _ = pem.Decode(bytes)
	if block == nil {
		panic("failed to decode PEM for key key-pub.pem")
	}
	pubInterface, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		panic(err)
	}
	publicKey := pubInterface.(*rsa.PublicKey)

	// Create cryptodata client
	client := cryptodata.NewClient(cryptodata.ClientConfig{
		PrivateKeys: []*rsa.PrivateKey{privateKey},
	})

	// Pass 1st argument from CLI as string to encrypt
	input := []byte(os.Args[1])
	fmt.Printf("> Input value:\n%v\n", string(input))

	// Encrypt input string using public key
	encrypted, err := client.EncryptData(input, publicKey)
	if err != nil {
		panic(err)
	}
	fmt.Printf("> Encrypted value:\n%v\n", encrypted)

	// Decrypt previously encrypted input string
	decrypted, err := client.DecryptData(encrypted)
	if err != nil {
		panic(err)
	}
	fmt.Printf("> Decrypted value:\n%v\n", string(decrypted))

	// Encode the string to base64 in order to make it compatible with JSON decrypter
	encryptedBase64 := base64.URLEncoding.EncodeToString(encrypted)

	// Try to decrypt JSON with encrypted data
	encryptedJSON := fmt.Sprintf(JSONData, encryptedBase64)
	fmt.Printf("> Encrypted json:\n%v\n", encryptedJSON)

	decrypter := cryptodata.NewDecrypterJSON()
	decryptedJSON, err := decrypter.Decrypt([]byte(encryptedJSON), client.DecryptData)
	if err != nil {
		panic(err)
	}

	fmt.Printf("> Decrypted json:\n%v\n", string(decryptedJSON.([]byte)))
}
