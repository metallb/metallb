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

package rpc

import (
	"context"
	"fmt"
	"testing"

	"github.com/ligato/cn-infra/logging/logrus"
	"github.com/ligato/vpp-agent/plugins/vpp/model/interfaces"
	"github.com/ligato/vpp-agent/plugins/vpp/model/rpc"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
)

type mockServer struct {
	grpc.ServerStream
	notifs []*rpc.NotificationsResponse
}

func (m *mockServer) Send(entry *rpc.NotificationsResponse) error {
	m.notifs = append(m.notifs, entry)
	return nil
}

func TestNotificationCases(t *testing.T) {
	for i, test := range []struct {
		inputN  int
		expectN int
	}{
		{inputN: 0, expectN: 0},
		{inputN: 1, expectN: 1},
		{inputN: bufferSize / 2, expectN: bufferSize / 2},
		{inputN: bufferSize, expectN: bufferSize},
		{inputN: bufferSize * 1.5, expectN: bufferSize},
	} {
		test := test
		t.Run(fmt.Sprintf("test-%v %+v", i, test), func(t *testing.T) {
			RegisterTestingT(t)

			mockSrv := &mockServer{}
			svc := NotificationSvc{log: logrus.DefaultLogger()}

			for i := 0; i < test.inputN; i++ {
				svc.updateNotifications(context.Background(), &interfaces.InterfaceNotification{
					Type: interfaces.InterfaceNotification_UPDOWN,
					State: &interfaces.InterfacesState_Interface{
						Name:    fmt.Sprintf("if%d", i),
						IfIndex: uint32(i),
					},
				})
			}

			from := &rpc.NotificationRequest{Idx: 0}
			Expect(svc.Get(from, mockSrv)).To(Succeed())
			Expect(mockSrv.notifs).To(HaveLen(test.expectN))
		})
	}
}

func TestNotifications(t *testing.T) {
	RegisterTestingT(t)

	svc := NotificationSvc{log: logrus.DefaultLogger()}

	from := &rpc.NotificationRequest{Idx: 0}
	mockSrv := &mockServer{}
	Expect(svc.Get(from, mockSrv)).To(Succeed())
	Expect(mockSrv.notifs).To(HaveLen(0))

	t.Logf("got %d notifs", len(mockSrv.notifs))
	for i, notif := range mockSrv.notifs {
		t.Logf(" #%d: %+v nextIndex: %v", i, notif.NIf, notif.NextIdx)
	}

	n := 1
	for ; n <= bufferSize/2; n++ {
		svc.updateNotifications(context.Background(), &interfaces.InterfaceNotification{
			Type: interfaces.InterfaceNotification_UPDOWN,
			State: &interfaces.InterfacesState_Interface{
				Name:    fmt.Sprintf("if%d", n),
				IfIndex: uint32(n),
			},
		})
	}

	from = &rpc.NotificationRequest{Idx: 0}
	mockSrv = &mockServer{}
	Expect(svc.Get(from, mockSrv)).To(Succeed())
	Expect(mockSrv.notifs).To(HaveLen(bufferSize / 2))

	t.Logf("got %d notifs", len(mockSrv.notifs))
	for i, notif := range mockSrv.notifs {
		t.Logf(" #%d: %+v nextIndex: %v", i, notif.NIf, notif.NextIdx)
	}

	for ; n <= bufferSize; n++ {
		svc.updateNotifications(context.Background(), &interfaces.InterfaceNotification{
			Type: interfaces.InterfaceNotification_UPDOWN,
			State: &interfaces.InterfacesState_Interface{
				Name:    fmt.Sprintf("if%d", n),
				IfIndex: uint32(n),
			},
		})
	}

	from = &rpc.NotificationRequest{Idx: mockSrv.notifs[len(mockSrv.notifs)-1].NextIdx}
	mockSrv = &mockServer{}
	Expect(svc.Get(from, mockSrv)).To(Succeed())
	Expect(mockSrv.notifs).To(HaveLen(bufferSize / 2))

	t.Logf("got %d notifs", len(mockSrv.notifs))
	for i, notif := range mockSrv.notifs {
		t.Logf(" #%d: %+v nextIndex: %v", i, notif.NIf, notif.NextIdx)
	}

	from = &rpc.NotificationRequest{Idx: 0}
	mockSrv = &mockServer{}
	Expect(svc.Get(from, mockSrv)).To(Succeed())
	Expect(mockSrv.notifs).To(HaveLen(bufferSize))

	t.Logf("got %d notifs", len(mockSrv.notifs))
	for i, notif := range mockSrv.notifs {
		t.Logf(" #%d: %+v nextIndex: %v", i, notif.NIf, notif.NextIdx)
	}

	for ; n <= bufferSize+bufferSize/2; n++ {
		svc.updateNotifications(context.Background(), &interfaces.InterfaceNotification{
			Type: interfaces.InterfaceNotification_UPDOWN,
			State: &interfaces.InterfacesState_Interface{
				Name:    fmt.Sprintf("if%d", n),
				IfIndex: uint32(n),
			},
		})
	}

	from = &rpc.NotificationRequest{Idx: 0}
	mockSrv = &mockServer{}
	Expect(svc.Get(from, mockSrv)).To(Succeed())
	Expect(mockSrv.notifs).To(HaveLen(bufferSize))

	t.Logf("got %d notifs", len(mockSrv.notifs))
	for i, notif := range mockSrv.notifs {
		t.Logf(" #%d: %+v nextIndex: %v", i, notif.NIf, notif.NextIdx)
	}
}
