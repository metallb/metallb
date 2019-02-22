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

package api

import (
	"fmt"
	"strings"
	"time"

	"github.com/gogo/protobuf/proto"

	"github.com/ligato/vpp-agent/plugins/kvscheduler/internal/utils"
)

// TxnType differentiates between NB transaction, retry of failed operations and
// SB notification. Once queued, all three different operations are classified
// as transactions, only with different parameters.
type TxnType int

const (
	// SBNotification is notification from southbound.
	SBNotification TxnType = iota

	// NBTransaction is transaction from northbound.
	NBTransaction

	// RetryFailedOps is a transaction re-trying failed operations from previous
	// northbound transaction.
	RetryFailedOps
)

// String returns human-readable string representation of the transaction type.
func (t TxnType) String() string {
	switch t {
	case SBNotification:
		return "SB notification"
	case NBTransaction:
		return "NB transaction"
	case RetryFailedOps:
		return "RETRY"
	}
	return "UNKNOWN"
}

// RecordedTxn is used to record executed transaction.
type RecordedTxn struct {
	PreRecord bool // not yet fully recorded, only args + plan + pre-processing errors

	// timestamps
	Start time.Time
	Stop  time.Time

	// arguments
	SeqNum       uint64
	TxnType      TxnType
	ResyncType   ResyncType
	Description  string
	RetryForTxn  uint64
	RetryAttempt int
	Values       []RecordedKVPair

	// operations
	Planned  RecordedTxnOps
	Executed RecordedTxnOps
}

// RecordedTxnOp is used to record executed/planned transaction operation.
type RecordedTxnOp struct {
	// identification
	Operation TxnOperation
	Key       string

	// changes
	PrevValue proto.Message
	NewValue  proto.Message
	PrevState ValueState
	NewState  ValueState
	PrevErr   error
	NewErr    error
	NOOP      bool

	// flags
	IsDerived  bool
	IsProperty bool
	IsRevert   bool
	IsRetry    bool
	IsRecreate bool
}

// RecordedKVPair is used to record key-value pair.
type RecordedKVPair struct {
	Key    string
	Value  proto.Message
	Origin ValueOrigin
}

// RecordedTxnOps is a list of recorded executed/planned transaction operations.
type RecordedTxnOps []*RecordedTxnOp

// RecordedTxns is a list of recorded transactions.
type RecordedTxns []*RecordedTxn

// String returns a *multi-line* human-readable string representation of recorded transaction.
func (txn *RecordedTxn) String() string {
	return txn.StringWithOpts(false, false, 0)
}

