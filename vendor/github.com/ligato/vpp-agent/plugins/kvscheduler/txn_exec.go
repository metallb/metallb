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
	"sort"

	"fmt"
	"github.com/ligato/cn-infra/logging"
	kvs "github.com/ligato/vpp-agent/plugins/kvscheduler/api"
	"github.com/ligato/vpp-agent/plugins/kvscheduler/internal/graph"
	"github.com/ligato/vpp-agent/plugins/kvscheduler/internal/utils"
	"strings"
)

// applyValueArgs collects all arguments to applyValue method.
type applyValueArgs struct {
	graphW  graph.RWAccess
	txn     *transaction
	kv      kvForTxn
	baseKey string

	isRetry bool
	dryRun  bool

	// set inside of the recursive chain of applyValue-s
	isUpdate  bool
	isDerived bool

	// handling of dependency cycles
	branch utils.KeySet
}

// executeTransaction executes pre-processed transaction.
// If <dry-run> is enabled, Validate/Add/Delete/Modify operations will not be executed
// and the graph will be returned to its original state at the end.
func (s *Scheduler) executeTransaction(txn *transaction, dryRun bool) (executed kvs.RecordedTxnOps) {
	if s.logGraphWalk {
		op := "execute transaction"
		if dryRun {
			op = "simulate transaction"
		}
		msg := fmt.Sprintf("%s (seqNum=%d)", op, txn.seqNum)
		fmt.Printf("%s %s\n", nodeVisitBeginMark, msg)
		defer fmt.Printf("%s %s\n", nodeVisitEndMark, msg)
	}
	downstreamResync := txn.txnType == kvs.NBTransaction && txn.nb.resyncType == kvs.DownstreamResync
	graphW := s.graph.Write(!downstreamResync)
	branch := utils.NewMapBasedKeySet() // branch of current recursive calls to applyValue used to handle cycles

	var revert bool
	prevValues := make([]kvs.KeyValuePair, 0, len(txn.values))
	// execute transaction either in best-effort mode or with revert on the first failure
	for _, kv := range txn.values {
		ops, prevValue, err := s.applyValue(
			&applyValueArgs{
				graphW:  graphW,
				txn:     txn,
				kv:      kv,
				baseKey: kv.key,
				dryRun:  dryRun,
				isRetry: txn.txnType == kvs.RetryFailedOps,
				branch:  branch,
			})
		executed = append(executed, ops...)
		prevValues = append(prevValues, kvs.KeyValuePair{})
		copy(prevValues[1:], prevValues)
		prevValues[0] = prevValue
		if err != nil {
			if txn.txnType == kvs.NBTransaction && txn.nb.revertOnFailure {
				// refresh failed value and trigger reverting
				failedKey := utils.NewSingletonKeySet(kv.key)
				s.refreshGraph(graphW, failedKey, nil, true)
				graphW.Save() // certainly not dry-run
				revert = true
				break
			}
		}
	}

	if revert {
		// record graph state in-between failure and revert
		graphW.Release()
		graphW = s.graph.Write(true)

		// revert back to previous values
		for _, kvPair := range prevValues {
			ops, _, _ := s.applyValue(
				&applyValueArgs{
					graphW: graphW,
					txn:    txn,
					kv: kvForTxn{
						key:      kvPair.Key,
						value:    kvPair.Value,
						origin:   kvs.FromNB,
						isRevert: true,
					},
					baseKey: kvPair.Key,
					dryRun:  dryRun,
					branch:  branch,
				})
			executed = append(executed, ops...)
		}
	}

	// get rid of uninteresting intermediate pending Add/Delete operations
	executed = s.compressTxnOps(executed)

	graphW.Release()
	return executed
}

