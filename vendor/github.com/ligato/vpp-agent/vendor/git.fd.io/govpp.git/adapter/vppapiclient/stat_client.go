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
#include <vpp-api/client/stat_client.h>

static int
govpp_stat_connect(char *socket_name)
{
	return stat_segment_connect(socket_name);
}

static void
govpp_stat_disconnect()
{
    stat_segment_disconnect();
}

static uint32_t*
govpp_stat_segment_ls(uint8_t ** pattern)
{
	return stat_segment_ls(pattern);
}

static int
govpp_stat_segment_vec_len(void *vec)
{
	return stat_segment_vec_len(vec);
}

static void
govpp_stat_segment_vec_free(void *vec)
{
	stat_segment_vec_free(vec);
}

static char*
govpp_stat_segment_dir_index_to_name(uint32_t *dir, uint32_t index)
{
	return stat_segment_index_to_name(dir[index]);
}

static stat_segment_data_t*
govpp_stat_segment_dump(uint32_t *counter_vec)
{
	return stat_segment_dump(counter_vec);
}

static stat_segment_data_t
govpp_stat_segment_dump_index(stat_segment_data_t *data, int index)
{
	return data[index];
}

static int
govpp_stat_segment_data_type(stat_segment_data_t *data)
{
	return data->type;
}

static double
govpp_stat_segment_data_get_scalar_value(stat_segment_data_t *data)
{
	return data->scalar_value;
}

static double
govpp_stat_segment_data_get_error_value(stat_segment_data_t *data)
{
	return data->error_value;
}

static uint64_t**
govpp_stat_segment_data_get_simple_counter(stat_segment_data_t *data)
{
	return data->simple_counter_vec;
}

static uint64_t*
govpp_stat_segment_data_get_simple_counter_index(stat_segment_data_t *data, int index)
{
	return data->simple_counter_vec[index];
}

static uint64_t
govpp_stat_segment_data_get_simple_counter_index_value(stat_segment_data_t *data, int index, int index2)
{
	return data->simple_counter_vec[index][index2];
}

static vlib_counter_t**
govpp_stat_segment_data_get_combined_counter(stat_segment_data_t *data)
{
	return data->combined_counter_vec;
}

static vlib_counter_t*
govpp_stat_segment_data_get_combined_counter_index(stat_segment_data_t *data, int index)
{
	return data->combined_counter_vec[index];
}

static uint64_t
govpp_stat_segment_data_get_combined_counter_index_packets(stat_segment_data_t *data, int index, int index2)
{
	return data->combined_counter_vec[index][index2].packets;
}

static uint64_t
govpp_stat_segment_data_get_combined_counter_index_bytes(stat_segment_data_t *data, int index, int index2)
{
	return data->combined_counter_vec[index][index2].bytes;
}

static void
govpp_stat_segment_data_free(stat_segment_data_t *data)
{
	stat_segment_data_free(data);
}

static uint8_t**
govpp_stat_segment_string_vector(uint8_t ** string_vector, char *string)
{
	return stat_segment_string_vector(string_vector, string);
}
*/
import "C"
import (
	"fmt"
	"os"
	"unsafe"

	"git.fd.io/govpp.git/adapter"
)

var (
	// DefaultStatSocket is the default path for the VPP stat socket file.
	DefaultStatSocket = "/run/vpp/stats.sock"
)

// global VPP stats API client, library vppapiclient only supports
// single connection at a time
var globalStatClient *statClient

// stubStatClient is the default implementation of StatsAPI.
type statClient struct {
	socketName string
}

// NewStatClient returns new VPP stats API client.
func NewStatClient(socketName string) adapter.StatsAPI {
	return &statClient{
		socketName: socketName,
	}
}

func (c *statClient) Connect() error {
	if globalStatClient != nil {
		return fmt.Errorf("already connected to stats API, disconnect first")
	}

	var sockName string
	if c.socketName == "" {
		sockName = DefaultStatSocket
	} else {
		sockName = c.socketName
	}

	rc := C.govpp_stat_connect(C.CString(sockName))
	if rc != 0 {
		return fmt.Errorf("connecting to VPP stats API failed (rc=%v)", rc)
	}

	globalStatClient = c
	return nil
}

