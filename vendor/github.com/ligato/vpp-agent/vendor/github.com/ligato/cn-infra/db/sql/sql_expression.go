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
	"bytes"
	"fmt"
	"strings"
)

// Expression represents part of SQL statement and optional binding ("?").
type Expression interface {
	// Stringer prints default representation of SQL to String.
	// Different implementations can override this using package specific
	// func ExpToString().
	String() string

	// Binding are values referenced ("?") from the statement.
	GetBinding() []interface{}

	// Accepts calls the methods on Visitor.
	Accept(Visitor)
}

// Visitor is used for traversing an expression tree.
type Visitor interface {
	VisitPrefixedExp(*PrefixedExp)
	VisitFieldExpression(*FieldExpression)
}

// PrefixedExp covers many SQL constructions. It implements sql.Expression
// interface. Instance of this structure is returned by many helper functions
// below.
type PrefixedExp struct {
	Prefix      string
	AfterPrefix []Expression
	Suffix      string
	Binding     []interface{}
}

// String returns Prefix + " " + AfterPrefix.
func (exp *PrefixedExp) String() string {
	if exp.AfterPrefix == nil {
		return exp.Prefix
	}

	if exp.Prefix == "FROM" && len(exp.Binding) > 0 {
		return exp.Prefix + " " + EntityTableName(exp.Binding[0]) + " " + ExpsToString(exp.AfterPrefix)
	}

	return exp.Prefix + " " + ExpsToString(exp.AfterPrefix)
}

// ExpsToString joins (without separator) individual expression string representations.
func ExpsToString(exps []Expression) string {
	if exps != nil {
		if len(exps) == 1 {
			return exps[0].String()
		}

		var buffer bytes.Buffer
		for _, exp := range exps {
			buffer.WriteString(exp.String())
		}

		return buffer.String()
	}
	return ""
}

// GetBinding is a getter.
func (exp *PrefixedExp) GetBinding() []interface{} {
	return exp.Binding
}

// Accept calls VisitPrefixedExp(...) & Accept(AfterPrefix).
func (exp *PrefixedExp) Accept(visitor Visitor) {
	visitor.VisitPrefixedExp(exp)
}

// FieldExpression is used for addressing field of an entity in an SQL expression.
type FieldExpression struct {
	PointerToAField interface{}
	AfterField      Expression
}

// String returns Prefix + " " + AfterPrefix.
func (exp *FieldExpression) String() string {
	prefix := fmt.Sprint("<field on ", exp.PointerToAField, ">")
	if exp.AfterField == nil {
		return prefix
	}
	return prefix + " " + exp.AfterField.String()
}

// GetBinding is a getter.
func (exp *FieldExpression) GetBinding() []interface{} {
	return nil
}

// Accept calls VisitFieldExpression(...) & Accept(AfterField).
func (exp *FieldExpression) Accept(visitor Visitor) {
	visitor.VisitFieldExpression(exp)
}

// SELECT keyword of an SQL expression.
func SELECT(entity interface{}, afterKeyword Expression, binding ...interface{}) Expression {
	return &PrefixedExp{"SELECT", []Expression{FROM(entity, afterKeyword)}, "", binding}
}

// FROM keyword of an SQL expression.
// Note, pointerToAStruct is assigned to Expression.binding.
// The implementation is supposed to try to cast to the sql.TableName & sql.SchemaName.
func FROM(pointerToAStruct interface{}, afterKeyword Expression) Expression {
	return &PrefixedExp{"FROM", []Expression{afterKeyword}, "", []interface{}{pointerToAStruct}}
}

// WHERE keyword of an SQL statement.
func WHERE(afterKeyword ...Expression) Expression {
	return &PrefixedExp{" WHERE ", afterKeyword, "", nil}
}

// DELETE keyword of an SQL statement.
func DELETE(entity interface{}, afterKeyword Expression) Expression {
	return &PrefixedExp{"DELETE", []Expression{afterKeyword}, "", nil}
}

// Exp function creates instance of sql.Expression from string statement & optional binding.
// Useful for:
// - rarely used parts of an SQL statements
// - CREATE IF NOT EXISTS statements
func Exp(statement string, binding ...interface{}) Expression {
	return &PrefixedExp{statement, nil, "", binding}
}

var emptyAND = &PrefixedExp{" AND ", nil, "", nil}

