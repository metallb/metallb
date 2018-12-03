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

import "bytes"

// API for file decoders
type API interface {
	// IsProcessable returns true if decoder can encode/decode given file
	IsProcessable(file string) bool
	// Encode requires common file representation to encode it to data stream
	Encode(data []*FileDataEntry) ([]byte, error)
	// Decode requires data stream from file reader and decodes it into common file representation if successful
	Decode(data []byte) ([]*FileDataEntry, error)
}

// File is common structure of a decoded file with path and list of key-value data
type File struct {
	Path string
	Data []*FileDataEntry
}

// FileDataEntry is single data record structure
type FileDataEntry struct {
	Key   string
	Value []byte
}

// CompareTo compares two files - new, modified and deleted entries. Result is against the parameter.
func (f1 *File) CompareTo(f2 *File) (changed, removed []*FileDataEntry) {
	if f1.Path != f2.Path {
		return f1.Data, f2.Data
	}
	for _, f2Data := range f2.Data {
		var found bool
		for _, f1Data := range f1.Data {
			if f1Data.Key == f2Data.Key {
				found = true
				if !bytes.Equal(f1Data.Value, f2Data.Value) {
					changed = append(changed, f1Data)
					break
				}
			}
		}
		if !found {
			removed = append(removed, f2Data)
		}
	}
	for _, f1Data := range f1.Data {
		var found bool
		for _, f2Data := range f2.Data {
			if f1Data.Key == f2Data.Key {
				found = true
				break
			}
		}
		if !found {
			changed = append(changed, f1Data)
		}
	}

	return
}
