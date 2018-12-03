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

package cassandra_test

import (
	"errors"

	"github.com/gocql/gocql"
	"github.com/ligato/cn-infra/db/sql"
	"github.com/ligato/cn-infra/db/sql/cassandra"
	"github.com/maraino/go-mock"
	"github.com/willfaught/gockle"
)

// test data
var JamesBond = &User{ID: "James Bond", FirstName: "James", LastName: "Bond"}
var PeterBond = &User{ID: "Peter Bond", FirstName: "Peter", LastName: "Bond"}

var myID gocql.UUID = gocql.TimeUUID()
var MyTweet = &Tweet{ID: myID.String(), Text: "hello"}

// instance that represents users table (used in queries to define columns)
var UserTable = &User{}

// instance that represents tweets table (used in queries to define columns)
var TweetTable = &Tweet{}

//var UsersTypeInfo = map[string /*FieldName*/ ]gocql.TypeInfo{
//runtimeutils.GetFunctionName(UserTable.GetLastName): gocql.NewNativeType(0x03, gocql.TypeVarchar, ""),
//"LastName":                                          gocql.NewNativeType(0x03, gocql.TypeVarchar, ""),
//}

// User is simple structure for testing purposes
type User struct {
	ID                string `cql:"id" pk:"id"`
	FirstName         string `cql:"first_name"`
	LastName          string `cql:"last_name"`
	ExportedButNotCql string `cql:"-"`
	notExported       string
}

// Tweet structure using uuid for testing purposes
type Tweet struct {
	ID   string `cql:"id" pk:"id"`
	Text string `cql:"text"`
}

// CustomizedTablenameAndSchema implements sql.TableName, sql.SchemaName interfaces
type CustomizedTablenameAndSchema struct {
	ID       string `cql:"id" pk:"id"`
	LastName string `cql:"last_name"`
}

// TableName implements sql.TableName interface
func (entity *CustomizedTablenameAndSchema) TableName() string {
	return "my_custom_name"
}

// SchemaName implements sql.SchemaName interface
func (entity *CustomizedTablenameAndSchema) SchemaName() string {
	return "my_custom_schema"
}

// simple structure that holds values of one row for mock iterator
type row struct {
	values []interface{}
	fields []string
}

// mockQuery is a helper for testing. It setups mock iterator
func mockQuery(sessionMock *gockle.SessionMock, query sql.Expression, rows ...*row) {
	sqlStr, _ /*binding*/, err := cassandra.SelectExpToString(query)
	if err != nil {
		panic(err.Error())
	}
	sessionMock.When("ScanIterator", sqlStr, mock.Any).Return(&IteratorMock{rows: rows})
	sessionMock.When("Close").Return()

}

// mockExec is a helper for testing. It setups mock iterator with any parameters/arguments
func mockExec(sessionMock *gockle.SessionMock, query string, binding []interface{}) {
	sessionMock.When("Exec", query, mock.Any).Return(nil)
	sessionMock.When("Close").Return()
}

// cells is a helper that harvests all exported fields values
func cells(entity interface{}) (cellsInRow *row) {
	fields, values := cassandra.SliceOfFieldsWithValPtrs(entity)
	return &row{values, fields}
}

// IteratorMock is a mock Iterator. See github.com/maraino/go-mock.
type IteratorMock struct {
	index   int
	rows    []*row
	closed  bool
	lastErr error
}

// Close implements Iterator.
func (m IteratorMock) Close() error {
	if m.closed {
		return errors.New("already closed")
	}
	return m.lastErr
}

// Scan implements Iterator.
func (m *IteratorMock) Scan(results ...interface{}) bool {
	if len(m.rows) > m.index {
		for i := 0; i < len(results) && i < len(m.rows[m.index].values); i++ {
			// TODO !!! types of fields
			typeInfo := gocql.NewNativeType(0x03, gocql.TypeVarchar, "")
			bytes, err := gocql.Marshal(typeInfo, m.rows[m.index].values[i])
			if err != nil {
				m.lastErr = err
				return false
			}
			err = gocql.Unmarshal(typeInfo, bytes, results[i])
			if err != nil {
				m.lastErr = err
				return false
			}
		}

		m.index++
		return true
	}
	return false
}

// ScanMap implements Iterator.
func (m *IteratorMock) ScanMap(results map[string]interface{}) bool {
	if len(m.rows) > m.index {
		for i := 0; i < len(m.rows[m.index].values) && i < len(m.rows[m.index].fields); i++ {
			key := m.rows[m.index].fields[i]
			value := m.rows[m.index].values[i]
			results[key] = value
		}

		m.index++
		return true
	}
	return false
}

func mockSession() (sessionMock *gockle.SessionMock) {
	sessionMock = &gockle.SessionMock{}
	sessionMock.When("Close").Return(nil)
	return sessionMock
}
