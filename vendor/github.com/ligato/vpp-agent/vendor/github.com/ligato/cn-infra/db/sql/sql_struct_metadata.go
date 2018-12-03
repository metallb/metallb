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

import "reflect"

// TableName interface specifies custom table name for SQL statements.
type TableName interface {
	// TableName returns sql table name.
	TableName() string
}

// SchemaName interface specifies custom schema name for SQL statements.
type SchemaName interface {
	// SchemaName returns sql schema name where the table resides
	SchemaName() string
}

// EntityTableName returns the table name, possibly prefixed with the schema
// name, associated with the <entity>.
// The function tries to cast <entity> to TableName and SchemaName in order to
// obtain the table name and the schema name, respectively.
// If table name cannot be obtained, the struct name is used instead.
// If schema name cannot be obtained, it is simply omitted from the result.
func EntityTableName(entity interface{}) string {
	var tableName, schemaName string
	if nameProvider, ok := entity.(TableName); ok {
		tableName = nameProvider.TableName()
	}

	if tableName == "" {
		tableName = reflect.Indirect(reflect.ValueOf(entity)).Type().Name()
	}

	if schemaNameProvider, ok := entity.(SchemaName); ok {
		schemaName = schemaNameProvider.SchemaName()
	}

	if schemaName == "" {
		return tableName
	}

	return schemaName + "." + tableName
}
