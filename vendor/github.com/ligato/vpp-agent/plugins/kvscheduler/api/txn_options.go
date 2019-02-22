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
	"context"
	"time"
)

type schedulerCtxKey int

const (
	// resyncCtxKey is a key under which *resync* txn option is stored
	// into the context.
	resyncCtxKey schedulerCtxKey = iota

	// nonBlockingTxnCtxKey is a key under which *non-blocking* txn option is
	// stored into the context.
	nonBlockingTxnCtxKey

	// retryCtxKey is a key under which *retry* txn option is stored into
	// the context.
	retryCtxKey

	// revertCtxKey is a key under which *revert* txn option is stored into
	// the context.
	revertCtxKey

	// txnDescriptionKey is a key under which transaction description is stored
	// into the context.
	txnDescriptionKey
)

// modifiable default parameters for the *retry* txn option
var (
	// DefaultRetryPeriod delays first retry by one second.
	DefaultRetryPeriod = time.Second

	// DefaultRetryMaxCount limits the number of retries to 3 attempts at maximum.
	DefaultRetryMaxCount = 3

	// DefaultRetryBackoff enables exponential back-off for retry delay.
	DefaultRetryBackoff = true
)

/* Full-Resync */

// resyncOpt represents the *resync* transaction option.
type resyncOpt struct {
	resyncType       ResyncType
	verboseSBRefresh bool
}

// ResyncType is one of: Upstream, Downstream, Full.
type ResyncType int

const (
	// NotResync is the default value for ResyncType, used when resync is actually
	// not enabled.
	NotResync ResyncType = iota

	// FullResync resynchronizes the agent with both SB and NB.
	FullResync

	// UpstreamResync resynchronizes the agent with NB.
	// It can be used by NB in situations when fully re-calculating the desired
	// state is far easier or more efficient that to determine the minimal difference
	// that needs to be applied to reach that state.
	// The agent's view of SB is not refreshed, instead it is expected to be up-to-date.
	UpstreamResync

	// DownstreamResync resynchronizes the agent with SB.
	// In this case it is assumed that the state required by NB is up-to-date
	// (transaction should be empty) and only the agent's view of SB is refreshed
	// and any discrepancies are acted upon.
	DownstreamResync
)

// WithResync prepares context for transaction that, based on the resync type,
// will trigger resync between the configuration states of NB, the agent and SB.
// For DownstreamResync the transaction should be empty, otherwise it should
// carry non-NIL values - existing NB values not included in the transaction
// are automatically removed.
// When <verboseSBRefresh> is enabled, the refreshed state of SB will be printed
// into stdout. The argument is irrelevant for UpstreamResync, where SB state is
// not refreshed.
func WithResync(ctx context.Context, resyncType ResyncType, verboseSBRefresh bool) context.Context {
	return context.WithValue(ctx, resyncCtxKey, &resyncOpt{
		resyncType:       resyncType,
		verboseSBRefresh: verboseSBRefresh,
	})
}

// IsResync returns true if the transaction context is configured to trigger resync.
func IsResync(ctx context.Context) (resyncType ResyncType, verboseSBRefresh bool) {
	resyncArgs, isResync := ctx.Value(resyncCtxKey).(*resyncOpt)
	if !isResync {
		return NotResync, false
	}
	return resyncArgs.resyncType, resyncArgs.verboseSBRefresh
}

/* Non-blocking Txn */

// nonBlockingTxnOpt represents the *non-blocking* transaction option.
type nonBlockingTxnOpt struct {
	// no attributes
}

// WithoutBlocking prepares context for transaction that should be scheduled
// for execution without blocking the caller of the Commit() method.
// By default, commit is blocking.
func WithoutBlocking(ctx context.Context) context.Context {
	return context.WithValue(ctx, nonBlockingTxnCtxKey, &nonBlockingTxnOpt{})
}

// IsNonBlockingTxn returns true if transaction context is configured for
// non-blocking Commit.
func IsNonBlockingTxn(ctx context.Context) bool {
	_, nonBlocking := ctx.Value(nonBlockingTxnCtxKey).(*nonBlockingTxnOpt)
	return nonBlocking
}

/* Retry */

// RetryOpt represents the *retry* transaction option.
type RetryOpt struct {
	Period     time.Duration
	MaxCount   int
	ExpBackoff bool
}

// WithRetry prepares context for transaction for which the scheduler will retry
// any (retriable) failed operations after given <period>. If <expBackoff>
// is enabled, every failed retry will double the next delay. Non-zero <maxCount>
// limits the maximum number of retries the scheduler will execute.
// Can be combined with revert - even failed revert operations will be re-tried.
// By default, the scheduler will not automatically retry failed operations.
func WithRetry(ctx context.Context, period time.Duration, maxCount int, expBackoff bool) context.Context {
	return context.WithValue(ctx, retryCtxKey, &RetryOpt{
		Period:     period,
		MaxCount:   maxCount,
		ExpBackoff: expBackoff,
	})
}

// WithRetryDefault is a specialization of WithRetry, where retry parameters
// are set to default values.
func WithRetryDefault(ctx context.Context) context.Context {
	return context.WithValue(ctx, retryCtxKey, &RetryOpt{
		Period:     DefaultRetryPeriod,
		MaxCount:   DefaultRetryMaxCount,
		ExpBackoff: DefaultRetryBackoff,
	})
}

// WithRetryMaxCount is a specialization of WithRetry, where <period> and <expBackoff>
// are set to default values and the maximum number of retries can be customized.
func WithRetryMaxCount(ctx context.Context, maxCount int) context.Context {
	return context.WithValue(ctx, retryCtxKey, &RetryOpt{
		Period:     DefaultRetryPeriod,
		MaxCount:   maxCount,
		ExpBackoff: DefaultRetryBackoff,
	})
}

// IsWithRetry returns true if transaction context is configured to allow retry,
// including the option parameters, or nil if retry is not enabled.
func IsWithRetry(ctx context.Context) (retryArgs *RetryOpt, withRetry bool) {
	retryArgs, withRetry = ctx.Value(retryCtxKey).(*RetryOpt)
	return
}

/* Revert */

// revertOpt represents the *revert* transaction option.
type revertOpt struct {
	// no attributes
}

// WithRevert prepares context for transaction that will be reverted if any
// of its operations fails.
// By default, the scheduler executes transactions in a best-effort mode - even
// in the case of an error it will keep the effects of successful operations.
func WithRevert(ctx context.Context) context.Context {
	return context.WithValue(ctx, revertCtxKey, &revertOpt{})
}

// IsWithRevert returns true if the transaction context is configured
// to revert transaction if any of its operations fails.
func IsWithRevert(ctx context.Context) bool {
	_, isWithRevert := ctx.Value(revertCtxKey).(*revertOpt)
	return isWithRevert
}

/* Txn Description */

// txnDescriptionOpt represents the *txn-description* transaction option.
type txnDescriptionOpt struct {
	description string
}

// WithDescription prepares context for transaction that will have description
// provided.
// By default, transactions are without description.
func WithDescription(ctx context.Context, description string) context.Context {
	return context.WithValue(ctx, txnDescriptionKey, &txnDescriptionOpt{description: description})
}

// IsWithDescription returns true if the transaction context is configured
// to include transaction description.
func IsWithDescription(ctx context.Context) (description string, withDescription bool) {
	descriptionOpt, withDescription := ctx.Value(txnDescriptionKey).(*txnDescriptionOpt)
	if !withDescription {
		return "", false
	}
	return descriptionOpt.description, true
}
