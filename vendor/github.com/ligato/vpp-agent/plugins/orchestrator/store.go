//  Copyright (c) 2019 Cisco and/or its affiliates.
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

package orchestrator

import "github.com/gogo/protobuf/proto"

type memStore struct {
	db map[string]proto.Message
}

func newMemStore() *memStore {
	return &memStore{
		db: map[string]proto.Message{},
	}
}

func (s *memStore) Reset() {
	s.db = map[string]proto.Message{}
}

func (s *memStore) Delete(key string) {
	delete(s.db, key)
}

func (s *memStore) Update(key string, val proto.Message) {
	s.db[key] = val
}
