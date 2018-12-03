package gockle

import (
	"github.com/gocql/gocql"
	"github.com/maraino/go-mock"
)

// ColumnApplied is the name of a special column that has a bool that indicates
// whether a conditional statement was applied.
const ColumnApplied = "[applied]"

// Batch is an ordered collection of CQL queries.
type Batch interface {
	// Add adds the query for statement and arguments.
	Add(statement string, arguments ...interface{})

	// Exec executes the queries in the order they were added.
	Exec() error

	// ExecTx executes the queries in the order they were added. It returns a slice
	// of maps from columns to values, the maps corresponding to all the conditional
	// queries, and ordered in the same relative order. The special column
	// ColumnApplied has a bool that indicates whether the conditional statement was
	// applied. If a conditional statement was not applied, the current values for
	// the columns are put into the map.
	ExecTx() ([]map[string]interface{}, error)
}

var (
	_ Batch = BatchMock{}
	_ Batch = batch{}
)

// BatchKind is the kind of Batch. The choice of kind mostly affects performance.
type BatchKind byte

// Kinds of batches.
const (
	// BatchLogged queries are atomic. Queries are only isolated within a single
	// partition.
	BatchLogged BatchKind = 0

	// BatchUnlogged queries are not atomic. Atomic queries spanning multiple partitions cost performance.
	BatchUnlogged BatchKind = 1

	// BatchCounter queries update counters and are not idempotent.
	BatchCounter BatchKind = 2
)

// BatchMock is a mock Batch. See github.com/maraino/go-mock.
type BatchMock struct {
	mock.Mock
}

// Add implements Batch.
func (m BatchMock) Add(statement string, arguments ...interface{}) {
	m.Called(statement, arguments)
}

// Exec implements Batch.
func (m BatchMock) Exec() error {
	return m.Called().Error(0)
}

// ExecTx implements Batch.
func (m BatchMock) ExecTx() ([]map[string]interface{}, error) {
	var r = m.Called()

	return r.Get(0).([]map[string]interface{}), r.Error(1)
}

type batch struct {
	b *gocql.Batch

	s *gocql.Session
}

func (b batch) Add(statement string, arguments ...interface{}) {
	b.b.Query(statement, arguments...)
}

func (b batch) Exec() error {
	return b.s.ExecuteBatch(b.b)
}

func (b batch) ExecTx() ([]map[string]interface{}, error) {
	var m = map[string]interface{}{}
	var a, i, err = b.s.MapExecuteBatchCAS(b.b, m)

	if err != nil {
		return nil, err
	}

	s, err := i.SliceMap()

	if err != nil {
		return nil, err
	}

	if err := i.Close(); err != nil {
		return nil, err
	}

	m[ColumnApplied] = a
	s = append([]map[string]interface{}{m}, s...)

	return s, nil
}
