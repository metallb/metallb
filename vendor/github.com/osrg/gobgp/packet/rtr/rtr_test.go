// Copyright (C) 2015 Nippon Telegraph and Telephone Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package rtr

import (
	"encoding/hex"
	"math/rand"
	"net"
	"reflect"
	"testing"
	"time"
)

func verifyRTRMessage(t *testing.T, m1 RTRMessage) {
	buf1, _ := m1.Serialize()
	m2, err := ParseRTR(buf1)
	if err != nil {
		t.Error(err)
	}
	buf2, _ := m2.Serialize()

	if reflect.DeepEqual(buf1, buf2) == true {
		t.Log("OK")
	} else {
		t.Errorf("Something wrong")
		t.Error(len(buf1), m1, hex.EncodeToString(buf1))
		t.Error(len(buf2), m2, hex.EncodeToString(buf2))
	}
}

func randUint32() uint32 {
	rand.Seed(time.Now().UnixNano())
	return rand.Uint32()
}

func Test_RTRSerialNotify(t *testing.T) {
	id := uint16(time.Now().Unix())
	sn := randUint32()
	verifyRTRMessage(t, NewRTRSerialNotify(id, sn))
}

func Test_RTRSerialQuery(t *testing.T) {
	id := uint16(time.Now().Unix())
	sn := randUint32()
	verifyRTRMessage(t, NewRTRSerialQuery(id, sn))
}

func Test_RTRResetQuery(t *testing.T) {
	verifyRTRMessage(t, NewRTRResetQuery())
}

func Test_RTRCacheResponse(t *testing.T) {
	id := uint16(time.Now().Unix())
	verifyRTRMessage(t, NewRTRCacheResponse(id))
}

type rtrIPPrefixTestCase struct {
	pString string
	pLen    uint8
	mLen    uint8
	asn     uint32
	flags   uint8
}

var rtrIPPrefixTestCases = []rtrIPPrefixTestCase{
	{"192.168.0.0", 16, 32, 65001, ANNOUNCEMENT},
	{"192.168.0.0", 16, 32, 65001, WITHDRAWAL},
	{"2001:db8::", 32, 128, 65001, ANNOUNCEMENT},
	{"2001:db8::", 32, 128, 65001, WITHDRAWAL},
	{"::ffff:0.0.0.0", 96, 128, 65001, ANNOUNCEMENT},
	{"::ffff:0.0.0.0", 96, 128, 65001, WITHDRAWAL},
}

func Test_RTRIPPrefix(t *testing.T) {
	for i := range rtrIPPrefixTestCases {
		test := &rtrIPPrefixTestCases[i]
		addr := net.ParseIP(test.pString)
		verifyRTRMessage(t, NewRTRIPPrefix(addr, test.pLen, test.mLen, test.asn, test.flags))
	}
}

func Test_RTREndOfData(t *testing.T) {
	id := uint16(time.Now().Unix())
	sn := randUint32()
	verifyRTRMessage(t, NewRTREndOfData(id, sn))
}

func Test_RTRCacheReset(t *testing.T) {
	verifyRTRMessage(t, NewRTRCacheReset())
}

func Test_RTRErrorReport(t *testing.T) {
	errPDU, _ := NewRTRResetQuery().Serialize()
	errText1 := []byte("Couldn't send CacheResponce PDU")
	errText2 := []byte("Wrong Length of PDU: 10 bytes")

	// See 5.10 ErrorReport in RFC6810
	// when it doesn't have both "erroneous PDU" and "Arbitrary Text"
	verifyRTRMessage(t, NewRTRErrorReport(NO_DATA_AVAILABLE, nil, nil))

	// when it has "erroneous PDU"
	verifyRTRMessage(t, NewRTRErrorReport(UNSUPPORTED_PROTOCOL_VERSION, errPDU, nil))

	// when it has "ArbitaryText"
	verifyRTRMessage(t, NewRTRErrorReport(INTERNAL_ERROR, nil, errText1))

	// when it has both "erroneous PDU" and "Arbitrary Text"
	verifyRTRMessage(t, NewRTRErrorReport(CORRUPT_DATA, errPDU, errText2))
}
