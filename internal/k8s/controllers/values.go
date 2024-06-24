// SPDX-License-Identifier:Apache-2.0

package controllers

import "errors"

// SyncState is the result of calling synchronization callbacks.
type SyncState int

const (
	// The update was processed successfully.
	SyncStateSuccess SyncState = iota
	// The update caused a transient error, the k8s client should
	// retry later.
	SyncStateError
	// The update was accepted, but requires reprocessing all watched
	// services.
	SyncStateReprocessAll
	// The update caused a non transient error, the k8s client should
	// just report and giveup.
	SyncStateErrorNoRetry
)

type NeedEndPoints int

const (
	NoNeed NeedEndPoints = iota
	Endpoints
	EndpointSlices
)

const (
	LabelAnnounceNode     = "metallb.io/node"
	LabelServiceName      = "metallb.io/service-name"
	LabelServiceNamespace = "metallb.io/service-namespace"
)

var errRetry = errors.New("event handling failed, retrying")
