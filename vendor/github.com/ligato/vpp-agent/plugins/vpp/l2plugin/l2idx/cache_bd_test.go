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
package l2idx_test

import (
	"testing"

	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/vpp-agent/plugins/vpp/l2plugin/l2idx"
	. "github.com/onsi/gomega"
	"io"
)

type testWatcher struct {
	datasync.KeyValProtoWatcher
	datasync.WatchRegistration
	io.Closer

	Handle func(resyncName string)
}

func (testWatcher *testWatcher) Watch(resyncName string, changeChan chan datasync.ChangeEvent,
	resyncChan chan datasync.ResyncEvent, keyPrefixes ...string) (datasync.WatchRegistration, error) {
	testWatcher.Handle(resyncName)
	return testWatcher.WatchRegistration, nil
}

func TestCache(t *testing.T) {
	RegisterTestingT(t)

	nameChan := make(chan string, 1)

	watcher := &testWatcher{
		Handle: func(resyncName string) {
			nameChan <- resyncName
		},
	}

	bdIdx := l2idx.Cache(watcher)
	Expect(bdIdx).To(Not(BeNil()))

	var notif string
	Eventually(nameChan).Should(Receive(&notif))
	Expect(notif).To(ContainSubstring("bd-cache"))
}