// applyValue applies new value received from NB or SB.
// It returns the list of executed operations.
func (s *Scheduler) applyValue(args *applyValueArgs) (executed kvs.RecordedTxnOps, prevValue kvs.KeyValuePair, err error) {
	if s.logGraphWalk {
		endLog := s.logNodeVisit("applyValue", args)
		defer endLog()
	}
	// dependency cycle detection
	if cycle := args.branch.Has(args.kv.key); cycle {
		return executed, prevValue, err
	}
	args.branch.Add(args.kv.key)
	defer args.branch.Del(args.kv.key)

	// create new revision of the node for the given key-value pair
	node := args.graphW.SetNode(args.kv.key)

	// remember previous value for a potential revert
	prevValue = kvs.KeyValuePair{Key: node.GetKey(), Value: node.GetValue()}

	// remember previous value status to detect and notify about changes
	prevState := getNodeState(node)
	prevOp := getNodeLastOperation(node)
	prevErr := getNodeErrorString(node)
	prevDetails := getValueDetails(node)

	// prepare operation description - fill attributes that we can even before executing the operation
	txnOp := s.preRecordTxnOp(args, node)

	// determine the operation type
	if args.isUpdate {
		s.determineUpdateOperation(node, txnOp)
		if txnOp.Operation == kvs.TxnOperation_UNDEFINED {
			// nothing needs to be updated
			return
		}
	} else if args.kv.value == nil {
		txnOp.Operation = kvs.TxnOperation_DELETE
	} else if node.GetValue() == nil || !isNodeAvailable(node) {
		txnOp.Operation = kvs.TxnOperation_ADD
	} else {
		txnOp.Operation = kvs.TxnOperation_MODIFY
	}

	// remaining txnOp attributes to fill:
	//		NewState   bool
	//		NewErr     error
	//      NOOP       bool
	//      IsRecreate bool

	// update node flags
	prevUpdate := getNodeLastUpdate(node)
	lastUpdateFlag := &LastUpdateFlag{
		txnSeqNum: args.txn.seqNum,
		txnOp:     txnOp.Operation,
		value:     args.kv.value,
		revert:    args.kv.isRevert,
	}
	if args.txn.txnType == kvs.NBTransaction {
		lastUpdateFlag.retryEnabled = args.txn.nb.retryEnabled
		lastUpdateFlag.retryArgs = args.txn.nb.retryArgs
	} else if prevUpdate != nil {
		// inherit retry arguments from the last NB txn for this value
		lastUpdateFlag.retryEnabled = prevUpdate.retryEnabled
		lastUpdateFlag.retryArgs = prevUpdate.retryArgs
	}
	node.SetFlags(lastUpdateFlag)

	// if the value is already "broken" by this transaction, do not try to update
	// anymore, unless this is a revert
	// (needs to be refreshed first in the post-processing stage)
	if (prevState == kvs.ValueState_FAILED || prevState == kvs.ValueState_RETRYING) &&
		!args.kv.isRevert && prevUpdate != nil && prevUpdate.txnSeqNum == args.txn.seqNum {
		_, prevErr := getNodeError(node)
		return executed, prevValue, prevErr
	}

	// run selected operation
	switch txnOp.Operation {
	case kvs.TxnOperation_DELETE:
		executed, err = s.applyDelete(node, txnOp, args, args.isUpdate)
	case kvs.TxnOperation_ADD:
		executed, err = s.applyAdd(node, txnOp, args)
	case kvs.TxnOperation_MODIFY:
		executed, err = s.applyModify(node, txnOp, args)
	}

	// detect value state changes
	if !args.dryRun {
		nodeR := args.graphW.GetNode(args.kv.key)
		if prevUpdate == nil || prevState != getNodeState(nodeR) || prevOp != getNodeLastOperation(nodeR) ||
			prevErr != getNodeErrorString(nodeR) || !equalValueDetails(prevDetails, getValueDetails(nodeR)) {
			s.updatedStates.Add(args.baseKey)
		}
	}

	return executed, prevValue, err
}

