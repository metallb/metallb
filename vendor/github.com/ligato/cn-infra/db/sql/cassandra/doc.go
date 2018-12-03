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

// Package cassandra is the implementation of the SQL Data Broker client
// API for the Cassandra data store. See cn-infra/db/sql for the definition
// of the key-value Data Broker client API.
//
// The entity that provides access to the data store is called gocql.Session (wrapped by Broker for convenience).
//
//      +--------+             +----------+          crud           +-----------+
//      | Broker |    ---->    | Session  |          ---->          | Cassandra |
//      +--------+             +----------+                         +-----------+
//
// To create a Session use the following function
//
//    import "github.com/gocql/gocql"
//
//    cluster := gocql.NewCluster("172.17.0.1")
//    cluster.Keyspace = "demo"
//    session, err := cluster.CreateSession()
//
// Then create broker instance:
//
//    import (
//          "github.com/ligato/cn-infra/db/sql/cassandra"
//          "github.com/willfaught/gockle"
//    )
//    db := cassandra.NewBrokerUsingSession(gockle.NewSession(session)))
//
// To insert single key-value pair into Cassandra run (both values are pointers, JamesBond is instance of User struct.):
//		db.Put(sql.PK(&JamesBond.ID), JamesBond)
// To remove a value identified by key:
//      datasync.Delete(sql.FROM(JamesBond, sql.WHERE(sql.PK(&JamesBond.ID)))
//
// To retrieve a value identified by key (both values are pointers):
//    data, found, rev, err := db.GetValue(sql.FROM(UserTable, sql.WHERE(sql.Field(&UserTable.ID, sql.EQ("James Bond"))))
//    if err == nil && found {
//       ...
//    }
//
// To retrieve all values matching a key prefix:
//    itr, err := db.ListValues(sql.FROM(UserTable, sql.WHERE(sql.Field(&UserTable.LastName, sql.EQ("Bond"))))
//    if err != nil {
//       for {
//          data, allReceived, rev, err := itr.GetNext()
//          if allReceived {
//              break
//          }
//          if err != nil {
//              return err
//          }
//          process data...
//       }
//    }
//
// To retrieve values more conveniently directrly in slice (without using iterator):
//    users := &[]User{}
//     err := sql.SliceIt(users, db.ListValues(sql.FROM(UserTable,
//                  sql.WHERE(sql.Field(&UserTable.LastName, sql.EQ("Bond"))))
//
//
package cassandra