func (c *statClient) Disconnect() error {
	globalStatClient = nil

	C.govpp_stat_disconnect()
	return nil
}

func (c *statClient) ListStats(patterns ...string) (stats []string, err error) {
	dir := C.govpp_stat_segment_ls(convertStringSlice(patterns))
	defer C.govpp_stat_segment_vec_free(unsafe.Pointer(dir))

	l := C.govpp_stat_segment_vec_len(unsafe.Pointer(dir))
	for i := 0; i < int(l); i++ {
		nameChar := C.govpp_stat_segment_dir_index_to_name(dir, C.uint32_t(i))
		stats = append(stats, C.GoString(nameChar))
		C.free(unsafe.Pointer(nameChar))
	}

	return stats, nil
}

func (c *statClient) DumpStats(patterns ...string) (stats []*adapter.StatEntry, err error) {
	dir := C.govpp_stat_segment_ls(convertStringSlice(patterns))
	defer C.govpp_stat_segment_vec_free(unsafe.Pointer(dir))

	dump := C.govpp_stat_segment_dump(dir)
	defer C.govpp_stat_segment_data_free(dump)

	l := C.govpp_stat_segment_vec_len(unsafe.Pointer(dump))
	for i := 0; i < int(l); i++ {
		v := C.govpp_stat_segment_dump_index(dump, C.int(i))
		nameChar := v.name
		name := C.GoString(nameChar)
		typ := adapter.StatType(C.govpp_stat_segment_data_type(&v))

		stat := &adapter.StatEntry{
			Name: name,
			Type: typ,
		}

		switch typ {
		case adapter.ScalarIndex:
			stat.Data = adapter.ScalarStat(C.govpp_stat_segment_data_get_scalar_value(&v))

		case adapter.ErrorIndex:
			stat.Data = adapter.ErrorStat(C.govpp_stat_segment_data_get_error_value(&v))

		case adapter.SimpleCounterVector:
			length := int(C.govpp_stat_segment_vec_len(unsafe.Pointer(C.govpp_stat_segment_data_get_simple_counter(&v))))
			vector := make([][]adapter.Counter, length)
			for k := 0; k < length; k++ {
				for j := 0; j < int(C.govpp_stat_segment_vec_len(unsafe.Pointer(C.govpp_stat_segment_data_get_simple_counter_index(&v, _Ctype_int(k))))); j++ {
					vector[k] = append(vector[k], adapter.Counter(C.govpp_stat_segment_data_get_simple_counter_index_value(&v, _Ctype_int(k), _Ctype_int(j))))
				}
			}
			stat.Data = adapter.SimpleCounterStat(vector)

		case adapter.CombinedCounterVector:
			length := int(C.govpp_stat_segment_vec_len(unsafe.Pointer(C.govpp_stat_segment_data_get_combined_counter(&v))))
			vector := make([][]adapter.CombinedCounter, length)
			for k := 0; k < length; k++ {
				for j := 0; j < int(C.govpp_stat_segment_vec_len(unsafe.Pointer(C.govpp_stat_segment_data_get_combined_counter_index(&v, _Ctype_int(k))))); j++ {
					vector[k] = append(vector[k], adapter.CombinedCounter{
						Packets: adapter.Counter(C.govpp_stat_segment_data_get_combined_counter_index_packets(&v, _Ctype_int(k), _Ctype_int(j))),
						Bytes:   adapter.Counter(C.govpp_stat_segment_data_get_combined_counter_index_bytes(&v, _Ctype_int(k), _Ctype_int(j))),
					})
				}
			}
			stat.Data = adapter.CombinedCounterStat(vector)

		default:
			fmt.Fprintf(os.Stderr, "invalid stat type: %v (%d)", typ, typ)
			continue

		}

		stats = append(stats, stat)
	}

	return stats, nil
}

func convertStringSlice(strs []string) **C.uint8_t {
	var arr **C.uint8_t
	for _, str := range strs {
		arr = C.govpp_stat_segment_string_vector(arr, C.CString(str))
	}
	return arr
}
