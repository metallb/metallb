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
	"errors"
	"fmt"
	"github.com/gogo/protobuf/proto"
	"strings"
)

var (
	// ErrCombinedDownstreamResyncWithChange is returned when transaction combines downstream-resync with data changes.
	ErrCombinedDownstreamResyncWithChange = errors.New("downstream resync combined with data changes in one transaction")

	// ErrRevertNotSupportedWithResync is returned when transaction combines resync with revert.
	ErrRevertNotSupportedWithResync = errors.New("it is not supported to combine resync with revert")

	// ErrClosedScheduler is returned when scheduler is closed during transaction execution.
	ErrClosedScheduler = errors.New("scheduler was closed")

	// ErrTxnWaitCanceled is returned when waiting for result of blocking transaction is canceled.
	ErrTxnWaitCanceled = errors.New("waiting for result of blocking transaction was canceled")

	// ErrTxnQueueFull is returned when the queue of pending transactions is full.
	ErrTxnQueueFull = errors.New("transaction queue is full")

	// ErrUnimplementedAdd is returned when NB transaction attempts to Add value
	// for which there is a descriptor, but Add operation is not implemented.
	ErrUnimplementedAdd = errors.New("Add operation is not implemented")

	// ErrUnimplementedDelete is returned when NB transaction attempts to Delete value
	// for which there is a descriptor, but Delete operation is not implemented.
	ErrUnimplementedDelete = errors.New("Delete operation is not implemented")

	// ErrUnimplementedModify is returned when NB transaction attempts to Modify value
	// for which there is a descriptor, but Modify operation is not implemented.
	ErrUnimplementedModify = errors.New("Modify operation is not implemented")
)

// ErrInvalidValueType is returned to scheduler by auto-generated descriptor adapter
// when value does not match expected type.
func ErrInvalidValueType(key string, value proto.Message) error {
	if key == "" {
		return fmt.Errorf("value (%s) has invalid type", value.String())
	}
	return fmt.Errorf("value (%s) has invalid type for key: %s", value.String(), key)
}

// ErrInvalidMetadataType is returned to scheduler by auto-generated descriptor adapter
// when value metadata does not match expected type.
func ErrInvalidMetadataType(key string) error {
	if key == "" {
		return errors.New("metadata has invalid type")
	}
	return fmt.Errorf("metadata has invalid type for key: %s", key)
}

/****************************** Transaction Error *****************************/

// TransactionError implements Error interface, wrapping all errors encountered
// during the processing of a single transaction.
type TransactionError struct {
	txnInitError error
	kvErrors     []KeyWithError
}

// NewTransactionError is a constructor for transaction error.
func NewTransactionError(txnInitError error, kvErrors []KeyWithError) *TransactionError {
	return &TransactionError{txnInitError: txnInitError, kvErrors: kvErrors}
}

// Error returns a string representation of all errors encountered during
// the transaction processing.
func (e *TransactionError) Error() string {
	if e == nil {
		return ""
	}
	if e.txnInitError != nil {
		return e.txnInitError.Error()
	}
	if len(e.kvErrors) > 0 {
		var kvErrMsgs []string
		for _, kvError := range e.kvErrors {
			kvErrMsgs = append(kvErrMsgs,
				fmt.Sprintf("%s (%v): %v", kvError.Key, kvError.TxnOperation, kvError.Error))
		}
		return fmt.Sprintf("failed key-value pairs: [%s]", strings.Join(kvErrMsgs, ", "))
	}
	return ""
}

// GetKVErrors returns errors for key-value pairs that failed to get applied.
func (e *TransactionError) GetKVErrors() (kvErrors []KeyWithError) {
	if e == nil {
		return kvErrors
	}
	return e.kvErrors
}

// GetTxnInitError returns error thrown during the transaction initialization.
// If the transaction initialization fails, the other stages of the transaction
// processing are not even started, therefore either GetTxnInitError or GetKVErrors
// may return some errors, but not both.
func (e *TransactionError) GetTxnInitError() error {
	if e == nil {
		return nil
	}
	return e.txnInitError
}

/******************************** Invalid Value *******************************/

// InvalidValueError can be used by descriptor for the Validate method to return
// validation error together with a list of invalid fields for further
// clarification.
type InvalidValueError struct {
	err           error
	invalidFields []string
}

// NewInvalidValueError is a constructor for invalid-value error.
func NewInvalidValueError(err error, invalidFields... string) *InvalidValueError {
	return &InvalidValueError{err: err, invalidFields: invalidFields}
}

// Error returns a string representation of all errors encountered during
// the transaction processing.
func (e *InvalidValueError) Error() string {
	if e == nil || e.err == nil {
		return ""
	}
	if len(e.invalidFields) == 0 {
		return e.err.Error()
	}
	if len(e.invalidFields) == 1 {
		return fmt.Sprintf("field %v is invalid: %v", e.invalidFields[0], e.err)
	}
	return fmt.Sprintf("fields %v are invalid: %v", e.invalidFields, e.err)
}

// GetValidationError returns internally stored validation error.
func (e *InvalidValueError) GetValidationError() error {
	return e.err
}

// GetInvalidFields returns internally stored slice of invalid fields.
func (e *InvalidValueError) GetInvalidFields() []string {
	return e.invalidFields
}

/***************************** Verification Failure ****************************/

type VerificationErrorType int

const (
	// ExpectedToExist marks verification error returned when configured (non-nil)
	// value is not found by the refresh.
	ExpectedToExist VerificationErrorType = iota

	// ExpectedToNotExist marks verification error returned when removed (nil)
	// value is found by the refresh to still exist.
	ExpectedToNotExist

	// NotEquivalent marks verification error returned when applied value is not
	// equivalent with the refreshed value.
	NotEquivalent
)

// VerificationError is returned by the scheduler for a transaction when an applied
// value does not match with the refreshed value.
type VerificationError struct {
	key     string
	errType VerificationErrorType
}

// NewVerificationError is constructor for a verification error.
func NewVerificationError(key string, errType VerificationErrorType) *VerificationError {
	return &VerificationError{key: key, errType: errType}
}

// Error returns a string representation of the error.
func (e *VerificationError) Error() string {
	switch e.errType {
	case ExpectedToExist:
		return "value is not actually configured"
	case ExpectedToNotExist:
		return "value is not actually removed"
	case NotEquivalent:
		return "applied value is not equivalent with the refreshed value"
	}
	return ""
}

// Key returns the key of the value for which the verification failed.
func (e *VerificationError) Key() string {
	return e.key
}

// Type returns the verification error type.
func (e *VerificationError) Type() VerificationErrorType {
	return e.errType
}