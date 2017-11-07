// Copyright 2017 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"net"
	"sync"
)

type Service struct {
	Addrs []net.IP
}

type State struct {
	sync.Mutex
	Services map[string]*Service
}

func (s *State) Set(svc string, addrs []net.IP) {
	s.Lock()
	defer s.Unlock()
	s.Services[svc] = &Service{
		Addrs: addrs,
	}
}

func (s *State) Delete(svc string) {
	s.Lock()
	defer s.Unlock()
	delete(s.Services, svc)
}
