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

package cassandra

import (
	"github.com/ligato/cn-infra/db/sql"
	"github.com/ligato/cn-infra/utils/structs"
	"github.com/willfaught/gockle"
)

// NewBrokerUsingSession is a Broker constructor. Use it like this:
//
// session := gockle.NewSession(gocql.NewCluster("172.17.0.1"))
// defer db.Close()
// db := NewBrokerUsingSession(session)
// db.ListValues(...)
func NewBrokerUsingSession(gocqlSession gockle.Session) *BrokerCassa {
	return &BrokerCassa{session: gocqlSession}
}

// BrokerCassa implements interface db.Broker. This implementation simplifies work with gocql in the way
// that it is not need to write "SQL" queries. But the "SQL" is not really hidden, one can use it if needed.
// The "SQL" queries are generated from the go structures (see more details in Put, Delete, Key, GetValue, ListValues).
type BrokerCassa struct {
	session gockle.Session
}

// ValIterator is an iterator returned by ListValues call
type ValIterator struct {
	Delegate gockle.Iterator
}

// ErrIterator is an iterator that stops immediately and just returns last error on Close()
type ErrIterator struct {
	LastError error
}

// Put - see the description in interface sql.Broker.Put().
// Put generates statement & binding for gocql Exec().
// Any error returned from gockle.Session.Exec is propagated upwards.
func (pdb *BrokerCassa) Put(where sql.Expression, pointerToAStruct interface{} /*TODO TTL, opts ...datasync.PutOption*/) error {
	statement, bindings, err := PutExpToString(where, pointerToAStruct)

	if err != nil {
		return err
	}
	return pdb.session.Exec(statement, bindings...)
}

// Exec - see the description in interface sql.Broker.ExecPut()
// Exec runs statement (AS-IS) using gocql
func (pdb *BrokerCassa) Exec(statement string, binding ...interface{}) error {
	return pdb.session.Exec(statement, binding...)
}

// Delete - see the description in interface sql.Broker.ExecPut()
// Delete generates statement & binding for gocql Exec()
func (pdb *BrokerCassa) Delete(fromWhere sql.Expression) error {
	statement, bindings, err := ExpToString(fromWhere)
	if err != nil {
		return err
	}
	return pdb.session.Exec("DELETE"+statement, bindings...)
}

// GetValue - see the description in interface sql.Broker.GetValue()
// GetValue just iterate once for ListValues()
func (pdb *BrokerCassa) GetValue(query sql.Expression, reqObj interface{}) (found bool, err error) {
	it := pdb.ListValues(query)
	stop := it.GetNext(reqObj)
	return !stop, it.Close()
}

// ListValues retrieves an iterator for elements stored under the provided key.
// ListValues runs query (AS-IS) using gocql Scan Iterator.
func (pdb *BrokerCassa) ListValues(query sql.Expression) sql.ValIterator {
	queryStr, binding, err := SelectExpToString(query)
	if err != nil {
		return &ErrIterator{err}
	}

	it := pdb.session.ScanIterator(queryStr, binding...)
	return &ValIterator{it}
}

// GetNext returns the following item from the result set. If data was returned, found is set to true.
// argument "outVal" can be:
// - pointer to structure
// - map
func (it *ValIterator) GetNext(outVal interface{}) (stop bool) {
	if m, ok := outVal.(map[string]interface{}); ok {
		ok = it.Delegate.ScanMap(m)
		return !ok //if not ok than stop
	}

	_, ptrs := structs.ListExportedFieldsPtrs(outVal, cqlExported)
	ok := it.Delegate.Scan(ptrs...)
	return !ok //if not ok than stop
}

// Close the iterator. Note, the error is important (may occure during marshalling/un-marshalling)
func (it *ValIterator) Close() error {
	return it.Delegate.Close()
}

// GetNext returns the following item from the result set. If data was returned, found is set to true.
// argument "outVal" can be:
// - pointer to structure
// - map
func (it *ErrIterator) GetNext(outVal interface{}) (stop bool) {
	return true
}

// Close the iterator. Note, the error is important (may occure during marshalling/un-marshalling)
func (it *ErrIterator) Close() error {
	return it.LastError
}
