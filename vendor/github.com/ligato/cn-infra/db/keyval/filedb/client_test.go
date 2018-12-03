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

package filedb_test

import (
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/db/keyval"
	"github.com/ligato/cn-infra/db/keyval/filedb"
	"github.com/ligato/cn-infra/db/keyval/filedb/decoder"
	"github.com/ligato/cn-infra/db/keyval/filedb/filesystem"
	"github.com/ligato/cn-infra/logging/logrus"
	. "github.com/onsi/gomega"
)

var log = logrus.DefaultLogger()

func TestNewClient(t *testing.T) {
	RegisterTestingT(t)

	// Mocks
	fsMock := filesystem.NewFileSystemMock()
	dcMock := decoder.NewDecoderMock()
	// Get Paths
	fsMock.When("GetFileNames").ThenReturn([]string{
		"/path/to/file1.json",
		"/path/to/file2.json",
		"/path/to/directory/file3.json",
		"/path/to/directory/file4.json",
	})
	// Decode (let's say there is only JSON decoder)
	dcMock.When("IsProcessable").ThenReturn(true)
	dcMock.When("Decode").ThenReturn([]*decoder.FileDataEntry{
		{
			Key:   "/test-path/path1Key1",
			Value: []byte("path1Key1"),
		},
	})
	dcMock.When("IsProcessable").ThenReturn(true)
	dcMock.When("Decode").ThenReturn([]*decoder.FileDataEntry{
		{
			Key:   "/test-path/path2Key1",
			Value: []byte("path1Key1"),
		},
		{
			Key:   "/test-path/path2Key2",
			Value: []byte("path1Key2"),
		},
	})
	dcMock.When("IsProcessable").ThenReturn(true)
	dcMock.When("Decode").ThenReturn([]*decoder.FileDataEntry{
		{
			Key:   "/test-path/path3Key1",
			Value: []byte("path3Key1"),
		},
	})
	dcMock.When("IsProcessable").ThenReturn(true)
	dcMock.When("Decode").ThenReturn([]*decoder.FileDataEntry{
		{
			Key:   "/test-path/path4Key1",
			Value: []byte("path4Key1"),
		},
		{
			Key:   "/test-path/path4Key2",
			Value: []byte("path4Key2"),
		},
	})

	// Params
	paths := []string{
		"/path/to/file1.json",
		"/path/to/file2.json",
		"/path/to/directory",
	}

	client, err := filedb.NewClient(paths, "", []decoder.API{dcMock}, fsMock, log)
	defer client.Close()

	Expect(err).To(BeNil())
	Expect(client).ToNot(BeNil())
	Expect(client.GetPaths()).To(HaveLen(3))
	// Path1
	data := client.GetDataForFile("/path/to/file1.json")
	Expect(data).To(HaveLen(1))
	// Path2
	data = client.GetDataForFile("/path/to/file2.json")
	Expect(data).To(HaveLen(2))
	// Path3
	data = client.GetDataForFile("/path/to/directory/file3.json")
	Expect(data).To(HaveLen(1))
	// Path4
	data = client.GetDataForFile("/path/to/directory/file4.json")
	Expect(data).To(HaveLen(2))
}

