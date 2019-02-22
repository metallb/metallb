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

package kvscheduler

import (
	"time"

	"github.com/ligato/cn-infra/logging"

	kvs "github.com/ligato/vpp-agent/plugins/kvscheduler/api"
)

// enqueueTxn adds transaction into the FIFO queue (channel) for execution.
func (s *Scheduler) enqueueTxn(txn *transaction) error {
	if txn.txnType == kvs.NBTransaction && txn.nb.isBlocking {
		select {
		case <-s.ctx.Done():
			return kvs.ErrClosedScheduler
		case s.txnQueue <- txn:
			return nil
		}
	}
	select {
	case <-s.ctx.Done():
		return kvs.ErrClosedScheduler
	case s.txnQueue <- txn:
		return nil
	default:
		return kvs.ErrTxnQueueFull
	}
}

// dequeueTxn pulls the oldest queued transaction.
func (s *Scheduler) dequeueTxn() (txn *transaction, canceled bool) {
	select {
	case <-s.ctx.Done():
		return nil, true
	case txn = <-s.txnQueue:
		return txn, false
	}
}

// enqueueRetry schedules retry for failed operations.
func (s *Scheduler) enqueueRetry(args *retryTxn) {
	go s.delayRetry(args)
}

// delayRetry postpones retry until a given time period has elapsed.
func (s *Scheduler) delayRetry(args *retryTxn) {
	s.wg.Add(1)
	defer s.wg.Done()

	select {
	case <-s.ctx.Done():
		return
	case <-time.After(args.delay):
		err := s.enqueueTxn(&transaction{txnType: kvs.RetryFailedOps, retry: args})
		if err != nil {
			s.Log.WithFields(logging.Fields{
				"txnSeqNum": args.txnSeqNum,
				"err":       err,
			}).Warn("Failed to enqueue re-try for failed operations")
			s.enqueueRetry(args) // try again with the same time period
		}
	}
}
