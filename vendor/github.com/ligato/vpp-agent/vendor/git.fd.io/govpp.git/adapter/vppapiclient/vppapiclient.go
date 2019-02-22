// Copyright (c) 2017 Cisco and/or its affiliates.
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

// +build !windows,!darwin

package vppapiclient

/*
#cgo CFLAGS: -DPNG_DEBUG=1
#cgo LDFLAGS: -lvppapiclient

#include <stdlib.h>
#include <stdio.h>
#include <stdint.h>
#include <arpa/inet.h>
#include <vpp-api/client/vppapiclient.h>

extern void go_msg_callback(uint16_t msg_id, void* data, size_t size);

typedef struct __attribute__((__packed__)) _req_header {
    uint16_t msg_id;
    uint32_t client_index;
    uint32_t context;
} req_header_t;

typedef struct __attribute__((__packed__)) _reply_header {
    uint16_t msg_id;
} reply_header_t;

static void
govpp_msg_callback(unsigned char *data, int size)
{
    reply_header_t *header = ((reply_header_t *)data);
    go_msg_callback(ntohs(header->msg_id), data, size);
}

static int
govpp_send(uint32_t context, void *data, size_t size)
{
	req_header_t *header = ((req_header_t *)data);
	header->context = htonl(context);
    return vac_write(data, size);
}

static int
govpp_connect(char *shm)
{
    return vac_connect("govpp", shm, govpp_msg_callback, 32);
}

static int
govpp_disconnect()
{
    return vac_disconnect();
}

static uint32_t
govpp_get_msg_index(char *name_and_crc)
{
    return vac_get_msg_index(name_and_crc);
}
*/
import "C"

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"unsafe"

	"git.fd.io/govpp.git/adapter"
	"github.com/fsnotify/fsnotify"
)

const (
	// shmDir is a directory where shared memory is supposed to be created.
	shmDir = "/dev/shm/"
	// vppShmFile is a default name of the file in the shmDir.
	vppShmFile = "vpe-api"
)

// global VPP binary API client, library vppapiclient only supports
// single connection at a time
var globalVppClient *vppClient

// stubVppClient is the default implementation of the VppAPI.
type vppClient struct {
	shmPrefix   string
	msgCallback adapter.MsgCallback
}

// NewVppClient returns a new VPP binary API client.
func NewVppClient(shmPrefix string) adapter.VppAPI {
	return &vppClient{
		shmPrefix: shmPrefix,
	}
}

// Connect connects the process to VPP.
func (a *vppClient) Connect() error {
	if globalVppClient != nil {
		return fmt.Errorf("already connected to binary API, disconnect first")
	}

	var rc _Ctype_int
	if a.shmPrefix == "" {
		rc = C.govpp_connect(nil)
	} else {
		shm := C.CString(a.shmPrefix)
		rc = C.govpp_connect(shm)
	}
	if rc != 0 {
		return fmt.Errorf("connecting to VPP binary API failed (rc=%v)", rc)
	}

	globalVppClient = a
	return nil
}

// Disconnect disconnects the process from VPP.
func (a *vppClient) Disconnect() error {
	globalVppClient = nil

	rc := C.govpp_disconnect()
	if rc != 0 {
		return fmt.Errorf("disconnecting from VPP binary API failed (rc=%v)", rc)
	}

	return nil
}

// GetMsgID returns a runtime message ID for the given message name and CRC.
func (a *vppClient) GetMsgID(msgName string, msgCrc string) (uint16, error) {
	nameAndCrc := C.CString(msgName + "_" + msgCrc)
	defer C.free(unsafe.Pointer(nameAndCrc))

	msgID := uint16(C.govpp_get_msg_index(nameAndCrc))
	if msgID == ^uint16(0) {
		// VPP does not know this message
		return msgID, fmt.Errorf("unknown message: %v (crc: %v)", msgName, msgCrc)
	}

	return msgID, nil
}

// SendMsg sends a binary-encoded message to VPP.
func (a *vppClient) SendMsg(context uint32, data []byte) error {
	rc := C.govpp_send(C.uint32_t(context), unsafe.Pointer(&data[0]), C.size_t(len(data)))
	if rc != 0 {
		return fmt.Errorf("unable to send the message (rc=%v)", rc)
	}
	return nil
}

// SetMsgCallback sets a callback function that will be called by the adapter
// whenever a message comes from VPP.
func (a *vppClient) SetMsgCallback(cb adapter.MsgCallback) {
	a.msgCallback = cb
}

// WaitReady blocks until shared memory for sending
// binary api calls is present on the file system.
func (a *vppClient) WaitReady() error {
	var path string

	// join the path to the shared memory segment
	if a.shmPrefix == "" {
		path = filepath.Join(shmDir, vppShmFile)
	} else {
		path = filepath.Join(shmDir, a.shmPrefix+"-"+vppShmFile)
	}

	// check if file at the path exists
	if _, err := os.Stat(path); err == nil {
		// file exists, we are ready
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	// file does not exist, start watching folder
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	if err := watcher.Add(shmDir); err != nil {
		return err
	}

	for {
		ev := <-watcher.Events
		if ev.Name == path {
			if (ev.Op & fsnotify.Create) == fsnotify.Create {
				// file was created, we are ready
				break
			}
		}
	}

	return nil
}

//export go_msg_callback
func go_msg_callback(msgID C.uint16_t, data unsafe.Pointer, size C.size_t) {
	// convert unsafe.Pointer to byte slice
	sliceHeader := &reflect.SliceHeader{Data: uintptr(data), Len: int(size), Cap: int(size)}
	byteSlice := *(*[]byte)(unsafe.Pointer(sliceHeader))

	globalVppClient.msgCallback(uint16(msgID), byteSlice)
}