// applyDelete removes value.
func (s *Scheduler) applyDelete(node graph.NodeRW, txnOp *kvs.RecordedTxnOp, args *applyValueArgs, pending bool) (executed kvs.RecordedTxnOps, err error) {
	if s.logGraphWalk {
		endLog := s.logNodeVisit("applyDelete", args)
		defer endLog()
	}
	if !args.dryRun {
		defer args.graphW.Save()
	}

	if node.GetValue() == nil {
		// remove value that does not exist => noop (do not even record)
		args.graphW.DeleteNode(args.kv.key)
		return executed, nil
	}

	// reflect removal in the graph at the return
	var (
		inheritedErr error
		retriableErr bool
	)
	defer func() {
		if inheritedErr != nil {
			// revert back to available, derived value failed instead
			node.DelFlags(UnavailValueFlagName)
			return
		}
		if err == nil {
			node.DelFlags(ErrorFlagName)
			if pending {
				// deleted due to missing dependencies
				txnOp.NewState = kvs.ValueState_PENDING
				s.updateNodeState(node, txnOp.NewState, args)
			} else {
				// removed by request
				txnOp.NewState = kvs.ValueState_REMOVED
				if args.isDerived {
					args.graphW.DeleteNode(args.kv.key)
				} else {
					s.updateNodeState(node, txnOp.NewState, args)
				}
			}
		} else {
			txnOp.NewErr = err
			txnOp.NewState = s.markFailedValue(node, args, err, retriableErr)
		}
		executed = append(executed, txnOp)
	}()

	if !isNodeAvailable(node) {
		// removing value that was pending => just update the state in the graph
		txnOp.NOOP = true
		return
	}

	// already mark as unavailable so that other nodes will not view it as satisfied
	// dependency during removal
	node.SetFlags(&UnavailValueFlag{})

	// remove derived values
	if !args.isDerived {
		var derivedVals []kvForTxn
		for _, derivedNode := range getDerivedNodes(node) {
			derivedVals = append(derivedVals, kvForTxn{
				key:      derivedNode.GetKey(),
				value:    nil, // delete
				origin:   args.kv.origin,
				isRevert: args.kv.isRevert,
			})
		}
		derExecs, inheritedErr := s.applyDerived(derivedVals, args, false)
		executed = append(executed, derExecs...)
		if inheritedErr != nil {
			err = inheritedErr
			return
		}
	}

	// update values that depend on this kv-pair
	executed = append(executed, s.runUpdates(node, args)...)

	// execute delete operation
	descriptor := s.registry.GetDescriptorForKey(node.GetKey())
	handler := &descriptorHandler{descriptor}
	if !args.dryRun && descriptor != nil {
		if args.kv.origin != kvs.FromSB {
			err = handler.delete(node.GetKey(), node.GetValue(), node.GetMetadata())
		}
		if err != nil {
			retriableErr = handler.isRetriableFailure(err)
		}
		if canNodeHaveMetadata(node) && descriptor.WithMetadata {
			node.SetMetadata(nil)
		}
	}
	return
}

