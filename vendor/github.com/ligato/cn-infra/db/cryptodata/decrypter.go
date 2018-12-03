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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/gogo/protobuf/proto"
)

// DecryptFunc is function that decrypts input data
type DecryptFunc func(inData []byte) (data []byte, err error)

// ArbitraryDecrypter represents decrypter that looks for encrypted values inside arbitrary data and returns
// the data with the values decrypted
type ArbitraryDecrypter interface {
	// IsEncrypted checks if provided data are encrypted
	IsEncrypted(inData interface{}) bool
	// Decrypt processes input data and decrypts specific fields using decryptFunc
	Decrypt(inData interface{}, decryptFunc DecryptFunc) (data interface{}, err error)
}

// EncryptionCheck is used to check for data to contain encrypted marker
type EncryptionCheck struct {
	// IsEncrypted returns true if data was marked as encrypted
	IsEncrypted bool `json:"encrypted"`
}

// DecrypterJSON is ArbitraryDecrypter implementation that can decrypt JSON values
type DecrypterJSON struct {
	// Prefix that is required for matching and decrypting values
	prefix string
}

// NewDecrypterJSON creates new JSON decrypter with default value for Prefix being `$crypto$`
func NewDecrypterJSON() *DecrypterJSON {
	return &DecrypterJSON{"$crypto$"}
}

// SetPrefix sets prefix that is required for matching and decrypting values
func (d DecrypterJSON) SetPrefix(prefix string) {
	d.prefix = prefix
}

// IsEncrypted checks if provided data are marked as encrypted. First it tries to unmarshal JSON to EncryptionCheck
// and then check the IsEncrypted for being true
func (d DecrypterJSON) IsEncrypted(object interface{}) bool {
	inData, ok := object.([]byte)
	if !ok {
		return false
	}

	var jsonData EncryptionCheck
	err := json.Unmarshal(inData, &jsonData)
	return err == nil && jsonData.IsEncrypted
}

// Decrypt tries to find encrypted values in JSON data and decrypt them. It uses IsEncrypted function on the
// data to check if it contains any encrypted data.
// Then it parses data as JSON as tries to lookup all values that begin with `Prefix`, then trim prefix, base64
// decode the data and decrypt them using provided decrypt function.
// This function can accept only []byte and return []byte
func (d DecrypterJSON) Decrypt(object interface{}, decryptFunc DecryptFunc) (interface{}, error) {
	if !d.IsEncrypted(object) {
		return object, nil
	}

	inData := object.([]byte)
	var jsonData map[string]interface{}
	err := json.Unmarshal(inData, &jsonData)
	if err != nil {
		return nil, err
	}

	err = d.decryptJSON(jsonData, decryptFunc)
	if err != nil {
		return nil, err
	}

	return json.Marshal(jsonData)
}

// decryptJSON recursively navigates JSON structure and tries to decrypt all string values with Prefix
func (d DecrypterJSON) decryptJSON(data map[string]interface{}, decryptFunc DecryptFunc) error {
	for k, v := range data {
		switch t := v.(type) {
		case string:
			if s := strings.TrimPrefix(t, d.prefix); s != t {
				s, err := base64.URLEncoding.DecodeString(s)
				if err != nil {
					return err
				}

				decryptedData, err := decryptFunc(s)
				if err != nil {
					return err
				}

				data[k] = string(decryptedData)
			}
		case map[string]interface{}:
			err := d.decryptJSON(t, decryptFunc)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// DecrypterProto is ArbitraryDecrypter implementation that can decrypt protobuf values
type DecrypterProto struct {
	// Mapping maps proto message type to path
	mapping map[reflect.Type][][]string
}

// NewDecrypterProto creates new protobuf decrypter with empty mapping
func NewDecrypterProto() *DecrypterProto {
	return &DecrypterProto{mapping: make(map[reflect.Type][][]string)}
}

// RegisterMapping registers mapping to decrypter that maps proto.Message type to path used to access encrypted values
func (d DecrypterProto) RegisterMapping(object proto.Message, paths ...[]string) {
	d.mapping[reflect.TypeOf(object)] = paths
}

// IsEncrypted checks if provided data type is contained in the Mapping
func (d DecrypterProto) IsEncrypted(object interface{}) bool {
	_, ok := object.(proto.Message)
	if !ok {
		return false
	}
	_, ok = d.mapping[reflect.TypeOf(object)]
	return ok
}

// Decrypt tries to find encrypted values in protobuf data and decrypt them. It uses IsEncrypted function on the
// data to check if it contains any encrypted data.
// Then it goes through provided mapping and tries to reflect all fields in the mapping and decrypt string values the
// mappings must point to.
// This function can accept only proto.Message and return proto.Message
func (d DecrypterProto) Decrypt(object interface{}, decryptFunc DecryptFunc) (interface{}, error) {
	if !d.IsEncrypted(object) {
		return object, nil
	}

	for _, path := range d.mapping[reflect.TypeOf(object)] {
		if err := d.decryptStruct(object, path, decryptFunc); err != nil {
			return nil, err
		}
	}

	return object, nil
}

// decryptStruct recursively tries to decrypt fields in object on provided path using provided decryptFunc
func (d DecrypterProto) decryptStruct(object interface{}, path []string, decryptFunc DecryptFunc) error {
	v, ok := object.(reflect.Value)
	if !ok {
		v = reflect.ValueOf(object)
	}

	for pathIndex, key := range path {
		if v.Kind() == reflect.Ptr {
			v = v.Elem()
		}

		if v.Kind() == reflect.Struct {
			v = v.FieldByName(key)
		}

		if v.Kind() == reflect.Slice {
			for i := 0; i < v.Len(); i++ {
				val := v.Index(i)
				kind := val.Kind()
				index := pathIndex

				if kind == reflect.Struct || kind == reflect.Ptr {
					index++
				}

				if err := d.decryptStruct(val, path[index:], decryptFunc); err != nil {
					return err
				}
			}

			return nil
		}

		if v.Kind() == reflect.String {
			val := v.String()
			if val == "" {
				continue
			}

			decoded, err := base64.URLEncoding.DecodeString(val)
			if err != nil {
				return err
			}

			decrypted, err := decryptFunc(decoded)
			if err != nil {
				return err
			}

			v.SetString(string(decrypted))
			return nil
		}

		return fmt.Errorf("failed to process path on %v", v)
	}

	return nil
}
