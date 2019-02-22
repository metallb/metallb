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
	"fmt"
	"sort"
	"strings"
	"time"

	kvs "github.com/ligato/vpp-agent/plugins/kvscheduler/api"
	"github.com/ligato/vpp-agent/plugins/kvscheduler/internal/graph"
	"github.com/ligato/vpp-agent/plugins/kvscheduler/internal/utils"
)

// GetTransactionHistory returns history of transactions started within the specified
// time window, or the full recorded history if the timestamps are zero values.
func (s *Scheduler) GetTransactionHistory(since, until time.Time) (history kvs.RecordedTxns) {
	s.historyLock.Lock()
	defer s.historyLock.Unlock()

	if !since.IsZero() && !until.IsZero() && until.Before(since) {
		// invalid time window
		return
	}

	lastBefore := -1
	firstAfter := len(s.txnHistory)

	if !since.IsZero() {
		for ; lastBefore+1 < len(s.txnHistory); lastBefore++ {
			if !s.txnHistory[lastBefore+1].Start.Before(since) {
				break
			}
		}
	}

	if !until.IsZero() {
		for ; firstAfter > 0; firstAfter-- {
			if !s.txnHistory[firstAfter-1].Start.After(until) {
				break
			}
		}
	}

	return s.txnHistory[lastBefore+1 : firstAfter]
}

// GetRecordedTransaction returns record of a transaction referenced by the sequence number.
func (s *Scheduler) GetRecordedTransaction(SeqNum uint64) (txn *kvs.RecordedTxn) {
	s.historyLock.Lock()
	defer s.historyLock.Unlock()

	for _, txn := range s.txnHistory {
		if txn.SeqNum == SeqNum {
			return txn
		}
	}
	return nil
}

// preRecordTxnOp prepares txn operation record - fills attributes that we can even
// before executing the operation.
func (s *Scheduler) preRecordTxnOp(args *applyValueArgs, node graph.Node) *kvs.RecordedTxnOp {
	prevOrigin := getNodeOrigin(node)
	if prevOrigin == kvs.UnknownOrigin {
		// new value
		prevOrigin = args.kv.origin
	}
	_, prevErr := getNodeError(node)
	return &kvs.RecordedTxnOp{
		Key:         args.kv.key,
		PrevValue:   utils.RecordProtoMessage(node.GetValue()),
		NewValue:    utils.RecordProtoMessage(args.kv.value),
		PrevState:   getNodeState(node),
		PrevErr:     prevErr,
		IsDerived:   args.isDerived,
		IsProperty:  args.isDerived && s.registry.GetDescriptorForKey(args.kv.key) == nil,
		IsRevert:    args.kv.isRevert,
		IsRetry:     args.isRetry,
	}
}

// preRecordTransaction logs transaction arguments + plan before execution to
// persist some information in case there is a crash during execution.
func (s *Scheduler) preRecordTransaction(txn *transaction, planned kvs.RecordedTxnOps) *kvs.RecordedTxn {
	// allocate new transaction record
	record := &kvs.RecordedTxn{
		PreRecord: true,
		SeqNum:    txn.seqNum,
		TxnType:   txn.txnType,
		Planned:   planned,
	}
	if txn.txnType == kvs.NBTransaction {
		record.ResyncType = txn.nb.resyncType
		record.Description = txn.nb.description
	}
	if txn.txnType == kvs.RetryFailedOps {
		record.RetryForTxn = txn.retry.txnSeqNum
		record.RetryAttempt = txn.retry.attempt
	}

	// build header for the log
	var downstreamResync bool
	txnInfo := fmt.Sprintf("%s", txn.txnType.String())
	if txn.txnType == kvs.NBTransaction && txn.nb.resyncType != kvs.NotResync {
		ResyncType := "Full Resync"
		if txn.nb.resyncType == kvs.DownstreamResync {
			ResyncType = "SB Sync"
			downstreamResync = true
		}
		if txn.nb.resyncType == kvs.UpstreamResync {
			ResyncType = "NB Sync"
		}
		txnInfo = fmt.Sprintf("%s (%s)", txn.txnType.String(), ResyncType)
	}

	// record values sorted alphabetically by keys
	if !downstreamResync {
		for _, kv := range txn.values {
			record.Values = append(record.Values, kvs.RecordedKVPair{
				Key:    kv.key,
				Value:  utils.RecordProtoMessage(kv.value),
				Origin: kv.origin,
			})
		}
		sort.Slice(record.Values, func(i, j int) bool {
			return record.Values[i].Key < record.Values[j].Key
		})
	}

	// send to the log
	var buf strings.Builder
	buf.WriteString("+======================================================================================================================+\n")
	msg := fmt.Sprintf("Transaction #%d", record.SeqNum)
	n := 115 - len(msg)
	buf.WriteString(fmt.Sprintf("| %s %"+fmt.Sprint(n)+"s |\n", msg, txnInfo))
	buf.WriteString("+======================================================================================================================+\n")
	buf.WriteString(record.StringWithOpts(false, false, 2))
	fmt.Println(buf.String())

	return record
}

// recordTransaction records the finalized transaction (log + in-memory).
func (s *Scheduler) recordTransaction(txnRecord *kvs.RecordedTxn, executed kvs.RecordedTxnOps, start, stop time.Time) {
	txnRecord.PreRecord = false
	txnRecord.Start = start
	txnRecord.Stop = stop
	txnRecord.Executed = executed

	var buf strings.Builder
	buf.WriteString("o----------------------------------------------------------------------------------------------------------------------o\n")
	buf.WriteString(txnRecord.StringWithOpts(true, false, 2))
	buf.WriteString("x----------------------------------------------------------------------------------------------------------------------x\n")
	msg := fmt.Sprintf("#%d", txnRecord.SeqNum)
	msg2 := fmt.Sprintf("took %v", stop.Sub(start).Round(time.Millisecond))
	buf.WriteString(fmt.Sprintf("x %s %"+fmt.Sprint(115-len(msg))+"s x\n", msg, msg2))
	buf.WriteString("x----------------------------------------------------------------------------------------------------------------------x\n")
	fmt.Println(buf.String())

	// add transaction record into the history
	if s.config.RecordTransactionHistory {
		s.historyLock.Lock()
		s.txnHistory = append(s.txnHistory, txnRecord)
		s.historyLock.Unlock()
	}
}

// transactionHistoryTrimming runs in a separate go routine and periodically removes
// transaction records too old to keep (by the configuration).
func (s *Scheduler) transactionHistoryTrimming() {
	defer s.wg.Done()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-time.After(txnHistoryTrimmingPeriod):
			s.historyLock.Lock()
			now := time.Now()
			ageLimit := time.Duration(s.config.TransactionHistoryAgeLimit) * time.Minute
			initPeriod := time.Duration(s.config.PermanentlyRecordedInitPeriod) * time.Minute
			var i, j int // i = first after init period, j = first after init period to keep
			for i = 0; i < len(s.txnHistory); i++ {
				sinceStart := s.txnHistory[i].Start.Sub(s.startTime)
				if sinceStart > initPeriod {
					break
				}
			}
			for j = i; j < len(s.txnHistory); j++ {
				elapsed := now.Sub(s.txnHistory[j].Stop)
				if elapsed <= ageLimit {
					break
				}
			}
			if j > i {
				copy(s.txnHistory[i:], s.txnHistory[j:])
				newLen := len(s.txnHistory) - (j - i)
				for k := newLen; k < len(s.txnHistory); k++ {
					s.txnHistory[k] = nil
				}
				s.txnHistory = s.txnHistory[:newLen]
			}
			s.historyLock.Unlock()
		}
	}
}