// applyAdd adds new value which previously didn't exist or was unavailable.
func (s *Scheduler) applyAdd(node graph.NodeRW, txnOp *kvs.RecordedTxnOp, args *applyValueArgs) (executed kvs.RecordedTxnOps, err error) {
	if s.logGraphWalk {
		endLog := s.logNodeVisit("applyAdd", args)
		defer endLog()
	}
	if !args.dryRun {
		defer args.graphW.Save()
	}
	node.SetValue(args.kv.value)

	// get descriptor
	descriptor := s.registry.GetDescriptorForKey(args.kv.key)
	handler := &descriptorHandler{descriptor}
	if descriptor != nil {
		node.SetFlags(&DescriptorFlag{descriptor.Name})
		node.SetLabel(handler.keyLabel(args.kv.key))
	}

	// handle unimplemented value
	unimplemented := args.kv.origin == kvs.FromNB && !args.isDerived && descriptor == nil
	if unimplemented {
		if getNodeState(node) == kvs.ValueState_UNIMPLEMENTED {
			// already known
			return
		}
		node.SetFlags(&UnavailValueFlag{})
		node.DelFlags(ErrorFlagName)
		txnOp.NOOP = true
		txnOp.NewState = kvs.ValueState_UNIMPLEMENTED
		s.updateNodeState(node, txnOp.NewState, args)
		return kvs.RecordedTxnOps{txnOp}, nil
	}

	// mark derived value
	if args.isDerived {
		node.SetFlags(&DerivedFlag{baseKey: args.baseKey})
	}

	// validate value
	if !args.dryRun && args.kv.origin == kvs.FromNB {
		err = handler.validate(node.GetKey(), node.GetValue())
		if err != nil {
			node.SetFlags(&UnavailValueFlag{})
			txnOp.NewErr = err
			txnOp.NewState = kvs.ValueState_INVALID
			txnOp.NOOP = true
			s.updateNodeState(node, txnOp.NewState, args)
			node.SetFlags(&ErrorFlag{err: err, retriable: false})
			return kvs.RecordedTxnOps{txnOp}, err
		}
	}

	// apply new relations
	_, updateExecs, inheritedErr := s.applyNewRelations(node, handler, args)
	executed = append(executed, updateExecs...)
	if inheritedErr != nil {
		// error is not expected here, executed operations should be NOOPs
		err = inheritedErr
		return
	}

	derives := handler.derivedValues(node.GetKey(), node.GetValue())
	dependencies := handler.dependencies(node.GetKey(), node.GetValue())
	node.SetTargets(constructTargets(dependencies, derives))

	if !isNodeReady(node) {
		// if not ready, nothing to do
		node.SetFlags(&UnavailValueFlag{})
		node.DelFlags(ErrorFlagName)
		txnOp.NewState = kvs.ValueState_PENDING
		txnOp.NOOP = true
		s.updateNodeState(node, txnOp.NewState, args)
		return kvs.RecordedTxnOps{txnOp}, nil
	}

	// execute add operation
	if !args.dryRun && descriptor != nil {
		var metadata interface{}

		if args.kv.origin != kvs.FromSB {
			metadata, err = handler.add(node.GetKey(), node.GetValue())
		} else {
			// already added in SB
			metadata = args.kv.metadata
		}

		if err != nil {
			// add failed => assume the value is unavailable
			node.SetFlags(&UnavailValueFlag{})
			retriableErr := handler.isRetriableFailure(err)
			txnOp.NewErr = err
			txnOp.NewState = s.markFailedValue(node, args, err, retriableErr)
			return kvs.RecordedTxnOps{txnOp}, err
		}

		// add metadata to the map
		if canNodeHaveMetadata(node) && descriptor.WithMetadata {
			node.SetMetadataMap(descriptor.Name)
			node.SetMetadata(metadata)
		}
	}

	// finalize node and save before going to derived values + dependencies
	node.DelFlags(ErrorFlagName, UnavailValueFlagName)
	if args.kv.origin == kvs.FromSB {
		txnOp.NewState = kvs.ValueState_RETRIEVED
	} else {
		txnOp.NewState = kvs.ValueState_CONFIGURED
	}
	s.updateNodeState(node, txnOp.NewState, args)
	executed = append(executed, txnOp)
	if !args.dryRun {
		args.graphW.Save()
	}

	// update values that depend on this kv-pair
	executed = append(executed, s.runUpdates(node, args)...)

	// created derived values
	if !args.isDerived {
		var derivedVals []kvForTxn
		for _, derivedVal := range derives {
			derivedVals = append(derivedVals, kvForTxn{
				key:      derivedVal.Key,
				value:    derivedVal.Value,
				origin:   args.kv.origin,
				isRevert: args.kv.isRevert,
			})
		}
		derExecs, inheritedErr := s.applyDerived(derivedVals, args, true)
		executed = append(executed, derExecs...)
		if inheritedErr != nil {
			err = inheritedErr
		}
	}
	return
}

