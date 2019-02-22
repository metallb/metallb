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

package decoder

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ghodss/yaml"
)

// Default extension supported by this decoder
var defaultYAMLExt = ".yaml"

// YAMLDecoder can be used to decode yaml-type files
type YAMLDecoder struct {
	extensions []string
}

// Represents data structure of yaml file used for configuration
type yamlFile struct {
	Data []yamlFileEntry `yaml:"data"`
}

// Single record of key-value, where key is defined as string, and value is modeled as raw message
// (rest of the yaml file under the "value").
type yamlFileEntry struct {
	Key   string          `yaml:"key"`
	Value json.RawMessage `yaml:"value"`
}

// NewYAMLDecoder creates a new yaml decoder instance
func NewYAMLDecoder(extensions ...string) *YAMLDecoder {
	return &YAMLDecoder{
		extensions: append(extensions, defaultYAMLExt),
	}
}

// IsProcessable returns true if decoder is able to decode provided file
func (yd *YAMLDecoder) IsProcessable(file string) bool {
	for _, ext := range yd.extensions {
		if strings.HasSuffix(file, ext) {
			return true
		}
	}
	return false
}

// Encode provided file entries into json byte set
func (yd *YAMLDecoder) Encode(data []*FileDataEntry) ([]byte, error) {
	// Convert to json-specific structure
	var yamlFileEntries []yamlFileEntry
	for _, dataEntry := range data {
		yamlFileEntries = append(yamlFileEntries, yamlFileEntry{
			Key:   dataEntry.Key,
			Value: dataEntry.Value,
		})
	}
	// Encode to type specific structure
	yamlFile := &yamlFile{Data: yamlFileEntries}
	return yaml.Marshal(yamlFile)
}

// Decode provided YAML file
func (yd *YAMLDecoder) Decode(byteSet []byte) ([]*FileDataEntry, error) {
	if len(byteSet) == 0 {
		return []*FileDataEntry{}, nil
	}
	// Decode to type-specific structure
	yamlFile := yamlFile{}
	err := yaml.Unmarshal(byteSet, &yamlFile)
	if err != nil {
		return nil, fmt.Errorf("failed to decode yaml file: %v", err)
	}
	// Convert to common file data entry list structure
	var dataEntries []*FileDataEntry
	for _, dataEntry := range yamlFile.Data {
		dataEntries = append(dataEntries, &FileDataEntry{Key: dataEntry.Key, Value: dataEntry.Value})
	}
	return dataEntries, nil
}
