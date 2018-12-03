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

// TestListValues1_convenient is most convenient way of selecting slice of entities
// User of the API does not need to write SQL string (string is calculated from the entity type.
// User of the API does not need to use the Iterator (user gets directly slice of th entity type - reflection needed).
func TestListValues1_convenient(t *testing.T) {
	gomega.RegisterTestingT(t)

	session := mockSession()
	defer session.Close()
	db := cassandra.NewBrokerUsingSession(session)

	query := sql.FROM(UserTable, sql.WHERE(sql.Field(&UserTable.LastName, sql.EQ("Bond"))))

	sqlStr, _ /*binding*/, err := cassandra.SelectExpToString(query)
	gomega.Expect(sqlStr).Should(gomega.BeEquivalentTo(
		"SELECT id, first_name, last_name FROM User WHERE last_name = ?"))

	mockQuery(session, query, cells(JamesBond), cells(PeterBond))

	users := &[]User{}
	err = sql.SliceIt(users, db.ListValues(query))

	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	gomega.Expect(users).ToNot(gomega.BeNil())
	gomega.Expect(users).To(gomega.BeEquivalentTo(&[]User{*JamesBond, *PeterBond}))
}

// TestListValues_AND used to test where clause with multiple AND conditions
func TestListValues_AND(t *testing.T) {
	gomega.RegisterTestingT(t)
	session := mockSession()
	defer session.Close()
	db := cassandra.NewBrokerUsingSession(session)
	query := sql.FROM(UserTable, sql.WHERE(
		sql.Field(&UserTable.FirstName), sql.EQ("James"),
		sql.AND(),
		sql.Field(&UserTable.LastName), sql.EQ("Bond")))
	sqlStr, _ /*binding*/, err := cassandra.SelectExpToString(query)
	gomega.Expect(sqlStr).Should(gomega.BeEquivalentTo(
		"SELECT id, first_name, last_name FROM User WHERE first_name = ? AND last_name = ? ALLOW FILTERING"))
	mockQuery(session, query, cells(JamesBond))
	users := &[]User{}
	err = sql.SliceIt(users, db.ListValues(query))
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	gomega.Expect(users).ToNot(gomega.BeNil())
	gomega.Expect(users).To(gomega.BeEquivalentTo(&[]User{*JamesBond}))
}

// TestListValues_IN used to test where clause containing IN condition
func TestListValues_IN(t *testing.T) {
	gomega.RegisterTestingT(t)
	session := mockSession()
	defer session.Close()
	db := cassandra.NewBrokerUsingSession(session)
	query := sql.FROM(UserTable, sql.WHERE(
		sql.Field(&UserTable.LastName), sql.EQ("Bond"),
		sql.AND(),
		sql.Field(&UserTable.FirstName), sql.IN("James", "Peter")))
	sqlStr, _ /*binding*/, err := cassandra.SelectExpToString(query)
	gomega.Expect(sqlStr).Should(gomega.BeEquivalentTo(
		"SELECT id, first_name, last_name FROM User WHERE last_name = ? AND first_name IN(?,?) ALLOW FILTERING"))
	mockQuery(session, query, cells(JamesBond), cells(PeterBond))
	users := &[]User{}
	err = sql.SliceIt(users, db.ListValues(query))
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	gomega.Expect(users).ToNot(gomega.BeNil())
	gomega.Expect(users).To(gomega.BeEquivalentTo(&[]User{*JamesBond, *PeterBond}))
}