// applyModify applies new value to existing non-pending value.
func (s *Scheduler) applyModify(node graph.NodeRW, txnOp *kvs.RecordedTxnOp, args *applyValueArgs) (executed kvs.RecordedTxnOps, err error) {
	if s.logGraphWalk {
		endLog := s.logNodeVisit("applyModify", args)
		defer endLog()
	}
	if !args.dryRun {
		defer args.graphW.Save()
	}

	// save the new value
	prevValue := node.GetValue()
	node.SetValue(args.kv.value)

	// validate new value
	descriptor := s.registry.GetDescriptorForKey(args.kv.key)
	handler := &descriptorHandler{descriptor}
	if !args.dryRun && args.kv.origin == kvs.FromNB {
		err = handler.validate(node.GetKey(), node.GetValue())
		if err != nil {
			node.SetFlags(&UnavailValueFlag{})
			txnOp.NewErr = err
			txnOp.NewState = kvs.ValueState_INVALID
			txnOp.NOOP = true
			s.updateNodeState(node, txnOp.NewState, args)
			node.SetFlags(&ErrorFlag{err: err, retriable: false})
			return kvs.RecordedTxnOps{txnOp}, err
		}
	}

	// compare new value with the old one
	equivalent := handler.equivalentValues(node.GetKey(), prevValue, args.kv.value)

	// re-create the value if required by the descriptor
	recreate := !equivalent &&
		args.kv.origin != kvs.FromSB &&
		handler.modifyWithRecreate(args.kv.key, node.GetValue(), args.kv.value, node.GetMetadata())

	if recreate {
		// record operation as two - delete followed by add
		delOp := s.preRecordTxnOp(args, node)
		delOp.Operation = kvs.TxnOperation_DELETE
		delOp.NewValue = nil
		delOp.IsRecreate = true
		addOp := s.preRecordTxnOp(args, node)
		addOp.Operation = kvs.TxnOperation_ADD
		addOp.PrevValue = nil
		addOp.IsRecreate = true
		// remove obsolete value
		delExec, inheritedErr := s.applyDelete(node, delOp, args, false)
		executed = append(executed, delExec...)
		if inheritedErr != nil {
			err = inheritedErr
			return
		}
		// add the new revision of the value
		addExec, inheritedErr := s.applyAdd(node, addOp, args)
		executed = append(executed, addExec...)
		err = inheritedErr
		return
	}

	// apply new relations
	derives, updateExecs, inheritedErr := s.applyNewRelations(node, handler, args)
	executed = append(executed, updateExecs...)
	if inheritedErr != nil {
		err = inheritedErr
		return
	}

	// if the new dependencies are not satisfied => delete and set as pending with the new value
	if !isNodeReady(node) {
		delExec, inheritedErr := s.applyDelete(node, txnOp, args, true)
		executed = append(executed, delExec...)
		if inheritedErr != nil {
			err = inheritedErr
		}
		return
	}

	// execute modify operation
	if !args.dryRun && !equivalent && descriptor != nil {
		var newMetadata interface{}

		// call Modify handler
		if args.kv.origin != kvs.FromSB {
			newMetadata, err = handler.modify(node.GetKey(), prevValue, node.GetValue(), node.GetMetadata())
		} else {
			// already modified in SB
			newMetadata = args.kv.metadata
		}

		if err != nil {
			retriableErr := handler.isRetriableFailure(err)
			txnOp.NewErr = err
			txnOp.NewState = s.markFailedValue(node, args, err, retriableErr)
			executed = append(executed, txnOp)
			return
		}

		// update metadata
		if canNodeHaveMetadata(node) && descriptor.WithMetadata {
			node.SetMetadata(newMetadata)
		}
	}

	// finalize node and save before going to new/modified derived values + dependencies
	node.DelFlags(ErrorFlagName, UnavailValueFlagName)
	if args.kv.origin == kvs.FromSB {
		txnOp.NewState = kvs.ValueState_RETRIEVED
	} else {
		txnOp.NewState = kvs.ValueState_CONFIGURED
	}
	s.updateNodeState(node, txnOp.NewState, args)

	// if the value was modified or the state changed, record operation
	if !equivalent || txnOp.PrevState != txnOp.NewState {
		// do not record transition if it only confirms that the value is in sync
		confirmsInSync := equivalent &&
			txnOp.PrevState == kvs.ValueState_FOUND &&
			txnOp.NewState == kvs.ValueState_CONFIGURED
		if !confirmsInSync {
			txnOp.NOOP = equivalent
			executed = append(executed, txnOp)
		}
	}

	// save before going into derived values
	if !args.dryRun {
		args.graphW.Save()
	}

	if !args.isDerived {
		// modify/add derived values
		var derivedVals []kvForTxn
		for _, derivedVal := range derives {
			derivedVals = append(derivedVals, kvForTxn{
				key:      derivedVal.Key,
				value:    derivedVal.Value,
				origin:   args.kv.origin,
				isRevert: args.kv.isRevert,
			})
		}
		derExecs, inheritedErr := s.applyDerived(derivedVals, args, true)
		executed = append(executed, derExecs...)
		if inheritedErr != nil {
			err = inheritedErr
		}
	}
	return
}