// AND keyword of SQL expression
//
// Example usage (alternative 1 - spare sequence of partenthesis):
//
// 		WHERE(FieldEQ(&JamesBond.FirstName), AND(), FieldEQ(&JamesBond.LastName))
//
// Example usage (alternative 2 - useful for nesting):
//
// 		WHERE(AND(FieldEQ(&JamesBond.FirstName), FieldEQ(&JamesBond.LastName)))
func AND(inside ...Expression) Expression {
	return intelligentOperator(emptyAND, inside...)
}

var emptyOR = &PrefixedExp{" OR ", nil, "", nil}

// OR keyword of SQL expression
//
// Example usage 1 (generated string does not contain parenthesis surrounding OR):
//
// 		WHERE(FieldEQ(&PeterBond.FirstName), OR(), FieldEQ(&JamesBond.FirstName))
//
// Example usage 2 (generated string does not contain parenthesis surrounding OR):
//
// 		WHERE(FieldEQ(&PeterBond.LastName), OR(FieldEQ(&PeterBond.FirstName), FieldEQ(&JamesBond.FirstName)))
//
func OR(inside ...Expression) Expression {
	return intelligentOperator(emptyOR, inside...)
}

func intelligentOperator(emptyOperator *PrefixedExp, inside ...Expression) Expression {
	lenInside := len(inside)
	if lenInside == 0 {
		return emptyOperator
	}
	if lenInside == 1 {
		return &PrefixedExp{emptyOperator.Prefix, inside, "", nil}
	}
	inside2 := []Expression{}
	for i, exp := range inside {
		if i > 0 {
			inside2 = append(inside2, emptyOperator, exp)
		} else {
			inside2 = append(inside2, exp)
		}
	}

	return Parenthesis(inside2...)
}

// Field is a helper function to address field of a structure.
//
// Example usage:
//   Where(Field(&UsersTable.LastName, UsersTable, EQ('Bond'))
//   // generates, for example, "WHERE last_name='Bond'"
func Field(pointerToAField interface{}, rigthOperand ...Expression) (exp Expression) {
	if len(rigthOperand) == 0 {
		return &FieldExpression{pointerToAField, nil}
	}
	return &FieldExpression{pointerToAField, rigthOperand[0]}
}

// FieldEQ is combination of Field & EQ on the same pointerToAField.
//
// Example usage:
//   FROM(JamesBond, Where(FieldEQ(&JamesBond.LastName))
//   // generates, for example, "WHERE last_name='Bond'"
//   // because JamesBond is a pointer to an instance of a structure that in field LastName contains "Bond"
func FieldEQ(pointerToAField interface{}) (exp Expression) {
	return &FieldExpression{pointerToAField, EQ(pointerToAField)}
}

// PK is alias FieldEQ (user for better readability)
//
// Example usage:
//   FROM(JamesBond, Where(PK(&JamesBond.LastName))
//   // generates, for example, "WHERE last_name='Bond'"
//   // because JamesBond is a pointer to an instance of a structure that in field LastName contains "Bond"
func PK(pointerToAField interface{}) (exp Expression) {
	return FieldEQ(pointerToAField)
}

// EQ operator "=" used in SQL expressions
func EQ(binding interface{}) (exp Expression) {
	return &PrefixedExp{" = ", []Expression{Exp("?", binding)}, "", nil}
}

// GT operator ">" used in SQL expressions
func GT(binding interface{}) (exp Expression) {
	return &PrefixedExp{" > ", []Expression{Exp("?", binding)}, "", nil}
}

// GTE operator "=>" used in SQL expressions
func GTE(binding interface{}) (exp Expression) {
	return &PrefixedExp{" => ", []Expression{Exp("?", binding)}, "", nil}
}

// LT operator "<" used in SQL expressions
func LT(binding interface{}) (exp Expression) {
	return &PrefixedExp{" < ", []Expression{Exp("?", binding)}, "", nil}
}

// LTE operator "=<" used in SQL expressions
func LTE(binding interface{}) (exp Expression) {
	return &PrefixedExp{" =< ", []Expression{Exp("?", binding)}, "", nil}
}

// Parenthesis expression that surrounds "inside Expression" with "(" and ")"
func Parenthesis(inside ...Expression) (exp Expression) {
	return &PrefixedExp{"(", inside, ")", nil}
}

// IN operator of SQL expression
// 		FROM(UserTable,WHERE(FieldEQ(&UserTable.FirstName, IN(JamesBond.FirstName, PeterBond.FirstName)))
func IN(binding ...interface{}) (exp Expression) {
	bindingRefs := strings.Repeat(",?", len(binding))
	return &PrefixedExp{" IN(", nil, bindingRefs[1:] + ")", binding}
}
