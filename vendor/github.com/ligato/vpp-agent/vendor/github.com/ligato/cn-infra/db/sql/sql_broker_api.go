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

package sql

import (
	"io"
)

// Broker executes SQL statements in the data store.
// It marshals/un-marshals go structures.
type Broker interface {
	// Put puts single value <inBinding> into the data store.
	// Example usage:
	//
	//    err = db.Put("ID='James Bond'", &User{"James Bond", "James", "Bond"})
	//
	Put(where Expression, inBinding interface{} /* TODO opts ...PutOption*/) error

	// NewTxn creates a transaction / batch.
	NewTxn() Txn

	// GetValue retrieves one item based on the <query>. If the item exists,
	// it is un-marshaled into the <outBinding>.
	//
	// Example usage 1:
	//
	//    query := sql.FROM(UserTable, sql.WHERE(sql.Field(&UserTable.ID, sql.EQ("Bond")))
	//    user := &User{}
	//    found, err := db.GetValue(query, user)
	//
	// Example usage 2:
	//
	//    query := sql.FROM(JamesBond, sql.WHERE(sql.PK(&JamesBond.ID))
	//    user := &User{}
	//    found, err := db.GetValue(query, user)
	//
	GetValue(query Expression, outBinding interface{}) (found bool, err error)

	// ListValues returns an iterator that enables traversing all items
	// returned by the <query>.
	// Use utilities to:
	// - generate query string
	// - fill slice by values from iterator (SliceIt).
	//
	// Example usage 1 (fill slice with values from iterator):
	//
	//    query := sql.FROM(UserTable, sql.WHERE(sql.Field(&UserTable.LastName, sql.EQ("Bond")))
	//    iterator := db.ListValues(query)
	//    users := &[]User{}
	//    err := sql.SliceIt(users, iterator)
	//
	// Example usage 2:
	//
	//    query := sql.FROM(UserTable, sql.WHERE(sql.Exec("last_name='Bond'")))
	//    iterator := db.ListValues(query)
	//    users := &[]User{}
	//    err := sql.SliceIt(users, iterator)
	//
	// Example usage 3:
	//
	//    iterator := db.ListValues("select ID, first_name, last_name from User where last_name='Bond'")
	//    user := map[string]interface{}
	//    stop := iterator.GetNext(user)
	//
	ListValues(query Expression) ValIterator

	// Delete removes data from the data store.
	// Example usage 1:
	//
	//    query := sql.FROM(JamesBond, sql.WHERE(sql.PK(&JamesBond.ID))
	//    err := datasync.Delete(query)
	//
	// Example usage 2:
	//
	//    err := datasync.Delete("from User where ID='James Bond'")
	//
	// Example usage 3:
	//
	//    query := sql.FROM(UserTable, sql.WHERE(sql.Field(&UserTable.LastName, sql.EQ("Bond")))
	//    err := datasync.Delete(query)
	//
	Delete(fromWhere Expression) error

	// Executes the SQL statement (can be used, for example, to create
	// "table/type" if not exits...)
	// Example usage:
	//
	//  	 err := db.Exec("CREATE INDEX IF NOT EXISTS...")
	Exec(statement string, bindings ...interface{}) error
}

// ValIterator is an iterator returned by ListValues call.
type ValIterator interface {
	// GetNext retrieves the current "row" from query result.
	// GetValue is un-marshaled into the provided argument.
	// The stop=true will be returned if there is no more record or if an error
	// occurred (to get the error call Close()).
	// When the stop=true is returned, the outBinding was not updated.
	GetNext(outBinding interface{}) (stop bool)

	// Closer retrieves an error (if occurred) and releases the cursor.
	io.Closer
}

// Txn allows to group operations into the transaction or batch
// (depending on a particular data store).
// Transaction executes usually multiple operations in a more efficient way
// in contrast to executing them one by one.
type Txn interface {
	// Put adds put operation into the transaction.
	Put(where Expression, data interface{}) Txn
	// Delete adds delete operation into the transaction.
	Delete(fromWhere Expression) Txn
	// Commit tries to commit the transaction.
	Commit() error
}
