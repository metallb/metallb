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

// TestDel1_convenient is most convenient way of deletening from cassandra
func TestDel1_convenient(t *testing.T) {
	gomega.RegisterTestingT(t)

	session := mockSession()
	defer session.Close()
	db := cassandra.NewBrokerUsingSession(session)

	mockExec(session, "DELETE FROM User WHERE id = ?",
		[]interface{}{
			"James Bond",
		})

	err := db.Delete(sql.FROM(JamesBond, sql.WHERE(sql.FieldEQ(&JamesBond.ID))))
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
}

// TestDel2_customTableSchema checks that generated SQL statements
// contain customized table name & schema (see interfaces sql.TableName, sql.SchemaName)
func TestDel2_customTableSchema(t *testing.T) {
	gomega.RegisterTestingT(t)

	session := mockSession()
	defer session.Close()
	db := cassandra.NewBrokerUsingSession(session)

	entity := &CustomizedTablenameAndSchema{ID: "id", LastName: "Bond"}

	mockExec(session, "DELETE FROM my_custom_schema.my_custom_name WHERE id = ?",
		[]interface{}{
			"James Bond",
		})

	err := db.Delete(sql.FROM(entity, sql.WHERE(sql.FieldEQ(&entity.ID))))
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
}