// This test prepares four events:
// 	1. Create a file with two configuration items
// 	2. Modify one configuration item
//  3. Delete one configuration item
//  4. Delete file
func TestJsonReaderWatcher(t *testing.T) {
	RegisterTestingT(t)

	// Mocks
	fsMock := filesystem.NewFileSystemMock()
	dcMock := decoder.NewDecoderMock()
	// Client initialization
	fsMock.When("GetFileNames").ThenReturn([]string{"/path/to/file1.json"})
	dcMock.When("IsProcessable").ThenReturn(true)
	dcMock.When("Decode").ThenReturn()
	// Event 1 (create two items)
	dcMock.When("IsProcessable").ThenReturn(true)
	dcMock.When("Decode").ThenReturn([]*decoder.FileDataEntry{
		{
			Key:   "/test-path/vpp/config/interfaces/if1",
			Value: []byte("if1-created"),
		},
		{
			Key:   "/test-path/vpp/config/interfaces/if2",
			Value: []byte("if2-created"),
		},
	})
	// Event 2 (modify one item)
	dcMock.When("IsProcessable").ThenReturn(true)
	dcMock.When("Decode").ThenReturn([]*decoder.FileDataEntry{
		{
			Key:   "/test-path/vpp/config/interfaces/if1",
			Value: []byte("if1-modified"),
		},
		{
			// This one is still the same
			Key:   "/test-path/vpp/config/interfaces/if2",
			Value: []byte("if2-created"),
		},
	})
	// Event 3 (delete one item)
	fsMock.When("FileExists").ThenReturn(true)
	dcMock.When("IsProcessable").ThenReturn(true)
	dcMock.When("Decode").ThenReturn([]*decoder.FileDataEntry{
		{
			// This one is still the same
			Key:   "/test-path/vpp/config/interfaces/if2",
			Value: []byte("if2-created"),
		},
	})
	// Event 4 (delete file - last item)
	fsMock.When("FileExists").ThenReturn(false)

	// Init custom client
	paths := []string{"/path/to/file1.json"}
	client, err := filedb.NewClient(paths, "", []decoder.API{dcMock}, fsMock, log)
	defer client.Close()
	Expect(err).To(BeNil())
	Expect(client).ToNot(BeNil())

	// Test responses. Tests expected event value.
	var create1, create2, update, del, delFile bool
	f := func(resp keyval.BytesWatchResp) {
		logrus.DefaultLogger().Warnf("resp: %v, %v, %v", resp.GetKey(), resp.GetValue(), resp.GetPrevValue())
		if !create1 {
			Expect(resp.GetChangeType()).To(BeEquivalentTo(datasync.Put))
			Expect(resp.GetKey()).To(BeEquivalentTo("/test-path/vpp/config/interfaces/if1"))
			Expect(resp.GetValue()).To(BeEquivalentTo([]byte("if1-created")))
			Expect(resp.GetPrevValue()).To(BeNil())
			create1 = true
		} else if !create2 {
			Expect(resp.GetChangeType()).To(BeEquivalentTo(datasync.Put))
			Expect(resp.GetKey()).To(BeEquivalentTo("/test-path/vpp/config/interfaces/if2"))
			Expect(resp.GetValue()).To(BeEquivalentTo([]byte("if2-created")))
			Expect(resp.GetPrevValue()).To(BeNil())
			create2 = true
		} else if !update {
			Expect(resp.GetChangeType()).To(BeEquivalentTo(datasync.Put))
			Expect(resp.GetKey()).To(BeEquivalentTo("/test-path/vpp/config/interfaces/if1"))
			Expect(resp.GetValue()).To(BeEquivalentTo([]byte("if1-modified")))
			Expect(resp.GetPrevValue()).To(BeEquivalentTo([]byte("if1-created")))
			update = true
		} else if !del {
			Expect(resp.GetChangeType()).To(BeEquivalentTo(datasync.Delete))
			Expect(resp.GetKey()).To(BeEquivalentTo("/test-path/vpp/config/interfaces/if1"))
			Expect(resp.GetValue()).To(BeNil())
			Expect(resp.GetPrevValue()).To(BeEquivalentTo([]byte("if1-modified")))
			del = true
		} else if !delFile {
			Expect(resp.GetChangeType()).To(BeEquivalentTo(datasync.Delete))
			Expect(resp.GetKey()).To(BeEquivalentTo("/test-path/vpp/config/interfaces/if2"))
			Expect(resp.GetValue()).To(BeNil())
			Expect(resp.GetPrevValue()).To(BeEquivalentTo([]byte("if2-created")))
			delFile = true
		}
	}

	closeChan := make(chan string)

	client.Watch(f, closeChan, "/vpp/config/interfaces", "/vpp/config/bds")

	// Manually run event watcher in goroutine and give some time to start
	filedb.RunEventWatcher(client)
	time.Sleep(100 * time.Millisecond)

	// Test first event (create)
	fsMock.SendEvent(fsnotify.Event{
		Name: "/path/to/file1.json",
		Op:   fsnotify.Create,
	})
	Eventually(func() bool {
		return create1 && create2
	}, 200).Should(BeTrue())
	data, ok := client.GetDataForKey("/test-path/vpp/config/interfaces/if1")
	Expect(ok).To(BeTrue())
	Expect(data.Value).To(BeEquivalentTo([]byte("if1-created")))
	data, ok = client.GetDataForKey("/test-path/vpp/config/interfaces/if2")
	Expect(ok).To(BeTrue())
	Expect(data.Value).To(BeEquivalentTo([]byte("if2-created")))

	// Test second event (modify)
	fsMock.SendEvent(fsnotify.Event{
		Name: "/path/to/file1.json",
		Op:   fsnotify.Create,
	})
	Eventually(func() bool {
		return update
	}, 200).Should(BeTrue())
	data, ok = client.GetDataForKey("/test-path/vpp/config/interfaces/if1")
	Expect(ok).To(BeTrue())
	Expect(data.Value).To(BeEquivalentTo([]byte("if1-modified")))
	data, ok = client.GetDataForKey("/test-path/vpp/config/interfaces/if2")
	Expect(ok).To(BeTrue())
	Expect(data.Value).To(BeEquivalentTo([]byte("if2-created")))

	// Test third event (delete)
	fsMock.SendEvent(fsnotify.Event{
		Name: "/path/to/file1.json",
		Op:   fsnotify.Remove,
	})
	Eventually(func() bool {
		return del
	}, 200).Should(BeTrue())
	_, ok = client.GetDataForKey("/test-path/vpp/config/interfaces/if1")
	Expect(ok).To(BeFalse())
	data, ok = client.GetDataForKey("/test-path/vpp/config/interfaces/if2")
	Expect(ok).To(BeTrue())
	Expect(data.Value).To(BeEquivalentTo([]byte("if2-created")))

	// Test last event (file delete)
	fsMock.SendEvent(fsnotify.Event{
		Name: "/path/to/file1.json",
		Op:   fsnotify.Remove,
	})
	Eventually(func() bool {
		return delFile
	}, 200).Should(BeTrue())
	_, ok = client.GetDataForKey("/test-path/vpp/config/interfaces/if1")
	Expect(ok).To(BeFalse())
	_, ok = client.GetDataForKey("/test-path/vpp/config/interfaces/if2")
	Expect(ok).To(BeFalse())
}