// StringWithOpts allows to format string representation of recorded transaction.
func (txn *RecordedTxn) StringWithOpts(resultOnly, verbose bool, indent int) string {
	var str string
	indent1 := strings.Repeat(" ", indent)
	indent2 := strings.Repeat(" ", indent+4)
	indent3 := strings.Repeat(" ", indent+8)

	if !resultOnly {
		// transaction arguments
		str += indent1 + "* transaction arguments:\n"
		str += indent2 + fmt.Sprintf("- seq-num: %d\n", txn.SeqNum)
		if txn.TxnType == NBTransaction && txn.ResyncType != NotResync {
			ResyncType := "Full Resync"
			if txn.ResyncType == DownstreamResync {
				ResyncType = "SB Sync"
			}
			if txn.ResyncType == UpstreamResync {
				ResyncType = "NB Sync"
			}
			str += indent2 + fmt.Sprintf("- type: %s, %s\n", txn.TxnType.String(), ResyncType)
		} else {
			if txn.TxnType == RetryFailedOps {
				str += indent2 + fmt.Sprintf("- type: %s (for txn %d, attempt #%d)\n",
					txn.TxnType.String(), txn.RetryForTxn, txn.RetryAttempt)
			} else {
				str += indent2 + fmt.Sprintf("- type: %s\n", txn.TxnType.String())
			}
		}
		if txn.Description != "" {
			descriptionLines := strings.Split(txn.Description, "\n")
			for idx, line := range descriptionLines {
				if idx == 0 {
					str += indent2 + fmt.Sprintf("- Description: %s\n", line)
				} else {
					str += indent3 + fmt.Sprintf("%s\n", line)
				}
			}
		}
		if txn.ResyncType == DownstreamResync {
			goto printOps
		}
		if len(txn.Values) == 0 {
			str += indent2 + fmt.Sprintf("- values: NONE\n")
		} else {
			str += indent2 + fmt.Sprintf("- values:\n")
		}
		for _, kv := range txn.Values {
			if txn.ResyncType != NotResync && kv.Origin == FromSB {
				// do not print SB values updated during resync
				continue
			}
			str += indent3 + fmt.Sprintf("- key: %s\n", kv.Key)
			str += indent3 + fmt.Sprintf("  value: %s\n", utils.ProtoToString(kv.Value))
		}

	printOps:
		// planned operations
		str += indent1 + "* planned operations:\n"
		str += txn.Planned.StringWithOpts(verbose, indent+4)
	}

	if !txn.PreRecord {
		if len(txn.Executed) == 0 {
			str += indent1 + "* executed operations:\n"
		} else {
			str += indent1 + fmt.Sprintf("* executed operations (%s - %s, duration = %s):\n",
				txn.Start.String(), txn.Stop.String(), txn.Stop.Sub(txn.Start).String())
		}
		str += txn.Executed.StringWithOpts(verbose, indent+4)
	}

	return str
}

// String returns a *multi-line* human-readable string representation of a recorded
// transaction operation.
func (op *RecordedTxnOp) String() string {
	return op.StringWithOpts(0, false, 0)
}

