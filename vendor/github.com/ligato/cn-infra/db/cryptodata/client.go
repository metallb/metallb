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

package cryptodata

import (
	"crypto/rsa"
	"github.com/ligato/cn-infra/db/keyval"
	"errors"
	"crypto/rand"
	"io"
	"hash"
	"crypto/sha256"
)

// ClientAPI handles encrypting/decrypting and wrapping data
type ClientAPI interface {
	// EncryptData encrypts input data using provided public key
	EncryptData(inData []byte, pub *rsa.PublicKey) (data []byte, err error)
	// DecryptData decrypts input data
	DecryptData(inData []byte) (data []byte, err error)
	// WrapBytes wraps kv bytes plugin with support for decrypting encrypted data in values
	WrapBytes(cbw keyval.KvBytesPlugin, decrypter ArbitraryDecrypter) keyval.KvBytesPlugin
	// WrapBytes wraps kv proto plugin with support for decrypting encrypted data in values
	WrapProto(kvp keyval.KvProtoPlugin, decrypter ArbitraryDecrypter) keyval.KvProtoPlugin
}

// ClientConfig is result of converting Config.PrivateKeyFile to PrivateKey
type ClientConfig struct {
	// Private key is used to decrypt encrypted keys while reading them from store
	PrivateKeys []*rsa.PrivateKey
	// Reader used for encrypting/decrypting
	Reader io.Reader
	// Hash function used for hashing while encrypting
	Hash hash.Hash
}

// Client implements ClientAPI and ClientConfig
type Client struct {
	ClientConfig
}

// NewClient creates new client from provided config and reader
func NewClient(clientConfig ClientConfig) *Client {
	client := &Client{
		ClientConfig: clientConfig,
	}

	// If reader is nil use default rand.Reader
	if clientConfig.Reader == nil {
		client.Reader = rand.Reader
	}

	// If hash is nil use default sha256
	if clientConfig.Hash == nil {
		client.Hash = sha256.New()
	}

	return client
}

// EncryptData implements ClientAPI.EncryptData
func (client *Client) EncryptData(inData []byte, pub *rsa.PublicKey) (data []byte, err error) {
	return rsa.EncryptOAEP(client.Hash, client.Reader, pub, inData, nil)
}

// DecryptData implements ClientAPI.DecryptData
func (client *Client) DecryptData(inData []byte) (data []byte, err error) {
	for _, key := range client.PrivateKeys {
		data, err := rsa.DecryptOAEP(client.Hash, client.Reader, key, inData, nil)

		if err == nil {
			return data, nil
		}
	}

	return nil, errors.New("failed to decrypt data due to no private key matching")
}

// WrapBytes implements ClientAPI.WrapBytes
func (client *Client) WrapBytes(cbw keyval.KvBytesPlugin, decrypter ArbitraryDecrypter) keyval.KvBytesPlugin {
	return NewKvBytesPluginWrapper(cbw, decrypter, client.DecryptData)
}

// WrapProto implements ClientAPI.WrapProto
func (client *Client) WrapProto(kvp keyval.KvProtoPlugin, decrypter ArbitraryDecrypter) keyval.KvProtoPlugin {
	return NewKvProtoPluginWrapper(kvp, decrypter, client.DecryptData)
}
