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
	"testing"

	"github.com/ligato/cn-infra/db/sql"
	"github.com/ligato/cn-infra/db/sql/cassandra"
	"github.com/onsi/gomega"
)

// TestPut1_convenient is most convenient way of putting one entity to cassandra
func TestPut1_convenient(t *testing.T) {
	gomega.RegisterTestingT(t)

	session := mockSession()
	defer session.Close()
	db := cassandra.NewBrokerUsingSession(session)

	sqlStr, _, _ := cassandra.PutExpToString(sql.FieldEQ(&JamesBond.ID), JamesBond)
	gomega.Expect(sqlStr).Should(gomega.BeEquivalentTo(
		"UPDATE User SET first_name = ?, last_name = ? WHERE id = ?"))

	mockExec(session, sqlStr, []interface{}{
		"James Bond", //set ID
		"James",      //set first_name
		"Bond",       //set last_name
		"James Bond", //where
	})
	err := db.Put(sql.FieldEQ(&JamesBond.ID), JamesBond)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
}

// TestPut2_EQ is most convenient way of putting one entity to cassandra
func TestPut2_EQ(t *testing.T) {
	gomega.RegisterTestingT(t)

	session := mockSession()
	defer session.Close()
	db := cassandra.NewBrokerUsingSession(session)

	sqlStr, _, _ := cassandra.PutExpToString(sql.FieldEQ(&JamesBond.ID), JamesBond)
	gomega.Expect(sqlStr).Should(gomega.BeEquivalentTo(
		"UPDATE User SET first_name = ?, last_name = ? WHERE id = ?"))

	mockExec(session, sqlStr, []interface{}{
		"James Bond", //set ID
		"James",      //set first_name
		"Bond",       //set last_name
		"James Bond", //where
	})
	err := db.Put(sql.Field(&JamesBond.ID, sql.EQ(JamesBond.ID)), JamesBond)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
}

// TestPut3_customTableSchema checks that generated SQL statements
// contain customized table name & schema (see interfaces sql.TableName, sql.SchemaName)
func TestPut3_customTableSchema(t *testing.T) {
	gomega.RegisterTestingT(t)

	session := mockSession()
	defer session.Close()
	db := cassandra.NewBrokerUsingSession(session)

	entity := &CustomizedTablenameAndSchema{ID: "id", LastName: "Bond"}

	sqlStr, _, _ := cassandra.PutExpToString(sql.FieldEQ(&entity.ID), entity)
	gomega.Expect(sqlStr).Should(gomega.BeEquivalentTo(
		"UPDATE my_custom_schema.my_custom_name SET last_name = ? WHERE id = ?"))

	mockExec(session, sqlStr, []interface{}{
		"James Bond", //set ID
		"James",      //set first_name
		"Bond",       //set last_name
		"James Bond", //where
	})
	err := db.Put(sql.FieldEQ(&entity.ID), entity)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
}

// TestPut4_uuid_EQ used to verify inserting entity having a field of type gocql.UUID and using FieldEQ in where condition
func TestPut4_uuid_FieldEQ(t *testing.T) {
	gomega.RegisterTestingT(t)

	session := mockSession()
	defer session.Close()
	db := cassandra.NewBrokerUsingSession(session)

	sqlStr, _, _ := cassandra.PutExpToString(sql.FieldEQ(&MyTweet.ID), MyTweet)

	gomega.Expect(sqlStr).Should(gomega.BeEquivalentTo(
		"UPDATE Tweet SET text = ? WHERE id = ?"))

	mockExec(session, sqlStr, []interface{}{
		myID,          //set ID
		"hello world", //set Text
		myID,          //where
	})
	err := db.Put(sql.FieldEQ(&MyTweet.ID), MyTweet)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
}

// TestPut5_EQ used to verify inserting entity having a field of type gocql.UUID and using EQ in where condition
func TestPut5_uuid_EQ(t *testing.T) {
	gomega.RegisterTestingT(t)

	session := mockSession()
	defer session.Close()
	db := cassandra.NewBrokerUsingSession(session)

	sqlStr, _, _ := cassandra.PutExpToString(sql.FieldEQ(&MyTweet.ID), MyTweet)
	gomega.Expect(sqlStr).Should(gomega.BeEquivalentTo(
		"UPDATE Tweet SET text = ? WHERE id = ?"))

	mockExec(session, sqlStr, []interface{}{
		myID,          //set ID
		"hello world", //set Text
		myID,          //where
	})
	err := db.Put(sql.Field(&MyTweet.ID, sql.EQ(MyTweet.ID)), MyTweet)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
}
