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
)

// Default extension supported by this decoder
var defaultJSONExt = ".json"

// JSONDecoder can be used to decode json-type files
type JSONDecoder struct {
	extensions []string
}

// Represents data structure of json file used for configuration
type jsonFile struct {
	Data []jsonFileEntry `json:"data"`
}

// Single record of key-value, where key is defined as string, and value is modeled as raw message
// (rest of the json file under the "value").
type jsonFileEntry struct {
	Key   string          `json:"key"`
	Value json.RawMessage `json:"value"`
}

// NewJSONDecoder creates a new decoder instance
func NewJSONDecoder(extensions ...string) *JSONDecoder {
	return &JSONDecoder{
		extensions: append(extensions, defaultJSONExt),
	}
}

// IsProcessable returns true if decoder is able to decode provided file
func (jd *JSONDecoder) IsProcessable(file string) bool {
	for _, ext := range jd.extensions {
		if strings.HasSuffix(file, ext) {
			return true
		}
	}
	return false
}

// Encode provided file entries into JSON byte set
func (jd *JSONDecoder) Encode(data []*FileDataEntry) ([]byte, error) {
	// Convert to json-specific structure
	var jsonFileEntries []jsonFileEntry
	for _, dataEntry := range data {
		jsonFileEntries = append(jsonFileEntries, jsonFileEntry{
			Key:   dataEntry.Key,
			Value: dataEntry.Value,
		})
	}
	// Encode to type specific structure
	jsonFile := &jsonFile{Data: jsonFileEntries}
	return json.Marshal(jsonFile)
}

// Decode provided byte set of json file and returns set of file data entries
func (jd *JSONDecoder) Decode(byteSet []byte) ([]*FileDataEntry, error) {
	if len(byteSet) == 0 {
		return []*FileDataEntry{}, nil
	}
	// Decode to type-specific structure
	jsonFile := jsonFile{}
	err := json.Unmarshal(byteSet, &jsonFile)
	if err != nil {
		return nil, fmt.Errorf("failed to decode json file: %v", err)
	}
	// Convert to common file data entry list structure
	var dataEntries []*FileDataEntry
	for _, dataEntry := range jsonFile.Data {
		dataEntries = append(dataEntries, &FileDataEntry{Key: dataEntry.Key, Value: dataEntry.Value})
	}
	return dataEntries, nil
}
