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

package grpcsync

import (
	"errors"
	"io"
	"strings"

	"github.com/ligato/cn-infra/datasync/syncbase/msg"
	"github.com/ligato/cn-infra/logging/logrus"
	"golang.org/x/net/context"
)

// NewDataMsgServiceServer creates a new instance of DataMsgServiceServer.
func NewDataMsgServiceServer(adapter *Adapter) *DataMsgServiceServer {
	return &DataMsgServiceServer{adapter: adapter}
}

// DataMsgServiceServer is //TODO
type DataMsgServiceServer struct {
	adapter *Adapter
}

// DataChanges propagates the events in the stream to go channels of registered plugins.
func (s *DataMsgServiceServer) DataChanges(stream msg.DataMsgService_DataChangesServer) error {
	for {
		chng, err := stream.Recv()

		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		for _, sub := range s.adapter.base.Subscriptions() {
			for _, keyPrefix := range sub.KeyPrefixes {
				if strings.HasPrefix(chng.Key, keyPrefix) {
					sub.ChangeChan <- msg.NewChangeWatchResp(chng, func(err2 error) {
						err = stream.Send(&msg.DataChangeReply{Key: chng.Key, OperationType: chng.OperationType,
							Result: 0 /*TODO VPP Result*/})
						if err != nil {
							logrus.DefaultLogger().Error(err) //Not able to propagate it somewhere else
						}
					})
				}
			}
		}
	}
}

// DataResyncs propagates the events in the stream to go channels of registered plugins.
func (s *DataMsgServiceServer) DataResyncs(ctx context.Context, req *msg.DataResyncRequests) (
	*msg.DataResyncReplies, error) {
	resyncs := req.GetDataResyncs()
	if resyncs != nil {
		//TODO propagate event like in Kafka transport

		/*localtxn := syncbase.NewLocalBytesTxn(s.adapter.localtransp.PropagateBytesResync)
		if len(resyncs) == 0 {
			log.Debug("received empty resync => DELETE ALL")
		}
		for _, chReq := range resyncs {
			//TODO chReq.ContentType
			localtxn.Put(chReq.Key, chReq.Content)
		}
		err := localtxn.Commit()*/
		var err error
		if err != nil {
			return &msg.DataResyncReplies{MsgId: replySeq(), Error: &msg.Error{Message: err.Error()} /*TODO all other fields*/}, err
		}

		return &msg.DataResyncReplies{MsgId: replySeq() /*TODO all other fields*/}, nil
	}

	err := errors.New("unexpected place - nil resyncs")
	return &msg.DataResyncReplies{MsgId: replySeq(), Error: &msg.Error{Message: err.Error()} /*TODO all other fields*/}, err
}

func replySeq() *msg.Seq {
	return &msg.Seq{} //TODO !!!
}

// Ping checks the connectivity/ measures the minimal transport latency.
func (s *DataMsgServiceServer) Ping(ctx context.Context, req *msg.PingRequest) (*msg.PingReply, error) {
	return &msg.PingReply{Message: "it works"}, nil
}