// applyNewRelations updates relation definitions and removes obsolete derived
// values.
func (s *Scheduler) applyNewRelations(node graph.NodeRW, handler *descriptorHandler,
	args *applyValueArgs) (derivedVals []kvs.KeyValuePair, executed kvs.RecordedTxnOps, err error) {

	// get the set of derived keys before update
	prevDerived := getDerivedKeys(node)

	// set new targets
	derivedVals = nil
	if !args.isDerived {
		derivedVals = handler.derivedValues(node.GetKey(), node.GetValue())
	}
	dependencies := handler.dependencies(node.GetKey(), node.GetValue())
	node.SetTargets(constructTargets(dependencies, derivedVals))

	if args.isDerived {
		return
	}

	// remove obsolete derived values
	var obsoleteDerVals []kvForTxn
	prevDerived.Subtract(getDerivedKeys(node))
	for _, obsolete := range prevDerived.Iterate() {
		obsoleteDerVals = append(obsoleteDerVals, kvForTxn{
			key:      obsolete,
			value:    nil, // delete
			origin:   args.kv.origin,
			isRevert: args.kv.isRevert,
		})
	}
	executed, err = s.applyDerived(obsoleteDerVals, args, false)
	return
}

// applyDerived (re-)applies the given list of derived values.
func (s *Scheduler) applyDerived(derivedVals []kvForTxn, args *applyValueArgs, check bool) (executed kvs.RecordedTxnOps, err error) {
	var wasErr error

	// order derivedVals by key (just for deterministic behaviour which simplifies testing)
	sort.Slice(derivedVals, func(i, j int) bool { return derivedVals[i].key < derivedVals[j].key })

	for _, derived := range derivedVals {
		if check && !s.validDerivedKV(args.graphW, derived, args.txn.seqNum) {
			continue
		}
		ops, _, err := s.applyValue(
			&applyValueArgs{
				graphW:    args.graphW,
				txn:       args.txn,
				kv:        derived,
				baseKey:   args.baseKey,
				isRetry:   args.isRetry,
				dryRun:    args.dryRun,
				isDerived: true, // <- is derived
				branch:    args.branch,
			})
		if err != nil {
			wasErr = err
		}
		executed = append(executed, ops...)
	}
	return executed, wasErr
}

// runUpdates triggers updates on all nodes that depend on the given node.
func (s *Scheduler) runUpdates(node graph.Node, args *applyValueArgs) (executed kvs.RecordedTxnOps) {
	depNodes := node.GetSources(DependencyRelation)

	// order depNodes by key (just for deterministic behaviour which simplifies testing)
	sort.Slice(depNodes, func(i, j int) bool { return depNodes[i].GetKey() < depNodes[j].GetKey() })

	for _, depNode := range depNodes {
		if getNodeOrigin(depNode) != kvs.FromNB {
			continue
		}
		value := depNode.GetValue()
		lastUpdate := getNodeLastUpdate(depNode)
		if lastUpdate != nil {
			// anything but state=FOUND
			value = lastUpdate.value
		}
		ops, _, _ := s.applyValue(
			&applyValueArgs{
				graphW: args.graphW,
				txn:    args.txn,
				kv: kvForTxn{
					key:      depNode.GetKey(),
					value:    value,
					origin:   getNodeOrigin(depNode),
					isRevert: args.kv.isRevert,
				},
				baseKey:   getNodeBaseKey(depNode),
				isRetry:   args.isRetry,
				dryRun:    args.dryRun,
				isDerived: isNodeDerived(depNode),
				isUpdate:  true, // <- update
				branch:    args.branch,
			})
		executed = append(executed, ops...)
	}
	return executed
}

// determineUpdateOperation determines if the value needs update and what operation to execute.
func (s *Scheduler) determineUpdateOperation(node graph.NodeRW, txnOp *kvs.RecordedTxnOp) {
	// add node if dependencies are now all met
	if !isNodeAvailable(node) {
		if !isNodeReady(node) {
			// nothing to do
			return
		}
		txnOp.Operation = kvs.TxnOperation_ADD
	} else if !isNodeReady(node) {
		// node should not be available anymore
		txnOp.Operation = kvs.TxnOperation_DELETE
	}
}