/*
// TestListValues2_constFieldname let's user to write field name in where statement in old way using string constant.
// All other is same as in TestListValues1
func TestListValues2_constFieldname(t *testing.T) {
	gomega.RegisterTestingT(t)
	session := mockSession()
	defer session.Close()
	db := cassandra.NewBrokerUsingSession(session)
	query := sql.SelectFrom(UserTable) + sql.Where(sql.EQ("last_name", "Bond"))
	mockQuery(session, query, cells(JamesBond), cells(PeterBond))
	users := &[]User{}
	err := sql.SliceIt(users, db.ListValues(query))
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	gomega.Expect(users).ToNot(gomega.BeNil())
	gomega.Expect(users).To(gomega.BeEquivalentTo(&[]User{JamesBond, PeterBond}))
}

/*
// TestListValues2_constFieldname let's user to write field name in where statement in old way using string constant.
// All other is same as in TestListValues1
func TestListValues2_constFieldname(t *testing.T) {
	gomega.RegisterTestingT(t)

	session := mockSession()
	defer session.Close()
	db := cassandra.NewBrokerUsingSession(session)

	query := sql.SelectFrom(UserTable) + sql.Where(sql.EQ("last_name", "Bond"))
	mockQuery(session, query, cells(JamesBond), cells(PeterBond))

	users := &[]User{}
	err := sql.SliceIt(users, db.ListValues(query))

	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	gomega.Expect(users).ToNot(gomega.BeNil())
	gomega.Expect(users).To(gomega.BeEquivalentTo(&[]User{JamesBond, PeterBond}))
}

// TestListValues3_customSQL let's user to write part of the SQL statement/query
// All other is same as in TestListValues1
func TestListValues3_customSQL(t *testing.T) {
	gomega.RegisterTestingT(t)

	session := mockSession()
	defer session.Close()
	db := cassandra.NewBrokerUsingSession(session)

	query := sql.SelectFrom(UserTable) + "WHERE last_name = 'Bond'"
	mockQuery(session, query, cells(JamesBond), cells(PeterBond))

	users := &[]User{}
	err := sql.SliceIt(users, db.ListValues(query))

	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	gomega.Expect(users).ToNot(gomega.BeNil())
	gomega.Expect(users).To(gomega.BeEquivalentTo(&[]User{JamesBond, PeterBond}))
}

// TestListValues4_iterator does not use reflection to fill slice of users (but the iterator)
// All other is same as in TestListValues1
func TestListValues4_iterator(t *testing.T) {
	gomega.RegisterTestingT(t)

	session := mockSession()
	defer session.Close()
	db := cassandra.NewBrokerUsingSession(session)

	query := sql.SelectFrom(UserTable) + sql.Where(sql.Field(&UserTable.LastName, UserTable, "Bond"))
	mockQuery(session, query, cells(JamesBond), cells(PeterBond))

	users := []*User{}
	it := db.ListValues(query)
	for {
		user := &User{}
		stop := it.GetNext(user)
		if stop {
			break
		}
		users = append(users, user)
	}
	err := it.Close()

	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	gomega.Expect(users).ToNot(gomega.BeNil())
	gomega.Expect(users).To(gomega.BeEquivalentTo([]*User{&JamesBond, &PeterBond}))
}


// TestListValues4_iteratorScanMap does not use reflection to fill slice of users (but the iterator)
// All other is same as in TestListValues1
func TestListValues4_iteratorScanMap(t *testing.T) {
	gomega.RegisterTestingT(t)

	session := mockSession()
	defer session.Close()
	db := cassandra.NewBrokerUsingSession(session)

	query := sql.SelectFrom(UserTable) + sql.Where(sql.Field(&UserTable.LastName, UserTable, "Bond"))
	mockQuery(session, query, cells(JamesBond), cells(PeterBond))

	it := db.ListValues(query)
	for {
		user := map[string]interface{}{}
		stop := it.GetNext(user)
		if stop {
			break
		}
		fmt.Println("user: ", user)
	}
	err := it.Close()

	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	//gomega.Expect(users).ToNot(gomega.BeNil())
	//gomega.Expect(users).To(gomega.BeEquivalentTo([]*User{&JamesBond, &PeterBond}))
}
*/

// TestListValues5_customTableSchema checks that generated SQL statements
// contain customized table name & schema (see interfaces sql.TableName, sql.SchemaName)
func TestListValues5_customTableSchema(t *testing.T) {
	gomega.RegisterTestingT(t)

	session := mockSession()
	defer session.Close()
	db := cassandra.NewBrokerUsingSession(session)

	entity := &CustomizedTablenameAndSchema{ID: "id", LastName: "Bond"}
	query := sql.FROM(entity, sql.WHERE(sql.Field(&entity.LastName, sql.EQ("Bond"))))
	mockQuery(session, query, cells(entity))

	sqlStr, _ /*binding*/, err := cassandra.SelectExpToString(query)
	gomega.Expect(sqlStr).Should(gomega.BeEquivalentTo(
		"SELECT id, last_name FROM my_custom_schema.my_custom_name WHERE last_name = ?"))

	users := &[]CustomizedTablenameAndSchema{}
	err = sql.SliceIt(users, db.ListValues(query))

	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	gomega.Expect(users).ToNot(gomega.BeNil())
	gomega.Expect(users).To(gomega.BeEquivalentTo(&[]CustomizedTablenameAndSchema{*entity}))
}

// TestListValues6_uuid checks whether we are able to retrieve uuid field
func TestListValues6_uuid(t *testing.T) {
	gomega.RegisterTestingT(t)

	session := mockSession()
	defer session.Close()
	db := cassandra.NewBrokerUsingSession(session)

	query := sql.FROM(TweetTable, sql.WHERE(sql.Field(&TweetTable.ID, sql.EQ(myID))))

	sqlStr, _, err := cassandra.SelectExpToString(query)
	gomega.Expect(sqlStr).Should(gomega.BeEquivalentTo(
		"SELECT id, text FROM Tweet WHERE id = ?"))

	mockQuery(session, query, cells(MyTweet))

	tweets := &[]Tweet{}
	err = sql.SliceIt(tweets, db.ListValues(query))

	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	gomega.Expect(tweets).ToNot(gomega.BeNil())
	gomega.Expect(tweets).To(gomega.BeEquivalentTo(&[]Tweet{*MyTweet}))
}