// StringWithOpts allows to format string representation of a transaction operation.
func (op *RecordedTxnOp) StringWithOpts(index int, verbose bool, indent int) string {
	var str string
	indent1 := strings.Repeat(" ", indent)
	indent2 := strings.Repeat(" ", indent+4)

	var flags []string
	// operation flags
	if op.IsDerived && !op.IsProperty {
		flags = append(flags, "DERIVED")
	}
	if op.IsProperty {
		flags = append(flags, "PROPERTY")
	}
	if op.NOOP {
		flags = append(flags, "NOOP")
	}
	if op.IsRevert && !op.IsProperty {
		flags = append(flags, "REVERT")
	}
	if op.IsRetry && !op.IsProperty {
		flags = append(flags, "RETRY")
	}
	if op.IsRecreate {
		flags = append(flags, "RECREATE")
	}
	// value state transition
	//  -> RETRIEVED
	if op.NewState == ValueState_RETRIEVED {
		flags = append(flags, "RETRIEVED")
	}
	if op.PrevState == ValueState_RETRIEVED && op.PrevState != op.NewState {
		flags = append(flags, "WAS-RETRIEVED")
	}
	//  -> UNIMPLEMENTED
	if op.NewState == ValueState_UNIMPLEMENTED {
		flags = append(flags, "UNIMPLEMENTED")
	}
	if op.PrevState == ValueState_UNIMPLEMENTED && op.PrevState != op.NewState {
		flags = append(flags, "WAS-UNIMPLEMENTED")
	}
	//  -> REMOVED / MISSING
	if op.PrevState == ValueState_REMOVED && !op.IsRecreate {
		flags = append(flags, "ALREADY-REMOVED")
	}
	if op.PrevState == ValueState_MISSING {
		if op.NewState == ValueState_REMOVED {
			flags = append(flags, "ALREADY-MISSING")
		} else {
			flags = append(flags, "WAS-MISSING")
		}
	}
	//  -> FOUND
	if op.PrevState == ValueState_FOUND {
		flags = append(flags, "FOUND")
	}
	//  -> PENDING
	if op.PrevState == ValueState_PENDING {
		if op.NewState == ValueState_PENDING {
			flags = append(flags, "STILL-PENDING")
		} else {
			flags = append(flags, "WAS-PENDING")
		}
	} else {
		if op.NewState == ValueState_PENDING {
			flags = append(flags, "IS-PENDING")
		}
	}
	//  -> FAILED / INVALID
	if op.PrevState == ValueState_FAILED {
		if op.NewState == ValueState_FAILED {
			flags = append(flags, "STILL-FAILING")
		} else if op.NewState == ValueState_CONFIGURED {
			flags = append(flags, "FIXED")
		}
	} else {
		if op.NewState == ValueState_FAILED {
			flags = append(flags, "FAILED")
		}
	}
	if op.PrevState == ValueState_INVALID {
		if op.NewState == ValueState_INVALID {
			flags = append(flags, "STILL-INVALID")
		} else if op.NewState == ValueState_CONFIGURED {
			flags = append(flags, "FIXED")
		}
	} else {
		if op.NewState == ValueState_INVALID {
			flags = append(flags, "INVALID")
		}
	}

	if index > 0 {
		if len(flags) == 0 {
			str += indent1 + fmt.Sprintf("%d. %s:\n", index, op.Operation.String())
		} else {
			str += indent1 + fmt.Sprintf("%d. %s %v:\n", index, op.Operation.String(), flags)
		}
	} else {
		if len(flags) == 0 {
			str += indent1 + fmt.Sprintf("%s:\n", op.Operation.String())
		} else {
			str += indent1 + fmt.Sprintf("%s %v:\n", op.Operation.String(), flags)
		}
	}

	str += indent2 + fmt.Sprintf("- key: %s\n", op.Key)
	if op.Operation == TxnOperation_MODIFY {
		str += indent2 + fmt.Sprintf("- prev-value: %s \n", utils.ProtoToString(op.PrevValue))
		str += indent2 + fmt.Sprintf("- new-value: %s \n", utils.ProtoToString(op.NewValue))
	}
	if op.Operation == TxnOperation_DELETE {
		str += indent2 + fmt.Sprintf("- value: %s \n", utils.ProtoToString(op.PrevValue))
	}
	if op.Operation == TxnOperation_ADD {
		str += indent2 + fmt.Sprintf("- value: %s \n", utils.ProtoToString(op.NewValue))
	}
	if op.PrevErr != nil {
		str += indent2 + fmt.Sprintf("- prev-error: %s\n", utils.ErrorToString(op.PrevErr))
	}
	if op.NewErr != nil {
		str += indent2 + fmt.Sprintf("- error: %s\n", utils.ErrorToString(op.NewErr))
	}
	if verbose {
		str += indent2 + fmt.Sprintf("- prev-state: %s \n", op.PrevState.String())
		str += indent2 + fmt.Sprintf("- new-state: %s \n", op.NewState.String())
	}

	return str
}

// String returns a *multi-line* human-readable string representation of transaction
// operations.
func (ops RecordedTxnOps) String() string {
	return ops.StringWithOpts(false, 0)
}

// StringWithOpts allows to format string representation of transaction operations.
func (ops RecordedTxnOps) StringWithOpts(verbose bool, indent int) string {
	if len(ops) == 0 {
		return strings.Repeat(" ", indent) + "<NONE>\n"
	}

	var str string
	for idx, op := range ops {
		str += op.StringWithOpts(idx+1, verbose, indent)
	}
	return str
}

// String returns a *multi-line* human-readable string representation of a transaction
// list.
func (txns RecordedTxns) String() string {
	return txns.StringWithOpts(false, false, 0)
}

// StringWithOpts allows to format string representation of a transaction list.
func (txns RecordedTxns) StringWithOpts(resultOnly, verbose bool, indent int) string {
	if len(txns) == 0 {
		return strings.Repeat(" ", indent) + "<NONE>\n"
	}

	var str string
	for idx, txn := range txns {
		str += strings.Repeat(" ", indent) + fmt.Sprintf("Transaction #%d:\n", txn.SeqNum)
		str += txn.StringWithOpts(resultOnly, verbose, indent+4)
		if idx < len(txns)-1 {
			str += "\n"
		}
	}
	return str
}