// compressTxnOps removes uninteresting intermediate pending Add/Delete operations.
func (s *Scheduler) compressTxnOps(executed kvs.RecordedTxnOps) kvs.RecordedTxnOps {
	// compress Add operations
	compressed := make(kvs.RecordedTxnOps, 0, len(executed))
	for i, op := range executed {
		compressedOp := false
		if op.Operation == kvs.TxnOperation_ADD && op.NewState == kvs.ValueState_PENDING {
			for j := i + 1; j < len(executed); j++ {
				if executed[j].Key == op.Key {
					if executed[j].Operation == kvs.TxnOperation_ADD {
						// compress
						compressedOp = true
						executed[j].PrevValue = op.PrevValue
						executed[j].PrevErr = op.PrevErr
						executed[j].PrevState = op.PrevState
					}
					break
				}
			}
		}
		if !compressedOp {
			compressed = append(compressed, op)
		}
	}

	// compress Delete operations
	length := len(compressed)
	for i := length - 1; i >= 0; i-- {
		op := compressed[i]
		compressedOp := false
		if op.Operation == kvs.TxnOperation_DELETE && op.PrevState == kvs.ValueState_PENDING {
			for j := i - 1; j >= 0; j-- {
				if compressed[j].Key == op.Key {
					if compressed[j].Operation == kvs.TxnOperation_DELETE {
						// compress
						compressedOp = true
						compressed[j].NewValue = op.NewValue
						compressed[j].NewErr = op.NewErr
						compressed[j].NewState = op.NewState
					}
					break
				}
			}
		}
		if compressedOp {
			copy(compressed[i:], compressed[i+1:])
			length--
		}
	}
	compressed = compressed[:length]
	return compressed
}

// updateNodeState updates node state if it is really necessary.
func (s *Scheduler) updateNodeState(node graph.NodeRW, newState kvs.ValueState, args *applyValueArgs) {
	if getNodeState(node) != newState {
		if s.logGraphWalk {
			indent := strings.Repeat(" ", args.branch.Length()*2)
			fmt.Printf("%s  -> change value state from %v to %v\n", indent, getNodeState(node), newState)
		}
		node.SetFlags(&ValueStateFlag{valueState: newState})
	}
}

func (s *Scheduler) markFailedValue(node graph.NodeRW, args *applyValueArgs, err error,
	retriableErr bool) (newState kvs.ValueState) {

	// decide value state between FAILED and RETRYING
	newState = kvs.ValueState_FAILED
	toBeReverted := args.txn.txnType == kvs.NBTransaction && args.txn.nb.revertOnFailure && !args.kv.isRevert
	if retriableErr && !toBeReverted {
		// consider operation retry
		var alreadyRetried bool
		if args.txn.txnType == kvs.RetryFailedOps {
			baseKey := getNodeBaseKey(node)
			_, alreadyRetried = args.txn.retry.keys[baseKey]
		}
		attempt := 1
		if alreadyRetried {
			attempt = args.txn.retry.attempt + 1
		}
		lastUpdate := getNodeLastUpdate(node)
		if lastUpdate.retryEnabled && lastUpdate.retryArgs != nil &&
			(lastUpdate.retryArgs.MaxCount == 0 || attempt <= lastUpdate.retryArgs.MaxCount) {
			// retry is allowed
			newState = kvs.ValueState_RETRYING
		}
	}
	s.updateNodeState(node, newState, args)
	node.SetFlags(&ErrorFlag{err: err, retriable: retriableErr})
	return newState
}

func (s *Scheduler) logNodeVisit(operation string, args *applyValueArgs) func() {
	msg := fmt.Sprintf("%s (key=%s, isDepUpdate=%t)", operation, args.kv.key, args.isUpdate)
	indent := strings.Repeat(" ", args.branch.Length()*2)
	fmt.Printf("%s%s %s\n", indent, nodeVisitBeginMark, msg)
	return func() {
		fmt.Printf("%s%s %s\n", indent, nodeVisitEndMark, msg)
	}
}

// validDerivedKV check validity of a derived KV pair.
func (s *Scheduler) validDerivedKV(graphR graph.ReadAccess, kv kvForTxn, txnSeqNum uint64) bool {
	node := graphR.GetNode(kv.key)
	if kv.value == nil {
		s.Log.WithFields(logging.Fields{
			"txnSeqNum": txnSeqNum,
			"key":       kv.key,
		}).Warn("Derived nil value")
		return false
	}
	if node != nil {
		if !isNodeDerived(node) {
			s.Log.WithFields(logging.Fields{
				"txnSeqNum": txnSeqNum,
				"value":     kv.value,
				"key":       kv.key,
			}).Warn("Skipping derived value colliding with a base value")
			return false
		}
	}
	return true
}
